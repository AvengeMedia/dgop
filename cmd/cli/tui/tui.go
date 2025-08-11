package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *ResponsiveTUIModel) Init() tea.Cmd {
	return tea.Batch(tick(), m.fetchData())
}

func (m *ResponsiveTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		
		tableHeight := m.height - 25
		if tableHeight < 10 {
			tableHeight = 10
		}
		m.processTable.SetHeight(tableHeight)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "d":
			m.showDetails = !m.showDetails
		case "up", "k":
			if !m.showDetails {
				m.selectedPID = -1
			}
			m.processTable, cmd = m.processTable.Update(msg)
			cmds = append(cmds, cmd)
			m.updateSelectedPID()
		case "down", "j":
			if !m.showDetails {
				m.selectedPID = -1
			}
			m.processTable, cmd = m.processTable.Update(msg)
			cmds = append(cmds, cmd)
			m.updateSelectedPID()
		default:
			m.processTable, cmd = m.processTable.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tickMsg:
		now := time.Now()
		
		if now.Sub(m.lastUpdate) >= 1*time.Second {
			cmds = append(cmds, m.fetchData())
		}
		
		if now.Sub(m.lastNetworkUpdate) >= 2*time.Second {
			cmds = append(cmds, m.fetchNetworkData())
		}
		
		if now.Sub(m.lastDiskUpdate) >= 2*time.Second {
			cmds = append(cmds, m.fetchDiskData())
		}
		
		cmds = append(cmds, tick())

	case fetchDataMsg:
		m.metrics = msg.metrics
		m.err = msg.err
		m.lastUpdate = time.Now()
		m.updateProcessTable()

	case fetchNetworkMsg:
		if msg.rates != nil && len(msg.rates.Interfaces) > 0 {
			m.lastNetworkUpdate = time.Now()
			m.networkCursor = msg.rates.Cursor
			
			for _, iface := range msg.rates.Interfaces {
				if iface.Interface == "lo" || strings.HasPrefix(iface.Interface, "docker") {
					continue
				}
				
				sample := NetworkSample{
					timestamp: time.Now(),
					rxBytes:   iface.RxTotal,
					txBytes:   iface.TxTotal,
					rxRate:    iface.RxRate,
					txRate:    iface.TxRate,
				}
				
				m.networkHistory = append(m.networkHistory, sample)
				if len(m.networkHistory) > m.maxNetHistory {
					m.networkHistory = m.networkHistory[1:]
				}
				break
			}
		}

	case fetchDiskMsg:
		if msg.rates != nil && len(msg.rates.Disks) > 0 {
			m.lastDiskUpdate = time.Now()
			m.diskCursor = msg.rates.Cursor
			
			for _, disk := range msg.rates.Disks {
				sample := DiskSample{
					timestamp:   time.Now(),
					readBytes:   disk.ReadTotal,
					writeBytes:  disk.WriteTotal,
					readRate:    disk.ReadRate,
					writeRate:   disk.WriteRate,
					device:      disk.Device,
				}
				
				m.diskHistory = append(m.diskHistory, sample)
				if len(m.diskHistory) > m.maxDiskHistory {
					m.diskHistory = m.diskHistory[1:]
				}
				break
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *ResponsiveTUIModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	title := fmt.Sprintf("dgop %s", getVersion())
	header := headerStyle.Width(m.width).Render(title)

	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth

	systemPanel := m.renderSystemInfoPanel(leftWidth)
	cpuPanel := m.renderCPUPanel(rightWidth)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, systemPanel, cpuPanel)

	memoryPanel := m.renderMemoryPanel(leftWidth)
	diskPanel := m.renderDiskPanel(rightWidth)

	middleRow := lipgloss.JoinHorizontal(lipgloss.Top, memoryPanel, diskPanel)

	diskChartPanel := m.renderDiskChart(rightWidth)

	diskRow := lipgloss.JoinHorizontal(lipgloss.Top, 
		strings.Repeat(" ", leftWidth), diskChartPanel)

	networkChartPanel := m.renderNetworkChart(leftWidth)
	networkRow := lipgloss.JoinHorizontal(lipgloss.Top, 
		networkChartPanel, strings.Repeat(" ", rightWidth))

	processPanel := panelStyle.Width(m.width).Render(m.processTable.View())

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		topRow,
		"",
		middleRow,
		"",
		diskRow,
		"",
		networkRow,
		"",
		processPanel,
	)

	if m.err != nil {
		content += fmt.Sprintf("\nError: %v", m.err)
	}

	return content
}

var Version = "dev"

func getVersion() string {
	return Version
}