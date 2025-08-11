package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/AvengeMedia/dgop/gops"
	"github.com/AvengeMedia/dgop/models"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type NetworkSample struct {
	timestamp time.Time
	rxBytes   uint64
	txBytes   uint64
	rxRate    float64 // bytes per second
	txRate    float64 // bytes per second
}

type DiskSample struct {
	timestamp  time.Time
	readBytes  uint64
	writeBytes uint64
	readRate   float64 // bytes per second
	writeRate  float64 // bytes per second
	device     string  // device name for this sample
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type ResponsiveTUIModel struct {
	gops       *gops.GopsUtil
	metrics    *models.SystemMetrics
	width      int
	height     int
	err        error
	lastUpdate time.Time

	// Components
	processTable table.Model
	viewport     viewport.Model

	// Static system info (fetch once)
	hardware *models.SystemHardware

	// Network history for graphs
	networkHistory    []NetworkSample
	maxNetHistory     int
	networkCursor     string
	lastNetworkUpdate time.Time

	// Disk history for graphs
	diskHistory    []DiskSample
	maxDiskHistory int
	diskCursor     string
	lastDiskUpdate time.Time

	// State
	sortBy      gops.ProcSortBy
	procLimit   int
	ready       bool
	showDetails bool  // Toggle for process details
	selectedPID int32 // Track selected process PID for sticky selection

	// Distro logo
	distroLogo  []string
	distroColor string
}

func newResponsiveTUIModel(gopsUtil *gops.GopsUtil) *ResponsiveTUIModel {
	// Create process table with proper columns (removed PPID)
	columns := []table.Column{
		{Title: "PID", Width: 8},
		{Title: "CPU%", Width: 8},
		{Title: "MEM%", Width: 8},
		{Title: "COMMAND", Width: 25},
		{Title: "FULL COMMAND", Width: 35},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithHeight(20),
		table.WithFocused(true),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("#8B5FBF")).
		Bold(false)
	t.SetStyles(s)

	// Fetch static hardware info once
	hardware, _ := gopsUtil.GetSystemHardware()

	// Get distro logo
	logo, color := GetDistroLogo()

	return &ResponsiveTUIModel{
		gops:           gopsUtil,
		sortBy:         gops.SortByCPU,
		procLimit:      50, // Increase from 15 to 50
		processTable:   t,
		viewport:       viewport.New(0, 0),
		hardware:       hardware,
		networkHistory: make([]NetworkSample, 0),
		maxNetHistory:  60, // Keep 60 seconds of history
		diskHistory:    make([]DiskSample, 0),
		maxDiskHistory: 60, // Keep 60 seconds of history
		distroLogo:     logo,
		distroColor:    color,
	}
}

func (m *ResponsiveTUIModel) Init() tea.Cmd {
	return tea.Batch(tick(), m.fetchMetrics())
}

func (m *ResponsiveTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			return m, m.fetchMetrics()
		case "c":
			m.sortBy = gops.SortByCPU
			return m, m.fetchMetrics()
		case "m":
			m.sortBy = gops.SortByMemory
			return m, m.fetchMetrics()
		case "n":
			m.sortBy = gops.SortByName
			return m, m.fetchMetrics()
		case "p":
			m.sortBy = gops.SortByPID
			return m, m.fetchMetrics()
		case "d":
			m.showDetails = !m.showDetails // Toggle details
			return m, nil
		}

		// Handle table navigation
		var cmd tea.Cmd
		oldCursor := m.processTable.Cursor()
		m.processTable, cmd = m.processTable.Update(msg)
		cmds = append(cmds, cmd)

		// Update selected PID when cursor moves
		newCursor := m.processTable.Cursor()
		if oldCursor != newCursor && m.metrics != nil && len(m.metrics.Processes) > newCursor {
			m.selectedPID = m.metrics.Processes[newCursor].PID
		}

	case tickMsg:
		cmds = append(cmds, tick(), m.fetchMetrics())

	case *models.SystemMetrics:
		m.metrics = msg
		m.lastUpdate = time.Now()
		m.err = nil
		m.updateProcessTable()

	case error:
		m.err = msg
	}

	return m, tea.Batch(cmds...)
}

func (m *ResponsiveTUIModel) fetchMetrics() tea.Cmd {
	return func() tea.Msg {
		metrics, err := m.gops.GetAllMetrics(m.sortBy, m.procLimit, true)
		if err != nil {
			return err
		}
		return metrics
	}
}

func (m *ResponsiveTUIModel) updateLayout() {
	// Layout will be handled by individual render functions
	// No stupid size restrictions - work with whatever we have
}

func (m *ResponsiveTUIModel) updateProcessTable() {
	if m.metrics == nil || len(m.metrics.Processes) == 0 {
		return
	}

	// Track current cursor position for sticky selection
	currentCursor := m.processTable.Cursor()

	// Build new rows (removed PPID column)
	rows := make([]table.Row, 0, len(m.metrics.Processes))
	selectedIndex := -1

	for i, proc := range m.metrics.Processes {
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", proc.PID),
			fmt.Sprintf("%.1f", proc.CPU),
			fmt.Sprintf("%.1f", proc.MemoryPercent),
			proc.Command,
			proc.FullCommand,
		})

		// Track selected PID position
		if proc.PID == m.selectedPID {
			selectedIndex = i
		}
	}

	m.processTable.SetRows(rows)

	// Implement sticky selection logic
	if m.selectedPID != 0 && selectedIndex >= 0 {
		// If we found the selected PID, move cursor to it (unless at top without details)
		if selectedIndex != 0 || m.showDetails {
			m.processTable.SetCursor(selectedIndex)
		}
		// If at top without details, let it stay at position 0
	} else if len(m.metrics.Processes) > 0 {
		// No previous selection or PID not found, update selectedPID to current position
		if currentCursor < len(m.metrics.Processes) {
			m.selectedPID = m.metrics.Processes[currentCursor].PID
		} else {
			m.selectedPID = m.metrics.Processes[0].PID
			m.processTable.SetCursor(0)
		}
	}
}

