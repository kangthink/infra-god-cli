#!/usr/bin/env bash
# Quick "is the webui running?" check.
# Shows app bundle, login-item registration, process, HTTP health,
# LAN reachability hint, recent stdout/stderr logs.

set -uo pipefail
PORT=9998
APP_NAME="InfraGod"
APP_DIR="${HOME}/Applications/${APP_NAME}.app"
LOG_DIR="${HOME}/Library/Logs/infra-god"

GREEN=$'\033[32m'; RED=$'\033[31m'; YEL=$'\033[33m'; DIM=$'\033[2m'; RST=$'\033[0m'

echo "── app bundle ──────────────────────────────"
if [ -d "${APP_DIR}" ]; then
  printf "  app:    %s%s%s\n" "${GREEN}" "${APP_DIR}" "${RST}"
else
  printf "  app:    %snot installed%s  (run install.sh)\n" "${YEL}" "${RST}"
fi

echo
echo "── login item ──────────────────────────────"
if osascript -e 'tell application "System Events" to get name of every login item' 2>/dev/null | tr ',' '\n' | grep -q "${APP_NAME}"; then
  printf "  → %sregistered%s — will start automatically at login\n" "${GREEN}" "${RST}"
else
  printf "  → %snot a login item%s\n" "${YEL}" "${RST}"
fi

echo
echo "── process ─────────────────────────────────"
if pid=$(pgrep -f 'infra-god serve' | head -1); [ -n "${pid:-}" ]; then
  printf "  pid: %s%s%s\n" "${GREEN}" "${pid}" "${RST}"
else
  printf "  pid: %snot running%s\n" "${RED}" "${RST}"
fi

echo
echo "── http /healthz ───────────────────────────"
healthz=""
if healthz=$(curl -sf -m 3 "http://localhost:${PORT}/healthz" 2>&1); then
  echo "${healthz}" | sed 's/^/  /'
  printf "  → %sresponding%s\n" "${GREEN}" "${RST}"
else
  printf "  → %snot responding%s on :%s\n" "${RED}" "${RST}" "${PORT}"
fi

echo
echo "── LAN reachability ────────────────────────"
status_json=$(curl -sf -m 3 "http://localhost:${PORT}/api/status" 2>/dev/null || true)
if [ -n "${status_json}" ]; then
  ok=$(printf '%s' "${status_json}" | grep -Eo '"status":[[:space:]]*"ok"' | wc -l | tr -d ' ')
  err=$(printf '%s' "${status_json}" | grep -Eo '"status":[[:space:]]*"error"' | wc -l | tr -d ' ')
  warn=$(printf '%s' "${status_json}" | grep -Eo '"status":[[:space:]]*"warning"' | wc -l | tr -d ' ')
  crit=$(printf '%s' "${status_json}" | grep -Eo '"status":[[:space:]]*"critical"' | wc -l | tr -d ' ')
  printf "  ok=%s  warn=%s  crit=%s  err=%s\n" "${ok}" "${warn}" "${crit}" "${err}"
  # If many "no route to host" errors, suggest Local Network permission
  if printf '%s' "${status_json}" | grep -q 'no route to host'; then
    printf "  %s⚠ 'no route to host' errors detected.%s\n" "${YEL}" "${RST}"
    printf "  %sLikely cause: Local Network privacy.%s\n" "${DIM}" "${RST}"
    printf "  %sFix: 시스템 설정 → 개인정보 보호 → 로컬 네트워크 → InfraGod 켜기%s\n" "${DIM}" "${RST}"
    printf "  %s     open 'x-apple.systempreferences:com.apple.preference.security?Privacy_LocalNetwork'%s\n" "${DIM}" "${RST}"
  fi
fi

echo
echo "── access URLs ─────────────────────────────"
IP=$(ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null || echo "<lan-ip>")
echo "  본인:      http://localhost:${PORT}/"
echo "  사내 공유: http://${IP}:${PORT}/"

echo
echo "── recent logs (out, last 8 lines) ─────────"
if [ -f "${LOG_DIR}/infra-god.out.log" ]; then
  tail -n 8 "${LOG_DIR}/infra-god.out.log" | sed "s|^|  ${DIM}|; s|$|${RST}|"
else
  echo "  (no log yet at ${LOG_DIR})"
fi

# Show err.log only if it has content beyond expected shutdown messages
if [ -f "${LOG_DIR}/infra-god.err.log" ] && [ -s "${LOG_DIR}/infra-god.err.log" ]; then
  if tail -n 30 "${LOG_DIR}/infra-god.err.log" | grep -qvE 'status refresh|container refresh|details refresh|folders refresh|^$'; then
    echo
    echo "── recent ERRORS (last 8 lines) ────────────"
    tail -n 30 "${LOG_DIR}/infra-god.err.log" | grep -vE 'status refresh|container refresh|details refresh|folders refresh' | tail -n 8 | sed "s|^|  ${RED}|; s|$|${RST}|"
  fi
fi
