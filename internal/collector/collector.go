package collector

// Remote commands for system information collection.
// Each command is designed to produce parseable output via SSH.

// StatusCmd collects quick status metrics (CPU%, MEM%, DISK%, GPU, load, uptime).
const StatusCmd = `
cpu_idle=$(top -bn1 | grep '%Cpu' | awk '{print $8}' | cut -d. -f1 2>/dev/null || echo "0")
cpu_used=$((100 - ${cpu_idle:-0}))
echo "CPU:${cpu_used}"

mem_info=$(free -b | grep Mem)
mem_total=$(echo "$mem_info" | awk '{print $2}')
mem_used=$(echo "$mem_info" | awk '{print $3}')
mem_avail=$(echo "$mem_info" | awk '{print $7}')
if [ "$mem_total" -gt 0 ] 2>/dev/null; then
  mem_pct=$((mem_used * 100 / mem_total))
else
  mem_pct=0
fi
echo "MEM:${mem_pct}:${mem_total}:${mem_used}:${mem_avail}"

disk_info=$(df / | tail -1)
disk_pct=$(echo "$disk_info" | awk '{print $5}' | tr -d '%')
disk_total=$(echo "$disk_info" | awk '{print $2}')
disk_used=$(echo "$disk_info" | awk '{print $3}')
disk_avail=$(echo "$disk_info" | awk '{print $4}')
echo "DISK:${disk_pct}:${disk_total}:${disk_used}:${disk_avail}"

load=$(cat /proc/loadavg | awk '{print $1}')
echo "LOAD:${load}"

cpus=$(grep -c processor /proc/cpuinfo)
echo "CPUS:${cpus}"

uptime_sec=$(cat /proc/uptime | awk '{print $1}' | cut -d. -f1)
echo "UPTIME:${uptime_sec}"

# OS and kernel
os_name=$(grep '^ID=' /etc/os-release 2>/dev/null | cut -d= -f2 | tr -d '"')
os_ver=$(grep '^VERSION_ID=' /etc/os-release 2>/dev/null | cut -d= -f2 | tr -d '"')
kern=$(uname -r | sed 's/-generic//')
echo "OS:${os_name}:${os_ver}:${kern}"

# GPU (nvidia-smi)
if command -v nvidia-smi &>/dev/null; then
  nvidia-smi --query-gpu=name,memory.total,memory.used,utilization.gpu,temperature.gpu --format=csv,noheader,nounits 2>/dev/null | while IFS=',' read -r name mtotal mused util temp; do
    name=$(echo "$name" | xargs)
    mtotal=$(echo "$mtotal" | xargs)
    mused=$(echo "$mused" | xargs)
    util=$(echo "$util" | xargs)
    temp=$(echo "$temp" | xargs)
    echo "GPU:${name}:${mtotal}:${mused}:${util}:${temp}"
  done
else
  echo "GPU:none"
fi

# Docker container count
if command -v docker &>/dev/null; then
  total=$(docker ps -q 2>/dev/null | wc -l)
  healthy=$(docker ps --filter health=healthy -q 2>/dev/null | wc -l)
  unhealthy=$(docker ps --filter health=unhealthy -q 2>/dev/null | wc -l)
  restarting=$(docker ps --filter status=restarting -q 2>/dev/null | wc -l)
  echo "DOCKER:${total}:${healthy}:${unhealthy}:${restarting}"
else
  echo "DOCKER:none"
fi
`

// InspectHWCmd collects detailed hardware info.
const InspectHWCmd = `
echo "===CPU==="
lscpu | grep -E 'Model name|^CPU\(s\)|Core|Socket|Thread'
echo "CPU_TEMP:"
cat /sys/class/thermal/thermal_zone*/temp 2>/dev/null | head -1 || sensors 2>/dev/null | grep -i 'Package' | awk '{print $4}' || echo "N/A"

echo "===GPU==="
nvidia-smi --query-gpu=index,name,memory.total,memory.used,utilization.gpu,temperature.gpu,driver_version --format=csv,noheader 2>/dev/null || echo "none"

echo "===RAM==="
free -h

echo "===STORAGE==="
df -h | grep -E '^/dev|Filesystem'
echo "---INODE---"
df -i | grep -E '^/dev' | head -5
`

// InspectGPUCmd collects GPU details and running processes.
const InspectGPUCmd = `
echo "===GPU==="
if command -v nvidia-smi &>/dev/null; then
  nvidia-smi --query-gpu=index,name,memory.total,memory.used,utilization.gpu,temperature.gpu,driver_version,pcie.link.gen.current,pcie.link.width.current --format=csv,noheader 2>/dev/null || echo "none"
  echo "===GPU_PROCS==="
  nvidia-smi --query-compute-apps=gpu_uuid,pid,used_memory,name --format=csv,noheader,nounits 2>/dev/null || echo "none"
  echo "===CUDA==="
  nvcc --version 2>/dev/null | grep release | awk '{print $6}' | tr -d ',' || echo "N/A"
  echo "===NVIDIA_DRIVER==="
  cat /proc/driver/nvidia/version 2>/dev/null | head -1 | awk '{print $8}' || echo "N/A"
else
  echo "none"
fi
`