func (m *ResponsiveTUIModel) View() string {
	if !m.ready {
		// Still show layout even when not ready
		return m.renderLayout()
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

func (m *ResponsiveTUIModel) renderMainContent() string {
	// No terminal size check - show whatever we can

	// Update network and disk history
	m.updateNetworkHistory()
	m.updateDiskHistory()

	// Calculate layout dimensions using dynamic approach
	leftWidth := m.width * 40 / 100   // 40% for left panels
	rightWidth := m.width - leftWidth // 60% for processes

	// DYNAMIC HEIGHT CALCULATION - measure header/footer first
	header := m.renderHeader()
	footer := m.renderFooter()
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)

	// Available height = total - header - footer - margins (more conservative)
	availableHeight := m.height - headerHeight - footerHeight - 2 // Extra margin for safety
	if availableHeight < 8 {
		availableHeight = 8 // Absolute minimum
	}

	// SMART HEIGHT ALLOCATION - system box should fit content, processes should be BIG
	// System needs: OS name + 4 lines of info = ~5 lines
	// CPU needs: name + freq/temp + usage bar + 24 cores in 4 cols = ~8 lines
	systemHeight := 6 // Tight fit for system info
	cpuHeight := 10   // Enough for CPU name + all cores with bars
	topRowHeight := systemHeight
	if cpuHeight > systemHeight {
		topRowHeight = cpuHeight
	}

	// Give MOST space to processes - it's the main content
	bottomRowHeight := availableHeight - topRowHeight
	if bottomRowHeight < 8 {
		bottomRowHeight = 8
		topRowHeight = availableHeight - bottomRowHeight
	}

	var sections []string

	// TOP ROW: SYSTEM INFO (40% like left col) + CPU (60% like right col) - PROPER ALIGNMENT
	systemPanel := m.renderSystemInfoPanel(leftWidth, topRowHeight)
	cpuPanel := m.renderCPUPanel(rightWidth, topRowHeight)

	// Use MaxHeight to prevent overflow
	systemPanel = lipgloss.NewStyle().MaxHeight(topRowHeight).Render(systemPanel)
	cpuPanel = lipgloss.NewStyle().MaxHeight(topRowHeight).Render(cpuPanel)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, systemPanel, cpuPanel)
	sections = append(sections, topRow)

	// BOTTOM ROW: Smart allocation - memory+disks need more space, network less
	// Memory+disks need ~8 lines for content, network only needs ~4
	memHeight := bottomRowHeight * 70 / 100 // Give most space to memory+disks
	networkHeight := bottomRowHeight - memHeight
	if memHeight < 8 {
		memHeight = 8
		networkHeight = bottomRowHeight - memHeight
	}
	if networkHeight < 3 {
		networkHeight = 3
		memHeight = bottomRowHeight - networkHeight
	}

	memPanel := m.renderMemDiskPanel(leftWidth, memHeight)
	networkPanel := m.renderNetworkPanel(leftWidth, networkHeight)

	// Constrain heights to prevent overflow
	memPanel = lipgloss.NewStyle().MaxHeight(memHeight).Render(memPanel)
	networkPanel = lipgloss.NewStyle().MaxHeight(networkHeight).Render(networkPanel)

	leftColumn := lipgloss.JoinVertical(lipgloss.Left, memPanel, networkPanel)

	// Right column: processes - with optional details toggle
	var rightColumn string
	if m.showDetails {
		// Split between processes (75%) and details (25%)
		processTableHeight := bottomRowHeight * 75 / 100
		processDetailsHeight := bottomRowHeight - processTableHeight

		processPanel := m.renderProcessPanel(rightWidth, processTableHeight)
		detailsPanel := m.renderProcessContinuedPanel(rightWidth, processDetailsHeight)

		processPanel = lipgloss.NewStyle().MaxHeight(processTableHeight).Render(processPanel)
		detailsPanel = lipgloss.NewStyle().MaxHeight(processDetailsHeight).Render(detailsPanel)

		rightColumn = lipgloss.JoinVertical(lipgloss.Left, processPanel, detailsPanel)
	} else {
		// Processes get ALL the space
		processPanel := m.renderProcessPanel(rightWidth, bottomRowHeight)
		rightColumn = lipgloss.NewStyle().MaxHeight(bottomRowHeight).Render(processPanel)
	}

	// Join left and right columns
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, rightColumn)

	// Final constraint on bottom row height
	bottomRow = lipgloss.NewStyle().MaxHeight(bottomRowHeight).Render(bottomRow)

	sections = append(sections, bottomRow)

	return strings.Join(sections, "\n")
}

