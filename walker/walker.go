package walker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"
)

// Result is returned to the TUI once a walk completes.
type Result struct {
	TotalFiles     int
	TotalEndpoints int
	TotalDBCalls   int
	DBCallKinds    map[string]int // add this
	Endpoints      []ResultEndpoint
	Languages      []string
	DBLibraries    []string
	JSONPath       string
	YAMLPath       string
	Err            error
}

type ResultEndpoint struct {
	Method   string
	FullPath string
}

// WalkDoneMsg is the bubbletea message delivered on completion.
type WalkDoneMsg struct {
	Result Result
}

var (
	initialized bool
	wf          *WalkerFile
	structs     map[Language]*LanguageStruct
)

func WriteJSON(ctx *ProjectContext, cfg JSONOutputConfig) error {
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}
	var data []byte
	var err error
	if cfg.Pretty {
		data, err = json.MarshalIndent(ctx, "", "  ")
	} else {
		data, err = json.Marshal(ctx)
	}
	if err != nil {
		return fmt.Errorf("marshalling JSON: %w", err)
	}
	return os.WriteFile(cfg.Path, data, 0644)
}

func WriteYAML(ctx *ProjectContext, cfg YAMLOutputConfig) error {
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}
	data, err := yaml.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("marshalling YAML: %w", err)
	}
	return os.WriteFile(cfg.Path, data, 0644)
}

// Init loads walkerfile and structs. Non-fatal — walker is opt-in.
func Init() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving cwd: %w", err)
	}

	wf, err = LoadWalkerFile(cwd)
	if err != nil {
		return fmt.Errorf("loading walkerfile: %w", err)
	}

	structsDir := filepath.Join(cwd, "walker/patterns")
	structs, err = LoadStructs(structsDir, wf)
	if err != nil {
		return fmt.Errorf("loading structs: %w", err)
	}

	if len(structs) == 0 {
		return fmt.Errorf("no language structs found in %q", structsDir)
	}

	initialized = true
	return nil
}

// Available reports whether the walker was initialised successfully.
func Available() bool {
	return initialized
}

// RunCmd returns a bubbletea Cmd that runs the walker against the given path
// and delivers a WalkDoneMsg when complete.
func RunCmd(scanPath string) tea.Cmd {
	return func() tea.Msg {
		if !initialized {
			return WalkDoneMsg{Result: Result{Err: fmt.Errorf("walker not initialised")}}
		}

		wf.Path = scanPath

		// Build scanner
		s := New(wf.Scanner, structs)
		files, err := s.Walk(scanPath)
		if err != nil {
			return WalkDoneMsg{Result: Result{Err: fmt.Errorf("scan failed: %w", err)}}
		}

		// Build matchers
		matchers := map[Language]*Matcher{}
		for lang, ls := range structs {
			m, err := NewMatcher(ls, wf.Bracket, wf.Context)
			if err != nil {
				return WalkDoneMsg{Result: Result{Err: fmt.Errorf("compiling patterns for %s: %w", lang, err)}}
			}
			matchers[lang] = m
		}

		// Run matching
		ctx := &ProjectContext{}
		for _, f := range files {
			m, ok := matchers[f.Language]
			if !ok {
				continue
			}
			endpoints, dbCalls := m.Match(f)
			if len(endpoints) == 0 && len(dbCalls) == 0 {
				continue
			}
			ctx.Files = append(ctx.Files, FileContext{
				Path:      f.Path,
				Language:  f.Language,
				Endpoints: endpoints,
				DBCalls:   dbCalls,
			})
		}

		// Write outputs
		if err := WriteJSON(ctx, wf.Output.JSON); err != nil {
			return WalkDoneMsg{Result: Result{Err: err}}
		}
		if err := WriteYAML(ctx, wf.Output.YAML); err != nil {
			return WalkDoneMsg{Result: Result{Err: err}}
		}

		// Build summary for TUI
		res := summarise(ctx)
		res.JSONPath = wf.Output.JSON.Path
		res.YAMLPath = wf.Output.YAML.Path
		return WalkDoneMsg{Result: res}
	}
}

func summarise(ctx *ProjectContext) Result {
	langSet := map[string]bool{}
	libSet := map[string]bool{}
	kindCounts := map[string]int{}
	var endpoints []ResultEndpoint
	totalEp := 0
	totalDB := 0

	for _, f := range ctx.Files {
		langSet[string(f.Language)] = true
		totalEp += len(f.Endpoints)

		for _, ep := range f.Endpoints {
			endpoints = append(endpoints, ResultEndpoint{
				Method:   ep.Method,
				FullPath: ep.FullPath,
			})
		}

		for _, db := range f.DBCalls {
			totalDB++
			if db.Library != "" {
				libSet[db.Library] = true
			}
			if db.Kind != "" {
				kindCounts[db.Kind]++
			}
		}
	}

	langs := make([]string, 0, len(langSet))
	for k := range langSet {
		langs = append(langs, k)
	}
	libs := make([]string, 0, len(libSet))
	for k := range libSet {
		libs = append(libs, k)
	}

	return Result{
		TotalFiles:     len(ctx.Files),
		TotalEndpoints: totalEp,
		TotalDBCalls:   totalDB,
		DBCallKinds:    kindCounts,
		Endpoints:      endpoints,
		Languages:      langs,
		DBLibraries:    libs,
	}
}
