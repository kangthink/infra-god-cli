# infra-god-cli

간단한 명령어로 여러 서버를 주기적으로 모니터링하고 관리하는 CLI 도구.

## 모니터링 핵심 지표

| 카테고리 | 지표 | 위험 기준 | 이유 |
|----------|------|-----------|------|
| CPU | 사용률, load average | >80% 지속 5분 | 서비스 응답 지연의 1차 원인 |
| 메모리 | used/available, swap | available <10%, swap 활성화 | OOM killer → 프로세스 강제 종료 |
| 디스크 | 사용률, inode, I/O wait | >90%, inode 고갈 | 로그/DB 쓰기 실패 → 서비스 장애 |
| 네트워크 | 패킷 loss, 연결 수, 대역폭 | loss >1%, TIME_WAIT 폭증 | 외부 통신 장애, 포트 고갈 |
| 프로세스 | 대상 프로세스 생존, 좀비 프로세스 | 프로세스 부재 | 서비스 다운 감지의 기본 |
| 응답 | HTTP status, 응답 시간 | 5xx, >3초 | 사용자 체감 품질 직접 지표 |
| SSL | 인증서 만료일 | <30일 | 만료 시 서비스 접속 불가 |
| 시스템 | uptime, dmesg, 시간 동기화 | 예기치 않은 재부팅 | 하드웨어 장애, 보안 사고 징후 |

## 시나리오

### A. 일상 헬스체크 (매 1~5분)

3대 웹 서버 + 1대 DB 서버 운영 기준. "지금 다 살아있나?"

**체크 항목:**
- 각 서버 SSH 접속 가능 여부
- 핵심 프로세스 생존 (nginx, node, postgres 등)
- HTTP 200 응답 확인 (health endpoint)
- 디스크 사용률 빠른 확인

**출력 예시:**
```
web-1   ✅ cpu:23%  mem:45%  disk:67%  nginx:running  http:200(120ms)
web-2   ✅ cpu:18%  mem:41%  disk:65%  nginx:running  http:200(95ms)
web-3   ⚠️ cpu:82%  mem:78%  disk:91%  nginx:running  http:200(890ms)
db-1    ✅ cpu:35%  mem:60%  disk:72%  postgres:running
```

### B. 디스크 풀 임박 (경고 → 자동 대응)

**감지:** 디스크 91% 도달

**원인 추적:**
- `du -sh /var/log/*` → 로그 파일 비대화 발견
- 최근 7일간 증가율 계산 → 하루 2% 증가
- 예상 포화 시점: 4.5일 후

**자동 대응:**
1. 오래된 로그 로테이션/삭제 (logrotate 강제 실행)
2. 큰 파일 top 10 리스트 → 관리자 알림
3. 임계치 초과 시 Slack/이메일 알림

### C. 프로세스 사망 → 자동 재시작

**감지:** 특정 서버에서 프로세스 없음

**진단:**
- 마지막 로그 확인 (`journalctl -u <service> --since "5min ago"`)
- 포트 충돌 확인 (`lsof -i :<port>`)
- 설정 파일 문법 검사

**자동 대응:**
1. 설정 유효 → `systemctl restart <service>`
2. 재시작 후 헬스체크 재확인
3. 3회 연속 실패 시 → 에스컬레이션 (관리자 호출)

### D. 메모리 누수 추적 (트렌드 분석)

**감지:** 메모리 사용률이 매일 2%씩 증가

**분석:**
- 프로세스별 메모리 사용량 추적 (`ps aux --sort=-%mem`)
- 지난 7일 트렌드
- 특정 프로세스 RSS 증가 패턴 확인

**대응:**
1. 문제 프로세스 식별 → 관리자 보고
2. 메모리 임계치(85%) 도달 시 graceful restart 스케줄
3. 장기: 해당 서비스 메모리 프로파일링 권고

### E. SSL 인증서 만료 관리

**주기:** 매일 1회

**체크:**
- 각 도메인 인증서 만료일 확인
- 30일 이내 만료 → 경고
- 7일 이내 만료 → 긴급 알림

**자동 대응:**
1. Let's Encrypt → `certbot renew` 자동 실행
2. 갱신 결과 확인 및 보고

### F. 배포 후 검증

**트리거:** 배포 완료 신호 수신

**체크 (배포 후 5분간 집중 모니터링):**
- 에러율 변화 (배포 전후 비교)
- 응답 시간 변화
- CPU/메모리 급변 여부
- 새 버전 엔드포인트 정상 응답 확인

