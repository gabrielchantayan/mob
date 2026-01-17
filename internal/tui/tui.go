package tui

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	TabChat = iota
	TabDaemon
	TabAgentOutput
	TabAgents
)

type Model struct {
	ActiveTab      int
	InputRows      int
	Sidebar        Sidebar
	Toasts         *ToastQueue
	DaemonTab      DaemonTab
	AgentOutputTab AgentOutputTab
	AgentsTab      AgentsTab
}

func NewModel() Model {
	return Model{
		ActiveTab:      TabChat,
		InputRows:      clampHeight(3),
		Sidebar:        NewSidebar(),
		Toasts:         NewToastQueue(),
		DaemonTab:      NewDaemonTab(),
		AgentOutputTab: NewAgentOutputTab(),
		AgentsTab:      NewAgentsTab(),
	}
}

var ErrNotImplemented = errors.New("tui not implemented")

var startProgram = func(model tea.Model) error {
	program := tea.NewProgram(model)
	_, err := program.Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() string {
	return "[Chat] [Daemon] [Agent Output] [Agents]"
}

func Run() error {
	return startProgram(NewModel())
}
