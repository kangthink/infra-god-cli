---
allowed-tools: [Read, Bash]
description: "여러 서버에 명령어를 병렬로 실행"
---

# /infra:exec - 병렬 명령 실행

## Purpose
infra-god CLI를 사용하여 지정된 서버들에 명령어를 병렬로 실행하고 결과를 취합한다.

## Usage
```
/infra:exec [command] [--target group|server] [--sudo] [--dry-run] [--serial] [--yes]
```

## Arguments
- `command` - 실행할 쉘 명령어
- `--target` - 대상 서버/그룹. 기본값: 전체 active
- `--sudo` - sudo 권한으로 실행
- `--dry-run` - 실행하지 않고 대상 서버와 명령만 표시
- `--serial` - 병렬 대신 순차 실행
- `--yes` - 위험 명령 시 확인 없이 실행

## Execution

### 1. CLI 실행
```bash
# 전체 서버에 명령 실행
./infra-god exec "uptime"

# 특정 그룹 대상
./infra-god exec "df -h" --group workers

# 특정 서버 대상
./infra-god exec "nvidia-smi" server1 server2

# sudo 실행
./infra-god exec "apt update && apt upgrade -y" --sudo --yes

# dry-run
./infra-god exec "reboot" --sudo --dry-run
```

### 2. 안전 기능
CLI에 내장된 안전 기능:
- **차단**: `rm -rf /`, `mkfs`, `dd if=` 등 파괴적 명령 자동 차단
- **확인 요청**: `reboot`, `shutdown`, `docker rm`, `systemctl stop` 등 중위험 명령
- **경고**: `apt upgrade`, `chmod -R` 등 주의 명령

### 3. 출력 형식
```
═══ EXEC: apt update    --all (4 servers) ═══
 ✅ web-1 (234ms)
 <output>
 ✅ gpu-1 (456ms)
 <output>
 ❌ worker-1 (10s) SSH handshake failed

 SUMMARY: 2/3 succeeded | avg 345ms
```

## 환경변수
- `INFRA_SSH_PASS` - SSH 비밀번호 (password auth 사용 시)
