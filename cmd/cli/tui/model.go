package tui

import (
	"time"

	"github.com/AvengeMedia/dgop/config"
	"github.com/AvengeMedia/dgop/gops"
	"github.com/AvengeMedia/dgop/models"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type NetworkSample struct {
	timestamp time.Time
	rxBytes   uint64
	txBytes   uint64
	rxRate    float64
	txRate    float64
}

type DiskSample struct {
	timestamp  time.Time
	readBytes  uint64
	writeBytes uint64
	readRate   float64
	writeRate  float64
	device     string
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type colorUpdateMsg struct{}

type ResponsiveTUIModel struct {
	gops         *gops.GopsUtil
	colorManager *config.ColorManager
	metrics      *models.SystemMetrics
	width        int
	height       int
	err          error
	lastUpdate   time.Time

	processTable table.Model
	viewport     viewport.Model

	hardware *models.SystemHardware

	networkHistory       []NetworkSample
	maxNetHistory        int
	networkCursor        string
	lastNetworkUpdate    time.Time
	selectedInterfaceName string

	diskHistory    []DiskSample
	maxDiskHistory int
	diskCursor     string
	lastDiskUpdate time.Time

	systemTemperatures []models.TemperatureSensor
	lastTempUpdate     time.Time

	sortBy      gops.ProcSortBy
	procLimit   int
	ready       bool
	showDetails bool
	selectedPID int32

	distroLogo  []string
	distroColor string
	
	logoTestMode     bool
	currentLogoIndex int
	lastLogoUpdate   time.Time

	hideCPUCores   bool
	summarizeCores bool
}

func (m *ResponsiveTUIModel) Cleanup() {
	if m.colorManager != nil {
		m.colorManager.Close()
	}
}
