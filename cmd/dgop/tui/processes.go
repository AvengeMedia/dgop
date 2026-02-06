package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/AvengeMedia/dgop/gops"
	"github.com/charmbracelet/bubbles/table"
)

func (m *ResponsiveTUIModel) updateProcessTable() {
	if m.metrics == nil || len(m.metrics.Processes) == 0 {
		return
	}

	columns := m.processTable.Columns()
	numCols := len(columns)
	var commandWidth, fullCommandWidth int

	switch {
	case numCols == 6:
		commandWidth = columns[4].Width
		fullCommandWidth = columns[5].Width
	case numCols > 4:
		commandWidth = columns[4].Width
	default:
		commandWidth = 30
	}

	rows := make([]table.Row, 0, len(m.metrics.Processes))
	selectedIndex := -1

	for i, proc := range m.metrics.Processes {
		if m.selectedPID > 0 && proc.PID == m.selectedPID {
			selectedIndex = i
		}

		memGB := float64(proc.MemoryKB) / 1048576
		var memStr string
		if memGB >= 1.0 {
			memStr = fmt.Sprintf("%.1f%% %.1fG", proc.MemoryPercent, memGB)
		} else {
			memStr = fmt.Sprintf("%.1f%% %.0fM", proc.MemoryPercent, memGB*1024)
		}

		var row table.Row
		switch numCols {
		case 6:
			row = table.Row{
				strconv.Itoa(int(proc.PID)),
				truncateString(proc.Username, 12),
				fmt.Sprintf("%.1f", proc.CPU),
				memStr,
				truncateString(proc.Command, commandWidth),
				truncateString(proc.FullCommand, fullCommandWidth),
			}
		default:
			row = table.Row{
				strconv.Itoa(int(proc.PID)),
				truncateString(proc.Username, 12),
				fmt.Sprintf("%.1f", proc.CPU),
				memStr,
				truncateString(proc.Command, commandWidth),
			}
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

func (m *ResponsiveTUIModel) sortProcessesLocally() {
	if m.metrics == nil || len(m.metrics.Processes) == 0 {
		return
	}

	processes := m.metrics.Processes

	switch m.sortBy {
	case gops.SortByCPU:
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].CPU > processes[j].CPU
		})
	case gops.SortByMemory:
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].MemoryKB > processes[j].MemoryKB
		})
	case gops.SortByName:
		sort.Slice(processes, func(i, j int) bool {
			return strings.ToLower(processes[i].Command) < strings.ToLower(processes[j].Command)
		})
	case gops.SortByPID:
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].PID < processes[j].PID
		})
	}

	m.metrics.Processes = processes
}
