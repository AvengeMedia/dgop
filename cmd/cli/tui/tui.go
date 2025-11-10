package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/AvengeMedia/dgop/gops"
	"github.com/AvengeMedia/dgop/models"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var Version = "dev"

func (m *ResponsiveTUIModel) Init() tea.Cmd {
	cmds := []tea.Cmd{tick(), m.fetchData(), m.fetchTemperatureData()}

	if m.colorManager != nil {
		cmds = append(cmds, m.listenForColorChanges())
	}

	return tea.Batch(cmds...)
}

func (m *ResponsiveTUIModel) listenForColorChanges() tea.Cmd {
	return func() tea.Msg {
		<-m.colorManager.ColorChanges()
		return colorUpdateMsg{}
	}
}

func (m *ResponsiveTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			return m, m.fetchData()
		case "d":
			m.showDetails = !m.showDetails
		case "c":
			m.sortBy = gops.SortByCPU
			return m, m.fetchData()
		case "m":
			m.sortBy = gops.SortByMemory
			return m, m.fetchData()
		case "n":
			m.sortBy = gops.SortByName
			return m, m.fetchData()
		case "p":
			m.sortBy = gops.SortByPID
			return m, m.fetchData()
		case "up", "k":
			oldCursor := m.processTable.Cursor()
			m.processTable, cmd = m.processTable.Update(msg)
			cmds = append(cmds, cmd)

			// Update selected PID when cursor moves
			newCursor := m.processTable.Cursor()
			if oldCursor != newCursor && m.metrics != nil && len(m.metrics.Processes) > newCursor {
				m.selectedPID = m.metrics.Processes[newCursor].PID
			}
		case "down", "j":
			oldCursor := m.processTable.Cursor()
			m.processTable, cmd = m.processTable.Update(msg)
			cmds = append(cmds, cmd)

			// Update selected PID when cursor moves
			newCursor := m.processTable.Cursor()
			if oldCursor != newCursor && m.metrics != nil && len(m.metrics.Processes) > newCursor {
				m.selectedPID = m.metrics.Processes[newCursor].PID
			}
		default:
			m.processTable, cmd = m.processTable.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tickMsg:
		cmds = append(cmds, tick())

		now := time.Now()

		// Update main metrics every second
		if now.Sub(m.lastUpdate) >= 1*time.Second {
			cmds = append(cmds, m.fetchData())
		}

		// Update network rates every 2 seconds
		if now.Sub(m.lastNetworkUpdate) >= 2*time.Second {
			cmds = append(cmds, m.fetchNetworkData())
			m.lastNetworkUpdate = now
		}

		// Update disk rates every 2 seconds
		if now.Sub(m.lastDiskUpdate) >= 2*time.Second {
			cmds = append(cmds, m.fetchDiskData())
			m.lastDiskUpdate = now
		}

		// Update temperatures every 10 seconds
		if now.Sub(m.lastTempUpdate) >= 10*time.Second {
			cmds = append(cmds, m.fetchTemperatureData())
			m.lastTempUpdate = now
		}

		// Logo cycling for testing - cycle every 3 seconds
		if m.logoTestMode && now.Sub(m.lastLogoUpdate) >= 3*time.Second {
			allLogos := getAllDistroLogos()
			m.currentLogoIndex = (m.currentLogoIndex + 1) % len(allLogos)
			currentLogo := allLogos[m.currentLogoIndex]
			m.distroLogo = currentLogo.logo
			m.distroColor = currentLogo.color
			// Update the hardware distro name to show which logo we're displaying
			if m.hardware != nil {
				m.hardware.Distro = currentLogo.name
			}
			m.lastLogoUpdate = now
		}

	case fetchDataMsg:
		m.metrics = msg.metrics
		m.err = msg.err
		m.lastUpdate = time.Now()
		m.updateProcessTable()

	case fetchNetworkMsg:
		if msg.rates != nil && len(msg.rates.Interfaces) > 0 {
			m.networkCursor = msg.rates.Cursor

			bestInterface := m.selectBestNetworkInterface(msg.rates.Interfaces)
			if bestInterface != nil {
				m.selectedInterfaceName = bestInterface.Interface

				sample := NetworkSample{
					timestamp: time.Now(),
					rxBytes:   bestInterface.RxTotal,
					txBytes:   bestInterface.TxTotal,
					rxRate:    bestInterface.RxRate,
					txRate:    bestInterface.TxRate,
				}

				m.networkHistory = append(m.networkHistory, sample)
				if len(m.networkHistory) > m.maxNetHistory {
					m.networkHistory = m.networkHistory[1:]
				}
			}
		}

	case fetchDiskMsg:
		// Force some data into history to test rendering
		if len(m.diskHistory) == 0 {
			// Add some initial samples to get started
			for i := 0; i < 5; i++ {
				m.diskHistory = append(m.diskHistory, DiskSample{
					timestamp:  time.Now().Add(time.Duration(-i) * time.Second),
					readBytes:  uint64(i * 1000000),
					writeBytes: uint64(i * 2000000),
					readRate:   float64(i * 100000),
					writeRate:  float64(i * 200000),
					device:     "test",
				})
			}
		}

		if msg.rates != nil && len(msg.rates.Disks) > 0 {
			m.diskCursor = msg.rates.Cursor

			// Aggregate all disk rates
			var totalReadRate, totalWriteRate float64
			var totalReadBytes, totalWriteBytes uint64

			for _, disk := range msg.rates.Disks {
				totalReadRate += disk.ReadRate
				totalWriteRate += disk.WriteRate
				totalReadBytes += disk.ReadTotal
				totalWriteBytes += disk.WriteTotal
			}

			sample := DiskSample{
				timestamp:  time.Now(),
				readBytes:  totalReadBytes,
				writeBytes: totalWriteBytes,
				readRate:   totalReadRate,
				writeRate:  totalWriteRate,
				device:     "total",
			}

			m.diskHistory = append(m.diskHistory, sample)
			if len(m.diskHistory) > m.maxDiskHistory {
				m.diskHistory = m.diskHistory[1:]
			}
		}

	case fetchTempMsg:
		if msg.err == nil {
			m.systemTemperatures = msg.temps
		}

	case colorUpdateMsg:
		m.updateTableStyles()
		cmds = append(cmds, m.listenForColorChanges())
	}

	return m, tea.Batch(cmds...)
}

