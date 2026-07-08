package walker

import (
	"db-seeder/tools"
	"db-seeder/walker/types"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// WalkerTool implements tools.Tool.
type WalkerTool struct {
	wf    *types.WalkerFile
	ready bool
}

func NewTool(dir string) (*WalkerTool, error) {
	wf, err := LoadWalkerFile(dir)
	if err != nil {
		return nil, fmt.Errorf("walker: loading walkerfile: %w", err)
	}

	return &WalkerTool{
		wf:    wf,
		ready: true,
	}, nil
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
	scanPath = strings.TrimSpace(scanPath)
	if scanPath == "" {
		scanPath = t.wf.Path
	}

	captured, err := Capture(CaptureOptions{
		ConfigPath: t.wf.Capture.ConfigPath,
		RulesDir:   t.wf.Capture.RulesDir,
		TargetDir:  scanPath,
	})
	if err != nil {
		return tools.ToolResult{
			Err: fmt.Errorf("walker capture failed: %w", err),
		}
	}

	if err := ValidateCaptureResult(captured); err != nil {
		return tools.ToolResult{
			Err: fmt.Errorf("invalid walker output: %w", err),
		}
	}

	context := BuildProjectContext(captured)
	if err := ValidateProjectContext(&context); err != nil {
		return tools.ToolResult{
			Err: fmt.Errorf("invalid walker context: %w", err),
		}
	}

	if err := WriteJSON(captured, t.wf.Output.JSON); err != nil {
		return tools.ToolResult{Err: err}
	}

	if err := WriteYAML(captured, t.wf.Output.YAML); err != nil {
		return tools.ToolResult{Err: err}
	}

	if err := WriteJSON(context, ContextJSONOutput); err != nil {
		return tools.ToolResult{Err: err}
	}

	if err := WriteYAML(context, ContextYAMLOutput); err != nil {
		return tools.ToolResult{Err: err}
	}

	return tools.ToolResult{
		Summary: summariseCaptureToLines(captured),
		Outputs: []string{
			t.wf.Output.JSON.Path,
			t.wf.Output.YAML.Path,
			ContextJSONOutput.Path,
			ContextYAMLOutput.Path,
		},
	}
}