**판정:**
- 정상 → "배포 성공" 보고
- 이상 → 자동 롤백 또는 관리자 알림

## CLI 명령어 구조

```bash
infra-god status                              # 전체 서버 빠른 확인
infra-god inspect web-1                       # 특정 서버 상세 정보
infra-god watch --interval 5m                 # 주기적 모니터링 (5분 간격)
infra-god cleanup web-3 --dry-run             # 디스크 정리 (안전 모드)
infra-god restart web-2 nginx                 # 프로세스 재시작
infra-god cert-check --all                    # 인증서 체크
infra-god config add web-4 --host 10.0.1.4    # 서버 추가
```

## 서버 설정 파일

```yaml
# ~/.infra-god/servers.yaml
servers:
  web-1:
    host: 10.0.1.1
    ssh_user: deploy
    checks:
      - type: process
        name: nginx
        restart: true
      - type: http
        url: http://localhost/health
        expect: 200
      - type: disk
        warn: 85%
        critical: 95%
    alerts:
      slack: "#ops-alerts"

  db-1:
    host: 10.0.1.10
    ssh_user: deploy
    checks:
      - type: process
        name: postgres
      - type: disk
        warn: 80%
        critical: 90%
      - type: custom
        command: "pg_isready"
        expect_exit: 0
```

## 경쟁 도구 분석

infra-god-cli가 해결하는 문제 영역에 기존 오픈소스 도구들이 존재한다. 각 도구의 강점과 한계를 파악하여 차별화 방향을 잡는다.

### 병렬 명령 실행

