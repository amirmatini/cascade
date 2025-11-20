#!/bin/bash
set -e

PROXY_HOST="${1:-localhost}"
PROXY_PORT="${2:-3142}"
PROXY_URL="http://${PROXY_HOST}:${PROXY_PORT}"

[ "$EUID" -ne 0 ] && echo "Error: root required" && exit 1

if command -v dnf &> /dev/null; then
    CONF_FILE="/etc/dnf/dnf.conf"
elif command -v yum &> /dev/null; then
    CONF_FILE="/etc/yum.conf"
else
    echo "Error: YUM/DNF not found" && exit 1
fi

BACKUP_FILE="${CONF_FILE}.backup-$(date +%Y%m%d-%H%M%S)"
cp "$CONF_FILE" "$BACKUP_FILE"

sed -i '/^proxy=/d' "$CONF_FILE"
echo "proxy=$PROXY_URL" >> "$CONF_FILE"

echo "Configured: $CONF_FILE"
echo "Backup: $BACKUP_FILE"

