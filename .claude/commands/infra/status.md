---
allowed-tools: [Read, Bash, Glob, Task]
description: "전체 서버 상태를 병렬로 조회하여 한눈에 표시"
---

# /infra:status - 전체 서버 상태 조회

## Purpose
servers.yaml에 등록된 모든 서버의 상태를 병렬로 조회하여 통합 대시보드 형태로 출력한다.

## Usage
```
/infra:status [--target group|server] [--detail] [--json]
```

## Arguments
- `--target` - 조회 대상 (그룹명: all, mains, machines, workers, gpu, active / 서버명: main1, machine3 등). 기본값: active
- `--detail` - 상세 정보 포함 (GPU, Docker, 네트워크)
- `--json` - JSON 형식 출력

## Execution

### 1. 설정 로드
- `servers.yaml` 파일을 Read로 읽어 서버 목록과 접속 정보를 파싱한다
- `--target`에 따라 대상 서버 목록을 필터링한다
- status: stopped인 서버는 "종료" 표시만 하고 스킵한다

### 2. 병렬 상태 수집
- Task 서브에이전트를 서버 수만큼 생성하여 **병렬로** SSH 접속한다
- 각 서브에이전트는 Bash로 다음 명령을 실행한다:

```bash
# 접속 IP 결정: wired 우선, 실패 시 wireless
# 하드웨어
ssh user@host "
  # CPU
  echo '=CPU='; nproc; top -bn1 | grep 'Cpu(s)' | awk '{print \$2}';
  # GPU (있으면)
  echo '=GPU='; nvidia-smi --query-gpu=name,utilization.gpu,memory.used,memory.total,temperature.gpu --format=csv,noheader,nounits 2>/dev/null || echo 'N/A';
  # RAM
  echo '=RAM='; free -m | awk '/Mem:/{printf \"%d/%dMB (%.0f%%)\", \$3, \$2, \$3/\$2*100}';
  # Storage
  echo '=DISK='; df -h --output=target,size,used,pcent / | tail -1;
"
```

- `--detail` 플래그 시 추가 수집:
```bash
ssh user@host "
  # OS/커널
  echo '=OS='; cat /etc/os-release | grep PRETTY_NAME; uname -r;
  # Docker
  echo '=DOCKER='; docker ps --format '{{.Names}}\t{{.Status}}' 2>/dev/null || echo 'N/A';
  # 네트워크
  echo '=NET='; ip -4 addr show | grep inet | awk '{print \$NF, \$2}';
  # 로그인 사용자
  echo '=USERS='; who | awk '{print \$1}' | sort -u;
"
```

### 3. 결과 취합 및 출력

**기본 출력 형식:**
```
╔══════════════════════════════════════════════════════════════════════╗
║  INFRA-GOD STATUS                              2026-02-19 14:30:00 ║
╠══════════════════════════════════════════════════════════════════════╣
║  SERVER     │ STATUS │  CPU  │  GPU  │   RAM    │  DISK  │ UPTIME  ║
╠════════════╪════════╪═══════╪═══════╪══════════╪════════╪═════════╣
║  main1      │  ✅    │  23%  │  N/A  │ 4/16GB   │  67%   │  45d    ║
║  main2      │  ✅    │  18%  │  N/A  │ 3/16GB   │  55%   │  30d    ║
║  machine1   │  ✅    │  45%  │  78%  │ 12/32GB  │  72%   │  12d    ║
║  machine2   │  ⏹️    │  -    │   -   │    -     │   -    │   -     ║
║  machine3   │  ✅    │  12%  │  25%  │ 8/64GB   │  45%   │  60d    ║
║  machine4   │  ⚠️    │  82%  │  91%  │ 28/32GB  │  88%   │  7d     ║
║  machine5   │  ✅    │  35%  │  50%  │ 16/64GB  │  60%   │  22d    ║
║  worker1    │  ✅    │  10%  │  N/A  │ 2/8GB    │  40%   │  90d    ║
║  worker2    │  ✅    │  8%   │  N/A  │ 3/16GB   │  35%   │  15d    ║
║  worker3    │  ❌    │  -    │   -   │    -     │   -    │   -     ║
╚══════════════════════════════════════════════════════════════════════╝
  ✅ 7 정상  ⚠️ 1 경고  ❌ 1 접속불가  ⏹️ 1 종료
```

**상태 판단 기준:**
- ✅ 정상: 모든 지표 정상 범위
- ⚠️ 경고: CPU >80% OR RAM >85% OR Disk >85% OR GPU >90%
- ❌ 접속불가: SSH 연결 실패
- ⏹️ 종료: status: stopped

### 4. 이상 감지 시 후속 안내
- 경고/위험 서버가 있으면 `/infra:inspect [server]` 또는 `/infra:diagnose [server]` 사용을 안내한다

## Claude Code Integration
- Read로 servers.yaml 설정 로드
- Task 서브에이전트로 서버별 병렬 SSH 실행
- Bash로 SSH 명령 수행
- 결과를 파싱하여 테이블 형태로 출력