func (m *ResponsiveTUIModel) renderSystemInfoPanel(width, height int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(0, 1).
		Width(width).
		Height(height)

	var content strings.Builder

	// Small DANK ASCII art (top right corner style)
	dankAscii := []string{
		"██▄ ██▄ █▄ █▄ ▄▄▄█",
		"██▄ █▄█ █▄▄█ █▄▄▄▄",
	}

	if m.hardware != nil {
		osName := m.hardware.Distro
		if osName == "" {
			osName = "Linux"
		}

		// Create centered content with DANK art
		var lines []string
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.distroColor)).Render(osName))

		hw := m.hardware
		maxWidth := width - 6
		if maxWidth < 10 {
			maxWidth = 10
		}
		lines = append(lines, fmt.Sprintf("Host: %s", m.truncate(hw.Hostname, maxWidth)))
		lines = append(lines, fmt.Sprintf("Kernel: %s", m.truncate(hw.Kernel, maxWidth)))
		lines = append(lines, fmt.Sprintf("Mobo: %s", m.truncate(hw.BIOS.Motherboard, maxWidth)))
		lines = append(lines, fmt.Sprintf("BIOS: %s", m.truncate(hw.BIOS.Version, maxWidth)))
		if m.metrics != nil && m.metrics.System != nil {
			lines = append(lines, fmt.Sprintf("Boot: %s", m.metrics.System.BootTime))
		}

		// Add DANK ASCII to the first two lines
		for i, line := range lines {
			if i < len(dankAscii) {
				// Try to fit DANK ASCII on the right
				padding := width - len(line) - len(dankAscii[i]) - 4
				if padding > 0 {
					content.WriteString(line + strings.Repeat(" ", padding) +
						lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5FBF")).Render(dankAscii[i]) + "\n")
				} else {
					content.WriteString(line + "\n")
				}
			} else {
				content.WriteString(line + "\n")
			}
		}
	}

	// Apply centering to the final content
	centeredContent := lipgloss.NewStyle().Align(lipgloss.Center).Render(content.String())
	return style.Render(centeredContent)
}

func (m *ResponsiveTUIModel) renderCPUPanel(width, height int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(0, 1). // Only horizontal padding
		Width(width).
		Height(height)

	var content strings.Builder

	if m.metrics == nil || m.metrics.CPU == nil {
		content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5FBF")).Render("CPU") + "\n\n")
		content.WriteString("Loading CPU data...")
		return style.Render(content.String())
	}

	cpu := m.metrics.CPU

	// CPU NAME with MHz right-aligned on same line
	cpuName := "Unknown CPU"
	if m.hardware != nil && m.hardware.CPU.Model != "" {
		cpuName = m.hardware.CPU.Model
	}

	freqStr := fmt.Sprintf("%.0fMHz", cpu.Frequency)
	nameWidth := width - len(freqStr) - 6
	if nameWidth < 10 {
		nameWidth = 10
	}

	truncatedName := m.truncate(cpuName, nameWidth)
	spaces := width - len(truncatedName) - len(freqStr) - 4
	if spaces < 1 {
		spaces = 1
	}

	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5FBF"))
	content.WriteString(nameStyle.Render(truncatedName) + strings.Repeat(" ", spaces) + freqStr + "\n")

	// TOTAL USAGE BAR with temp on same line
	barWidth := width - 20
	if barWidth < 8 {
		barWidth = 8
	}
	cpuBar := m.renderProgressBar(cpu.Usage, 100.0, barWidth)
	content.WriteString(fmt.Sprintf("%s %.1f%% %.1f°C\n", cpuBar, cpu.Usage, cpu.Temperature))

	// ALL THE CORES - MUST show all 24 cores no matter what
	totalCores := len(cpu.CoreUsage)

	// Calculate optimal columns to fit all cores in available space
	// Each core needs: "C00 ▓▓░░ 12% " = ~12 chars minimum
	minCoreWidth := 12
	maxCoresPerRow := (width - 4) / minCoreWidth
	if maxCoresPerRow < 1 {
		maxCoresPerRow = 1
	}

	// Calculate rows needed and adjust columns if necessary
	coresPerRow := maxCoresPerRow
	rowsNeeded := (totalCores + coresPerRow - 1) / coresPerRow

	// If too many rows, try to balance
	if rowsNeeded > 6 { // Max 6 rows
		coresPerRow = (totalCores + 5) / 6 // Distribute across 6 rows max
	}

	// Show ALL CPU cores with proper gaps and sizing
	for i := 0; i < totalCores; i += coresPerRow {
		var coreLine strings.Builder
		for j := 0; j < coresPerRow && i+j < totalCores; j++ {
			coreIdx := i + j
			usage := cpu.CoreUsage[coreIdx]

			if j > 0 {
				coreLine.WriteString("  ") // GAP between cores
			}

			// Calculate bar width to fit available space with gaps
			availableWidth := width - 4       // Border padding
			gapSpace := (coresPerRow - 1) * 2 // Spaces between cores
			coreSpace := availableWidth - gapSpace
			barWidth := (coreSpace / coresPerRow) - 8 // Subtract space for "C00  12%"

			if barWidth < 3 {
				barWidth = 3
			}
			if barWidth > 8 {
				barWidth = 8
			}

			miniBar := m.renderProgressBar(usage, 100.0, barWidth)
			coreLine.WriteString(fmt.Sprintf("C%02d %s %2.0f%%", coreIdx, miniBar, usage))
		}
		content.WriteString(coreLine.String() + "\n")
	}

	// BOTTOM THE LOAD NUMBERS
	if m.metrics.System != nil && m.metrics.System.LoadAvg != "" {
		content.WriteString("\n" + m.metrics.System.LoadAvg)
	}

	return style.Render(content.String())
}

