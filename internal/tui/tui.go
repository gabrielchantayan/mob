package tui

import tea "github.com/charmbracelet/bubbletea"

// Model represents the TUI state
type Model struct {
	activeTab int
}

// New creates a new TUI model
func New() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() string {
	return "Mob TUI - Coming Soon"
}
