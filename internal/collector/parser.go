package collector

import (
	"strconv"
	"strings"
)

// Parsed is the structured result of running StatusCmd on a server.
// All fields are color-free; CLI and WebUI render their own presentation.
type Parsed struct {
	CPUPct  int
	MemPct  int
	DiskPct int
	Load    float64
	CPUs    int
	Uptime  int // seconds

	OSName    string
	OSVersion string
	Kernel    string

	GPUs        []ParsedGPU // empty if no NVIDIA GPU or driver error
	GPUDriverOK bool        // false when nvidia-smi failed but command exists

	Docker ParsedDocker
}

type ParsedGPU struct {
	Name   string
	MemMB  int // total
	UsedMB int
	UtilPct int
	TempC  int
}

type ParsedDocker struct {
	Available  bool
	Total      int
	Healthy    int
	Unhealthy  int
	Restarting int
}

// ParseStatus parses output from StatusCmd into a neutral struct.
// It tolerates missing/garbled fields by leaving zero values.
func ParseStatus(out string) Parsed {
	var p Parsed
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch key {
		case "CPU":
			p.CPUPct, _ = strconv.Atoi(val)
		case "MEM":
			parts := strings.Split(val, ":")
			if len(parts) >= 1 {
				p.MemPct, _ = strconv.Atoi(parts[0])
			}
		case "DISK":
			parts := strings.Split(val, ":")
			if len(parts) >= 1 {
				p.DiskPct, _ = strconv.Atoi(parts[0])
			}
		case "LOAD":
			p.Load, _ = strconv.ParseFloat(val, 64)
		case "CPUS":
			p.CPUs, _ = strconv.Atoi(val)
		case "UPTIME":
			p.Uptime, _ = strconv.Atoi(val)
		case "OS":
			parts := strings.Split(val, ":")
			if len(parts) >= 3 {
				p.OSName = parts[0]
				p.OSVersion = parts[1]
				p.Kernel = parts[2]
			}
		case "GPU":
			if val == "none" {
				p.GPUDriverOK = true // no driver expected
				continue
			}
			lower := strings.ToLower(val)
			if strings.Contains(lower, "unable") ||
				strings.Contains(lower, "nvml") ||
				strings.Contains(lower, "failed") ||
				strings.Contains(lower, "error") {
				// driver present but broken — leave GPUs empty, GPUDriverOK false
				continue
			}
			parts := strings.Split(val, ":")
			if len(parts) < 5 {
				continue
			}
			gpu := ParsedGPU{
				Name: strings.TrimSpace(parts[0]),
			}
			gpu.MemMB, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			gpu.UsedMB, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
			util := strings.TrimSpace(parts[3])
			if u, err := strconv.Atoi(util); err == nil {
				gpu.UtilPct = u
				p.GPUs = append(p.GPUs, gpu)
				p.GPUDriverOK = true
			}
			// (temperature is parts[4] — currently unused but easy to add)
		case "DOCKER":
			if val == "none" {
				continue
			}
			parts := strings.Split(val, ":")
			if len(parts) >= 4 {
				p.Docker.Available = true
				p.Docker.Total, _ = strconv.Atoi(parts[0])
				p.Docker.Healthy, _ = strconv.Atoi(parts[1])
				p.Docker.Unhealthy, _ = strconv.Atoi(parts[2])
				p.Docker.Restarting, _ = strconv.Atoi(parts[3])
			}
		}
	}
	return p
}

// Folder is one top-level folder under a mount with its du size.
type Folder struct {
	Size string `json:"size"` // e.g. "12G"
	Path string `json:"path"` // e.g. "/var/"
}

// MountFolders groups Folder entries by mount point.
type MountFolders struct {
	MountPt string   `json:"mount"`
	Folders []Folder `json:"folders"`
}

// ParseFolders parses output of FoldersCmd into MountFolders entries.
func ParseFolders(out string) []MountFolders {
	var result []MountFolders
	var current *MountFolders
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "MOUNT:") {
			if current != nil {
				result = append(result, *current)
			}
			current = &MountFolders{MountPt: strings.TrimPrefix(line, "MOUNT:")}
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "F|") {
			parts := strings.SplitN(line, "|", 3)
			if len(parts) == 3 {
				current.Folders = append(current.Folders, Folder{
					Size: parts[1],
					Path: parts[2],
				})
			}
		}
	}
	if current != nil {
		result = append(result, *current)
	}
	return result
}

// Mount is a parsed disk mount entry from `df -h`.
type Mount struct {
	Device  string `json:"device"`
	Size    string `json:"size"`
	Used    string `json:"used"`
	Avail   string `json:"avail"`
	UsedPct int    `json:"used_pct"`
	MountPt string `json:"mount"`
}

// Details is the parsed result of DetailsCmd.
type Details struct {
	ListeningPorts []string `json:"ports"` // raw "addr:port" lines
	Mounts         []Mount  `json:"mounts"`
}

// ParseDetails parses output of DetailsCmd.
func ParseDetails(out string) Details {
	var d Details
	section := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch line {
		case "===PORTS===":
			section = "ports"
			continue
		case "===MOUNTS===":
			section = "mounts"
			continue
		}
		switch section {
		case "ports":
			d.ListeningPorts = append(d.ListeningPorts, line)
		case "mounts":
			fields := strings.Fields(line)
			if len(fields) < 6 {
				continue
			}
			m := Mount{
				Device:  fields[0],
				Size:    fields[1],
				Used:    fields[2],
				Avail:   fields[3],
				MountPt: fields[5],
			}
			pct := strings.TrimSuffix(fields[4], "%")
			m.UsedPct, _ = strconv.Atoi(pct)
			d.Mounts = append(d.Mounts, m)
		}
	}
	return d
}

// Container is the parsed view of one docker container line.
type Container struct {
	Name      string    `json:"name"`
	State     string    `json:"state"`  // running/exited/restarting/paused/created/dead
	Health    string    `json:"health"` // healthy/unhealthy/starting/none/-
	Image     string    `json:"image"`
	Restart   string    `json:"restart"`
	StartedAt string    `json:"started_at"`
	Ports     []string  `json:"ports"`
}

// ParseContainers parses ContainersCmd output. Lines beginning with "none"
// or empty lines are skipped.
func ParseContainers(out string) []Container {
	var out2 []Container
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "none" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 7 {
			continue
		}
		c := Container{
			Name:      parts[0],
			State:     parts[1],
			Health:    parts[2],
			Image:     parts[3],
			Restart:   parts[4],
			StartedAt: parts[5],
		}
		ports := strings.Fields(parts[6])
		if len(ports) > 0 {
			c.Ports = ports
		}
		if c.Health == "" {
			c.Health = "-"
		}
		out2 = append(out2, c)
	}
	return out2
}

// ShortenGPU collapses verbose vendor strings.
func ShortenGPU(name string) string {
	r := strings.NewReplacer(
		"NVIDIA GeForce ", "",
		"NVIDIA ", "",
		"Tesla ", "",
	)
	return r.Replace(name)
}

// FormatUptime turns seconds into a compact label (12m / 3h / 5d / 12w).
func FormatUptime(seconds int) string {
	if seconds < 3600 {
		return strconv.Itoa(seconds/60) + "m"
	}
	if seconds < 86400 {
		return strconv.Itoa(seconds/3600) + "h"
	}
	days := seconds / 86400
	if days < 7 {
		return strconv.Itoa(days) + "d"
	}
	return strconv.Itoa(days/7) + "w"
}
