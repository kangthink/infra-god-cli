package cmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/kangthink/infra-god-cli/internal/collector"
	"github.com/kangthink/infra-god-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	inspectHWFlag       bool
	inspectGPUFlag      bool
	inspectDockerFlag   bool
	inspectStorageFlag  bool
	inspectNetworkFlag  bool
	inspectSoftwareFlag bool
	inspectServicesFlag bool
	inspectSaveFlag     bool
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <server>",
	Short: "Deep inspection of a server",
	Long:  "Collect detailed hardware, software, Docker, network, and service information.",
	Args:  cobra.ExactArgs(1),
	Run:   runInspect,
}

func init() {
	inspectCmd.Flags().BoolVar(&inspectHWFlag, "hw", false, "hardware only")
	inspectCmd.Flags().BoolVar(&inspectGPUFlag, "gpu", false, "GPU only")
	inspectCmd.Flags().BoolVar(&inspectDockerFlag, "docker", false, "Docker only")
	inspectCmd.Flags().BoolVar(&inspectStorageFlag, "storage", false, "storage only")
	inspectCmd.Flags().BoolVar(&inspectNetworkFlag, "network", false, "network only")
	inspectCmd.Flags().BoolVar(&inspectSoftwareFlag, "software", false, "software only")
	inspectCmd.Flags().BoolVar(&inspectServicesFlag, "services", false, "systemd services only")
	inspectCmd.Flags().BoolVar(&inspectSaveFlag, "save", false, "save output to reports/")
	rootCmd.AddCommand(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) {
	_, servers, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	srv, ok := servers[args[0]]
	if !ok {
		fatalErr("unknown server", fmt.Errorf("%s", args[0]))
	}

	if !srv.IsActive() {
		fmt.Printf(" %s %s is stopped\n", output.Dim("⏹️"), srv.Name)
		return
	}

	client := newSSHClient()

	// Check if specific section requested
	showAll := !inspectHWFlag && !inspectGPUFlag && !inspectDockerFlag &&
		!inspectStorageFlag && !inspectNetworkFlag && !inspectSoftwareFlag && !inspectServicesFlag

	fmt.Printf("\n %s\n", output.Bold(fmt.Sprintf("═══ INSPECT: %s (%s)  role: %s ═══", srv.Name, srv.PrimaryIP(), srv.Role)))

	// Collect sections in parallel
	type section struct {
		name    string
		command string
		output  string
		err     error
	}

	var sections []section
	if showAll || inspectHWFlag {
		sections = append(sections, section{name: "hardware", command: collector.InspectHWCmd})
	}
	if showAll || inspectGPUFlag {
		sections = append(sections, section{name: "gpu", command: collector.InspectGPUCmd})
	}
	if showAll || inspectStorageFlag {
		sections = append(sections, section{name: "storage", command: collector.InspectStorageCmd})
	}
	if showAll || inspectSoftwareFlag {
		sections = append(sections, section{name: "software", command: collector.InspectSoftwareCmd})
	}
	if showAll || inspectDockerFlag {
		sections = append(sections, section{name: "docker", command: collector.InspectDockerCmd})
	}
	if showAll || inspectNetworkFlag {
		sections = append(sections, section{name: "network", command: collector.InspectNetworkCmd})
	}
	if showAll || inspectServicesFlag {
		sections = append(sections, section{name: "services", command: collector.InspectServicesCmd})
	}
	if showAll {
		sections = append(sections, section{name: "users", command: collector.UsersCmd})
	}

	// Run all sections in parallel
	var wg sync.WaitGroup
	for i := range sections {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			result := client.Run(srv, sections[idx].command)
			sections[idx].output = result.Output
			sections[idx].err = result.Error
		}(i)
	}
	wg.Wait()

	// Print results in order
	var fullOutput strings.Builder
	for _, sec := range sections {
		if sec.err != nil {
			fmt.Printf("\n %s %s: %v\n", output.Red("❌"), sec.name, sec.err)
			continue
		}
		formatted := formatSection(sec.name, sec.output)
		fmt.Print(formatted)
		fullOutput.WriteString(formatted)
	}

	// Save to file
	if inspectSaveFlag {
		// TODO: save to reports/
		fmt.Printf("\n %s Saved to reports/%s-inspect.txt\n", output.Green("✅"), srv.Name)
	}
	fmt.Println()
}

