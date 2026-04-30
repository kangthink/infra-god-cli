package webui

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// NewHandler builds the HTTP router for the read-only web UI.
func NewHandler(c *Cache) http.Handler {
	mux := http.NewServeMux()

	// Static assets (CSS, htmx)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS()))))

	// JSON endpoints
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, c.Snapshot())
	})

	mux.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		snap := c.Snapshot()
		writeJSON(w, map[string]any{
			"timestamp": snap.Timestamp,
			"alerts":    snap.Alerts,
		})
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		snap := c.Snapshot()
		age := time.Since(snap.Timestamp).Round(time.Second).String()
		if snap.Timestamp.IsZero() {
			age = "never"
		}
		writeJSON(w, map[string]any{
			"status":          "ok",
			"last_sync":       snap.Timestamp,
			"snapshot_age":    age,
			"servers_tracked": snap.Summary.Total,
		})
	})

	// Per-server JSON
	mux.HandleFunc("/api/servers/", func(w http.ResponseWriter, r *http.Request) {
		name, sub := splitServerPath(r.URL.Path)
		if name == "" {
			http.NotFound(w, r)
			return
		}
		s := c.Server(name)
		if s.Name == "" {
			http.Error(w, "unknown server", http.StatusNotFound)
			return
		}
		switch sub {
		case "containers":
			writeJSON(w, c.Containers(name))
		case "details":
			writeJSON(w, c.Details(name))
		case "folders":
			writeJSON(w, c.Folders(name))
		case "":
			writeJSON(w, map[string]any{
				"server":     s,
				"containers": c.Containers(name),
				"details":    c.Details(name),
				"folders":    c.Folders(name),
				"alerts":     c.AlertsForServer(name),
			})
		default:
			http.NotFound(w, r)
		}
	})

	// htmx HTML partials
	mux.HandleFunc("/partials/dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(renderDashboard(c.Snapshot())))
	})
	mux.HandleFunc("/partials/server/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/partials/server/")
		name = strings.TrimSuffix(name, "/")
		if name == "" {
			http.NotFound(w, r)
			return
		}
		s := c.Server(name)
		csnap := c.Containers(name)
		dsnap := c.Details(name)
		fsnap := c.Folders(name)
		alertsList := c.AlertsForServer(name)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(renderServerDetail(s, csnap, dsnap, fsnap, alertsList)))
	})

	// Server detail HTML shell
	mux.HandleFunc("/servers/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/servers/")
		name = strings.TrimSuffix(name, "/")
		if name == "" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(serverHTMLBytes(name))
	})

	// Root: dashboard HTML shell
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTMLBytes())
	})

	return mux
}

// splitServerPath parses paths like /api/servers/foo or /api/servers/foo/containers.
// Returns (name, subPath); subPath is "" for the bare server path.
func splitServerPath(p string) (string, string) {
	rest := strings.TrimPrefix(p, "/api/servers/")
	if rest == p {
		return "", ""
	}
	rest = strings.TrimSuffix(rest, "/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
