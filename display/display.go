package display

import (
	"db-seeder/handlers"
	"db-seeder/walker"
	"fmt"
	"strconv"
	"strings"

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
	viewWalker
	viewWalkerRunning
	viewWalkerResult
)

type model struct {
	current      view
	cursor       int
	quitting     bool
	input        textinput.Model
	conn         *pgx.Conn
	walkerResult *walker.Result
}

func initialModel(conn *pgx.Conn) model {
	ti := textinput.New()
	ti.Placeholder = "200"
	ti.Focus()
	ti.CharLimit = 260
	ti.Width = 100

	return model{
		current: viewMenu,
		cursor:  0,
		input:   ti,
		conn:    conn,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle walker completion message
	if done, ok := msg.(walker.WalkDoneMsg); ok {
		m.walkerResult = &done.Result
		m.current = viewWalkerResult
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
			// Don't allow backing out of a running walk
			if m.current == viewWalkerRunning {
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
				if walker.Available() {
					limit = 3 // 4 items
				} else {
					limit = 2 // 3 items
				}
			case viewRemove:
				limit = 1
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
					if walker.Available() {
						m.current = viewWalker
						m.input.SetValue("")
						m.input.Placeholder = "./path/to/project"
						m.input.Focus()
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
				}

			case viewRemoveAll:
				handlers.RemoveAll(m.conn)
				m.current = viewMenu
				m.cursor = 0

			case viewRemoveField:
				handlers.RemoveByField(m.conn, m.input.Value())
				m.current = viewMenu
				m.cursor = 0

			case viewWalker:
				path := m.input.Value()
				if path == "" {
					path = "."
				}
				m.current = viewWalkerRunning
				return m, walker.RunCmd(path)

			case viewWalkerResult:
				m.walkerResult = nil
				m.current = viewMenu
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
	case viewWalker:
		return walkerInputView(m)
	case viewWalkerRunning:
		return walkerRunningView()
	case viewWalkerResult:
		return walkerResultView(m)
	default:
		return menuView(m)
	}
}

// ---------------------------------------------------------------------------
// Views
// ---------------------------------------------------------------------------

func menuView(m model) string {
	items := []string{"Seed Database", "Generate Test", "Remove Records"}
	if walker.Available() {
		items = append(items, "Run Walker")
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
	items := []string{"Delete All", "Delete by Field"}
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

func removeFieldView(m model) string {
	s := "enter value to delete by:\n\n"
	s += m.input.View()
	s += "\n\npress enter to confirm, q to go back"
	return s
}

func testView(m model) string {
	return "test generator — coming soon\n\npress q to go back"
}

func walkerInputView(m model) string {
	s := "path to scan:\n\n"
	s += m.input.View()
	s += "\n\npress enter to start, q to go back"
	return s
}

func walkerRunningView() string {
	return "scanning...\n\nplease wait"
}

func walkerResultView(m model) string {
	if m.walkerResult == nil {
		return "no result\n\npress enter to go back"
	}

	r := m.walkerResult

	if r.Err != nil {
		return fmt.Sprintf("walker failed\n\n%v\n\npress enter to go back", r.Err)
	}

	s := "walk complete\n\n"
	s += fmt.Sprintf("  files scanned   %d\n", r.TotalFiles)
	s += fmt.Sprintf("  endpoints found %d\n", r.TotalEndpoints)
	s += fmt.Sprintf("  db calls found  %d\n", r.TotalDBCalls)

	if len(r.Languages) > 0 {
		s += fmt.Sprintf("  languages       %s\n", strings.Join(r.Languages, ", "))
	}
	if len(r.DBLibraries) > 0 {
		s += fmt.Sprintf("  db libraries    %s\n", strings.Join(r.DBLibraries, ", "))
	}

	s += fmt.Sprintf("\n  → %s\n", r.JSONPath)
	s += fmt.Sprintf("  → %s\n", r.YAMLPath)

	s += "\npress enter to go back"
	return s
}

func StartInterface(conn *pgx.Conn) {
	p := tea.NewProgram(initialModel(conn))
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
