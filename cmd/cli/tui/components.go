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
		logoTestMode:   false, // Disable logo test mode
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

	// Use current logo (either system distro or test cycling logo)
	logo := m.distroLogo
	color := m.distroColor

	// Build left content with expanded info - keep raw and styled versions separate
	var leftLines []string
	var styledLeftLines []string
	if m.hardware != nil {
		// Use primary purple for distro name
		distroStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5FBF")).Bold(true)
		leftLines = append(leftLines, m.hardware.Distro)
		styledLeftLines = append(styledLeftLines, distroStyle.Render(m.hardware.Distro))
		
		// Add logged in user with hostname
		username := os.Getenv("USER")
		if username == "" {
			username = "user"
		}
		userHostLine := fmt.Sprintf("%s@%s", username, m.hardware.Hostname)
		leftLines = append(leftLines, userHostLine)
		styledLeftLines = append(styledLeftLines, userHostLine)
		
		leftLines = append(leftLines, m.hardware.Kernel)
		styledLeftLines = append(styledLeftLines, m.hardware.Kernel)
		
		leftLines = append(leftLines, m.hardware.BIOS.Motherboard)
		styledLeftLines = append(styledLeftLines, m.hardware.BIOS.Motherboard)
		
		biosLine := fmt.Sprintf("%s %s", m.hardware.BIOS.Version, m.hardware.BIOS.Date)
		leftLines = append(leftLines, biosLine)
		styledLeftLines = append(styledLeftLines, biosLine)

		// Add CPU count if available
		if m.metrics != nil && m.metrics.CPU != nil {
			cpuCount := len(m.metrics.CPU.CoreUsage)
			if cpuCount > 0 {
				threadsLine := fmt.Sprintf("%d threads", cpuCount)
				leftLines = append(leftLines, threadsLine)
				styledLeftLines = append(styledLeftLines, threadsLine)
			}
		}

		// Add uptime if available
		if m.metrics != nil && m.metrics.System != nil && m.metrics.System.BootTime != "" {
			uptimeLine := fmt.Sprintf("Uptime: %s", m.metrics.System.BootTime)
			leftLines = append(leftLines, uptimeLine)
			styledLeftLines = append(styledLeftLines, uptimeLine)
		}
	}


	// Calculate logo dimensions from raw strings first (use lipgloss width for Unicode)
	logoWidth := 0
	for _, line := range logo {
		lineWidth := lipgloss.Width(line)
		if lineWidth > logoWidth {
			logoWidth = lineWidth
		}
	}
	
	// Build logo with preserved ASCII art alignment
	logoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	// Apply style to each line individually to preserve alignment
	var styledLogoLines []string
	for _, line := range logo {
		styledLogoLines = append(styledLogoLines, logoStyle.Render(line))
	}

	// Calculate available space
	availableWidth := width - 4 // account for borders and padding

	// For very small screens, stack vertically
	if availableWidth < 35 || logoWidth + 15 > availableWidth {
		// Stack layout: system info on top, logo below
		var finalContent string
		
		// Truncate left content if needed
		maxLeftWidth := availableWidth
		truncatedStyledLines := make([]string, len(styledLeftLines))
		for i, styledLine := range styledLeftLines {
			rawLine := leftLines[i]
			rawLineWidth := lipgloss.Width(rawLine)
			if rawLineWidth > maxLeftWidth {
				if maxLeftWidth > 3 {
					truncLen := maxLeftWidth - 3
					if i == 0 { // distro line is styled
						distroStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5FBF")).Bold(true)
						truncatedStyledLines[i] = distroStyle.Render(rawLine[:truncLen]) + "..."
					} else {
						truncatedStyledLines[i] = rawLine[:truncLen] + "..."
					}
				} else {
					if i == 0 { // distro line is styled
						distroStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5FBF")).Bold(true)
						truncatedStyledLines[i] = distroStyle.Render(rawLine[:maxLeftWidth])
					} else {
						truncatedStyledLines[i] = rawLine[:maxLeftWidth]
					}
				}
			} else {
				truncatedStyledLines[i] = styledLine
			}
		}
		
		finalContent = strings.Join(truncatedStyledLines, "\n")
		if len(truncatedStyledLines) > 0 && len(logo) > 0 {
			finalContent += "\n\n" // spacing
		}
		
		// Add logo, potentially centered
		for i, logoLine := range styledLogoLines {
			rawLine := logo[i] // use raw line for width calculation
			rawLineWidth := lipgloss.Width(rawLine)
			if rawLineWidth < availableWidth {
				padding := (availableWidth - rawLineWidth) / 2
				if padding > 0 {
					logoLine = strings.Repeat(" ", padding) + logoLine
				}
			}
			finalContent += logoLine + "\n"
		}
		
		// Remove trailing newline
		finalContent = strings.TrimSuffix(finalContent, "\n")
		
		// Ensure content fills allocated height
		contentHeight := lipgloss.Height(finalContent)
		innerHeight := height - 2
		if contentHeight < innerHeight {
			padding := strings.Repeat("\n", innerHeight-contentHeight)
			finalContent = finalContent + padding
		} else if contentHeight > innerHeight {
			lines := strings.Split(finalContent, "\n")
			finalContent = strings.Join(lines[:innerHeight], "\n")
		}
		
		return style.Render(finalContent)
	}

	// Side-by-side layout for wider screens
	maxLeftWidth := availableWidth - logoWidth - 2 // -2 for separator
	if maxLeftWidth < 10 {
		maxLeftWidth = 10 // minimum
	}

	// Truncate left content if needed using raw lengths but styled display
	truncatedStyledLines := make([]string, len(styledLeftLines))
	truncatedRawLines := make([]string, len(leftLines))
	for i, styledLine := range styledLeftLines {
		rawLine := leftLines[i]
		rawLineWidth := lipgloss.Width(rawLine)
		if rawLineWidth > maxLeftWidth {
			if maxLeftWidth > 3 {
				truncLen := maxLeftWidth - 3
				truncatedRawLines[i] = rawLine[:truncLen] + "..."
				// For styled line, need to handle the styling properly
				if i == 0 { // distro line is styled
					distroStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5FBF")).Bold(true)
					truncatedStyledLines[i] = distroStyle.Render(rawLine[:truncLen]) + "..."
				} else {
					truncatedStyledLines[i] = rawLine[:truncLen] + "..."
				}
			} else {
				truncatedRawLines[i] = rawLine[:maxLeftWidth]
				if i == 0 { // distro line is styled
					distroStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5FBF")).Bold(true)
					truncatedStyledLines[i] = distroStyle.Render(rawLine[:maxLeftWidth])
				} else {
					truncatedStyledLines[i] = rawLine[:maxLeftWidth]
				}
			}
		} else {
			truncatedRawLines[i] = rawLine
			truncatedStyledLines[i] = styledLine
		}
	}

	// Build content line by line to ensure perfect alignment
	maxLines := len(truncatedStyledLines)
	if len(styledLogoLines) > maxLines {
		maxLines = len(styledLogoLines)
	}
	
	var finalLines []string
	for i := 0; i < maxLines; i++ {
		var leftPart, rightPart string
		var leftRawLen int
		
		// Get left part (system info)
		if i < len(truncatedStyledLines) {
			leftPart = truncatedStyledLines[i]
			leftRawLen = lipgloss.Width(truncatedRawLines[i])
		}
		
		// Get right part (logo)  
		if i < len(styledLogoLines) {
			rightPart = styledLogoLines[i]
		}
		
		// Pad left part to exact width using raw length
		if leftRawLen < maxLeftWidth {
			leftPart += strings.Repeat(" ", maxLeftWidth-leftRawLen)
		}
		
		// Combine with spacing
		finalLines = append(finalLines, leftPart+"  "+rightPart)
	}
	
	finalContent := strings.Join(finalLines, "\n")

	// Ensure content fills allocated height
	contentHeight := lipgloss.Height(finalContent)
	innerHeight := height - 2 // subtract borders

	if contentHeight < innerHeight {
		padding := strings.Repeat("\n", innerHeight-contentHeight)
		finalContent = finalContent + padding
	} else if contentHeight > innerHeight {
		lines := strings.Split(finalContent, "\n")
		finalContent = strings.Join(lines[:innerHeight], "\n")
	}

	return style.Render(finalContent)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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