func (m *ResponsiveTUIModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	return m.renderLayout()
}

func (m *ResponsiveTUIModel) renderLayout() string {
	// Pre-render header and footer to measure their heights
	header := m.renderHeader()
	footer := m.renderFooter()

	var sections []string
	sections = append(sections, header)

	// Main content gets exact remaining space
	mainContent := m.renderMainContent()

	// Ensure main content doesn't exceed available space
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	maxMainHeight := m.height - headerHeight - footerHeight

	if maxMainHeight > 0 {
		mainContent = lipgloss.NewStyle().MaxHeight(maxMainHeight).Render(mainContent)
	}

	sections = append(sections, mainContent)
	sections = append(sections, footer)

	return strings.Join(sections, "\n")
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

type panelSpec struct{ min, max, weight int }

// shrink-aware allocator: shrinks when needed, grows by weight within limits
func allocCapped(total int, specs []panelSpec, floor int, shrinkOrder []int) []int {
	out := make([]int, len(specs))
	sum := 0
	for i, s := range specs {
		out[i] = s.min
		sum += s.min
	}

	// If too big, shrink to fit (never below floor)
	for sum > total {
		shrunk := false
		for _, i := range shrinkOrder {
			if out[i] > floor {
				out[i]--
				sum--
				shrunk = true
				if sum == total {
					break
				}
			}
		}
		if !shrunk {
			break // nothing left to shrink
		}
	}

	// If room left, grow toward max by weight
	if sum < total {
		rem := total - sum
		for rem > 0 {
			progressed := false
			for i, s := range specs {
				if out[i] < s.max && s.weight > 0 {
					out[i]++
					rem--
					progressed = true
					if rem == 0 {
						break
					}
				}
			}
			if !progressed {
				break
			}
		}
	}
	return out
}

func (m *ResponsiveTUIModel) minSystemLines(width int) int {
	// distro, user@hostname, kernel, bios maker, bios version+date, CPU count, uptime - 7 lines
	// accommodate logos up to 9 lines (like fedora) with vertical centering
	return 9 // increased to accommodate 9-line logos
}

func (m *ResponsiveTUIModel) minCPULines(width int) int {
	// title + usage bar
	lines := 2

	// core rows - depends on hide/summarize options
	if m.metrics != nil && m.metrics.CPU != nil && !m.hideCPUCores {
		cores := len(m.metrics.CPU.CoreUsage)
		if cores > 0 {
			if m.summarizeCores {
				// Summarized mode: much fewer lines
				groupSize := 8
				if cores > 64 {
					groupSize = 16
				}
				groups := (cores + groupSize - 1) / groupSize
				lines += groups + 1 // +1 for summary line
			} else {
				// Original detailed mode: 3 cores per row
				rows := (cores + 2) / 3
				lines += rows
			}
		}
	}

	// system info line under cores if System present
	if m.metrics != nil && m.metrics.System != nil {
		lines += 1
	}

	return lines
}

func (m *ResponsiveTUIModel) minMemDiskLines(width int) int {
	// MEMORY header + bars + numbers
	lines := 3 // header + bar + size info
	if m.metrics != nil && m.metrics.Memory != nil && m.metrics.Memory.SwapTotal > 0 {
		lines += 2 // swap bar + size info
	}
	// DISK header + at least 2 disks (2 lines each)
	lines += 1 + 4 // blank + header + 2 disks
	// disk I/O rates when history exists
	if len(m.diskHistory) > 0 {
		lines += 2 // blank + rates
	}
	// sensors block if present
	if len(m.systemTemperatures) > 0 {
		sensorsToShow := len(m.systemTemperatures)
		if sensorsToShow > 6 {
			sensorsToShow = 6
		}
		lines += 2 + sensorsToShow // blank + header + sensors
	}
	return lines
}

func (m *ResponsiveTUIModel) minNetworkLines(width int) int {
	// header + rates + chart + totals
	// Give network more height to balance with right column
	return 12 // increased further to match processes better
}

func (m *ResponsiveTUIModel) renderMainContent() string {
	// Calculate layout dimensions with cushioned right width
	leftWidth := m.width * 40 / 100 // 40% for left panels
	spacer := 1
	rightWidth := m.width - leftWidth - spacer - 4 // 4-col cushion to ensure right border visible
	if rightWidth < 10 {
		rightWidth = 10 // safety
	}

	// DYNAMIC HEIGHT CALCULATION - measure header/footer first
	header := m.renderHeader()
	footer := m.renderFooter()
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)

	// Available height = total - header - footer (remove extra -2)
	availableHeight := m.height - headerHeight - footerHeight
	if availableHeight < 8 {
		availableHeight = 8
	}

	// Chrome calculation (full borders only - gaps are rendered but not budgeted)
	leftPanels := 3
	rightPanels := 2
	if m.showDetails {
		rightPanels = 3
	}

	leftChrome := leftPanels * 2 // full borders only
	rightChrome := rightPanels * 2

	leftInnerTotal := availableHeight - leftChrome
	rightInnerTotal := availableHeight - rightChrome
	if leftInnerTotal < 3 {
		leftInnerTotal = 3
	}
	if rightInnerTotal < 3 {
		rightInnerTotal = 3
	}

	// Left column: System (exact content), Mem/Disk (realistic), Network (tight)
	sysMin := m.minSystemLines(leftWidth)
	sysMax := sysMin // exact content only

	memDiskMin := m.minMemDiskLines(leftWidth)
	memDiskMax := 999 // gets the slack

	netMin := m.minNetworkLines(leftWidth)
	netMax := netMin + 8 // give network flex to fill space

	leftSpecs := []panelSpec{
		{sysMin, sysMax, 0},         // System: no flex
		{memDiskMin, memDiskMax, 3}, // Mem/Disk: medium weight
		{netMin, netMax, 5},         // Network: highest weight to fill space
	}
	leftShrinkOrder := []int{2, 1, 0} // net→memdisk→system
	leftInner := allocCapped(leftInnerTotal, leftSpecs, 3, leftShrinkOrder)
	leftHeights := []int{leftInner[0] + 2, leftInner[1] + 2, leftInner[2] + 2}

	// Right column: CPU (exact content), Processes (main flex)
	cpuMin := m.minCPULines(rightWidth)
	cpuMax := cpuMin // no empty space, content only

	procMin := 6   // reduced to balance left/right columns
	procMax := 999 // main flex sink
	detMin := 5
	detMax := 24

	var rightHeights []int
	if m.showDetails {
		rightSpecs := []panelSpec{
			{cpuMin, cpuMax, 0},   // CPU: no flex
			{procMin, procMax, 3}, // Processes: main flex
			{detMin, detMax, 1},   // Details: light flex
		}
		rightShrinkOrder := []int{2, 1, 0} // details→processes→cpu
		rightInner := allocCapped(rightInnerTotal, rightSpecs, 3, rightShrinkOrder)
		rightHeights = []int{rightInner[0] + 2, rightInner[1] + 2, rightInner[2] + 2}
	} else {
		rightSpecs := []panelSpec{
			{cpuMin, cpuMax, 0},   // CPU: no flex
			{procMin, procMax, 5}, // processes: main flex sink
		}
		rightShrinkOrder := []int{1, 0} // processes→cpu
		rightInner := allocCapped(rightInnerTotal, rightSpecs, 3, rightShrinkOrder)
		rightHeights = []int{rightInner[0] + 2, rightInner[1] + 2}
	}

	// Render panels with exact allocated heights
	systemPanel := m.renderSystemInfoPanel(leftWidth, leftHeights[0])
	memDiskPanel := m.renderMemDiskPanel(leftWidth, leftHeights[1])
	networkPanel := m.renderNetworkPanel(leftWidth, leftHeights[2])

	cpuPanel := m.renderCPUPanel(rightWidth, rightHeights[0])
	var processColumn string
	if m.showDetails {
		processPanel := m.renderProcessPanel(rightWidth, rightHeights[1])
		detailsPanel := m.renderProcessDetailsPanel(rightWidth, rightHeights[2])

		// Stack with borders only
		processColumn = lipgloss.JoinVertical(lipgloss.Left, processPanel, detailsPanel)
	} else {
		// Processes get ALL the available space
		processPanel := m.renderProcessPanel(rightWidth, rightHeights[1])
		processColumn = processPanel
	}

	leftColumn := lipgloss.JoinVertical(lipgloss.Left, systemPanel, memDiskPanel, networkPanel)
	rightColumn := lipgloss.JoinVertical(lipgloss.Left, cpuPanel, processColumn)

	// Join the two complete columns with spacer
	spacerCol := lipgloss.NewStyle().Width(spacer).Render(" ")
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, spacerCol, rightColumn)

	return mainContent
}

