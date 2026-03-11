---
allowed-tools: [Read, Bash]
description: "특정 서버의 하드웨어/소프트웨어/네트워크/사용자 정보를 심층 수집"
---

# /infra:inspect - 서버 심층 분석

## Purpose
infra-god CLI를 사용하여 특정 서버의 전체 정보를 수집하여 상세 보고서를 출력한다.

## Usage
```
/infra:inspect [server] [--section hw|gpu|docker|storage|network|software|services]
```

## Arguments
- `server` - 대상 서버명 (필수)
- `--section` - 특정 섹션만 조회 (플래그). 미지정 시 전체 조회

## Execution

### 1. CLI 실행
```bash
# 전체 inspect
./infra-god inspect gpu-1

# 하드웨어만
./infra-god inspect gpu-1 --hw

# GPU 상세 (프로세스, CUDA, PCIe 포함)
./infra-god inspect gpu-1 --gpu

# Docker 컨테이너
./infra-god inspect gpu-1 --docker

# 스토리지 (마운트별 폴더 사용량)
./infra-god inspect gpu-1 --storage

# 네트워크
./infra-god inspect gpu-1 --network

# 소프트웨어 버전
./infra-god inspect gpu-1 --software

# systemd 서비스
./infra-god inspect gpu-1 --services
```

### 2. 출력 형식
CLI가 섹션별로 포맷팅하여 출력:
```
═══ INSPECT: gpu-1 (10.0.1.10)  role: machine ═══

── HARDWARE ──

  CPU
    Model name: AMD Ryzen 9 5950X 16-Core Processor
    CPU(s): 32
    ...

  GPU
    [0] NVIDIA RTX 3090
        VRAM: 8192 / 24576  util: 25%  temp: 45°C
        Driver: 535.288.01  PCIe: gen4 x16

  Memory
    total: 64Gi  used: 12Gi  free: 48Gi

  Storage
    /dev/sda1  500G  120G  380G  24%

── GPU ──
  [0] NVIDIA RTX 3090
      VRAM: 8192 / 24576  util: 25%  temp: 45°C
      Driver: 535.288.01  PCIe: gen4 x16

  Processes
    PID      GPU MEM      COMMAND
    4521     8192 MiB     .../python3

  CUDA: V12.2.140

── DOCKER ──
  NAME                STATUS              IMAGE                RESTART
  nginx               running (healthy)   nginx:latest         always
  postgres            running             postgres:15          always
```

### 3. 추가 조사가 필요한 경우
inspect 결과를 바탕으로:
- GPU 이상 → `./infra-god exec "dmesg | grep -i nvidia" server`
- 프로세스 상세 → `./infra-god ps server`
- 사용자 정보 → `./infra-god users server`

## 환경변수
- `INFRA_SSH_PASS` - SSH 비밀번호 (password auth 사용 시)
