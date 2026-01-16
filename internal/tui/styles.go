package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	primaryColor   = lipgloss.Color("#7C3AED")
	secondaryColor = lipgloss.Color("#10B981")
	mutedColor     = lipgloss.Color("#6B7280")
	errorColor     = lipgloss.Color("#EF4444")
)

// Tab styles
var (
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Padding(0, 2)

	tabBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(mutedColor)
)

// Content styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	contentStyle = lipgloss.NewStyle().
			Padding(1, 2)
)
