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

var destroyForce bool
var destroyKeepChildren bool

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Remove the base image for this repo",
	Args:  cobra.NoArgs,
	RunE:  runDestroy,
}

func init() {
	destroyCmd.Flags().BoolVarP(&destroyForce, "force", "f", false, "Ignore a missing base image, and delete any existing rooms without asking")
	destroyCmd.Flags().BoolVarP(&destroyKeepChildren, "keep-children", "k", false, "Leave existing rooms in place instead of deleting them")
	rootCmd.AddCommand(destroyCmd)
}

func runDestroy(cmd *cobra.Command, args []string) error {
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

	baseImage := fmt.Sprintf("localhost/dev-%s-%s:base", owner, repo)

	_, err = imageID(cfg.Runtime, baseImage, "{{.Id}}")
	imageExists := err == nil
	if !imageExists && !destroyForce {
		return fmt.Errorf("base image %q does not exist", baseImage)
	}

	nicknames, err := listRoomNicknames(cfg.Runtime, owner, repo)
	if err != nil {
		return err
	}

	// Containers freeze to the image ID they were created from, so removing
	// the tag doesn't break existing rooms — but the runtime still refuses
	// to remove an image that any container (running or stopped) still
	// references, unless the removal itself is forced. -k keeps the rooms
	// and accepts that the removal below needs -f; otherwise we either
	// clear them out first (making a forced removal unnecessary) or abort.
	if len(nicknames) > 0 && !destroyKeepChildren {
		proceed := destroyForce
		if !proceed {
			fmt.Printf("The following rooms still exist for this repo: %s\n", strings.Join(nicknames, ", "))
			proceed = confirmYN("Delete these rooms too?", false)
		}
		if !proceed {
			fmt.Println("Aborted: rooms still reference this image.")
			fmt.Println("Re-run with -k to leave them and destroy the image anyway, or retire them first.")
			return nil
		}
		for _, nickname := range nicknames {
			containerName := fmt.Sprintf("devroom-%s-%s-%s", owner, repo, nickname)
			state, err := containerState(cfg.Runtime, containerName)
			if err != nil {
				return err
			}
			if err := stopAndRemoveContainer(cfg.Runtime, containerName, state); err != nil {
				return err
			}
		}
		nicknames = nil
	}

	if !imageExists {
		// destroyForce is set (otherwise we'd have returned above) and
		// there's nothing left to remove.
		return nil
	}

	fmt.Printf("==> Removing base image %q ...\n", baseImage)
	rmiArgs := []string{"rmi"}
	if len(nicknames) > 0 {
		// -k kept rooms that still reference this image.
		rmiArgs = append(rmiArgs, "-f")
	}
	rmiArgs = append(rmiArgs, baseImage)
	rmi := exec.Command(cfg.Runtime, rmiArgs...)
	rmi.Stdout = os.Stdout
	rmi.Stderr = os.Stderr
	if err := rmi.Run(); err != nil {
		return fmt.Errorf("removing image: %w", err)
	}

	fmt.Printf("==> Base image %q destroyed.\n", baseImage)
	return nil
}
