---
allowed-tools: [Read, Bash, Task]
description: "특정 서버의 하드웨어/소프트웨어/네트워크/사용자 정보를 심층 수집"
---

# /infra:inspect - 서버 심층 분석

## Purpose
특정 서버의 전체 정보를 수집하여 상세 보고서를 생성한다.
servers.yaml의 monitor_schema에 정의된 모든 항목을 수집한다.

## Usage
```
/infra:inspect [server] [--section hardware|software|network|users|folders|all]
```

## Arguments
- `server` - 대상 서버명 (필수)
- `--section` - 특정 섹션만 조회. 기본값: all
- `--json` - JSON 형식 출력
- `--save` - 결과를 파일로 저장

## Execution

### 1. 서버 접속
- servers.yaml에서 대상 서버 IP 및 접속 정보 로드
- SSH 접속 (wired 우선)

### 2. 정보 수집 (섹션별)

**하드웨어 (hardware)**
```bash
ssh user@host "
  echo '===CPU===';
  lscpu | grep -E 'Model name|Socket|Core|Thread|MHz|Architecture';
  echo '===GPU===';
  nvidia-smi --query-gpu=name,driver_version,memory.total,memory.used,utilization.gpu,temperature.gpu --format=csv,noheader 2>/dev/null || echo 'No NVIDIA GPU';
  lspci | grep -i 'vga\|3d\|display' 2>/dev/null;
  echo '===RAM===';
  free -h;
  echo '===STORAGE===';
  lsblk -o NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE;
  df -hT;
"
```

**소프트웨어 (software)**
```bash
ssh user@host "
  echo '===OS===';
  cat /etc/os-release | grep -E 'PRETTY_NAME|VERSION_ID';
  echo '===KERNEL===';
  uname -r;
  echo '===ESSENTIAL===';
  docker --version 2>/dev/null; python3 --version 2>/dev/null; node --version 2>/dev/null; java -version 2>&1 | head -1; go version 2>/dev/null; rustc --version 2>/dev/null;
  echo '===RUNNING===';
  systemctl list-units --type=service --state=running --no-pager | head -30;
  echo '===DOCKER===';
  docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null;
"
```

**사용자/그룹/권한 (users)**
```bash
ssh user@host "
  echo '===LOGGED_IN===';
  who;
  echo '===ALL_USERS===';
  awk -F: '\$3>=1000{print \$1,\$3,\$6,\$7}' /etc/passwd;
  echo '===GROUPS===';
  getent group | awk -F: '\$3>=1000{print \$1,\$4}';
  echo '===SUDOERS===';
  getent group sudo 2>/dev/null || getent group wheel 2>/dev/null;
"
```

**네트워크 (network)**
```bash
ssh user@host "
  echo '===INTERFACES===';
  ip -4 addr show | grep -E 'inet |state';
  echo '===ROUTES===';
  ip route show default;
  echo '===PORTS===';
  ss -tlnp 2>/dev/null | head -20;
  echo '===DNS===';
  cat /etc/resolv.conf | grep nameserver;
"
```

**폴더구조 (folders)**
```bash
ssh user@host "
  echo '===HOME===';
  ls -la /home/;
  echo '===WORKSPACE===';
  find /home -maxdepth 3 -name 'workspace' -o -name 'projects' -o -name 'repos' 2>/dev/null;
  echo '===DISK_USAGE===';
  du -sh /home/*/ 2>/dev/null;
"
```

### 3. 출력 형식
```
╔══════════════════════════════════════════════════════╗
║  INSPECT: gpu-1 (10.0.1.10)                         ║
╠══════════════════════════════════════════════════════╣
║                                                      ║
║  [HARDWARE]                                          ║
║  CPU    : AMD Ryzen 9 5950X (16C/32T) @ 3.4GHz      ║
║  GPU    : NVIDIA RTX 3090 24GB (45°C, 25%)           ║
║  RAM    : 12/64GB (18%) | Swap: 0/8GB                ║
║  Storage: / 120/500GB (24%) | /data 800/2TB (40%)    ║
║                                                      ║
║  [SOFTWARE]                                          ║
║  OS     : Ubuntu 22.04.3 LTS (kernel 5.15.0-91)     ║
║  Docker : 24.0.7 | Python 3.10.12 | Node 18.19.0    ║
║  Running: nginx, postgres, docker (12 containers)    ║
║                                                      ║
║  [USERS]                                             ║
║  Logged : deploy(pts/0), admin(pts/1)                ║
║  Sudoers: deploy, admin                              ║
║                                                      ║
║  [NETWORK]                                           ║
║  eth0   : 10.0.1.10/24 (UP)                          ║
║  wlan0  : 10.0.2.10/24 (UP)                          ║
║  Ports  : 22, 80, 443, 5432, 8080                    ║
║                                                      ║
║  [WORKSPACE]                                         ║
║  /home/deploy/workspace (45GB)                       ║
║  /home/admin/projects (120GB)                        ║
╚══════════════════════════════════════════════════════╝
```

## Claude Code Integration
- Read로 servers.yaml 로드
- Bash로 SSH 명령 실행하여 정보 수집
- 결과를 구조화하여 출력
