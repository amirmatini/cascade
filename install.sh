#!/bin/bash
set -e

REPO="amirmatini/cascade"
VERSION="${1:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="/etc/cascade"
CACHE_DIR="/var/cache/cascade"
SERVICE_FILE="/etc/systemd/system/cascade.service"
TMPDIR=$(mktemp -d)

cleanup() {
    rm -rf "$TMPDIR"
}
trap cleanup EXIT

download() {
    local url="$1"
    local output="$2"
    if command -v curl &> /dev/null; then
        curl -fsSL "$url" -o "$output" || return 1
    elif command -v wget &> /dev/null; then
        wget -q "$url" -O "$output" || return 1
    else
        echo "Error: curl/wget required" && exit 1
    fi
}

[ "$EUID" -ne 0 ] && echo "Error: root required" && exit 1
[ "$(uname -s)" != "Linux" ] && echo "Error: Linux only" && exit 1

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Error: unsupported arch" && exit 1 ;;
esac

BINARY_NAME="cascade-linux-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/${VERSION:-latest}/download/${BINARY_NAME}"

download "$DOWNLOAD_URL" "${TMPDIR}/cascade" || { echo "Error: download failed"; exit 1; }

install -m 755 "${TMPDIR}/cascade" "${INSTALL_DIR}/cascade"

id cascade &>/dev/null || useradd -r -s /bin/false cascade
mkdir -p "$CONFIG_DIR" "$CACHE_DIR"
chown cascade:cascade "$CACHE_DIR"

if [ ! -f "${CONFIG_DIR}/config.yaml" ]; then
    download "https://raw.githubusercontent.com/${REPO}/main/config.yaml" "${CONFIG_DIR}/config.yaml"
    sed -i "s|directory:.*|directory: \"${CACHE_DIR}\"|g" "${CONFIG_DIR}/config.yaml"
    chown cascade:cascade "${CONFIG_DIR}/config.yaml"
    chmod 640 "${CONFIG_DIR}/config.yaml"
fi

if [ ! -f "$SERVICE_FILE" ]; then
    download "https://raw.githubusercontent.com/${REPO}/main/cascade.service" "$SERVICE_FILE"
    systemctl daemon-reload
    systemctl enable cascade
fi

if systemctl is-active --quiet cascade; then
    systemctl restart cascade
    echo "Updated and restarted on port 3142"
else
    systemctl start cascade
    echo "Installed and started on port 3142"
fi
