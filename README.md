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

# Set SSH password (if using password auth)
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

## Requirements

- Go 1.24+
- SSH access to target servers (password or key-based)
- Target servers: Linux with standard utils (`free`, `df`, `ps`, `ss`, etc.)
- Optional: `nvidia-smi` for GPU monitoring, `docker` for container monitoring

## License

MIT
