package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
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
// initial clone + shell setup. It creates an unprivileged user inside the
// container matching the host UID/GID so that mounted credentials are readable
// and Claude Code does not see a different user identity.
func firstEntry(runtime, containerName, baseImage, remoteURL, nickname string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	u, err := user.Current()
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
		"-e", "DEVROOM_UID=" + u.Uid,
		"-e", "DEVROOM_GID=" + u.Gid,
		"-e", "DEVROOM_USER=" + u.Username,
		"-e", "DEVROOM_HOME=" + home,
		"-e", "DEVROOM_NICK=" + nickname,
		"-e", "DEVROOM_REMOTE=" + remoteURL,
		"-v", home + "/.claude:" + home + "/.claude",
		"-v", home + "/.ssh:" + home + "/.ssh:ro",
		"-v", home + "/.gitconfig:" + home + "/.gitconfig:ro",
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

	// Create a matching user inside the container, clone the repo into the
	// user's home directory, write ~/.bash_profile to set PS1 and cd into the
	// workspace, then hand off to an interactive login shell as that user.
	initScript := `
getent group "${DEVROOM_GID}" >/dev/null 2>&1 || groupadd -g "${DEVROOM_GID}" "${DEVROOM_USER}"
getent passwd "${DEVROOM_UID}" >/dev/null 2>&1 || useradd -u "${DEVROOM_UID}" -g "${DEVROOM_GID}" -d "${DEVROOM_HOME}" -s /bin/bash -M "${DEVROOM_USER}"
mkdir -p "${DEVROOM_HOME}"
chown "${DEVROOM_UID}:${DEVROOM_GID}" "${DEVROOM_HOME}"
[ -f "${DEVROOM_HOME}/.bashrc" ] || { cp /etc/skel/.bashrc "${DEVROOM_HOME}/.bashrc" 2>/dev/null && chown "${DEVROOM_UID}:${DEVROOM_GID}" "${DEVROOM_HOME}/.bashrc"; } 2>/dev/null || true
[ -d "${DEVROOM_HOME}/workspace/.git" ] || su - "${DEVROOM_USER}" -c "git clone \"${DEVROOM_REMOTE}\" \"${DEVROOM_HOME}/workspace\""
{ echo '. ~/.bashrc 2>/dev/null'; echo "PS1='${DEVROOM_NICK}% '"; echo 'cd ~/workspace'; } > "${DEVROOM_HOME}/.bash_profile"
chown "${DEVROOM_UID}:${DEVROOM_GID}" "${DEVROOM_HOME}/.bash_profile"
exec su - "${DEVROOM_USER}"
`

	runArgs = append(runArgs, baseImage, "bash", "-c", initScript)

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

// execShell opens an interactive shell in a running container as the
// host user (matched by UID/GID), starting in ~/workspace.
func execShell(runtime, containerName, nickname string) error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	home := u.HomeDir
	devroomRc := home + "/.devroom_rc"

	// Write .devroom_rc then exec bash with it as the init file.
	// Source /etc/profile first so PATH includes /usr/local/go/bin etc.
	setup := fmt.Sprintf(
		`{ echo '. /etc/profile 2>/dev/null'; echo '. ~/.bashrc 2>/dev/null'; echo "PS1='%s%% '"; echo 'cd ~/workspace'; } > %s && exec bash --init-file %s -i`,
		nickname, devroomRc, devroomRc,
	)

	c := exec.Command(runtime, "exec", "-it",
		"--user", u.Uid+":"+u.Gid,
		"-e", "HOME="+home,
		containerName,
		"bash", "-c", setup,
	)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// forgeCredentialMount returns the -v mount string for the detected forge tool,
// or "" if the forge is not recognised. The mount target matches the host path
// so the user identity inside the container sees the same paths as on the host.
func forgeCredentialMount(remoteURL, home string) string {
	switch {
	case strings.Contains(remoteURL, "github.com"):
		return home + "/.config/gh:" + home + "/.config/gh:ro"
	case strings.Contains(remoteURL, "gitlab"):
		return home + "/.config/glab:" + home + "/.config/glab:ro"
	default:
		return ""
	}
}
