---
allowed-tools: [Read, Bash, Task, Write]
description: "배포 전후 서버 상태를 비교하여 배포 안전성 검증"
---

# /infra:deploy-check - 배포 후 검증

## Purpose
배포 전후의 서버 메트릭을 비교하여 배포 성공/실패를 판정한다.

## Usage
```
/infra:deploy-check [phase] [--target group|server] [--duration 5m] [--threshold strict|normal|loose]
```

## Arguments
- `phase` - 실행 단계: `before` | `after` | `compare` | `watch`
- `--target` - 대상 서버/그룹. 기본값: active
- `--duration` - watch 모드 모니터링 시간. 기본: 5m
- `--threshold` - 판정 기준 엄격도. 기본: normal

## Execution

### Phase 1: before (배포 전 기준선 수집)
```
/infra:deploy-check before --target mains
```
- 대상 서버들의 현재 메트릭을 수집하여 `.infra-god/baseline.json`에 저장:
  - CPU 사용률 (1분 평균)
  - 메모리 사용량
  - 에러율 (최근 5분 로그 기준)
  - HTTP 응답 시간 (health endpoint)
  - 활성 연결 수
  - Docker 컨테이너 상태

### Phase 2: after (배포 후 메트릭 수집)
```
/infra:deploy-check after --target mains
```
- 동일한 메트릭을 다시 수집하여 `.infra-god/after.json`에 저장

### Phase 3: compare (비교 분석)
```
/infra:deploy-check compare --target mains
```
- baseline.json과 after.json을 비교:

```
╔═══════════════════════════════════════════════════════════╗
║  DEPLOY CHECK: mains                                     ║
╠═══════════════════════════════════════════════════════════╣
║  METRIC         │  BEFORE  │  AFTER   │ CHANGE │ STATUS  ║
╠═════════════════╪══════════╪══════════╪════════╪═════════╣
║  CPU (avg)      │  23%     │  28%     │ +5%    │  ✅     ║
║  Memory         │  4.2GB   │  4.5GB   │ +300MB │  ✅     ║
║  Error rate     │  0.1%    │  0.1%    │  0%    │  ✅     ║
║  Response time  │  120ms   │  135ms   │ +12%   │  ✅     ║
║  Connections    │  245     │  260     │ +6%    │  ✅     ║
║  Containers     │  5/5     │  5/5     │  0     │  ✅     ║
╠═══════════════════════════════════════════════════════════╣
║  VERDICT: ✅ 배포 성공 — 모든 지표 정상 범위              ║
╚═══════════════════════════════════════════════════════════╝
```

### Phase 4: watch (실시간 모니터링)
```
/infra:deploy-check watch --target mains --duration 5m
```
- before → 배포 대기 → after를 자동으로 수행
- duration 동안 30초 간격으로 메트릭 수집
- 이상 감지 시 즉시 알림

### 판정 기준 (--threshold)

| 지표 | strict | normal | loose |
|------|--------|--------|-------|
| CPU 증가 | <10% | <20% | <30% |
| Memory 증가 | <5% | <15% | <25% |
| Error rate 증가 | 0% | <0.5% | <1% |
| Response time 증가 | <10% | <25% | <50% |

### 판정 결과
- ✅ **배포 성공**: 모든 지표 정상 범위
- ⚠️ **주의 필요**: 1~2개 지표 경계 근처
- ❌ **배포 실패**: 주요 지표 초과 → 롤백 권고

## Claude Code Integration
- Read로 servers.yaml 및 baseline 데이터 로드
- Bash로 SSH 메트릭 수집
- Task 서브에이전트로 멀티서버 병렬 수집
- Write로 baseline/after 데이터 저장
