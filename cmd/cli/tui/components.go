package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/AvengeMedia/dgop/gops"
	"github.com/AvengeMedia/dgop/models"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func NewResponsiveTUIModel(gopsUtil *gops.GopsUtil) *ResponsiveTUIModel {
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

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		BorderBottom(true).
		Bold(true)

	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Bold(false)

	t.SetStyles(s)

	model := &ResponsiveTUIModel{
		gops:           gopsUtil,
		processTable:   t,
		sortBy:         gops.SortByCPU,
		procLimit:      50,
		maxNetHistory:  60,
		maxDiskHistory: 60,
		selectedPID:    -1,
	}

	hardware, _ := gopsUtil.GetSystemHardware()
	model.hardware = hardware
	model.distroLogo, model.distroColor = getDistroInfo(hardware)

	return model
}

func (m *ResponsiveTUIModel) renderProgressBar(used, total uint64, width int) string {
	if total == 0 {
		return strings.Repeat("─", width)
	}

	percentage := float64(used) / float64(total) * 100.0
	usedWidth := int(math.Round(float64(width) * float64(used) / float64(total)))

	if usedWidth == 0 && used > 0 {
		usedWidth = 1
	}
	if usedWidth > width {
		usedWidth = width
	}

	usedBar := strings.Repeat("█", usedWidth)
	freeBar := strings.Repeat("─", width-usedWidth)

	return progressBarUsedStyle.Render(usedBar) + progressBarStyle.Render(freeBar) +
		fmt.Sprintf(" %.1f%%", percentage)
}

func (m *ResponsiveTUIModel) renderSystemInfoPanel(width int) string {
	if m.metrics == nil || m.metrics.System == nil {
		return panelStyle.Width(width).Render("Loading system info...")
	}

	dankArt := []string{
		"  ██╗  ██╗",
		"  ██║  ██║",
		"  ███████║",
		"  ██╔══██║",
		"  ██║  ██║",
		"  ╚═╝  ╚═╝",
	}

	content := []string{}

	hostname := "unknown"
	if m.hardware != nil {
		hostname = m.hardware.Hostname
	}

	uptimeStr := ""
	if m.metrics.System.BootTime != "" {
		uptimeStr = fmt.Sprintf("up %s", calculateUptime(m.metrics.System.BootTime))
	}

	now := time.Now()
	clock := now.Format("15:04:05")

	headerLine := fmt.Sprintf("%s  %s  %s", hostname, uptimeStr, clock)
	content = append(content, boldTextStyle.Render(headerLine))
	content = append(content, "")

	if m.metrics.System != nil {
		content = append(content, fmt.Sprintf("Load: %s", m.metrics.System.LoadAvg))
		content = append(content, fmt.Sprintf("Tasks: %d processes", m.metrics.System.Processes))
		content = append(content, fmt.Sprintf("Threads: %d", m.metrics.System.Threads))
	}

	contentStr := strings.Join(content, "\n")
	artStr := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Render(strings.Join(dankArt, "\n"))

	combinedContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		contentStr,
		strings.Repeat(" ", 4),
		artStr,
	)

	centeredContent := lipgloss.NewStyle().
		Width(width - 4).
		Align(lipgloss.Center).
		Render(combinedContent)

	return panelStyle.Width(width).Render(centeredContent)
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatRate(rate float64) string {
	return formatBytes(uint64(rate)) + "/s"
}

func calculateUptime(bootTimeStr string) string {
	bootTime, err := time.Parse("2006-01-02 15:04:05", bootTimeStr)
	if err != nil {
		return "unknown"
	}

	uptime := time.Since(bootTime)
	days := int(uptime.Hours()) / 24
	hours := int(uptime.Hours()) % 24
	minutes := int(uptime.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func getDistroInfo(hardware *models.SystemHardware) ([]string, string) {
	if hardware == nil {
		return []string{}, "#7D56F4"
	}

	distro := strings.ToLower(hardware.Distro)

	switch {
	case strings.Contains(distro, "arch"):
		return []string{
			"      /\\      ",
			"     /  \\     ",
			"    /\\   \\    ",
			"   /      \\   ",
			"  /   ,,   \\  ",
			" /   |  |  -\\ ",
			"/_-''    ''-_\\",
		}, "#1793D1"
	case strings.Contains(distro, "ubuntu"):
		return []string{
			"         _   ",
			"     ---(_)  ",
			" _/  ---  \\  ",
			"(_) |   |    ",
			"  \\  --- _/  ",
			"     ---(_)  ",
		}, "#E95420"
	default:
		return []string{
			"  ▄▄▄▄▄▄▄▄▄▄▄",
			"  █         █",
			"  █  ▄▄▄▄▄  █",
			"  █  █   █  █",
			"  █  █▄▄▄█  █",
			"  █         █",
			"  ▀▀▀▀▀▀▀▀▀▀▀",
		}, "#7D56F4"
	}
}
