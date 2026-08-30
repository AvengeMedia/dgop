package tui

import "strings"

var chartRamp = []rune("▁▂▃▄▅▆▇█")

func blockChart(values []float64, width, height int, max float64) []string {
	rows := make([]string, height)
	if width <= 0 || height <= 0 {
		return rows
	}
	if max <= 0 {
		max = 1
	}

	cols := values
	if len(cols) > width {
		cols = cols[len(cols)-width:]
	}
	leftPad := strings.Repeat(" ", width-len(cols))

	builders := make([]strings.Builder, height)
	for i := range builders {
		builders[i].WriteString(leftPad)
	}

	for _, v := range cols {
		eighths := int(v / max * float64(height*8))
		if v > 0 && eighths == 0 {
			eighths = 1
		}
		if eighths > height*8 {
			eighths = height * 8
		}

		for row := range height {
			depth := eighths - (height-1-row)*8
			switch {
			case depth <= 0:
				builders[row].WriteRune(' ')
			case depth >= 8:
				builders[row].WriteRune('█')
			default:
				builders[row].WriteRune(chartRamp[depth-1])
			}
		}
	}

	for i := range rows {
		rows[i] = builders[i].String()
	}
	return rows
}

func sparkline(values []float64, width int, max float64) string {
	return blockChart(values, width, 1, max)[0]
}
