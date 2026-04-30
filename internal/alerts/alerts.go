package alerts

import (
	"fmt"
	"sort"
	"strings"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Category string

const (
	CategoryDisk      Category = "disk"
	CategoryMemory    Category = "memory"
	CategoryLoad      Category = "load"
	CategoryGPU       Category = "gpu"
	CategorySSH       Category = "ssh"
	CategoryContainer Category = "container"
)

// Status values for a server.
const (
	StatusOK       = "ok"
	StatusWarning  = "warning"
	StatusCritical = "critical"
	StatusError    = "error"
	StatusAuthFail = "auth_fail"
	StatusStopped  = "stopped"
)

// Thresholds used to derive the per-server status (the table row icon).
const (
	StatusWarnDisk      = 90
	StatusCritDisk      = 95
	StatusWarnMem       = 90
	StatusCritMem       = 95
	StatusWarnLoadRatio = 0.8
)

// Thresholds used for the alert list (more sensitive than status thresholds —
// includes early warnings the operator should notice before they become red).
const (
	AlertDiskWarn  = 85
	AlertLoadRatio = 0.6
)

// Metric is the minimal slice of server data alerts care about.
// Both CLI and WebUI assemble this from their own internal types.
type Metric struct {
	Server  string
	Status  string // pre-existing status hint (e.g. "stopped"/"auth_fail" set by collector)
	DiskPct int
	MemPct  int
	CPUs    int
	Load    float64
	GPUInfo string // raw label; "driver err" / contains "error" => GPU alert
}

type Alert struct {
	Server   string   `json:"server"`
	Severity Severity `json:"severity"`
	Category Category `json:"category"`
	Message  string   `json:"message"`
}

// DeriveStatus returns the server status string given metrics.
// Preserves existing CLI behavior: warning at 90% disk/mem or 0.8 load ratio,
// critical at 95% disk/mem.
func DeriveStatus(m Metric) string {
	switch m.Status {
	case StatusStopped, StatusError, StatusAuthFail:
		return m.Status
	}
	if m.DiskPct >= StatusCritDisk || m.MemPct >= StatusCritMem {
		return StatusCritical
	}
	if m.DiskPct >= StatusWarnDisk || m.MemPct >= StatusWarnMem {
		return StatusWarning
	}
	if m.CPUs > 0 && m.Load >= float64(m.CPUs)*StatusWarnLoadRatio {
		return StatusWarning
	}
	return StatusOK
}

// Derive returns alerts for a single server based on the more sensitive
// alert thresholds.
func Derive(m Metric) []Alert {
	var out []Alert

	if m.Status == StatusAuthFail {
		out = append(out, Alert{
			Server: m.Server, Severity: SeverityCritical, Category: CategorySSH,
			Message: "SSH authentication failed",
		})
		return out
	}
	if m.Status == StatusError {
		out = append(out, Alert{
			Server: m.Server, Severity: SeverityCritical, Category: CategorySSH,
			Message: "unreachable",
		})
		return out
	}

	switch {
	case m.DiskPct >= StatusCritDisk:
		out = append(out, Alert{m.Server, SeverityCritical, CategoryDisk,
			fmt.Sprintf("disk %d%%", m.DiskPct)})
	case m.DiskPct >= AlertDiskWarn:
		out = append(out, Alert{m.Server, SeverityWarning, CategoryDisk,
			fmt.Sprintf("disk %d%%", m.DiskPct)})
	}

	switch {
	case m.MemPct >= StatusCritMem:
		out = append(out, Alert{m.Server, SeverityCritical, CategoryMemory,
			fmt.Sprintf("memory %d%%", m.MemPct)})
	case m.MemPct >= StatusWarnMem:
		out = append(out, Alert{m.Server, SeverityWarning, CategoryMemory,
			fmt.Sprintf("memory %d%%", m.MemPct)})
	}

	if m.CPUs > 0 && m.Load >= float64(m.CPUs)*AlertLoadRatio {
		out = append(out, Alert{m.Server, SeverityWarning, CategoryLoad,
			fmt.Sprintf("load %.1f (CPUs: %d)", m.Load, m.CPUs)})
	}

	gpuLow := strings.ToLower(m.GPUInfo)
	if strings.Contains(gpuLow, "error") || strings.Contains(gpuLow, "driver") {
		out = append(out, Alert{m.Server, SeverityWarning, CategoryGPU,
			fmt.Sprintf("GPU issue: %s", m.GPUInfo)})
	}

	return out
}

// DeriveAll runs Derive over many metrics and sorts results by
// severity (critical first) then server name.
func DeriveAll(metrics []Metric) []Alert {
	var all []Alert
	for _, m := range metrics {
		all = append(all, Derive(m)...)
	}
	Sort(all)
	return all
}

// Sort orders alerts by severity (critical first) then server name.
func Sort(list []Alert) {
	sort.SliceStable(list, func(i, j int) bool {
		si, sj := severityOrder(list[i].Severity), severityOrder(list[j].Severity)
		if si != sj {
			return si < sj
		}
		return list[i].Server < list[j].Server
	})
}

func severityOrder(s Severity) int {
	switch s {
	case SeverityCritical:
		return 0
	case SeverityWarning:
		return 1
	default:
		return 2
	}
}
