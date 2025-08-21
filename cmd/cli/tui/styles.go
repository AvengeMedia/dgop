package tui

import (
	"github.com/AvengeMedia/dgop/models"
	"github.com/charmbracelet/lipgloss"
)

func (m *ResponsiveTUIModel) getColors() *models.ColorPalette {
	if m.colorManager != nil {
		return m.colorManager.GetPalette()
	}
	return models.DefaultColorPalette()
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

func (m *ResponsiveTUIModel) textStyle() lipgloss.Style {
	colors := m.getColors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.UI.TextSecondary))
}

func (m *ResponsiveTUIModel) boldTextStyle() lipgloss.Style {
	colors := m.getColors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.UI.TextPrimary)).
		Bold(true)
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
	
	if temp > 85 {
		return colors.Temperature.Danger
	} else if temp > 70 {
		return colors.Temperature.Hot
	} else if temp > 50 {
		return colors.Temperature.Warm
	}
	return colors.Temperature.Cold
}

func (m *ResponsiveTUIModel) getNetworkColors() (string, string) {
	colors := m.getColors()
	return colors.Charts.NetworkDownload, colors.Charts.NetworkUpload
}