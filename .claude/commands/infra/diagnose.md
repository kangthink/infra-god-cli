---
allowed-tools: [Read, Bash, mcp__sequential-thinking__sequentialthinking]
description: "서버 문제를 AI 기반으로 진단하고 근본 원인을 분석"
---

# /infra:diagnose - AI 기반 문제 진단

## Purpose
infra-god CLI로 서버 데이터를 수집하고, sequential-thinking으로 근본 원인을 분석한다.

## Usage
```
/infra:diagnose [server] [symptom] [--deep]
```

## Arguments
- `server` - 대상 서버명 (필수)
- `symptom` - 증상 설명 (선택, 없으면 자동 탐지)
- `--deep` - 심층 분석 (로그, 히스토리, 트렌드 포함)

## 진단 가능 증상
- CPU 과부하 (high cpu, cpu spike)
- 메모리 부족/누수 (memory leak, oom, high memory)
- 디스크 풀 (disk full, no space)
- 프로세스 사망 (process down, service stopped)
- 네트워크 문제 (connection refused, timeout)
- GPU 이상 (gpu error, cuda error, driver mismatch)

## Execution

### 1. 데이터 수집 (infra-god CLI 사용)
```bash
# 서버 전체 상태 확인
./infra-god status server-name

# 상세 하드웨어/소프트웨어 정보
./infra-god inspect server-name

# 프로세스 목록 (CPU/메모리 순)
./infra-god ps server-name
./infra-god ps server-name --sort mem

# GPU 상세 (GPU 관련 증상 시)
./infra-god inspect server-name --gpu

# Docker 상태 (컨테이너 관련 증상 시)
./infra-god inspect server-name --docker

# 보안 감사 (보안 관련 증상 시)
./infra-god security server-name
```

### 2. 증상별 추가 데이터 수집 (exec 활용)
```bash
# CPU 과부하 — 상세 프로세스 및 로드
./infra-god exec "uptime; cat /proc/loadavg; dmesg | tail -20" server-name

# 메모리 부족 — OOM 이력, swap 상태
./infra-god exec "dmesg | grep -i 'oom\|out of memory' | tail -5; cat /proc/meminfo | grep -E 'MemTotal|MemAvailable|Swap'" server-name

# 디스크 풀 — 대용량 파일/디렉토리
./infra-god exec "du -sh /var/log/* 2>/dev/null | sort -rh | head -10; journalctl --disk-usage" server-name

# 프로세스 사망 — 서비스 로그
./infra-god exec "journalctl -u SERVICE_NAME --since '30min ago' --no-pager | tail -30" server-name --sudo

# GPU 이상 — 커널 로그
./infra-god exec "dmesg | grep -i 'nvidia\|gpu\|xid' | tail -10; cat /proc/driver/nvidia/version 2>/dev/null" server-name
```

### 3. AI 분석 (sequential-thinking)
수집된 데이터를 sequential-thinking에 전달하여:
1. 가능한 원인 가설 3~5개 생성
2. 수집된 증거로 각 가설 검증
3. 근본 원인 식별
4. 영향 범위 평가
5. 해결 방안 도출 (즉시/단기/장기)

### 4. 진단 보고서 출력
```
═══ DIAGNOSE: server-name - "CPU 과부하" ═══

[증상] CPU 82%, load average 18.5 (16코어)

[근본 원인]
python3 (PID 4521) - ML 학습 프로세스
→ 16코어 모두 점유, OOM 임박 (RAM 28/32GB)

[영향]
- 다른 서비스 응답 지연
- swap 활성화로 추가 성능 저하 가능성

[권장 조치]
즉시: ./infra-god exec "renice -n 10 -p 4521" server-name --sudo --yes
단기: 해당 학습을 GPU 서버로 이전
장기: 리소스 제한 (cgroup) 설정
```

### 5. 후속 조치 안내
- 디스크 문제 → `./infra-god heal server-name disk-cleanup --dry-run`
- 서비스 재시작 → `./infra-god exec "systemctl restart SERVICE" server-name --sudo --yes`
- 보안 강화 → `./infra-god heal server-name security-harden`

## 환경변수
- `INFRA_SSH_PASS` - SSH 비밀번호 (password auth 사용 시)
