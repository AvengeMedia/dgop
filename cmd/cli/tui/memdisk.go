package tui

import (
	"fmt"
	"strconv"
	"strings"
	"github.com/charmbracelet/lipgloss"
)

func (m *ResponsiveTUIModel) renderMemDiskPanel(width, height int) string {
	style := m.panelStyle(width, height)

	var content []string

	// Memory section
	content = append(content, m.titleStyle().Render("MEMORY"))
	
	if m.metrics != nil && m.metrics.Memory != nil {
		mem := m.metrics.Memory
		totalGB := float64(mem.Total) / 1024 / 1024
		usedGB := float64(mem.Total-mem.Available) / 1024 / 1024

		usedPercent := usedGB / totalGB * 100
		barWidth := width - 15
		if barWidth < 8 {
			barWidth = 8
		}
		memBar := m.renderProgressBar(mem.Total-mem.Available, mem.Total, barWidth, "memory")

		content = append(content, fmt.Sprintf("%s %.1f%%", memBar, usedPercent))
		content = append(content, fmt.Sprintf("%.1f/%.1fGB", usedGB, totalGB))

		if mem.SwapTotal > 0 {
			swapTotalGB := float64(mem.SwapTotal) / 1024 / 1024
			swapUsedGB := float64(mem.SwapTotal-mem.SwapFree) / 1024 / 1024
			swapPercent := swapUsedGB / swapTotalGB * 100
			swapBar := m.renderProgressBar(mem.SwapTotal-mem.SwapFree, mem.SwapTotal, barWidth, "memory")

			content = append(content, fmt.Sprintf("%s %.1f%%", swapBar, swapPercent))
			content = append(content, fmt.Sprintf("%.1f/%.1fGB Swap", swapUsedGB, swapTotalGB))
		}
	} else {
		content = append(content, "Loading memory info...")
	}

	// Disk section
	content = append(content, "")
	content = append(content, m.titleStyle().Render("DISK"))

	if m.metrics == nil || len(m.metrics.DiskMounts) == 0 {
		content = append(content, "Loading...")
	} else {
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
			
			// Show usage as "Used/Total" format
			usageInfo := fmt.Sprintf("%s/%s", mount.Used, mount.Size)
			content = append(content, fmt.Sprintf("%s %s", m.renderProgressBar(uint64(percent*100), 10000, barWidth, "disk"), usageInfo))

			disksShown++
		}

		// Add disk I/O chart
		if len(m.diskHistory) > 1 {
			content = append(content, "")
			latest := m.diskHistory[len(m.diskHistory)-1]
			content = append(content, fmt.Sprintf("R: %s W: %s", m.formatBytes(uint64(latest.readRate))+"/s", m.formatBytes(uint64(latest.writeRate))+"/s"))
		}

		// Add sensors if available
		if len(m.systemTemperatures) > 0 {
			content = append(content, "")
			content = append(content, m.titleStyle().Render("SENSORS"))
			
			// Show a reasonable number of sensors that fit
			sensorsToShow := len(m.systemTemperatures)
			if sensorsToShow > 6 { // Limit to prevent overcrowding
				sensorsToShow = 6
			}

			for i := 0; i < sensorsToShow; i++ {
				sensor := m.systemTemperatures[i]
				// Use full sensor name, don't truncate unnecessarily
				name := sensor.Name
				if len(name) > 20 { // Only truncate if really long
					name = name[:20]
				}

				// Color based on temperature
				tempStr := fmt.Sprintf("%.0f°C", sensor.Temperature)
				color := m.getTemperatureColor(sensor.Temperature)
				tempStr = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(tempStr)

				content = append(content, fmt.Sprintf("%s: %s", name, tempStr))
			}
		}
	}

	// Ensure content fills allocated height
	contentStr := strings.Join(content, "\n")
	lines := strings.Split(contentStr, "\n")
	innerHeight := height - 2 // subtract borders
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}

	return style.Render(strings.Join(lines, "\n"))
}