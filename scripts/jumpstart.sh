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

# When running as root, sudo is unnecessary and often absent in containers.
if [ "$(id -u)" -eq 0 ]; then
    SUDO=""
else
    SUDO="sudo"
fi

echo "==> Installing apt packages..."
$SUDO apt-get -qq update >/dev/null
$SUDO apt-get -qq install -y \
    build-essential \
    curl \
    git \
    python3 \
    jq \
    unzip \
    >/dev/null

# Go: install via official tarball if not present or below minimum version.
install_go() {
    echo "==> Installing Go ${GO_VERSION}..."
    local archive="go${GO_VERSION}.linux-amd64.tar.gz"
    local tmpdir
    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' RETURN
    curl -sL "https://go.dev/dl/${archive}" -o "$tmpdir/${archive}"
    $SUDO rm -rf /usr/local/go
    $SUDO tar -C /usr/local -xzf "$tmpdir/${archive}"
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
export PATH="/usr/local/go/bin:$HOME/go/bin:$HOME/.local/bin:$PATH"

# Persist PATH additions for interactive shells in the container.
# Use ${HOME} so the path works for any user, not just root.
cat > /etc/profile.d/devroom-path.sh << 'EOF'
export PATH="/usr/local/go/bin:${HOME}/go/bin:${HOME}/.local/bin:$PATH"
EOF

# Claude Code CLI.
if command -v claude >/dev/null 2>&1; then
    echo "==> Claude Code already installed ($(claude --version 2>&1 | head -1))."
else
    echo "==> Installing Claude Code CLI..."
    curl -L https://claude.ai/install.sh | bash
    echo "==> Claude Code installed."
fi

# Make claude available system-wide so non-root container users can run it.
if [ -f "${HOME}/.local/bin/claude" ] && [ ! -f /usr/local/bin/claude ]; then
    cp "${HOME}/.local/bin/claude" /usr/local/bin/claude
    chmod 755 /usr/local/bin/claude
    echo "==> Claude Code copied to /usr/local/bin/claude."
fi

echo ""
echo "All prerequisites installed. You can now build with:"
echo "  go build ./..."
echo "  go test ./..."
echo ""
