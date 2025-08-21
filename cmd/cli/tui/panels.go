package tui

import (
	"fmt"
	"strings"
)

func (m *ResponsiveTUIModel) renderCPUPanel(width, height int) string {
	style := m.panelStyle(width, height)

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

	titleLine := m.titleStyle().Render(cpuName)
	content.WriteString(titleLine + strings.Repeat(" ", spaces) + freqText + "\n")

	// CPU bar with usage and temperature - make bar wider so temp isn't too far left
	barWidth := width - 15 // Make bar wider to push temp right
	if barWidth < 8 {
		barWidth = 8
	}

	cpuBar := m.renderProgressBar(uint64(cpu.Usage*100), 10000, barWidth, "cpu")
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
			core1Bar := m.renderProgressBar(uint64(core1*100), 10000, coreBarWidth, "cpu")
			core1Str := fmt.Sprintf("C%02d%s%3.0f%%", i, core1Bar, core1) // No spaces
			line.WriteString(core1Str)
			line.WriteString(" ") // Space separator between columns

			// Second core if exists
			if i+1 < len(cpu.CoreUsage) {
				core2 := cpu.CoreUsage[i+1]
				core2Bar := m.renderProgressBar(uint64(core2*100), 10000, coreBarWidth, "cpu")
				core2Str := fmt.Sprintf("C%02d%s%3.0f%%", i+1, core2Bar, core2)
				line.WriteString(core2Str)
				line.WriteString(" ") // Space separator between columns
			}

			// Third core if exists
			if i+2 < len(cpu.CoreUsage) {
				core3 := cpu.CoreUsage[i+2]
				core3Bar := m.renderProgressBar(uint64(core3*100), 10000, coreBarWidth, "cpu")
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


