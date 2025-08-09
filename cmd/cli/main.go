package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bbedward/DankMaterialShell/dankgop/gops"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

var (
	jsonOutput     bool
	procSortBy     string
	procLimit      int
	disableProcCPU bool
	metaModules    []string
	gpuPciId       string
	metaGPUPciIds  []string
	cpuSampleData  string
	procSampleData string
)

var style = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#7C7C7C")).
	MarginLeft(1).
	MarginRight(1)

var titleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FAFAFA")).
	Background(lipgloss.Color("#7D56F4")).
	PaddingTop(0).
	PaddingLeft(1).
	PaddingRight(1)

var keyStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FAFAFA"))

var valueStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#C9C9C9"))

var headerStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#8B5FBF"))

func printHeader() {
	header := `
 ██████╗  █████╗ ███╗   ██╗██╗  ██╗
 ██╔══██╗██╔══██╗████╗  ██║██║ ██╔╝
 ██║  ██║███████║██╔██╗ ██║█████╔╝ 
 ██║  ██║██╔══██║██║╚██╗██║██╔═██╗ 
 ██████╔╝██║  ██║██║ ╚████║██║  ██╗
 ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝`
	fmt.Println(headerStyle.Render(header))
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&disableProcCPU, "no-cpu", false, "Disable CPU calculation for faster process listing")

	allCmd.Flags().StringVar(&procSortBy, "sort", "cpu", "Sort processes by (cpu, memory, name, pid)")
	allCmd.Flags().IntVar(&procLimit, "limit", 0, "Limit number of processes (0 = no limit)")
	allCmd.Flags().StringVar(&cpuSampleData, "cpu-sample", "", "CPU sample data (JSON encoded)")
	allCmd.Flags().StringVar(&procSampleData, "proc-sample", "", "Process sample data (JSON encoded)")

	cpuCmd.Flags().StringVar(&cpuSampleData, "sample", "", "CPU sample data (JSON encoded)")

	processesCmd.Flags().StringVar(&procSortBy, "sort", "cpu", "Sort processes by (cpu, memory, name, pid)")
	processesCmd.Flags().IntVar(&procLimit, "limit", 0, "Limit number of processes (0 = no limit)")
	processesCmd.Flags().StringVar(&procSampleData, "sample", "", "Process sample data (JSON encoded)")

	metaCmd.Flags().StringSliceVar(&metaModules, "modules", []string{"all"}, "Modules to include (cpu,memory,network,etc)")
	metaCmd.Flags().StringVar(&procSortBy, "sort", "cpu", "Sort processes by (cpu, memory, name, pid)")
	metaCmd.Flags().IntVar(&procLimit, "limit", 0, "Limit number of processes (0 = no limit)")
	metaCmd.Flags().StringSliceVar(&metaGPUPciIds, "gpu-pci-ids", []string{}, "PCI IDs for GPU temperatures (e.g., 10de:2684,1002:164e)")
	metaCmd.Flags().StringVar(&cpuSampleData, "cpu-sample", "", "CPU sample data (JSON encoded)")
	metaCmd.Flags().StringVar(&procSampleData, "proc-sample", "", "Process sample data (JSON encoded)")

	gpuTempCmd.Flags().StringVar(&gpuPciId, "pci-id", "", "PCI ID of GPU to get temperature (e.g., 10de:2684)")
	gpuTempCmd.MarkFlagRequired("pci-id")
}

var rootCmd = &cobra.Command{
	Use:   "dankgop",
	Short: "A system monitoring tool",
	Long:  "dankgop provides APIs and CLI commands to monitor system metrics such as CPU, GPU, memory, disk, network, and processes.",
	Run: func(cmd *cobra.Command, args []string) {
		printHeader()
		cmd.Help()
	},
}

func main() {
	gopsUtil := gops.NewGopsUtil()

	// Set the gopsUtil in context for commands
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		cmd.SetContext(cmd.Context())
	}

	setupCommands(gopsUtil)

	if err := rootCmd.Execute(); err != nil {
		log.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}

func setupCommands(gopsUtil *gops.GopsUtil) {
	rootCmd.AddCommand(allCmd)
	rootCmd.AddCommand(cpuCmd)
	rootCmd.AddCommand(memoryCmd)
	rootCmd.AddCommand(networkCmd)
	rootCmd.AddCommand(diskCmd)
	rootCmd.AddCommand(processesCmd)
	rootCmd.AddCommand(systemCmd)
	rootCmd.AddCommand(hardwareCmd)
	rootCmd.AddCommand(gpuCmd)
	rootCmd.AddCommand(gpuTempCmd)
	rootCmd.AddCommand(metaCmd)
	rootCmd.AddCommand(modulesCmd)
	rootCmd.AddCommand(serverCmd)

	// Set gopsUtil for all commands
	allCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAllCommand(gopsUtil)
	}

	cpuCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCpuCommand(gopsUtil)
	}

	memoryCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMemoryCommand(gopsUtil)
	}

	networkCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runNetworkCommand(gopsUtil)
	}

	diskCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDiskCommand(gopsUtil)
	}

	processesCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runProcessesCommand(gopsUtil)
	}

	systemCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSystemCommand(gopsUtil)
	}

	hardwareCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runHardwareCommand(gopsUtil)
	}

	gpuCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runGPUCommand(gopsUtil)
	}

	gpuTempCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runGPUTempCommand(gopsUtil)
	}

	metaCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMetaCommand(gopsUtil)
	}

	modulesCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runModulesCommand(gopsUtil)
	}
}

func outputJSON(data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	fmt.Println(string(jsonData))
	return nil
}

func parseProcessSortBy(sortBy string, cpuDisabled bool) gops.ProcSortBy {
	// If CPU is disabled and user chose CPU sort, default to memory
	if cpuDisabled && sortBy == "cpu" {
		return gops.SortByMemory
	}

	switch sortBy {
	case "cpu":
		return gops.SortByCPU
	case "memory":
		return gops.SortByMemory
	case "name":
		return gops.SortByName
	case "pid":
		return gops.SortByPID
	default:
		// Default behavior: CPU if enabled, memory if CPU disabled
		if cpuDisabled {
			return gops.SortByMemory
		}
		return gops.SortByCPU
	}
}