func (m *ResponsiveTUIModel) renderHeader() string {
	style := m.headerStyle()

	// Just show current time in header
	currentTime := time.Now().Format("15:04:05")
	rightText := currentTime

	title := fmt.Sprintf("dgop %s", Version)
	// rightText already set above
	spaces := m.width - len(title) - len(rightText) - 4
	if spaces < 0 {
		spaces = 0
	}
	headerText := fmt.Sprintf("%s%s%s", title, strings.Repeat(" ", spaces), rightText)

	return style.Render(headerText)
}

func (m *ResponsiveTUIModel) renderFooter() string {
	style := m.footerStyle()

	controls := "Controls: [q]uit [r]efresh [d]etails | Sort: [c]pu [m]emory [n]ame [p]id | ↑↓ Navigate"
	return style.Render(controls)
}

func (m *ResponsiveTUIModel) renderProcessPanel(width, height int) string {
	style := m.panelStyle(width, height)

	var content strings.Builder

	// Sort indicator
	sortIndicator := ""
	switch m.sortBy {
	case gops.SortByCPU:
		sortIndicator = " ↓CPU"
	case gops.SortByMemory:
		sortIndicator = " ↓MEM"
	case gops.SortByName:
		sortIndicator = " ↓NAME"
	case gops.SortByPID:
		sortIndicator = " ↓PID"
	}

	processCount := 0
	if m.metrics != nil {
		processCount = len(m.metrics.Processes)
	}

	title := fmt.Sprintf("PROCESSES (%d)%s", processCount, sortIndicator)
	titleStyle := m.titleStyle()

	content.WriteString(titleStyle.Render(title) + "\n")

	// Update table dimensions and column widths for this panel
	tableHeight := height - 3 // 2 borders + 1 title line
	if tableHeight < 3 {
		tableHeight = 3 // Never exceed container by forcing 8
	}

	// Update process table for this panel
	m.updateProcessColumnWidthsForPanel(width - 4)
	m.processTable.SetHeight(tableHeight)

	content.WriteString(m.processTable.View())

	return style.Render(content.String())
}