func (m *ResponsiveTUIModel) renderMemDiskPanel(width, height int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(0, 1). // Only horizontal padding
		Width(width).
		Height(height)

	var content strings.Builder

	content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5FBF")).Render("MEMORY") + "\n")

	if m.metrics != nil && m.metrics.Memory != nil {
		mem := m.metrics.Memory
		totalGB := float64(mem.Total) / 1024 / 1024
		usedGB := float64(mem.Total-mem.Available) / 1024 / 1024
		usedPercent := usedGB / totalGB * 100

		barWidth := width - 15
		if barWidth < 8 {
			barWidth = 8
		}
		memBar := m.renderProgressBar(usedPercent, 100.0, barWidth)

		content.WriteString(fmt.Sprintf("%s %.1f%%\n", memBar, usedPercent))
		content.WriteString(fmt.Sprintf("%.1f/%.1fGB\n", usedGB, totalGB))

		swapTotalGB := float64(mem.SwapTotal) / 1024 / 1024
		swapUsedGB := float64(mem.SwapTotal-mem.SwapFree) / 1024 / 1024
		if swapTotalGB > 0 {
			swapPercent := swapUsedGB / swapTotalGB * 100
			swapBar := m.renderProgressBar(swapPercent, 100.0, barWidth)
			content.WriteString(fmt.Sprintf("%s %.1f%%\n", swapBar, swapPercent))
			content.WriteString(fmt.Sprintf("%.1f/%.1fGB Swap\n", swapUsedGB, swapTotalGB))
		}
	} else {
		content.WriteString("Loading...\n")
	}

	content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5FBF")).Render("DISKS") + "\n")

	if m.metrics != nil && len(m.metrics.DiskMounts) > 0 {
		// Limit disk display to 5 to prevent overflow
		maxDisks := 5
		diskCount := len(m.metrics.DiskMounts)
		if diskCount > maxDisks {
			diskCount = maxDisks
		}

		for i := 0; i < diskCount; i++ {
			mount := m.metrics.DiskMounts[i]
			// Parse the size information
			percentStr := strings.TrimSuffix(mount.Percent, "%")
			var percent float64
			if p, err := fmt.Sscanf(percentStr, "%f", &percent); p > 0 && err == nil {
				// Header: device / mount / total size
				maxHeaderWidth := width - 4
				headerText := fmt.Sprintf("%s / %s", mount.Device, mount.Mount)
				truncatedHeader := m.truncate(headerText, maxHeaderWidth)
				content.WriteString(fmt.Sprintf("%s\n", truncatedHeader))

				// Calculate bar width more conservatively to prevent wrapping
				// Format: "[bar] XX.X% - XXXgb" needs ~15-20 chars for text
				diskBarWidth := width - 25
				if diskBarWidth < 4 {
					diskBarWidth = 4
				}

				usedBar := m.renderDiskBar(percent, 100.0, diskBarWidth, true)
				content.WriteString(fmt.Sprintf("%s %.1f%% - %s\n", usedBar, percent, mount.Used))
			}
		}

		if len(m.metrics.DiskMounts) > maxDisks {
			content.WriteString(fmt.Sprintf("... and %d more\n", len(m.metrics.DiskMounts)-maxDisks))
		}

		// Add disk I/O chart at the bottom
		if len(m.diskHistory) > 0 {
			content.WriteString("\n")
			latest := m.diskHistory[len(m.diskHistory)-1]
			readRateStr := m.formatBytes(uint64(latest.readRate))
			writeRateStr := m.formatBytes(uint64(latest.writeRate))

			content.WriteString(fmt.Sprintf("R:%s/s W:%s/s\n", readRateStr, writeRateStr))

			// Calculate space for chart - more conservative spacing
			usedLines := diskCount*2 + 6 // each disk = 2 lines, plus headers/labels/rate line
			if len(m.metrics.DiskMounts) > maxDisks {
				usedLines++ // "... and X more" line
			}
			chartHeight := height - usedLines
			if chartHeight > 3 { // Need at least 3 lines for meaningful chart
				diskGraph := m.renderSplitDiskGraph(m.diskHistory, width-4, chartHeight)
				content.WriteString(diskGraph)

				// Add bottom data line
				totalRead := m.formatBytes(latest.readBytes)
				totalWrite := m.formatBytes(latest.writeBytes)
				bottomLine := fmt.Sprintf("R: %s W: %s", totalRead, totalWrite)
				if len(bottomLine) > width-4 {
					bottomLine = m.truncate(bottomLine, width-4)
				}
				content.WriteString("\n" + bottomLine)
			}
		}
	} else {
		content.WriteString("Loading...\n")
	}

	return style.Render(content.String())
}

func (m *ResponsiveTUIModel) updateNetworkHistory() {
	now := time.Now()
	// Only update network history every 2 seconds to prevent rapid sampling
	if now.Sub(m.lastNetworkUpdate) < 2*time.Second {
		return
	}
	m.lastNetworkUpdate = now

	rateResponse, err := m.gops.GetNetworkRates(m.networkCursor)
	if err != nil {
		return
	}

	m.networkCursor = rateResponse.Cursor

	if len(rateResponse.Interfaces) == 0 {
		return
	}

	// Use first interface for now
	iface := rateResponse.Interfaces[0]

	sample := NetworkSample{
		timestamp: now,
		rxBytes:   iface.RxTotal,
		txBytes:   iface.TxTotal,
		rxRate:    iface.RxRate,
		txRate:    iface.TxRate,
	}

	m.networkHistory = append(m.networkHistory, sample)

	if len(m.networkHistory) > m.maxNetHistory {
		m.networkHistory = m.networkHistory[1:]
	}
}

