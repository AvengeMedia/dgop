package tui

import "github.com/charmbracelet/lipgloss"

var (
	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(1)

	progressBarStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#8B5FBF"))

	progressBarUsedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#7D56F4"))

	textStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C9C9C9"))

	boldTextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Bold(true)
)