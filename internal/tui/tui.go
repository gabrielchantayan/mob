package tui

const (
	TabChat = iota
	TabDaemon
	TabAgentOutput
	TabAgents
)

type Model struct {
	ActiveTab int
	InputRows int
}

func NewModel() Model {
	return Model{ActiveTab: TabChat, InputRows: clampHeight(3)}
}
