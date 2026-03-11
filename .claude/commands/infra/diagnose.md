---
allowed-tools: [Read, Bash, Task, mcp__sequential-thinking__sequentialthinking]
description: "서버 문제를 AI 기반으로 진단하고 근본 원인을 분석"
---

# /infra:diagnose - AI 기반 문제 진단

## Purpose
서버의 이상 증상을 체계적으로 조사하고, sequential-thinking을 활용하여 근본 원인을 분석한다.

## Usage
```
/infra:diagnose [server] [symptom] [--deep] [--auto-fix]
```

## Arguments
- `server` - 대상 서버명 (필수)
- `symptom` - 증상 설명 (선택, 없으면 자동 탐지)
- `--deep` - 심층 분석 (로그, 히스토리, 트렌드 포함)
- `--auto-fix` - 안전한 수정은 자동 적용

## 진단 가능 증상
- CPU 과부하 (high cpu, cpu spike)
- 메모리 부족/누수 (memory leak, oom, high memory)
- 디스크 풀 (disk full, no space)
- 프로세스 사망 (process down, service stopped)
- 네트워크 문제 (connection refused, timeout, packet loss)
- 응답 지연 (slow response, high latency)
- GPU 이상 (gpu error, cuda error)

## Execution

### 1. 증상 분류
- sequential-thinking으로 증상을 분석하고 진단 계획 수립:
  - 가능한 원인 가설 3~5개 생성
  - 각 가설 검증에 필요한 데이터 수집 계획
  - 우선순위에 따른 조사 순서 결정

### 2. 데이터 수집 (증상별)

**CPU 과부하:**
```bash
ssh user@host "
  top -bn1 -o %CPU | head -15;
  ps aux --sort=-%cpu | head -10;
  uptime;
  dmesg | tail -20;
  cat /proc/loadavg;
"
```

**메모리 부족/누수:**
```bash
ssh user@host "
  free -m;
  ps aux --sort=-%mem | head -10;
  cat /proc/meminfo | grep -E 'MemTotal|MemAvailable|SwapTotal|SwapFree';
  dmesg | grep -i 'oom\|out of memory' | tail -5;
  smem -t -k 2>/dev/null | tail -10;
"
```

**디스크 풀:**
```bash
ssh user@host "
  df -h;
  df -i;
  du -sh /var/log/* 2>/dev/null | sort -rh | head -10;
  du -sh /tmp/* 2>/dev/null | sort -rh | head -5;
  find / -xdev -size +100M -exec ls -lh {} \; 2>/dev/null | head -10;
  journalctl --disk-usage;
"
```

**프로세스 사망:**
```bash
ssh user@host "
  systemctl status [service] 2>/dev/null;
  journalctl -u [service] --since '30min ago' --no-pager | tail -30;
  lsof -i :[port] 2>/dev/null;
  coredumpctl list 2>/dev/null | tail -5;
"
```

**GPU 이상:**
```bash
ssh user@host "
  nvidia-smi 2>&1;
  nvidia-smi --query-gpu=ecc.errors.corrected.volatile.total --format=csv 2>/dev/null;
  dmesg | grep -i 'nvidia\|gpu\|xid' | tail -10;
  nvidia-smi --query-compute-apps=pid,process_name,used_memory --format=csv 2>/dev/null;
"
```

### 3. AI 분석
- sequential-thinking으로 수집된 데이터를 분석:
  - 각 가설에 대해 수집된 증거 평가
  - 근본 원인 식별
  - 영향 범위 평가
  - 해결 방안 도출 (즉시/단기/장기)

### 4. 진단 보고서 출력
```
╔══════════════════════════════════════════════════════╗
║  DIAGNOSE: machine4 - "CPU 과부하"                    ║
╠══════════════════════════════════════════════════════╣
║                                                      ║
║  [증상] CPU 82%, load average 18.5 (16코어)            ║
║                                                      ║
║  [근본 원인]                                          ║
║  python3 (PID 4521) - ML 학습 프로세스                 ║
║  → 16코어 모두 점유, OOM 임박 (RAM 28/32GB)            ║
║                                                      ║
║  [영향]                                               ║
║  - 다른 서비스 응답 지연 (nginx p99: 890ms)             ║
║  - swap 활성화로 추가 성능 저하 가능성                    ║
║                                                      ║
║  [권장 조치]                                           ║
║  즉시: nice -n 10으로 우선순위 낮추기                    ║
║  단기: 해당 학습을 GPU 서버로 이전                       ║
║  장기: 리소스 제한 (cgroup) 설정                        ║
║                                                      ║
║  /infra:heal machine4 renice --pid 4521 으로 즉시 조치  ║
╚══════════════════════════════════════════════════════╝
```

### 5. --auto-fix 시
- 위험도 낮은 조치만 자동 실행 (로그 정리, renice 등)
- 서비스 재시작, 프로세스 kill 등은 확인 요청

## Claude Code Integration
- Read로 servers.yaml 로드
- Bash로 SSH 진단 명령 실행
- sequential-thinking으로 근본 원인 분석
- Task 서브에이전트로 병렬 데이터 수집 (--deep 시)
