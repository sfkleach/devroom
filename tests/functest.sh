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

assert_not_contains() {
    local desc="$1" pattern="$2" path="$3"
    if ! grep -q "$pattern" "$path" 2>/dev/null; then
        pass "$desc"
    else
        fail "$desc (pattern '$pattern' unexpectedly found in $path)"
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
assert_contains "config has ai_default key"      '^ai_default '     "$d/.config/devroom/devroom.toml"
assert_contains "config has [[ai]] block"        '^\[\[ai\]\]'      "$d/.config/devroom/devroom.toml"
assert_contains "config has install_command key" 'install_command'  "$d/.config/devroom/devroom.toml"
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
# describe — error paths
# ===========================================================================

# No config file at all
d=$(mktmpgit)
out=$("$DEVROOM" describe somenick --rootdir "$d" 2>&1) || true
if echo "$out" | grep -q "No devroom configuration"; then
    pass "describe reports missing config file"
else
    fail "describe reports missing config file (got: $out)"
fi

# Config present but runtime key missing
d=$(mktmpgit)
mkdir -p "$d/.config/devroom"
printf 'base_image = "ubuntu:latest"\n' > "$d/.config/devroom/devroom.toml"
assert_output "describe reports missing runtime key" "runtime" "$DEVROOM" describe somenick --rootdir "$d"

# Runtime set, but ai_default entirely missing
d=$(mktmpgit)
mkdir -p "$d/.config/devroom"
printf 'runtime = "docker"\n' > "$d/.config/devroom/devroom.toml"
assert_output "describe reports missing ai_default key" "ai_default" "$DEVROOM" describe somenick --rootdir "$d"

# ai_default names an unknown [[ai]] entry (none configured at all)
d=$(mktmpgit)
mkdir -p "$d/.config/devroom"
printf 'runtime = "docker"\nai_default = "claude"\n' > "$d/.config/devroom/devroom.toml"
assert_output "describe reports unknown ai_default entry" "claude" "$DEVROOM" describe somenick --rootdir "$d"

# ai_default names a disabled [[ai]] entry
d=$(mktmpgit)
mkdir -p "$d/.config/devroom"
cat > "$d/.config/devroom/devroom.toml" <<'EOF'
runtime = "docker"
ai_default = "claude"

[[ai]]
name = "claude"
enabled = false
describe_command = "claude -p {}"
EOF
assert_output "describe reports disabled ai_default entry" "disabled" "$DEVROOM" describe somenick --rootdir "$d"

# Valid AI config, but the named room does not exist
d=$(mktmpgit)
git -C "$d" remote add origin https://github.com/example/testrepo.git
mkdir -p "$d/.config/devroom"
cat > "$d/.config/devroom/devroom.toml" <<'EOF'
runtime = "docker"
ai_default = "claude"

[[ai]]
name = "claude"
describe_command = "claude -p {}"
EOF
assert_output "describe reports nonexistent room" "no room named" "$DEVROOM" describe nosuchroom --rootdir "$d"

# ===========================================================================
# configure — happy path: edit one scalar field and save
# ===========================================================================

d=$(mktmpgit)
"$DEVROOM" init --rootdir "$d" >/dev/null 2>&1
printf '3\n.config/devroom/custom-build.sh\ns\n' | "$DEVROOM" configure --rootdir "$d" >/dev/null 2>&1
assert_contains "configure updates build_script" 'build_script = ".config/devroom/custom-build.sh"' "$d/.config/devroom/devroom.toml"

# ===========================================================================
# configure — leaving a field blank keeps it unchanged
# ===========================================================================

d=$(mktmpgit)
mkdir -p "$d/.config/devroom"
printf 'runtime = "podman"\n' > "$d/.config/devroom/devroom.toml"
printf '1\n\ns\n' | "$DEVROOM" configure --rootdir "$d" >/dev/null 2>&1
assert_contains "configure leaves a skipped field unchanged" 'runtime = "podman"' "$d/.config/devroom/devroom.toml"

# ===========================================================================
# configure — add, edit, and delete an [[ai]] entry
# ===========================================================================

d=$(mktmpgit)
"$DEVROOM" init --rootdir "$d" >/dev/null 2>&1

# Add: main menu -> AI submenu -> add "gemini" -> set install_command -> back, back, save.
printf 'a\na\ngemini\n3\nnpm install -g gemini-cli\nb\nb\ns\n' | "$DEVROOM" configure --rootdir "$d" >/dev/null 2>&1
assert_contains "configure adds a new [[ai]] entry" 'name = "gemini"' "$d/.config/devroom/devroom.toml"
assert_contains "configure sets install_command on the new entry" 'npm install -g gemini-cli' "$d/.config/devroom/devroom.toml"

# Edit: main menu -> AI submenu -> edit entry 2 (gemini) -> set enabled = false -> back, back, save.
printf 'a\n2\n2\nfalse\nb\nb\ns\n' | "$DEVROOM" configure --rootdir "$d" >/dev/null 2>&1
assert_contains "configure edits an existing [[ai]] entry's enabled flag" 'enabled = false' "$d/.config/devroom/devroom.toml"

# Delete: main menu -> AI submenu -> delete entry 2 (gemini), confirm -> back, save.
printf 'a\nd\n2\ny\nb\ns\n' | "$DEVROOM" configure --rootdir "$d" >/dev/null 2>&1
assert_not_contains "configure deletes an [[ai]] entry" 'name = "gemini"' "$d/.config/devroom/devroom.toml"
assert_contains "configure delete leaves the other entry intact" 'name = "claude"' "$d/.config/devroom/devroom.toml"

# ===========================================================================
# configure — warns about and can drop unrecognised keys on save
# ===========================================================================

d=$(mktmpgit)
"$DEVROOM" init --rootdir "$d" >/dev/null 2>&1
printf '\nextra_mounts = ["/foo"]\n' >> "$d/.config/devroom/devroom.toml"
out=$(printf 's\ny\n' | "$DEVROOM" configure --rootdir "$d" 2>&1)
if echo "$out" | grep -q "extra_mounts"; then
    pass "configure warns about unrecognised keys"
else
    fail "configure warns about unrecognised keys (got: $out)"
fi
assert_not_contains "configure drops the unrecognised key on confirmed save" 'extra_mounts' "$d/.config/devroom/devroom.toml"

printf '\nResults: %d passed, %d failed\n' "$PASS" "$FAIL"
[[ $FAIL -eq 0 ]]
