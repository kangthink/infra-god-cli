package webui

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/kangthink/infra-god-cli/internal/alerts"
	"github.com/kangthink/infra-god-cli/internal/collector"
	"github.com/kangthink/infra-god-cli/internal/inventory"
	sshclient "github.com/kangthink/infra-god-cli/internal/ssh"
)

// ServerSnapshot is a single server's view of status data.
type ServerSnapshot struct {
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	IP        string    `json:"ip"`
	Status    string    `json:"status"` // ok|warning|critical|error|auth_fail|stopped
	CPU       int       `json:"cpu_pct"`
	Mem       int       `json:"mem_pct"`
	Disk      int       `json:"disk_pct"`
	Load      float64   `json:"load"`
	CPUs      int       `json:"cpus"`
	GPULabel  string    `json:"gpu"`            // pre-formatted label e.g. "RTX 4090 ×2 0%"
	GPUOK     bool      `json:"gpu_ok"`         // false → driver err
	GPUCount  int       `json:"gpu_count"`
	Uptime    string    `json:"uptime"`
	UptimeSec int       `json:"uptime_sec"`
	OS        string    `json:"os"`
	Kernel    string    `json:"kernel"`
	Docker    string    `json:"docker"` // pre-formatted "12 (3 unhealthy)"
	Error     string    `json:"error,omitempty"`
	LastSync  time.Time `json:"last_sync"`
}

// StatusSummary mirrors the CLI summary line.
type StatusSummary struct {
	Total   int `json:"total"`
	OK      int `json:"ok"`
	Warning int `json:"warning"`
	Error   int `json:"error"`
	Stopped int `json:"stopped"`
}

// Snapshot is the full cache contents returned to API clients.
type Snapshot struct {
	Timestamp time.Time        `json:"timestamp"`
	Servers   []ServerSnapshot `json:"servers"`
	Alerts    []alerts.Alert   `json:"alerts"`
	Summary   StatusSummary    `json:"summary"`
}

// ContainerSnapshot is per-server docker ps cache.
type ContainerSnapshot struct {
	Server     string                `json:"server"`
	Containers []collector.Container `json:"containers"`
	LastSync   time.Time             `json:"last_sync"`
	Error      string                `json:"error,omitempty"`
}

// DetailsSnapshot is per-server listening-ports + mounts cache.
type DetailsSnapshot struct {
	Server   string            `json:"server"`
	Details  collector.Details `json:"details"`
	LastSync time.Time         `json:"last_sync"`
	Error    string            `json:"error,omitempty"`
}

// FoldersSnapshot is per-server top-level folders per mount cache.
type FoldersSnapshot struct {
	Server   string                   `json:"server"`
	Mounts   []collector.MountFolders `json:"mounts"`
	LastSync time.Time                `json:"last_sync"`
	Error    string                   `json:"error,omitempty"`
}

// Cache holds the latest status snapshot and runs background pollers
// to keep it fresh.
type Cache struct {
	mu         sync.RWMutex
	snapshot   Snapshot
	containers map[string]ContainerSnapshot // by server name
	details    map[string]DetailsSnapshot   // by server name
	folders    map[string]FoldersSnapshot   // by server name

	cfg           *inventory.Config
	servers       map[string]*inventory.ResolvedServer
	sshClient     *sshclient.Client
	parallel      int
	statusFreq    time.Duration
	containerFreq time.Duration
	detailsFreq   time.Duration
	foldersFreq   time.Duration
}

// New constructs a Cache. Call Start to begin polling.
func New(
	cfg *inventory.Config,
	servers map[string]*inventory.ResolvedServer,
	sshClient *sshclient.Client,
	parallel int,
	statusFreq time.Duration,
	containerFreq time.Duration,
	detailsFreq time.Duration,
	foldersFreq time.Duration,
) *Cache {
	return &Cache{
		cfg:           cfg,
		servers:       servers,
		sshClient:     sshClient,
		parallel:      parallel,
		statusFreq:    statusFreq,
		containerFreq: containerFreq,
		detailsFreq:   detailsFreq,
		foldersFreq:   foldersFreq,
		containers:    make(map[string]ContainerSnapshot),
		details:       make(map[string]DetailsSnapshot),
		folders:       make(map[string]FoldersSnapshot),
	}
}

