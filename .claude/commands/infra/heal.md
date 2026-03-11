---
allowed-tools: [Read, Bash]
description: "서버 문제에 대한 자동 복구 작업 실행"
---

# /infra:heal - 자동 복구 실행

## Purpose
infra-god CLI를 사용하여 진단된 서버 문제에 대해 사전 정의된 복구 절차를 실행한다.

## Usage
```
/infra:heal [server] [action] [--dry-run] [--aggressive]
```

## Arguments
- `server` - 대상 서버명 (필수)
- `action` - 복구 액션: `disk`, `security`
- `--dry-run` - 실행하지 않고 계획만 표시
- `--aggressive` - 강력한 정리 (Docker builder cache, snap 포함)

## 복구 액션

### disk - 디스크 정리
```bash
# 안전 모드 - 정리 계획 확인
./infra-god heal gpu-1 disk --dry-run

# 실제 정리 실행
./infra-god heal gpu-1 disk

# 공격적 정리 (Docker prune, snap 정리 포함)
./infra-god heal gpu-1 disk --aggressive
```

**정리 대상:**
- apt 캐시
- systemd 저널 (500MB 이상)
- /tmp 7일 이상 파일
- 로테이션된 로그 (.gz, .old, .1)
- Docker 미사용 이미지
- (--aggressive) Docker builder cache, 비활성 snap

**출력:**
```
═══ HEAL: gpu-1 - disk cleanup ═══
  BEFORE: / 88% used (440/500GB)

  APT cache:     1.2GB → cleaned
  Journal:       890MB → vacuumed to 500MB
  Old logs:      340MB → removed
  /tmp cleanup:  150MB → removed
  Docker images: 2.1GB → pruned

  AFTER: / 72% used (360/500GB) — 4.68GB freed
```

### security - 보안 강화
```bash
./infra-god heal gpu-1 security
```

**조치 내용:**
- fail2ban 설치/활성화
- ufw 방화벽 설정 (SSH 허용, 기본 차단)
- SSH 강화 (PermitRootLogin no, MaxAuthTries 5)

### 추가 복구 작업 (CLI exec 활용)
CLI에 내장되지 않은 복구는 exec로 수행:
```bash
# 서비스 재시작
./infra-god exec "systemctl restart nginx" gpu-1 --sudo --yes

# Docker 컨테이너 재시작
./infra-god exec "docker restart my-app" gpu-1

# 프로세스 우선순위 조정
./infra-god exec "renice -n 10 -p 4521" gpu-1 --sudo --yes

# 시간 동기화
./infra-god exec "timedatectl set-ntp true" gpu-1 --sudo --yes
```

## 환경변수
- `INFRA_SSH_PASS` - SSH 비밀번호 (password auth 사용 시)
