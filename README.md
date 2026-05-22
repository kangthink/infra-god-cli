# infra-god

Agentless CLI tool for monitoring and managing multiple Linux servers over SSH.

No agents to install. No web dashboards. Just one binary and a YAML config.

## Features

- **Status Dashboard** — CPU, memory, disk, GPU, load, uptime across all servers at a glance
- **Deep Inspection** — Hardware, software, Docker, network, storage, GPU details per server
- **Parallel Execution** — Run commands across servers concurrently with safety checks
- **Auto-Healing** — Disk cleanup, service restart, security hardening
- **Security Audit** — SSH config, firewall, fail2ban, open ports analysis
- **File Transfer** — SCP files to/from multiple servers
- **Process Monitoring** — Top processes by CPU/memory, per-server or fleet-wide
- **IP Fallback** — Wired IP first, automatic fallback to wireless
- **Web UI** (`serve`) — read-only LAN dashboard so teammates can see status without SSH access

## Installation

### Download binary (recommended)

Download the latest release from [GitHub Releases](https://github.com/kangthink/infra-god-cli/releases):

```bash
# Linux (amd64)
curl -Lo infra-god https://github.com/kangthink/infra-god-cli/releases/latest/download/infra-god-linux-amd64
chmod +x infra-god
sudo mv infra-god /usr/local/bin/

# Linux (arm64)
curl -Lo infra-god https://github.com/kangthink/infra-god-cli/releases/latest/download/infra-god-linux-arm64
chmod +x infra-god
sudo mv infra-god /usr/local/bin/

# macOS (Apple Silicon)
curl -Lo infra-god https://github.com/kangthink/infra-god-cli/releases/latest/download/infra-god-darwin-arm64
chmod +x infra-god
sudo mv infra-god /usr/local/bin/

# macOS (Intel)
curl -Lo infra-god https://github.com/kangthink/infra-god-cli/releases/latest/download/infra-god-darwin-amd64
chmod +x infra-god
sudo mv infra-god /usr/local/bin/
```

### Install with Go

```bash
go install github.com/kangthink/infra-god-cli@latest
# Binary will be installed as 'infra-god-cli' in $GOPATH/bin
# Optionally rename: mv $(go env GOPATH)/bin/infra-god-cli $(go env GOPATH)/bin/infra-god
```

### Build from source

```bash
git clone https://github.com/kangthink/infra-god-cli.git
cd infra-god-cli
go build -o infra-god .
```

## Quick Start

```bash
# Set up config
cp servers.yaml.example servers.yaml
# Edit servers.yaml with your server details

# Set SSH password — either export, or put in .env (see Environment Variables below)
export INFRA_SSH_PASS="your-password"

# Check all servers
infra-god status

# Inspect a specific server
infra-god inspect web-1
```

## Commands

| Command | Description |
|---------|-------------|
| `status [server...]` | Server status dashboard |
| `inspect <server>` | Deep server inspection |
| `exec <command> [server...]` | Run command across servers |
| `ps <server>` | Top processes by CPU/memory |
| `logs <server> [service]` | View service logs |
| `security <server>` | Security audit |
| `heal <server> <action>` | Auto-healing (disk cleanup, restart, harden) |
| `users <server>` | User and permission info |
| `cp <src> <dst> [server...]` | File transfer via SCP |
| `config list\|add\|edit\|remove\|test` | Manage server inventory |
| `serve` | Read-only LAN web UI (status + containers + folders) |

## Status Dashboard

```
═══ INFRA-GOD STATUS ═══
SERVER       ROLE     OS           CPU%  MEM%  DISK%  GPU          LOAD  UPTIME
✅ web-1     main     ubuntu 22.04  23%   45%   67%  -            0.5   14d
✅ gpu-1     machine  ubuntu 22.04  45%   62%   71%  A100 32%     2.1   7d
⚠️ worker-1  worker   debian 12     82%   78%   91%  RTX4090 87%  6.3   3d
⏹  gpu-2     machine  -             -     -     -    -            -     stopped

TOTAL: 4 servers | 2 ok | 1 warning | 0 error | 1 stopped
```

## Inspect

```bash
# Full inspection
./infra-god inspect gpu-1

# Specific sections
./infra-god inspect gpu-1 --hw       # Hardware only
./infra-god inspect gpu-1 --gpu      # GPU details + processes
./infra-god inspect gpu-1 --docker   # Docker containers
./infra-god inspect gpu-1 --storage  # Disk usage breakdown
./infra-god inspect gpu-1 --network  # Interfaces and ports
./infra-god inspect gpu-1 --services # Systemd services
```

## Exec

```bash
# Run on all servers
./infra-god exec "uptime"

# Target specific servers or groups
./infra-god exec "df -h" web-1 gpu-1
./infra-god exec "apt update" --group workers --sudo

# Safety features
./infra-god exec "docker stop myapp" --yes     # Skip confirmation
./infra-god exec "apt upgrade -y" --dry-run     # Preview targets
./infra-god exec "systemctl restart nginx" --serial  # One at a time
```

## Heal

```bash
# Disk cleanup (safe)
./infra-god heal web-1 disk --dry-run
./infra-god heal web-1 disk

# Aggressive cleanup (includes Docker prune)
./infra-god heal web-1 disk --aggressive

# Security hardening (fail2ban + ufw + SSH hardening)
./infra-god heal web-1 security
```

## Configuration

See `servers.yaml.example` for the full format.

```yaml
defaults:
  ssh_user: deploy
  ssh_port: 22
  auth:
    type: password
    password_env: INFRA_SSH_PASS

servers:
  web-1:
    host: 10.0.1.1
    role: main

  gpu-1:
    host:
      wired: 10.0.1.10
      wireless: 10.0.2.10
    role: machine

  gpu-2:
    host: null
    role: machine
    status: stopped
```

### Auth Methods

**Password** (default):
```bash
export INFRA_SSH_PASS="your-password"
```

**SSH Key** (per-server override):
```yaml
servers:
  web-1:
    auth:
      type: key
      key_path: ~/.ssh/web1_id_rsa
```

### Environment Variables (`.env`)

Any environment variable referenced by `servers.yaml` (typically `password_env`) can be loaded from a `.env` file instead of `export`-ing in your shell.

**Load order** (later sources do not override already-set vars, so shell wins, then project, then global):

1. Shell environment (`export FOO=...`)
2. `./.env` — project-local, in the directory you run `infra-god` from
3. `~/.infra-god/.env` — user-global fallback

**Example user-global setup:**
```bash
mkdir -p ~/.infra-god
cat > ~/.infra-god/.env <<'EOF'
INFRA_SSH_PASS=your-password
WEB1_SSH_PASS=different-password-for-web1
EOF
chmod 600 ~/.infra-god/.env
```

Now `infra-god status` works from any directory without manually exporting passwords each shell session. Combined with `~/.infra-god/servers.yaml` (auto-discovered when no `./servers.yaml` exists), this gives you a fully global setup.

### Config File Discovery

When `--config` is not specified, infra-god searches in this order:

1. `./servers.yaml` — project-local
2. `~/.infra-god/servers.yaml` — user-global

## Web UI (`serve`)

Read-only dashboard for the rest of your team — they can check fleet status without SSH access or admin privileges.

### Quick start

```bash
# Foreground (Ctrl+C to stop)
infra-god serve

# Listening on http://0.0.0.0:9998
#   본인 (this Mac):     http://localhost:9998/
#   사내 다른 사람들:    http://<your-LAN-IP>:9998/
```

### What viewers can see

- Dashboard (`/`) — live status table, alert banner, color-coded warnings
- Per-server detail (`/servers/<name>`) — Docker containers (with health/restart state), listening ports, disk mounts, top-level folder sizes
- Read-only by design — `exec`, `heal`, `cp` and any write endpoints are not exposed

### Prerequisites

- The Mac/Linux running `serve` is the **operator's machine** — it must stay on for others to view
- All conditions for the CLI apply: `servers.yaml` inventory, SSH reachability, `INFRA_SSH_PASS` if password auth
- Other people just need a browser and to be on the same LAN
- **Security model: LAN trust.** No auth, no HTTPS. Don't expose port 9998 to the internet.

### Flags

| Flag | Default | Notes |
|------|---------|-------|
| `--addr` | `0.0.0.0:9998` | Bind address. Use `127.0.0.1:9998` to keep it on the local machine only. |
| `--refresh` | `30s` | Status (CPU/MEM/DISK/GPU) poll interval. |
| `--container-refresh` | `60s` | `docker ps` poll interval. |
| `--details-refresh` | `2m` | Listening ports + `df` poll interval. |
| `--folders-refresh` | `5m` | `du`-based top-level folder size scan. Heavier; runs separately. |

Background pollers run on a fixed cadence regardless of viewer count, so SSH load on your servers is constant whether 1 or 50 people are looking.

### Auto-start on macOS (Login Item .app)

Install once — a tiny `InfraGod.app` bundle is registered as a Login Item, so the daemon starts silently every time you log in. macOS's Local Network privacy works correctly because we run as a proper app (with `NSLocalNetworkUsageDescription`) rather than a raw launchd daemon.

> **Why an .app bundle, not LaunchAgent?**
> macOS 14+ blocks LaunchAgent-spawned processes from connecting to LAN IPs (Local Network privacy gate). LaunchAgents have no way to surface the permission prompt. Wrapping the binary in an `.app` lets macOS prompt you once on first run, then remember the grant forever.

```bash
# 1) Build the binary into your PATH
go build -o /usr/local/bin/infra-god .

# 2) Set SSH password in your shell (if your servers use password auth)
export INFRA_SSH_PASS='your-password'

# 3) Install the .app + Login Item
./scripts/autostart/install.sh
```

Output example:
```
✅ InfraGod.app is running.
   본인:        http://localhost:9998/
   사내 공유:   http://192.168.1.9:9998/
   app:         ~/Applications/InfraGod.app
   logs:        ~/Library/Logs/infra-god/infra-god.{out,err}.log

🔔 처음 실행 시 macOS가 'Local Network 접근 허용' 알림을 띄울 수 있습니다.
   '허용'을 누르면 LAN 서버에 접근 가능해집니다.
```

The installer:
- Builds `~/Applications/InfraGod.app` with `LSUIElement` (no Dock icon, runs silently)
- Registers it as a Login Item via AppleScript
- Codesigns ad-hoc so TCC tracks the bundle as a stable identity
- Opens it immediately so you can grant the Local Network prompt now

### Is it running?

```bash
./scripts/autostart/check.sh
```

One command shows: app bundle path, Login Item registration, process pid, `/healthz` response, **LAN reachability counts**, recent log tails. Errors trigger a hint about Local Network permission. Use any time you wonder "is it still up?".

Quick alternatives:
```bash
curl -s http://localhost:9998/healthz                  # is the HTTP server alive?
pgrep -af 'infra-god serve'                            # is the process there?
tail -f ~/Library/Logs/infra-god/infra-god.out.log     # live log
```

### Update / Reinstall

After rebuilding the binary or changing your `INFRA_SSH_PASS`:
```bash
./scripts/autostart/install.sh   # safe to re-run; replaces the bundle and restarts
```

### Stop / Remove

```bash
./scripts/autostart/uninstall.sh   # stops the app, removes Login Item and bundle
```

Logs at `~/Library/Logs/infra-god/` are kept (delete manually if not wanted). The Local Network permission entry remains in System Settings — toggle off there if desired.

### Troubleshooting

| Symptom | Cause / fix |
|---------|------------|
| All LAN servers show `no route to host` | Local Network permission not granted. `open 'x-apple.systempreferences:com.apple.preference.security?Privacy_LocalNetwork'` and toggle **InfraGod** on. |
| `connection refused` for the first 1-2 minutes | First poll is still finishing. Wait, then refresh. |
| FOLDERS section shows "scanning…" for ~2 minutes after start | First `du` scan is in progress. Subsequent scans use cached data. |
| Some server shows `auth_fail` | SSH password wrong / changed. `export INFRA_SSH_PASS=...` in your shell, then re-run `install.sh`. |
| Port already in use | Edit `scripts/autostart/install.sh` and change `0.0.0.0:9998` to a free port (also update wrapper). |
| LAN colleagues can't connect | Check macOS firewall (System Settings → Network → Firewall) — incoming connections must be allowed for `infra-god`. |
| Binary moved | Re-run `./scripts/autostart/install.sh` to refresh the wrapper script's path. |

## Global Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Config file path (default: `./servers.yaml`) |
| `--group <name>` | Target server group |
| `--all` | Target all active servers |
| `--json` | JSON output |
| `--timeout <duration>` | SSH timeout (default: 10s) |
| `--parallel <n>` | Max concurrent SSH connections (default: 10) |
| `--sudo` | Execute with sudo |
| `--verbose` | Verbose output |

## Using with Claude Code

infra-god includes built-in [Claude Code](https://docs.anthropic.com/en/docs/claude-code) slash commands for AI-powered server management. Clone the repo and use the commands directly from the project directory.

### Available Commands

| Command | Description |
|---------|-------------|
| `/infra:status` | Fleet-wide status dashboard with warnings |
| `/infra:inspect` | Deep inspection of a specific server |
| `/infra:exec` | Run commands across multiple servers |
| `/infra:diagnose` | AI-powered root cause analysis |
| `/infra:heal` | Automated recovery (disk cleanup, restart, harden) |
| `/infra:cert` | SSL certificate expiry check and renewal |
| `/infra:deploy-check` | Pre/post deploy metric comparison |
| `/infra:report` | Generate daily/weekly infrastructure reports |
| `/infra:config` | Manage server inventory (add/edit/remove) |

### Example Workflow

```bash
# Morning health check
/infra:status

# Server showing high CPU? Diagnose it
/infra:diagnose machine4 "CPU high"

# Disk full? Auto-clean
/infra:heal machine4 disk-cleanup --dry-run
/infra:heal machine4 disk-cleanup

# Check SSL certs
/infra:cert --check

# Generate weekly report
/infra:report --type weekly
```

The commands use `infra-god` CLI internally, so make sure the binary is built and available in the project directory.

## Requirements

- Go 1.24+
- SSH access to target servers (password or key-based)
- Target servers: Linux with standard utils (`free`, `df`, `ps`, `ss`, etc.)
- Optional: `nvidia-smi` for GPU monitoring, `docker` for container monitoring

## License

MIT