| 도구 | Stars | 언어 | 강점 | 한계 | infra-god 차별점 |
|------|-------|------|------|------|-----------------|
| [Ansible](https://github.com/ansible/ansible) | ~68K | Python | 에이전트리스, 거대 생태계, 인벤토리 그룹 관리 | YAML 학습곡선, 단순 명령에도 무거움 | 설정 최소화, 즉시 실행 가능한 CLI UX |
| [Fabric](https://github.com/fabric/fabric) | ~15K | Python | Pythonic API, 스크립팅 유연성 | 기본 직렬 실행, 대규모 플릿 부적합 | 병렬 실행 기본, 모니터링 통합 |
| [parallel-ssh](https://github.com/ParallelSSH/parallel-ssh) | ~1.3K | Python | libssh2 기반 고성능, 수만 대 동시 실행 | 에러 핸들링 빈약, 모니터링 없음 | 실행 + 모니터링 통합 도구 |
| [gossh](https://github.com/windvalley/gossh) | ~187 | Go | 단일 바이너리, Ansible 대비 10x 빠름, 위험 명령 감지 | 작은 커뮤니티, 모니터링 없음 | 모니터링/알림/자동 대응 통합 |

### 서버 모니터링 (CPU/메모리/디스크/네트워크)

| 도구 | Stars | 멀티서버 | 강점 | 한계 | infra-god 차별점 |
|------|-------|---------|------|------|-----------------|
| [Netdata](https://github.com/netdata/netdata) | ~78K | parent/child 구조 | 초당 메트릭, GPU/Docker 기본 지원, ML 이상탐지 | 웹 대시보드 중심 (CLI 약함), 에이전트 설치 필요 | CLI-first, 에이전트리스 |
| [Glances](https://github.com/nicolargo/glances) | ~32K | client-server | CLI 기반, GPU/Docker 지원, REST API | 한 번에 한 서버만 연결, 통합 뷰 없음 | 전체 서버 한눈에 보기 |
| [btop++](https://github.com/aristocratos/btop) | ~30K | 없음 (SSH) | 가장 미려한 TUI, GPU 지원 | 로컬 전용, 서버마다 SSH 접속 필요 | 원격 멀티서버 통합 대시보드 |

### GPU 모니터링

| 도구 | Stars | 멀티서버 | 강점 | 한계 | infra-god 차별점 |
|------|-------|---------|------|------|-----------------|
| [cluster-smi](https://github.com/PatWie/cluster-smi) | ~81 | 네이티브 | GPU 클러스터 전용 통합 뷰 | 유지보수 중단, GPU 외 지표 없음 | CPU/메모리/Docker와 GPU 통합 모니터링 |
| [GPUStack](https://github.com/gpustack/gpustack) | ~4.5K | 네이티브 | GPU 클러스터 관리 + AI 모델 배포 | AI 추론 서빙 특화, 범용성 부족 | 범용 서버 관리 + GPU |
| [gpustat](https://github.com/wookayin/gpustat) | ~4.3K | SSH 조합 | 한 줄 출력, JSON, 스크립팅 최적 | NVIDIA 전용, 단독으로 멀티서버 불가 | 벤더 무관 GPU 통합 |
| [nvtop](https://github.com/Syllo/nvtop) | ~10K | 없음 | NVIDIA/AMD/Intel 멀티벤더, htop 스타일 | 로컬 전용 | 원격 멀티서버 |

### Docker 모니터링

| 도구 | Stars | 강점 | 한계 | infra-god 차별점 |
|------|-------|------|------|-----------------|
| [LazyDocker](https://github.com/jesseduffield/lazydocker) | ~50K | 뛰어난 TUI, 로그/CPU/메모리 실시간 | 단일 호스트 전용 | 멀티서버 Docker 통합 모니터링 |

### 핵심 차별화 전략

기존 도구들은 **"명령 실행"** 또는 **"모니터링"** 중 하나에 특화되어 있고, 두 기능을 통합한 CLI-first 도구는 부재하다.

1. **통합**: 모니터링 + 명령 실행 + 자동 대응을 하나의 CLI에서
2. **에이전트리스**: SSH만으로 동작, 대상 서버에 설치 불필요
3. **CLI-first**: 웹 대시보드 없이 터미널에서 전체 서버 상태 한눈에 파악
4. **설정 최소화**: `servers.yaml` 하나로 시작, Ansible 수준의 학습곡선 불필요
5. **자동 대응**: 단순 모니터링을 넘어 디스크 정리, 프로세스 재시작 등 자동 대응 내장

## 서버 인벤토리

`servers.yaml`에 정의. `servers.yaml.example` 참조.

**서버 그룹:** `all`, `mains`, `machines`, `workers`, `active`(종료 제외), `gpu`(GPU 장착 서버)

### 서버별 수집 정보

```
머신명
├── 하드웨어
│   ├── CPU (모델, 코어수, 사용률, 온도)
│   ├── GPU (모델, VRAM, 사용률, 온도)
│   ├── RAM (총량, 사용량, available, swap)
│   └── Storage (마운트별 총량/사용량/여유, I/O wait)
├── 소프트웨어
│   ├── OS/커널 (배포판, 커널 버전)
│   ├── 필수응용프로그램 (docker, python, node 등 버전)
│   └── 운용프로그램 (실행 중 서비스, Docker 컨테이너)
├── 사용자/그룹/권한 (로그인 사용자, sudoers)
├── 네트워크 (모든 NIC, IP, listening 포트)
└── 폴더구조/워크스페이스 (경로, 디스크 사용량)
```

## 에이전트 커맨드 스킬

`.claude/commands/infra/` 에 정의된 9개 커맨드. `/infra:[command]` 형식으로 실행.

### 스킬 목록

| 커맨드 | 설명 | 주요 도구 |
|--------|------|----------|
| `/infra:status` | 전체 서버 상태 병렬 조회 및 통합 대시보드 | Read, Bash, Task |
| `/infra:exec` | 여러 서버에 명령어 병렬 실행 | Read, Bash, Task |
| `/infra:config` | servers.yaml 서버 인벤토리 CRUD | Read, Write, Edit |
| `/infra:inspect` | 특정 서버 하드웨어/소프트웨어/네트워크 심층 수집 | Read, Bash, Task |
| `/infra:diagnose` | AI 기반 문제 진단, 근본 원인 분석 | Read, Bash, Task, sequential-thinking |
| `/infra:heal` | 자동 복구 (디스크 정리, 서비스 재시작 등) | Read, Bash, Task |
| `/infra:deploy-check` | 배포 전후 메트릭 비교, 성공/실패 판정 | Read, Bash, Task, Write |
| `/infra:cert` | SSL 인증서 만료일 확인 및 갱신 | Read, Bash, Task |
| `/infra:report` | 일일/주간/전체 인프라 보고서 Markdown 생성 | Read, Bash, Task, Write |

### 내부 동작 원리

모든 스킬은 동일한 파이프라인을 따른다:
```
Read(servers.yaml) → Task(서브에이전트 병렬 생성) → Bash(SSH 명령) → 결과 취합 → 출력/대응
```

- **병렬 처리**: Task 서브에이전트가 서버 수만큼 생성되어 SSH를 동시에 실행
- **IP fallback**: wired IP 우선 시도, 실패 시 wireless IP 사용
- **stopped 서버**: status가 stopped인 서버는 자동 스킵

## 시나리오별 스킬 사용 가이드

### 시나리오 A: 일상 헬스체크

"지금 다 살아있나?" 빠르게 확인할 때.

```bash
# 1단계: 전체 서버 빠른 상태 확인
/infra:status

# 특정 그룹만 확인
/infra:status --target mains
/infra:status --target gpu

# GPU/Docker 포함 상세 확인
/infra:status --detail
```

**이상 발견 시 후속 조치:**
```bash
# 경고 서버 상세 확인
/infra:inspect machine4

# AI로 원인 분석
/infra:diagnose machine4 "CPU 높음"
```

### 시나리오 B: 디스크 풀 임박

status에서 디스크 경고(>85%) 발견 시.

```bash
# 1단계: 정확한 원인 진단
/infra:diagnose machine4 "disk full"
# → 어떤 디렉토리/파일이 원인인지 분석, 증가 추세 계산

# 2단계: 안전 모드로 정리 계획 확인
/infra:heal machine4 disk-cleanup --dry-run
# → 삭제 대상과 확보 가능 용량 표시

# 3단계: 실제 정리 실행
/infra:heal machine4 disk-cleanup
# → 로그 정리, apt 캐시 삭제, tmp 정리 → 사전/사후 비교 출력

# Docker까지 포함한 공격적 정리
/infra:heal machine4 disk-cleanup --aggressive
```

### 시나리오 C: 프로세스 사망 → 자동 재시작

status에서 프로세스 다운 감지 시.

```bash
# 1단계: 원인 진단 (로그, 포트 충돌, 설정 오류)
/infra:diagnose main1 "nginx down"
# → journalctl, lsof, 설정 문법 검사 → 근본 원인 보고

# 2단계: 서비스 재시작
/infra:heal main1 restart-service nginx
# → 사전: 설정 유효성 검사 → 재시작 → 사후: 헬스체크

# Docker 컨테이너 재시작
/infra:heal machine3 restart-docker my-app
```

### 시나리오 D: 메모리 누수 추적

status에서 메모리 높은 서버 발견 시.

```bash
# 1단계: 심층 분석 (프로세스별 메모리, swap, OOM 이력)
/infra:diagnose machine4 "memory leak" --deep
# → sequential-thinking으로 메모리 사용 패턴 분석
# → 문제 프로세스 식별, RSS 증가 추세 계산

# 2단계: 필요 시 프로세스 우선순위 조정
/infra:heal machine4 renice --pid 4521 --priority 10

# 3단계: 최후 수단 — 프로세스 종료
/infra:heal machine4 kill-process --pid 4521 --signal TERM
```

### 시나리오 E: SSL 인증서 만료 관리

정기적 점검 (매일 1회 권장).

```bash
# 1단계: 전체 인증서 만료일 확인
/infra:cert --check
# → 30일 이내 ⚠️, 7일 이내 🚨 표시

# 2단계: 만료 임박 인증서 갱신
/infra:cert --renew --target main1
# → certbot renew → nginx reload → 갱신 결과 확인

# 특정 도메인만 확인
/infra:cert --check --domain api.example.com
```

### 시나리오 F: 배포 후 검증

배포 전후 메트릭을 비교하여 이상 탐지.

```bash
# 1단계: 배포 전 기준선 수집
/infra:deploy-check before --target mains

# 2단계: (배포 실행)

# 3단계: 배포 후 메트릭 수집 및 비교
/infra:deploy-check after --target mains
/infra:deploy-check compare --target mains
# → CPU/메모리/에러율/응답시간 변화 표시 → 성공/실패 판정

# 원스텝: 배포 후 5분간 자동 모니터링
/infra:deploy-check watch --target mains --duration 5m
```

### 일상 운영 워크플로우

```bash
# 아침: 전체 현황 파악
/infra:status --detail

# 전 서버 패키지 업데이트
/infra:exec "apt update && apt upgrade -y" --target active --sudo

# 특정 도구 전체 설치
/infra:exec "pip install torch==2.1.0" --target gpu --sudo

# 주간 보고서 생성
/infra:report --type weekly

# 전체 인벤토리 보고서
/infra:report --type full

# 새 서버 추가
/infra:config add worker4 --host 10.0.1.30 --role worker

# 서버 연결 테스트
/infra:config test worker4
```
