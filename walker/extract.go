package walker

import (
	"db-seeder/walker/types"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Function name extraction
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Method + path extraction
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Flask methods kwarg
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Group variable assignment
// e.g. api := router.Group("/api")
//      v1  := api.Group("/v1")
// ---------------------------------------------------------------------------

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

	receiverRe := regexp.MustCompile(`^\w+\s*:?=\s*(\w+)\.Group\(`)
	if rm := receiverRe.FindStringSubmatch(line); rm != nil {
		if base, known := groupVars[rm[1]]; known {
			return varName, joinPaths(base, rawPath), true
		}
	}

	return varName, rawPath, true
}

// ---------------------------------------------------------------------------
// Group receiver
// e.g. api.Group("/auth") → "api"
// ---------------------------------------------------------------------------

var groupReceiverRe = regexp.MustCompile(`^(\w+)\.Group\(`)

func extractGroupReceiver(line string) (string, bool) {
	m := groupReceiverRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// ---------------------------------------------------------------------------
// Path helpers
// ---------------------------------------------------------------------------

func joinPaths(base, rel string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(rel, "/")
}

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

// ---------------------------------------------------------------------------
// Pattern compilation
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

type PatternError struct {
	Name string
	Err  error
}

func (e *PatternError) Error() string {
	return "pattern " + e.Name + ": " + e.Err.Error()
}
