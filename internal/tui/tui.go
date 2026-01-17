package tui

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
