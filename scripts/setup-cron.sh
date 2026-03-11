#!/bin/bash
# setup-cron.sh — crontab에 infra-god 작업 등록
#
# 사용법:
#   1. 환경변수 설정 (~/.zshrc 또는 ~/.bashrc)
#
#      # Notion (권장)
#      export NOTION_API_KEY="secret_..."
#      export NOTION_DB_ID="a1b2c3d4..."
#
#      # Slack (선택, 즉시 알림용)
#      export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/..."
#
#   2. 이 스크립트 실행
#      ./scripts/setup-cron.sh
#
#   3. 확인
#      crontab -l

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# ─── 환경변수 확인 ─────────────────────────────────────────
HAS_NOTION=false
HAS_SLACK=false
ENV_LINES=""

if [ -n "${NOTION_API_KEY:-}" ] && [ -n "${NOTION_DB_ID:-}" ]; then
  HAS_NOTION=true
  ENV_LINES="${ENV_LINES}NOTION_API_KEY=${NOTION_API_KEY}
NOTION_DB_ID=${NOTION_DB_ID}
"
  echo "✅ Notion 설정 감지됨"
fi

if [ -n "${SLACK_WEBHOOK_URL:-}" ]; then
  HAS_SLACK=true
  ENV_LINES="${ENV_LINES}SLACK_WEBHOOK_URL=${SLACK_WEBHOOK_URL}
"
  echo "✅ Slack 설정 감지됨"
fi

if [ "$HAS_NOTION" = false ] && [ "$HAS_SLACK" = false ]; then
  echo ""
  echo "⚠️  알림 대상이 설정되지 않았습니다."
  echo ""
  echo "하나 이상 설정해주세요:"
  echo ""
  echo "  [Notion — 권장: 데이터 누적 및 대시보드]"
  echo "  1. https://notion.so/profile/integrations → New integration"
  echo "  2. 워크스페이스에 DB 생성 (Server, Status, CPU, RAM, Disk, GPU, Issues, Checked At)"
  echo "  3. DB 페이지 → ... → Add connections → 생성한 integration 연결"
  echo "  4. 환경변수 등록:"
  echo '     export NOTION_API_KEY="secret_..."'
  echo '     export NOTION_DB_ID="DB페이지URL에서_32자_hex_ID"'
  echo ""
  echo "  [Slack — 선택: 이상 시 즉시 알림]"
  echo "  1. https://api.slack.com/apps → Create New App → Incoming Webhooks"
  echo "  2. 환경변수 등록:"
  echo '     export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/..."'
  exit 1
fi

echo ""
echo "현재 등록된 crontab:"
echo "─────────────────────"
crontab -l 2>/dev/null || echo "(비어있음)"
echo ""

# ─── cron 항목 생성 ────────────────────────────────────────
CRON_ENTRIES="
# ─── infra-god 자동 모니터링 ───────────────────────────────
# 환경변수
${ENV_LINES}PATH=/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin

# 매 30분: 서버 상태 확인 → Notion DB 기록 + 이상 시 Slack 알림
*/30 * * * * ${SCRIPT_DIR}/cron-status.sh >> ${SCRIPT_DIR}/../logs/cron.log 2>&1

# 매일 09:00: SSL 인증서 만료 확인
0 9 * * * ${SCRIPT_DIR}/cron-cert.sh >> ${SCRIPT_DIR}/../logs/cron.log 2>&1

# 매일 08:00: 일일 보고서
0 8 * * * ${SCRIPT_DIR}/cron-report.sh daily >> ${SCRIPT_DIR}/../logs/cron.log 2>&1

# 매주 월요일 09:00: 주간 보고서
0 9 * * 1 ${SCRIPT_DIR}/cron-report.sh weekly >> ${SCRIPT_DIR}/../logs/cron.log 2>&1
# ─── /infra-god ────────────────────────────────────────────
"

echo "등록할 cron 항목:"
echo "─────────────────────"
echo "$CRON_ENTRIES"
echo ""
echo "알림 대상:"
[ "$HAS_NOTION" = true ] && echo "  📝 Notion DB — 매 실행마다 상태 기록"
[ "$HAS_SLACK" = true ] && echo "  💬 Slack — 이상 감지 시에만 알림"
echo ""

read -p "crontab에 등록하시겠습니까? (y/N) " -n 1 -r
echo ""

if [[ $REPLY =~ ^[Yy]$ ]]; then
  (crontab -l 2>/dev/null | sed '/# ─── infra-god/,/# ─── \/infra-god/d'; echo "$CRON_ENTRIES") | crontab -
  echo "✅ crontab 등록 완료"
  echo ""
  crontab -l
else
  echo "취소됨"
fi
