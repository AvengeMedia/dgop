package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *ResponsiveTUIModel) renderCPUPanel(width, height int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(0, 1).
		Margin(0, 0, 0, 0).
		Width(width).
		MaxHeight(height)

	var content strings.Builder

	if m.metrics == nil || m.metrics.CPU == nil {
		content.WriteString("Loading CPU info...")
		// Pad to fill allocated height even when loading
		contentStr := content.String()
		lines := strings.Split(contentStr, "\n")
		innerHeight := height - 2
		for len(lines) < innerHeight {
			lines = append(lines, "")
		}
		return style.Render(strings.Join(lines, "\n"))
	}

	cpu := m.metrics.CPU
	cpuName := cpu.Model
	if len(cpuName) > width-10 {
		cpuName = cpuName[:width-10] + ".."
	}

	// CPU name as title, with right-aligned frequency - align with core layout
	freqText := fmt.Sprintf("%.0fMHz", cpu.Frequency)
	// Calculate spaces to align with core columns - adjust for proper C/MHz alignment
	availableWidth := width - 5                                 // account for borders+padding, align with cores
	spaces := availableWidth - len(cpuName) - len(freqText)
	if spaces < 1 {
		spaces = 1
	}

	titleLine := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5FBF")).Render(cpuName)
	content.WriteString(titleLine + strings.Repeat(" ", spaces) + freqText + "\n")

	// CPU bar with usage and temperature - make bar wider so temp isn't too far left
	barWidth := width - 15 // Make bar wider to push temp right
	if barWidth < 8 {
		barWidth = 8
	}

	cpuBar := m.renderProgressBar(uint64(cpu.Usage*100), 10000, barWidth)
	// Format as fixed-width strings for consistent alignment
	usageText := fmt.Sprintf("%3.0f%%", cpu.Usage) // Always 3 chars for percentage (e.g. " 5%" or "100%")
	tempText := fmt.Sprintf("%.0f°C", cpu.Temperature)
	content.WriteString(fmt.Sprintf("%s %s %s\n", cpuBar, usageText, tempText))

	// All cores with bars in 3 columns filling 100% width
	if len(cpu.CoreUsage) > 0 {
		// Each column gets 33% of available width (accounting for borders/padding)
		availableWidth := width - 4 // Account for borders/padding
		columnWidth := availableWidth / 3

		// Each core needs space for "C00" (3 chars) + bar + "100%" (4 chars) = 7 + bar (no spaces)
		coreBarWidth := columnWidth - 8 // More space for wider bars
		if coreBarWidth < 6 {
			coreBarWidth = 6
		}

		for i := 0; i < len(cpu.CoreUsage); i += 3 {
			var line strings.Builder

			// First core - format as "C01[bar]5%" with no spaces, add separator
			core1 := cpu.CoreUsage[i]
			core1Bar := m.renderProgressBar(uint64(core1*100), 10000, coreBarWidth)
			core1Str := fmt.Sprintf("C%02d%s%3.0f%%", i, core1Bar, core1) // No spaces
			line.WriteString(core1Str)
			line.WriteString(" ") // Space separator between columns

			// Second core if exists
			if i+1 < len(cpu.CoreUsage) {
				core2 := cpu.CoreUsage[i+1]
				core2Bar := m.renderProgressBar(uint64(core2*100), 10000, coreBarWidth)
				core2Str := fmt.Sprintf("C%02d%s%3.0f%%", i+1, core2Bar, core2)
				line.WriteString(core2Str)
				line.WriteString(" ") // Space separator between columns
			}

			// Third core if exists
			if i+2 < len(cpu.CoreUsage) {
				core3 := cpu.CoreUsage[i+2]
				core3Bar := m.renderProgressBar(uint64(core3*100), 10000, coreBarWidth)
				core3Str := fmt.Sprintf("C%02d%s%3.0f%%", i+2, core3Bar, core3)
				line.WriteString(core3Str) // No separator after last column
			}

			content.WriteString(line.String() + "\n")
		}

		// Add load/tasks/threads on a single line under CPU cores
		if m.metrics != nil && m.metrics.System != nil {
			systemInfo := fmt.Sprintf("Load: %s | Tasks: %d | Threads: %d",
				m.metrics.System.LoadAvg,
				m.metrics.System.Processes,
				m.metrics.System.Threads)
			content.WriteString(systemInfo)
		}
	}

	// Ensure content fills allocated height
	contentStr := content.String()
	lines := strings.Split(contentStr, "\n")
	innerHeight := height - 2
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}

	return style.Render(strings.Join(lines, "\n"))
}

