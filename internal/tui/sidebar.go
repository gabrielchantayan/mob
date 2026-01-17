package tui

type Sidebar struct{}

func NewSidebar() Sidebar {
	return Sidebar{}
}

func (Sidebar) View() string {
	return "Sidebar"
}
