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


################################################################################
# Apt packages
################################################################################

echo "==> Installing apt packages..."
$SUDO apt-get -qq update >/dev/null
$SUDO apt-get -qq install -y \
    build-essential \
    curl \
    git \
    python3 \
    jq \
    unzip \
    vim-tiny \
    >/dev/null


################################################################################
# Golang
################################################################################

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


################################################################################
# Node.js, via fnm
################################################################################

# Node.js, via fnm: required by devroom.toml's "claude" [[ai]] entry, whose
# install_command (npm install -g ...) runs after this script and needs
# npm already on PATH.
echo "==> Installing Node.js via fnm..."
curl -fsSL https://fnm.vercel.app/install | bash -s -- --install-dir /usr/local/fnm --skip-shell
export PATH="/usr/local/fnm:$PATH"
eval "$(fnm env)"
fnm install --lts
# Symlink into /usr/local/bin: the one location guaranteed to be on PATH
# for every invocation style, including devroom's own non-interactive
# "bash -c" execs (which don't source /etc/profile or ~/.bashrc).
ln -sf "$(fnm exec --using=lts-latest -- which node)" /usr/local/bin/node
ln -sf "$(fnm exec --using=lts-latest -- which npm)" /usr/local/bin/npm


################################################################################
# Tidying up - including PATH persistence for interactive shells
################################################################################

# Persist PATH additions for interactive shells in the container, once all
# of the above are known. Use ${HOME} so the path works for any user, not
# just root.
cat > /etc/profile.d/devroom-path.sh << 'EOF'
export PATH="/usr/local/go/bin:${HOME}/go/bin:${HOME}/.local/bin:/usr/local/fnm:$PATH"
EOF

echo ""
echo "All prerequisites installed. You can now build with:"
echo "  go build ./..."
echo "  go test ./..."
echo ""
