---
allowed-tools: [Read, Bash, Write]
description: "서버 인프라 상태 보고서 생성"
---

# /infra:report - 인프라 보고서 생성

## Purpose
infra-god CLI로 전체 인프라 상태를 수집하여 Markdown 보고서를 생성한다.

## Usage
```
/infra:report [--type daily|weekly|full] [--target group] [--output path]
```

## Arguments
- `--type` - 보고서 유형. 기본: daily
- `--target` - 대상 서버/그룹. 기본: 전체
- `--output` - 출력 파일 경로. 기본: `./reports/YYYY-MM-DD-[type].md`

## Execution

### 1. 데이터 수집 (CLI 사용)
```bash
# 전체 상태 (JSON)
./infra-god status --json

# 서버별 상세 정보 (필요 시)
./infra-god inspect server-name

# 보안 상태 (weekly/full)
./infra-god security server-name

# 사용자 정보 (full)
./infra-god users server-name
```

### 2. 보고서 유형별 수집 범위

**daily (일일 보고서):**
- `./infra-god status --json` — 전체 서버 상태
- 경고/위험 항목 목록

**weekly (주간 보고서):**
- daily 항목 전부
- SSL 인증서 만료 예정 (`/infra:cert --check`)
- 보안 감사 요약

**full (전체 인벤토리 보고서):**
- weekly 항목 전부
- `./infra-god inspect server --hw --software` — 서버별 하드웨어/소프트웨어 스펙
- `./infra-god users server` — 사용자/권한 현황
- `./infra-god inspect server --network` — 네트워크 구성

### 3. 보고서 형식

```markdown
# Infra-God Daily Report
**Date**: 2026-03-11 | **Servers**: 9 active / 1 stopped / 10 total

## Summary
| Status | Count | Servers |
|--------|-------|---------|
| ✅ Normal | 7 | web-1, web-2, gpu-1, ... |
| ⚠️ Warning | 1 | worker-1 (CPU: 82%, Disk: 88%) |
| ❌ Unreachable | 1 | worker-3 |
| ⏹ Stopped | 1 | gpu-2 |

## Server Details
| Server | OS | CPU% | MEM% | DISK% | GPU | LOAD | UPTIME |
|--------|----|------|------|-------|-----|------|--------|
| web-1 | ubuntu 22.04 | 23% | 45% | 67% | - | 0.5 | 14d |
| ... |

## Warnings & Issues
- **worker-1**: CPU 82%, load 6.3 (8 CPUs)
- **worker-1**: Disk 88%
- **worker-3**: SSH 접속 불가

## Recommendations
- worker-1: `./infra-god heal worker-1 disk-cleanup --dry-run` 실행 권장
- worker-3: 네트워크 연결 확인 필요
```

### 4. 파일 저장
- Write 도구로 `--output` 경로에 Markdown 저장
- 디렉토리 없으면 `reports/` 생성

## 환경변수
- `INFRA_SSH_PASS` - SSH 비밀번호 (password auth 사용 시)
