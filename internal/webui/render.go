package webui

import (
	"fmt"
	"html"
	"strings"

	"github.com/kangthink/infra-god-cli/internal/alerts"
	"github.com/kangthink/infra-god-cli/internal/collector"
)

// renderDashboard returns the HTML fragment that htmx swaps into #dashboard.
// It contains the alerts banner + the server table.
func renderDashboard(snap Snapshot) string {
	var sb strings.Builder

	// Summary line + last-sync
	sb.WriteString(`<div class="meta" hx-swap-oob="true" id="meta">`)
	fmt.Fprintf(&sb,
		`<span class="dot %s">●</span> %d ok · `+
			`<span class="dot warn">●</span> %d warn · `+
			`<span class="dot crit">●</span> %d err · `+
			`%d stopped &nbsp;|&nbsp; sync %s`,
		summaryDotClass(snap.Summary),
		snap.Summary.OK,
		snap.Summary.Warning,
		snap.Summary.Error,
		snap.Summary.Stopped,
		snap.Timestamp.Format("15:04:05"),
	)
	sb.WriteString(`</div>`)

	// Alerts panel
	sb.WriteString(renderAlerts(snap.Alerts))

	// Servers table
	sb.WriteString(renderServerTable(snap.Servers))

	return sb.String()
}

func summaryDotClass(s StatusSummary) string {
	switch {
	case s.Error > 0:
		return "crit"
	case s.Warning > 0:
		return "warn"
	default:
		return ""
	}
}

func renderAlerts(list []alerts.Alert) string {
	var sb strings.Builder
	if len(list) == 0 {
		sb.WriteString(`<div class="alerts">`)
		sb.WriteString(`<div class="alerts-header empty"><span class="title">알림 없음</span><span class="meta">all clear ✓</span></div>`)
		sb.WriteString(`</div>`)
		return sb.String()
	}
	sb.WriteString(`<div class="alerts">`)
	fmt.Fprintf(&sb, `<div class="alerts-header"><span class="title">⚠ ALERTS (%d)</span></div>`, len(list))
	sb.WriteString(`<div class="alerts-list">`)
	for _, a := range list {
		fmt.Fprintf(&sb,
			`<div class="alert-row"><span class="badge %s">%s</span><span class="server">%s</span><span class="cat">%s</span><span class="msg">%s</span></div>`,
			html.EscapeString(string(a.Severity)),
			html.EscapeString(string(a.Severity)),
			html.EscapeString(a.Server),
			html.EscapeString(string(a.Category)),
			html.EscapeString(a.Message),
		)
	}
	sb.WriteString(`</div></div>`)
	return sb.String()
}

func renderServerTable(servers []ServerSnapshot) string {
	var sb strings.Builder
	sb.WriteString(`<div class="panel" style="margin-top:16px">`)
	sb.WriteString(`<div class="panel-header">SERVERS</div>`)
	sb.WriteString(`<table class="servers">`)
	sb.WriteString(`<thead><tr>`)
	for _, h := range []string{"", "Server", "Role", "OS", "Kernel", "CPU%", "MEM%", "DISK%", "GPU", "LOAD", "Docker", "Uptime"} {
		fmt.Fprintf(&sb, `<th>%s</th>`, html.EscapeString(h))
	}
	sb.WriteString(`</tr></thead><tbody>`)

	for _, s := range servers {
		rowClass := "row-" + s.Status
		fmt.Fprintf(&sb, `<tr class="%s">`, html.EscapeString(rowClass))
		fmt.Fprintf(&sb, `<td class="icon">%s</td>`, statusIcon(s.Status))

		switch s.Status {
		case alerts.StatusStopped:
			fmt.Fprintf(&sb, `<td class="name muted">%s</td>`, html.EscapeString(s.Name))
			fmt.Fprintf(&sb, `<td class="muted">%s</td>`, html.EscapeString(s.Role))
			sb.WriteString(`<td class="muted" colspan="9">stopped</td>`)
		case alerts.StatusError, alerts.StatusAuthFail:
			fmt.Fprintf(&sb, `<td class="name">%s</td>`, html.EscapeString(s.Name))
			fmt.Fprintf(&sb, `<td>%s</td>`, html.EscapeString(s.Role))
			fmt.Fprintf(&sb, `<td class="crit" colspan="9">%s — %s</td>`, html.EscapeString(s.Status), html.EscapeString(truncate(s.Error, 80)))
		default:
			fmt.Fprintf(&sb, `<td class="name"><a href="/servers/%s">%s</a></td>`, html.EscapeString(s.Name), html.EscapeString(s.Name))
			fmt.Fprintf(&sb, `<td>%s</td>`, html.EscapeString(s.Role))
			fmt.Fprintf(&sb, `<td>%s</td>`, html.EscapeString(s.OS))
			fmt.Fprintf(&sb, `<td>%s</td>`, html.EscapeString(s.Kernel))
			fmt.Fprintf(&sb, `<td class="num %s">%d%%</td>`, pctClass(s.CPU, 70, 90), s.CPU)
			fmt.Fprintf(&sb, `<td class="num %s">%d%%</td>`, pctClass(s.Mem, 80, 90), s.Mem)
			fmt.Fprintf(&sb, `<td class="num %s">%d%%</td>`, pctClass(s.Disk, 85, 95), s.Disk)
			fmt.Fprintf(&sb, `<td>%s</td>`, html.EscapeString(s.GPULabel))
			fmt.Fprintf(&sb, `<td class="num %s">%.1f</td>`, loadClass(s.Load, s.CPUs), s.Load)
			fmt.Fprintf(&sb, `<td class="muted">%s</td>`, html.EscapeString(s.Docker))
			fmt.Fprintf(&sb, `<td class="muted">%s</td>`, html.EscapeString(s.Uptime))
		}
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)
	return sb.String()
}