func (m *ResponsiveTUIModel) renderMemoryPanel(width int) string {
	if m.metrics == nil || m.metrics.Memory == nil {
		return panelStyle.Width(width).Render("Loading memory info...")
	}

	mem := m.metrics.Memory
	totalGB := float64(mem.Total) / 1024 / 1024
	usedGB := float64(mem.Total-mem.Available) / 1024 / 1024
	availableGB := float64(mem.Available) / 1024 / 1024

	content := []string{
		fmt.Sprintf("Total: %.1fGB", totalGB),
		fmt.Sprintf("Used:  %.1fGB", usedGB),
		fmt.Sprintf("Avail: %.1fGB", availableGB),
		"",
		fmt.Sprintf("Usage: %s", m.renderProgressBar(mem.Total-mem.Available, mem.Total, width-15)),
	}

	if mem.SwapTotal > 0 {
		swapTotalGB := float64(mem.SwapTotal) / 1024 / 1024
		swapUsedGB := float64(mem.SwapTotal-mem.SwapFree) / 1024 / 1024

		content = append(content, "")
		content = append(content, fmt.Sprintf("Swap:  %.1fGB / %.1fGB", swapUsedGB, swapTotalGB))
		content = append(content, fmt.Sprintf("Usage: %s", m.renderProgressBar(mem.SwapTotal-mem.SwapFree, mem.SwapTotal, width-15)))
	}

	return panelStyle.Width(width).Render(strings.Join(content, "\n"))
}

func (m *ResponsiveTUIModel) renderDiskPanel(width int) string {
	var content []string
	content = append(content, boldTextStyle.Render("DISK"))

	if m.metrics == nil || len(m.metrics.DiskMounts) == 0 {
		content = append(content, "Loading...")
		return panelStyle.Width(width).Render(strings.Join(content, "\n"))
	}

	// Show top 3 disks
	disksShown := 0
	for _, mount := range m.metrics.DiskMounts {
		if disksShown >= 3 {
			break
		}

		if mount.Device == "tmpfs" || mount.Device == "devtmpfs" ||
			strings.HasPrefix(mount.Mount, "/dev") || strings.HasPrefix(mount.Mount, "/proc") ||
			strings.HasPrefix(mount.Mount, "/sys") || strings.HasPrefix(mount.Mount, "/run") {
			continue
		}

		deviceName := mount.Device
		if len(deviceName) > 15 {
			deviceName = deviceName[:12] + "..."
		}

		// Parse percentage
		percentStr := strings.TrimSuffix(mount.Percent, "%")
		percent, _ := strconv.ParseFloat(percentStr, 64)

		barWidth := width - 20
		if barWidth < 10 {
			barWidth = 10
		}

		// Show device and mount point clearly
		displayName := fmt.Sprintf("%s → %s", deviceName, mount.Mount)
		if len(displayName) > width-8 {
			// If too long, try shorter device name
			shortDevice := deviceName
			if len(shortDevice) > 8 {
				shortDevice = shortDevice[:8] + "..."
			}
			displayName = fmt.Sprintf("%s → %s", shortDevice, mount.Mount)
			if len(displayName) > width-8 {
				displayName = mount.Mount // fallback to just mount point
			}
		}
		content = append(content, displayName)
		content = append(content, fmt.Sprintf("%s %s", m.renderProgressBar(uint64(percent*100), 10000, barWidth), mount.Used))

		disksShown++
	}

	// Add disk I/O chart
	if len(m.diskHistory) > 1 {
		content = append(content, "")
		latest := m.diskHistory[len(m.diskHistory)-1]
		content = append(content, fmt.Sprintf("R: %s/s W: %s/s", formatBytes(uint64(latest.readRate)), formatBytes(uint64(latest.writeRate))))

		// Mini chart
		chartHeight := 4
		chartWidth := width - 4
		chart := m.renderMiniDiskChart(chartWidth, chartHeight)
		content = append(content, chart)
	}

	return panelStyle.Width(width).Render(strings.Join(content, "\n"))
}

func (m *ResponsiveTUIModel) renderMiniDiskChart(width, height int) string {
	if len(m.diskHistory) < 2 {
		return ""
	}

	var maxRate float64
	for _, sample := range m.diskHistory {
		if sample.readRate > maxRate {
			maxRate = sample.readRate
		}
		if sample.writeRate > maxRate {
			maxRate = sample.writeRate
		}
	}

	if maxRate == 0 {
		return strings.Repeat("─", width)
	}

	// Simple sparkline
	result := ""
	samplesShown := min(len(m.diskHistory), width)

	for i := 0; i < samplesShown; i++ {
		sample := m.diskHistory[len(m.diskHistory)-samplesShown+i]
		combinedRate := sample.readRate + sample.writeRate
		level := int((combinedRate / (maxRate * 2)) * 8)

		switch level {
		case 0:
			result += "▁"
		case 1:
			result += "▂"
		case 2:
			result += "▃"
		case 3:
			result += "▄"
		case 4:
			result += "▅"
		case 5:
			result += "▆"
		case 6:
			result += "▇"
		default:
			result += "█"
		}
	}

	return result
}
