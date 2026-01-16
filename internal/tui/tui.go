package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gabe/mob/internal/agent"
	"github.com/gabe/mob/internal/daemon"
	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/registry"
	"github.com/gabe/mob/internal/soldati"
	"github.com/gabe/mob/internal/storage"
	"github.com/gabe/mob/internal/underboss"
)

// tab represents the active tab in the TUI
type tab int

const (
	tabChat tab = iota
	tabLogs
	tabAgents
)

// Tab names for display
var tabNames = []string{"Chat", "Logs", "Agents"}

// Layout constants
const (
	sidebarWidthConst  = 42  // Fixed sidebar width
	minWidthForSidebar = 120 // Below this, hide sidebar
)

// SoldatiStatus for sidebar display
type SoldatiStatus struct {
	Name   string
	Status string // "active", "idle"
}

// Tool icons (OpenCode style)
const (
	iconBash   = "$" // Shell commands
	iconRead   = "→" // File read
	iconSearch = "✱" // Search/grep
	iconWrite  = "←" // File write
	iconTask   = "◉" // Task/todo operations
	iconThink  = "◐" // Thinking
)

// Tool name to icon mapping
var toolIcons = map[string]string{
	"Bash":      iconBash,
	"Read":      iconRead,
	"Grep":      iconSearch,
	"Glob":      iconSearch,
	"Write":     iconWrite,
	"Edit":      iconWrite,
	"TodoWrite": iconTask,
	"Task":      iconTask,
}

// ChatMessage represents a message in the chat history
type ChatMessage struct {
	Role       string // "user" or "assistant"
	Content    string // For user messages
	Blocks     []agent.ChatContentBlock // For assistant messages
	Model      string
	DurationMs int64
	Timestamp  time.Time
}

// chatResponseMsg is sent when Claude responds
type chatResponseMsg struct {
	response *agent.ChatResponse
	err      error
}

// streamUpdateMsg is sent for streaming updates
type streamUpdateMsg struct {
	block agent.ChatContentBlock
}

// streamDoneMsg signals streaming is complete with final response
type streamDoneMsg struct {
	response *agent.ChatResponse
	err      error
}

// streamState holds the streaming state
type streamState struct {
	blockChan    chan agent.ChatContentBlock
	responseChan chan streamDoneMsg
}

// activeStream holds the current streaming state
var activeStream *streamState

// Model represents the TUI state
type Model struct {
	activeTab tab
	width     int
	height    int
	quitting  bool

	// Layout state
	sidebarVisible bool
	sidebarWidth   int

	// Data for display
	daemonStatus     string
	daemonPID        int
	sessionStartTime time.Time
	currentModel     string

	// Beads by status (for sidebar)
	beadsInProgress int
	beadsOpen       int
	beadsClosed     int

	// Active soldati (for sidebar)
	activeSoldati []SoldatiStatus

	// Chat state
	chatInput       textarea.Model
	chatViewport    viewport.Model
	chatMessages    []ChatMessage
	chatWaiting     bool
	chatError       string
	currentBlocks   []agent.ChatContentBlock // Blocks being streamed
	underboss       *underboss.Underboss
	mobDir          string

	// Agent records from registry
	agentRecords []*registry.AgentRecord
}

// New creates a new TUI model
func New() Model {
	home, _ := os.UserHomeDir()
	mobDir := filepath.Join(home, "mob")

	// Initialize textarea for multiline input
	ti := textarea.New()
	ti.Placeholder = "Type a message... (Alt+Enter for newline)"
	ti.CharLimit = 10000
	ti.SetWidth(80)
	ti.SetHeight(3)
	ti.ShowLineNumbers = false
	ti.KeyMap.InsertNewline.SetKeys("alt+enter", "ctrl+enter")

	// Initialize viewport for chat history
	vp := viewport.New(80, 10)

	// Initialize underboss
	spawner := agent.NewSpawner()
	ub := underboss.New(mobDir, spawner)

	// Focus the text input immediately
	ti.Focus()

	m := Model{
		activeTab:        tabChat, // Chat-first
		sidebarVisible:   true,
		sidebarWidth:     sidebarWidthConst,
		sessionStartTime: time.Now(),
		daemonStatus:     "unknown",
		chatInput:        ti,
		chatViewport:     vp,
		chatMessages:     []ChatMessage{},
		currentBlocks:    []agent.ChatContentBlock{},
		underboss:        ub,
		mobDir:           mobDir,
	}
	m.loadData()
	return m
}

