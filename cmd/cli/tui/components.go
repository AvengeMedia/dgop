package tui

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/AvengeMedia/dgop/gops"
	"github.com/AvengeMedia/dgop/models"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func NewResponsiveTUIModel(gopsUtil *gops.GopsUtil) *ResponsiveTUIModel {
	columns := []table.Column{
		{Title: "PID", Width: 5},
		{Title: "USER", Width: 4},
		{Title: "CPU", Width: 3},
		{Title: "MEMORY", Width: 18},
		{Title: "COMMAND", Width: 53},
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
		return strings.Repeat("░", width)
	}

	percentage := float64(used) / float64(total) * 100.0
	usedWidth := int(math.Round(float64(width) * float64(used) / float64(total)))

	if usedWidth == 0 && used > 0 {
		usedWidth = 1
	}
	if usedWidth > width {
		usedWidth = width
	}

	// Use the same style as the monolith - solid blocks
	var bar strings.Builder
	for i := 0; i < width; i++ {
		if i < usedWidth {
			bar.WriteString("▓") // Filled block
		} else {
			bar.WriteString("░") // Empty block
		}
	}

	result := bar.String()

	// Color based on usage using purple theme
	if percentage > 80 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#D946EF")).Render(result) // Bright purple for high
	} else if percentage > 60 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).Render(result) // Medium purple for medium
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5FBF")).Render(result) // Theme purple for low
}