func (m *ResponsiveTUIModel) renderProcessDetailsPanel(width, height int) string {
	style := m.panelStyle(width, height)

	var content strings.Builder

	title := "PROCESS DETAILS"
	titleStyle := m.titleStyle()

	content.WriteString(titleStyle.Render(title) + "\n")

	if m.metrics != nil && len(m.metrics.Processes) > 0 {
		selectedIdx := m.processTable.Cursor()
		if selectedIdx < len(m.metrics.Processes) {
			proc := m.metrics.Processes[selectedIdx]

			content.WriteString(fmt.Sprintf("PID: %d\n", proc.PID))
			content.WriteString(fmt.Sprintf("PPID: %d\n", proc.PPID))
			content.WriteString(fmt.Sprintf("USER: %s\n", proc.Username))
			content.WriteString(fmt.Sprintf("CPU: %.1f%%\n", proc.CPU))
			memGB := float64(proc.MemoryKB) / 1024 / 1024
			if memGB >= 1.0 {
				content.WriteString(fmt.Sprintf("Memory: %.1f%% (%.1f GB)\n", proc.MemoryPercent, memGB))
			} else {
				content.WriteString(fmt.Sprintf("Memory: %.1f%% (%.0f MB)\n", proc.MemoryPercent, memGB*1024))
			}
			content.WriteString(fmt.Sprintf("Command: %s\n", proc.Command))

			// Show full command with word wrapping
			maxWidth := width - 6
			if len(proc.FullCommand) > maxWidth {
				content.WriteString("Full Command:\n")
				words := strings.Fields(proc.FullCommand)
				currentLine := ""
				for _, word := range words {
					if len(currentLine)+len(word)+1 > maxWidth {
						if currentLine != "" {
							content.WriteString(currentLine + "\n")
							currentLine = word
						} else {
							content.WriteString(word[:maxWidth-3] + "...\n")
						}
					} else {
						if currentLine != "" {
							currentLine += " "
						}
						currentLine += word
					}
				}
				if currentLine != "" {
					content.WriteString(currentLine)
				}
			} else {
				content.WriteString(fmt.Sprintf("Full Command: %s", proc.FullCommand))
			}
		} else {
			content.WriteString("No process selected")
		}
	} else {
		content.WriteString("Loading process data...")
	}

	return style.Render(content.String())
}

