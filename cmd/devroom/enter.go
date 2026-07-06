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
		host, err := devgit.Host(remoteURL)
		if err != nil {
			return err
		}
		f := detectForge(host)
		if f == forgeUnknown {
			return fmt.Errorf("unrecognised git forge host %q; only github.com and gitlab hosts are supported", host)
		}
		// Acquire the auth token up front, before creating any container, so
		// misconfigured/missing forge CLI credentials fail fast and clearly
		// rather than after a room has already been partially created.
		token, err := f.token()
		if err != nil {
			return err
		}
		httpsRemote := devgit.HTTPSRemote(host, owner, repo)
		fmt.Printf("==> First entry: creating room %q ...\n", containerName)
		return firstEntry(cfg.Runtime, containerName, baseImage, host, httpsRemote, token, f, nickname)
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

// firstEntry creates a new container with all credential mounts, provisions
// the matching user, authenticates with the detected forge CLI, clones the
// repo, and then hands off to an interactive shell via execShell. It creates
// an unprivileged user inside the container matching the host UID/GID so
// that mounted credentials are readable and Claude Code does not see a
// different user identity.
//
// Container creation and provisioning are done as separate, non-interactive
// steps (rather than as one big "run -it ... init script") so that the
// forge login token can be piped over a dedicated stdin without disturbing
// the real terminal stdin that the final interactive shell needs for proper
// raw-mode TTY behaviour.
func firstEntry(runtime, containerName, baseImage, host, httpsRemote, token string, f forge, nickname string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	u, err := user.Current()
	if err != nil {
		return err
	}

	hostGitconfig := home + "/.gitconfig"
	containerGitconfigHostRO := home + "/.gitconfig.host-ro"

	runArgs := []string{
		"run", "-d", "--name", containerName,
		"-e", "DEVROOM_UID=" + u.Uid,
		"-e", "DEVROOM_GID=" + u.Gid,
		"-e", "DEVROOM_USER=" + u.Username,
		"-e", "DEVROOM_HOME=" + home,
		"-v", home + "/.claude:" + home + "/.claude",
	}

	// Mount the host's claude binary so the container gets the same version
	// without needing to install it during the image build.
	if claudePath, err := exec.LookPath("claude"); err == nil {
		runArgs = append(runArgs, "-v", claudePath+":/usr/local/bin/claude:ro")
	}

	if runtime == "podman" {
		// Under rootless Podman, container UIDs are remapped through the
		// subuid range by default, so a container process running as
		// "host UID 1000" is not actually the host's UID 1000 as far as the
		// kernel is concerned. That breaks read access to bind-mounted
		// files owned by the real host user, such as ~/.ssh/known_hosts,
		// which in turn makes ssh silently fail host key verification.
		// --userns=keep-id maps the container's matching UID/GID back to
		// the real host UID/GID so bind mounts keep the right ownership.
		runArgs = append(runArgs, "--userns=keep-id")
	}

	// Mount ~/.gitconfig read-only to a side path rather than straight onto
	// $HOME/.gitconfig: the setup step below needs to write the forge CLI's
	// credential helper into the container user's global gitconfig, which
	// isn't possible if that path is a read-only bind mount of the host file.
	if _, err := os.Stat(hostGitconfig); err == nil {
		runArgs = append(runArgs, "-v", hostGitconfig+":"+containerGitconfigHostRO+":ro")
	}

	runArgs = append(runArgs, baseImage, "sleep", "infinity")

	run := exec.Command(runtime, runArgs...)
	run.Stdout = os.Stdout
	run.Stderr = os.Stderr
	if err := run.Run(); err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	// Run as root explicitly: with --userns=keep-id (used for podman), the
	// container's default exec user is the current host user rather than
	// root, so useradd/chown here would otherwise fail with EPERM.
	setup := exec.Command(runtime, "exec", "--user", "root", containerName, "bash", "-c", userSetupScript)
	setup.Stdout = os.Stdout
	setup.Stderr = os.Stderr
	if err := setup.Run(); err != nil {
		return fmt.Errorf("setting up user: %w", err)
	}

	fmt.Printf("==> Authenticating with %s via %s...\n", host, f.name())
	login := exec.Command(runtime, "exec", "-i",
		"--user", u.Uid+":"+u.Gid,
		containerName, "bash", "-c", loginScript(f, host),
	)
	login.Stdin = strings.NewReader(token + "\n")
	login.Stdout = os.Stdout
	login.Stderr = os.Stderr
	if err := login.Run(); err != nil {
		return fmt.Errorf("authenticating with %s: %w", f.name(), err)
	}

	fmt.Println("==> Cloning repository...")
	clone := exec.Command(runtime, "exec",
		"--user", u.Uid+":"+u.Gid,
		containerName, "git", "clone", httpsRemote, home+"/workspace",
	)
	clone.Stdout = os.Stdout
	clone.Stderr = os.Stderr
	if err := clone.Run(); err != nil {
		return fmt.Errorf("cloning repository: %w", err)
	}

	return execShell(runtime, containerName, nickname)
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