// InspectSoftwareCmd collects OS and software info.
const InspectSoftwareCmd = `
echo "===OS==="
cat /etc/os-release | grep -E '^(NAME|VERSION|ID)='
uname -r

echo "===SOFTWARE==="
echo "docker:$(docker --version 2>/dev/null || echo N/A)"
echo "python:$(python3 --version 2>/dev/null || echo N/A)"
echo "node:$(node --version 2>/dev/null || echo N/A)"
echo "go:$(go version 2>/dev/null || echo N/A)"
echo "java:$(java --version 2>/dev/null | head -1 || echo N/A)"
echo "nvidia-driver:$(nvidia-smi --query-gpu=driver_version --format=csv,noheader 2>/dev/null | head -1 || echo N/A)"
echo "cuda:$(nvcc --version 2>/dev/null | grep release | awk '{print $6}' | tr -d ',' || echo N/A)"
`

// InspectDockerCmd collects Docker container details with restart policy.
const InspectDockerCmd = `
if command -v docker &>/dev/null; then
  for c in $(docker ps -aq 2>/dev/null); do
    docker inspect "$c" --format '{{.Name}}	{{.State.Status}} {{if .State.Health}}({{.State.Health.Status}}){{end}} up {{.State.StartedAt | printf "%.10s"}}	{{.Config.Image}}	{{.HostConfig.RestartPolicy.Name}}' 2>/dev/null | sed 's|^/||'
  done
else
  echo "none"
fi
`

// InspectNetworkCmd collects network info.
const InspectNetworkCmd = `
echo "===INTERFACES==="
ip -br addr | grep -v lo
echo "===PORTS==="
ss -tlnp 2>/dev/null | grep LISTEN | head -30
`

// InspectServicesCmd collects systemd service info.
const InspectServicesCmd = `
echo "===FAILED==="
systemctl --failed --no-pager --no-legend 2>/dev/null

echo "===CUSTOM==="
systemctl list-units --type=service --state=active --no-pager --no-legend 2>/dev/null | grep -vE 'systemd-|dbus|cron\.|ssh\.|network|udev|polkit|rsyslog|irqbalance|multipathd|packagekit|udisks|upower|ModemManager|wpa_supplicant|accounts-daemon|avahi|bolt|colord|fwupd|kerneloops|power-profiles|rtkit|switcheroo|thermald|snap\.' | head -20
`

// InspectStorageCmd collects mount points with top-level folder sizes.
const InspectStorageCmd = `
echo "===MOUNTS==="
df -hT | grep -E '^/dev' | grep -v 'loop\|squashfs\|tmpfs'
echo "---"
df -hT | grep -E '^/dev' | grep -c 'loop' | xargs -I{} echo "  (+ {} snap mounts hidden)"

echo "===INODE==="
df -i | grep '^/dev' | grep -v loop | head -10

echo "===FOLDERS==="
# For each real (non-loop, non-snap) mount point, show top folders + children
for mp in $(df -hT | grep '^/dev' | grep -v 'loop\|squashfs\|tmpfs' | awk '{print $NF}' | grep -v '^/$'); do
  echo "MOUNT:${mp}"
  for d in $(du -sh "${mp}"/*/ 2>/dev/null | sort -rh | head -6 | awk '{print $2}'); do
    size=$(du -sh "$d" 2>/dev/null | awk '{print $1}')
    echo "L1:${size}:${d}"
    du -sh "${d}"*/ 2>/dev/null | sort -rh | head -3 | while read s p; do
      echo "L2:${s}:${p}"
    done
  done
done
# Root mount — key directories + children
echo "MOUNT:/"
for d in /home /var /opt /usr /tmp /srv /docker /snap /root; do
  size=$(du -sh "$d" 2>/dev/null | awk '{print $1}')
  [ -z "$size" ] && continue
  echo "L1:${size}:${d}"
  du -sh "${d}"/*/ 2>/dev/null | sort -rh | head -3 | while read s p; do
    echo "L2:${s}:${p}"
  done
done
`

// UsersCmd collects user and permission info.
const UsersCmd = `
echo "===LOGGED_IN==="
who 2>/dev/null

echo "===ACCOUNTS==="
awk -F: '$3 >= 1000 && $3 < 65534 {printf "%s:%s:%s:%s\n", $1, $3, $7, $6}' /etc/passwd 2>/dev/null
echo "root:0:/bin/bash:/root"

echo "===SUDOERS==="
getent group sudo 2>/dev/null | cut -d: -f4
getent group wheel 2>/dev/null | cut -d: -f4

echo "===SSH_CONFIG==="
grep -E '^(PasswordAuthentication|PermitRootLogin|PubkeyAuthentication|Port)' /etc/ssh/sshd_config 2>/dev/null || true
`

// PsCmd collects top processes by CPU and memory.
const PsCmd = `ps aux --sort=-%cpu | head -16`

