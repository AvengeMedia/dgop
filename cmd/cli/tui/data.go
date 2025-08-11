package tui

import (
	"github.com/AvengeMedia/dgop/gops"
	"github.com/AvengeMedia/dgop/models"
	tea "github.com/charmbracelet/bubbletea"
)

type fetchDataMsg struct {
	metrics *models.SystemMetrics
	err     error
}

type fetchNetworkMsg struct {
	rates *models.NetworkRateResponse
	err   error
}

type fetchDiskMsg struct {
	rates *models.DiskRateResponse
	err   error
}

func (m *ResponsiveTUIModel) fetchData() tea.Cmd {
	return func() tea.Msg {
		params := gops.MetaParams{
			SortBy:    m.sortBy,
			ProcLimit: m.procLimit,
			EnableCPU: true,
		}

		modules := []string{"cpu", "memory", "system", "network", "disk", "processes"}
		metrics, err := m.gops.GetMeta(modules, params)

		if err != nil {
			return fetchDataMsg{err: err}
		}

		systemMetrics := &models.SystemMetrics{
			CPU:        metrics.CPU,
			Memory:     metrics.Memory,
			System:     metrics.System,
			Network:    metrics.Network,
			Disk:       metrics.Disk,
			DiskMounts: metrics.DiskMounts,
			Processes:  metrics.Processes,
		}

		return fetchDataMsg{metrics: systemMetrics, err: nil}
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
