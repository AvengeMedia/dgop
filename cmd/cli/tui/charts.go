package tui

import (
	"fmt"
	"strings"
)

func (m *ResponsiveTUIModel) renderNetworkChart(width int) string {
	if len(m.networkHistory) < 2 {
		return panelStyle.Width(width).Render("Network chart (collecting data...)")
	}

	chartWidth := width - 4
	chartHeight := 8

	var maxTx, maxRx float64
	for _, sample := range m.networkHistory {
		if sample.txRate > maxTx {
			maxTx = sample.txRate
		}
		if sample.rxRate > maxRx {
			maxRx = sample.rxRate
		}
	}

	if maxTx == 0 && maxRx == 0 {
		return panelStyle.Width(width).Render("Network chart (no activity)")
	}

	chart := make([][]rune, chartHeight)
	for i := range chart {
		chart[i] = make([]rune, chartWidth)
		for j := range chart[i] {
			chart[i][j] = ' '
		}
	}

	halfHeight := chartHeight / 2
	samplesShown := min(len(m.networkHistory), chartWidth)

	for i := 0; i < samplesShown; i++ {
		sample := m.networkHistory[len(m.networkHistory)-samplesShown+i]
		
		if maxTx > 0 {
			txHeight := int(float64(halfHeight) * sample.txRate / maxTx)
			if txHeight > 0 {
				for h := 0; h < txHeight && h < halfHeight; h++ {
					chart[halfHeight-1-h][i] = '▲'
				}
			}
		}
		
		if maxRx > 0 {
			rxHeight := int(float64(halfHeight) * sample.rxRate / maxRx)
			if rxHeight > 0 {
				for h := 0; h < rxHeight && h < halfHeight; h++ {
					chart[halfHeight+h][i] = '▼'
				}
			}
		}
	}

	var chartLines []string
	for _, row := range chart {
		line := string(row)
		if strings.TrimSpace(line) == "" {
			line = strings.Repeat(" ", chartWidth)
		}
		chartLines = append(chartLines, line)
	}

	chartStr := strings.Join(chartLines, "\n")
	
	labels := fmt.Sprintf("Upload: %-12s Download: %-12s", formatRate(maxTx), formatRate(maxRx))
	
	content := labels + "\n" + chartStr
	
	return panelStyle.Width(width).Render(content)
}

func (m *ResponsiveTUIModel) renderDiskChart(width int) string {
	if len(m.diskHistory) < 2 {
		return panelStyle.Width(width).Render("Disk I/O chart (collecting data...)")
	}

	chartWidth := width - 4
	chartHeight := 8

	var maxWrite, maxRead float64
	for _, sample := range m.diskHistory {
		if sample.writeRate > maxWrite {
			maxWrite = sample.writeRate
		}
		if sample.readRate > maxRead {
			maxRead = sample.readRate
		}
	}

	if maxWrite == 0 && maxRead == 0 {
		return panelStyle.Width(width).Render("Disk I/O chart (no activity)")
	}

	chart := make([][]rune, chartHeight)
	for i := range chart {
		chart[i] = make([]rune, chartWidth)
		for j := range chart[i] {
			chart[i][j] = ' '
		}
	}

	halfHeight := chartHeight / 2
	samplesShown := min(len(m.diskHistory), chartWidth)

	for i := 0; i < samplesShown; i++ {
		sample := m.diskHistory[len(m.diskHistory)-samplesShown+i]
		
		if maxWrite > 0 {
			writeHeight := int(float64(halfHeight) * sample.writeRate / maxWrite)
			if writeHeight > 0 {
				for h := 0; h < writeHeight && h < halfHeight; h++ {
					chart[halfHeight-1-h][i] = '▲'
				}
			}
		}
		
		if maxRead > 0 {
			readHeight := int(float64(halfHeight) * sample.readRate / maxRead)
			if readHeight > 0 {
				for h := 0; h < readHeight && h < halfHeight; h++ {
					chart[halfHeight+h][i] = '▼'
				}
			}
		}
	}

	var chartLines []string
	for _, row := range chart {
		line := string(row)
		if strings.TrimSpace(line) == "" {
			line = strings.Repeat(" ", chartWidth)
		}
		chartLines = append(chartLines, line)
	}

	chartStr := strings.Join(chartLines, "\n")
	
	labels := fmt.Sprintf("Write: %-12s Read: %-12s", formatRate(maxWrite), formatRate(maxRead))
	
	content := labels + "\n" + chartStr
	
	return panelStyle.Width(width).Render(content)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}