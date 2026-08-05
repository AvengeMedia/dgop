package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/AvengeMedia/dgop/gops"
	"github.com/AvengeMedia/dgop/models"
	"github.com/charmbracelet/bubbles/table"
)

func (m *ResponsiveTUIModel) activeSearchQuery() string {
	if m.searchActive {
		return m.searchInput
	}
	return m.searchQuery
}

func (m *ResponsiveTUIModel) visibleProcesses() []*models.ProcessInfo {
	if m.metrics == nil {
		return nil
	}

	query := strings.ToLower(m.activeSearchQuery())
	if query == "" {
		return m.metrics.Processes
	}

	filtered := make([]*models.ProcessInfo, 0, len(m.metrics.Processes))
	for _, proc := range m.metrics.Processes {
		if strings.Contains(strings.ToLower(proc.Command), query) ||
			strings.Contains(strings.ToLower(proc.FullCommand), query) {
			filtered = append(filtered, proc)
		}
	}
	return filtered
}

func (m *ResponsiveTUIModel) moveProcessCursor(delta int) {
	oldCursor := m.processTable.Cursor()
	if delta < 0 {
		m.processTable.MoveUp(-delta)
	} else {
		m.processTable.MoveDown(delta)
	}

	newCursor := m.processTable.Cursor()
	if newCursor == oldCursor {
		return
	}

	visible := m.visibleProcesses()
	if newCursor >= len(visible) {
		return
	}
	m.selectedPID = visible[newCursor].PID
}

func (m *ResponsiveTUIModel) updateProcessTable() {
	if m.metrics == nil {
		return
	}

	processes := m.visibleProcesses()

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

	rows := make([]table.Row, 0, len(processes))
	selectedIndex := -1

	for i, proc := range processes {
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

	if len(rows) == 0 {
		m.processTable.SetCursor(0)
		return
	}

	switch {
	case selectedIndex >= 0:
		m.processTable.SetCursor(selectedIndex)
	case m.selectedPID == -1:
		m.processTable.SetCursor(0)
	case m.processTable.Cursor() >= len(rows):
		m.processTable.SetCursor(len(rows) - 1)
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