func (m *ResponsiveTUIModel) renderSystemInfoPanel(width, height int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#8B5FBF")).
		Padding(0, 1).
		Margin(0).
		Width(width).
		MaxHeight(height)

	// Use actual system distro
	logo, color := getDistroInfo(m.hardware)

	// Build left content with expanded info
	var leftLines []string
	if m.hardware != nil {
		// Use primary purple for distro name
		distroStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5FBF")).Bold(true)
		leftLines = append(leftLines, distroStyle.Render(m.hardware.Distro))
		// Add logged in user with hostname
		username := os.Getenv("USER")
		if username == "" {
			username = "user"
		}
		leftLines = append(leftLines, fmt.Sprintf("%s@%s", username, m.hardware.Hostname))
		leftLines = append(leftLines, m.hardware.Kernel)
		leftLines = append(leftLines, m.hardware.BIOS.Motherboard)
		leftLines = append(leftLines, fmt.Sprintf("%s %s", m.hardware.BIOS.Version, m.hardware.BIOS.Date))

		// Add CPU count if available
		if m.metrics != nil && m.metrics.CPU != nil {
			cpuCount := len(m.metrics.CPU.CoreUsage)
			if cpuCount > 0 {
				leftLines = append(leftLines, fmt.Sprintf("%d threads", cpuCount))
			}
		}

		// Add uptime if available
		if m.metrics != nil && m.metrics.System != nil && m.metrics.System.BootTime != "" {
			leftLines = append(leftLines, fmt.Sprintf("Uptime: %s", m.metrics.System.BootTime))
		}
	}

	// Build left content as single block
	leftContent := strings.Join(leftLines, "\n")

	// Build logo as single styled block, right-aligned
	logoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	logoBlock := logoStyle.Render(strings.Join(logo, "\n"))

	// Calculate available space and create proper right alignment
	logoWidth := lipgloss.Width(logoBlock)
	leftWidth := lipgloss.Width(leftContent)
	availableWidth := width - 4 // account for borders and padding

	// Create right-aligned logo by padding left content to push logo right
	leftPadWidth := availableWidth - logoWidth - 2 // -2 for separator
	var finalContent string
	if leftPadWidth > leftWidth {
		// Pad the left content to push logo to the right
		paddedLeft := lipgloss.NewStyle().Width(leftPadWidth).Align(lipgloss.Left).Render(leftContent)
		finalContent = lipgloss.JoinHorizontal(lipgloss.Top, paddedLeft, "  ", logoBlock)
	} else {
		// If no room for padding, just join normally
		finalContent = lipgloss.JoinHorizontal(lipgloss.Top, leftContent, "  ", logoBlock)
	}

	// Only truncate if really necessary - be more generous with width
	if lipgloss.Width(finalContent) > width-2 { // reduced from width-4
		// Calculate actual available space for left content
		logoBlockWidth := lipgloss.Width(logoBlock)
		maxLeftWidth := width - 6 - logoBlockWidth // borders + padding + separator
		if maxLeftWidth > 15 {                     // only truncate if we have reasonable space
			truncatedLeft := ""
			for i, line := range leftLines {
				if len(line) > maxLeftWidth {
					if maxLeftWidth > 3 {
						line = line[:maxLeftWidth-3] + "..."
					} else {
						line = line[:maxLeftWidth]
					}
				}
				if i > 0 {
					truncatedLeft += "\n"
				}
				truncatedLeft += line
			}
			finalContent = lipgloss.JoinHorizontal(lipgloss.Top, truncatedLeft, "  ", logoBlock)
		}
		// If maxLeftWidth <= 15, keep original content and let it overflow slightly
	}

	// Ensure content fills allocated height to show borders
	contentHeight := lipgloss.Height(finalContent)
	innerHeight := height - 2 // subtract borders

	// Pad content to fill allocated height
	if contentHeight < innerHeight {
		padding := strings.Repeat("\n", innerHeight-contentHeight)
		finalContent = finalContent + padding
	}

	// Limit to inner dimensions
	innerStyle := lipgloss.NewStyle().MaxHeight(height - 2).MaxWidth(width - 4)
	return style.Render(innerStyle.Render(finalContent))
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
			"      /\\",
			"     /  \\",
			"    /    \\",
			"   /      \\",
			"  /   ,,   \\",
			" /   |  |   \\",
			"/_-''    ''-_\\",
		}, "#1793D1"
	case strings.Contains(distro, "ubuntu"):
		return []string{
			"         _",
			"     ---(_)",
			" _/  ---  \\",
			"(_) |   |",
			"  \\  --- _/",
			"     ---(_)",
		}, "#E95420"
	case strings.Contains(distro, "fedora"):
		return []string{
			"        ,'''''.",
			"       |   ,.  | ",
			"       |  |  '_'",
			"  ,....|  |..",
			".'  ,_;|   ..'",
			"|  |   |  |",
			"|  ',_,'  |",
			" '.     ,'",
			"   '''''",
		}, "#0B57A4"
	case strings.Contains(distro, "nix"):
		return []string{
			"  ▗▄   ▗▄ ▄▖",
			" ▄▄🬸█▄▄▄🬸█▛ ▃",
			"   ▟▛    ▜▃▟🬕",
			"🬋🬋🬫█      █🬛🬋🬋",
			" 🬷▛🮃▙    ▟▛",
			" 🮃 ▟█🬴▀▀▀█🬴▀▀",
			"  ▝▀ ▀▘   ▀▘",
		}, "#5294e2"
	case strings.Contains(distro, "debian"):
		return []string{
			"  _____",
			" /  __ \\",
			"|  /    |",
			"|  \\___-",
			"-_",
			"  --_",
		}, "#D70A53"
	case strings.Contains(distro, "mint"):
		return []string{
			" __________",
			"|_          \\",
			"  | | _____ |",
			"  | | | | | |",
			"  | | | | | |",
			"  | \\____/ |",
			"  \\_________/",
		}, "#3EB489"
	case strings.Contains(distro, "gentoo"):
		return []string{
			" *-----*",
			"(       \\",
			"\\    0   \\",
			" \\        )",
			" /      _/",
			"(     _-",
			"\\____-",
		}, "#54487A"
	case strings.Contains(distro, "cachyos"):
		return []string{
			"   /''''''''''''/",
			"  /''''''''''''/",
			" /''''''/",
			"/''''''/",
			"\\......\\",
			" \\......\\",
			"  \\............../",
			"   \\............./",
		}, "#08A283"
	case strings.Contains(distro, "elementary"):
		return []string{
			"  _______",
			" / ____  \\",
			"/  |  /  /\\",
			"|__\\ /  / |",
			"\\   /__/  /",
			" \\_______/",
		}, "#64BAFF"
	case strings.Contains(distro, "pop"):
		return []string{
			"______",
			"\\   * \\        *_",
			" \\ \\ \\ \\      / /",
			"  \\ \\_\\ \\    / /",
			"   \\  ___\\  /_/",
			"    \\ \\    _",
			"   __\\_\\__(_)_",
			"  (___________)`",
		}, "#48B9C7"
	case strings.Contains(distro, "suse"):
		return []string{
			"  _______",
			"**|   ** \\",
			"     / .\\ \\",
			"     \\__/ |",
			"   _______|",
			"   \\_______",
			"__________/",
		}, "#73BA25"
	case strings.Contains(distro, "endeavour"):
		return []string{
			"          /o.",
			"        /sssso-",
			"      /ossssssso:",
			"    /ssssssssssso+",
			"  /ssssssssssssssso+",
			"//osssssssssssssso+-",
			" `+++++++++++++++-`",
		}, "#7F3FBF"
	default:
		return []string{
			"    ___",
			"   (.. \\",
			"   (<> |",
			"  //  \\ \\",
			" ( |  | /|",
			"_/\\ __)/_)",
			"\\/____\\/",
		}, "#7D56F4"
	}
}
