---
allowed-tools: [Read, Bash, Task]
description: "여러 서버에 명령어를 병렬로 실행"
---

# /infra:exec - 병렬 명령 실행

## Purpose
지정된 서버 그룹에 명령어를 병렬로 실행하고 결과를 취합한다.

## Usage
```
/infra:exec [command] [--target group|server] [--sudo] [--timeout 30] [--dry-run]
```

## Arguments
- `command` - 실행할 쉘 명령어 (따옴표로 감싸기)
- `--target` - 대상 서버/그룹. 기본값: active
- `--sudo` - sudo 권한으로 실행
- `--timeout` - 명령 타임아웃 초 (기본: 30)
- `--dry-run` - 실행하지 않고 대상 서버와 명령만 표시
- `--confirm` - 위험 명령 시 확인 없이 실행 (기본: 위험 명령은 확인 요청)

## Execution

### 1. 사전 검증
- servers.yaml에서 대상 서버 목록 로드
- 명령어 위험도 평가:
  - **차단**: `rm -rf /`, `mkfs`, `dd if=`, `:(){ :|:& };:` 등
  - **확인 필요**: `rm`, `kill`, `systemctl stop`, `reboot`, `shutdown`, `apt remove`, `pip uninstall`
  - **안전**: 그 외 읽기 전용 명령

### 2. --dry-run 시
```
[DRY-RUN] 대상: web-1(10.0.1.1), web-2(10.0.1.2), gpu-1(10.0.1.10)
[DRY-RUN] 명령: apt update && apt upgrade -y
[DRY-RUN] 옵션: sudo=true, timeout=60s
실행하려면 --dry-run 플래그를 제거하세요.
```

### 3. 병렬 실행
- Task 서브에이전트를 서버 수만큼 생성
- 각 서브에이전트가 Bash로 SSH 명령 실행:
```bash
ssh [-t] user@host "[sudo] command"
```
- wired IP 우선 시도, 실패 시 wireless IP fallback

### 4. 결과 취합
```
╔═══════════════════════════════════════════════════════╗
║  EXEC: apt update && apt upgrade -y                  ║
║  TARGET: mains (2 servers)  SUDO: yes  TIMEOUT: 60s  ║
╠═══════════════════════════════════════════════════════╣
║  main1   │ ✅ 성공 │ exit:0 │ 12.3s │ 45 packages upgraded  ║
║  main2   │ ✅ 성공 │ exit:0 │ 15.1s │ 45 packages upgraded  ║
╠═══════════════════════════════════════════════════════╣
║  결과: 2/2 성공                                        ║
╚═══════════════════════════════════════════════════════╝
```

- 실패 서버가 있으면 stderr 출력 포함
- `--json` 시 구조화된 JSON 출력

## 사용 예시
```
/infra:exec "apt update && apt upgrade -y" --target all --sudo
/infra:exec "docker pull nginx:latest" --target machines
/infra:exec "nvidia-smi" --target gpu
/infra:exec "df -h" --target active
/infra:exec "systemctl restart nginx" --target mains --sudo --confirm
```

## Claude Code Integration
- Read로 servers.yaml 로드
- Task 서브에이전트로 병렬 SSH 실행
- Bash로 실제 SSH 명령 수행
- 위험 명령 감지 시 사용자에게 확인 요청
