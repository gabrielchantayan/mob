package tui

type AgentOutputTab struct{}

func NewAgentOutputTab() AgentOutputTab {
	return AgentOutputTab{}
}

func (AgentOutputTab) View() string {
	return ""
}
