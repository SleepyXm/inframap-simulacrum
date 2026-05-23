package walker

import (
	"regexp"
	"strings"
)

// compiledPattern holds a compiled regex alongside its source Pattern metadata.
type compiledPattern struct {
	re  *regexp.Regexp
	src Pattern
}

// Matcher applies language struct patterns against a file's lines.
type Matcher struct {
	ls             *LanguageStruct
	cfg            BracketConfig
	ctxCfg         ContextConfig
	routerPatterns []compiledPattern
	groupPatterns  []compiledPattern
	dbPatterns     []compiledPattern
}

// NewMatcher compiles all patterns from a LanguageStruct.
// Returns an error if any pattern fails to compile.
func NewMatcher(ls *LanguageStruct, bracketCfg BracketConfig, ctxCfg ContextConfig) (*Matcher, error) {
	m := &Matcher{ls: ls, cfg: bracketCfg, ctxCfg: ctxCfg}

	var err error
	if m.routerPatterns, err = compilePatterns(ls.RouterRegistration); err != nil {
		return nil, err
	}
	if m.groupPatterns, err = compilePatterns(ls.GroupPrefix); err != nil {
		return nil, err
	}
	if m.dbPatterns, err = compilePatterns(ls.DBCalls); err != nil {
		return nil, err
	}

	return m, nil
}

// Match runs all patterns against a file and returns extracted endpoints and DB calls.
type epKey struct {
	line   int
	method string
	path   string
}

type dbKey struct {
	line    int
	library string
	kind    string
}

func (m *Matcher) Match(f File) ([]Endpoint, []DBCall) {
	prefixStack := newPrefixStack(m.cfg.MaxDepth)
	var currentHandler string

	var endpoints []Endpoint
	var dbCalls []DBCall

	seenEp := map[epKey]bool{}
	seenDB := map[dbKey]bool{}

	for i, line := range f.Lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Skip commented lines
		switch f.Language {
		case LangPython:
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
		case LangGo:
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
		}

		prefixStack.update(trimmed)

		if prefix, ok := m.matchGroupPrefix(trimmed); ok {
			prefixStack.push(prefix)
			continue
		}

		if handler, ok := extractFunctionName(trimmed, f.Language); ok {
			currentHandler = handler
		}

		endpoints = m.matchEndpoints(trimmed, lineNum, currentHandler, prefixStack, seenEp, endpoints)
		dbCalls = m.matchDBCalls(trimmed, lineNum, currentHandler, seenDB, dbCalls)
	}

	return endpoints, dbCalls
}

func (m *Matcher) matchEndpoints(
	trimmed string,
	lineNum int,
	handler string,
	prefixStack *prefixStack,
	seen map[epKey]bool,
	endpoints []Endpoint,
) []Endpoint {
	for _, pat := range m.routerPatterns {
		matches := pat.re.FindStringSubmatch(trimmed)
		if matches == nil {
			continue
		}

		method, path := extractMethodPath(matches, pat.src)
		if path == "" {
			continue
		}

		if pat.src.Name == "flask_route" {
			if methods := extractFlaskMethods(trimmed); methods != "" {
				method = methods
			}
		}

		key := epKey{lineNum, method, path}
		if seen[key] {
			continue
		}
		seen[key] = true

		ep := Endpoint{
			Method:   method,
			Path:     path,
			FullPath: prefixStack.resolve(path),
		}
		if m.ctxCfg.IncludeLineNumbers {
			ep.Line = lineNum
		}
		if m.ctxCfg.IncludeHandler {
			ep.Handler = handler
		}
		if m.ctxCfg.IncludeRawLine {
			ep.RawLine = trimmed
		}

		endpoints = append(endpoints, ep)
	}

	return endpoints
}

func (m *Matcher) matchDBCalls(
	trimmed string,
	lineNum int,
	handler string,
	seen map[dbKey]bool,
	dbCalls []DBCall,
) []DBCall {
	for _, pat := range m.dbPatterns {
		matches := pat.re.FindStringSubmatch(trimmed)
		if matches == nil {
			continue
		}

		key := dbKey{lineNum, pat.src.Library, pat.src.Kind}
		if seen[key] {
			continue
		}
		seen[key] = true

		db := DBCall{
			Library: pat.src.Library,
		}
		if m.ctxCfg.IncludeDBKind {
			db.Kind = pat.src.Kind
		}
		if len(matches) > 1 && matches[1] != "" {
			db.Query = strings.TrimSpace(matches[1])
		}
		if m.ctxCfg.IncludeLineNumbers {
			db.Line = lineNum
		}
		if m.ctxCfg.IncludeHandler {
			db.Handler = handler
		}
		if m.ctxCfg.IncludeRawLine {
			db.RawLine = trimmed
		}

		dbCalls = append(dbCalls, db)
	}

	return dbCalls
}

