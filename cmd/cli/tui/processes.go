package tui

import (
	"fmt"
	"strconv"

	"github.com/AvengeMedia/dgop/models"
	"github.com/charmbracelet/bubbles/table"
)

func (m *ResponsiveTUIModel) updateProcessTable() {
	if m.metrics == nil || len(m.metrics.Processes) == 0 {
		return
	}

	var rows []table.Row
	selectedIndex := -1

	for i, proc := range m.metrics.Processes {
		if m.selectedPID > 0 && proc.PID == m.selectedPID {
			selectedIndex = i
		}

		row := table.Row{
			strconv.Itoa(int(proc.PID)),
			fmt.Sprintf("%.1f", proc.CPU),
			fmt.Sprintf("%.1f", proc.MemoryPercent),
			truncateString(proc.Command, 25),
			truncateString(proc.FullCommand, 35),
		}
		rows = append(rows, row)
	}

	m.processTable.SetRows(rows)

	if selectedIndex >= 0 {
		m.processTable.SetCursor(selectedIndex)
	} else if m.selectedPID == -1 {
		m.processTable.SetCursor(0)
	}
}

func (m *ResponsiveTUIModel) getSelectedProcess() *models.ProcessInfo {
	if m.metrics == nil || len(m.metrics.Processes) == 0 {
		return nil
	}

	cursor := m.processTable.Cursor()
	if cursor >= 0 && cursor < len(m.metrics.Processes) {
		return m.metrics.Processes[cursor]
	}

	return nil
}

func (m *ResponsiveTUIModel) updateSelectedPID() {
	if selectedProc := m.getSelectedProcess(); selectedProc != nil {
		m.selectedPID = selectedProc.PID
	}
}
