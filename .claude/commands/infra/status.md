---
allowed-tools: [Read, Bash]
description: "전체 서버 상태를 병렬로 조회하여 한눈에 표시"
---

# /infra:status - 전체 서버 상태 조회

## Purpose
infra-god CLI를 사용하여 모든 서버의 상태를 병렬 조회하고 통합 대시보드를 출력한다.

## Usage
```
/infra:status [--target group|server] [--detail] [--json] [--sort cpu|mem|disk|load] [--watch 30s]
```

## Arguments
- `--target` - 조회 대상 (그룹명 또는 서버명). 기본값: 전체 active
- `--detail` - 상세 정보 포함 (Docker, 네트워크)
- `--json` - JSON 형식 출력
- `--sort` - 정렬 기준 (cpu, mem, disk, load)
- `--watch` - 주기적 새로고침

## Execution

### 1. CLI 실행
```bash
# 전체 서버 상태
./infra-god status

# 특정 그룹
./infra-god status --group mains

# 특정 서버들
./infra-god status server1 server2

# JSON 출력
./infra-god status --json

# 정렬
./infra-god status --sort cpu

# 실시간 모니터링
./infra-god status --watch 30s
```

### 2. 출력 형식
CLI가 자동으로 테이블 형태로 출력:
```
═══ INFRA-GOD STATUS                              2026-03-11 14:30:00 ═══
SERVER       ROLE     OS           CPU%  MEM%  DISK%  GPU          LOAD  UPTIME
✅ web-1     main     ubuntu 22.04  23%   45%   67%  -            0.5   14d
✅ gpu-1     machine  ubuntu 22.04  45%   62%   71%  A100 32%     2.1   7d
⚠️ worker-1  worker   debian 12     82%   78%   91%  RTX4090 87%  6.3   3d
⏹  gpu-2     machine  -             -     -     -    -            -     stopped

 TOTAL: 4 servers | 2 ok | 1 warning | 0 error | 1 stopped

 ALERTS:
  - worker-1  disk 91%
  - worker-1  load 6.3 (CPUs: 8)
```

### 3. 이상 감지 시 후속 안내
- 경고/위험 서버가 있으면 `./infra-god inspect [server]` 사용을 안내
- 디스크 위험이면 `./infra-god heal [server] disk --dry-run` 안내

## 환경변수
- `INFRA_SSH_PASS` - SSH 비밀번호 (password auth 사용 시)