func statusIcon(status string) string {
	switch status {
	case alerts.StatusOK:
		return "✓"
	case alerts.StatusWarning:
		return "⚠"
	case alerts.StatusCritical:
		return "✕"
	case alerts.StatusError, alerts.StatusAuthFail:
		return "✕"
	case alerts.StatusStopped:
		return "·"
	}
	return "?"
}

func pctClass(v, warn, crit int) string {
	if v >= crit {
		return "crit"
	}
	if v >= warn {
		return "warn"
	}
	return ""
}

func loadClass(load float64, cpus int) string {
	if cpus == 0 {
		return ""
	}
	if load >= float64(cpus)*alerts.AlertLoadRatio {
		return "warn"
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// readStaticFile reads a file from the embedded static dir.
func readStaticFile(name string) []byte {
	f, err := staticFS().Open(name)
	if err != nil {
		return nil
	}
	defer f.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := f.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf
}

// indexHTMLBytes returns the dashboard shell HTML.
func indexHTMLBytes() []byte {
	b := readStaticFile("index.html")
	if b == nil {
		return []byte("infra-god (template missing)")
	}
	return b
}

// serverHTMLBytes returns the per-server detail shell with the server name substituted.
func serverHTMLBytes(name string) []byte {
	b := readStaticFile("server.html")
	if b == nil {
		return []byte("infra-god (template missing)")
	}
	safe := html.EscapeString(name)
	return []byte(strings.ReplaceAll(string(b), "{{NAME}}", safe))
}

// renderServerDetail returns HTML fragment for the per-server page.
func renderServerDetail(s ServerSnapshot, csnap ContainerSnapshot, dsnap DetailsSnapshot, fsnap FoldersSnapshot, list []alerts.Alert) string {
	var sb strings.Builder

	// Update meta line OOB
	sb.WriteString(`<div class="meta" hx-swap-oob="true" id="meta">`)
	if !csnap.LastSync.IsZero() {
		fmt.Fprintf(&sb, `containers sync %s · `, csnap.LastSync.Format("15:04:05"))
	}
	if !s.LastSync.IsZero() {
		fmt.Fprintf(&sb, `status sync %s`, s.LastSync.Format("15:04:05"))
	}
	sb.WriteString(`</div>`)

	if s.Name == "" {
		sb.WriteString(`<div class="alerts"><div class="alerts-header"><span class="title">unknown server</span></div></div>`)
		return sb.String()
	}

	// Header info
	sb.WriteString(`<div class="panel">`)
	fmt.Fprintf(&sb, `<div class="panel-header">%s · %s · %s · uptime %s</div>`,
		html.EscapeString(s.Name),
		html.EscapeString(s.Role),
		html.EscapeString(s.OS),
		html.EscapeString(s.Uptime),
	)
	sb.WriteString(`<table class="servers"><tr>`)
	fmt.Fprintf(&sb, `<th>Status</th><td class="row-%s icon">%s</td>`,
		html.EscapeString(s.Status), statusIcon(s.Status))
	fmt.Fprintf(&sb, `<th>IP</th><td>%s</td>`, html.EscapeString(s.IP))
	fmt.Fprintf(&sb, `<th>Kernel</th><td>%s</td>`, html.EscapeString(s.Kernel))
	sb.WriteString(`</tr><tr>`)
	fmt.Fprintf(&sb, `<th>CPU</th><td class="num %s">%d%%</td>`, pctClass(s.CPU, 70, 90), s.CPU)
	fmt.Fprintf(&sb, `<th>MEM</th><td class="num %s">%d%%</td>`, pctClass(s.Mem, 80, 90), s.Mem)
	fmt.Fprintf(&sb, `<th>DISK</th><td class="num %s">%d%%</td>`, pctClass(s.Disk, 85, 95), s.Disk)
	sb.WriteString(`</tr><tr>`)
	fmt.Fprintf(&sb, `<th>LOAD</th><td class="num %s">%.2f / %d cpus</td>`,
		loadClass(s.Load, s.CPUs), s.Load, s.CPUs)
	fmt.Fprintf(&sb, `<th>GPU</th><td>%s</td>`, html.EscapeString(s.GPULabel))
	fmt.Fprintf(&sb, `<th>Docker</th><td>%s</td>`, html.EscapeString(s.Docker))
	sb.WriteString(`</tr></table></div>`)

	// Alerts for this server
	if len(list) > 0 {
		sb.WriteString(`<div style="margin-top:16px">`)
		sb.WriteString(renderAlerts(list))
		sb.WriteString(`</div>`)
	}

	// Containers
	sb.WriteString(`<div class="panel" style="margin-top:16px">`)
	if csnap.Error != "" {
		fmt.Fprintf(&sb, `<div class="panel-header">CONTAINERS — error</div>`)
		fmt.Fprintf(&sb, `<div style="padding:12px 16px;color:var(--crit)">%s</div>`, html.EscapeString(csnap.Error))
	} else if len(csnap.Containers) == 0 {
		sb.WriteString(`<div class="panel-header">CONTAINERS</div>`)
		sb.WriteString(`<div style="padding:12px 16px;color:var(--muted)">no containers</div>`)
	} else {
		fmt.Fprintf(&sb, `<div class="panel-header">CONTAINERS (%d)</div>`, len(csnap.Containers))
		sb.WriteString(`<table class="servers"><thead><tr>`)
		for _, h := range []string{"Name", "State", "Health", "Image", "Restart", "Ports"} {
			fmt.Fprintf(&sb, `<th>%s</th>`, h)
		}
		sb.WriteString(`</tr></thead><tbody>`)

		// Sort: problematic first (restarting/unhealthy), then running, then others
		ordered := sortContainers(csnap.Containers)
		for _, c := range ordered {
			rowCls := containerRowClass(c)
			fmt.Fprintf(&sb, `<tr class="%s">`, rowCls)
			fmt.Fprintf(&sb, `<td class="name">%s</td>`, html.EscapeString(c.Name))
			fmt.Fprintf(&sb, `<td class="%s">%s</td>`, stateClass(c.State), html.EscapeString(c.State))
			fmt.Fprintf(&sb, `<td class="%s">%s</td>`, healthClass(c.Health), html.EscapeString(c.Health))
			fmt.Fprintf(&sb, `<td class="muted">%s</td>`, html.EscapeString(truncate(c.Image, 40)))
			fmt.Fprintf(&sb, `<td class="muted">%s</td>`, html.EscapeString(c.Restart))
			fmt.Fprintf(&sb, `<td class="muted">%s</td>`, html.EscapeString(formatPorts(c.Ports)))
			sb.WriteString(`</tr>`)
		}
		sb.WriteString(`</tbody></table>`)
	}
	sb.WriteString(`</div>`)

	// Listening ports + Mounts
	sb.WriteString(renderDetails(dsnap))

	// Folder breakdown (slower poll, may be empty if not yet scanned)
	sb.WriteString(renderFolders(fsnap))

	return sb.String()
}

func renderFolders(f FoldersSnapshot) string {
	if f.Server == "" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(`<div class="panel" style="margin-top:16px">`)
	sb.WriteString(`<div class="panel-header">FOLDERS (top-level, by size)</div>`)
	if f.Error != "" {
		fmt.Fprintf(&sb, `<div style="padding:12px 16px;color:var(--crit)">%s</div>`, html.EscapeString(f.Error))
	} else if len(f.Mounts) == 0 {
		sb.WriteString(`<div style="padding:12px 16px;color:var(--muted)">scanning… (first scan after start can take 30-60s)</div>`)
	} else {
		for _, m := range f.Mounts {
			fmt.Fprintf(&sb, `<div style="padding:10px 16px;border-bottom:1px solid var(--border)">`)
			fmt.Fprintf(&sb, `<div class="name" style="margin-bottom:6px">%s</div>`, html.EscapeString(m.MountPt))
			if len(m.Folders) == 0 {
				sb.WriteString(`<span class="muted">(empty or scan timed out)</span>`)
			} else {
				sb.WriteString(`<div style="font-family:ui-monospace,SF Mono,Menlo,monospace;font-size:12px">`)
				for _, fld := range m.Folders {
					fmt.Fprintf(&sb, `<div style="display:flex;gap:12px;padding:2px 0"><span style="display:inline-block;min-width:60px;text-align:right;color:var(--accent)">%s</span><span class="muted">%s</span></div>`,
						html.EscapeString(fld.Size),
						html.EscapeString(fld.Path),
					)
				}
				sb.WriteString(`</div>`)
			}
			sb.WriteString(`</div>`)
		}
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

func renderDetails(d DetailsSnapshot) string {
	if d.Server == "" {
		return ""
	}
	var sb strings.Builder

	// Mounts
	sb.WriteString(`<div class="panel" style="margin-top:16px">`)
	sb.WriteString(`<div class="panel-header">DISK MOUNTS</div>`)
	if len(d.Details.Mounts) == 0 {
		sb.WriteString(`<div style="padding:12px 16px;color:var(--muted)">no data</div>`)
	} else {
		sb.WriteString(`<table class="servers"><thead><tr>`)
		for _, h := range []string{"Mount", "Device", "Size", "Used", "Avail", "Used%"} {
			fmt.Fprintf(&sb, `<th>%s</th>`, h)
		}
		sb.WriteString(`</tr></thead><tbody>`)
		for _, m := range d.Details.Mounts {
			fmt.Fprintf(&sb, `<tr><td class="name">%s</td><td class="muted">%s</td><td class="num">%s</td><td class="num">%s</td><td class="num">%s</td><td class="num %s">%d%%</td></tr>`,
				html.EscapeString(m.MountPt),
				html.EscapeString(m.Device),
				html.EscapeString(m.Size),
				html.EscapeString(m.Used),
				html.EscapeString(m.Avail),
				pctClass(m.UsedPct, 85, 95),
				m.UsedPct,
			)
		}
		sb.WriteString(`</tbody></table>`)
	}
	sb.WriteString(`</div>`)

	// Listening ports
	sb.WriteString(`<div class="panel" style="margin-top:16px">`)
	fmt.Fprintf(&sb, `<div class="panel-header">LISTENING PORTS (%d)</div>`, len(d.Details.ListeningPorts))
	if len(d.Details.ListeningPorts) == 0 {
		sb.WriteString(`<div style="padding:12px 16px;color:var(--muted)">no public listeners</div>`)
	} else {
		sb.WriteString(`<div style="padding:12px 16px;font-family:ui-monospace,SF Mono,Menlo,monospace;font-size:12px;line-height:1.8">`)
		for _, p := range d.Details.ListeningPorts {
			fmt.Fprintf(&sb, `<span style="display:inline-block;background:var(--panel-2);border:1px solid var(--border);border-radius:4px;padding:1px 8px;margin:2px 4px 2px 0">%s</span>`, html.EscapeString(p))
		}
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</div>`)

	return sb.String()
}

func stateClass(state string) string {
	switch state {
	case "running":
		return ""
	case "restarting":
		return "warn"
	case "exited", "dead":
		return "muted"
	}
	return "muted"
}

func healthClass(h string) string {
	switch h {
	case "healthy":
		return ""
	case "unhealthy":
		return "warn"
	}
	return "muted"
}

func containerRowClass(c collector.Container) string {
	if c.State == "restarting" || c.Health == "unhealthy" {
		return "row-warning"
	}
	if c.State == "running" {
		return "row-ok"
	}
	return "row-stopped"
}

func sortContainers(list []collector.Container) []collector.Container {
	// Sort by problem severity then name. (Stable, simple insertion sort.)
	out := make([]collector.Container, len(list))
	copy(out, list)
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && containerOrder(out[j-1]) > containerOrder(out[j]) {
			out[j-1], out[j] = out[j], out[j-1]
			j--
		}
	}
	return out
}

func containerOrder(c collector.Container) int {
	if c.State == "restarting" {
		return 0
	}
	if c.Health == "unhealthy" {
		return 1
	}
	if c.State == "running" {
		return 2
	}
	return 3
}

func formatPorts(ports []string) string {
	if len(ports) == 0 {
		return ""
	}
	if len(ports) <= 3 {
		return strings.Join(ports, " ")
	}
	return strings.Join(ports[:3], " ") + " …+" + fmt.Sprint(len(ports)-3)
}