func (m *ResponsiveTUIModel) renderNetworkPanel(width, height int) string {
	style := m.panelStyle(width, height)

	var content strings.Builder

	interfaceName := "NETWORK"
	if len(m.networkHistory) > 0 {
		interfaceName = m.getSelectedInterfaceName()
	} else if m.metrics != nil && len(m.metrics.Network) > 0 {
		interfaceName = m.metrics.Network[0].Name
	}

	content.WriteString(m.titleStyle().Render(interfaceName) + "\n")

	innerHeight := height - 2

	if len(m.networkHistory) == 0 {
		content.WriteString("Loading...")
		// Pad to fill height even when loading
		contentStr := content.String()
		lines := strings.Split(contentStr, "\n")
		for len(lines) < innerHeight {
			lines = append(lines, "")
		}
		return style.Render(strings.Join(lines, "\n"))
	}

	// Get latest rates
	latest := m.networkHistory[len(m.networkHistory)-1]

	// Format rates in human readable format
	rxRateStr := m.formatBytes(uint64(latest.rxRate))
	txRateStr := m.formatBytes(uint64(latest.txRate))

	content.WriteString(fmt.Sprintf("↓%s/s ↑%s/s\n", rxRateStr, txRateStr))

	// Build totals line first to know exact space needed
	totalRx := m.formatBytes(latest.rxBytes)
	totalTx := m.formatBytes(latest.txBytes)
	bottomLine := fmt.Sprintf("RX: %s TX: %s", totalRx, totalTx)
	if len(bottomLine) > width-4 {
		bottomLine = m.truncate(bottomLine, width-4)
	}

	// Calculate exact chart height to fill space
	used := lipgloss.Height(content.String())
	remaining := innerHeight - used - 1 // -1 for the totals line
	chartHeight := remaining
	if chartHeight < 1 {
		chartHeight = 1
	}

	// Render chart to fill exact space
	if chartHeight > 0 {
		content.WriteString(m.renderSplitNetworkGraph(m.networkHistory, width-2, chartHeight))
	}

	// Add totals at bottom
	content.WriteString("\n" + bottomLine)

	// Ensure content exactly fills inner height to prevent shrinking
	contentStr := content.String()
	contentHeight := lipgloss.Height(contentStr)

	// Add padding to reach exactly innerHeight lines
	if contentHeight < innerHeight {
		padding := strings.Repeat("\n", innerHeight-contentHeight)
		contentStr = contentStr + padding
	} else if contentHeight > innerHeight {
		// If too tall, truncate to fit
		lines := strings.Split(contentStr, "\n")
		contentStr = strings.Join(lines[:innerHeight], "\n")
	}

	return style.Render(contentStr)
}

