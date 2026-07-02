#!/usr/bin/env bash
# Functional tests for devroom.
#
# Usage: bash tests/functest.sh [path/to/devroom]
#   Defaults to ./devroom (relative to project root).
#
# Exit code: 0 if all tests passed, 1 if any failed.

DEVROOM="${1:-$(cd "$(dirname "$0")/.." && pwd)/devroom}"
PASS=0
FAIL=0

# All temporary directories live under _build/functest-$$ so that
# cleanup is always `rm -rf _build/functest-<pid>` — a static prefix.
TESTDIR="_build/functest-$$"
mkdir -p "$TESTDIR"
trap 'rm -rf _build/functest-'"$$" EXIT

mktmpgit() {
    local d="$TESTDIR/git-${RANDOM}${RANDOM}"
    mkdir -p "$d"
    git init -q "$d"
    echo "$d"
}

mktmpdir() {
    local d="$TESTDIR/dir-${RANDOM}${RANDOM}"
    mkdir -p "$d"
    echo "$d"
}

pass() { printf 'PASS: %s\n' "$1"; PASS=$((PASS + 1)); }
fail() { printf 'FAIL: %s\n' "$1"; FAIL=$((FAIL + 1)); }

assert_output() {
    local desc="$1" pattern="$2"; shift 2
    local out
    out=$("$@" 2>&1) || true
    if echo "$out" | grep -q "$pattern"; then
        pass "$desc"
    else
        fail "$desc (got: $(echo "$out" | head -3))"
    fi
}

assert_file() {
    local desc="$1" path="$2"
    if [[ -f "$path" ]]; then pass "$desc"; else fail "$desc (missing: $path)"; fi
}

assert_no_file() {
    local desc="$1" path="$2"
    if [[ ! -f "$path" ]]; then pass "$desc"; else fail "$desc (unexpectedly exists: $path)"; fi
}

assert_contains() {
    local desc="$1" pattern="$2" path="$3"
    if grep -q "$pattern" "$path" 2>/dev/null; then
        pass "$desc"
    else
        fail "$desc (pattern '$pattern' not found in $path)"
    fi
}

# ===========================================================================
# version
# ===========================================================================

assert_output "--version prints 'devroom <version>'"           '^devroom ' "$DEVROOM" --version
assert_output "-V prints 'devroom <version>'"                  '^devroom ' "$DEVROOM" -V
assert_output "version subcommand prints 'devroom <version>'"  '^devroom ' "$DEVROOM" version

v1=$("$DEVROOM" --version 2>&1)
v2=$("$DEVROOM" -V       2>&1)
v3=$("$DEVROOM" version  2>&1)
if [[ "$v1" == "$v2" && "$v1" == "$v3" ]]; then
    pass "--version, -V, and version subcommand all agree"
else
    fail "--version, -V, and version subcommand all agree (got: '$v1' '$v2' '$v3')"
fi

# ===========================================================================
# init — happy path in a git repo
# ===========================================================================

d=$(mktmpgit)
"$DEVROOM" init --rootdir "$d" >/dev/null 2>&1
assert_file     "init creates .config/devroom/devroom.toml"  "$d/.config/devroom/devroom.toml"
assert_contains "config has runtime key"       '^runtime '       "$d/.config/devroom/devroom.toml"
assert_contains "config has base_image key"    '^base_image '    "$d/.config/devroom/devroom.toml"
assert_contains "config has summary_model key" '^summary_model ' "$d/.config/devroom/devroom.toml"
runtime=$(grep '^runtime' "$d/.config/devroom/devroom.toml" | sed 's/.*= *"\(.*\)"/\1/')
if [[ "$runtime" == "docker" || "$runtime" == "podman" ]]; then
    pass "runtime value is docker or podman"
else
    fail "runtime value is docker or podman (got: '$runtime')"
fi

# ===========================================================================
# init — does not overwrite an existing config
# ===========================================================================

d=$(mktmpgit)
"$DEVROOM" init --rootdir "$d" >/dev/null 2>&1
echo "sentinel_value" >> "$d/.config/devroom/devroom.toml"
"$DEVROOM" init --rootdir "$d" >/dev/null 2>&1
assert_contains "init does not overwrite existing config" "sentinel_value" "$d/.config/devroom/devroom.toml"
assert_output   "init reports that config already exists" "already exists" "$DEVROOM" init --rootdir "$d"

# ===========================================================================
# init — non-git directory
# ===========================================================================

d=$(mktmpdir)
out=$(echo "n" | "$DEVROOM" init --rootdir "$d" 2>&1) || true
if echo "$out" | grep -qi "warning"; then
    pass "init warns when not in a git repo"
else
    fail "init warns when not in a git repo (got: $out)"
fi
assert_no_file "init aborts when user answers n" "$d/.config/devroom/devroom.toml"

d=$(mktmpdir)
echo "y" | "$DEVROOM" init --rootdir "$d" >/dev/null 2>&1
assert_file "init proceeds when user answers y" "$d/.config/devroom/devroom.toml"

# ===========================================================================
# build — error paths
# ===========================================================================

# No config file at all
d=$(mktmpgit)
out=$("$DEVROOM" build --rootdir "$d" 2>&1) || true
if echo "$out" | grep -q "No devroom configuration"; then
    pass "build reports missing config file"
else
    fail "build reports missing config file (got: $out)"
fi
if echo "$out" | grep -q "devroom init"; then
    pass "build hints at devroom init"
else
    fail "build hints at devroom init (got: $out)"
fi

# Config present but runtime key missing
d=$(mktmpgit)
mkdir -p "$d/.config/devroom"
printf 'base_image = "ubuntu:latest"\n' > "$d/.config/devroom/devroom.toml"
assert_output "build reports missing runtime key" "runtime" "$DEVROOM" build --rootdir "$d"

# Config present but base_image key missing
d=$(mktmpgit)
mkdir -p "$d/.config/devroom"
printf 'runtime = "podman"\n' > "$d/.config/devroom/devroom.toml"
assert_output "build reports missing base_image key" "base_image" "$DEVROOM" build --rootdir "$d"

# ===========================================================================
# summary
# ===========================================================================

printf '\nResults: %d passed, %d failed\n' "$PASS" "$FAIL"
[[ $FAIL -eq 0 ]]
