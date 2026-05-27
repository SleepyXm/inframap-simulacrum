package walker

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const WalkerFileName = "walkerfile.yml"

func LoadWalkerFile(dir string) (*WalkerFile, error) {
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

	var wf WalkerFile
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parsing walkerfile: %w", err)
	}

	return &wf, nil
}

func DefaultWalkerFile() *WalkerFile {
	return &WalkerFile{
		Path:    ".",
		Structs: "auto",
		Output: OutputConfig{
			JSON: JSONOutputConfig{
				Path:   ".walker/context.json",
				Pretty: true,
			},
			YAML: YAMLOutputConfig{
				Path: ".walker/context.yml",
			},
		},
		Scanner: ScannerConfig{
			FollowSymlinks: false,
			MaxDepth:       0,
			SkipDirs:       []string{".walker", ".git", "vendor", "node_modules"},
		},
		Bracket: BracketConfig{
			MaxDepth:           10,
			CrossFunctionScope: true,
		},
		Context: ContextConfig{
			IncludeLineNumbers: true,
			IncludeHandler:     true,
			IncludeDBKind:      true,
			GroupByFile:        true,
		},
	}
}

// LoadStructsFromDir loads all LanguageStruct definitions from a directory.
// Filtering by walkerfile allowlist is a separate concern — see ApplyCustomPatterns.
func LoadStructsFromDir(structsDir string) (map[Language]*LanguageStruct, error) {
	entries, err := filepath.Glob(filepath.Join(structsDir, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("reading structs dir: %w", err)
	}

	result := map[Language]*LanguageStruct{}
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
func LoadStructsFiltered(structsDir string, wf *WalkerFile) (map[Language]*LanguageStruct, error) {
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

	filtered := map[Language]*LanguageStruct{}
	for lang, ls := range all {
		if allowed[string(lang)] {
			filtered[lang] = ls
		}
	}
	return filtered, nil
}

// ApplyCustomPatterns merges walkerfile custom patterns into loaded structs.
func ApplyCustomPatterns(structs map[Language]*LanguageStruct, custom CustomPatterns) {
	for _, ls := range structs {
		ls.RouterRegistration = append(ls.RouterRegistration, custom.RouterRegistration...)
		ls.GroupPrefix = append(ls.GroupPrefix, custom.GroupPrefix...)
		ls.DBCalls = append(ls.DBCalls, custom.DBCalls...)
		ls.Models = append(ls.Models, custom.Models...)
	}
}

func loadStructFile(path string) (*LanguageStruct, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ls LanguageStruct
	if err := yaml.Unmarshal(data, &ls); err != nil {
		return nil, fmt.Errorf("parsing struct file %q: %w", path, err)
	}
	return &ls, nil
}

func init() {
	// Ensure the map is populated even if loader.go is the only file imported.
	_ = ExtensionLanguage
}
