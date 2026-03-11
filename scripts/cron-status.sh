#!/bin/bash
# cron-status.sh — claude -p 로 서버 상태 확인 후 Notion/Slack 알림
#
# crontab 등록 예시:
#   */30 * * * * /path/to/infra-god-cli/scripts/cron-status.sh
#
# 필수 환경변수 (하나 이상):
#   NOTION_API_KEY + NOTION_DB_ID — Notion 데이터베이스에 기록
#   SLACK_WEBHOOK_URL             — Slack 채널에 알림 (선택)

set -euo pipefail

# ─── 설정 ──────────────────────────────────────────────────
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
LOG_DIR="${PROJECT_DIR}/logs"
TIMESTAMP=$(date '+%Y%m%d_%H%M')
LOG_FILE="${LOG_DIR}/status-${TIMESTAMP}.json"

mkdir -p "$LOG_DIR"

# ─── 알림 헬퍼 로드 ───────────────────────────────────────
NOTIFY_NOTION=false
NOTIFY_SLACK=false

if [ -n "${NOTION_API_KEY:-}" ] && [ -n "${NOTION_DB_ID:-}" ]; then
  source "${PROJECT_DIR}/scripts/notion-notify.sh"
  NOTIFY_NOTION=true
fi

if [ -n "${SLACK_WEBHOOK_URL:-}" ]; then
  source "${PROJECT_DIR}/scripts/slack-notify.sh"
  NOTIFY_SLACK=true
fi

if [ "$NOTIFY_NOTION" = false ] && [ "$NOTIFY_SLACK" = false ]; then
  echo "⚠️  알림 대상이 없습니다. NOTION_API_KEY+NOTION_DB_ID 또는 SLACK_WEBHOOK_URL을 설정하세요."
  echo "   Notion: export NOTION_API_KEY='secret_...' && export NOTION_DB_ID='...'"
  echo "   Slack:  export SLACK_WEBHOOK_URL='https://hooks.slack.com/services/...'"
  exit 1
fi

# ─── Claude 로 서버 상태 수집 ──────────────────────────────
cd "$PROJECT_DIR"

claude -p "servers.yaml을 읽고 active 그룹의 모든 서버에 SSH로 접속해서 상태를 확인해.
각 서버별로 다음 정보를 수집해:
- CPU 사용률 (%)
- RAM 사용률 (%)
- Disk 사용률 (%)
- GPU 사용률 (있으면)
- 접속 가능 여부

결과를 JSON으로 출력해. 형식:
{
  \"timestamp\": \"ISO8601\",
  \"servers\": {
    \"서버명\": {
      \"status\": \"ok|warning|critical|unreachable\",
      \"cpu\": 숫자,
      \"ram\": 숫자,
      \"disk\": 숫자,
      \"gpu\": 숫자 또는 null,
      \"issues\": [\"이슈 설명\"]
    }
  },
  \"summary\": {
    \"total\": 숫자,
    \"ok\": 숫자,
    \"warning\": 숫자,
    \"critical\": 숫자,
    \"unreachable\": 숫자
  }
}" \
  --allowedTools "Read,Bash(ssh *),Task" \
  --output-format json \
  --max-turns 20 \
  --max-budget-usd 1.00 \
  > "$LOG_FILE" 2>&1

# ─── 결과 파싱 ────────────────────────────────────────────
RESULT=$(jq -r '.result // empty' "$LOG_FILE" 2>/dev/null)

if [ -z "$RESULT" ]; then
  [ "$NOTIFY_SLACK" = true ] && slack_critical "cron-status" "상태 수집 실패\n로그: ${LOG_FILE}"
  echo "[$(date)] 상태 수집 실패" >&2
  exit 1
fi

# ─── Notion에 기록 ─────────────────────────────────────────
if [ "$NOTIFY_NOTION" = true ]; then
  # JSON에서 서버별 데이터 추출하여 Notion DB에 기록
  echo "$RESULT" | python3 -c "
import sys, json

try:
    # claude 결과에서 JSON 부분 추출
    text = sys.stdin.read()
    # JSON 블록 찾기
    start = text.find('{')
    end = text.rfind('}') + 1
    if start >= 0 and end > start:
        data = json.loads(text[start:end])
        servers = data.get('servers', {})
        for name, info in servers.items():
            status_map = {'ok': '✅정상', 'warning': '⚠️경고', 'critical': '❌위험', 'unreachable': '❌접속불가'}
            status = status_map.get(info.get('status', 'ok'), '✅정상')
            cpu = info.get('cpu', 0)
            ram = info.get('ram', 0)
            disk = info.get('disk', 0)
            gpu = info.get('gpu', '')
            gpu = str(gpu) if gpu and gpu != 'null' else ''
            issues = ', '.join(info.get('issues', []))
            print(f'{name}|{status}|{cpu}|{ram}|{disk}|{gpu}|{issues}')
except Exception as e:
    print(f'PARSE_ERROR|{e}', file=sys.stderr)
" | while IFS='|' read -r server status cpu ram disk gpu issues; do
    if [ "$server" != "PARSE_ERROR" ]; then
      notion_upsert_status "$server" "$status" "$cpu" "$ram" "$disk" "$gpu" "$issues"
    fi
  done

  echo "[$(date)] Notion DB 기록 완료"
fi

# ─── Slack 알림 (이상 시에만) ──────────────────────────────
if [ "$NOTIFY_SLACK" = true ]; then
  HAS_ISSUES=false
  echo "$RESULT" | grep -qi '"status"[[:space:]]*:[[:space:]]*"warning"' && HAS_ISSUES=true
  echo "$RESULT" | grep -qi '"status"[[:space:]]*:[[:space:]]*"critical"' && HAS_ISSUES=true
  echo "$RESULT" | grep -qi '"status"[[:space:]]*:[[:space:]]*"unreachable"' && HAS_ISSUES=true

  if [ "$HAS_ISSUES" = true ]; then
    slack_warning "infra-god" "서버 이상 감지됨\n상세 로그: \`${LOG_FILE}\`\n\n\`\`\`\n$(echo "$RESULT" | head -50)\n\`\`\`"
  fi
fi

# ─── 오래된 로그 정리 (7일 이상) ──────────────────────────
find "$LOG_DIR" -name "status-*.json" -mtime +7 -delete 2>/dev/null || true

echo "[$(date)] 상태 확인 완료: ${LOG_FILE}"
