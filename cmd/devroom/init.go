package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create an initial .config/devroom/devroom.toml",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	root, err := effectiveRootDir()
	if err != nil {
		return err
	}

	if !isGitRepo(root) {
		fmt.Fprintf(os.Stderr, "Warning: %s does not appear to be a git repository.\n", root)
		if !confirmYN("Create configuration anyway?", false) {
			fmt.Println("Aborted.")
			return nil
		}
	}

	configPath := filepath.Join(root, ".config", "devroom", "devroom.toml")
	if _, err := os.Stat(configPath); err == nil {
		fmt.Fprintf(os.Stderr, "Configuration already exists at %s — not overwriting.\n", configPath)
		return nil
	}

	runtime := detectRuntime()

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	content := buildInitConfig(runtime)
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	fmt.Printf("Created %s\n", configPath)

	configDir := filepath.Dir(configPath)
	if err := writeIfAbsent(filepath.Join(configDir, "build.sh"), buildScriptTemplate); err != nil {
		return err
	}
	if err := writeIfAbsent(filepath.Join(configDir, "enter.sh"), enterScriptTemplate); err != nil {
		return err
	}

	return nil
}

// writeIfAbsent writes content to path unless a file already exists there,
// in which case it leaves it untouched and reports the fact.
func writeIfAbsent(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("%s already exists — not overwriting.\n", path)
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Printf("Created %s\n", path)
	return nil
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// detectRuntime checks for docker and podman in PATH.
// If both are found, docker is chosen and the user is informed.
func detectRuntime() string {
	_, hasDocker := exec.LookPath("docker")
	_, hasPodman := exec.LookPath("podman")
	switch {
	case hasDocker == nil && hasPodman == nil:
		fmt.Println("Both docker and podman found; defaulting to docker.")
		return "docker"
	case hasDocker == nil:
		return "docker"
	case hasPodman == nil:
		return "podman"
	default:
		return "podman"
	}
}

// confirmYN prompts the user with a Y/n or y/N question. def is the default.
func confirmYN(prompt string, def bool) bool {
	if def {
		fmt.Printf("%s (Y/n): ", prompt)
	} else {
		fmt.Printf("%s (y/N): ", prompt)
	}
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return def
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer == "" {
		return def
	}
	return answer == "y" || answer == "yes"
}

func buildInitConfig(runtime string) string {
	return fmt.Sprintf(`# The runtime should be docker or podman.
runtime = "%s"

# This can be any base image in the docker hub registry.
base_image = "ubuntu:latest"

# At the time of writing only Claude is supported.
summary_model = "claude sonnet 4.5"

# Runs during 'devroom build', baked into the shared base image. Delete
# .config/devroom/build.sh (and this line) to skip this step entirely.
build_script = ".config/devroom/build.sh"

# Sourced during 'devroom enter', just before the interactive shell starts.
# Delete .config/devroom/enter.sh (and this line) to skip this step entirely.
enter_script = ".config/devroom/enter.sh"
`, runtime)
}

// buildScriptTemplate is the starter build_script scaffolded by `devroom
// init`. Every real command in it is commented out; it exists to show where
// project-specific toolchain installs go and to make clear it's safe to
// delete.
const buildScriptTemplate = `#!/usr/bin/env bash
# This is devroom's build_script (see devroom.toml): it runs once during
# 'devroom build', baked into the shared base image used by every room in
# this repo.
#
# It's entirely optional. Delete this file (and the build_script line in
# devroom.toml) and devroom simply skips this step — nothing else depends
# on it existing.
#
# Everything below is commented out: starter commands for some popular
# toolchains on an Ubuntu/apt-based base image. Uncomment whichever this
# project actually needs.
set -euo pipefail

# --- C/C++ ---
# apt-get update -qq
# apt-get install -y -qq --no-install-recommends build-essential cmake

# --- Node.js, via fnm ---
# curl -fsSL https://fnm.vercel.app/install | bash -s -- --install-dir /usr/local/fnm --skip-shell
# export PATH="/usr/local/fnm:$PATH"
# eval "$(fnm env)"
# fnm install --lts

# --- Python, via uv ---
# curl -LsSf https://astral.sh/uv/install.sh | sh
# ~/.local/bin/uv python install 3.12

# --- Go ---
# curl -fsSL https://go.dev/dl/go1.24.2.linux-amd64.tar.gz -o /tmp/go.tar.gz
# tar -C /usr/local -xzf /tmp/go.tar.gz

# --- Rust ---
# curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
`

// enterScriptTemplate is the starter enter_script scaffolded by `devroom
// init`, matching the toolchains suggested in buildScriptTemplate.
const enterScriptTemplate = `# This is devroom's enter_script (see devroom.toml): it is sourced inside
# the room, once per 'devroom enter'/'devroom new', just before the
# interactive shell starts. Use it for env vars, aliases, and prompt setup
# that belong in a shell session rather than baked into the image.
#
# It's entirely optional. Delete this file (and the enter_script line in
# devroom.toml) and devroom falls back to its default minimal prompt
# ("<nickname>% ") — nothing else depends on it existing.
#
# Everything below is commented out: starter commands matching the
# toolchains suggested in build.sh. Uncomment whichever this project
# actually needs.

# --- C/C++ ---
# (nothing to source once build-essential/cmake are installed)

# --- Node.js, via fnm ---
# export PATH="/usr/local/fnm:$PATH"
# eval "$(fnm env --use-on-cd)"

# --- Python, via uv ---
# export PATH="$HOME/.local/bin:$PATH"

# --- Go ---
# export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"

# --- Rust ---
# source "$HOME/.cargo/env"

# --- Prompt ---
# export PS1="${DEVROOM_NICKNAME}% "
`
