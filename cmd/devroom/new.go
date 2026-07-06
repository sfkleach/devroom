package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/sfkleach/devroom/internal/config"
	devgit "github.com/sfkleach/devroom/internal/git"
	"github.com/spf13/cobra"
)

var newBranch bool

var newCmd = &cobra.Command{
	Use:   "new <nickname>",
	Short: "Create a new devroom",
	Args:  cobra.ExactArgs(1),
	RunE:  runNew,
}

func init() {
	newCmd.Flags().BoolVarP(&newBranch, "branch", "b", false, "Create and checkout a feature branch with the room name")
	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, args []string) error {
	newName := args[0]

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

	if newBranch {
		if err := checkBranchAvailable(root, newName); err != nil {
			return err
		}
	}

	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("getting current user: %w", err)
	}
	home := u.HomeDir
	workspace := home + "/workspace"
	hostGitconfig := filepath.Join(home, ".gitconfig")
	containerGitconfigHostRO := home + "/.gitconfig.host-ro"

	baseImage := fmt.Sprintf("localhost/dev-%s-%s:base", owner, repo)
	containerName := fmt.Sprintf("devroom-%s-%s-%s", owner, repo, newName)

	fmt.Printf("==> Creating room %q from %s ...\n", containerName, baseImage)
	runArgs := []string{
		"run", "-d", "--name", containerName,
		"-e", "DEVROOM_UID=" + u.Uid,
		"-e", "DEVROOM_GID=" + u.Gid,
		"-e", "DEVROOM_USER=" + u.Username,
		"-e", "DEVROOM_HOME=" + home,
	}
	if cfg.Runtime == "podman" {
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

	run := exec.Command(cfg.Runtime, runArgs...)
	run.Stdout = os.Stdout
	run.Stderr = os.Stderr
	if err := run.Run(); err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	fmt.Println("==> Setting up user inside container...")
	// Run as root explicitly: with --userns=keep-id (used for podman), the
	// container's default exec user is the current host user rather than
	// root, so useradd/chown here would otherwise fail with EPERM.
	setup := exec.Command(cfg.Runtime, "exec", "--user", "root", containerName, "bash", "-c", userSetupScript)
	setup.Stdout = os.Stdout
	setup.Stderr = os.Stderr
	if err := setup.Run(); err != nil {
		return fmt.Errorf("setting up user: %w", err)
	}

	fmt.Printf("==> Authenticating with %s via %s...\n", host, f.name())
	login := exec.Command(cfg.Runtime, "exec", "-i",
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
	clone := exec.Command(cfg.Runtime, "exec",
		"--user", u.Uid+":"+u.Gid,
		containerName, "git", "clone", httpsRemote, workspace,
	)
	clone.Stdout = os.Stdout
	clone.Stderr = os.Stderr
	if err := clone.Run(); err != nil {
		return fmt.Errorf("cloning repository: %w", err)
	}

	if newBranch {
		fmt.Printf("==> Creating branch %q ...\n", newName)
		branch := exec.Command(cfg.Runtime, "exec",
			"--user", u.Uid+":"+u.Gid,
			containerName, "git", "-C", workspace, "checkout", "-b", newName,
		)
		branch.Stdout = os.Stdout
		branch.Stderr = os.Stderr
		if err := branch.Run(); err != nil {
			return fmt.Errorf("creating branch: %w", err)
		}
	}

	fmt.Printf("==> Room %q is ready. Use 'devroom enter %s' to start working.\n", containerName, newName)
	return nil
}

func checkBranchAvailable(root, name string) error {
	out, err := exec.Command("git", "-C", root, "branch", "--list", name).Output()
	if err != nil {
		return fmt.Errorf("checking branch availability: %w", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("branch %q already exists", name)
	}
	return nil
}
