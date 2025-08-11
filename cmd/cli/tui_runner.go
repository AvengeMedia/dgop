package main

import (
	"github.com/AvengeMedia/dgop/cmd/cli/tui"
	"github.com/AvengeMedia/dgop/gops"
	tea "github.com/charmbracelet/bubbletea"
)

func runTUI(gopsUtil *gops.GopsUtil) error {
	tui.Version = Version
	model := tui.NewResponsiveTUIModel(gopsUtil)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}
