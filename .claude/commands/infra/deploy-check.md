---
allowed-tools: [Read, Bash, Write]
description: "배포 전후 서버 상태를 비교하여 배포 안전성 검증"
---

# /infra:deploy-check - 배포 후 검증

## Purpose
infra-god CLI로 배포 전후의 서버 메트릭을 수집하여 비교하고 배포 성공/실패를 판정한다.

## Usage
```
/infra:deploy-check [phase] [--target group|server] [--threshold strict|normal|loose]
```

## Arguments
- `phase` - 실행 단계: `before` | `after` | `compare`
- `--target` - 대상 서버/그룹. 기본값: 전체 active
- `--threshold` - 판정 기준 엄격도. 기본: normal

## Execution

### Phase 1: before (배포 전 기준선 수집)
```bash
# JSON으로 현재 상태 수집
./infra-god status --json > .infra-god-baseline.json

# 특정 그룹만
./infra-god status server1 server2 --json > .infra-god-baseline.json
```
- 결과를 `.infra-god-baseline.json`에 저장

### Phase 2: after (배포 후 메트릭 수집)
```bash
# 동일한 방식으로 수집
./infra-god status --json > .infra-god-after.json

# Docker 상태 추가 확인
./infra-god inspect server-name --docker
```
- 결과를 `.infra-god-after.json`에 저장

### Phase 3: compare (비교 분석)
Read로 baseline과 after JSON을 읽어 비교:

```
═══ DEPLOY CHECK ═══
METRIC         │  BEFORE  │  AFTER   │ CHANGE │ STATUS
CPU (avg)      │  23%     │  28%     │ +5%    │  ✅
Memory (avg)   │  45%     │  48%     │ +3%    │  ✅
Disk (avg)     │  67%     │  68%     │ +1%    │  ✅
Error servers  │  0       │  0       │  0     │  ✅

VERDICT: ✅ 배포 성공 — 모든 지표 정상 범위
```

### 판정 기준 (--threshold)

| 지표 | strict | normal | loose |
|------|--------|--------|-------|
| CPU 증가 | <10% | <20% | <30% |
| Memory 증가 | <5% | <15% | <25% |
| Error 서버 수 | 0 | <=1 | <=2 |

### 판정 결과
- ✅ **배포 성공**: 모든 지표 정상 범위
- ⚠️ **주의 필요**: 1~2개 지표 경계 근처
- ❌ **배포 실패**: 주요 지표 초과 → 롤백 권고

## 환경변수
- `INFRA_SSH_PASS` - SSH 비밀번호 (password auth 사용 시)