func (m *ResponsiveTUIModel) updateDiskHistory() {
	now := time.Now()
	// Only update disk history every 2 seconds to prevent rapid sampling
	if now.Sub(m.lastDiskUpdate) < 2*time.Second {
		return
	}
	m.lastDiskUpdate = now

	diskRateResponse, err := m.gops.GetDiskRates(m.diskCursor)
	if err != nil {
		return
	}

	m.diskCursor = diskRateResponse.Cursor

	if len(diskRateResponse.Disks) == 0 {
		return
	}

	// Aggregate all disks into a single sample (total I/O across all devices)
	var totalReadRate, totalWriteRate float64
	var totalReadBytes, totalWriteBytes uint64

	for _, disk := range diskRateResponse.Disks {
		totalReadRate += disk.ReadRate
		totalWriteRate += disk.WriteRate
		totalReadBytes += disk.ReadTotal
		totalWriteBytes += disk.WriteTotal
	}

	sample := DiskSample{
		timestamp:  now,
		readBytes:  totalReadBytes,
		writeBytes: totalWriteBytes,
		readRate:   totalReadRate,
		writeRate:  totalWriteRate,
		device:     "total", // Aggregate of all devices
	}

	m.diskHistory = append(m.diskHistory, sample)

	if len(m.diskHistory) > m.maxDiskHistory {
		m.diskHistory = m.diskHistory[1:]
	}
}

func (m *ResponsiveTUIModel) renderNetworkPanel(width, height int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(0, 1). // Only horizontal padding
		Width(width).
		Height(height)

	var content strings.Builder

	// Use interface name as header instead of "NETWORK"
	interfaceName := "NETWORK"
	if m.metrics != nil && len(m.metrics.Network) > 0 {
		interfaceName = m.metrics.Network[0].Name // Use first interface name
	}

	content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5FBF")).Render(interfaceName) + "\n")

	if len(m.networkHistory) == 0 {
		content.WriteString("Loading...")
		return style.Render(content.String())
	}

	// Get latest rates
	latest := m.networkHistory[len(m.networkHistory)-1]

	// Format rates in human readable format
	rxRateStr := m.formatBytes(uint64(latest.rxRate))
	txRateStr := m.formatBytes(uint64(latest.txRate))

	content.WriteString(fmt.Sprintf("↓%s/s ↑%s/s\n", rxRateStr, txRateStr))

	// Reserve space for bottom labels
	graphHeight := height - 4 // One extra line for bottom data
	if graphHeight > 2 {
		splitGraph := m.renderSplitNetworkGraph(m.networkHistory, width-4, graphHeight)
		content.WriteString(splitGraph)

		// Add bottom data line with totals
		totalRx := m.formatBytes(latest.rxBytes)
		totalTx := m.formatBytes(latest.txBytes)
		bottomLine := fmt.Sprintf("RX: %s TX: %s", totalRx, totalTx)
		if len(bottomLine) > width-4 {
			bottomLine = m.truncate(bottomLine, width-4)
		}
		content.WriteString("\n" + bottomLine)
	}

	return style.Render(content.String())
}