func (m *ResponsiveTUIModel) updateProcessColumnWidthsForPanel(totalWidth int) {
	// Calculate column widths with dynamic 5th column
	bordersPadding := 16 // Increased padding for safety
	availableWidth := totalWidth - bordersPadding

	// Define minimum widths that can shrink if needed
	pidWidth := 5
	userWidth := 6
	cpuWidth := 5
	memWidth := 13

	// If space is really tight, shrink fixed columns further
	fixedColumnsWidth := pidWidth + userWidth + cpuWidth + memWidth
	if availableWidth < fixedColumnsWidth+10 {
		// Emergency shrink mode
		pidWidth = 5
		userWidth = 6
		cpuWidth = 5
		memWidth = 11
		fixedColumnsWidth = pidWidth + userWidth + cpuWidth + memWidth
	}

	// Check if we have enough space for 5th column (FULL COMMAND)
	minCommandWidth := 15
	minFullCommandWidth := 20
	remainingWidth := availableWidth - fixedColumnsWidth

	var columns []table.Column
	if remainingWidth >= minCommandWidth+minFullCommandWidth+2 { // +2 for spacing
		// 5-column layout with separate COMMAND and FULL COMMAND
		commandWidth := minCommandWidth
		fullCommandWidth := remainingWidth - commandWidth
		if fullCommandWidth > 60 {
			fullCommandWidth = 60 // reasonable max
			commandWidth = remainingWidth - fullCommandWidth
		}

		columns = []table.Column{
			{Title: "PID", Width: pidWidth},
			{Title: "USER", Width: userWidth},
			{Title: "CPU%", Width: cpuWidth},
			{Title: "MEM%", Width: memWidth},
			{Title: "COMMAND", Width: commandWidth},
			{Title: "FULL COMMAND", Width: fullCommandWidth},
		}
	} else {
		// 4-column layout (original)
		commandWidth := remainingWidth
		if commandWidth < 8 {
			commandWidth = 8 // Absolute minimum
		}
		if commandWidth > 80 {
			commandWidth = 80 // Reasonable max
		}

		columns = []table.Column{
			{Title: "PID", Width: pidWidth},
			{Title: "USER", Width: userWidth},
			{Title: "CPU%", Width: cpuWidth},
			{Title: "MEM%", Width: memWidth},
			{Title: "COMMAND", Width: commandWidth},
		}
	}

	// Clear table completely before changing column structure to prevent panic
	m.processTable.SetRows([]table.Row{})
	m.processTable.SetColumns(columns)
	// Force table to update viewport with new column structure
	m.processTable.UpdateViewport()
	// Now repopulate with correct column structure
	m.updateProcessTable()
}

func (m *ResponsiveTUIModel) formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (m *ResponsiveTUIModel) truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 2 {
		return s[:maxLen]
	}
	return s[:maxLen-2] + ".."
}