func formatSection(name string, raw string) string {
	var sb strings.Builder
	sb.WriteString(output.SubHeader(strings.ToUpper(name)))
	sb.WriteString("\n")

	switch name {
	case "hardware":
		sectionNames := map[string]string{
			"===CPU===":       "CPU",
			"===GPU===":       "GPU",
			"===GPU_PROCS===": "GPU Processes",
			"===RAM===":       "Memory",
			"===STORAGE===":   "Storage",
		}
		currentSection := ""
		hasProcs := false
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if label, ok := sectionNames[line]; ok {
				currentSection = line
				sb.WriteString(fmt.Sprintf("\n  %s\n", output.Bold(label)))
				if line == "===GPU_PROCS===" {
					sb.WriteString(fmt.Sprintf("    %-8s %-12s %s\n",
						output.Dim("PID"), output.Dim("GPU MEM"), output.Dim("COMMAND")))
				}
				continue
			}
			if strings.HasPrefix(line, "===") || strings.HasPrefix(line, "---INODE---") {
				currentSection = line
				continue
			}
			switch currentSection {
			case "===GPU_PROCS===":
				if line == "none" {
					sb.WriteString("    No GPU processes\n")
					continue
				}
				parts := strings.SplitN(line, ", ", 4)
				if len(parts) >= 4 {
					hasProcs = true
					pid := strings.TrimSpace(parts[1])
					mem := strings.TrimSpace(parts[2]) + " MiB"
					cmd := strings.TrimSpace(parts[3])
					if idx := strings.LastIndex(cmd, "/"); idx > 20 {
						cmd = "..." + cmd[idx:]
					}
					sb.WriteString(fmt.Sprintf("    %-8s %-12s %s\n", pid, mem, cmd))
				}
			case "===GPU===":
				if line == "none" {
					sb.WriteString("    No GPU detected\n")
				} else {
					parts := strings.SplitN(line, ", ", 7)
					if len(parts) >= 7 {
						idx := strings.TrimSpace(parts[0])
						name := strings.TrimSpace(parts[1])
						memTotal := strings.TrimSpace(parts[2])
						memUsed := strings.TrimSpace(parts[3])
						util := strings.TrimSpace(parts[4])
						temp := strings.TrimSpace(parts[5])
						driver := strings.TrimSpace(parts[6])
						sb.WriteString(fmt.Sprintf("    [%s] %s  %s/%s  util:%s  temp:%s°C  driver:%s\n",
							idx, name, memUsed, memTotal, util, temp, driver))
					} else {
						sb.WriteString(fmt.Sprintf("    %s\n", line))
					}
				}
			case "===RAM===":
				sb.WriteString(fmt.Sprintf("    %s\n", line))
			case "===STORAGE===":
				sb.WriteString(fmt.Sprintf("    %s\n", line))
			default:
				sb.WriteString(fmt.Sprintf("    %s\n", line))
			}
		}
		_ = hasProcs
	case "gpu":
		currentSection := ""
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if line == "===GPU===" {
				currentSection = "gpu"
				continue
			}
			if line == "===GPU_PROCS===" {
				currentSection = "procs"
				sb.WriteString(fmt.Sprintf("\n  %s\n", output.Bold("Processes")))
				sb.WriteString(fmt.Sprintf("    %-8s %-12s %s\n",
					output.Dim("PID"), output.Dim("GPU MEM"), output.Dim("COMMAND")))
				continue
			}
			if line == "===CUDA===" {
				currentSection = "cuda"
				continue
			}
			if line == "===NVIDIA_DRIVER===" {
				currentSection = "driver"
				continue
			}
			if strings.HasPrefix(line, "===") {
				currentSection = ""
				continue
			}
			switch currentSection {
			case "gpu":
				if line == "none" {
					sb.WriteString("  No GPU detected\n")
					continue
				}
				parts := strings.SplitN(line, ", ", 9)
				if len(parts) >= 9 {
					idx := strings.TrimSpace(parts[0])
					name := strings.TrimSpace(parts[1])
					memTotal := strings.TrimSpace(parts[2])
					memUsed := strings.TrimSpace(parts[3])
					util := strings.TrimSpace(parts[4])
					temp := strings.TrimSpace(parts[5])
					driver := strings.TrimSpace(parts[6])
					pcieGen := strings.TrimSpace(parts[7])
					pcieWidth := strings.TrimSpace(parts[8])
					sb.WriteString(fmt.Sprintf("  [%s] %s\n", idx, output.Bold(name)))
					sb.WriteString(fmt.Sprintf("      VRAM: %s / %s  util: %s  temp: %s°C\n", memUsed, memTotal, util, temp))
					sb.WriteString(fmt.Sprintf("      Driver: %s  PCIe: gen%s x%s\n", driver, pcieGen, pcieWidth))
				} else if len(parts) >= 7 {
					idx := strings.TrimSpace(parts[0])
					name := strings.TrimSpace(parts[1])
					memTotal := strings.TrimSpace(parts[2])
					memUsed := strings.TrimSpace(parts[3])
					util := strings.TrimSpace(parts[4])
					temp := strings.TrimSpace(parts[5])
					driver := strings.TrimSpace(parts[6])
					sb.WriteString(fmt.Sprintf("  [%s] %s\n", idx, output.Bold(name)))
					sb.WriteString(fmt.Sprintf("      VRAM: %s / %s  util: %s  temp: %s°C  driver: %s\n", memUsed, memTotal, util, temp, driver))
				} else {
					sb.WriteString(fmt.Sprintf("  %s\n", line))
				}
			case "procs":
				if line == "none" {
					sb.WriteString("    No GPU processes\n")
					continue
				}
				parts := strings.SplitN(line, ", ", 4)
				if len(parts) >= 4 {
					pid := strings.TrimSpace(parts[1])
					mem := strings.TrimSpace(parts[2]) + " MiB"
					cmd := strings.TrimSpace(parts[3])
					if idx := strings.LastIndex(cmd, "/"); idx > 20 {
						cmd = "..." + cmd[idx:]
					}
					sb.WriteString(fmt.Sprintf("    %-8s %-12s %s\n", pid, mem, cmd))
				}
			case "cuda":
				if line != "N/A" {
					sb.WriteString(fmt.Sprintf("\n  CUDA: %s\n", line))
				}
			case "driver":
				if line != "N/A" {
					sb.WriteString(fmt.Sprintf("  Kernel Module: %s\n", line))
				}
			}
		}
	case "docker":
		if strings.TrimSpace(raw) == "none" || strings.TrimSpace(raw) == "" {
			sb.WriteString("  No Docker containers\n")
		} else {
			sb.WriteString(fmt.Sprintf("  %-30s %-30s %-30s %s\n",
				output.Bold("NAME"), output.Bold("STATUS"), output.Bold("IMAGE"), output.Bold("RESTART")))
			for _, line := range strings.Split(raw, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				parts := strings.Split(line, "\t")
				if len(parts) >= 4 {
					status := parts[1]
					statusStr := parts[1]
					if strings.Contains(status, "unhealthy") || strings.Contains(status, "exited") {
						statusStr = output.Red(status)
					} else if strings.Contains(status, "running") {
						statusStr = output.Green(status)
					}
					restart := parts[3]
					restartStr := restart
					if restart == "no" || restart == "" {
						restartStr = output.Yellow("no")
					} else if restart == "always" || restart == "unless-stopped" {
						restartStr = output.Green(restart)
					} else {
						restartStr = output.Cyan(restart)
					}
					sb.WriteString(fmt.Sprintf("  %-30s %-30s %-30s %s\n", parts[0], statusStr, parts[2], restartStr))
				} else if len(parts) >= 3 {
					sb.WriteString(fmt.Sprintf("  %-30s %-30s %s\n", parts[0], parts[1], parts[2]))
				}
			}
		}
	case "storage":
		currentMount := ""
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "===") {
				continue
			}
			if strings.HasPrefix(line, "Filesystem") {
				sb.WriteString(fmt.Sprintf("  %s\n", output.Bold(line)))
				continue
			}
			if strings.HasPrefix(line, "MOUNT:") {
				currentMount = strings.TrimPrefix(line, "MOUNT:")
				sb.WriteString(fmt.Sprintf("\n  %s\n", output.Cyan(fmt.Sprintf("📂 %s", currentMount))))
				continue
			}
			if strings.HasPrefix(line, "---INODE---") || strings.HasPrefix(line, "---") {
				continue
			}
			if strings.HasPrefix(line, "L1:") {
				parts := strings.SplitN(line, ":", 3)
				if len(parts) == 3 {
					size := parts[1]
					path := parts[2]
					rel := strings.TrimPrefix(path, currentMount)
					if rel == "" || rel == "/" {
						rel = path
					}
					sb.WriteString(fmt.Sprintf("    %-10s %s\n", size, rel))
				}
				continue
			}
			if strings.HasPrefix(line, "L2:") {
				parts := strings.SplitN(line, ":", 3)
				if len(parts) == 3 {
					size := parts[1]
					path := parts[2]
					rel := strings.TrimPrefix(path, currentMount)
					if rel == "" || rel == "/" {
						rel = path
					}
					sb.WriteString(fmt.Sprintf("        %-10s %s\n", size, output.Dim(rel)))
				}
				continue
			}
			if currentMount != "" {
				sb.WriteString(fmt.Sprintf("    %s\n", line))
			} else {
				sb.WriteString(fmt.Sprintf("  %s\n", line))
			}
		}
	case "services":
		inFailed := false
		inCustom := false
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "===FAILED===" {
				inFailed = true
				inCustom = false
				sb.WriteString(fmt.Sprintf("\n  %s\n", output.BoldRed("FAILED SERVICES:")))
				continue
			}
			if line == "===CUSTOM===" {
				inFailed = false
				inCustom = true
				if sb.String() != "" {
					sb.WriteString(fmt.Sprintf("\n  %s\n", output.Bold("ACTIVE SERVICES (custom):")))
				}
				continue
			}
			if line == "" {
				continue
			}
			if inFailed {
				sb.WriteString(fmt.Sprintf("  %s %s\n", output.Red("✗"), line))
			} else if inCustom {
				sb.WriteString(fmt.Sprintf("  %s %s\n", output.Green("●"), line))
			}
		}
		if !inFailed && !inCustom {
			sb.WriteString("  No failed services\n")
		}
	default:
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "===") {
				continue
			}
			sb.WriteString(fmt.Sprintf("  %s\n", line))
		}
	}

	return sb.String()
}
