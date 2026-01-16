package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gabe/mob/internal/daemon"
	"github.com/gabe/mob/internal/soldati"
	"github.com/gabe/mob/internal/storage"
	"github.com/gabe/mob/internal/turf"
)

// tab represents the active tab in the TUI
type tab int

const (
	tabDashboard tab = iota
	tabAgents
	tabBeads
	tabLogs
)

// Tab names for display
var tabNames = []string{"Dashboard", "Agents", "Beads", "Logs"}

// Model represents the TUI state
type Model struct {
	activeTab tab
	width     int
	height    int
	quitting  bool

	// Data for display
	daemonStatus string
	daemonPID    int
	soldatiCount int
	beadsCount   int
	turfsCount   int
}

// New creates a new TUI model
func New() Model {
	m := Model{
		daemonStatus: "unknown",
	}
	m.loadData()
	return m
}

// loadData fetches current state from the various managers
func (m *Model) loadData() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	mobDir := filepath.Join(home, "mob")

	// Check daemon status
	d := daemon.New(mobDir)
	state, pid, err := d.Status()
	if err == nil {
		m.daemonStatus = string(state)
		m.daemonPID = pid
	}

	// Count soldati
	soldatiMgr, err := soldati.NewManager(filepath.Join(mobDir, "soldati"))
	if err == nil {
		list, err := soldatiMgr.List()
		if err == nil {
			m.soldatiCount = len(list)
		}
	}

	// Count beads
	beadsStore, err := storage.NewBeadStore(filepath.Join(mobDir, "beads"))
	if err == nil {
		beads, err := beadsStore.List(storage.BeadFilter{})
		if err == nil {
			m.beadsCount = len(beads)
		}
	}

	// Count turfs
	turfMgr, err := turf.NewManager(filepath.Join(mobDir, "turfs.toml"))
	if err == nil {
		m.turfsCount = len(turfMgr.List())
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % tab(len(tabNames))
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + tab(len(tabNames))) % tab(len(tabNames))
		case "1":
			m.activeTab = tabDashboard
		case "2":
			m.activeTab = tabAgents
		case "3":
			m.activeTab = tabBeads
		case "4":
			m.activeTab = tabLogs
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Title
	title := titleStyle.Render("MOB - Agent Orchestrator")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	// Content area
	content := m.renderContent()
	b.WriteString(content)
	b.WriteString("\n\n")

	// Help text
	help := helpStyle.Render("tab/arrows: switch tabs | 1-4: jump to tab | q: quit")
	b.WriteString(help)

	return b.String()
}

func (m Model) renderTabBar() string {
	var tabs []string

	for i, name := range tabNames {
		var style lipgloss.Style
		if tab(i) == m.activeTab {
			style = activeTabStyle
		} else {
			style = inactiveTabStyle
		}
		tabs = append(tabs, style.Render(fmt.Sprintf("[%d] %s", i+1, name)))
	}

	return tabBarStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, tabs...))
}

func (m Model) renderContent() string {
	var content string

	switch m.activeTab {
	case tabDashboard:
		content = m.renderDashboard()
	case tabAgents:
		content = m.renderAgents()
	case tabBeads:
		content = m.renderBeads()
	case tabLogs:
		content = m.renderLogs()
	}

	return contentStyle.Render(content)
}

func (m Model) renderDashboard() string {
	var b strings.Builder

	// Daemon status
	daemonLabel := "Daemon: "
	var daemonValue string
	if m.daemonStatus == "running" {
		daemonValue = statusStyle.Render(fmt.Sprintf("%s (PID %d)", m.daemonStatus, m.daemonPID))
	} else {
		daemonValue = mutedStyle.Render(m.daemonStatus)
	}
	b.WriteString(daemonLabel + daemonValue + "\n")

	// Counts
	b.WriteString(fmt.Sprintf("Soldati: %s\n", statusStyle.Render(fmt.Sprintf("%d", m.soldatiCount))))
	b.WriteString(fmt.Sprintf("Beads:   %s\n", statusStyle.Render(fmt.Sprintf("%d", m.beadsCount))))
	b.WriteString(fmt.Sprintf("Turfs:   %s\n", statusStyle.Render(fmt.Sprintf("%d", m.turfsCount))))

	return b.String()
}

func (m Model) renderAgents() string {
	return mutedStyle.Render("No active agents.")
}

func (m Model) renderBeads() string {
	return mutedStyle.Render("No beads.")
}

func (m Model) renderLogs() string {
	return mutedStyle.Render("No logs yet.")
}

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(New(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
