package walker

import (
	"db-seeder/walker/types"
	"regexp"
	"strings"
)

// compiledPattern holds a compiled regex alongside its source Pattern metadata.
type compiledPattern struct {
	re  *regexp.Regexp
	src types.Pattern
}

// Matcher applies language struct patterns against a file's lines.
type Matcher struct {
	ls             *types.LanguageStruct
	cfg            types.BracketConfig
	ctxCfg         types.ContextConfig
	routerPatterns []compiledPattern
	groupPatterns  []compiledPattern
	dbPatterns     []compiledPattern
	modelPatterns  []compiledPattern
}

// NewMatcher compiles all patterns from a types.LanguageStruct.
// Returns an error if any pattern fails to compile.
func NewMatcher(ls *types.LanguageStruct, bracketCfg types.BracketConfig, ctxCfg types.ContextConfig) (*Matcher, error) {
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
	if m.modelPatterns, err = compilePatterns(ls.Models); err != nil {
		return nil, err
	}

	return m, nil
}

// Match runs all patterns against a file and returns extracted endpoints, DB calls, and models.
// inheritedPrefix is prepended to all resolved endpoint paths — used when a file's router
// registration function is called with a pre-grouped router (e.g. api.Group("/auth")).
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

func (m *Matcher) Match(f File, inheritedPrefix string) ([]types.Endpoint, []types.DBCall, []types.ModelDef) {
	prefixStack := newPrefixStack(m.cfg.MaxDepth)
	if inheritedPrefix != "" {
		// Seed the stack with the inherited prefix at depth 0 so it's never popped.
		prefixStack.pushPermanent(inheritedPrefix)
	}

	// groupVars tracks variables that hold a router group, e.g.:
	//   api := router.Group("/api")  →  groupVars["api"] = "/api"
	// These are resolved relative to whatever is already on the prefix stack.
	groupVars := map[string]string{}

	var currentHandler string

	var endpoints []types.Endpoint
	var dbCalls []types.DBCall
	var models []types.ModelDef

	seenModel := map[struct {
		line int
		name string
	}]bool{}
	seenEp := map[epKey]bool{}
	seenDB := map[dbKey]bool{}

	for i, line := range f.Lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Skip commented lines
		switch f.Language {
		case types.LangPython:
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
		case types.LangGo:
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
		}

		prefixStack.update(trimmed)

		// Check for group variable assignment: api := router.Group("/api")
		if varName, prefix, ok := extractGroupVarAssignment(trimmed, groupVars); ok {
			groupVars[varName] = prefixStack.resolve(prefix)
			continue
		}

		// Check for inline group prefix: r.Group("/auth", handler) or
		// a named-var group call: api.Group("/users")
		if prefix, ok := m.matchGroupPrefix(trimmed, groupVars); ok {
			prefixStack.push(prefix)
			continue
		}

		if handler, ok := extractFunctionName(trimmed, f.Language); ok {
			currentHandler = handler
		}

		endpoints = m.matchEndpoints(trimmed, lineNum, currentHandler, prefixStack, seenEp, endpoints)
		dbCalls = m.matchDBCalls(trimmed, lineNum, currentHandler, seenDB, dbCalls)
		models = m.matchModels(trimmed, lineNum, seenModel, models)
	}

	return endpoints, dbCalls, models
}

