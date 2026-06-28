#!/usr/bin/env bash
# Install all prerequisites for building and testing devroom on Debian/Ubuntu.
# Safe to run more than once — each step checks whether the tool is already present.
set -euo pipefail

GO_VERSION="1.24.2"
GO_MIN_MAJOR=1
GO_MIN_MINOR=24

# Require a Debian/Ubuntu system.
if ! command -v apt-get >/dev/null 2>&1; then
    echo "error: this script requires apt-get (Debian/Ubuntu only)" >&2
    exit 1
fi

echo "==> Installing apt packages (requires sudo)..."
sudo apt-get update -q
sudo apt-get install -y \
    build-essential \
    curl \
    git \
    python3

# Go: install via official tarball if not present or below minimum version.
install_go() {
    echo "==> Installing Go ${GO_VERSION}..."
    local archive="go${GO_VERSION}.linux-amd64.tar.gz"
    local tmpdir
    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' RETURN
    curl -sL "https://go.dev/dl/${archive}" -o "$tmpdir/${archive}"
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "$tmpdir/${archive}"
    echo "==> Go ${GO_VERSION} installed."
}

if command -v go >/dev/null 2>&1; then
    INSTALLED=$(go version | grep -oP '\d+\.\d+' | head -1)
    INST_MAJOR=$(echo "$INSTALLED" | cut -d. -f1)
    INST_MINOR=$(echo "$INSTALLED" | cut -d. -f2)
    if [ "$INST_MAJOR" -lt "$GO_MIN_MAJOR" ] || \
       { [ "$INST_MAJOR" -eq "$GO_MIN_MAJOR" ] && [ "$INST_MINOR" -lt "$GO_MIN_MINOR" ]; }; then
        echo "==> Updating Go (need >=${GO_MIN_MAJOR}.${GO_MIN_MINOR}, have ${INSTALLED})..."
        install_go
    else
        echo "==> Go already installed ($(go version))."
    fi
else
    install_go
fi

# Make Go and installed binaries available for subsequent steps.
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"

# podman: needed for container operations.
if command -v podman >/dev/null 2>&1; then
    echo "==> podman already installed ($(podman --version))."
else
    echo "==> Installing podman..."
    sudo apt-get install -y podman
    echo "==> podman installed."
fi

echo ""
echo "All prerequisites installed. You can now build with:"
echo "  go build ./..."
echo "  go test ./..."
echo ""