// Start launches background pollers. Returns immediately.
// Cancel ctx to stop them.
func (c *Cache) Start(ctx context.Context) {
	// Run initial syncs synchronously so the snapshot isn't empty
	// when the HTTP server starts accepting requests.
	c.refreshStatus()
	c.refreshContainers()

	go func() {
		t := time.NewTicker(c.statusFreq)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.refreshStatus()
			}
		}
	}()

	go func() {
		t := time.NewTicker(c.containerFreq)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.refreshContainers()
			}
		}
	}()

	go func() {
		c.refreshDetails()
		t := time.NewTicker(c.detailsFreq)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.refreshDetails()
			}
		}
	}()

	go func() {
		// Folders polling can be slow (du can take 10-60s on big mounts).
		// Run as a single goroutine, sequentially after each tick.
		c.refreshFolders()
		t := time.NewTicker(c.foldersFreq)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.refreshFolders()
			}
		}
	}()
}

// Details returns the latest listening-ports + mounts snapshot for a server.
func (c *Cache) Details(name string) DetailsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.details[name]
}

// Folders returns the latest top-level folder listing per mount.
func (c *Cache) Folders(name string) FoldersSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.folders[name]
}

func (c *Cache) refreshDetails() {
	now := time.Now()
	var active []*inventory.ResolvedServer
	for _, srv := range c.servers {
		if srv.IsActive() && srv.PrimaryIP() != "" {
			active = append(active, srv)
		}
	}
	results := sshclient.RunParallel(c.sshClient, active, collector.DetailsCmd, c.parallel)

	updated := make(map[string]DetailsSnapshot, len(active))
	for i, srv := range active {
		snap := DetailsSnapshot{Server: srv.Name, LastSync: now}
		if results[i].Error != nil {
			snap.Error = results[i].Error.Error()
		} else {
			snap.Details = collector.ParseDetails(results[i].Output)
		}
		updated[srv.Name] = snap
	}
	c.mu.Lock()
	for k, v := range updated {
		c.details[k] = v
	}
	c.mu.Unlock()
	log.Printf("details refresh: %d servers", len(updated))
}

func (c *Cache) refreshFolders() {
	now := time.Now()
	var active []*inventory.ResolvedServer
	for _, srv := range c.servers {
		if srv.IsActive() && srv.PrimaryIP() != "" {
			active = append(active, srv)
		}
	}
	results := sshclient.RunParallel(c.sshClient, active, collector.FoldersCmd, c.parallel)

	updated := make(map[string]FoldersSnapshot, len(active))
	for i, srv := range active {
		snap := FoldersSnapshot{Server: srv.Name, LastSync: now}
		if results[i].Error != nil {
			snap.Error = results[i].Error.Error()
		} else {
			snap.Mounts = collector.ParseFolders(results[i].Output)
		}
		updated[srv.Name] = snap
	}
	c.mu.Lock()
	for k, v := range updated {
		c.folders[k] = v
	}
	c.mu.Unlock()
	log.Printf("folders refresh: %d servers", len(updated))
}

// Containers returns the latest container snapshot for a server.
// Returns zero value with empty server name if not yet polled.
func (c *Cache) Containers(name string) ContainerSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.containers[name]
}

// Server returns the cached status row for a single server, or zero value
// with empty Name if unknown.
func (c *Cache) Server(name string) ServerSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, s := range c.snapshot.Servers {
		if s.Name == name {
			return s
		}
	}
	return ServerSnapshot{}
}

// AlertsForServer returns alerts whose Server field matches name.
// Includes both status-derived and container-derived alerts.
func (c *Cache) AlertsForServer(name string) []alerts.Alert {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []alerts.Alert
	for _, a := range c.snapshot.Alerts {
		if a.Server == name {
			out = append(out, a)
		}
	}
	cs, ok := c.containers[name]
	if ok {
		out = append(out, deriveContainerAlerts(name, cs.Containers)...)
	}
	return out
}