// PsMemCmd collects top processes by memory.
const PsMemCmd = `ps aux --sort=-%mem | head -16`

// SecurityCmd collects security audit info.
const SecurityCmd = `
echo "===SSH==="
grep -E '^(PasswordAuthentication|PermitRootLogin|PubkeyAuthentication|Port|MaxAuthTries)' /etc/ssh/sshd_config 2>/dev/null

echo "===FAIL2BAN==="
systemctl is-active fail2ban 2>/dev/null || echo "not_installed"
fail2ban-client status 2>/dev/null | head -5

echo "===FIREWALL==="
ufw status 2>/dev/null || echo "ufw_not_installed"
iptables -L -n 2>/dev/null | head -10 || echo "no_iptables"

echo "===EMPTY_PASSWORDS==="
awk -F: '($2 == "" || $2 == "!") && $1 != "*" {print $1}' /etc/shadow 2>/dev/null

echo "===UID0==="
awk -F: '$3 == 0 {print $1}' /etc/passwd

echo "===LISTENING==="
ss -tlnp 2>/dev/null | grep LISTEN | awk '{print $4}' | head -30

echo "===UPDATES==="
apt list --upgradable 2>/dev/null | grep -c '/' || echo "0"
cat /var/run/reboot-required 2>/dev/null || echo "no_reboot"

echo "===DOCKER_SECURITY==="
docker ps --format '{{.Names}}:{{.Ports}}' 2>/dev/null | head -20
docker info 2>/dev/null | grep -E 'Root Dir|Security Options' | head -5
`

// HealDiskPlanCmd shows what can be cleaned and how much space.
const HealDiskPlanCmd = `
echo "===APT_CACHE==="
du -sh /var/cache/apt/archives/ 2>/dev/null || echo "0"

echo "===JOURNAL==="
journalctl --disk-usage 2>/dev/null || echo "0"

echo "===TMP==="
find /tmp -type f -atime +7 -exec du -cb {} + 2>/dev/null | tail -1 | awk '{print $1}' || echo "0"

echo "===DOCKER_IMAGES==="
docker system df 2>/dev/null | grep Images || echo "0"

echo "===SNAP==="
snap list --all 2>/dev/null | grep disabled | wc -l || echo "0"

echo "===OLD_LOGS==="
find /var/log -name '*.gz' -o -name '*.old' -o -name '*.1' 2>/dev/null | xargs du -cb 2>/dev/null | tail -1 | awk '{print $1}' || echo "0"

echo "===CURRENT_DISK==="
df -h / | tail -1
`

// HealDiskCleanCmd performs actual disk cleanup.
const HealDiskCleanCmd = `
echo "===APT==="
apt-get clean -y 2>/dev/null && echo "done" || echo "skip"

echo "===JOURNAL==="
journalctl --vacuum-size=500M 2>/dev/null && echo "done" || echo "skip"

echo "===TMP==="
find /tmp -type f -atime +7 -delete 2>/dev/null && echo "done" || echo "skip"

echo "===OLD_LOGS==="
find /var/log -name '*.gz' -o -name '*.old' -o -name '*.1' -delete 2>/dev/null && echo "done" || echo "skip"

echo "===DOCKER==="
docker image prune -f 2>/dev/null && echo "done" || echo "skip"

echo "===POST_DISK==="
df -h / | tail -1
`

// HealDiskCleanAggressiveCmd includes Docker volumes and builder cache.
const HealDiskCleanAggressiveCmd = HealDiskCleanCmd + `
echo "===DOCKER_BUILDER==="
docker builder prune -f 2>/dev/null && echo "done" || echo "skip"

echo "===SNAP==="
snap list --all 2>/dev/null | grep disabled | awk '{print "snap remove " $1 " --revision=" $3}' | sh 2>/dev/null && echo "done" || echo "skip"

echo "===POST_DISK_AGGRESSIVE==="
df -h / | tail -1
`

// HealSecurityHardenCmd applies basic security hardening.
const HealSecurityHardenCmd = `
echo "===FAIL2BAN==="
if ! command -v fail2ban-server &>/dev/null; then
  apt-get install -y fail2ban 2>/dev/null && systemctl enable --now fail2ban && echo "installed"
else
  echo "already_installed"
fi

echo "===UFW==="
if command -v ufw &>/dev/null; then
  ufw --force enable 2>/dev/null
  ufw default deny incoming 2>/dev/null
  ufw default allow outgoing 2>/dev/null
  ufw allow ssh 2>/dev/null
  echo "enabled"
else
  apt-get install -y ufw 2>/dev/null
  ufw --force enable 2>/dev/null
  ufw default deny incoming 2>/dev/null
  ufw default allow outgoing 2>/dev/null
  ufw allow ssh 2>/dev/null
  echo "installed_and_enabled"
fi

echo "===SSH_HARDENING==="
cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak
sed -i 's/^#*PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
sed -i 's/^#*MaxAuthTries.*/MaxAuthTries 5/' /etc/ssh/sshd_config
systemctl reload sshd 2>/dev/null || systemctl reload ssh 2>/dev/null
echo "hardened"
`
