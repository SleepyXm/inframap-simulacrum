package walker

import (
	"db-seeder/walker/types"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Cross-file route prefix resolution
// ---------------------------------------------------------------------------
//
// The problem:
//
//   main.go (or routes.go):
//       api := router.Group("/api")
//       routes.RegisterAuthRoutes(api.Group("/auth"), db)
//
//   auth_routes.go:
//       func RegisterAuthRoutes(r *gin.RouterGroup, db *sql.DB) {
//           r.POST("/login", ...)    ← should resolve to /api/auth/login
//       }
//
// The Matcher handles same-file group variable tracking, but it cannot follow
// the prefix across a function call boundary into another file.
//
// The Resolver performs a two-pass scan:
//
//   Pass 1 — scan every file with an empty inherited prefix, collecting:
//     · normal endpoints / DB calls / models  (already correct for files that
//       don't receive a router group as a parameter)
//     · RouteRegistrations: calls like RegisterAuthRoutes(api.Group("/auth"), db)
//       where the first argument is a .Group(...) expression. The prefix is
//       resolved in the context of that file's group-var state.
//
//   Pass 2 — for each RouteRegistration, find the file that defines the named
//     function and re-run Match() on it with the resolved prefix injected.
//     The re-scan replaces (or supplements) that file's endpoints.

// Resolver wraps a set of matchers and files, running the two-pass scan.
type Resolver struct {
	matchers map[types.Language]*Matcher
	files    []File
	ctxCfg   types.ContextConfig
}

// NewResolver creates a Resolver.
func NewResolver(matchers map[types.Language]*Matcher, files []File, ctxCfg types.ContextConfig) *Resolver {
	return &Resolver{matchers: matchers, files: files, ctxCfg: ctxCfg}
}

// Resolve runs both passes and returns the final ProjectContext.
func (r *Resolver) Resolve() *types.ProjectContext {
	// Pass 1: scan all files, collect registrations.
	type pass1Result struct {
		fc            types.FileContext
		registrations []types.RouteRegistration
	}
	p1 := make([]pass1Result, 0, len(r.files))

	for _, f := range r.files {
		m, ok := r.matchers[f.Language]
		if !ok {
			p1 = append(p1, pass1Result{fc: types.FileContext{Path: f.Path, Language: f.Language}})
			continue
		}
		endpoints, dbCalls, models := m.Match(f, "")
		regs := extractRouteRegistrations(f)

		p1 = append(p1, pass1Result{
			fc: types.FileContext{
				Path:      f.Path,
				Language:  f.Language,
				Endpoints: endpoints,
				DBCalls:   dbCalls,
				Models:    models,
			},
			registrations: regs,
		})
	}

	// Build a lookup: funcName → File (the file that defines it).
	funcToFile := buildFuncIndex(r.files)

	// Pass 2: for each registration, re-scan the target file with the prefix.
	// Track which files were re-scanned so we merge, not duplicate.
	rescannedPrefixes := map[string][]string{} // filePath → []resolvedPrefixes already applied

	for _, res := range p1 {
		for _, reg := range res.registrations {
			targetFile, ok := funcToFile[reg.FuncName]
			if !ok {
				continue
			}
			// Avoid duplicate rescans with the same prefix.
			already := false
			for _, applied := range rescannedPrefixes[targetFile.Path] {
				if applied == reg.ResolvedPrefix {
					already = true
					break
				}
			}
			if already {
				continue
			}
			rescannedPrefixes[targetFile.Path] = append(
				rescannedPrefixes[targetFile.Path], reg.ResolvedPrefix,
			)
		}
	}

	// Build final FileContext list.
	// For files that need re-scanning, run Match again with each prefix and merge.
	ctx := &types.ProjectContext{}
	for _, res := range p1 {
		fc := res.fc
		prefixes, needsRescan := rescannedPrefixes[fc.Path]
		if !needsRescan {
			ctx.Files = append(ctx.Files, fc)
			continue
		}

		m, ok := r.matchers[fc.Language]
		if !ok {
			ctx.Files = append(ctx.Files, fc)
			continue
		}

		// Find the actual File struct for this path.
		var targetFile File
		for _, f := range r.files {
			if f.Path == fc.Path {
				targetFile = f
				break
			}
		}

		// Re-scan with each inherited prefix and collect endpoints.
		// Models and DB calls are prefix-agnostic so we keep the pass-1 results.
		var allEndpoints []types.Endpoint
		seenEp := map[epKey]bool{}

		for _, prefix := range prefixes {
			eps, _, _ := m.Match(targetFile, prefix)
			for _, ep := range eps {
				k := epKey{ep.Line, ep.Method, ep.FullPath}
				if !seenEp[k] {
					seenEp[k] = true
					allEndpoints = append(allEndpoints, ep)
				}
			}
		}

		fc.Endpoints = allEndpoints
		ctx.Files = append(ctx.Files, fc)
	}

	return ctx
}

// ---------------------------------------------------------------------------
// Route registration extraction (pass 1 side-channel)
// ---------------------------------------------------------------------------

// extractRouteRegistrations scans a file's lines for call sites like:
//
//	routes.RegisterAuthRoutes(api.Group("/auth"), db)
//	RegisterAuthRoutes(r.Group("/auth"), db)
//
// It resolves the prefix by replaying the same group-var logic used in Match,
// so that chained groups like api.Group("/auth") → /api/auth work correctly.
func extractRouteRegistrations(f File) []types.RouteRegistration {
	groupVars := map[string]string{}
	var regs []types.RouteRegistration

	for i, line := range f.Lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Skip comments
		switch f.Language {
		case types.LangGo:
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
		case types.LangPython:
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
		}

		// Track group variable assignments so we can resolve receivers.
		if varName, prefix, ok := extractGroupVarAssignment(trimmed, groupVars); ok {
			groupVars[varName] = prefix
			continue
		}

		// Match call sites where first arg is a .Group(...) call.
		if reg, ok := extractRegistrationCall(trimmed, lineNum, groupVars); ok {
			regs = append(regs, reg)
		}
	}
	return regs
}

