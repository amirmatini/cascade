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
        if ! curl -fsSL "$url" -o "$output"; then
            return 1
        fi
    elif command -v wget &> /dev/null; then
        if ! wget -q "$url" -O "$output"; then
            return 1
        fi
    else
        echo "Error: curl or wget required" >&2
        exit 1
    fi
}

if [ "$EUID" -ne 0 ]; then
    echo "Error: root privileges required" >&2
    exit 1
fi

if [ "$(uname -s)" != "Linux" ]; then
    echo "Error: Linux only" >&2
    exit 1
fi

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
        echo "Error: unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

BINARY_NAME="cascade-linux-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/${VERSION}/download/${BINARY_NAME}"

if ! download "$DOWNLOAD_URL" "${TMPDIR}/cascade"; then
    echo "Error: failed to download binary from $DOWNLOAD_URL" >&2
    exit 1
fi

if ! install -m 755 "${TMPDIR}/cascade" "${INSTALL_DIR}/cascade"; then
    echo "Error: failed to install binary" >&2
    exit 1
fi

if ! id cascade &>/dev/null; then
    if ! useradd -r -s /bin/false cascade; then
        echo "Error: failed to create cascade user" >&2
        exit 1
    fi
fi

if ! mkdir -p "$CONFIG_DIR" "$CACHE_DIR"; then
    echo "Error: failed to create directories" >&2
    exit 1
fi

if ! chown cascade:cascade "$CACHE_DIR"; then
    echo "Error: failed to set cache directory ownership" >&2
    exit 1
fi

if [ ! -f "${CONFIG_DIR}/config.yaml" ]; then
    if ! download "https://raw.githubusercontent.com/${REPO}/main/config.yaml" "${CONFIG_DIR}/config.yaml"; then
        echo "Error: failed to download config file" >&2
        exit 1
    fi
    sed -i "s|directory:.*|directory: \"${CACHE_DIR}\"|g" "${CONFIG_DIR}/config.yaml"
    chown cascade:cascade "${CONFIG_DIR}/config.yaml"
    chmod 640 "${CONFIG_DIR}/config.yaml"
fi

if [ ! -f "$SERVICE_FILE" ]; then
    if ! download "https://raw.githubusercontent.com/${REPO}/main/cascade.service" "$SERVICE_FILE"; then
        echo "Error: failed to download service file" >&2
        exit 1
    fi
    systemctl daemon-reload
    systemctl enable cascade
fi

if systemctl is-active --quiet cascade; then
    systemctl restart cascade
    echo "Cascade updated and restarted on port 3142"
else
    systemctl start cascade
    echo "Cascade installed and started on port 3142"
fi
