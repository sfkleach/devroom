package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/sfkleach/devroom/internal/config"
	devgit "github.com/sfkleach/devroom/internal/git"
	"github.com/spf13/cobra"
)

var retireCmd = &cobra.Command{
	Use:   "retire <nickname>",
	Short: "Stop and delete a devroom's container (base image kept)",
	Args:  cobra.ExactArgs(1),
	RunE:  runRetire,
}

func init() {
	rootCmd.AddCommand(retireCmd)
}

func runRetire(cmd *cobra.Command, args []string) error {
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

	state, err := containerState(cfg.Runtime, containerName)
	if err != nil {
		return err
	}
	if state == "" {
		return fmt.Errorf("no room named %q (container %q does not exist)", nickname, containerName)
	}

	if err := stopAndRemoveContainer(cfg.Runtime, containerName, state); err != nil {
		return err
	}

	fmt.Printf("==> Room %q retired.\n", nickname)
	return nil
}

// stopAndRemoveContainer stops containerName if it's running, then removes
// it. state is the container's current State.Status (as returned by
// containerState), passed in so callers that already know it don't need a
// redundant inspect.
func stopAndRemoveContainer(runtime, containerName, state string) error {
	if state == "running" {
		fmt.Printf("==> Stopping room %q ...\n", containerName)
		stop := exec.Command(runtime, "stop", containerName)
		stop.Stdout = os.Stdout
		stop.Stderr = os.Stderr
		if err := stop.Run(); err != nil {
			return fmt.Errorf("stopping container: %w", err)
		}
	}

	fmt.Printf("==> Removing room %q ...\n", containerName)
	rm := exec.Command(runtime, "rm", containerName)
	rm.Stdout = os.Stdout
	rm.Stderr = os.Stderr
	if err := rm.Run(); err != nil {
		return fmt.Errorf("removing container: %w", err)
	}
	return nil
}
