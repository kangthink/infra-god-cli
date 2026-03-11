#!/bin/bash
# cron-cert.sh — SSL 인증서 만료일 확인 후 Slack 알림
#
# crontab 등록 예시:
#   0 9 * * * /path/to/infra-god-cli/scripts/cron-cert.sh
#
# 필수 환경변수:
#   SLACK_WEBHOOK_URL

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
LOG_DIR="${PROJECT_DIR}/logs"
TIMESTAMP=$(date '+%Y%m%d')
LOG_FILE="${LOG_DIR}/cert-${TIMESTAMP}.json"

mkdir -p "$LOG_DIR"
source "${PROJECT_DIR}/scripts/slack-notify.sh"

cd "$PROJECT_DIR"

claude -p "servers.yaml을 읽고 active 그룹 서버들의 SSL 인증서 만료일을 확인해.
각 서버에 SSH로 접속해서 /etc/letsencrypt/live/ 하위의 cert.pem과 /etc/ssl/ 하위 인증서를 openssl로 확인해.
만료일, 남은 일수, 도메인명을 JSON으로 출력해.
30일 이내 만료는 warning, 7일 이내는 critical로 표시해." \
  --allowedTools "Read,Bash(ssh *),Task" \
  --output-format json \
  --max-turns 15 \
  --max-budget-usd 0.50 \
  > "$LOG_FILE" 2>&1

RESULT=$(jq -r '.result // empty' "$LOG_FILE" 2>/dev/null)

if echo "$RESULT" | grep -qi "critical"; then
  slack_critical "SSL 인증서" "7일 이내 만료 인증서 발견\n\`\`\`\n$(echo "$RESULT" | head -30)\n\`\`\`"
elif echo "$RESULT" | grep -qi "warning"; then
  slack_warning "SSL 인증서" "30일 이내 만료 인증서 발견\n\`\`\`\n$(echo "$RESULT" | head -30)\n\`\`\`"
fi

echo "[$(date)] 인증서 확인 완료: ${LOG_FILE}"