// matchGroupPrefix checks if a line declares a route group or prefix.
func (m *Matcher) matchGroupPrefix(line string) (string, bool) {
	for _, cp := range m.groupPatterns {
		matches := cp.re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		groupIdx := cp.src.PathGroup
		if groupIdx > 0 && groupIdx < len(matches) {
			return matches[groupIdx], true
		}
		if len(matches) > 1 {
			return matches[1], true
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Prefix stack — bracket-aware prefix resolution
// ---------------------------------------------------------------------------

type prefixStack struct {
	prefixes   []string
	braceDepth []int // brace depth at which each prefix was pushed
	current    int   // current brace depth
	maxDepth   int
}

func newPrefixStack(maxDepth int) *prefixStack {
	if maxDepth == 0 {
		maxDepth = 50
	}
	return &prefixStack{maxDepth: maxDepth}
}

// update tracks brace open/close and pops prefixes when their scope closes.
func (ps *prefixStack) update(line string) {
	for _, ch := range line {
		switch ch {
		case '{':
			ps.current++
		case '}':
			ps.current--
			// Pop any prefixes that were pushed at this depth
			for len(ps.prefixes) > 0 && ps.braceDepth[len(ps.braceDepth)-1] >= ps.current {
				ps.prefixes = ps.prefixes[:len(ps.prefixes)-1]
				ps.braceDepth = ps.braceDepth[:len(ps.braceDepth)-1]
			}
		}
	}
}

// push adds a new prefix at the current brace depth.
func (ps *prefixStack) push(prefix string) {
	if len(ps.prefixes) >= ps.maxDepth {
		return
	}
	ps.prefixes = append(ps.prefixes, prefix)
	ps.braceDepth = append(ps.braceDepth, ps.current)
}

// resolve joins all active prefixes with the given path.
func (ps *prefixStack) resolve(path string) string {
	if len(ps.prefixes) == 0 {
		return path
	}
	base := strings.Join(ps.prefixes, "")
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// compilePatterns compiles a slice of Pattern into compiledPattern.
func compilePatterns(patterns []Pattern) ([]compiledPattern, error) {
	compiled := make([]compiledPattern, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			return nil, &PatternError{Name: p.Name, Err: err}
		}
		compiled = append(compiled, compiledPattern{re: re, src: p})
	}
	return compiled, nil
}

// extractMethodPath pulls method and path from regex matches using group indices.
func extractMethodPath(matches []string, p Pattern) (method, path string) {
	method = strings.ToUpper(p.Method)

	if p.MethodGroup > 0 && p.MethodGroup < len(matches) {
		method = strings.ToUpper(matches[p.MethodGroup])
	}
	if p.PathGroup > 0 && p.PathGroup < len(matches) {
		path = matches[p.PathGroup]
	} else if len(matches) > 1 {
		path = matches[1]
	}

	if method == "" {
		method = "ANY"
	}
	return
}

// extractFlaskMethods extracts the methods list from a Flask @app.route line.
// e.g. methods=["GET", "POST"] → "GET,POST"
var flaskMethodsRe = regexp.MustCompile(`methods\s*=\s*\[([^\]]+)\]`)

func extractFlaskMethods(line string) string {
	m := flaskMethodsRe.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	// Clean up: remove quotes and spaces
	raw := strings.ReplaceAll(m[1], `"`, "")
	raw = strings.ReplaceAll(raw, `'`, "")
	parts := strings.Split(raw, ",")
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, strings.ToUpper(p))
		}
	}
	return strings.Join(cleaned, ",")
}

// extractFunctionName detects a function definition and returns the name.
var goFuncRe = regexp.MustCompile(`^func\s+(?:\(\w+\s+\*?\w+\)\s+)?(\w+)\s*\(`)
var pyFuncRe = regexp.MustCompile(`^def\s+(\w+)\s*\(`)

func extractFunctionName(line string, lang Language) (string, bool) {
	switch lang {
	case LangGo:
		if m := goFuncRe.FindStringSubmatch(line); m != nil {
			return m[1], true
		}
	case LangPython:
		if m := pyFuncRe.FindStringSubmatch(line); m != nil {
			return m[1], true
		}
	}
	return "", false
}

// PatternError is returned when a pattern fails to compile.
type PatternError struct {
	Name string
	Err  error
}

func (e *PatternError) Error() string {
	return "pattern " + e.Name + ": " + e.Err.Error()
}
