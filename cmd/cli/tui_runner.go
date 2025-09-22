package main

import (
	"github.com/AvengeMedia/dgop/cmd/cli/tui"
	"github.com/AvengeMedia/dgop/gops"
	tea "github.com/charmbracelet/bubbletea"
)

func runTUI(gopsUtil *gops.GopsUtil) error {
	return runTUIWithOptions(gopsUtil, false, false)
}

func runTUIWithOptions(gopsUtil *gops.GopsUtil, hideCPUCores, summarizeCores bool) error {
	tui.Version = Version
	model := tui.NewResponsiveTUIModelWithOptions(gopsUtil, hideCPUCores, summarizeCores)
	defer model.Cleanup()

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}
