package display

import (
	"db-seeder/config"
	"db-seeder/handlers"
	"db-seeder/simulation/corpus"
	"db-seeder/tools"
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jackc/pgx/v5"
)

type view int

const (
	viewMenu view = iota
	viewSeed
	viewTest
	viewRemove
	viewRemoveAll
	viewRemoveField
	viewRemoveSimulation
	viewToolInput
	viewToolRunning
	viewToolResult
	viewConfigure
	viewConfigureField
)

type model struct {
	current    view
	cursor     int
	quitting   bool
	input      textinput.Model
	conn       *pgx.Conn
	tools      []tools.Tool
	activeTool tools.Tool
	toolResult *tools.ToolResult
	cfg        config.Config
	configKey  string // key currently being edited; empty = adding new
}

func initialModel(conn *pgx.Conn, ts []tools.Tool) model {
	ti := textinput.New()
	ti.Placeholder = "200"
	ti.Focus()
	ti.CharLimit = 260
	ti.Width = 100

	cfg, _ := config.Load("")

	return model{
		current: viewMenu,
		cursor:  0,
		input:   ti,
		conn:    conn,
		tools:   ts,
		cfg:     cfg,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func availableTools(ts []tools.Tool) []tools.Tool {
	var out []tools.Tool
	for _, t := range ts {
		if t.Available() {
			out = append(out, t)
		}
	}
	return out
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if done, ok := msg.(tools.ToolDoneMsg); ok {
		m.toolResult = &done.Result
		m.current = viewToolResult
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.input, cmd = m.input.Update(msg)

		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "q":
			if m.current == viewMenu {
				m.quitting = true
				return m, tea.Quit
			}
			if m.current == viewToolRunning {
				return m, cmd
			}
			m.current = viewMenu
			m.cursor = 0

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			limit := 0
			switch m.current {
			case viewMenu:
				limit = 3 + len(availableTools(m.tools)) // Seed, Test, Remove, Configure + tools
			case viewRemove:
				limit = 2
			case viewConfigure:
				limit = len(m.cfg.Keys()) // last entry is "add new"
			}
			if m.cursor < limit {
				m.cursor++
			}

		case "enter":
			switch m.current {

			case viewMenu:
				switch m.cursor {
				case 0:
					m.current = viewSeed
					m.input.SetValue("")
					m.input.Placeholder = "200"
				case 1:
					m.current = viewTest
				case 2:
					m.current = viewRemove
					m.cursor = 0
				case 3:
					m.current = viewConfigure
					m.cursor = 0
				default:
					ts := availableTools(m.tools)
					idx := m.cursor - 4
					if idx >= 0 && idx < len(ts) {
						m.activeTool = ts[idx]
						m.input.SetValue("")
						m.input.Placeholder = m.activeTool.Prompt()
						m.input.Focus()
						m.current = viewToolInput
					}
				}

			case viewSeed:
				count, err := strconv.Atoi(m.input.Value())
				if err != nil {
					break
				}
				handlers.InsertBatch(m.conn, count, count)
				m.current = viewMenu
				m.cursor = 0

			case viewRemove:
				switch m.cursor {
				case 0:
					m.current = viewRemoveAll
				case 1:
					m.current = viewRemoveField
					m.input.SetValue("")
					m.input.Placeholder = "enter email, username..."
					m.input.Focus()
				case 2:
					m.current = viewRemoveSimulation
				}

			case viewRemoveAll:
				handlers.RemoveAll(m.conn)
				m.current = viewMenu
				m.cursor = 0

			case viewRemoveField:
				handlers.RemoveByField(m.conn, m.input.Value())
				m.current = viewMenu
				m.cursor = 0

			case viewRemoveSimulation:
				if err := corpus.DeleteCorpus(corpus.DefaultPath); err != nil {
					fmt.Printf("corpus delete failed: %v\n", err)
				}
				m.current = viewMenu
				m.cursor = 0

			case viewToolInput:
				input := m.input.Value()
				if input == "" {
					input = "."
				}
				m.current = viewToolRunning
				return m, m.activeTool.Run(input)

			case viewToolResult:
				m.toolResult = nil
				m.activeTool = nil
				m.current = viewMenu
				m.cursor = 0

			case viewConfigure:
				keys := m.cfg.Keys()
				if m.cursor == len(keys) {
					// "add new" — first collect the key name
					m.configKey = ""
					m.input.SetValue("")
					m.input.Placeholder = "key name"
					m.input.Focus()
					m.current = viewConfigureField
				} else {
					// editing existing key
					m.configKey = keys[m.cursor]
					m.input.SetValue(m.cfg.Get(m.configKey))
					m.input.Focus()
					m.current = viewConfigureField
				}

			case viewConfigureField:
				if m.configKey == "" {
					// step 1 of add new — input was the key name, now get the value
					m.configKey = m.input.Value()
					m.input.SetValue("")
					m.input.Placeholder = "value"
					m.input.Focus()
					return m, nil
				}
				// step 2 or editing existing — save
				m.cfg.Set(m.configKey, m.input.Value())
				if err := config.Save(m.cfg, ""); err != nil {
					fmt.Printf("config save failed: %v\n", err)
				}
				m.configKey = ""
				m.current = viewConfigure
				m.cursor = 0
			}
		}
	}

	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return "bye\n"
	}

	switch m.current {
	case viewSeed:
		return seedView(m)
	case viewTest:
		return testView(m)
	case viewRemove:
		return removeView(m)
	case viewRemoveAll:
		return removeAllView(m)
	case viewRemoveField:
		return removeFieldView(m)
	case viewRemoveSimulation:
		return removeSimulationView(m)
	case viewToolInput:
		return toolInputView(m)
	case viewToolRunning:
		return toolRunningView(m)
	case viewToolResult:
		return toolResultView(m)
	case viewConfigure:
		return configureView(m)
	case viewConfigureField:
		return configureFieldView(m)
	default:
		return menuView(m)
	}
}

