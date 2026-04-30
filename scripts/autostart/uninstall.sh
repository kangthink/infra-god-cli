#!/usr/bin/env bash
# Remove InfraGod.app login item + bundle.

set -uo pipefail
APP_NAME="InfraGod"
APP_DIR="${HOME}/Applications/${APP_NAME}.app"

# Stop running instance
pkill -f 'infra-god serve' 2>/dev/null || true

# Remove login item
osascript -e 'tell application "System Events" to delete login item "InfraGod"' 2>/dev/null || true

# Remove bundle
rm -rf "${APP_DIR}"

echo "✅ InfraGod.app uninstalled. (logs kept at ~/Library/Logs/infra-god/)"
echo "   Local Network permission entry remains in System Settings — toggle off there if desired."
