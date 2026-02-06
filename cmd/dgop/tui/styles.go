package tui

import (
	"github.com/AvengeMedia/dgop/models"
	"github.com/charmbracelet/lipgloss"
)

func (m *ResponsiveTUIModel) getColors() *models.ColorPalette {
	if m.cachedColors != nil {
		return m.cachedColors
	}
	if m.colorManager != nil {
		m.cachedColors = m.colorManager.GetPalette()
	} else {
		m.cachedColors = models.DefaultColorPalette()
	}
	m.updateCachedNetChars()
	return m.cachedColors
}

func (m *ResponsiveTUIModel) updateCachedNetChars() {
	if m.cachedColors == nil {
		return
	}
	m.cachedNetDownChar = lipgloss.NewStyle().Foreground(lipgloss.Color(m.cachedColors.Charts.NetworkDownload)).Render("█")
	m.cachedNetUpChar = lipgloss.NewStyle().Foreground(lipgloss.Color(m.cachedColors.Charts.NetworkUpload)).Render("▓")
}

func (m *ResponsiveTUIModel) refreshColorCache() {
	if m.colorManager != nil {
		m.cachedColors = m.colorManager.GetPalette()
	} else {
		m.cachedColors = models.DefaultColorPalette()
	}
	m.updateCachedNetChars()
}

func (m *ResponsiveTUIModel) panelStyle(width, height int) lipgloss.Style {
	colors := m.getColors()
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(colors.UI.BorderPrimary)).
		Padding(0, 1).
		Width(width).
		MaxHeight(height)
}

func (m *ResponsiveTUIModel) titleStyle() lipgloss.Style {
	colors := m.getColors()
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colors.UI.TextAccent))
}

func (m *ResponsiveTUIModel) headerStyle() lipgloss.Style {
	colors := m.getColors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.UI.HeaderText)).
		Background(lipgloss.Color(colors.UI.HeaderBackground)).
		Bold(true).
		Width(m.width).
		Padding(0, 2)
}

func (m *ResponsiveTUIModel) footerStyle() lipgloss.Style {
	colors := m.getColors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.UI.FooterText)).
		Background(lipgloss.Color(colors.UI.FooterBackground)).
		Width(m.width).
		Padding(0, 2)
}

func (m *ResponsiveTUIModel) getProgressBarColor(usage float64, colorType string) string {
	colors := m.getColors()

	switch colorType {
	case "memory":
		if usage > 80 {
			return colors.ProgressBars.MemoryHigh
		} else if usage > 60 {
			return colors.ProgressBars.MemoryMedium
		}
		return colors.ProgressBars.MemoryLow
	case "disk":
		if usage > 90 {
			return colors.ProgressBars.DiskHigh
		} else if usage > 70 {
			return colors.ProgressBars.DiskMedium
		}
		return colors.ProgressBars.DiskLow
	case "cpu":
		if usage > 80 {
			return colors.ProgressBars.CPUHigh
		} else if usage > 60 {
			return colors.ProgressBars.CPUMedium
		}
		return colors.ProgressBars.CPULow
	default:
		return colors.ProgressBars.MemoryLow
	}
}

func (m *ResponsiveTUIModel) getTemperatureColor(temp float64) string {
	colors := m.getColors()

	switch {
	case temp > 85:
		return colors.Temperature.Danger
	case temp > 70:
		return colors.Temperature.Hot
	case temp > 50:
		return colors.Temperature.Warm
	default:
		return colors.Temperature.Cold
	}
}
