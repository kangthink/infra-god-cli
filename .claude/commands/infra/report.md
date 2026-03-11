---
allowed-tools: [Read, Bash, Task, Write]
description: "서버 인프라 상태 보고서 생성"
---

# /infra:report - 인프라 보고서 생성

## Purpose
전체 인프라 상태를 수집하여 Markdown 보고서를 생성한다.

## Usage
```
/infra:report [--type daily|weekly|full] [--target group] [--output path]
```

## Arguments
- `--type` - 보고서 유형. 기본: daily
- `--target` - 대상 서버/그룹. 기본: all
- `--output` - 출력 파일 경로. 기본: `./reports/YYYY-MM-DD-[type].md`

## 보고서 유형

### daily (일일 보고서)
수집 항목:
- 전체 서버 상태 요약 (✅/⚠️/❌ 카운트)
- 서버별 CPU/RAM/Disk/GPU 현재 수치
- 경고/위험 항목 목록
- Docker 컨테이너 상태
- 당일 발생 이슈 (dmesg, journalctl 최근 에러)

### weekly (주간 보고서)
daily 항목 + 추가:
- 주간 리소스 사용 추이 (일별 평균)
- 디스크 증가율 및 예상 포화일
- SSL 인증서 만료 예정 목록
- 주간 주요 이벤트 (재시작, 장애, 배포)

### full (전체 인벤토리 보고서)
weekly 항목 + 추가:
- 서버별 하드웨어 상세 스펙
- 설치된 소프트웨어 버전 목록
- 사용자/권한 현황
- 네트워크 구성 현황
- 워크스페이스 디스크 사용량

## Execution

### 1. 데이터 수집
- /infra:status 와 동일한 방식으로 병렬 SSH 수집
- 보고서 유형에 따라 수집 범위 결정

### 2. 보고서 생성

**daily 출력 형식:**
```markdown
# Infra-God Daily Report
**Date**: 2026-02-19 | **Servers**: 9 active / 1 stopped / 10 total

## Summary
| Status | Count | Servers |
|--------|-------|---------|
| ✅ Normal | 7 | main1, main2, machine1, machine3, machine5, worker1, worker2 |
| ⚠️ Warning | 1 | machine4 (CPU: 82%, Disk: 88%) |
| ❌ Unreachable | 1 | worker3 |
| ⏹️ Stopped | 1 | machine2 |

## Server Details
| Server | CPU | RAM | Disk | GPU | Docker | Uptime |
|--------|-----|-----|------|-----|--------|--------|
| main1 | 23% | 4/16GB | 67% | N/A | 3/3 | 45d |
| ... | ... | ... | ... | ... | ... | ... |

## Warnings & Issues
- **machine4**: CPU 82% — python3(PID 4521) ML 학습 프로세스
- **machine4**: Disk 88% — /var/log 비대화 (1.2GB)
- **worker3**: SSH 접속 불가 — 네트워크 확인 필요

## Certificates
- api.example.com: 24일 남음 ⚠️
- app.example.com: 6일 남음 🚨
```

### 3. 파일 저장
- Write 도구로 `--output` 경로에 Markdown 저장
- 디렉토리 없으면 생성

## Claude Code Integration
- Read로 servers.yaml 로드
- Task 서브에이전트로 병렬 데이터 수집
- Bash로 SSH 명령 실행
- Write로 보고서 파일 생성
