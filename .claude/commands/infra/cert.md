---
allowed-tools: [Read, Bash]
description: "SSL 인증서 만료일 확인 및 갱신 관리"
---

# /infra:cert - SSL 인증서 관리

## Purpose
infra-god CLI의 exec 기능을 활용하여 전체 서버의 SSL 인증서 만료일을 확인하고 갱신한다.

## Usage
```
/infra:cert [--check|--renew] [--target group|server] [--domain domain]
```

## Arguments
- `--check` - 인증서 만료일 확인 (기본)
- `--renew` - 만료 임박 인증서 갱신
- `--target` - 대상 서버/그룹. 기본값: 전체 active
- `--domain` - 특정 도메인만 확인

## Execution

### --check (인증서 확인)

**서버 내부 인증서 확인:**
```bash
./infra-god exec 'for cert in /etc/letsencrypt/live/*/cert.pem; do domain=$(basename $(dirname $cert)); expiry=$(openssl x509 -in $cert -enddate -noout | cut -d= -f2); days=$(( ($(date -d "$expiry" +%s) - $(date +%s)) / 86400 )); echo "$domain $expiry ${days}d"; done 2>/dev/null' --sudo
```

**특정 서버만:**
```bash
./infra-god exec 'for cert in /etc/letsencrypt/live/*/cert.pem; do domain=$(basename $(dirname $cert)); expiry=$(openssl x509 -in $cert -enddate -noout | cut -d= -f2); days=$(( ($(date -d "$expiry" +%s) - $(date +%s)) / 86400 )); echo "$domain $expiry ${days}d"; done 2>/dev/null' web-1 --sudo
```

**외부 도메인 직접 확인 (로컬에서):**
```bash
echo | openssl s_client -connect example.com:443 -servername example.com 2>/dev/null | openssl x509 -noout -enddate -subject
```

### --renew (인증서 갱신)

```bash
# certbot 갱신
./infra-god exec "certbot renew --cert-name DOMAIN --non-interactive" web-1 --sudo --yes

# 갱신 확인
./infra-god exec "certbot certificates --cert-name DOMAIN" web-1 --sudo

# 웹서버 리로드
./infra-god exec "nginx -t && systemctl reload nginx" web-1 --sudo --yes
```

### 출력 형식
```
═══ CERT CHECK ═══
SERVER  │ DOMAIN          │ EXPIRES    │ DAYS │ STATUS
web-1   │ example.com     │ 2026-06-20 │  90  │  ✅
web-1   │ api.example.com │ 2026-04-15 │  24  │  ⚠️
web-2   │ app.example.com │ 2026-03-25 │   6  │  🚨

✅ 1 정상  ⚠️ 1 경고(<30일)  🚨 1 긴급(<7일)
```

### 상태 기준
- ✅ 정상: >30일
- ⚠️ 경고: 7~30일
- 🚨 긴급: <7일

## 환경변수
- `INFRA_SSH_PASS` - SSH 비밀번호 (password auth 사용 시)
