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
		prefixStack.pushPermanent(inheritedPrefix)
	}

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

		if isComment(trimmed, f.Language) {
			continue
		}

		prefixStack.update(trimmed)

		if varName, prefix, ok := extractGroupVarAssignment(trimmed, groupVars); ok {
			groupVars[varName] = prefixStack.resolve(prefix)
			continue
		}

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

// ---------------------------------------------------------------------------
// Match helpers
// ---------------------------------------------------------------------------

func (m *Matcher) matchEndpoints(trimmed string, lineNum int, handler string, prefixStack *prefixStack, seen map[epKey]bool, endpoints []types.Endpoint) []types.Endpoint {
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

func (m *Matcher) matchDBCalls(trimmed string, lineNum int, handler string, seen map[dbKey]bool, dbCalls []types.DBCall) []types.DBCall {
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
		db := types.DBCall{Library: pat.src.Library}
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
		if receiver, ok := extractGroupReceiver(line); ok {
			if base, known := groupVars[receiver]; known {
				return joinPaths(base, rawPath), true
			}
		}
		return rawPath, true
	}
	return "", false
}

func (m *Matcher) matchModels(trimmed string, lineNum int, seen map[struct {
	line int
	name string
}]bool, models []types.ModelDef) []types.ModelDef {
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
// Comment detection
// ---------------------------------------------------------------------------

func isComment(line string, lang types.Language) bool {
	switch lang {
	case types.LangGo:
		return strings.HasPrefix(line, "//")
	case types.LangPython:
		return strings.HasPrefix(line, "#")
	}
	return false
}

// ---------------------------------------------------------------------------
// Prefix stack
// ---------------------------------------------------------------------------

type prefixEntry struct {
	prefix     string
	braceDepth int
	permanent  bool
}

type prefixStack struct {
	entries  []prefixEntry
	current  int
	maxDepth int
}

func newPrefixStack(maxDepth int) *prefixStack {
	if maxDepth == 0 {
		maxDepth = 50
	}
	return &prefixStack{maxDepth: maxDepth}
}

func (ps *prefixStack) pushPermanent(prefix string) {
	ps.entries = append(ps.entries, prefixEntry{prefix: prefix, braceDepth: -1, permanent: true})
}

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

func (ps *prefixStack) push(prefix string) {
	if len(ps.entries) >= ps.maxDepth {
		return
	}
	ps.entries = append(ps.entries, prefixEntry{prefix: prefix, braceDepth: ps.current})
}

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
