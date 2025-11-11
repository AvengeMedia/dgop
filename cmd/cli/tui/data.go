package tui

import (
	"github.com/AvengeMedia/dgop/gops"
	"github.com/AvengeMedia/dgop/models"
	tea "github.com/charmbracelet/bubbletea"
)

type fetchDataMsg struct {
	metrics    *models.SystemMetrics
	err        error
	generation int
	cpuCursor  string
	procCursor string
}

type fetchNetworkMsg struct {
	rates *models.NetworkRateResponse
	err   error
}

type fetchDiskMsg struct {
	rates *models.DiskRateResponse
	err   error
}

type fetchTempMsg struct {
	temps []models.TemperatureSensor
	err   error
}

func (m *ResponsiveTUIModel) fetchData() tea.Cmd {
	generation := m.fetchGeneration
	cpuCursor := m.cpuCursor
	procCursor := m.procCursor
	sortBy := m.sortBy
	procLimit := m.procLimit
	return func() tea.Msg {
		params := gops.MetaParams{
			SortBy:     sortBy,
			ProcLimit:  procLimit,
			EnableCPU:  true,
			CPUCursor:  cpuCursor,
			ProcCursor: procCursor,
		}

		modules := []string{"cpu", "memory", "system", "network", "disk", "processes"}
		metrics, err := m.gops.GetMeta(modules, params)

		if err != nil {
			return fetchDataMsg{err: err, generation: generation}
		}

		systemMetrics := &models.SystemMetrics{
			CPU:        metrics.CPU,
			Memory:     metrics.Memory,
			System:     metrics.System,
			Network:    metrics.Network,
			Disk:       metrics.Disk,
			DiskMounts: nil,
			Processes:  metrics.Processes,
		}

		newCPUCursor := ""
		if metrics.CPU != nil {
			newCPUCursor = metrics.CPU.Cursor
		}

		return fetchDataMsg{
			metrics:    systemMetrics,
			err:        nil,
			generation: generation,
			cpuCursor:  newCPUCursor,
			procCursor: metrics.Cursor,
		}
	}
}

func (m *ResponsiveTUIModel) fetchNetworkData() tea.Cmd {
	return func() tea.Msg {
		rates, err := m.gops.GetNetworkRates(m.networkCursor)
		return fetchNetworkMsg{rates: rates, err: err}
	}
}

func (m *ResponsiveTUIModel) fetchDiskData() tea.Cmd {
	return func() tea.Msg {
		rates, err := m.gops.GetDiskRates(m.diskCursor)
		return fetchDiskMsg{rates: rates, err: err}
	}
}

func (m *ResponsiveTUIModel) fetchTemperatureData() tea.Cmd {
	return func() tea.Msg {
		temps, err := m.gops.GetSystemTemperatures()
		return fetchTempMsg{temps: temps, err: err}
	}
}