// updateLayout calculates layout dimensions based on sidebar visibility
func (m *Model) updateLayout() {
	// Determine if sidebar should be visible based on width
	if m.width < minWidthForSidebar {
		m.sidebarVisible = false
		m.sidebarWidth = 0
	} else if m.sidebarVisible {
		m.sidebarWidth = sidebarWidthConst
	} else {
		m.sidebarWidth = 0
	}

	// Calculate main content width
	mainWidth := m.width - m.sidebarWidth - 4 // 4 for padding/borders
	if mainWidth < 40 {
		mainWidth = 40
	}

	// Update viewport dimensions
	m.chatViewport.Width = mainWidth - 4
	m.chatViewport.Height = m.height - 12

	// Update input width
	m.chatInput.SetWidth(mainWidth - 8)
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

	// Beads by status (for sidebar)
	beadsStore, err := storage.NewBeadStore(filepath.Join(mobDir, "beads"))
	if err == nil {
		allBeads, err := beadsStore.List(storage.BeadFilter{})
		if err == nil {
			m.beadsOpen = 0
			m.beadsInProgress = 0
			m.beadsClosed = 0
			for _, b := range allBeads {
				switch b.Status {
				case models.BeadStatusOpen:
					m.beadsOpen++
				case models.BeadStatusInProgress:
					m.beadsInProgress++
				case models.BeadStatusClosed:
					m.beadsClosed++
				}
			}
		}
	}

	// Soldati list (for sidebar)
	soldatiMgr, err := soldati.NewManager(filepath.Join(mobDir, "soldati"))
	if err == nil {
		list, err := soldatiMgr.List()
		if err == nil {
			m.activeSoldati = make([]SoldatiStatus, 0, len(list))
			for _, s := range list {
				status := "idle"
				if time.Since(s.LastActive) < 5*time.Minute {
					status = "active"
				}
				m.activeSoldati = append(m.activeSoldati, SoldatiStatus{
					Name:   s.Name,
					Status: status,
				})
			}
		}
	}

	// Load agent records from registry
	reg := registry.New(registry.DefaultPath(mobDir))
	agents, err := reg.List()
	if err == nil {
		m.agentRecords = agents
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case chatResponseMsg:
		m.chatWaiting = false
		if msg.err != nil {
			m.chatError = msg.err.Error()
		} else {
			// Add assistant message with full response
			m.chatMessages = append(m.chatMessages, ChatMessage{
				Role:       "assistant",
				Blocks:     msg.response.Blocks,
				Model:      msg.response.Model,
				DurationMs: msg.response.DurationMs,
				Timestamp:  time.Now(),
			})
			m.chatError = ""
			m.currentBlocks = nil
		}
		m.updateChatViewport()
		return m, nil

	case streamUpdateMsg:
		// Update current streaming blocks - replace or append based on block index
		m.updateStreamingBlock(msg.block)
		m.updateChatViewport()
		// Continue listening for more updates
		return m, listenForStreamUpdates()

	case streamDoneMsg:
		// Streaming complete - finalize the message
		m.chatWaiting = false
		if msg.err != nil {
			m.chatError = msg.err.Error()
		} else if msg.response != nil {
			// Add assistant message with full response
			m.chatMessages = append(m.chatMessages, ChatMessage{
				Role:       "assistant",
				Blocks:     msg.response.Blocks,
				Model:      msg.response.Model,
				DurationMs: msg.response.DurationMs,
				Timestamp:  time.Now(),
			})
			m.chatError = ""
		}
		m.currentBlocks = nil
		m.updateChatViewport()
		return m, nil

	case tea.KeyMsg:
		// Handle chat input when on chat tab
		if m.activeTab == tabChat && m.chatInput.Focused() {
			// Handle paste events with CRLF normalization
			if msg.Paste {
				normalized := strings.ReplaceAll(string(msg.Runes), "\r\n", "\n")
				normalized = strings.ReplaceAll(normalized, "\r", "\n")
				m.chatInput.InsertString(normalized)
				return m, nil
			}

			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc":
				m.chatInput.Blur()
				return m, nil
			case "enter":
				if !m.chatWaiting && strings.TrimSpace(m.chatInput.Value()) != "" {
					return m, m.sendMessage()
				}
				return m, nil
			// Scroll keybindings while input is focused
			case "ctrl+j":
				m.chatViewport.LineDown(1)
				return m, nil
			case "ctrl+k":
				m.chatViewport.LineUp(1)
				return m, nil
			case "ctrl+d":
				m.chatViewport.HalfViewDown()
				return m, nil
			case "ctrl+u":
				m.chatViewport.HalfViewUp()
				return m, nil
			case "ctrl+f":
				m.chatViewport.ViewDown()
				return m, nil
			case "ctrl+b":
				m.chatViewport.ViewUp()
				return m, nil
			}
			// Update text input
			var cmd tea.Cmd
			m.chatInput, cmd = m.chatInput.Update(msg)
			return m, cmd
		}

		// Normal navigation when not typing
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % tab(len(tabNames))
			m.updateFocus()
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + tab(len(tabNames))) % tab(len(tabNames))
			m.updateFocus()
		case "1":
			m.activeTab = tabChat
			m.updateFocus()
		case "2":
			m.activeTab = tabLogs
			m.updateFocus()
		case "3":
			m.activeTab = tabAgents
			m.updateFocus()
		case "s":
			// Toggle sidebar (only if terminal is wide enough)
			if m.width >= minWidthForSidebar {
				m.sidebarVisible = !m.sidebarVisible
				m.updateLayout()
			}
		case "i", "enter":
			if m.activeTab == tabChat && !m.chatWaiting {
				m.chatInput.Focus()
				return m, textarea.Blink
			}
		// Vim-style scroll keybindings when input is not focused
		case "j":
			if m.activeTab == tabChat {
				m.chatViewport.LineDown(1)
				return m, nil
			}
		case "k":
			if m.activeTab == tabChat {
				m.chatViewport.LineUp(1)
				return m, nil
			}
		case "ctrl+d":
			if m.activeTab == tabChat {
				m.chatViewport.HalfViewDown()
				return m, nil
			}
		case "ctrl+u":
			if m.activeTab == tabChat {
				m.chatViewport.HalfViewUp()
				return m, nil
			}
		case "ctrl+f":
			if m.activeTab == tabChat {
				m.chatViewport.ViewDown()
				return m, nil
			}
		case "ctrl+b":
			if m.activeTab == tabChat {
				m.chatViewport.ViewUp()
				return m, nil
			}
		case "g":
			if m.activeTab == tabChat {
				m.chatViewport.GotoTop()
				return m, nil
			}
		case "G":
			if m.activeTab == tabChat {
				m.chatViewport.GotoBottom()
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		m.updateChatViewport()

	case tea.MouseMsg:
		// Handle mouse wheel scrolling in chat viewport
		if m.activeTab == tabChat {
			var cmd tea.Cmd
			m.chatViewport, cmd = m.chatViewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update viewport for scrolling
	var cmd tea.Cmd
	m.chatViewport, cmd = m.chatViewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) updateFocus() {
	if m.activeTab == tabChat {
		m.chatInput.Focus()
	} else {
		m.chatInput.Blur()
	}
}

func (m *Model) updateStreamingBlock(block agent.ChatContentBlock) {
	// Find existing block by index and update it, or append new one
	for i, existing := range m.currentBlocks {
		if existing.Index == block.Index {
			m.currentBlocks[i] = block
			return
		}
	}
	// New block, append it
	m.currentBlocks = append(m.currentBlocks, block)
}

// isQuitCommand checks if the message is a vim-style quit command
func isQuitCommand(msg string) bool {
	msg = strings.TrimSpace(msg)
	// Match :q, :q!, :qa, :qa!, :wq, :wq!, etc.
	if !strings.HasPrefix(msg, ":") {
		return false
	}
	cmd := strings.TrimPrefix(msg, ":")
	cmd = strings.TrimSuffix(cmd, "!")
	// Common quit commands
	quitCmds := []string{"q", "qa", "wq", "wqa", "x", "xa"}
	for _, qc := range quitCmds {
		if cmd == qc {
			return true
		}
	}
	return false
}

func (m *Model) sendMessage() tea.Cmd {
	message := m.chatInput.Value()
	trimmed := strings.TrimSpace(message)

	// Check for quit commands first
	if isQuitCommand(trimmed) {
		m.chatInput.Reset()
		m.quitting = true
		return tea.Quit
	}

	// Check for slash commands
	if strings.HasPrefix(trimmed, "/") {
		return m.handleSlashCommand(trimmed)
	}

	m.chatInput.Reset()

	// Add user message to history
	m.chatMessages = append(m.chatMessages, ChatMessage{
		Role:      "user",
		Content:   message,
		Timestamp: time.Now(),
	})
	m.chatWaiting = true
	m.currentBlocks = nil
	m.updateChatViewport()

	// Create streaming state
	activeStream = &streamState{
		blockChan:    make(chan agent.ChatContentBlock, 100),
		responseChan: make(chan streamDoneMsg, 1),
	}

	// Start streaming request in background
	go func() {
		stream := activeStream
		callback := func(block agent.ChatContentBlock) {
			if stream != nil && stream.blockChan != nil {
				stream.blockChan <- block
			}
		}
		resp, err := m.underboss.AskStream(context.Background(), message, callback)
		if stream != nil {
			close(stream.blockChan)
			stream.responseChan <- streamDoneMsg{response: resp, err: err}
			close(stream.responseChan)
		}
	}()

	// Return command to listen for stream updates
	return listenForStreamUpdates()
}

// handleSlashCommand processes slash commands like /new
func (m *Model) handleSlashCommand(cmd string) tea.Cmd {
	m.chatInput.Reset()

	// Parse the command (remove leading slash, get first word)
	parts := strings.Fields(strings.TrimPrefix(cmd, "/"))
	if len(parts) == 0 {
		return nil
	}

	switch strings.ToLower(parts[0]) {
	case "new":
		// Clear the chat history and reset underboss session
		m.chatMessages = []ChatMessage{}
		m.currentBlocks = nil
		m.chatError = ""
		m.underboss.ClearSession()
		m.updateChatViewport()
		return nil
	default:
		// Unknown command - show error briefly
		m.chatError = fmt.Sprintf("Unknown command: /%s", parts[0])
		return nil
	}
}

// listenForStreamUpdates returns a command that waits for the next stream update
func listenForStreamUpdates() tea.Cmd {
	return func() tea.Msg {
		if activeStream == nil {
			return nil
		}

		// Try to read from block channel first
		select {
		case block, ok := <-activeStream.blockChan:
			if ok {
				return streamUpdateMsg{block: block}
			}
			// Block channel closed, wait for final response
			select {
			case done := <-activeStream.responseChan:
				activeStream = nil
				return done
			}
		case done := <-activeStream.responseChan:
			activeStream = nil
			return done
		}
	}
}

func (m *Model) updateChatViewport() {
	content := m.renderChatHistory()
	m.chatViewport.SetContent(content)
	m.chatViewport.GotoBottom()
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	// Calculate main content width
	mainWidth := m.width - m.sidebarWidth - 8 // padding
	if mainWidth < 40 {
		mainWidth = 40
	}

	// Build main area (tabs + content)
	mainArea := m.renderMainArea(mainWidth)

	// Build full content with optional sidebar
	var content string
	if m.sidebarVisible && m.sidebarWidth > 0 {
		sidebar := m.renderSidebar()
		content = lipgloss.JoinHorizontal(lipgloss.Top, mainArea, sidebar)
	} else {
		content = mainArea
	}

	// Add help footer
	var b strings.Builder
	b.WriteString(content)
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	// Add padding around the whole app
	padded := lipgloss.NewStyle().
		Padding(1, 2).
		Render(b.String())

	return m.fillBackground(padded)
}

func (m Model) renderMainArea(width int) string {
	var b strings.Builder

	// Tab bar at top
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	// Content area based on active tab
	switch m.activeTab {
	case tabChat:
		b.WriteString(m.renderChat())
	case tabLogs:
		b.WriteString(m.renderLogs())
	case tabAgents:
		b.WriteString(m.renderAgents())
	}

	return lipgloss.NewStyle().
		Width(width).
		Render(b.String())
}

func (m Model) fillBackground(content string) string {
	if m.width == 0 || m.height == 0 {
		return content
	}

	fullScreen := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Background(bgColor)

	return fullScreen.Render(content)
}

func (m Model) renderTabBar() string {
	var tabs []string

	for i, name := range tabNames {
		if tab(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(name))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(name))
		}
	}

	inner := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	box := selectorBarStyle.Render(inner)
	lines := strings.Count(box, "\n") + 1
	accent := accentBarStyle.Render(strings.Repeat("▌\n", lines-1) + "▌")

	return lipgloss.JoinHorizontal(lipgloss.Top, accent, box)
}

