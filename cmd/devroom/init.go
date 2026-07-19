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

# Uncomment this to run a script during 'devroom build', baked into the
# shared base image (installing tools, etc.). Defaults to
# ~/.config/devroom/build.sh if unset.
# build_script = "scripts/build.sh"

# Uncomment this to source a shell snippet during 'devroom enter', just
# before the interactive shell starts (prompt tools, aliases, etc.).
# Defaults to ~/.config/devroom/enter.sh if unset.
# enter_script = "~/.config/devroom/enter.sh"
`, runtime)
}