// registrationCallRe matches:
//
//	[pkg.]FuncName( receiver.Group("path") , ...
//
// Groups: 1=FuncName  2=receiver  3=path
var registrationCallRe = regexp.MustCompile(
	`(?:\w+\.)?(\w+)\s*\(\s*(\w+)\.Group\(\s*["` + "`" + `]([^"` + "`" + `]+)["` + "`" + `]`,
)

func extractRegistrationCall(line string, lineNum int, groupVars map[string]string) (types.RouteRegistration, bool) {
	m := registrationCallRe.FindStringSubmatch(line)
	if m == nil {
		return types.RouteRegistration{}, false
	}
	funcName := m[1]
	receiver := m[2]
	rawPath := m[3]

	// Resolve the receiver against known group vars.
	resolved := rawPath
	if base, ok := groupVars[receiver]; ok {
		resolved = joinPaths(base, rawPath)
	}

	// Ignore lines that look like method calls on the router itself
	// (e.g. r.Group("/foo") without being passed to another function).
	// We only want calls where the Group() result is passed as an argument.
	if funcName == "Group" {
		return types.RouteRegistration{}, false
	}

	return types.RouteRegistration{
		FuncName:       funcName,
		ResolvedPrefix: resolved,
		Line:           lineNum,
	}, true
}

// ---------------------------------------------------------------------------
// Function index — maps func name → File
// ---------------------------------------------------------------------------

// buildFuncIndex scans all files and maps each top-level function name to the
// file that defines it.
func buildFuncIndex(files []File) map[string]File {
	index := map[string]File{}
	for _, f := range files {
		for _, line := range f.Lines {
			trimmed := strings.TrimSpace(line)
			if name, ok := extractFunctionName(trimmed, f.Language); ok {
				// Don't overwrite; first definition wins.
				if _, exists := index[name]; !exists {
					index[name] = f
				}
			}
		}
	}
	return index
}
