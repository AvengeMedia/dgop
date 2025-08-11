package tui

import "github.com/charmbracelet/lipgloss"

var (
	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(0, 1)

	textStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C9C9C9"))

	boldTextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Bold(true)
)