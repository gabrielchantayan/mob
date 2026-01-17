package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	"github.com/mattn/go-runewidth"
)

// tab represents the active tab in the TUI
type tab int

const (
	tabChat tab = iota
	tabDaemon
	tabAgentOutput
	tabAgents
)

// Tab names for display
var tabNames = []string{"Chat", "Daemon", "Agent Output", "Agents"}

// Layout constants
const (
	sidebarWidthConst  = 42  // Fixed sidebar width
	minWidthForSidebar = 120 // Below this, hide sidebar
	minInputHeight     = 3   // Minimum textarea height
	maxInputHeight     = 24  // Maximum textarea height before scrolling
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
	Role       string                   // "user" or "assistant"
	Content    string                   // For user messages
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

// sidebarTickMsg triggers periodic sidebar data refresh
type sidebarTickMsg time.Time

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
	beadsInProgress      int
	beadsOpen            int
	beadsPendingApproval int
	beadsClosed          int

	// Active soldati (for sidebar)
	activeSoldati []SoldatiStatus

	// Chat state
	chatInput     textarea.Model
	chatViewport  viewport.Model
	chatMessages  []ChatMessage
	chatWaiting   bool
	chatError     string
	currentBlocks []agent.ChatContentBlock // Blocks being streamed
	underboss     *underboss.Underboss
	mobDir        string

	// Agent records from registry
	agentRecords []*registry.AgentRecord

	// Daemon log state
	daemonLogViewport viewport.Model
	daemonLogLines    []string
	daemonLogFile     string

	// Agent output state
	agentOutputViewport viewport.Model
	agentOutputLines    []AgentOutputLine
	agentOutputFilter   string // Filter by agent name (empty = all)

	// Usage statistics
	sessionInputTokens  int
	sessionOutputTokens int
	sessionTotalCost    float64
}

// AgentOutputLine represents a line of agent output with metadata
type AgentOutputLine struct {
	AgentID   string
	AgentName string
	Line      string
	Timestamp time.Time
	Stream    string
}

