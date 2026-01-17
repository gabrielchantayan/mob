package tui

type DaemonTab struct{}

func NewDaemonTab() DaemonTab {
	return DaemonTab{}
}

func (DaemonTab) View() string {
	return "Daemon"
}
