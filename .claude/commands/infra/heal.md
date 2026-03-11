---
allowed-tools: [Read, Bash, Task]
description: "서버 문제에 대한 자동 복구 작업 실행"
---

# /infra:heal - 자동 복구 실행

## Purpose
진단된 서버 문제에 대해 사전 정의된 복구 절차를 실행한다.
항상 사전 검증 → 실행 → 사후 검증의 3단계를 따른다.

## Usage
```
/infra:heal [server] [action] [--dry-run] [--force]
```

## Arguments
- `server` - 대상 서버명 (필수)
- `action` - 복구 액션 (아래 목록 참조)
- `--dry-run` - 실행하지 않고 계획만 표시
- `--force` - 확인 없이 실행 (주의)

## 복구 액션 목록

### disk-cleanup - 디스크 정리
```
/infra:heal [server] disk-cleanup [--aggressive]
```
**절차:**
1. 사전: `df -h`로 현재 사용량 확인
2. 실행:
   - `journalctl --vacuum-time=7d` (시스템 로그 정리)
   - `apt autoremove -y && apt clean` (패키지 캐시)
   - `/var/log/*.gz`, `/var/log/*.1` 삭제 (로테이션된 로그)
   - `/tmp` 7일 이상 파일 삭제
   - `--aggressive`: Docker 미사용 이미지/볼륨 정리
3. 사후: `df -h`로 확보된 공간 확인 및 보고

### restart-service - 서비스 재시작
```
/infra:heal [server] restart-service [service-name]
```
**절차:**
1. 사전: 서비스 상태 확인, 설정 파일 문법 검사
2. 실행: `systemctl restart [service]`
3. 사후: 서비스 상태 재확인, 포트 리스닝 확인, 헬스체크

### restart-docker - Docker 컨테이너 재시작
```
/infra:heal [server] restart-docker [container-name|all]
```
**절차:**
1. 사전: 컨테이너 상태 및 로그 마지막 10줄 확인
2. 실행: `docker restart [container]`
3. 사후: 컨테이너 상태, 헬스체크, 로그 확인

### renice - 프로세스 우선순위 조정
```
/infra:heal [server] renice --pid [pid] --priority [0-19]
```

### kill-process - 프로세스 종료
```
/infra:heal [server] kill-process --pid [pid] [--signal TERM|KILL]
```
**절차:**
1. 사전: 프로세스 정보 확인 (이름, 사용자, 리소스)
2. 확인: 사용자에게 종료 확인 요청 (--force 제외)
3. 실행: SIGTERM 먼저, 5초 후 미종료 시 SIGKILL
4. 사후: 프로세스 종료 확인

### rotate-logs - 로그 로테이션
```
/infra:heal [server] rotate-logs [--path /var/log]
```

### sync-time - 시간 동기화
```
/infra:heal [server] sync-time
```
**절차:** `timedatectl set-ntp true && systemctl restart systemd-timesyncd`

## Execution

### 안전 등급 분류
- **안전** (확인 불필요): disk-cleanup(기본), rotate-logs, sync-time, renice
- **주의** (확인 필요): restart-service, restart-docker, disk-cleanup(--aggressive)
- **위험** (반드시 확인): kill-process, 사용자 정의 명령

### 실행 흐름
```
1. servers.yaml에서 서버 정보 로드
2. 복구 액션 유효성 검증
3. [사전 검증] 현재 상태 수집
4. [--dry-run 시] 계획 출력 후 종료
5. [주의/위험 등급] 사용자 확인 요청
6. [실행] 복구 명령 수행
7. [사후 검증] 복구 결과 확인
8. 결과 보고
```

### 출력 형식
```
╔═════════════════════════════════════════════════════╗
║  HEAL: machine4 - disk-cleanup                      ║
╠═════════════════════════════════════════════════════╣
║                                                     ║
║  [사전] 디스크: / 88% (440/500GB)                     ║
║                                                     ║
║  [실행]                                              ║
║  ✅ journalctl vacuum: 1.2GB 확보                    ║
║  ✅ apt clean: 340MB 확보                            ║
║  ✅ old logs: 890MB 확보                             ║
║  ✅ /tmp cleanup: 150MB 확보                         ║
║                                                     ║
║  [사후] 디스크: / 72% (360/500GB) — 2.58GB 확보       ║
╚═════════════════════════════════════════════════════╝
```

## Claude Code Integration
- Read로 servers.yaml 로드
- Bash로 SSH 경유 복구 명령 실행
- 사전/사후 검증으로 안전한 복구 보장
