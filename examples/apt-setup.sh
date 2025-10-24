#!/bin/bash
set -e

PROXY_HOST="${1:-localhost}"
PROXY_PORT="${2:-3142}"
PROXY_URL="http://${PROXY_HOST}:${PROXY_PORT}"
APT_PROXY_FILE="/etc/apt/apt.conf.d/01cascade-proxy"

[ "$EUID" -ne 0 ] && echo "Error: root required" && exit 1

cat > "$APT_PROXY_FILE" <<EOF
Acquire::http::Proxy "$PROXY_URL";
Acquire::http::Timeout "120";
Acquire::Retries "3";
EOF

echo "Configured: $APT_PROXY_FILE"
echo "To disable: rm $APT_PROXY_FILE"

