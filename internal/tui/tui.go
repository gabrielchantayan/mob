package tui

const (
	TabChat = iota
	TabDaemon
	TabAgentOutput
	TabAgents
)

type Model struct {
	ActiveTab int
}

func NewModel() Model {
	return Model{ActiveTab: TabChat}
}