// ---------------------------------------------------------------------------
// Views
// ---------------------------------------------------------------------------

func menuView(m model) string {
	items := []string{"Seed Database", "Generate Test", "Remove Records", "Configure"}
	for _, t := range availableTools(m.tools) {
		items = append(items, "Run "+t.Name())
	}
	s := "what do you want to do?\n\n"
	for i, item := range items {
		if m.cursor == i {
			s += "> " + item + "\n"
		} else {
			s += "  " + item + "\n"
		}
	}
	s += "\n(arrow keys to navigate, enter to select, q to quit)"
	return s
}

func seedView(m model) string {
	s := "how many records to seed?\n\n"
	s += m.input.View()
	s += "\n\npress enter to confirm, q to go back"
	return s
}

func removeView(m model) string {
	items := []string{"Delete All", "Delete by Field", "Delete Simulation Corpus"}
	s := "what would you like to remove?\n\n"
	for i, item := range items {
		if m.cursor == i {
			s += "> " + item + "\n"
		} else {
			s += "  " + item + "\n"
		}
	}
	s += "\n(enter to confirm, q to go back)"
	return s
}

func removeAllView(m model) string {
	return "are you sure you want to delete all records?\n\npress enter to confirm, q to go back"
}

func removeSimulationView(m model) string {
	return "are you sure you want to delete the simulation corpus?\n\npress enter to confirm, q to go back"
}

func removeFieldView(m model) string {
	s := "enter value to delete by:\n\n"
	s += m.input.View()
	s += "\n\npress enter to confirm, q to go back"
	return s
}

func testView(m model) string {
	return "test generator — coming soon\n\npress q to go back"
}

func toolInputView(m model) string {
	s := m.activeTool.Name() + " — path to scan:\n\n"
	s += m.input.View()
	s += "\n\npress enter to start, q to go back"
	return s
}

func toolRunningView(m model) string {
	return m.activeTool.Name() + " running...\n\nplease wait"
}

func toolResultView(m model) string {
	r := m.toolResult
	if r == nil {
		return "no result\n\npress enter to go back"
	}
	if r.Err != nil {
		return fmt.Sprintf("%s failed\n\n%v\n\npress enter to go back", m.activeTool.Name(), r.Err)
	}
	s := m.activeTool.Name() + " complete\n\n"
	for _, line := range r.Summary {
		if line.Indent {
			s += fmt.Sprintf("    %s\n", line.Value)
		} else {
			s += fmt.Sprintf("  %-20s %s\n", line.Label, line.Value)
		}
	}
	for _, path := range r.Outputs {
		s += fmt.Sprintf("\n  → %s\n", path)
	}
	s += "\npress enter to go back"
	return s
}

func configureView(m model) string {
	keys := m.cfg.Keys()
	s := "configuration\n\n"
	if len(keys) == 0 {
		s += "  (no config set)\n"
	}
	for i, k := range keys {
		v := m.cfg.Get(k)
		if v == "" {
			v = "(not set)"
		}
		line := fmt.Sprintf("%-20s %s", k, v)
		if m.cursor == i {
			s += "> " + line + "\n"
		} else {
			s += "  " + line + "\n"
		}
	}
	if m.cursor == len(keys) {
		s += "> + add new\n"
	} else {
		s += "  + add new\n"
	}
	s += "\n(enter to edit, q to go back)"
	return s
}

func configureFieldView(m model) string {
	label := m.configKey
	if label == "" {
		label = "new key"
	}
	s := fmt.Sprintf("editing %s\n\n", label)
	s += m.input.View()
	s += "\n\npress enter to save, q to cancel"
	return s
}

func StartInterface(conn *pgx.Conn, ts []tools.Tool) {
	p := tea.NewProgram(initialModel(conn, ts), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