// New creates a new TUI model
func New() Model {
	home, _ := os.UserHomeDir()
	mobDir := filepath.Join(home, "mob")

	// Initialize textarea for multiline input
	ti := textarea.New()
	ti.Placeholder = "Type a message... (Ctrl+Enter to send)"
	ti.CharLimit = 10000
	ti.SetWidth(80)
	ti.SetHeight(minInputHeight) // Start small, grows dynamically
	ti.ShowLineNumbers = false
	ti.KeyMap.InsertNewline.SetKeys("enter") // Enter adds newline, like opencode

	// Set textarea styles to match the panel background
	ti.FocusedStyle.Base = panelBaseStyle
	ti.BlurredStyle.Base = panelBaseStyle
	ti.FocusedStyle.CursorLine = panelBaseStyle
	ti.BlurredStyle.CursorLine = panelBaseStyle
	ti.Cursor.Style = panelBaseStyle.Foreground(textColor)
	ti.Prompt = " " // No prompt, we use the sidebar

	// Initialize viewport for chat history
	vp := viewport.New(80, 10)

	// Initialize viewport for daemon logs
	daemonVp := viewport.New(80, 10)

	// Initialize viewport for agent output
	agentOutputVp := viewport.New(80, 10)

	// Initialize underboss
	spawner := agent.NewSpawner()
	ub := underboss.New(mobDir, spawner)

	// Focus the text input immediately
	ti.Focus()

	m := Model{
		activeTab:           tabChat, // Chat-first
		sidebarVisible:      true,
		sidebarWidth:        sidebarWidthConst,
		sessionStartTime:    time.Now(),
		daemonStatus:        "unknown",
		chatInput:           ti,
		chatViewport:        vp,
		chatMessages:        []ChatMessage{},
		currentBlocks:       []agent.ChatContentBlock{},
		underboss:           ub,
		mobDir:              mobDir,
		daemonLogViewport:   daemonVp,
		daemonLogLines:      []string{},
		daemonLogFile:       filepath.Join(mobDir, ".mob", "daemon.log"),
		agentOutputViewport: agentOutputVp,
		agentOutputLines:    []AgentOutputLine{},
		agentOutputFilter:   "",
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

	// Update input width and height first (so we know the input height)
	m.chatInput.SetWidth(mainWidth - 8)
	m.updateInputHeight()

	// Calculate viewport height dynamically based on input height
	// Layout: padding(2) + tabbar(3) + viewport + input(dynamic) + status(1) + help(2) + padding(2)
	// Fixed overhead = ~10 lines, plus the dynamic input height
	inputHeight := m.chatInput.Height()
	fixedOverhead := 10 // tabs, borders, help, padding, etc.
	viewportHeight := m.height - fixedOverhead - inputHeight
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	// Update viewport dimensions
	m.chatViewport.Width = mainWidth - 4
	m.chatViewport.Height = viewportHeight

	// Update daemon log viewport dimensions (simpler - no input area)
	daemonViewportHeight := m.height - 10 // tabs, borders, help, padding
	if daemonViewportHeight < 5 {
		daemonViewportHeight = 5
	}
	m.daemonLogViewport.Width = mainWidth - 4
	m.daemonLogViewport.Height = daemonViewportHeight

	// Update agent output viewport dimensions (same as daemon log)
	m.agentOutputViewport.Width = mainWidth - 4
	m.agentOutputViewport.Height = daemonViewportHeight
}

// updateInputHeight adjusts textarea height based on content lines
// and recalculates viewport height so the input expands upward
func (m *Model) updateInputHeight() {
	content := m.chatInput.Value()

	// Count lines in the content
	lines := 1
	if content != "" {
		lines = strings.Count(content, "\n") + 1
	}

	// Clamp to min/max bounds
	height := lines
	if height < minInputHeight {
		height = minInputHeight
	}
	if height > maxInputHeight {
		height = maxInputHeight
	}

	oldHeight := m.chatInput.Height()
	m.chatInput.SetHeight(height)

	// If input height changed, recalculate viewport height so input expands upward
	if height != oldHeight && m.height > 0 {
		fixedOverhead := 10 // tabs, borders, help, padding, etc.
		viewportHeight := m.height - fixedOverhead - height
		if viewportHeight < 5 {
			viewportHeight = 5
		}
		m.chatViewport.Height = viewportHeight
	}
}

// loadData fetches current state from the various managers
func (m *Model) loadData() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	mobDir := filepath.Join(home, "mob")

	// Check daemon status
	d := daemon.New(mobDir, log.New(io.Discard, "", 0))
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
			m.beadsPendingApproval = 0
			m.beadsClosed = 0
			for _, b := range allBeads {
				switch b.Status {
				case models.BeadStatusOpen:
					m.beadsOpen++
				case models.BeadStatusInProgress:
					m.beadsInProgress++
				case models.BeadStatusPendingApproval:
					m.beadsPendingApproval++
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

	// Load daemon logs (tail last 500 lines)
	m.loadDaemonLogs()
}

func (m Model) Init() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return sidebarTickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case sidebarTickMsg:
		// Refresh sidebar data periodically
		m.loadData()
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return sidebarTickMsg(t)
		})

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
			// Update usage statistics
			m.sessionInputTokens += msg.response.InputTokens
			m.sessionOutputTokens += msg.response.OutputTokens
			m.sessionTotalCost += msg.response.TotalCost
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
			// Update usage statistics
			m.sessionInputTokens += msg.response.InputTokens
			m.sessionOutputTokens += msg.response.OutputTokens
			m.sessionTotalCost += msg.response.TotalCost
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
				// Dynamically adjust height for pasted content
				m.updateInputHeight()
				return m, nil
			}

			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc":
				m.chatInput.Blur()
				return m, nil
			case "ctrl+enter":
				// Ctrl+Enter sends the message
				if !m.chatWaiting && strings.TrimSpace(m.chatInput.Value()) != "" {
					return m, m.sendMessage()
				}
				return m, nil
			// Scroll keybindings while input is focused
			// Note: ctrl+j is used for newline in textarea, use ctrl+n/ctrl+p for line scroll
			case "ctrl+n":
				m.chatViewport.LineDown(1)
				return m, nil
			case "ctrl+p":
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
			// Dynamically adjust height based on content
			m.updateInputHeight()
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
			m.activeTab = tabDaemon
			m.updateFocus()
		case "3":
			m.activeTab = tabAgentOutput
			m.updateFocus()
		case "4":
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
			switch m.activeTab {
			case tabChat:
				m.chatViewport.LineDown(1)
			case tabDaemon:
				m.daemonLogViewport.LineDown(1)
			case tabAgentOutput:
				m.agentOutputViewport.LineDown(1)
			}
			return m, nil
		case "k":
			switch m.activeTab {
			case tabChat:
				m.chatViewport.LineUp(1)
			case tabDaemon:
				m.daemonLogViewport.LineUp(1)
			case tabAgentOutput:
				m.agentOutputViewport.LineUp(1)
			}
			return m, nil
		case "ctrl+d":
			switch m.activeTab {
			case tabChat:
				m.chatViewport.HalfViewDown()
			case tabDaemon:
				m.daemonLogViewport.HalfViewDown()
			case tabAgentOutput:
				m.agentOutputViewport.HalfViewDown()
			}
			return m, nil
		case "ctrl+u":
			switch m.activeTab {
			case tabChat:
				m.chatViewport.HalfViewUp()
			case tabDaemon:
				m.daemonLogViewport.HalfViewUp()
			case tabAgentOutput:
				m.agentOutputViewport.HalfViewUp()
			}
			return m, nil
		case "ctrl+f":
			switch m.activeTab {
			case tabChat:
				m.chatViewport.ViewDown()
			case tabDaemon:
				m.daemonLogViewport.ViewDown()
			case tabAgentOutput:
				m.agentOutputViewport.ViewDown()
			}
			return m, nil
		case "ctrl+b":
			switch m.activeTab {
			case tabChat:
				m.chatViewport.ViewUp()
			case tabDaemon:
				m.daemonLogViewport.ViewUp()
			case tabAgentOutput:
				m.agentOutputViewport.ViewUp()
			}
			return m, nil
		case "g":
			switch m.activeTab {
			case tabChat:
				m.chatViewport.GotoTop()
			case tabDaemon:
				m.daemonLogViewport.GotoTop()
			case tabAgentOutput:
				m.agentOutputViewport.GotoTop()
			}
			return m, nil
		case "G":
			switch m.activeTab {
			case tabChat:
				m.chatViewport.GotoBottom()
			case tabDaemon:
				m.daemonLogViewport.GotoBottom()
			case tabAgentOutput:
				m.agentOutputViewport.GotoBottom()
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		m.updateChatViewport()
		m.updateDaemonViewport()

	case tea.MouseMsg:
		// Handle mouse wheel scrolling in viewports
		switch m.activeTab {
		case tabChat:
			var cmd tea.Cmd
			m.chatViewport, cmd = m.chatViewport.Update(msg)
			cmds = append(cmds, cmd)
		case tabDaemon:
			var cmd tea.Cmd
			m.daemonLogViewport, cmd = m.daemonLogViewport.Update(msg)
			cmds = append(cmds, cmd)
		case tabAgentOutput:
			var cmd tea.Cmd
			m.agentOutputViewport, cmd = m.agentOutputViewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update viewports for scrolling
	var cmd tea.Cmd
	switch m.activeTab {
	case tabChat:
		m.chatViewport, cmd = m.chatViewport.Update(msg)
	case tabDaemon:
		m.daemonLogViewport, cmd = m.daemonLogViewport.Update(msg)
	case tabAgentOutput:
		m.agentOutputViewport, cmd = m.agentOutputViewport.Update(msg)
	}
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
		m.updateInputHeight()
		m.quitting = true
		return tea.Quit
	}

	// Check for slash commands
	if strings.HasPrefix(trimmed, "/") {
		return m.handleSlashCommand(trimmed)
	}

	m.chatInput.Reset()
	m.updateInputHeight()

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
	m.updateInputHeight()

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
	case tabDaemon:
		b.WriteString(m.renderDaemon())
	case tabAgentOutput:
		b.WriteString(m.renderAgentOutput())
	case tabAgents:
		b.WriteString(m.renderAgents())
	}

	// Fixed height to prevent layout shifts when input grows
	contentHeight := m.height - 6 // Account for padding and help text
	return lipgloss.NewStyle().
		Width(width).
		Height(contentHeight).
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
	b.WriteString("\n\n")

	// Usage section
	b.WriteString(m.renderSidebarUsage(width))

	// Sidebar container with left border
	return lipgloss.NewStyle().
		Width(m.sidebarWidth).
		Height(m.height-6).
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

	totalBeads := m.beadsInProgress + m.beadsOpen + m.beadsPendingApproval + m.beadsClosed

	if totalBeads == 0 {
		b.WriteString(mutedStyle.Render("No beads"))
	} else {
		if m.beadsInProgress > 0 {
			b.WriteString(fmt.Sprintf("%s %d in progress\n",
				panelBaseStyle.Foreground(primaryColor).Render("●"),
				m.beadsInProgress))
		}
		if m.beadsPendingApproval > 0 {
			b.WriteString(fmt.Sprintf("%s %d pending approval\n",
				panelBaseStyle.Foreground(lipgloss.Color("#FFA500")).Render("⚠"),
				m.beadsPendingApproval))
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

func (m Model) renderSidebarUsage(width int) string {
	var b strings.Builder

	b.WriteString(sidebarHeaderStyle.Render("┃ Usage"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	// Session stats
	sessionDuration := time.Since(m.sessionStartTime)
	b.WriteString(labelStyle.Render("Session") + "  " + valueStyle.Render(formatDuration(sessionDuration)) + "\n")

	// Token usage
	totalTokens := m.sessionInputTokens + m.sessionOutputTokens
	if totalTokens > 0 {
		b.WriteString(fmt.Sprintf("%s  %s\n",
			labelStyle.Render("Tokens"),
			valueStyle.Render(fmt.Sprintf("%d in / %d out", m.sessionInputTokens, m.sessionOutputTokens))))
	} else {
		b.WriteString(labelStyle.Render("Tokens") + "  " + mutedStyle.Render("none yet") + "\n")
	}

	// Cost
	if m.sessionTotalCost > 0 {
		b.WriteString(fmt.Sprintf("%s  %s\n",
			labelStyle.Render("Cost"),
			valueStyle.Render(fmt.Sprintf("$%.4f", m.sessionTotalCost))))
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

// loadDaemonLogs reads the daemon log file and updates the viewport
func (m *Model) loadDaemonLogs() {
	content, err := os.ReadFile(m.daemonLogFile)
	if err != nil {
		if os.IsNotExist(err) {
			m.daemonLogLines = []string{}
		}
		return
	}

	lines := strings.Split(string(content), "\n")

	// Keep last 500 lines to avoid memory issues
	maxLines := 500
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	m.daemonLogLines = lines
	m.updateDaemonViewport()
}

// updateDaemonViewport refreshes the daemon log viewport content
func (m *Model) updateDaemonViewport() {
	if len(m.daemonLogLines) == 0 {
		m.daemonLogViewport.SetContent(mutedStyle.Render("No daemon logs yet. Start the daemon with: mob daemon start"))
		return
	}

	var b strings.Builder
	width := m.daemonLogViewport.Width - 4
	if width < 40 {
		width = 80
	}

	for _, line := range m.daemonLogLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Style log lines - highlight important events
		styledLine := m.styleDaemonLogLine(line, width)
		b.WriteString(styledLine)
		b.WriteString("\n")
	}

	m.daemonLogViewport.SetContent(b.String())
	m.daemonLogViewport.GotoBottom()
}

// styleDaemonLogLine applies styling to a single daemon log line
func (m Model) styleDaemonLogLine(line string, width int) string {
	// Log format: "2024/01/16 12:34:56 Message here"
	// Extract timestamp and message
	if len(line) < 20 {
		return mutedStyle.Render(line)
	}

	// Detect log type by keywords and style accordingly
	lower := strings.ToLower(line)

	var msgStyle lipgloss.Style
	switch {
	case strings.Contains(lower, "error"):
		msgStyle = baseStyle.Foreground(errorColor)
	case strings.Contains(lower, "warning"):
		msgStyle = baseStyle.Foreground(warningColor)
	case strings.Contains(lower, "started") || strings.Contains(lower, "spawning") || strings.Contains(lower, "active"):
		msgStyle = baseStyle.Foreground(successColor)
	case strings.Contains(lower, "stopped") || strings.Contains(lower, "shutdown") || strings.Contains(lower, "killing"):
		msgStyle = baseStyle.Foreground(warningColor)
	case strings.Contains(lower, "hook:") || strings.Contains(lower, "assignment"):
		msgStyle = baseStyle.Foreground(secondaryColor)
	case strings.Contains(lower, "patrol:"):
		msgStyle = baseStyle.Foreground(textMutedColor)
	default:
		msgStyle = baseStyle.Foreground(textColor)
	}

	// Try to extract timestamp (first 19 chars typically "2024/01/16 12:34:56")
	if len(line) >= 20 && line[4] == '/' && line[7] == '/' {
		timestamp := line[:19]
		message := line[20:]

		timestampStyled := mutedStyle.Render(timestamp)
		messageStyled := msgStyle.Render(message)

		return timestampStyled + " " + messageStyled
	}

	return msgStyle.Render(line)
}

func (m Model) renderDaemon() string {
	var b strings.Builder

	// Status header
	var statusText string
	if m.daemonStatus == "running" {
		statusText = statusStyle.Render(fmt.Sprintf("● Daemon running (PID %d)", m.daemonPID))
	} else {
		statusText = statusInactiveStyle.Render("○ Daemon not running")
	}
	b.WriteString(statusText)
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(strings.Repeat("─", m.daemonLogViewport.Width)))
	b.WriteString("\n")

	// Log viewport
	b.WriteString(m.daemonLogViewport.View())

	return b.String()
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

func (m Model) renderAgentOutput() string {
	if len(m.agentOutputLines) == 0 {
		return mutedStyle.Render("No agent output yet. Agents will appear here when they start working.")
	}

	var b strings.Builder

	// Header
	filterText := "all agents"
	if m.agentOutputFilter != "" {
		filterText = m.agentOutputFilter
	}
	header := fmt.Sprintf("Agent Output - Showing: %s (%d lines)",
		filterText, len(m.agentOutputLines))
	b.WriteString(labelStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(strings.Repeat("─", m.agentOutputViewport.Width)))
	b.WriteString("\n")

	// Output viewport
	b.WriteString(m.agentOutputViewport.View())

	return b.String()
}

// updateAgentOutputViewport refreshes the agent output viewport content
func (m *Model) updateAgentOutputViewport() {
	if len(m.agentOutputLines) == 0 {
		m.agentOutputViewport.SetContent(mutedStyle.Render("Waiting for agent output..."))
		return
	}

	var b strings.Builder
	width := m.agentOutputViewport.Width - 4
	if width < 40 {
		width = 80
	}

	for _, line := range m.agentOutputLines {
		// Apply filter if set
		if m.agentOutputFilter != "" && line.AgentName != m.agentOutputFilter {
			continue
		}

		// Format: [timestamp] [agent-name] line
		timestamp := line.Timestamp.Format("15:04:05")
		timestampStyled := mutedStyle.Render(timestamp)

		// Color-code by agent name (cycle through a few colors)
		agentColor := getAgentColor(line.AgentName)
		agentNameStyled := lipgloss.NewStyle().Foreground(agentColor).Render(line.AgentName)

		// Style based on stream
		var lineStyled string
		if line.Stream == "stderr" {
			lineStyled = errorStyle.Render(line.Line)
		} else {
			lineStyled = baseStyle.Foreground(textColor).Render(line.Line)
		}

		b.WriteString(fmt.Sprintf("%s [%s] %s\n", timestampStyled, agentNameStyled, lineStyled))
	}

	m.agentOutputViewport.SetContent(b.String())
	m.agentOutputViewport.GotoBottom()
}

// getAgentColor returns a color for an agent name (deterministic based on name)
func getAgentColor(name string) lipgloss.Color {
	colors := []lipgloss.Color{
		lipgloss.Color("#00D4FF"), // cyan
		lipgloss.Color("#A6E22E"), // green
		lipgloss.Color("#F92672"), // pink
		lipgloss.Color("#FD971F"), // orange
		lipgloss.Color("#AE81FF"), // purple
		lipgloss.Color("#E6DB74"), // yellow
	}

	// Simple hash of the name to pick a color
	hash := 0
	for _, c := range name {
		hash += int(c)
	}
	return colors[hash%len(colors)]
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

	// Calculate input width based on main area
	inputWidth := m.width - m.sidebarWidth - 16
	if inputWidth < 30 {
		inputWidth = 30
	}
	// Update textarea width
	m.chatInput.SetWidth(inputWidth)

	// Shared style for footer elements to match input box background
	footerStyle := lipgloss.NewStyle().
		Background(bgPanelColor).
		PaddingTop(1).
		PaddingBottom(1).
		Padding(0, 1).
		Width(inputWidth)

	// Build footer content (Status + Input)
	var footerBuilder strings.Builder

	// Status/error line
	if m.chatWaiting {
		footerBuilder.WriteString(footerStyle.Foreground(textMutedColor).Render("Thinking..."))
		footerBuilder.WriteString("\n")
	} else if m.chatError != "" {
		footerBuilder.WriteString(footerStyle.Foreground(errorColor).Render("Error: " + m.chatError))
		footerBuilder.WriteString("\n")
	}

	// Input area
	// We want the input area to also have the panel background and full width
	inputView := m.chatInput.View()
	// Force background color on lines from textarea view
	inputLines := strings.Split(inputView, "\n")
	for i, line := range inputLines {
		if i > 0 {
			footerBuilder.WriteString("\n")
		}
		// Render each line with the footer style to ensure full width background
		footerBuilder.WriteString(footerStyle.Render(line))
	}

	// Add accent bar to the whole footer
	footerContent := footerBuilder.String()
	footerHeight := lipgloss.Height(footerContent)
	accent := accentBarStyle.Render(strings.Repeat("▌\n", footerHeight-1) + "▌")

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, accent, footerContent))

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
		for _, part := range buildAssistantParts(m.currentBlocks) {
			b.WriteString(m.renderAssistantPart(part, width))
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

func (m Model) renderAssistantFooter(msg ChatMessage, width int) string {
	if msg.Model == "" && msg.DurationMs == 0 {
		return ""
	}

	footer := "▣ Build"
	if msg.Model != "" {
		footer += " · " + formatModelName(msg.Model)
	}
	if msg.DurationMs > 0 {
		footer += fmt.Sprintf(" · %.1fs", float64(msg.DurationMs)/1000.0)
	}

	return lipgloss.NewStyle().
		Foreground(textMutedColor).
		Background(bgColor).
		PaddingLeft(3).
		Width(width).
		Render(footer)
}

func (m Model) renderAssistantMessage(msg ChatMessage, width int) string {
	var b strings.Builder

	for _, part := range buildAssistantParts(msg.Blocks) {
		b.WriteString(m.renderAssistantPart(part, width))
	}

	footer := m.renderAssistantFooter(msg, width)
	if footer != "" {
		b.WriteString("\n" + footer + "\n")
	}

	return b.String()
}

func (m Model) renderContentBlock(block agent.ChatContentBlock, width int) string {
	var b strings.Builder

	for _, part := range buildAssistantParts([]agent.ChatContentBlock{block}) {
		b.WriteString(m.renderAssistantPart(part, width))
	}

	return b.String()
}

func (m Model) renderAssistantPart(part chatPart, width int) string {
	var b strings.Builder

	// Box-drawing border character (OpenCode uses "┃")
	borderChar := "┃"

	// Line style with background filling the width
	lineStyle := lipgloss.NewStyle().
		Background(bgColor).
		Width(width)

	switch part.Type {
	case partReasoning:
		// OpenCode: paddingLeft 2, marginTop 1, border left with backgroundElement color
		border := lipgloss.NewStyle().
			Foreground(bgElementColor). // Subtle border for thinking
			Background(bgColor).
			Render(borderChar)

		// Header: "_Thinking:_" prefix like OpenCode
		header := "_Thinking:_ "
		if part.Summary != "" {
			header += part.Summary
		}
		headerStyled := lipgloss.NewStyle().
			Foreground(textMutedColor).
			Italic(true).
			Render(header)

		// marginTop 1
		b.WriteString(lineStyle.Render("") + "\n")
		b.WriteString(lineStyle.Render(border+"  "+headerStyled) + "\n")

		// Thinking content (muted, wrapped) - paddingLeft 2 from border
		if part.Text != "" {
			lines := strings.Split(wrapText(part.Text, width-6), "\n")
			for _, line := range lines {
				styledLine := lipgloss.NewStyle().Foreground(textMutedColor).Italic(true).Render(line)
				b.WriteString(lineStyle.Render(border+"  "+styledLine) + "\n")
			}
		}

	case partTool:
		b.WriteString(m.renderAssistantTool(part, width))

	case partText:
		// OpenCode TextPart: paddingLeft 3, marginTop 1
		// marginTop 1
		b.WriteString(lineStyle.Render("") + "\n")

		textLineStyle := lipgloss.NewStyle().
			Foreground(textColor).
			Background(bgColor).
			PaddingLeft(3).
			Width(width)

		lines := strings.Split(wrapText(part.Text, width-6), "\n")
		for _, line := range lines {
			b.WriteString(textLineStyle.Render(line) + "\n")
		}
	}

	return b.String()
}

func (m Model) renderAssistantTool(part chatPart, width int) string {
	// Inline tool row when output not yet available
	if part.ToolOutput == "" {
		icon := toolIcons[part.ToolName]
		if icon == "" {
			icon = "⊛"
		}

		toolHeader := fmt.Sprintf("%s %s", icon, part.ToolName)
		headerStyled := lipgloss.NewStyle().
			Foreground(textMutedColor).
			Render(toolHeader)

		desc := extractToolDescription(part.ToolInput)
		descStyled := ""
		if desc != "" {
			if len(desc) > width-12 {
				desc = desc[:width-15] + "..."
			}
			descStyled = lipgloss.NewStyle().Foreground(textMutedColor).Render(" " + desc)
		}

		toolLineStyle := lipgloss.NewStyle().
			Background(bgColor).
			PaddingLeft(6).
			Width(width)

		return toolLineStyle.Render(headerStyled+descStyled) + "\n"
	}

	// Block tool output for completed tool
	title := fmt.Sprintf("# %s", titlecaseLabel(part.ToolName))
	titleLine := lipgloss.NewStyle().
		Foreground(textColor).
		Background(bgElementColor).
		PaddingLeft(3).
		PaddingTop(1).
		PaddingBottom(1).
		Width(width).
		Render(title)

	bodyStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Background(bgElementColor).
		PaddingLeft(3).
		PaddingRight(1).
		Width(width)

	lines := wrapPreserveWhitespace(part.ToolOutput, width-6)
	var body strings.Builder
	for _, line := range lines {
		body.WriteString(bodyStyle.Render(line))
		body.WriteString("\n")
	}

	return titleLine + "\n" + strings.TrimRight(body.String(), "\n") + "\n"
}

func titlecaseLabel(input string) string {
	if input == "" {
		return input
	}
	runes := []rune(input)
	upper := []rune(strings.ToUpper(string(runes[0])))
	return string(upper) + string(runes[1:])
}

func wrapPreserveWhitespace(text string, width int) []string {
	if width <= 0 {
		return strings.Split(text, "\n")
	}

	var wrapped []string
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			wrapped = append(wrapped, "")
			continue
		}
		runes := []rune(line)
		for len(runes) > 0 {
			segmentWidth := 0
			cut := 0
			for cut < len(runes) {
				segmentWidth += runewidth.RuneWidth(runes[cut])
				if segmentWidth > width {
					break
				}
				cut++
			}
			if cut == 0 {
				cut = 1
			}
			wrapped = append(wrapped, string(runes[:cut]))
			runes = runes[cut:]
		}
	}

	return wrapped
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
		{"1-4", "jump"},
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

	switch m.activeTab {
	case tabChat:
		if m.chatInput.Focused() {
			items = []struct {
				key  string
				desc string
			}{
				{"enter", "newline"},
				{"ctrl+enter", "send"},
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
	case tabDaemon:
		items = append(items, struct {
			key  string
			desc string
		}{"j/k", "scroll"}, struct {
			key  string
			desc string
		}{"g/G", "top/btm"})
	case tabAgentOutput:
		items = append(items, struct {
			key  string
			desc string
		}{"j/k", "scroll"}, struct {
			key  string
			desc string
		}{"g/G", "top/btm"})
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
