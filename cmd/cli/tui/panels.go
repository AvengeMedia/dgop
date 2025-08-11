package tui

import (
	"fmt"
	"strconv"
	"strings"
)

func (m *ResponsiveTUIModel) renderCPUPanel(width int) string {
	if m.metrics == nil || m.metrics.CPU == nil {
		return panelStyle.Width(width).Render("Loading CPU info...")
	}

	cpu := m.metrics.CPU
	content := []string{
		fmt.Sprintf("Model: %s", cpu.Model),
		fmt.Sprintf("Cores: %d @ %.0fMHz", cpu.Count, cpu.Frequency),
		fmt.Sprintf("Temp: %.1f°C", cpu.Temperature),
		"",
		fmt.Sprintf("Usage: %s", m.renderProgressBar(uint64(cpu.Usage*100), 10000, width-15)),
	}

	if len(cpu.CoreUsage) > 0 {
		content = append(content, "")
		content = append(content, "Core Usage:")
		
		for i := 0; i < len(cpu.CoreUsage); i += 2 {
			var line string
			core1 := fmt.Sprintf("C%02d: %s", i, 
				m.renderProgressBar(uint64(cpu.CoreUsage[i]*100), 10000, (width-20)/2))
			
			if i+1 < len(cpu.CoreUsage) {
				core2 := fmt.Sprintf("C%02d: %s", i+1,
					m.renderProgressBar(uint64(cpu.CoreUsage[i+1]*100), 10000, (width-20)/2))
				line = fmt.Sprintf("%s  %s", core1, core2)
			} else {
				line = core1
			}
			content = append(content, line)
		}
	}

	return panelStyle.Width(width).Render(strings.Join(content, "\n"))
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
	if m.metrics == nil || len(m.metrics.DiskMounts) == 0 {
		return panelStyle.Width(width).Render("Loading disk info...")
	}

	var content []string
	
	maxDisks := 5
	disksShown := 0
	
	for _, mount := range m.metrics.DiskMounts {
		if disksShown >= maxDisks {
			break
		}
		
		if mount.Device == "tmpfs" || mount.Device == "devtmpfs" || 
		   strings.HasPrefix(mount.Mount, "/dev") || strings.HasPrefix(mount.Mount, "/proc") ||
		   strings.HasPrefix(mount.Mount, "/sys") || strings.HasPrefix(mount.Mount, "/run") {
			continue
		}

		usedStr := strings.TrimSuffix(mount.Used, "B")
		totalStr := strings.TrimSuffix(mount.Size, "B")
		
		var used, total uint64
		var err error
		
		if strings.HasSuffix(usedStr, "G") {
			usedFloat, _ := strconv.ParseFloat(strings.TrimSuffix(usedStr, "G"), 64)
			used = uint64(usedFloat * 1024 * 1024 * 1024)
		} else if strings.HasSuffix(usedStr, "M") {
			usedFloat, _ := strconv.ParseFloat(strings.TrimSuffix(usedStr, "M"), 64)
			used = uint64(usedFloat * 1024 * 1024)
		} else {
			used, err = strconv.ParseUint(usedStr, 10, 64)
			if err != nil {
				used = 0
			}
		}
		
		if strings.HasSuffix(totalStr, "G") {
			totalFloat, _ := strconv.ParseFloat(strings.TrimSuffix(totalStr, "G"), 64)
			total = uint64(totalFloat * 1024 * 1024 * 1024)
		} else if strings.HasSuffix(totalStr, "M") {
			totalFloat, _ := strconv.ParseFloat(strings.TrimSuffix(totalStr, "M"), 64)
			total = uint64(totalFloat * 1024 * 1024)
		} else {
			total, err = strconv.ParseUint(totalStr, 10, 64)
			if err != nil {
				total = 0
			}
		}

		deviceName := mount.Device
		if len(deviceName) > 12 {
			deviceName = deviceName[:12] + "..."
		}

		content = append(content, fmt.Sprintf("%-15s %s", deviceName, mount.Mount))
		content = append(content, fmt.Sprintf("%-8s %s", mount.Used, m.renderProgressBar(used, total, width-25)))
		content = append(content, "")
		
		disksShown++
	}

	if len(content) > 0 {
		content = content[:len(content)-1]
	}

	return panelStyle.Width(width).Render(strings.Join(content, "\n"))
}