// getAllDistroLogos returns all available distro logos with their names and colors
func getAllDistroLogos() []struct {
	name  string
	logo  []string
	color string
} {
	return []struct {
		name  string
		logo  []string
		color string
	}{
		{
			"Arch Linux",
			[]string{
				"      /\\",
				"     /  \\",
				"    /    \\",
				"   /      \\",
				"  /   ,,   \\",
				" /   |  |   \\",
				"/_-''    ''-_\\",
			},
			"#1793D1",
		},
		{
			"Ubuntu",
			[]string{
				"         _",
				"     ---(_)",
				" _/  ---  \\",
				"(_) |   |",
				"  \\  --- _/",
				"     ---(_)",
			},
			"#E95420",
		},
		{
			"Fedora",
			[]string{
				"        ,'''''.",
				"       |   ,.  | ",
				"       |  |  '_'",
				"  ,....|  |..",
				".'  ,_;|   ..'",
				"|  |   |  |",
				"|  ',_,'  |",
				" '.     ,'",
				"   '''''",
			},
			"#0B57A4",
		},
		{
			"NixOS",
			[]string{
				"  ▗▄   ▗▄ ▄▖",
				" ▄▄🬸█▄▄▄🬸█▛ ▃",
				"   ▟▛    ▜▃▟🬕",
				"🬋🬋🬫█      █🬛🬋🬋",
				" 🬷▛🮃▙    ▟▛",
				" 🮃 ▟█🬴▀▀▀█🬴▀▀",
				"  ▝▀ ▀▘   ▀▘",
			},
			"#5294e2",
		},
		{
			"Debian",
			[]string{
				"  _____",
				" /  __ \\",
				"|  /    |",
				"|  \\___-",
				"-_",
				"  --_",
			},
			"#D70A53",
		},
		{
			"Linux Mint",
			[]string{
				" __________",
				"|_          \\",
				"  | | _____ |",
				"  | | | | | |",
				"  | | | | | |",
				"  | \\____/ |",
				"  \\_________/",
			},
			"#3EB489",
		},
		{
			"Gentoo",
			[]string{
				" *-----*",
				"(       \\",
				"\\    0   \\",
				" \\        )",
				" /      _/",
				"(     _-",
				"\\____-",
			},
			"#54487A",
		},
		{
			"CachyOS",
			[]string{
				"   /''''''''''''/",
				"  /''''''''''''/",
				" /''''''/",
				"/''''''/",
				"\\......\\",
				" \\......\\",
				"  \\............../",
				"   \\............./",
			},
			"#08A283",
		},
		{
			"elementary OS",
			[]string{
				"  _______",
				" / ____  \\",
				"/  |  /  /\\",
				"|__\\ /  / |",
				"\\   /__/  /",
				" \\_______/",
			},
			"#64BAFF",
		},
		{
			"Pop!_OS",
			[]string{
				"______",
				"\\   * \\        *_",
				" \\ \\ \\ \\      / /",
				"  \\ \\_\\ \\    / /",
				"   \\  ___\\  /_/",
				"    \\ \\    _",
				"   __\\_\\__(_)_",
				"  (___________)`",
			},
			"#48B9C7",
		},
		{
			"openSUSE",
			[]string{
				"  _______",
				"**|   ** \\",
				"     / .\\ \\",
				"     \\__/ |",
				"   _______|",
				"   \\_______",
				"__________/",
			},
			"#73BA25",
		},
		{
			"EndeavourOS",
			[]string{
				"          /o.",
				"        /sssso-",
				"      /ossssssso:",
				"    /ssssssssssso+",
				"  /ssssssssssssssso+",
				"//osssssssssssssso+-",
				" `+++++++++++++++-`",
			},
			"#7F3FBF",
		},
		{
			"Generic Linux",
			[]string{
				"    ___",
				"   (.. \\",
				"   (<> |",
				"  //  \\ \\",
				" ( |  | /|",
				"_/\\ __)/_)",
				"\\/____\\/",
			},
			"#7D56F4",
		},
	}
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
