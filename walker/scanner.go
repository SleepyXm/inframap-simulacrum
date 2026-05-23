package walker

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// File represents a source file ready for pattern matching.
type File struct {
	Path     string
	Language Language
	Lines    []string
}

// Scanner walks a directory tree and returns source files to scan.
type Scanner struct {
	cfg     ScannerConfig
	structs map[Language]*LanguageStruct
}

// New creates a Scanner with the given config and loaded structs.
func New(cfg ScannerConfig, structs map[Language]*LanguageStruct) *Scanner {
	return &Scanner{cfg: cfg, structs: structs}
}

// Walk traverses root and returns all scannable source files.
func (s *Scanner) Walk(root string) ([]File, error) {
	var files []File

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks unless configured to follow
		if !s.cfg.FollowSymlinks && d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() {
			if s.shouldSkipDir(path, d.Name()) {
				return filepath.SkipDir
			}
			// Enforce max depth
			if s.cfg.MaxDepth > 0 {
				rel, err := filepath.Rel(root, path)
				if err == nil {
					depth := len(strings.Split(rel, string(os.PathSeparator)))
					if depth > s.cfg.MaxDepth {
						return filepath.SkipDir
					}
				}
			}
			return nil
		}

		// Detect language by extension
		ext := strings.ToLower(filepath.Ext(path))
		lang, ok := ExtensionLanguage[ext]
		if !ok {
			return nil // not a language we handle
		}

		// Apply include_only filter if set
		if len(s.cfg.IncludeOnly) > 0 && !s.matchesAny(d.Name(), s.cfg.IncludeOnly) {
			return nil
		}

		// Apply per-language file skip rules
		if ls, ok := s.structs[lang]; ok {
			if s.matchesAny(d.Name(), ls.Skip.Files) {
				return nil
			}
		}

		// Apply global file skip rules from walkerfile
		if s.matchesAny(d.Name(), s.cfg.SkipFiles) {
			return nil
		}

		lines, err := readLines(path)
		if err != nil {
			return nil // skip unreadable files gracefully
		}

		files = append(files, File{
			Path:     path,
			Language: lang,
			Lines:    lines,
		})

		return nil
	})

	return files, err
}

// shouldSkipDir returns true if a directory should be skipped entirely.
func (s *Scanner) shouldSkipDir(path, name string) bool {
	// Global skip dirs from walkerfile
	for _, skip := range s.cfg.SkipDirs {
		if name == skip {
			return true
		}
	}

	// Per-language skip dirs from struct files
	for _, ls := range s.structs {
		for _, skip := range ls.Skip.Dirs {
			if name == skip {
				return true
			}
		}
	}

	return false
}

// matchesAny returns true if name matches any of the given glob patterns.
func (s *Scanner) matchesAny(name string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, name)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// readLines reads a file and returns its lines.
func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}
