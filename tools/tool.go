package tools

import tea "github.com/charmbracelet/bubbletea"

// Tool is anything the TUI can run asynchronously.
type Tool interface {
	Name() string             // "Walker", "Simulation", etc.
	Available() bool          // whether it initialised successfully
	Prompt() string           // input placeholder, e.g. "./path/to/project"
	Run(input string) tea.Cmd // returns a Cmd that delivers a ToolDoneMsg
}

// ToolDoneMsg is the single message type all tools deliver on completion.
type ToolDoneMsg struct {
	Tool   string // matches Tool.Name()
	Result ToolResult
}

type ToolResult struct {
	Summary []SummaryLine // ordered display rows
	Outputs []string      // file paths written, e.g. [".walker/output.json"]
	Err     error
}

// SummaryLine is one display row in the result view.
type SummaryLine struct {
	Label  string
	Value  string
	Indent bool
}
