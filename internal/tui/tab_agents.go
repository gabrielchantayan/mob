package tui

type AgentsTab struct{}

func NewAgentsTab() AgentsTab {
	return AgentsTab{}
}

func (AgentsTab) View() string {
	return ""
}
