package models

type ColorPalette struct {
	UI           UIColors           `json:"ui"`
	Charts       ChartColors        `json:"charts"`
	ProgressBars ProgressBarColors  `json:"progress_bars"`
	Temperature  TemperatureColors  `json:"temperature"`
	Status       StatusColors       `json:"status"`
}

type UIColors struct {
	BorderPrimary       string `json:"border_primary"`
	BorderSecondary     string `json:"border_secondary"`
	HeaderBackground    string `json:"header_background"`
	HeaderText          string `json:"header_text"`
	FooterBackground    string `json:"footer_background"`
	FooterText          string `json:"footer_text"`
	TextPrimary         string `json:"text_primary"`
	TextSecondary       string `json:"text_secondary"`
	TextAccent          string `json:"text_accent"`
	SelectionBackground string `json:"selection_background"`
	SelectionText       string `json:"selection_text"`
}

type ChartColors struct {
	NetworkDownload string `json:"network_download"`
	NetworkUpload   string `json:"network_upload"`
	NetworkLine     string `json:"network_line"`
	CPUCoreLow      string `json:"cpu_core_low"`
	CPUCoreMedium   string `json:"cpu_core_medium"`
	CPUCoreHigh     string `json:"cpu_core_high"`
	DiskRead        string `json:"disk_read"`
	DiskWrite       string `json:"disk_write"`
}

type ProgressBarColors struct {
	MemoryLow          string `json:"memory_low"`
	MemoryMedium       string `json:"memory_medium"`
	MemoryHigh         string `json:"memory_high"`
	DiskLow            string `json:"disk_low"`
	DiskMedium         string `json:"disk_medium"`
	DiskHigh           string `json:"disk_high"`
	CPULow             string `json:"cpu_low"`
	CPUMedium          string `json:"cpu_medium"`
	CPUHigh            string `json:"cpu_high"`
	ProgressBackground string `json:"progress_background"`
}

type TemperatureColors struct {
	Cold   string `json:"cold"`
	Warm   string `json:"warm"`
	Hot    string `json:"hot"`
	Danger string `json:"danger"`
}

type StatusColors struct {
	Success string `json:"success"`
	Warning string `json:"warning"`
	Error   string `json:"error"`
	Info    string `json:"info"`
}

// DefaultColorPalette returns the default color palette matching current dgop colors
func DefaultColorPalette() *ColorPalette {
	return &ColorPalette{
		UI: UIColors{
			BorderPrimary:       "#8B5FBF",
			BorderSecondary:     "#6B46C1",
			HeaderBackground:    "#7D56F4",
			HeaderText:          "#FAFAFA",
			FooterBackground:    "#2A2A2A",
			FooterText:          "#7C7C7C",
			TextPrimary:         "#FAFAFA",
			TextSecondary:       "#C9C9C9",
			TextAccent:          "#8B5FBF",
			SelectionBackground: "#7D56F4",
			SelectionText:       "#FAFAFA",
		},
		Charts: ChartColors{
			NetworkDownload: "#A855F7",
			NetworkUpload:   "#8B5FBF",
			NetworkLine:     "#6B46C1",
			CPUCoreLow:      "#8B5FBF",
			CPUCoreMedium:   "#A855F7",
			CPUCoreHigh:     "#D946EF",
			DiskRead:        "#A855F7",
			DiskWrite:       "#8B5FBF",
		},
		ProgressBars: ProgressBarColors{
			MemoryLow:          "#8B5FBF",
			MemoryMedium:       "#A855F7", 
			MemoryHigh:         "#D946EF",
			DiskLow:            "#8B5FBF",
			DiskMedium:         "#A855F7",
			DiskHigh:           "#D946EF",
			CPULow:             "#8B5FBF",
			CPUMedium:          "#A855F7",
			CPUHigh:            "#D946EF",
			ProgressBackground: "#404040",
		},
		Temperature: TemperatureColors{
			Cold:   "#8B5FBF",
			Warm:   "#A855F7",
			Hot:    "#D946EF",
			Danger: "#FF6B6B",
		},
		Status: StatusColors{
			Success: "#22C55E",
			Warning: "#F59E0B", 
			Error:   "#EF4444",
			Info:    "#3B82F6",
		},
	}
}