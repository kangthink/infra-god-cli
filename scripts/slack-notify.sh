#!/bin/bash
# slack-notify.sh — infra-god Slack 알림 헬퍼
# 사용법: source scripts/slack-notify.sh

SLACK_WEBHOOK_URL="${SLACK_WEBHOOK_URL:?SLACK_WEBHOOK_URL 환경변수를 설정하세요}"

# ─── 공통 전송 함수 (재시도 포함) ──────────────────────────
_slack_send() {
  local payload="$1"
  local max_attempts=3
  local attempt=1

  while [ $attempt -le $max_attempts ]; do
    http_code=$(curl -s -o /dev/null -w "%{http_code}" \
      -X POST -H 'Content-type: application/json' \
      --data "$payload" \
      "$SLACK_WEBHOOK_URL")

    if [ "$http_code" = "200" ]; then
      return 0
    elif [ "$http_code" = "429" ]; then
      sleep $((attempt * 2))
      ((attempt++))
    else
      echo "[slack] 전송 실패: HTTP $http_code" >&2
      return 1
    fi
  done
  echo "[slack] $max_attempts회 재시도 후 실패" >&2
  return 1
}

# ─── 서버 상태 요약 알림 ───────────────────────────────────
# 사용: slack_status "main1:✅:23%:45%:67%" "machine4:⚠️:82%:88%:91%"
slack_status() {
  local timestamp
  timestamp=$(date '+%Y-%m-%d %H:%M')
  local fields=""

  for entry in "$@"; do
    IFS=':' read -r name status cpu ram disk <<< "$entry"
    fields="${fields}{\"type\":\"mrkdwn\",\"text\":\"*${name}* ${status}\nCPU:${cpu} RAM:${ram} Disk:${disk}\"},"
  done
  fields="${fields%,}"  # 마지막 쉼표 제거

  _slack_send "{
    \"blocks\": [
      {\"type\":\"header\",\"text\":{\"type\":\"plain_text\",\"text\":\"infra-god 서버 상태\"}},
      {\"type\":\"context\",\"elements\":[{\"type\":\"mrkdwn\",\"text\":\"${timestamp}\"}]},
      {\"type\":\"divider\"},
      {\"type\":\"section\",\"fields\":[${fields}]}
    ]
  }"
}

# ─── 경고 알림 ─────────────────────────────────────────────
# 사용: slack_warning "machine4" "CPU 82% — python3 ML 학습 프로세스"
slack_warning() {
  local server="$1"
  local message="$2"

  _slack_send "{
    \"blocks\": [
      {\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"⚠️ *경고: ${server}*\"}},
      {\"type\":\"section\",\"fields\":[
        {\"type\":\"mrkdwn\",\"text\":\"*서버*\n${server}\"},
        {\"type\":\"mrkdwn\",\"text\":\"*시각*\n$(date '+%H:%M:%S')\"}
      ]},
      {\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"${message}\"}}
    ],
    \"attachments\":[{\"color\":\"#FFA500\",\"text\":\"주의 필요\"}]
  }"
}

# ─── 긴급 알림 ─────────────────────────────────────────────
# 사용: slack_critical "worker3" "SSH 접속 불가 — 네트워크 확인 필요"
slack_critical() {
  local server="$1"
  local message="$2"

  _slack_send "{
    \"blocks\": [
      {\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"🚨 *긴급: ${server}*\"}},
      {\"type\":\"section\",\"fields\":[
        {\"type\":\"mrkdwn\",\"text\":\"*서버*\n${server}\"},
        {\"type\":\"mrkdwn\",\"text\":\"*시각*\n$(date '+%H:%M:%S')\"}
      ]},
      {\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"${message}\"}}
    ],
    \"attachments\":[{\"color\":\"#FF0000\",\"text\":\"즉시 조치 필요\"}]
  }"
}

# ─── 정상 복구 알림 ────────────────────────────────────────
# 사용: slack_resolved "machine4" "디스크 정리 완료: 88% → 67%"
slack_resolved() {
  local server="$1"
  local message="$2"

  _slack_send "{
    \"blocks\": [
      {\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"✅ *복구 완료: ${server}*\"}},
      {\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"${message}\"}}
    ],
    \"attachments\":[{\"color\":\"#36a64f\",\"text\":\"정상 복구됨\"}]
  }"
}

# ─── 일일/주간 보고서 알림 ─────────────────────────────────
# 사용: slack_report "daily" "9/10 정상" "machine4: CPU 82%" "worker3: 접속불가"
slack_report() {
  local type="$1"
  local summary="$2"
  shift 2
  local issues=""
  for issue in "$@"; do
    issues="${issues}• ${issue}\n"
  done
  [ -z "$issues" ] && issues="없음"

  local title="일일 인프라 보고서"
  [ "$type" = "weekly" ] && title="주간 인프라 보고서"

  _slack_send "{
    \"blocks\": [
      {\"type\":\"header\",\"text\":{\"type\":\"plain_text\",\"text\":\"📊 ${title}\"}},
      {\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"*날짜:* $(date '+%Y-%m-%d') | *요약:* ${summary}\"}},
      {\"type\":\"divider\"},
      {\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"*주요 이슈:*\n${issues}\"}}
    ]
  }"
}
