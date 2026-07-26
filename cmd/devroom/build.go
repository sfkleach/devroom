package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

// baseImageBuilt reports whether the shared base image for this repo has
// already been built via `devroom build`.
func baseImageBuilt(runtime, owner, repo string) bool {
	baseImage := fmt.Sprintf("localhost/dev-%s-%s:base", owner, repo)
	return exec.Command(runtime, "image", "inspect", baseImage).Run() == nil
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

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	buildScriptPath := resolveHostScript(cfg.BuildScript, root, home, "build.sh")
	if buildScriptPath != "" {
		if err := copyFile(buildScriptPath, filepath.Join(tmpDir, "build.sh")); err != nil {
			return fmt.Errorf("copying build script: %w", err)
		}
	}

	containerfile := generateContainerfile(cfg, buildScriptPath != "")
	if err := os.WriteFile(filepath.Join(tmpDir, "Containerfile"), []byte(containerfile), 0644); err != nil {
		return err
	}

	fmt.Printf("==> Building %s using %s ...\n", image, cfg.Runtime)

	c := exec.Command(cfg.Runtime, "build", "-t", image, tmpDir)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func generateContainerfile(cfg *config.Config, hasBuildScript bool) string {
	cf := fmt.Sprintf("FROM %s\n\nENV DEBIAN_FRONTEND=noninteractive\n", cfg.BaseImage)
	// gh and glab are hard dependencies: devroom authenticates git clones
	// inside the room using whichever forge CLI matches the origin host,
	// instead of relying on SSH keys/agent forwarding. Install them before
	// the project's own build script so it can assume they are present.
	cf += "\n" + forgeToolsInstall
	if hasBuildScript {
		cf += "\nCOPY build.sh /tmp/build.sh\nRUN bash /tmp/build.sh\n"
	}
	cf += aiInstallSteps(cfg)
	return cf
}

// aiInstallSteps emits one "RUN <install_command>" line per enabled [[ai]]
// entry with a non-empty install_command, so each AI CLI is baked into the
// shared base image once at build time rather than per-room. Runs after
// the project's own build_script, since an AI CLI's install_command may
// depend on a toolchain build_script installs (e.g. Claude Code needing
// Node/npm, which isn't present on a bare base image otherwise).
func aiInstallSteps(cfg *config.Config) string {
	var sb strings.Builder
	for _, entry := range cfg.AI {
		if !entry.IsEnabled() || entry.InstallCommand == "" {
			continue
		}
		fmt.Fprintf(&sb, "\nRUN %s\n", entry.InstallCommand)
	}
	return sb.String()
}

// forgeToolsInstall installs the GitHub and GitLab CLIs via their official
// apt repositories. This assumes a Debian/Ubuntu base image, consistent with
// build.sh's own requirement of apt-get.
const forgeToolsInstall = `RUN apt-get update -qq \
    && apt-get install -y -qq --no-install-recommends curl ca-certificates sudo \
    && mkdir -p -m 755 /etc/apt/keyrings \
    && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg -o /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list \
    && curl -fsSL https://packages.gitlab.com/install/repositories/gitlab.com/glab/script.deb.sh | bash \
    && apt-get update -qq \
    && apt-get install -y -qq --no-install-recommends gh glab \
    && rm -rf /var/lib/apt/lists/*
`

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