// deriveContainerAlerts emits warnings for unhealthy / restarting containers.
func deriveContainerAlerts(server string, list []collector.Container) []alerts.Alert {
	var out []alerts.Alert
	for _, c := range list {
		switch c.State {
		case "restarting":
			out = append(out, alerts.Alert{
				Server: server, Severity: alerts.SeverityWarning, Category: alerts.CategoryContainer,
				Message: "container " + c.Name + " restarting",
			})
		}
		if c.Health == "unhealthy" {
			out = append(out, alerts.Alert{
				Server: server, Severity: alerts.SeverityWarning, Category: alerts.CategoryContainer,
				Message: "container " + c.Name + " unhealthy",
			})
		}
	}
	return out
}

// AllContainerAlerts merges container-derived alerts across all servers.
// Used by the dashboard so warnings show even before the user opens detail.
func (c *Cache) AllContainerAlerts() []alerts.Alert {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []alerts.Alert
	for srv, snap := range c.containers {
		out = append(out, deriveContainerAlerts(srv, snap.Containers)...)
	}
	return out
}

// refreshContainers polls docker ps across active servers.
func (c *Cache) refreshContainers() {
	now := time.Now()
	var active []*inventory.ResolvedServer
	for _, srv := range c.servers {
		if srv.IsActive() && srv.PrimaryIP() != "" {
			active = append(active, srv)
		}
	}

	results := sshclient.RunParallel(c.sshClient, active, collector.ContainersCmd, c.parallel)

	updated := make(map[string]ContainerSnapshot, len(active))
	for i, srv := range active {
		snap := ContainerSnapshot{
			Server:   srv.Name,
			LastSync: now,
		}
		if results[i].Error != nil {
			snap.Error = results[i].Error.Error()
		} else {
			snap.Containers = collector.ParseContainers(results[i].Output)
		}
		updated[srv.Name] = snap
	}

	c.mu.Lock()
	for k, v := range updated {
		c.containers[k] = v
	}
	c.mu.Unlock()

	log.Printf("container refresh: %d servers", len(updated))
}

// Snapshot returns a copy-safe view of the latest cached status with
// container-derived alerts merged in.
func (c *Cache) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap := c.snapshot
	var extra []alerts.Alert
	for srv, cs := range c.containers {
		extra = append(extra, deriveContainerAlerts(srv, cs.Containers)...)
	}
	if len(extra) > 0 {
		merged := make([]alerts.Alert, 0, len(snap.Alerts)+len(extra))
		merged = append(merged, snap.Alerts...)
		merged = append(merged, extra...)
		alerts.Sort(merged)
		snap.Alerts = merged
	}
	return snap
}

