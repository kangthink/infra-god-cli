#!/bin/bash
# cron-report.sh — 일일/주간 보고서 생성 후 Slack 발송
#
# crontab 등록 예시:
#   0 8 * * *   /path/to/infra-god-cli/scripts/cron-report.sh daily
#   0 9 * * 1   /path/to/infra-god-cli/scripts/cron-report.sh weekly
#
# 필수 환경변수:
#   SLACK_WEBHOOK_URL

set -euo pipefail

REPORT_TYPE="${1:-daily}"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPORT_DIR="${PROJECT_DIR}/reports"
TIMESTAMP=$(date '+%Y-%m-%d')
REPORT_FILE="${REPORT_DIR}/${TIMESTAMP}-${REPORT_TYPE}.md"

mkdir -p "$REPORT_DIR"
source "${PROJECT_DIR}/scripts/slack-notify.sh"

cd "$PROJECT_DIR"

claude -p "servers.yaml을 읽고 ${REPORT_TYPE} 인프라 보고서를 생성해.
active 그룹 모든 서버에 SSH로 접속해서 CPU, RAM, Disk, GPU 상태를 수집하고,
이상 항목과 주요 이슈를 정리해서 Markdown 보고서를 ${REPORT_FILE}에 저장해.
마지막에 요약을 터미널에도 출력해." \
  --allowedTools "Read,Bash(ssh *),Task,Write" \
  --output-format json \
  --max-turns 25 \
  --max-budget-usd 2.00 \
  > "${REPORT_DIR}/${TIMESTAMP}-${REPORT_TYPE}-raw.json" 2>&1

# 보고서가 생성되었으면 Slack 알림
if [ -f "$REPORT_FILE" ]; then
  # 보고서에서 요약 추출 (처음 5줄)
  SUMMARY=$(head -10 "$REPORT_FILE" | tr '\n' ' ' | cut -c1-200)
  slack_report "$REPORT_TYPE" "$SUMMARY"
else
  slack_warning "보고서" "${REPORT_TYPE} 보고서 생성 실패\n로그: ${REPORT_DIR}/${TIMESTAMP}-${REPORT_TYPE}-raw.json"
fi

# 30일 이상 된 보고서 정리
find "$REPORT_DIR" -name "*.md" -mtime +30 -delete 2>/dev/null || true
find "$REPORT_DIR" -name "*-raw.json" -mtime +7 -delete 2>/dev/null || true

echo "[$(date)] ${REPORT_TYPE} 보고서 완료: ${REPORT_FILE}"
