package tui

import "github.com/charmbracelet/lipgloss"

// OpenCode theme colors (dark mode)
var (
	// Background scale
	bgColor             = lipgloss.Color("#0a0a0a") // darkStep1 - main background
	bgPanelColor        = lipgloss.Color("#141414") // darkStep2 - panel background
	bgElementColor      = lipgloss.Color("#1e1e1e") // darkStep3 - element background
	bgStep4             = lipgloss.Color("#282828") // darkStep4
	bgStep5             = lipgloss.Color("#323232") // darkStep5
	backgroundMenuColor = lipgloss.Color("#1e1e1e") // darkStep3 - menu background

	// Border colors
	borderSubtleColor = lipgloss.Color("#3c3c3c") // darkStep6
	borderColor       = lipgloss.Color("#484848") // darkStep7
	borderActiveColor = lipgloss.Color("#606060") // darkStep8

	// Primary/Accent colors
	primaryColor   = lipgloss.Color("#fab283") // darkStep9 - warm peach/orange
	primaryBright  = lipgloss.Color("#ffc09f") // darkStep10
	secondaryColor = lipgloss.Color("#5c9cf5") // blue
	accentColor    = lipgloss.Color("#9d7cd8") // purple

	// Semantic colors
	errorColor   = lipgloss.Color("#e06c75") // red
	warningColor = lipgloss.Color("#f5a742") // orange
	successColor = lipgloss.Color("#7fd88f") // green
	infoColor    = lipgloss.Color("#56b6c2") // cyan
	yellowColor  = lipgloss.Color("#e5c07b") // yellow

	// Text colors
	textColor      = lipgloss.Color("#eeeeee") // darkStep12 - primary text
	textMutedColor = lipgloss.Color("#808080") // darkStep11 - muted text
)

// Base style with background - all styles inherit from this
var baseStyle = lipgloss.NewStyle().Background(bgColor)

// Logo style
var logoStyle = baseStyle.
	Foreground(textColor).
	Bold(true)

var logoDimStyle = baseStyle.
	Foreground(textMutedColor)

// Base style for panel content (uses panel background)
var panelBaseStyle = lipgloss.NewStyle().Background(bgPanelColor)

// Tab styles (inside panel, so use panel background)
var (
	activeTabStyle = panelBaseStyle.
			Foreground(primaryColor).
			Bold(true).
			Padding(0, 2)

	inactiveTabStyle = panelBaseStyle.
				Foreground(textMutedColor).
				Padding(0, 2)

	tabBarStyle = baseStyle.
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(borderSubtleColor)
)

// Content styles
var (
	titleStyle = baseStyle.
			Foreground(textColor).
			Bold(true)

	statusStyle = baseStyle.
			Foreground(successColor)

	statusInactiveStyle = baseStyle.
				Foreground(textMutedColor)

	errorStyle = baseStyle.
			Foreground(errorColor)

	helpStyle = baseStyle.
			Foreground(textMutedColor)

	mutedStyle = baseStyle.
			Foreground(textMutedColor)

	contentStyle = baseStyle.
			Padding(1, 2)

	// Card/panel style
	panelStyle = lipgloss.NewStyle().
			Background(bgPanelColor).
			Padding(1, 2).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderSubtleColor)

	// Label styles - use primary (peach) for labels
	labelStyle = baseStyle.
			Foreground(primaryColor).
			Bold(true)

	valueStyle = baseStyle.
			Foreground(textMutedColor)

	// Key hint style (for showing keyboard shortcuts)
	keyStyle = baseStyle.
			Foreground(textColor).
			Bold(true)

	keyDescStyle = baseStyle.
			Foreground(textMutedColor)

	// Selector bar style - dark box with blue left accent (like OpenCode's input bar)
	selectorBarStyle = lipgloss.NewStyle().
				Background(bgPanelColor).
				Padding(1, 2)

	// Blue left accent bar (rendered separately)
	accentBarStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Background(bgPanelColor)

	// Sidebar header style
	sidebarHeaderStyle = baseStyle.
				Foreground(primaryColor).
				Bold(true)
)