func (m *ResponsiveTUIModel) renderNetworkGraph(history []NetworkSample, graphType string, width, height int) string {
	if len(history) == 0 || width < 10 || height < 2 {
		return "No data"
	}

	// Get data points based on graph type
	var maxRate float64
	rates := make([]float64, len(history))
	for i, sample := range history {
		if graphType == "rx" {
			rates[i] = sample.rxRate
		} else {
			rates[i] = sample.txRate
		}
		if rates[i] > maxRate {
			maxRate = rates[i]
		}
	}

	if maxRate == 0 {
		return strings.Repeat("_", width)
	}

	// Create simple bar graph
	var result strings.Builder
	graphWidth := width
	if len(rates) < graphWidth {
		graphWidth = len(rates)
	}

	// Take the last graphWidth samples
	startIdx := len(rates) - graphWidth
	if startIdx < 0 {
		startIdx = 0
	}

	for h := height - 1; h >= 0; h-- {
		for i := startIdx; i < len(rates) && (i-startIdx) < graphWidth; i++ {
			rate := rates[i]
			barHeight := int((rate / maxRate) * float64(height))

			if barHeight > h {
				if rate > maxRate*0.8 {
					result.WriteString("█") // High usage - red
				} else if rate > maxRate*0.5 {
					result.WriteString("▓") // Medium usage - yellow
				} else {
					result.WriteString("▒") // Low usage - green
				}
			} else {
				result.WriteString(" ")
			}
		}
		if h > 0 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func (m *ResponsiveTUIModel) renderSimpleGraph(history []NetworkSample, graphType string, width, height int) string {
	if len(history) == 0 || width < 10 || height < 1 {
		return strings.Repeat("▁", width) + "\n"
	}

	var maxRate float64
	rates := make([]float64, len(history))
	for i, sample := range history {
		if graphType == "rx" {
			rates[i] = sample.rxRate
		} else {
			rates[i] = sample.txRate
		}
		if rates[i] > maxRate {
			maxRate = rates[i]
		}
	}

	if maxRate == 0 {
		return strings.Repeat("▁", width) + "\n"
	}

	var result strings.Builder
	startIdx := len(rates) - width
	if startIdx < 0 {
		startIdx = 0
	}

	for i := startIdx; i < len(rates) && (i-startIdx) < width; i++ {
		rate := rates[i]
		level := int((rate / maxRate) * 8)

		switch level {
		case 0:
			result.WriteString("▁")
		case 1:
			result.WriteString("▂")
		case 2:
			result.WriteString("▃")
		case 3:
			result.WriteString("▄")
		case 4:
			result.WriteString("▅")
		case 5:
			result.WriteString("▆")
		case 6:
			result.WriteString("▇")
		default:
			result.WriteString("█")
		}
	}

	for result.Len() < width {
		result.WriteString("▁")
	}

	return result.String() + "\n"
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
					// Use bright purple for download
					colored := lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).Render("█")
					result.WriteString(colored)
				} else {
					result.WriteString(" ")
				}
			} else {
				// Upload (below center) - use separate scaling for better visibility
				uploadHeight := int((sample.txRate / maxTxRate) * float64(downRows))
				if uploadHeight >= (row - centerLine) {
					// Use different purple shade for upload
					colored := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5FBF")).Render("▓")
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

func (m *ResponsiveTUIModel) renderSplitDiskGraph(history []DiskSample, width, height int) string {
	if len(history) == 0 || width < 10 || height < 3 {
		return strings.Repeat("─", width) + "\n"
	}

	// Find max rates for scaling
	var maxReadRate, maxWriteRate float64
	for _, sample := range history {
		if sample.readRate > maxReadRate {
			maxReadRate = sample.readRate
		}
		if sample.writeRate > maxWriteRate {
			maxWriteRate = sample.writeRate
		}
	}

	// Use separate scaling for read and write to make both visible
	if maxReadRate == 0 && maxWriteRate == 0 {
		return strings.Repeat("─", width) + "\n"
	}

	// Ensure minimum scaling to make small values visible
	if maxReadRate > 0 && maxReadRate < 1024 {
		maxReadRate = 1024 // Minimum 1KB for scaling
	}
	if maxWriteRate > 0 && maxWriteRate < 1024 {
		maxWriteRate = 1024 // Minimum 1KB for scaling
	}

	// Create split graph - read above center line, write below
	centerLine := height / 2
	upRows := centerLine
	downRows := height - centerLine - 1 // -1 for center line

	var result strings.Builder

	// Use all available samples, but sample them to fit the width
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
			var avgRead, avgWrite float64
			sampleCount := 0
			for i := 0; i < samplesPerCol && histIdx+i < len(history); i++ {
				sample := history[histIdx+i]
				avgRead += sample.readRate
				avgWrite += sample.writeRate
				sampleCount++
			}
			if sampleCount > 0 {
				avgRead /= float64(sampleCount)
				avgWrite /= float64(sampleCount)
			}

			sample := DiskSample{readRate: avgRead, writeRate: avgWrite}

			if row == centerLine {
				result.WriteString("─") // Center line
			} else if row < centerLine {
				// Read (above center) - row 0 is top, use separate scaling
				readHeight := int((sample.readRate / maxReadRate) * float64(upRows))
				if readHeight >= (upRows - row) {
					// Use bright purple for read
					colored := lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).Render("█")
					result.WriteString(colored)
				} else {
					result.WriteString(" ")
				}
			} else {
				// Write (below center) - use separate scaling for better visibility
				writeHeight := int((sample.writeRate / maxWriteRate) * float64(downRows))
				if writeHeight >= (row - centerLine) {
					// Use different purple shade for write
					colored := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5FBF")).Render("▓")
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

func (m *ResponsiveTUIModel) renderProcessPanel(width, height int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(0, 1).
		Width(width).
		Height(height)

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
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#8B5FBF"))

	content.WriteString(titleStyle.Render(title) + "\n")

	// Update table dimensions and column widths for this panel
	tableHeight := height - 3 // Account for title and borders
	if tableHeight < 5 {
		tableHeight = 5
	}

	// Update process table for this panel
	m.updateProcessColumnWidthsForPanel(width - 4)
	m.processTable.SetHeight(tableHeight)

	content.WriteString(m.processTable.View())

	return style.Render(content.String())
}

func (m *ResponsiveTUIModel) renderProcessContinuedPanel(width, height int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(0, 1).
		Width(width).
		Height(height)

	var content strings.Builder

	// This panel shows the continuation of the process list
	// For now, we'll show additional process information or keep it simple
	title := "PROCESS DETAILS"
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#8B5FBF"))

	content.WriteString(titleStyle.Render(title) + "\n")

	if m.metrics != nil && len(m.metrics.Processes) > 0 {
		// Get currently selected process from table
		selectedIdx := m.processTable.Cursor()
		if selectedIdx < len(m.metrics.Processes) {
			proc := m.metrics.Processes[selectedIdx]

			content.WriteString(fmt.Sprintf("PID: %d\n", proc.PID))
			content.WriteString(fmt.Sprintf("PPID: %d\n", proc.PPID))
			content.WriteString(fmt.Sprintf("CPU: %.1f%%\n", proc.CPU))
			content.WriteString(fmt.Sprintf("Memory: %.1f%%\n", proc.MemoryPercent))
			// Status field doesn't exist in ProcessInfo model
			// content.WriteString(fmt.Sprintf("Status: %s\n", proc.Status))
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
							// Word is too long, truncate it
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

func (m *ResponsiveTUIModel) updateProcessColumnWidthsForPanel(totalWidth int) {
	// Calculate responsive column widths for process table in panel
	fixedWidth := 8 + 8 + 8 + 25 + 6               // PID, CPU%, MEM%, COMMAND, borders/padding
	remainingWidth := totalWidth - fixedWidth - 10 // Extra margin

	if remainingWidth < 30 {
		remainingWidth = 30
	}

	// Distribute remaining width between COMMAND and FULL COMMAND
	commandWidth := remainingWidth / 3
	fullCommandWidth := remainingWidth - commandWidth

	if commandWidth < 12 {
		commandWidth = 12
		fullCommandWidth = remainingWidth - commandWidth
	}
	if fullCommandWidth < 15 {
		fullCommandWidth = 15
	}

	// Update table columns - removed PPID
	columns := []table.Column{
		{Title: "PID", Width: 8},
		{Title: "CPU%", Width: 8},
		{Title: "MEM%", Width: 8},
		{Title: "COMMAND", Width: 25},
		{Title: "FULL COMMAND", Width: totalWidth - 32 - 10}, // Use remaining space
	}
	m.processTable.SetColumns(columns)
}

func (m *ResponsiveTUIModel) renderInfoBar() string {
	var parts []string

	// Static info (no shifting data)
	if m.hardware != nil {
		hw := m.hardware
		parts = append(parts, hw.Distro)
		parts = append(parts, hw.Kernel)
		parts = append(parts, hw.Hostname)
	}

	// Load average from real-time data
	if m.metrics != nil && m.metrics.System != nil {
		parts = append(parts, fmt.Sprintf("Load: %s", m.metrics.System.LoadAvg))
	}

	info := strings.Join(parts, " | ")

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C9C9C9")).
		Background(lipgloss.Color("#2A2A2A")).
		Width(m.width).
		Padding(0, 2)

	return style.Render(info)
}

func (m *ResponsiveTUIModel) renderMiddleSection(height int) string {
	leftWidth := m.width * 35 / 100   // 35% for left panel
	rightWidth := m.width - leftWidth // 65% for right panel

	leftPanel := m.renderSystemPanel(leftWidth, height)
	rightPanel := m.renderCPUMemoryPanel(rightWidth, height)

	// Join horizontally without adding extra height
	combined := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Ensure fixed height
	lines := strings.Split(combined, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", m.width))
	}

	return strings.Join(lines, "\n")
}

func (m *ResponsiveTUIModel) renderSystemPanel(width, height int) string {
	if width < 10 || height < 5 {
		return ""
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(1).
		Width(width).
		Height(height)

	var content strings.Builder
	content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5FBF")).Render("SYSTEM") + "\n\n")

	if m.hardware != nil {
		hw := m.hardware
		maxWidth := width - 8
		if maxWidth < 10 {
			maxWidth = 10
		}
		content.WriteString(fmt.Sprintf("CPU: %s\n", m.truncate(hw.CPU.Model, maxWidth)))
		content.WriteString(fmt.Sprintf("Cores: %d\n", hw.CPU.Count))
		content.WriteString(fmt.Sprintf("Mobo: %s\n", m.truncate(hw.BIOS.Motherboard, maxWidth)))
		content.WriteString(fmt.Sprintf("BIOS: %s\n", m.truncate(hw.BIOS.Version, maxWidth)))
		content.WriteString(fmt.Sprintf("Date: %s\n", hw.BIOS.Date))
	}

	return style.Render(content.String())
}

func (m *ResponsiveTUIModel) renderCPUMemoryPanel(width, height int) string {
	if width < 20 || height < 5 {
		return ""
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(1).
		Width(width).
		Height(height)

	var content strings.Builder

	if m.metrics == nil {
		content.WriteString("Loading metrics...")
	} else {
		// CPU section with all cores
		if m.metrics.CPU != nil {
			cpu := m.metrics.CPU
			content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5FBF")).Render("CPU") + "\n")

			barWidth := width - 20
			if barWidth < 10 {
				barWidth = 10
			}
			cpuBar := m.renderProgressBar(cpu.Usage, 100.0, barWidth)
			content.WriteString(fmt.Sprintf("Overall: %s %.1f%%\n", cpuBar, cpu.Usage))
			content.WriteString(fmt.Sprintf("Freq: %.0fMHz  Temp: %.1f°C\n\n", cpu.Frequency, cpu.Temperature))

			// Show all CPU cores in a compact grid
			coresPerRow := (width - 8) / 12 // Estimate cores per row
			if coresPerRow < 1 {
				coresPerRow = 1
			}

			for i := 0; i < len(cpu.CoreUsage); i += coresPerRow {
				var coreLine strings.Builder
				for j := 0; j < coresPerRow && i+j < len(cpu.CoreUsage); j++ {
					coreIdx := i + j
					usage := cpu.CoreUsage[coreIdx]
					coreLine.WriteString(fmt.Sprintf("C%02d:%4.1f%% ", coreIdx, usage))
				}
				content.WriteString(coreLine.String() + "\n")
			}
		}

		content.WriteString("\n")

		// Memory section
		if m.metrics.Memory != nil {
			mem := m.metrics.Memory
			totalGB := float64(mem.Total) / 1024 / 1024
			usedGB := float64(mem.Total-mem.Available) / 1024 / 1024
			usedPercent := usedGB / totalGB * 100

			content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5FBF")).Render("MEMORY") + "\n")

			barWidth := width - 20
			if barWidth < 10 {
				barWidth = 10
			}
			memBar := m.renderProgressBar(usedPercent, 100.0, barWidth)
			content.WriteString(fmt.Sprintf("Used: %s %.1f%%\n", memBar, usedPercent))
			content.WriteString(fmt.Sprintf("%.1f/%.1f GB\n", usedGB, totalGB))
		}
	}

	// Add distro logo as a separate block if there's space
	if len(m.distroLogo) > 0 && width > 60 {
		content.WriteString("\n")
		// Limit logo size and ensure it fits
		logoLines := m.distroLogo
		if len(logoLines) > 6 { // Limit height
			logoLines = logoLines[:6]
		}

		for _, logoLine := range logoLines {
			// Truncate logo line if it's too wide
			maxLogoWidth := width - 8 // Account for borders and padding
			if len(logoLine) > maxLogoWidth {
				logoLine = logoLine[:maxLogoWidth]
			}
			content.WriteString(logoLine + "\n")
		}
	}

	return style.Render(content.String())
}

func (m *ResponsiveTUIModel) renderProcessSection(height int) string {
	// Update table dimensions to fixed height
	tableHeight := height - 4 // Account for title and borders
	if tableHeight < 5 {
		tableHeight = 5
	}
	m.processTable.SetHeight(tableHeight)

	// Update column widths for full width
	m.updateProcessColumnWidths(m.width - 4)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(0, 1).
		Width(m.width).
		MaxHeight(height) // Force max height

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
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#8B5FBF"))

	content.WriteString(titleStyle.Render(title) + "\n")
	content.WriteString(m.processTable.View())

	// Ensure we don't exceed height
	result := style.Render(content.String())
	lines := strings.Split(result, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

func (m *ResponsiveTUIModel) renderDiskSection() string {
	fixedHeight := 4 // Always 4 lines high

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(0, 1).
		Width(m.width).
		MaxHeight(fixedHeight)

	var content strings.Builder
	content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5FBF")).Render("DISKS") + " ")

	if m.metrics != nil && len(m.metrics.DiskMounts) > 0 {
		// Show inline disk info to save space
		for i, mount := range m.metrics.DiskMounts {
			if i > 2 { // Limit to 3 mounts
				break
			}
			device := mount.Device
			if strings.HasPrefix(device, "/dev/") {
				device = device[5:]
			}
			content.WriteString(fmt.Sprintf("%s:%s %s  ",
				device,
				mount.Mount,
				mount.Percent))
		}
	} else {
		content.WriteString("No disk data")
	}

	// Ensure fixed height
	result := style.Render(content.String())
	lines := strings.Split(result, "\n")
	if len(lines) > fixedHeight {
		lines = lines[:fixedHeight]
	}
	for len(lines) < fixedHeight {
		lines = append(lines, strings.Repeat(" ", m.width))
	}

	return strings.Join(lines, "\n")
}

func (m *ResponsiveTUIModel) updateProcessColumnWidths(totalWidth int) {
	// Calculate responsive column widths for process table
	commandWidth := totalWidth - 32 - 10 // Total minus fixed columns minus margins
	if commandWidth < 20 {
		commandWidth = 20
	}

	// Update table columns - removed PPID
	columns := []table.Column{
		{Title: "PID", Width: 8},
		{Title: "CPU%", Width: 8},
		{Title: "MEM%", Width: 8},
		{Title: "COMMAND", Width: 25},
		{Title: "FULL COMMAND", Width: commandWidth},
	}
	m.processTable.SetColumns(columns)
}

func (m *ResponsiveTUIModel) renderHeader() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Bold(true).
		Width(m.width).
		Padding(0, 2)

	var rightSide strings.Builder

	if m.metrics != nil && m.metrics.System != nil {
		uptime := m.metrics.System.BootTime
		if uptime != "" {
			rightSide.WriteString("Up: ")
			rightSide.WriteString(uptime)
		}
	}

	// Current time
	currentTime := time.Now().Format("15:04:05")
	if rightSide.Len() > 0 {
		rightSide.WriteString(" | ")
		rightSide.WriteString(currentTime)
	} else {
		rightSide.WriteString(currentTime)
	}

	title := fmt.Sprintf("dgop %s", Version)
	rightText := rightSide.String()
	spaces := m.width - len(title) - len(rightText) - 4
	if spaces < 0 {
		spaces = 0
	}
	headerText := fmt.Sprintf("%s%s%s", title, strings.Repeat(" ", spaces), rightText)

	return style.Render(headerText)
}

func (m *ResponsiveTUIModel) renderProgressBar(current, max float64, width int) string {
	if max == 0 || width <= 0 {
		return strings.Repeat("░", width)
	}

	filled := int((current / max) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	// Ensure at least 1 character shows for any non-zero percentage
	if current > 0 && filled == 0 {
		filled = 1
	}

	// Use sleek pixely style like btop - different chars for different fill levels
	var bar strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			bar.WriteString("▓") // Filled block
		} else {
			bar.WriteString("░") // Empty block
		}
	}

	result := bar.String()

	// Color based on usage using purple theme
	if current > 80 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#D946EF")).Render(result) // Bright purple for high
	} else if current > 60 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).Render(result) // Medium purple for medium
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5FBF")).Render(result) // Theme purple for low
}

func (m *ResponsiveTUIModel) renderDiskBar(current, max float64, width int, isUsed bool) string {
	if max == 0 || width <= 0 {
		return strings.Repeat("░", width)
	}

	filled := int((current / max) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	var bar strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			bar.WriteString("▓")
		} else {
			bar.WriteString("░")
		}
	}

	result := bar.String()

	if isUsed {
		if current > 90 {
			return lipgloss.NewStyle().Foreground(lipgloss.Color("#D946EF")).Render(result) // Bright purple for high usage
		} else if current > 70 {
			return lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).Render(result) // Medium purple for medium usage
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5FBF")).Render(result) // Theme purple for used space
	} else {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#6B46C1")).Render(result) // Slightly different purple for free space
	}
}

func (m *ResponsiveTUIModel) renderFooter() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C7C7C")).
		Background(lipgloss.Color("#2A2A2A")).
		Width(m.width).
		Padding(0, 2)

	controls := "Controls: [q]uit [r]efresh [d]etails | Sort: [c]pu [m]emory [n]ame [p]id | ↑↓ Navigate"
	return style.Render(controls)
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

func runResponsiveTUIMonitor(gopsUtil *gops.GopsUtil) error {
	m := newResponsiveTUIModel(gopsUtil)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run responsive DANK TUI: %w", err)
	}

	return nil
}
