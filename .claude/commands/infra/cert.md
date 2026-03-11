---
allowed-tools: [Read, Bash, Task]
description: "SSL 인증서 만료일 확인 및 갱신 관리"
---

# /infra:cert - SSL 인증서 관리

## Purpose
전체 서버의 SSL 인증서 만료일을 확인하고, 만료 임박 시 갱신을 실행한다.

## Usage
```
/infra:cert [--check|--renew] [--target group|server] [--domain domain]
```

## Arguments
- `--check` - 인증서 만료일 확인 (기본)
- `--renew` - 만료 임박 인증서 갱신
- `--target` - 대상 서버/그룹. 기본값: active
- `--domain` - 특정 도메인만 확인

## Execution

### --check (인증서 확인)

**서버 내부 인증서 확인:**
```bash
ssh user@host "
  # Let's Encrypt 인증서
  for cert in /etc/letsencrypt/live/*/cert.pem; do
    domain=\$(basename \$(dirname \$cert));
    expiry=\$(openssl x509 -in \$cert -enddate -noout | cut -d= -f2);
    days=\$(( ( \$(date -d \"\$expiry\" +%s) - \$(date +%s) ) / 86400 ));
    echo \"\$domain|\$expiry|\$days\";
  done 2>/dev/null;

  # 기타 인증서 (/etc/ssl/)
  find /etc/ssl/certs /etc/nginx/ssl -name '*.pem' -o -name '*.crt' 2>/dev/null | while read cert; do
    expiry=\$(openssl x509 -in \$cert -enddate -noout 2>/dev/null | cut -d= -f2);
    if [ -n \"\$expiry\" ]; then
      days=\$(( ( \$(date -d \"\$expiry\" +%s) - \$(date +%s) ) / 86400 ));
      echo \"\$cert|\$expiry|\$days\";
    fi
  done;
"
```

**외부 도메인 확인 (서버 불필요):**
```bash
echo | openssl s_client -connect domain:443 -servername domain 2>/dev/null | openssl x509 -noout -enddate -subject
```

**출력:**
```
╔════════════════════════════════════════════════════════╗
║  CERT CHECK                          2026-02-19       ║
╠════════════════════════════════════════════════════════╣
║  SERVER  │ DOMAIN          │ EXPIRES    │ DAYS │ STATUS║
╠═════════╪═════════════════╪════════════╪══════╪═══════╣
║  main1   │ example.com     │ 2026-05-20 │  90  │  ✅  ║
║  main1   │ api.example.com │ 2026-03-15 │  24  │  ⚠️  ║
║  main2   │ app.example.com │ 2026-02-25 │   6  │  🚨  ║
╠════════════════════════════════════════════════════════╣
║  ✅ 1 정상  ⚠️ 1 경고(<30일)  🚨 1 긴급(<7일)          ║
╚════════════════════════════════════════════════════════╝
```

### --renew (인증서 갱신)

**절차:**
1. 만료 30일 이내 인증서 식별
2. 갱신 방법 확인:
   - Let's Encrypt: `certbot renew --cert-name [domain]`
   - 기타: 수동 갱신 안내
3. 갱신 실행:
```bash
ssh user@host "sudo certbot renew --cert-name [domain] --non-interactive"
```
4. 갱신 결과 확인:
```bash
ssh user@host "sudo certbot certificates --cert-name [domain]"
```
5. 웹서버 리로드:
```bash
ssh user@host "sudo nginx -t && sudo systemctl reload nginx"
```

**상태 기준:**
- ✅ 정상: >30일
- ⚠️ 경고: 7~30일
- 🚨 긴급: <7일

## Claude Code Integration
- Read로 servers.yaml 로드
- Bash로 SSH 인증서 확인/갱신
- Task 서브에이전트로 멀티서버 병렬 확인
