package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sfkleach/devroom/internal/config"
	devgit "github.com/sfkleach/devroom/internal/git"
	"github.com/spf13/cobra"
)

var enterCmd = &cobra.Command{
	Use:   "enter <nickname>",
	Short: "Enter a devroom (start or resume its container)",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnter,
}

func init() {
	rootCmd.AddCommand(enterCmd)
}

func runEnter(cmd *cobra.Command, args []string) error {
	nickname := args[0]

	root, err := effectiveRootDir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		if errors.Is(err, config.ErrNoConfig) {
			fmt.Fprintln(os.Stderr, "No devroom configuration file found.")
			fmt.Fprintln(os.Stderr, "Hint: run 'devroom init' to create one.")
			os.Exit(1)
		}
		return err
	}

	if cfg.Runtime == "" {
		return fmt.Errorf("missing required config key 'runtime' in devroom.toml")
	}

	remoteURL, err := devgit.RemoteOrigin(root)
	if err != nil {
		return fmt.Errorf("reading git remote: %w", err)
	}
	owner, repo, err := devgit.OwnerRepo(remoteURL)
	if err != nil {
		return err
	}

	containerName := fmt.Sprintf("devroom-%s-%s-%s", owner, repo, nickname)
	baseImage := fmt.Sprintf("localhost/dev-%s-%s:base", owner, repo)

	state, err := containerState(cfg.Runtime, containerName)
	if err != nil {
		return err
	}

	switch state {
	case "":
		fmt.Printf("==> First entry: creating room %q ...\n", containerName)
		return firstEntry(cfg.Runtime, containerName, baseImage, remoteURL, nickname)
	case "running":
		return execShell(cfg.Runtime, containerName, nickname)
	default:
		fmt.Printf("==> Resuming room %q ...\n", containerName)
		return resumeEntry(cfg.Runtime, containerName, nickname)
	}
}

// containerState returns the container's State.Status ("running", "exited", etc.)
// or "" if the container does not exist.
func containerState(runtime, name string) (string, error) {
	out, err := exec.Command(runtime, "inspect", "--format", "{{.State.Status}}", name).Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// firstEntry creates a new container with all credential mounts and runs the
// initial clone + shell setup.
func firstEntry(runtime, containerName, baseImage, remoteURL, nickname string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	runArgs := []string{
		"run", "-it", "--name", containerName,
		"-e", "DEVROOM_SHELL=" + shell,
		"-v", home + "/.claude:/root/.claude:ro",
		"-v", home + "/.ssh:/root/.ssh:ro",
		"-v", home + "/.gitconfig:/root/.gitconfig:ro",
	}

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		runArgs = append(runArgs,
			"-v", sock+":/run/ssh-agent:ro",
			"-e", "SSH_AUTH_SOCK=/run/ssh-agent",
		)
	}

	if mount := forgeCredentialMount(remoteURL, home); mount != "" {
		runArgs = append(runArgs, "-v", mount)
	}

	runArgs = append(runArgs, baseImage, "bash", "-c",
		fmt.Sprintf(
			`[ -d /workspace ] || git clone %s /workspace && `+
				`cd /workspace && `+
				`printf '. /root/.bashrc 2>/dev/null\nPS1="%s%%%% "\n' > /root/.devroom_rc && `+
				`exec bash --init-file /root/.devroom_rc -i`,
			remoteURL, nickname,
		),
	)

	c := exec.Command(runtime, runArgs...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// resumeEntry starts a stopped container then opens a shell inside it.
func resumeEntry(runtime, containerName, nickname string) error {
	start := exec.Command(runtime, "start", containerName)
	start.Stderr = os.Stderr
	if err := start.Run(); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}
	return execShell(runtime, containerName, nickname)
}

// execShell opens an interactive shell in a running container.
func execShell(runtime, containerName, nickname string) error {
	setup := fmt.Sprintf(
		`printf '. /root/.bashrc 2>/dev/null\nPS1="%s%%%% "\n' > /root/.devroom_rc && exec bash --init-file /root/.devroom_rc -i`,
		nickname,
	)
	c := exec.Command(runtime, "exec", "-it", containerName, "bash", "-c", setup)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// forgeCredentialMount returns the -v mount string for the detected forge tool,
// or "" if the forge is not recognised.
func forgeCredentialMount(remoteURL, home string) string {
	switch {
	case strings.Contains(remoteURL, "github.com"):
		return home + "/.config/gh:/root/.config/gh:ro"
	case strings.Contains(remoteURL, "gitlab"):
		return home + "/.config/glab:/root/.config/glab:ro"
	default:
		return ""
	}
}
