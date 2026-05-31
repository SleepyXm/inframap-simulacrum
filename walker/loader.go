package walker

import (
	"db-seeder/walker/types"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const WalkerFileName = "walkerfile.yml"

func LoadWalkerFile(dir string) (*types.WalkerFile, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("could not determine working directory: %w", err)
		}
	}

	path := filepath.Join(dir, WalkerFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultWalkerFile(), nil
		}
		return nil, fmt.Errorf("reading walkerfile: %w", err)
	}

	var wf types.WalkerFile
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parsing walkerfile: %w", err)
	}

	return &wf, nil
}

func DefaultWalkerFile() *types.WalkerFile {
	return &types.WalkerFile{
		Path:    ".",
		Structs: "auto",
		Output: types.OutputConfig{
			JSON: types.JSONOutputConfig{
				Path:   ".walker/context.json",
				Pretty: true,
			},
			YAML: types.YAMLOutputConfig{
				Path: ".walker/context.yml",
			},
		},
		Scanner: types.ScannerConfig{
			FollowSymlinks: false,
			MaxDepth:       0,
			SkipDirs:       []string{".walker", ".git", "vendor", "node_modules"},
		},
		Bracket: types.BracketConfig{
			MaxDepth:           10,
			CrossFunctionScope: true,
		},
		Context: types.ContextConfig{
			IncludeLineNumbers: true,
			IncludeHandler:     true,
			IncludeDBKind:      true,
			GroupByFile:        true,
		},
	}
}

// LoadStructsFromDir loads all LanguageStruct definitions from a directory.
// Filtering by walkerfile allowlist is a separate concern — see ApplyCustomPatterns.
func LoadStructsFromDir(structsDir string) (map[types.Language]*types.LanguageStruct, error) {
	entries, err := filepath.Glob(filepath.Join(structsDir, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("reading structs dir: %w", err)
	}

	result := map[types.Language]*types.LanguageStruct{}
	for _, entry := range entries {
		ls, err := loadStructFile(entry)
		if err != nil {
			return nil, fmt.Errorf("loading struct %q: %w", entry, err)
		}
		result[ls.Language] = ls
	}
	return result, nil
}

// LoadStructsFiltered loads only the structs named in the walkerfile allowlist.
// Pass wf.Structs == "auto" to load all.
func LoadStructsFiltered(structsDir string, wf *types.WalkerFile) (map[types.Language]*types.LanguageStruct, error) {
	all, err := LoadStructsFromDir(structsDir)
	if err != nil {
		return nil, err
	}

	if wf.Structs == "auto" {
		return all, nil
	}

	allowed := map[string]bool{}
	if names, ok := wf.Structs.([]interface{}); ok {
		for _, n := range names {
			if s, ok := n.(string); ok {
				allowed[s] = true
			}
		}
	}

	if len(allowed) == 0 {
		return all, nil
	}

	filtered := map[types.Language]*types.LanguageStruct{}
	for lang, ls := range all {
		if allowed[string(lang)] {
			filtered[lang] = ls
		}
	}
	return filtered, nil
}

// ApplyCustomPatterns merges walkerfile custom patterns into loaded structs.
func ApplyCustomPatterns(structs map[types.Language]*types.LanguageStruct, custom types.CustomPatterns) {
	for _, ls := range structs {
		ls.RouterRegistration = append(ls.RouterRegistration, custom.RouterRegistration...)
		ls.GroupPrefix = append(ls.GroupPrefix, custom.GroupPrefix...)
		ls.DBCalls = append(ls.DBCalls, custom.DBCalls...)
		ls.Models = append(ls.Models, custom.Models...)
	}
}

func loadStructFile(path string) (*types.LanguageStruct, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ls types.LanguageStruct
	if err := yaml.Unmarshal(data, &ls); err != nil {
		return nil, fmt.Errorf("parsing struct file %q: %w", path, err)
	}
	return &ls, nil
}

func init() {
	// Ensure the map is populated even if loader.go is the only file imported.
	_ = types.ExtensionLanguage
}
