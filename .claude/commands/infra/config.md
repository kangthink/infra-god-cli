---
allowed-tools: [Read, Bash]
description: "서버 인벤토리 설정(servers.yaml) 관리"
---

# /infra:config - 서버 설정 관리

## Purpose
infra-god CLI를 사용하여 servers.yaml의 서버 인벤토리를 관리한다.

## Usage
```
/infra:config [operation] [arguments]
```

## Operations

### list - 서버 목록 조회
```bash
./infra-god config list
```
서버 목록과 그룹 정보를 테이블로 표시.

### add - 서버 추가
```bash
./infra-god config add worker-3 --host 10.0.1.30 --role worker
./infra-god config add gpu-2 --host 10.0.1.50 --wireless 10.0.2.50 --role machine --auth key --key ~/.ssh/gpu2
```

### edit - 서버 수정
```bash
./infra-god config edit gpu-1 --host 10.0.1.51
./infra-god config edit worker-1 --user deploy --auth key --key ~/.ssh/worker1
./infra-god config edit gpu-2 --status stopped
```

### remove - 서버 제거
```bash
./infra-god config remove worker-3
```
servers.yaml에서 서버를 제거하고 그룹 목록에서도 삭제.

### test - 연결 테스트
```bash
# 전체 서버 연결 테스트
./infra-god config test

# 특정 서버
./infra-god config test gpu-1

# 그룹
./infra-god config test --group workers
```

## 출력 형식
```
═══ SERVER INVENTORY ═══
SERVER    IP                  ROLE     STATUS   SSH USER  AUTH
web-1     10.0.1.1            main     active   deploy    password
gpu-1     10.0.1.10 / 10.0.2.10  machine  active   deploy    password
gpu-2     -                   machine  stopped  deploy    password

── GROUPS ──
  all          web-1, gpu-1, gpu-2
  mains        web-1
  machines     gpu-1, gpu-2
  active       web-1, gpu-1
```
