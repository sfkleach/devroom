package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sfkleach/devroom/internal/config"
	devgit "github.com/sfkleach/devroom/internal/git"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build or rebuild the base container image",
	RunE:  runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) error {
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
	if cfg.BaseImage == "" {
		return fmt.Errorf("missing required config key 'base_image' in devroom.toml")
	}

	remoteURL, err := devgit.RemoteOrigin(root)
	if err != nil {
		return fmt.Errorf("reading git remote: %w", err)
	}
	owner, repo, err := devgit.OwnerRepo(remoteURL)
	if err != nil {
		return err
	}
	image := fmt.Sprintf("dev-%s-%s:base", owner, repo)

	tmpDir, err := os.MkdirTemp("", "devroom-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if cfg.JumpstartScript != "" {
		src := filepath.Join(root, cfg.JumpstartScript)
		if err := copyFile(src, filepath.Join(tmpDir, "jumpstart.sh")); err != nil {
			return fmt.Errorf("copying jumpstart script: %w", err)
		}
	}

	containerfile := generateContainerfile(cfg)
	if err := os.WriteFile(filepath.Join(tmpDir, "Containerfile"), []byte(containerfile), 0644); err != nil {
		return err
	}

	fmt.Printf("==> Building %s using %s ...\n", image, cfg.Runtime)

	c := exec.Command(cfg.Runtime, "build", "-t", image, tmpDir)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func generateContainerfile(cfg *config.Config) string {
	cf := fmt.Sprintf("FROM %s\n\nENV DEBIAN_FRONTEND=noninteractive\n", cfg.BaseImage)
	if cfg.JumpstartScript != "" {
		cf += "\nCOPY jumpstart.sh /tmp/jumpstart.sh\nRUN bash /tmp/jumpstart.sh\n"
	}
	return cf
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
