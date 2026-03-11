---
allowed-tools: [Read, Write, Edit, Glob]
description: "서버 인벤토리 설정(servers.yaml) 관리"
---

# /infra:config - 서버 설정 관리

## Purpose
servers.yaml 파일의 서버 인벤토리를 조회, 추가, 수정, 삭제한다.

## Usage
```
/infra:config [operation] [arguments]
```

## Operations

### list - 서버 목록 조회
```
/infra:config list [--group mains|machines|workers|all]
```
servers.yaml에서 서버 목록을 읽어 테이블로 표시:
```
SERVER     │ ROLE     │ WIRED IP        │ WIRELESS IP     │ STATUS
web-1      │ main     │ 10.0.1.1        │ -               │ active
gpu-1      │ machine  │ 10.0.1.10       │ 10.0.2.10       │ active
gpu-2      │ machine  │ -               │ -               │ stopped
```

### add - 서버 추가
```
/infra:config add [name] --host [ip] [--wireless ip] [--role main|machine|worker] [--user deploy]
```
- servers.yaml에 새 서버 엔트리 추가
- 해당 그룹에도 자동 추가

### edit - 서버 수정
```
/infra:config edit [name] [--host ip] [--wireless ip] [--role role] [--status active|stopped]
```
- 기존 서버 정보 수정

### remove - 서버 제거
```
/infra:config remove [name] [--confirm]
```
- servers.yaml에서 서버 제거
- 그룹 목록에서도 제거

### group - 그룹 관리
```
/infra:config group add [group-name] [server1,server2,...]
/infra:config group remove [group-name]
/infra:config group edit [group-name] --add server1 --remove server2
```

### test - 연결 테스트
```
/infra:config test [name|group]
```
- 대상 서버에 SSH 연결 테스트 수행
- wired/wireless 양쪽 테스트

## Execution
1. Read로 servers.yaml 현재 상태 로드
2. 요청된 작업 수행 (조회/추가/수정/삭제)
3. 변경 시 Edit으로 servers.yaml 업데이트
4. 변경 결과 출력

## Claude Code Integration
- Read/Edit로 servers.yaml CRUD
- Write로 신규 설정 파일 생성 (최초 실행 시)
- Glob으로 설정 파일 위치 탐색
