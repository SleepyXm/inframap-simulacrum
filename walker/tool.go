package walker

import (
	"db-seeder/tools"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// WalkerTool implements tools.Tool.
type WalkerTool struct {
	wf      *WalkerFile
	structs map[Language]*LanguageStruct
	ready   bool
}

func NewTool(dir string) (*WalkerTool, error) {
	wf, err := LoadWalkerFile(dir)
	if err != nil {
		return nil, fmt.Errorf("walker: loading walkerfile: %w", err)
	}
	structs, err := LoadStructsFromDir(filepath.Join(dir, "walker/patterns"))
	if err != nil {
		return nil, fmt.Errorf("walker: loading structs: %w", err)
	}
	if len(structs) == 0 {
		return nil, fmt.Errorf("walker: no language structs found")
	}
	ApplyCustomPatterns(structs, wf.Custom)
	return &WalkerTool{wf: wf, structs: structs, ready: true}, nil
}

func (t *WalkerTool) Name() string    { return "Walker" }
func (t *WalkerTool) Available() bool { return t.ready }
func (t *WalkerTool) Prompt() string  { return "./path/to/project" }

func (t *WalkerTool) Run(input string) tea.Cmd {
	return func() tea.Msg {
		result := t.run(input)
		return tools.ToolDoneMsg{Tool: t.Name(), Result: result}
	}
}

func (t *WalkerTool) run(scanPath string) tools.ToolResult {
	s := New(t.wf.Scanner, t.structs)
	files, err := s.Walk(scanPath)
	if err != nil {
		return tools.ToolResult{Err: fmt.Errorf("scan failed: %w", err)}
	}

	matchers := map[Language]*Matcher{}
	for lang, ls := range t.structs {
		m, err := NewMatcher(ls, t.wf.Bracket, t.wf.Context)
		if err != nil {
			return tools.ToolResult{Err: fmt.Errorf("compiling patterns for %s: %w", lang, err)}
		}
		matchers[lang] = m
	}

	ctx := &ProjectContext{}
	for _, f := range files {
		m, ok := matchers[f.Language]
		if !ok {
			continue
		}
		endpoints, dbCalls, models := m.Match(f)
		if len(endpoints) == 0 && len(dbCalls) == 0 && len(models) == 0 {
			continue
		}
		ctx.Files = append(ctx.Files, FileContext{
			Path:      f.Path,
			Language:  f.Language,
			Endpoints: endpoints,
			DBCalls:   dbCalls,
			Models:    models,
		})
	}

	if err := WriteJSON(ctx, t.wf.Output.JSON); err != nil {
		return tools.ToolResult{Err: err}
	}
	if err := WriteYAML(ctx, t.wf.Output.YAML); err != nil {
		return tools.ToolResult{Err: err}
	}

	return tools.ToolResult{
		Summary: summariseToLines(ctx),
		Outputs: []string{t.wf.Output.JSON.Path, t.wf.Output.YAML.Path},
	}
}

func summariseToLines(ctx *ProjectContext) []tools.SummaryLine {
	var lines []tools.SummaryLine

	langSet := map[string]bool{}
	libSet := map[string]bool{}
	kindCounts := map[string]int{}
	totalEp, totalDB, totalModels := 0, 0, 0

	for _, f := range ctx.Files {
		langSet[string(f.Language)] = true
		totalEp += len(f.Endpoints)
		for _, db := range f.DBCalls {
			totalDB++
			if db.Library != "" {
				libSet[db.Library] = true
			}
			if db.Kind != "" {
				kindCounts[db.Kind]++
			}
		}
		totalModels += len(f.Models)
	}

	lines = append(lines, tools.SummaryLine{Label: "files scanned", Value: fmt.Sprintf("%d", len(ctx.Files))})

	lines = append(lines, tools.SummaryLine{Label: "endpoints found", Value: fmt.Sprintf("%d", totalEp)})
	for _, f := range ctx.Files {
		for _, ep := range f.Endpoints {
			lines = append(lines, tools.SummaryLine{
				Value:  fmt.Sprintf("%-8s %s", ep.Method, ep.FullPath),
				Indent: true,
			})
		}
	}

	lines = append(lines, tools.SummaryLine{Label: "db calls found", Value: fmt.Sprintf("%d", totalDB)})
	for _, kind := range []string{"exec", "raw", "query", "exec_many", "copy", "cursor"} {
		if count, ok := kindCounts[kind]; ok {
			lines = append(lines, tools.SummaryLine{
				Value:  fmt.Sprintf("%-12s %d", kind, count),
				Indent: true,
			})
		}
	}

	lines = append(lines, tools.SummaryLine{Label: "models found", Value: fmt.Sprintf("%d", totalModels)})
	for _, f := range ctx.Files {
		for _, model := range f.Models {
			lines = append(lines, tools.SummaryLine{
				Value:  fmt.Sprintf("%-12s %s", model.Kind, model.Name),
				Indent: true,
			})
		}
	}

	if len(langSet) > 0 {
		langs := make([]string, 0, len(langSet))
		for k := range langSet {
			langs = append(langs, k)
		}
		lines = append(lines, tools.SummaryLine{Label: "languages", Value: strings.Join(langs, ", ")})
	}

	if len(libSet) > 0 {
		libs := make([]string, 0, len(libSet))
		for k := range libSet {
			libs = append(libs, k)
		}
		lines = append(lines, tools.SummaryLine{Label: "db libraries", Value: strings.Join(libs, ", ")})
	}

	return lines
}
