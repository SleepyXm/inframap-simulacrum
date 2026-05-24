package walker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const WalkerFileName = "walkerfile.yml"

// LoadWalkerFile reads and parses a walkerfile.yml from the given directory.
// If dir is empty, it looks in the current working directory.
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

// DefaultWalkerFile returns a sensible default config when no walkerfile.yml exists.
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

// ---------------------------------------------------------------------------
// Struct loader
// ---------------------------------------------------------------------------

// LoadStructs loads all LanguageStruct definitions from the structs/ directory.
// If the walkerfile specifies "auto", all .yml files in structs/ are loaded.
// If it specifies a list, only those named structs are loaded.
func LoadStructs(structsDir string, wf *WalkerFile) (map[Language]*LanguageStruct, error) {
	entries, err := filepath.Glob(filepath.Join(structsDir, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("reading structs dir: %w", err)
	}

	// Determine which structs to load
	allowed := map[string]bool{}
	if wf.Structs != "auto" {
		if names, ok := wf.Structs.([]interface{}); ok {
			for _, n := range names {
				if s, ok := n.(string); ok {
					allowed[s] = true
				}
			}
		}
	}

	result := map[Language]*LanguageStruct{}

	for _, entry := range entries {
		name := strings.TrimSuffix(filepath.Base(entry), ".yml")

		// Skip if explicit list provided and this isn't in it
		if len(allowed) > 0 && !allowed[name] {
			continue
		}

		ls, err := loadStructFile(entry)
		if err != nil {
			return nil, fmt.Errorf("loading struct %q: %w", entry, err)
		}

		// Merge any custom patterns from walkerfile
		mergeCustomPatterns(ls, wf.Custom)

		result[ls.Language] = ls
	}

	return result, nil
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

// mergeCustomPatterns appends any custom patterns from walkerfile into the struct.
func mergeCustomPatterns(ls *LanguageStruct, custom CustomPatterns) {
	ls.RouterRegistration = append(ls.RouterRegistration, custom.RouterRegistration...)
	ls.GroupPrefix = append(ls.GroupPrefix, custom.GroupPrefix...)
	ls.DBCalls = append(ls.DBCalls, custom.DBCalls...)
	ls.Models = append(ls.Models, custom.Models...)
}