// renderSidebar renders the right sidebar with status info
func (m Model) renderSidebar() string {
	width := m.sidebarWidth - 4 // padding

	var b strings.Builder

	// Status section
	b.WriteString(m.renderSidebarStatus(width))
	b.WriteString("\n\n")

	// Beads section
	b.WriteString(m.renderSidebarBeads(width))
	b.WriteString("\n\n")

	// Agents section
	b.WriteString(m.renderSidebarAgents(width))

	// Sidebar container with left border
	return lipgloss.NewStyle().
		Width(m.sidebarWidth).
		Height(m.height - 6).
		Padding(1, 2).
		Background(bgPanelColor).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(borderSubtleColor).
		Render(b.String())
}

func (m Model) renderSidebarStatus(width int) string {
	var b strings.Builder

	// Section header
	b.WriteString(sidebarHeaderStyle.Render("┃ Status"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	// Daemon status
	daemonLabel := labelStyle.Render("Daemon")
	var daemonValue string
	if m.daemonStatus == "running" {
		daemonValue = valueStyle.Render(fmt.Sprintf("running (%d)", m.daemonPID))
	} else {
		daemonValue = statusInactiveStyle.Render(m.daemonStatus)
	}
	b.WriteString(fmt.Sprintf("%s  %s\n", daemonLabel, daemonValue))

	// Model (if known)
	if m.currentModel != "" {
		b.WriteString(fmt.Sprintf("%s   %s\n",
			labelStyle.Render("Model"),
			valueStyle.Render(formatModelName(m.currentModel))))
	}

	// Session duration
	duration := time.Since(m.sessionStartTime)
	b.WriteString(fmt.Sprintf("%s %s\n",
		labelStyle.Render("Session"),
		valueStyle.Render(formatDuration(duration))))

	return b.String()
}

func (m Model) renderSidebarBeads(width int) string {
	var b strings.Builder

	b.WriteString(sidebarHeaderStyle.Render("┃ Beads"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	totalBeads := m.beadsInProgress + m.beadsOpen + m.beadsClosed

	if totalBeads == 0 {
		b.WriteString(mutedStyle.Render("No beads"))
	} else {
		if m.beadsInProgress > 0 {
			b.WriteString(fmt.Sprintf("%s %d in progress\n",
				panelBaseStyle.Foreground(primaryColor).Render("●"),
				m.beadsInProgress))
		}
		if m.beadsOpen > 0 {
			b.WriteString(fmt.Sprintf("%s %d open\n",
				panelBaseStyle.Foreground(textMutedColor).Render("○"),
				m.beadsOpen))
		}
		if m.beadsClosed > 0 {
			b.WriteString(fmt.Sprintf("%s %d closed\n",
				panelBaseStyle.Foreground(successColor).Render("✓"),
				m.beadsClosed))
		}
	}

	return b.String()
}

func (m Model) renderSidebarAgents(width int) string {
	var b strings.Builder

	b.WriteString(sidebarHeaderStyle.Render("┃ Agents"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	if len(m.activeSoldati) == 0 {
		b.WriteString(mutedStyle.Render("No agents"))
	} else {
		for _, s := range m.activeSoldati {
			statusColor := textMutedColor
			if s.Status == "active" {
				statusColor = successColor
			}
			b.WriteString(fmt.Sprintf("%s  %s\n",
				labelStyle.Render(s.Name),
				panelBaseStyle.Foreground(statusColor).Render(s.Status)))
		}
	}

	return b.String()
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func (m Model) renderLogs() string {
	return mutedStyle.Render("No logs yet")
}

func (m Model) renderAgents() string {
	if len(m.agentRecords) == 0 {
		return mutedStyle.Render("No active agents")
	}

	var b strings.Builder

	// Header
	header := fmt.Sprintf("  %-20s %-12s %-10s %-30s %s\n",
		labelStyle.Render("Name"),
		labelStyle.Render("Type"),
		labelStyle.Render("Status"),
		labelStyle.Render("Task"),
		labelStyle.Render("Last Ping"))
	b.WriteString(header)
	b.WriteString(mutedStyle.Render(strings.Repeat("─", 90)))
	b.WriteString("\n")

	for _, agent := range m.agentRecords {
		name := agent.Name
		if name == "" {
			name = agent.ID[:8]
		}

		// Status color
		var statusStyled string
		switch agent.Status {
		case "active":
			statusStyled = statusStyle.Render(agent.Status)
		case "idle":
			statusStyled = mutedStyle.Render(agent.Status)
		case "stuck":
			statusStyled = errorStyle.Render(agent.Status)
		default:
			statusStyled = mutedStyle.Render(agent.Status)
		}

		// Truncate task
		task := agent.Task
		if len(task) > 28 {
			task = task[:25] + "..."
		}

		// Format last ping
		lastPing := formatRelativeTime(agent.LastPing)

		row := fmt.Sprintf("  %-20s %-12s %-10s %-30s %s\n",
			valueStyle.Render(name),
			mutedStyle.Render(agent.Type),
			statusStyled,
			valueStyle.Render(task),
			mutedStyle.Render(lastPing))
		b.WriteString(row)
	}

	return b.String()
}

func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh ago", int(d.Hours()))
}

func (m Model) renderChat() string {
	var b strings.Builder

	// Chat history viewport
	b.WriteString(m.chatViewport.View())
	b.WriteString("\n")

	// Status/error line
	if m.chatWaiting {
		b.WriteString(mutedStyle.Render("  Thinking..."))
		b.WriteString("\n")
	} else if m.chatError != "" {
		b.WriteString(errorStyle.Render("  Error: " + m.chatError))
		b.WriteString("\n")
	}

	// Calculate input width based on main area
	inputWidth := m.width - m.sidebarWidth - 16
	if inputWidth < 30 {
		inputWidth = 30
	}

	// Input area - minimal style with accent bar
	inputAccent := accentBarStyle.Render("▌")
	inputBox := lipgloss.NewStyle().
		Background(bgPanelColor).
		Padding(0, 1).
		Width(inputWidth).
		Render(m.chatInput.View())

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, inputAccent, inputBox))

	return b.String()
}

func (m Model) renderChatHistory() string {
	if len(m.chatMessages) == 0 && !m.chatWaiting {
		return mutedStyle.Render("Start a conversation with the Underboss...")
	}

	var b strings.Builder
	width := m.chatViewport.Width - 4

	for i, msg := range m.chatMessages {
		if msg.Role == "user" {
			// User message with blue left accent bar and background
			b.WriteString(m.renderUserMessage(msg.Content, width, i == 0))
			b.WriteString("\n")
		} else {
			// Assistant message with thinking, tool use, and text
			b.WriteString(m.renderAssistantMessage(msg, width))
			b.WriteString("\n")
		}
	}

	// Show current streaming blocks if waiting
	if m.chatWaiting && len(m.currentBlocks) > 0 {
		for _, block := range m.currentBlocks {
			b.WriteString(m.renderContentBlock(block, width))
		}
	}

	return b.String()
}

func (m Model) renderUserMessage(content string, width int, isFirst bool) string {
	// OpenCode style: left border with background panel
	// Structure: border | paddingLeft 2 | content

	borderChar := lipgloss.NewStyle().
		Foreground(secondaryColor).
		Background(bgPanelColor).
		Render("┃")

	// Wrap text accounting for border (1) + padding (2) = 3 chars
	wrapped := wrapText(content, width-5)
	lines := strings.Split(wrapped, "\n")

	// Build the inner content with padding
	var inner strings.Builder
	for i, line := range lines {
		if i > 0 {
			inner.WriteString("\n")
		}
		inner.WriteString(lipgloss.NewStyle().
			Foreground(textColor).
			Background(bgPanelColor).
			Render(line))
	}

	// Create the content box with padding and background
	contentBox := lipgloss.NewStyle().
		Background(bgPanelColor).
		PaddingTop(1).
		PaddingBottom(1).
		PaddingLeft(2).
		Width(width - 1). // Account for border
		Render(inner.String())

	// Join border with content line by line
	contentLines := strings.Split(contentBox, "\n")
	var b strings.Builder

	// Add margin top if not first message
	if !isFirst {
		b.WriteString("\n")
	}

	for i, line := range contentLines {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(borderChar + line)
	}

	return b.String()
}

func (m Model) renderAssistantMessage(msg ChatMessage, width int) string {
	var b strings.Builder

	for _, block := range msg.Blocks {
		b.WriteString(m.renderContentBlock(block, width))
	}

	// Footer with model and timing - OpenCode style: "▣ Mode · model · duration"
	// paddingLeft 3 to match text parts
	if msg.Model != "" || msg.DurationMs > 0 {
		// Build footer content
		footerContent := "▣ Build"
		if msg.Model != "" {
			footerContent += " · " + formatModelName(msg.Model)
		}
		if msg.DurationMs > 0 {
			footerContent += fmt.Sprintf(" · %.1fs", float64(msg.DurationMs)/1000.0)
		}

		// Style the entire footer line with background filling the width
		footerStyle := lipgloss.NewStyle().
			Foreground(textMutedColor).
			Background(bgColor).
			PaddingLeft(3).
			Width(width)

		b.WriteString("\n" + footerStyle.Render(footerContent) + "\n")
	}

	return b.String()
}

func (m Model) renderContentBlock(block agent.ChatContentBlock, width int) string {
	var b strings.Builder

	// Box-drawing border character (OpenCode uses "┃")
	borderChar := "┃"

	// Line style with background filling the width
	lineStyle := lipgloss.NewStyle().
		Background(bgColor).
		Width(width)

	switch block.Type {
	case agent.ContentTypeThinking:
		// OpenCode: paddingLeft 2, marginTop 1, border left with backgroundElement color
		border := lipgloss.NewStyle().
			Foreground(bgElementColor). // Subtle border for thinking
			Background(bgColor).
			Render(borderChar)

		// Header: "_Thinking:_" prefix like OpenCode
		header := "_Thinking:_ "
		if block.Summary != "" {
			header += block.Summary
		}
		headerStyled := lipgloss.NewStyle().
			Foreground(textMutedColor).
			Italic(true).
			Render(header)

		// marginTop 1
		b.WriteString(lineStyle.Render("") + "\n")
		b.WriteString(lineStyle.Render(border + "  " + headerStyled) + "\n")

		// Thinking content (muted, wrapped) - paddingLeft 2 from border
		if block.Text != "" {
			lines := strings.Split(wrapText(block.Text, width-6), "\n")
			for _, line := range lines {
				styledLine := lipgloss.NewStyle().Foreground(textMutedColor).Render(line)
				b.WriteString(lineStyle.Render(border + "  " + styledLine) + "\n")
			}
		}

	case agent.ContentTypeToolUse:
		// OpenCode InlineTool: paddingLeft 3, then paddingLeft 3 for text = 6 total
		// Get tool icon
		icon := toolIcons[block.Name]
		if icon == "" {
			icon = "⊛" // Default tool icon
		}

		// Tool header: icon + tool name
		toolHeader := fmt.Sprintf("%s %s", icon, block.Name)
		headerStyled := lipgloss.NewStyle().
			Foreground(textMutedColor).
			Render(toolHeader)

		// Extract and display description from input
		desc := extractToolDescription(block.Input)
		descStyled := ""
		if desc != "" {
			// Truncate if too long
			if len(desc) > width-12 {
				desc = desc[:width-15] + "..."
			}
			descStyled = lipgloss.NewStyle().Foreground(textMutedColor).Render(" " + desc)
		}

		// Inline format like OpenCode: icon name description with full-width background
		toolLineStyle := lipgloss.NewStyle().
			Background(bgColor).
			PaddingLeft(6).
			Width(width)
		b.WriteString(toolLineStyle.Render(headerStyled + descStyled) + "\n")

	case agent.ContentTypeText:
		// OpenCode TextPart: paddingLeft 3, marginTop 1
		// marginTop 1
		b.WriteString(lineStyle.Render("") + "\n")

		textLineStyle := lipgloss.NewStyle().
			Foreground(textColor).
			Background(bgColor).
			PaddingLeft(3).
			Width(width)

		lines := strings.Split(wrapText(block.Text, width-6), "\n")
		for _, line := range lines {
			b.WriteString(textLineStyle.Render(line) + "\n")
		}
	}

	return b.String()
}

func extractToolDescription(input string) string {
	var toolInfo struct {
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
		Command     string `json:"command"`
		FilePath    string `json:"file_path"`
		Pattern     string `json:"pattern"`
	}
	if input != "" {
		json.Unmarshal([]byte(input), &toolInfo)
	}

	// Priority: description > prompt > command > file_path > pattern
	if toolInfo.Description != "" {
		return toolInfo.Description
	}
	if toolInfo.Prompt != "" {
		return toolInfo.Prompt
	}
	if toolInfo.Command != "" {
		return toolInfo.Command
	}
	if toolInfo.FilePath != "" {
		return toolInfo.FilePath
	}
	if toolInfo.Pattern != "" {
		return toolInfo.Pattern
	}
	return ""
}

func formatModelName(model string) string {
	// Shorten model names for display
	if strings.Contains(model, "opus") {
		return "opus"
	}
	if strings.Contains(model, "sonnet") {
		return "sonnet"
	}
	if strings.Contains(model, "haiku") {
		return "haiku"
	}
	if model == "" {
		return "claude"
	}
	// Return last part of model name
	parts := strings.Split(model, "-")
	if len(parts) > 1 {
		return parts[len(parts)-2]
	}
	return model
}

func wrapText(text string, width int) string {
	if width <= 0 {
		width = 80
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		words := strings.Fields(line)
		lineLen := 0

		for j, word := range words {
			wordLen := len(word)

			if lineLen+wordLen+1 > width && lineLen > 0 {
				result.WriteString("\n")
				lineLen = 0
			}

			if lineLen > 0 {
				result.WriteString(" ")
				lineLen++
			}

			result.WriteString(word)
			lineLen += wordLen

			_ = j // Suppress unused variable warning
		}
	}

	return result.String()
}

func (m Model) renderHelp() string {
	items := []struct {
		key  string
		desc string
	}{
		{"tab", "navigate"},
		{"1-3", "jump"},
	}

	// Add sidebar toggle if terminal is wide enough
	if m.width >= minWidthForSidebar {
		items = append(items, struct {
			key  string
			desc string
		}{"s", "sidebar"})
	}

	items = append(items, struct {
		key  string
		desc string
	}{":q", "quit"})

	if m.activeTab == tabChat {
		if m.chatInput.Focused() {
			items = []struct {
				key  string
				desc string
			}{
				{"enter", "send"},
				{"alt+enter", "newline"},
				{"ctrl+j/k", "scroll"},
				{"ctrl+u/d", "½page"},
				{"esc", "cancel"},
			}
		} else {
			items = append(items, struct {
				key  string
				desc string
			}{"i", "type"}, struct {
				key  string
				desc string
			}{"j/k", "scroll"}, struct {
				key  string
				desc string
			}{"g/G", "top/btm"})
		}
	}

	var parts []string
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("%s %s",
			keyStyle.Render(item.key),
			keyDescStyle.Render(item.desc)))
	}

	return strings.Join(parts, "    ")
}

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(New(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