func (m *Matcher) matchEndpoints(
	trimmed string,
	lineNum int,
	handler string,
	prefixStack *prefixStack,
	seen map[epKey]bool,
	endpoints []types.Endpoint,
) []types.Endpoint {
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

		ep := types.Endpoint{
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
	dbCalls []types.DBCall,
) []types.DBCall {
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

		db := types.DBCall{
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
// It now also resolves groups formed from tracked group variables, e.g. api.Group("/auth").
func (m *Matcher) matchGroupPrefix(line string, groupVars map[string]string) (string, bool) {
	for _, cp := range m.groupPatterns {
		matches := cp.re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		groupIdx := cp.src.PathGroup
		var rawPath string
		if groupIdx > 0 && groupIdx < len(matches) {
			rawPath = matches[groupIdx]
		} else if len(matches) > 1 {
			rawPath = matches[1]
		}
		if rawPath == "" {
			continue
		}

		// Check if the receiver is a known group variable so we can chain it.
		// e.g. line = `api.Group("/auth", ...)` and groupVars["api"] = "/api"
		// → resolved prefix = "/api/auth"
		if receiver, ok := extractGroupReceiver(line); ok {
			if base, known := groupVars[receiver]; known {
				return joinPaths(base, rawPath), true
			}
		}

		return rawPath, true
	}
	return "", false
}

func (m *Matcher) matchModels(
	trimmed string,
	lineNum int,
	seen map[struct {
		line int
		name string
	}]bool,
	models []types.ModelDef,
) []types.ModelDef {
	for _, pat := range m.modelPatterns {
		matches := pat.re.FindStringSubmatch(trimmed)
		if matches == nil {
			continue
		}
		name := matches[1]
		key := struct {
			line int
			name string
		}{lineNum, name}
		if seen[key] {
			continue
		}
		seen[key] = true
		models = append(models, types.ModelDef{
			Name: name,
			Kind: pat.src.Kind,
			Line: lineNum,
		})
	}
	return models
}

// ---------------------------------------------------------------------------
// Prefix stack — bracket-aware prefix resolution
// ---------------------------------------------------------------------------

type prefixEntry struct {
	prefix     string
	braceDepth int
	permanent  bool // never popped (used for inherited prefixes)
}

type prefixStack struct {
	entries  []prefixEntry
	current  int // current brace depth
	maxDepth int
}

func newPrefixStack(maxDepth int) *prefixStack {
	if maxDepth == 0 {
		maxDepth = 50
	}
	return &prefixStack{maxDepth: maxDepth}
}

// pushPermanent adds a prefix that is never popped by brace tracking.
// Used to seed an inherited cross-file prefix.
func (ps *prefixStack) pushPermanent(prefix string) {
	ps.entries = append(ps.entries, prefixEntry{
		prefix:     prefix,
		braceDepth: -1,
		permanent:  true,
	})
}

// update tracks brace open/close and pops non-permanent prefixes when their scope closes.
func (ps *prefixStack) update(line string) {
	for _, ch := range line {
		switch ch {
		case '{':
			ps.current++
		case '}':
			ps.current--
			for len(ps.entries) > 0 {
				top := ps.entries[len(ps.entries)-1]
				if top.permanent {
					break
				}
				if top.braceDepth > ps.current {
					ps.entries = ps.entries[:len(ps.entries)-1]
				} else {
					break
				}
			}
		}
	}
}

// push adds a new prefix at the current brace depth.
func (ps *prefixStack) push(prefix string) {
	if len(ps.entries) >= ps.maxDepth {
		return
	}
	ps.entries = append(ps.entries, prefixEntry{
		prefix:     prefix,
		braceDepth: ps.current,
	})
}

// resolve joins all active prefixes with the given path.
func (ps *prefixStack) resolve(path string) string {
	if len(ps.entries) == 0 {
		return path
	}
	parts := make([]string, 0, len(ps.entries)+1)
	for _, e := range ps.entries {
		parts = append(parts, e.prefix)
	}
	parts = append(parts, path)
	return joinPathSegments(parts)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// compilePatterns compiles a slice of Pattern into compiledPattern.
func compilePatterns(patterns []types.Pattern) ([]compiledPattern, error) {
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
func extractMethodPath(matches []string, p types.Pattern) (method, path string) {
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

func extractFunctionName(line string, lang types.Language) (string, bool) {
	switch lang {
	case types.LangGo:
		if m := goFuncRe.FindStringSubmatch(line); m != nil {
			return m[1], true
		}
	case types.LangPython:
		if m := pyFuncRe.FindStringSubmatch(line); m != nil {
			return m[1], true
		}
	}
	return "", false
}

// extractGroupVarAssignment detects lines like:
//
//	api := router.Group("/api")
//	v1 := api.Group("/v1")       ← chained; base looked up from groupVars
//
// Returns (varName, rawPath, true) when matched.
// rawPath is the literal string from the call; the caller resolves it against
// the current prefix stack or the groupVars map.
var groupVarAssignRe = regexp.MustCompile(
	`^(\w+)\s*:?=\s*\w+\.Group\(\s*["` + "`" + `]([^"` + "`" + `]+)["` + "`" + `]`,
)

func extractGroupVarAssignment(line string, groupVars map[string]string) (varName, prefix string, ok bool) {
	m := groupVarAssignRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	varName = m[1]
	rawPath := m[2]

	// Check if the RHS receiver is itself a known group var: v1 := api.Group("/v1")
	// The regex captures the receiver as the identifier before .Group(
	receiverRe := regexp.MustCompile(`^\w+\s*:?=\s*(\w+)\.Group\(`)
	rm := receiverRe.FindStringSubmatch(line)
	if rm != nil {
		if base, known := groupVars[rm[1]]; known {
			return varName, joinPaths(base, rawPath), true
		}
	}

	return varName, rawPath, true
}

// extractGroupReceiver returns the variable name that .Group() is called on.
// e.g. "api.Group("/auth")" → "api", true
var groupReceiverRe = regexp.MustCompile(`^(\w+)\.Group\(`)

func extractGroupReceiver(line string) (string, bool) {
	m := groupReceiverRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// joinPaths joins two path segments, normalising duplicate slashes.
func joinPaths(base, rel string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(rel, "/")
}

// joinPathSegments joins multiple path segments cleanly.
func joinPathSegments(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result = joinPaths(result, p)
	}
	return result
}

// PatternError is returned when a pattern fails to compile.
type PatternError struct {
	Name string
	Err  error
}

func (e *PatternError) Error() string {
	return "pattern " + e.Name + ": " + e.Err.Error()
}
