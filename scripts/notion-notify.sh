#!/bin/bash
# notion-notify.sh — infra-god Notion API 알림 헬퍼
# 사용법: source scripts/notion-notify.sh
#
# 필수 환경변수:
#   NOTION_API_KEY   — Notion Internal Integration Secret
#   NOTION_DB_ID     — 서버 상태 데이터베이스 ID

NOTION_API_KEY="${NOTION_API_KEY:?NOTION_API_KEY 환경변수를 설정하세요}"
NOTION_DB_ID="${NOTION_DB_ID:?NOTION_DB_ID 환경변수를 설정하세요}"
NOTION_VERSION="2022-06-28"
NOTION_API="https://api.notion.com/v1"

# ─── 공통 전송 함수 (재시도 포함) ──────────────────────────
_notion_request() {
  local method="$1"
  local url="$2"
  local payload="$3"
  local max_attempts=3
  local attempt=1

  while [ $attempt -le $max_attempts ]; do
    response=$(curl -s -w "\n%{http_code}" \
      -X "$method" "$url" \
      -H "Authorization: Bearer $NOTION_API_KEY" \
      -H "Content-Type: application/json" \
      -H "Notion-Version: $NOTION_VERSION" \
      -d "$payload")

    http_code=$(echo "$response" | tail -1)
    body=$(echo "$response" | head -n -1)

    if [ "$http_code" = "200" ]; then
      echo "$body"
      return 0
    elif [ "$http_code" = "429" ]; then
      local wait=$((attempt * 3))
      echo "[notion] Rate limited, ${wait}s 대기..." >&2
      sleep "$wait"
      ((attempt++))
    else
      echo "[notion] 요청 실패: HTTP $http_code" >&2
      echo "$body" >&2
      return 1
    fi
  done
  echo "[notion] $max_attempts회 재시도 후 실패" >&2
  return 1
}

# ─── 서버 상태 행 추가 ─────────────────────────────────────
# 사용: notion_add_status "main1" "✅정상" 23 45 67 "" ""
#       notion_add_status "machine4" "⚠️경고" 82 88 91 "95" "CPU 과부하"
notion_add_status() {
  local server="$1"
  local status="$2"
  local cpu="$3"
  local ram="$4"
  local disk="$5"
  local gpu="${6:-}"
  local issues="${7:-}"
  local timestamp
  timestamp=$(date -u '+%Y-%m-%dT%H:%M:%S.000Z')

  # GPU 필드 (값이 있으면 추가)
  local gpu_json=""
  if [ -n "$gpu" ] && [ "$gpu" != "null" ] && [ "$gpu" != "" ]; then
    gpu_json='"GPU": {"number": '"$gpu"'},'
  fi

  # Issues 필드
  local issues_json=""
  if [ -n "$issues" ]; then
    issues_json='"Issues": {"rich_text": [{"type": "text", "text": {"content": "'"$issues"'"}}]},'
  fi

  _notion_request POST "${NOTION_API}/pages" '{
    "parent": {"database_id": "'"$NOTION_DB_ID"'"},
    "properties": {
      "Server": {"title": [{"type": "text", "text": {"content": "'"$server"'"}}]},
      "Status": {"select": {"name": "'"$status"'"}},
      "CPU": {"number": '"$cpu"'},
      "RAM": {"number": '"$ram"'},
      "Disk": {"number": '"$disk"'},
      '"$gpu_json"'
      '"$issues_json"'
      "Checked At": {"date": {"start": "'"$timestamp"'"}}
    }
  }'
}

# ─── 기존 서버 행 찾기 ─────────────────────────────────────
# 사용: page_id=$(notion_find_server "main1")
notion_find_server() {
  local server="$1"

  result=$(_notion_request POST "${NOTION_API}/databases/${NOTION_DB_ID}/query" '{
    "filter": {
      "property": "Server",
      "rich_text": {"equals": "'"$server"'"}
    },
    "sorts": [{"property": "Checked At", "direction": "descending"}],
    "page_size": 1
  }')

  echo "$result" | python3 -c "
import sys, json
data = json.load(sys.stdin)
results = data.get('results', [])
if results:
    print(results[0]['id'])
" 2>/dev/null
}

# ─── 기존 서버 행 업데이트 ─────────────────────────────────
# 사용: notion_update_status "page-id" "✅정상" 23 45 67 "" ""
notion_update_status() {
  local page_id="$1"
  local status="$2"
  local cpu="$3"
  local ram="$4"
  local disk="$5"
  local gpu="${6:-}"
  local issues="${7:-}"
  local timestamp
  timestamp=$(date -u '+%Y-%m-%dT%H:%M:%S.000Z')

  local gpu_json=""
  if [ -n "$gpu" ] && [ "$gpu" != "null" ] && [ "$gpu" != "" ]; then
    gpu_json='"GPU": {"number": '"$gpu"'},'
  fi

  local issues_json=""
  if [ -n "$issues" ]; then
    issues_json='"Issues": {"rich_text": [{"type": "text", "text": {"content": "'"$issues"'"}}]},'
  fi

  _notion_request PATCH "${NOTION_API}/pages/${page_id}" '{
    "properties": {
      "Status": {"select": {"name": "'"$status"'"}},
      "CPU": {"number": '"$cpu"'},
      "RAM": {"number": '"$ram"'},
      "Disk": {"number": '"$disk"'},
      '"$gpu_json"'
      '"$issues_json"'
      "Checked At": {"date": {"start": "'"$timestamp"'"}}
    }
  }'
}

# ─── 서버 상태 upsert (있으면 업데이트, 없으면 추가) ──────
# 사용: notion_upsert_status "main1" "✅정상" 23 45 67 "" ""
notion_upsert_status() {
  local server="$1"
  local page_id
  page_id=$(notion_find_server "$server")

  if [ -n "$page_id" ]; then
    notion_update_status "$page_id" "${@:2}" > /dev/null
    echo "[notion] 업데이트: $server"
  else
    notion_add_status "$@" > /dev/null
    echo "[notion] 신규 추가: $server"
  fi
}

# ─── 전체 상태 일괄 기록 ───────────────────────────────────
# 사용: notion_bulk_status \
#         "main1|✅정상|23|45|67||" \
#         "machine4|⚠️경고|82|88|91|95|CPU 과부하"
notion_bulk_status() {
  for entry in "$@"; do
    IFS='|' read -r server status cpu ram disk gpu issues <<< "$entry"
    notion_upsert_status "$server" "$status" "$cpu" "$ram" "$disk" "$gpu" "$issues"
    sleep 0.4  # rate limit 보호 (3 req/sec)
  done
}
