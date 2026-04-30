package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kangthink/infra-god-cli/internal/alerts"
	"github.com/kangthink/infra-god-cli/internal/collector"
	"github.com/kangthink/infra-god-cli/internal/inventory"
	"github.com/kangthink/infra-god-cli/internal/output"
	sshclient "github.com/kangthink/infra-god-cli/internal/ssh"
	"github.com/spf13/cobra"
)

var (
	statusDetailFlag bool
	statusSortFlag   string
	statusWatchFlag  time.Duration
)

var statusCmd = &cobra.Command{
	Use:   "status [server...]",
	Short: "Show server status dashboard",
	Long:  "Display CPU, memory, disk, GPU, load and uptime for all servers.",
	Run:   runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusDetailFlag, "detail", false, "include Docker and network info")
	statusCmd.Flags().StringVar(&statusSortFlag, "sort", "", "sort by: cpu, mem, disk, load, uptime")
	statusCmd.Flags().DurationVar(&statusWatchFlag, "watch", 0, "refresh interval (e.g. 30s)")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) {
	cfg, servers, err := loadInventory()
	if err != nil {
		fatalErr("load config", err)
	}

	targets, err := resolveTargets(cfg, servers, args)
	if err != nil {
		fatalErr("resolve targets", err)
	}

	if statusWatchFlag > 0 {
		// Handle Ctrl+C gracefully
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		tick := 0
		for {
			fmt.Print("\033[H\033[2J") // clear screen
			tick++
			printStatus(targets)
			fmt.Printf(" %s refresh #%d | next in %s | Ctrl+C to stop\n",
				output.Dim("⟳"), tick, statusWatchFlag)

			select {
			case <-sigCh:
				fmt.Printf("\n %s Watch stopped.\n", output.Dim("⏹"))
				return
			case <-time.After(statusWatchFlag):
			}
		}
	} else {
		printStatus(targets)
	}
}

type serverStatus struct {
	server   *inventory.ResolvedServer
	result   *sshclient.Result
	cpuPct   int
	memPct   int
	diskPct  int
	load     float64
	cpus     int
	uptime   string
	gpuInfo  string
	gpuCount int
	docker   string
	osInfo   string // e.g. "ubuntu 22.04"
	kernel   string
	status   string // ok, warning, critical, error, stopped, auth_fail
}

// metric extracts the alert-relevant view of a serverStatus.
// gpuInfo is stripped of ANSI codes so alert rules can match plain text.
func (s *serverStatus) metric() alerts.Metric {
	return alerts.Metric{
		Server:  s.server.Name,
		Status:  s.status,
		DiskPct: s.diskPct,
		MemPct:  s.memPct,
		CPUs:    s.cpus,
		Load:    s.load,
		GPUInfo: output.StripANSI(s.gpuInfo),
	}
}

type statusJSON struct {
	Timestamp string             `json:"timestamp"`
	Servers   []serverStatusJSON `json:"servers"`
	Summary   statusSummaryJSON  `json:"summary"`
}

type serverStatusJSON struct {
	Name    string  `json:"name"`
	Role    string  `json:"role"`
	IP      string  `json:"ip"`
	Status  string  `json:"status"`
	CPU     int     `json:"cpu_pct"`
	Mem     int     `json:"mem_pct"`
	Disk    int     `json:"disk_pct"`
	Load    float64 `json:"load"`
	CPUs    int     `json:"cpus"`
	GPU     string  `json:"gpu,omitempty"`
	Uptime  string  `json:"uptime"`
	OS      string  `json:"os,omitempty"`
	Kernel  string  `json:"kernel,omitempty"`
	Docker  string  `json:"docker,omitempty"`
	Error   string  `json:"error,omitempty"`
}

type statusSummaryJSON struct {
	Total    int `json:"total"`
	OK       int `json:"ok"`
	Warning  int `json:"warning"`
	Error    int `json:"error"`
	Stopped  int `json:"stopped"`
}