func (m *ResponsiveTUIModel) renderSplitNetworkGraph(history []NetworkSample, width, height int) string {
	if len(history) == 0 || width < 10 || height < 3 {
		return strings.Repeat("─", width) + "\n"
	}

	// Find max rates for scaling
	var maxRxRate, maxTxRate float64
	for _, sample := range history {
		if sample.rxRate > maxRxRate {
			maxRxRate = sample.rxRate
		}
		if sample.txRate > maxTxRate {
			maxTxRate = sample.txRate
		}
	}

	// Use separate scaling for rx and tx to make both visible
	if maxRxRate == 0 && maxTxRate == 0 {
		return strings.Repeat("─", width) + "\n"
	}

	// Ensure minimum scaling to make small values visible
	if maxRxRate > 0 && maxRxRate < 1024 {
		maxRxRate = 1024 // Minimum 1KB for scaling
	}
	if maxTxRate > 0 && maxTxRate < 1024 {
		maxTxRate = 1024 // Minimum 1KB for scaling
	}

	// Create split graph - download above center line, upload below
	centerLine := height / 2
	upRows := centerLine
	downRows := height - centerLine - 1 // -1 for center line

	var result strings.Builder

	// Use all available samples, but sample them to fit the width
	// This preserves history better than just taking the last `width` samples
	samplesPerCol := 1
	if len(history) > width {
		samplesPerCol = len(history) / width
		if len(history)%width != 0 {
			samplesPerCol++
		}
	}

	// Render from top to bottom
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			// Sample from the history using intelligent sampling
			histIdx := col * samplesPerCol
			if histIdx >= len(history) {
				result.WriteString(" ")
				continue
			}

			// If we have multiple samples per column, average them
			var avgRx, avgTx float64
			sampleCount := 0
			for i := 0; i < samplesPerCol && histIdx+i < len(history); i++ {
				sample := history[histIdx+i]
				avgRx += sample.rxRate
				avgTx += sample.txRate
				sampleCount++
			}
			if sampleCount > 0 {
				avgRx /= float64(sampleCount)
				avgTx /= float64(sampleCount)
			}

			sample := NetworkSample{rxRate: avgRx, txRate: avgTx}

			if row == centerLine {
				result.WriteString("─") // Center line
			} else if row < centerLine {
				// Download (above center) - row 0 is top, use separate scaling
				downloadHeight := int((sample.rxRate / maxRxRate) * float64(upRows))
				if downloadHeight >= (upRows - row) {
					downloadColor, _ := m.getNetworkColors()
					colored := lipgloss.NewStyle().Foreground(lipgloss.Color(downloadColor)).Render("█")
					result.WriteString(colored)
				} else {
					result.WriteString(" ")
				}
			} else {
				// Upload (below center) - use separate scaling for better visibility
				uploadHeight := int((sample.txRate / maxTxRate) * float64(downRows))
				if uploadHeight >= (row - centerLine) {
					_, uploadColor := m.getNetworkColors()
					colored := lipgloss.NewStyle().Foreground(lipgloss.Color(uploadColor)).Render("▓")
					result.WriteString(colored)
				} else {
					result.WriteString(" ")
				}
			}
		}
		if row < height-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *ResponsiveTUIModel) selectBestNetworkInterface(interfaces []*models.NetworkRateInfo) *models.NetworkRateInfo {
	if len(interfaces) == 0 {
		return nil
	}

	var candidates []*models.NetworkRateInfo

	for _, iface := range interfaces {
		if iface.Interface == "lo" ||
			strings.HasPrefix(iface.Interface, "docker") ||
			strings.HasPrefix(iface.Interface, "br-") ||
			strings.HasPrefix(iface.Interface, "veth") {
			continue
		}
		candidates = append(candidates, iface)
	}

	if len(candidates) == 0 {
		for _, iface := range interfaces {
			if iface.Interface != "lo" {
				return iface
			}
		}
		return interfaces[0]
	}

	var bestInterface *models.NetworkRateInfo
	var maxActivity uint64

	for _, iface := range candidates {
		totalActivity := iface.RxTotal + iface.TxTotal
		currentActivity := uint64(iface.RxRate + iface.TxRate)

		score := totalActivity
		if currentActivity > 0 {
			score += currentActivity * 1000
		}

		if bestInterface == nil || score > maxActivity {
			bestInterface = iface
			maxActivity = score
		}
	}

	return bestInterface
}

func (m *ResponsiveTUIModel) getSelectedInterfaceName() string {
	if m.selectedInterfaceName != "" {
		return strings.ToUpper(m.selectedInterfaceName)
	}
	return "NETWORK"
}
