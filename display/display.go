package display

import (
	"db-seeder/handlers"
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
	viewRemove      // remove sub-menu
	viewRemoveAll   // confirm delete all
	viewRemoveField // delete by field input
)

type model struct {
	current  view
	cursor   int
	quitting bool
	input    textinput.Model
	conn     *pgx.Conn
}

func initialModel(conn *pgx.Conn) model {
	ti := textinput.New()
	ti.Placeholder = "200"
	ti.Focus()
	ti.CharLimit = 6
	ti.Width = 20

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

	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.input, cmd = m.input.Update(msg)

		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		// q goes back to menu from sub-screens, quits from menu
		case "q":
			if m.current == viewMenu {
				m.quitting = true
				return m, tea.Quit
			}
			m.current = viewMenu
			m.cursor = 0

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			// limit depends on which screen we're on
			limit := 0
			switch m.current {
			case viewMenu:
				limit = 2 // 3 items
			case viewRemove:
				limit = 1 // 2 items
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
	default:
		return menuView(m)
	}
}

func menuView(m model) string {
	items := []string{"Seed Database", "Generate Test", "Remove Records"}

	menu := "what do you want to do?\n\n"
	for i, item := range items {
		if m.cursor == i {
			menu += "> " + item + "\n"
		} else {
			menu += "  " + item + "\n"
		}
	}
	menu += "\n(arrow keys to navigate, enter to select, q to quit)"
	return menu
}

func seedView(m model) string {
	screen := "how many records to seed?\n\n"
	screen += m.input.View()
	screen += "\n\npress enter to confirm, q to go back"
	return screen
}

func removeView(m model) string {
	items := []string{"Delete All", "Delete by Field"}

	screen := "what would you like to remove?\n\n"
	for i, item := range items {
		if m.cursor == i {
			screen += "> " + item + "\n"
		} else {
			screen += "  " + item + "\n"
		}
	}
	screen += "\n(enter to confirm, q to go back)"
	return screen
}

func removeAllView(m model) string {
	screen := "are you sure you want to delete all records?\n\n"
	screen += "press enter to confirm, q to go back"
	return screen
}

func removeFieldView(m model) string {
	screen := "enter value to delete by:\n\n"
	screen += m.input.View()
	screen += "\n\npress enter to confirm, q to go back"
	return screen
}

func testView(m model) string {
	return "test generator — coming soon\n\npress q to go back"
}

func StartInterface(conn *pgx.Conn) {
	p := tea.NewProgram(initialModel(conn))
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