func printStatus(targets []*inventory.ResolvedServer) {
	client := newSSHClient()
	now := time.Now().Format("2006-01-02 15:04:05")

	// Separate active and inactive
	var active []*inventory.ResolvedServer
	var inactive []serverStatus
	for _, srv := range targets {
		if !srv.IsActive() {
			inactive = append(inactive, serverStatus{
				server: srv,
				status: "stopped",
			})
		} else if srv.PrimaryIP() == "" {
			inactive = append(inactive, serverStatus{
				server: srv,
				status: "error",
			})
		} else {
			active = append(active, srv)
		}
	}

	// Run status collection in parallel
	results := sshclient.RunParallel(client, active, collector.StatusCmd, parallelFlag)

	// Parse results
	var statuses []serverStatus
	for i, srv := range active {
		s := serverStatus{
			server: srv,
			result: results[i],
			status: "ok",
		}

		if results[i].Error != nil {
			if strings.Contains(results[i].Error.Error(), "auth") || strings.Contains(results[i].Error.Error(), "handshake") {
				s.status = alerts.StatusAuthFail
			} else {
				s.status = alerts.StatusError
			}
			statuses = append(statuses, s)
			continue
		}

		parseStatusOutput(results[i].Output, &s)
		s.status = alerts.DeriveStatus(s.metric())

		statuses = append(statuses, s)
	}
	statuses = append(statuses, inactive...)

	// Sort — default by name, or user-specified
	if statusSortFlag != "" {
		sortStatuses(statuses, statusSortFlag)
	} else {
		sortStatuses(statuses, "name")
	}

	// Count summary
	var okCount, warnCount, critCount, errCount, stopCount int
	for _, s := range statuses {
		switch s.status {
		case "ok":
			okCount++
		case "warning":
			warnCount++
		case "critical":
			critCount++
		case "error", "auth_fail":
			errCount++
		case "stopped":
			stopCount++
		}
	}

	// JSON output
	if jsonFlag {
		j := statusJSON{
			Timestamp: now,
			Summary: statusSummaryJSON{
				Total:   len(statuses),
				OK:      okCount,
				Warning: warnCount + critCount,
				Error:   errCount,
				Stopped: stopCount,
			},
		}
		for _, s := range statuses {
			sj := serverStatusJSON{
				Name:   s.server.Name,
				Role:   s.server.Role,
				IP:     s.server.PrimaryIP(),
				Status: s.status,
				CPU:    s.cpuPct,
				Mem:    s.memPct,
				Disk:   s.diskPct,
				Load:   s.load,
				CPUs:   s.cpus,
				GPU:    output.StripANSI(s.gpuInfo),
				Uptime: s.uptime,
				OS:     s.osInfo,
				Kernel: s.kernel,
				Docker: s.docker,
			}
			if s.result != nil && s.result.Error != nil {
				sj.Error = s.result.Error.Error()
			}
			j.Servers = append(j.Servers, sj)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(j)
		return
	}

	// Render table
	fmt.Println(output.Header(fmt.Sprintf("INFRA-GOD STATUS                                    %s", now)))

	table := output.NewTable("SERVER", "ROLE", "OS", "KERNEL", "CPU%", "MEM%", "DISK%", "GPU", "LOAD", "UPTIME")
	for _, s := range statuses {
		icon := output.StatusIcon(s.status)
		name := fmt.Sprintf("%s %s", icon, s.server.Name)

		if s.status == "stopped" || s.status == "error" || s.status == "auth_fail" {
			table.AddRow(name, s.server.Role, "-", "-", "-", "-", "-", "-", "-", s.status)
			continue
		}

		table.AddRow(
			name,
			s.server.Role,
			s.osInfo,
			s.kernel,
			output.ColorPercent(s.cpuPct, 70, 90),
			output.ColorPercent(s.memPct, 80, 90),
			output.ColorPercent(s.diskPct, 85, 95),
			s.gpuInfo,
			output.ColorLoad(s.load, s.cpus),
			s.uptime,
		)
	}
	fmt.Println(table.Render())

	fmt.Printf(" TOTAL: %d servers | %s ok | %s warning | %s error | %s stopped\n",
		len(statuses),
		output.Green(fmt.Sprintf("%d", okCount)),
		output.Yellow(fmt.Sprintf("%d", warnCount+critCount)),
		output.Red(fmt.Sprintf("%d", errCount)),
		output.Dim(fmt.Sprintf("%d", stopCount)),
	)

	// Alerts
	var metrics []alerts.Metric
	for _, s := range statuses {
		metrics = append(metrics, s.metric())
	}
	derived := alerts.DeriveAll(metrics)
	if len(derived) > 0 {
		fmt.Println()
		fmt.Println(output.BoldRed(" ALERTS:"))
		for _, a := range derived {
			fmt.Printf("  • %s  %s\n", a.Server, a.Message)
		}
	}
	fmt.Println()
}

func parseStatusOutput(out string, s *serverStatus) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		key := parts[0]
		val := parts[1]

		switch key {
		case "CPU":
			s.cpuPct, _ = strconv.Atoi(val)
		case "MEM":
			fields := strings.Split(val, ":")
			if len(fields) >= 1 {
				s.memPct, _ = strconv.Atoi(fields[0])
			}
		case "DISK":
			fields := strings.Split(val, ":")
			if len(fields) >= 1 {
				s.diskPct, _ = strconv.Atoi(fields[0])
			}
		case "LOAD":
			s.load, _ = strconv.ParseFloat(val, 64)
		case "CPUS":
			s.cpus, _ = strconv.Atoi(val)
		case "UPTIME":
			sec, _ := strconv.Atoi(val)
			s.uptime = formatUptime(sec)
		case "OS":
			fields := strings.Split(val, ":")
			if len(fields) >= 3 {
				s.osInfo = fields[0] + " " + fields[1]
				s.kernel = fields[2]
			}
		case "GPU":
			if val == "none" {
				s.gpuInfo = output.Dim("-")
			} else if strings.Contains(strings.ToLower(val), "unable") ||
				strings.Contains(strings.ToLower(val), "nvml") ||
				strings.Contains(strings.ToLower(val), "failed") ||
				strings.Contains(strings.ToLower(val), "error") {
				s.gpuInfo = output.Yellow("driver err")
			} else {
				fields := strings.Split(val, ":")
				if len(fields) >= 5 {
					name := strings.TrimSpace(fields[0])
					util := strings.TrimSpace(fields[3])
					if _, err := strconv.Atoi(util); err != nil {
						s.gpuInfo = output.Yellow("driver err")
						continue
					}
					short := shortenGPU(name)
					s.gpuCount++
					utilInt, _ := strconv.Atoi(util)
					if s.gpuCount == 1 {
						s.gpuInfo = fmt.Sprintf("%s %d%%", short, utilInt)
					} else {
						s.gpuInfo = fmt.Sprintf("%s ×%d %d%%", short, s.gpuCount, utilInt)
					}
				} else {
					s.gpuInfo = output.Yellow("driver err")
				}
			}
		case "DOCKER":
			if val == "none" {
				s.docker = "-"
			} else {
				fields := strings.Split(val, ":")
				if len(fields) >= 4 {
					total := fields[0]
					unhealthy := fields[2]
					restarting := fields[3]
					s.docker = total
					if unhealthy != "0" {
						s.docker += fmt.Sprintf(" (%s unhealthy)", unhealthy)
					}
					if restarting != "0" {
						s.docker += fmt.Sprintf(" (%s restarting)", restarting)
					}
				}
			}
		}
	}

	// Default GPU
	if s.gpuInfo == "" {
		s.gpuInfo = output.Dim("-")
	}
}

func shortenGPU(name string) string {
	replacer := strings.NewReplacer(
		"NVIDIA GeForce ", "",
		"NVIDIA ", "",
		"Tesla ", "",
	)
	return replacer.Replace(name)
}

func formatUptime(seconds int) string {
	if seconds < 3600 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	if seconds < 86400 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	days := seconds / 86400
	if days < 7 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dw", days/7)
}

func sortStatuses(statuses []serverStatus, by string) {
	sort.Slice(statuses, func(i, j int) bool {
		switch by {
		case "cpu":
			return statuses[i].cpuPct > statuses[j].cpuPct
		case "mem":
			return statuses[i].memPct > statuses[j].memPct
		case "disk":
			return statuses[i].diskPct > statuses[j].diskPct
		case "load":
			return statuses[i].load > statuses[j].load
		default:
			return statuses[i].server.Name < statuses[j].server.Name
		}
	})
}