// refreshStatus runs the status command on all servers in parallel
// and rebuilds the cached Snapshot.
func (c *Cache) refreshStatus() {
	now := time.Now()

	var active []*inventory.ResolvedServer
	var inactive []ServerSnapshot
	for _, srv := range c.servers {
		if !srv.IsActive() {
			inactive = append(inactive, ServerSnapshot{
				Name:     srv.Name,
				Role:     srv.Role,
				Status:   alerts.StatusStopped,
				LastSync: now,
			})
			continue
		}
		if srv.PrimaryIP() == "" {
			inactive = append(inactive, ServerSnapshot{
				Name:     srv.Name,
				Role:     srv.Role,
				Status:   alerts.StatusError,
				Error:    "no IP configured",
				LastSync: now,
			})
			continue
		}
		active = append(active, srv)
	}

	results := sshclient.RunParallel(c.sshClient, active, collector.StatusCmd, c.parallel)

	servers := make([]ServerSnapshot, 0, len(active)+len(inactive))
	for i, srv := range active {
		s := ServerSnapshot{
			Name:     srv.Name,
			Role:     srv.Role,
			IP:       results[i].IP,
			Status:   alerts.StatusOK,
			LastSync: now,
		}
		if results[i].Error != nil {
			errMsg := results[i].Error.Error()
			if strings.Contains(errMsg, "auth") || strings.Contains(errMsg, "handshake") {
				s.Status = alerts.StatusAuthFail
			} else {
				s.Status = alerts.StatusError
			}
			s.Error = errMsg
			servers = append(servers, s)
			continue
		}
		applyParsed(&s, collector.ParseStatus(results[i].Output))
		s.Status = alerts.DeriveStatus(metricFor(s))
		servers = append(servers, s)
	}
	servers = append(servers, inactive...)

	// Stable sort by name for predictable UI output.
	sortByName(servers)

	// Derive alerts.
	mts := make([]alerts.Metric, 0, len(servers))
	for _, s := range servers {
		mts = append(mts, metricFor(s))
	}
	derived := alerts.DeriveAll(mts)

	summary := StatusSummary{Total: len(servers)}
	for _, s := range servers {
		switch s.Status {
		case alerts.StatusOK:
			summary.OK++
		case alerts.StatusWarning, alerts.StatusCritical:
			summary.Warning++
		case alerts.StatusError, alerts.StatusAuthFail:
			summary.Error++
		case alerts.StatusStopped:
			summary.Stopped++
		}
	}

	snap := Snapshot{
		Timestamp: now,
		Servers:   servers,
		Alerts:    derived,
		Summary:   summary,
	}

	c.mu.Lock()
	c.snapshot = snap
	c.mu.Unlock()

	if derived != nil {
		log.Printf("status refresh: %d servers (%d ok, %d warn, %d err, %d stopped) — %d alerts",
			summary.Total, summary.OK, summary.Warning, summary.Error, summary.Stopped, len(derived))
	}
}

// applyParsed copies parsed fields onto a snapshot, generating display labels.
func applyParsed(s *ServerSnapshot, p collector.Parsed) {
	s.CPU = p.CPUPct
	s.Mem = p.MemPct
	s.Disk = p.DiskPct
	s.Load = p.Load
	s.CPUs = p.CPUs
	s.UptimeSec = p.Uptime
	s.Uptime = collector.FormatUptime(p.Uptime)
	if p.OSName != "" || p.OSVersion != "" {
		s.OS = strings.TrimSpace(p.OSName + " " + p.OSVersion)
	}
	s.Kernel = p.Kernel

	// GPU label
	switch {
	case len(p.GPUs) == 0 && p.GPUDriverOK:
		s.GPULabel = "-"
		s.GPUOK = true
	case len(p.GPUs) == 0:
		s.GPULabel = "driver err"
		s.GPUOK = false
	default:
		first := p.GPUs[0]
		short := collector.ShortenGPU(first.Name)
		if len(p.GPUs) == 1 {
			s.GPULabel = fmt.Sprintf("%s %d%%", short, first.UtilPct)
		} else {
			s.GPULabel = fmt.Sprintf("%s ×%d %d%%", short, len(p.GPUs), first.UtilPct)
		}
		s.GPUCount = len(p.GPUs)
		s.GPUOK = true
	}

	if p.Docker.Available {
		label := fmt.Sprintf("%d", p.Docker.Total)
		if p.Docker.Unhealthy > 0 {
			label += fmt.Sprintf(" (%d unhealthy)", p.Docker.Unhealthy)
		}
		if p.Docker.Restarting > 0 {
			label += fmt.Sprintf(" (%d restarting)", p.Docker.Restarting)
		}
		s.Docker = label
	} else {
		s.Docker = "-"
	}
}

func metricFor(s ServerSnapshot) alerts.Metric {
	return alerts.Metric{
		Server:  s.Name,
		Status:  s.Status,
		DiskPct: s.Disk,
		MemPct:  s.Mem,
		CPUs:    s.CPUs,
		Load:    s.Load,
		GPUInfo: s.GPULabel,
	}
}

func sortByName(s []ServerSnapshot) {
	// Insertion sort — small N, avoids importing sort for one call.
	for i := 1; i < len(s); i++ {
		j := i
		for j > 0 && s[j-1].Name > s[j].Name {
			s[j-1], s[j] = s[j], s[j-1]
			j--
		}
	}
}
