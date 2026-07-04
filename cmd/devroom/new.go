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

var (
	newName   string
	newBranch bool
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new devroom",
	RunE:  runNew,
}

func init() {
	newCmd.Flags().StringVarP(&newName, "name", "n", "", "Name of the room (required)")
	_ = newCmd.MarkFlagRequired("name")
	newCmd.Flags().BoolVarP(&newBranch, "branch", "b", false, "Create and checkout a feature branch with the room name")
	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, args []string) error {
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

	baseImage := fmt.Sprintf("localhost/dev-%s-%s:base", owner, repo)
	containerName := fmt.Sprintf("devroom-%s-%s-%s", owner, repo, newName)

	fmt.Printf("==> Creating room %q from %s ...\n", containerName, baseImage)
	run := exec.Command(cfg.Runtime, "run", "-d", "--name", containerName,
		"-e", "DEVROOM_UID="+u.Uid,
		"-e", "DEVROOM_GID="+u.Gid,
		"-e", "DEVROOM_USER="+u.Username,
		"-e", "DEVROOM_HOME="+home,
		baseImage, "sleep", "infinity",
	)
	run.Stdout = os.Stdout
	run.Stderr = os.Stderr
	if err := run.Run(); err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	fmt.Println("==> Setting up user inside container...")
	userSetup := `
getent group "${DEVROOM_GID}" >/dev/null 2>&1 || groupadd -g "${DEVROOM_GID}" "${DEVROOM_USER}"
getent passwd "${DEVROOM_UID}" >/dev/null 2>&1 || useradd -u "${DEVROOM_UID}" -g "${DEVROOM_GID}" -d "${DEVROOM_HOME}" -s /bin/bash -M "${DEVROOM_USER}"
mkdir -p "${DEVROOM_HOME}"
chown "${DEVROOM_UID}:${DEVROOM_GID}" "${DEVROOM_HOME}"
`
	setup := exec.Command(cfg.Runtime, "exec", containerName, "bash", "-c", userSetup)
	setup.Stdout = os.Stdout
	setup.Stderr = os.Stderr
	if err := setup.Run(); err != nil {
		return fmt.Errorf("setting up user: %w", err)
	}

	fmt.Println("==> Cloning repository...")
	clone := exec.Command(cfg.Runtime, "exec",
		"--user", u.Uid+":"+u.Gid,
		containerName, "git", "clone", remoteURL, workspace,
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
