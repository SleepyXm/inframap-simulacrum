package walker

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"db-seeder/walker/core/worker"
	"db-seeder/walker/types"
)

var (
	ContextJSONOutput = types.JSONOutputConfig{
		Path:   ".walker/context.json",
		Pretty: true,
	}
	ContextYAMLOutput = types.YAMLOutputConfig{
		Path: ".walker/context.yml",
	}
)

func BuildProjectContext(result *CaptureResult) types.ProjectContext {
	snapshots := snapshotsFromCapture(result)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Path < snapshots[j].Path
	})

	files := make([]types.FileContext, 0, len(snapshots))
	for _, snap := range snapshots {
		language := languageForPath(snap.Path)
		file := types.FileContext{
			Path:      snap.Path,
			Language:  language,
			Endpoints: endpointsFromRoutes(snap.Routes, snap.Functions),
			Models:    modelsFromClasses(snap.Classes, language),
		}
		files = append(files, file)
	}

	return types.ProjectContext{Files: files}
}

func ValidateProjectContext(ctx *types.ProjectContext) error {
	if ctx == nil {
		return errors.New("project context is nil")
	}
	if len(ctx.Files) == 0 {
		return errors.New("project context contains no files")
	}

	var problems []string
	seenFiles := map[string]bool{}
	for fileIndex, file := range ctx.Files {
		if strings.TrimSpace(file.Path) == "" {
			problems = append(problems, fmt.Sprintf("files[%d] has empty path", fileIndex))
			continue
		}
		if seenFiles[file.Path] {
			problems = append(problems, fmt.Sprintf("duplicate context file: %s", file.Path))
		}
		seenFiles[file.Path] = true

		for endpointIndex, endpoint := range file.Endpoints {
			if strings.TrimSpace(endpoint.Method) == "" {
				problems = append(problems, fmt.Sprintf("files[%d].endpoints[%d] has empty method", fileIndex, endpointIndex))
			}
			if strings.TrimSpace(endpoint.Path) == "" {
				problems = append(problems, fmt.Sprintf("files[%d].endpoints[%d] has empty path", fileIndex, endpointIndex))
			}
		}
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}

	return nil
}

func snapshotsFromCapture(result *CaptureResult) []worker.FileSnapshot {
	if result == nil {
		return nil
	}
	if len(result.Snapshots) > 0 {
		out := make([]worker.FileSnapshot, len(result.Snapshots))
		copy(out, result.Snapshots)
		return out
	}

	var out []worker.FileSnapshot
	for _, group := range result.Groups {
		out = append(out, group.Files...)
	}
	return out
}

func endpointsFromRoutes(routes []types.Primitive, functions []types.FunctionDef) []types.Endpoint {
	endpoints := make([]types.Endpoint, 0, len(routes))
	for _, route := range routes {
		method := strings.ToUpper(strings.TrimSpace(route.Data["method"]))
		path := strings.TrimSpace(route.Data["path"])
		if method == "" || path == "" {
			continue
		}

		endpoints = append(endpoints, types.Endpoint{
			Method:   method,
			Path:     path,
			FullPath: path,
			Handler:  containingFunction(functions, route.StartLine),
			Line:     route.StartLine,
		})
	}

	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Line == endpoints[j].Line {
			if endpoints[i].Path == endpoints[j].Path {
				return endpoints[i].Method < endpoints[j].Method
			}
			return endpoints[i].Path < endpoints[j].Path
		}
		return endpoints[i].Line < endpoints[j].Line
	})

	return endpoints
}

func modelsFromClasses(classes []types.ClassDef, language types.Language) []types.ModelDef {
	models := make([]types.ModelDef, 0, len(classes))
	for _, class := range classes {
		models = append(models, types.ModelDef{
			Name:   class.Name,
			Kind:   modelKind(class, language),
			Line:   class.StartLine,
			Fields: fieldsFromClass(class),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		if models[i].Line == models[j].Line {
			return models[i].Name < models[j].Name
		}
		return models[i].Line < models[j].Line
	})

	return models
}

func fieldsFromClass(class types.ClassDef) map[string]string {
	if len(class.Fields) == 0 {
		return nil
	}

	fields := make(map[string]string, len(class.Fields))
	for _, field := range class.Fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			continue
		}
		fields[name] = strings.TrimSpace(field.Type)
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func modelKind(class types.ClassDef, language types.Language) string {
	for _, base := range class.Bases {
		normalized := strings.ToLower(strings.TrimSpace(base))
		switch {
		case normalized == "basemodel" || strings.Contains(normalized, "schema"):
			return "request_model"
		case normalized == "base" || strings.Contains(normalized, "model"):
			return "db_model"
		}
	}

	if language == types.LangGo {
		return "struct"
	}
	return "struct"
}

func containingFunction(functions []types.FunctionDef, line int) string {
	for _, fn := range functions {
		if line < fn.StartLine {
			continue
		}
		if fn.EndLine == 0 || line <= fn.EndLine {
			return fn.Name
		}
	}
	return ""
}

func languageForPath(path string) types.Language {
	switch filepath.Ext(path) {
	case ".go":
		return types.LangGo
	case ".py":
		return types.LangPython
	case ".js", ".ts", ".tsx", ".jsx":
		return types.Language("javascript")
	case ".rs":
		return types.Language("rust")
	case ".rb":
		return types.Language("ruby")
	case ".java":
		return types.Language("java")
	case ".c", ".h":
		return types.Language("c")
	case ".cpp", ".hpp":
		return types.Language("cpp")
	default:
		return types.LangUnknown
	}
}
