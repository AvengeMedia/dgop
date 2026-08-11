package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strconv"
	"strings"
)

func (m *ResponsiveTUIModel) renderMemDiskPanel(width, height int) string {
	style := m.panelStyle(width, height)

	var content []string

	// Memory section
	content = append(content, m.titleStyle().Render("MEMORY"))

	if m.metrics != nil && m.metrics.Memory != nil {
		mem := m.metrics.Memory
		totalGB := float64(mem.Total) / 1024 / 1024
		usedGB := float64(mem.Used) / 1024 / 1024

		barWidth := width - 15
		if barWidth < 8 {
			barWidth = 8
		}
		memBar := m.renderProgressBar(mem.Used, mem.Total, barWidth, "memory")

		content = append(content, fmt.Sprintf("%s %.1f%%", memBar, mem.UsedPercent))
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

			// VIBE CODED BY GEMINI
			// Check if your TUI model has access to the gops wrapper instance
			if m.gops != nil {
				// Query your updated GPU information array
				if gpuInfo, err := m.gops.GetGPUInfo(); err == nil && len(gpuInfo.GPUs) > 0 {
					for _, gpu := range gpuInfo.GPUs {
						if gpu.Vendor == "Intel" || gpu.Driver == "i915" {
							// Format the live dynamic temperature using Lipgloss styles
							gpuTempStr := fmt.Sprintf("%.0f°C", gpu.Temperature)
							tempColor := m.getTemperatureColor(gpu.Temperature)
							gpuTempStr = lipgloss.NewStyle().Foreground(lipgloss.Color(tempColor)).Render(gpuTempStr)

							// Format your working live usage percentage line
							gpuUsageStr := fmt.Sprintf("%.1f%%", gpu.Usage)
							usageColor := m.getProgressBarColor(gpu.Usage, "primary")
							gpuUsageStr = lipgloss.NewStyle().Foreground(lipgloss.Color(usageColor)).Render(gpuUsageStr)

							// Append them right at the top of the SENSORS list
							content = append(content, fmt.Sprintf("gpu_temp: %s", gpuTempStr))
							content = append(content, fmt.Sprintf("gpu_usage: %s", gpuUsageStr))
						}
					}
				}
			}

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
