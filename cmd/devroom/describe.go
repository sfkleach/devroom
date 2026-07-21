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

var describeVerbose int

var describeCmd = &cobra.Command{
	Use:   "describe <nickname>",
	Short: "Generate an AI-written description of a room's current state",
	Args:  cobra.ExactArgs(1),
	RunE:  runDescribe,
}

func init() {
	describeCmd.Flags().CountVarP(&describeVerbose, "verbose", "v",
		"Increase description detail (repeatable: -v paragraph, -vv+ full detail)")
	rootCmd.AddCommand(describeCmd)
}

func runDescribe(cmd *cobra.Command, args []string) error {
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

	// Resolve the describe AI before touching git/containers at all, so
	// config-only mistakes fail fast, mirroring build.go's up-front checks.
	ai, err := cfg.ResolveDefaultAI()
	if err != nil {
		return err
	}
	if ai.DescribeCommand == "" {
		return fmt.Errorf("[[ai]] entry %q has no describe_command configured", ai.Name)
	}
	if !strings.Contains(ai.DescribeCommand, "{}") {
		return fmt.Errorf("describe_command for %q has no {} placeholder for the prompt", ai.Name)
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

	if state != "running" {
		fmt.Printf("==> Starting room %q temporarily to generate description...\n", containerName)
		start := exec.Command(cfg.Runtime, "start", containerName)
		start.Stderr = os.Stderr
		if err := start.Run(); err != nil {
			return fmt.Errorf("starting container: %w", err)
		}
		defer func() {
			stop := exec.Command(cfg.Runtime, "stop", containerName)
			stop.Stderr = os.Stderr
			if err := stop.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to stop room %q after describe: %v\n", nickname, err)
			}
		}()
	}

	u, err := user.Current()
	if err != nil {
		return err
	}
	workspace := u.HomeDir + "/workspace"

	prompt := describePrompt(describeVerbose)
	describeInvocation := strings.ReplaceAll(ai.DescribeCommand, "{}", shellQuote(prompt))
	script := fmt.Sprintf(
		`cd %s && { git diff main..HEAD; echo "---"; cat CHANGELOG* 2>/dev/null; } | %s`,
		shellQuote(workspace), describeInvocation,
	)

	genCmd := exec.Command(cfg.Runtime, "exec", "--user", u.Uid+":"+u.Gid, containerName, "bash", "-c", script)
	genCmd.Stderr = os.Stderr
	out, err := genCmd.Output()
	if err != nil {
		return fmt.Errorf("generating description: %w", err)
	}

	fmt.Println(strings.TrimSpace(string(out)))
	return nil
}

// describePrompt returns the natural-language instruction text for a given
// verbosity level: 0 (no -v) is a single line, 1 (-v) is a paragraph, and
// 2 or more (-vv, -vvv, ...) is a capped one-page detailed description.
func describePrompt(level int) string {
	switch {
	case level <= 0:
		return "In a single line, describe the purpose of this feature branch and how far along the work is."
	case level == 1:
		return "In one paragraph, describe: the purpose of this room, the working branch, the changes made so far, an indication of progress, and a readable summary of the current git status."
	default:
		return "Write a one-page detailed description of the work in this room: the tool stack in use, the state of testing, the approach taken, and the plan going forward."
	}
}

// shellQuote single-quotes s for safe interpolation into a shell one-liner,
// escaping any embedded single quotes. Needed because describe_command is
// config-supplied and the prompt text is substituted into it, unlike
// existing code that only interpolates trusted nicknames/paths.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
