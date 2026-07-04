package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// forge identifies which hosted git forge a repository's origin belongs to,
// so devroom knows which CLI tool and credential helper to use for HTTPS
// authentication instead of relying on SSH keys/agents.
type forge int

const (
	forgeUnknown forge = iota
	forgeGitHub
	forgeGitLab
)

// detectForge classifies a remote host, matching the heuristic already
// documented in docs/devroom-proposal.md: github.com is GitHub; any other
// host containing "gitlab" (self-hosted instances included) is GitLab.
func detectForge(host string) forge {
	switch {
	case host == "github.com":
		return forgeGitHub
	case strings.Contains(host, "gitlab"):
		return forgeGitLab
	default:
		return forgeUnknown
	}
}

// name returns the CLI tool name for the forge (e.g. "gh", "glab").
func (f forge) name() string {
	switch f {
	case forgeGitHub:
		return "gh"
	case forgeGitLab:
		return "glab"
	default:
		return ""
	}
}

// credentialHelper returns the git credential helper command for the forge.
func (f forge) credentialHelper() string {
	switch f {
	case forgeGitHub:
		return "!gh auth git-credential"
	case forgeGitLab:
		return "!glab auth git-credential"
	default:
		return ""
	}
}

// token retrieves a usable access token for the forge from its CLI tool on
// the host. It fails clearly rather than falling back silently to SSH, since
// a silent fallback was the source of confusing, hard-to-diagnose failures.
func (f forge) token() (string, error) {
	switch f {
	case forgeGitHub:
		return ghToken()
	case forgeGitLab:
		return glabToken()
	default:
		return "", fmt.Errorf("unrecognised git forge host; only github.com and gitlab hosts are supported")
	}
}

// ghToken shells out to 'gh auth token', which resolves the active token
// regardless of whether gh stores it in its config file or the OS keyring.
func ghToken() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("gh auth token: %w (is 'gh' installed and are you logged in? run 'gh auth login')", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("'gh auth token' returned an empty token; run 'gh auth login'")
	}
	return token, nil
}

// glabTokenPattern extracts the token value from the human-oriented output of
// 'glab auth status --show-token'. glab has no dedicated machine-readable
// token command (unlike gh's 'gh auth token'), so this is best-effort and may
// need updating if glab's output format changes.
var glabTokenPattern = regexp.MustCompile(`Token:\s*(\S+)`)

func glabToken() (string, error) {
	out, err := exec.Command("glab", "auth", "status", "--show-token").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("glab auth status: %w (is 'glab' installed and are you logged in? run 'glab auth login')", err)
	}
	if bytes.Contains(out, []byte("Invalid token")) {
		return "", fmt.Errorf("glab reports the stored token is invalid; run 'glab auth login' to refresh it")
	}
	match := glabTokenPattern.FindSubmatch(out)
	if match == nil {
		return "", fmt.Errorf("could not find a token in 'glab auth status --show-token' output; run 'glab auth login'")
	}
	return string(match[1]), nil
}

// loginScript returns a bash script that reads a token from stdin, uses it
// to log the forge CLI in (persisted inside the container's own filesystem,
// not a host bind mount), and configures git to use that CLI as the
// credential helper for the given host. It must be run with $HOME/.gitconfig
// already writable — see new.go/enter.go for why the host's ~/.gitconfig is
// copied to a container-local file rather than bind-mounted directly there.
func loginScript(f forge, host string) string {
	switch f {
	case forgeGitHub:
		return fmt.Sprintf(
			`read -r DEVROOM_FORGE_TOKEN
gh auth login --with-token --insecure-storage --git-protocol https --hostname %s <<< "$DEVROOM_FORGE_TOKEN"
git config --global credential.%q.helper %q
`, host, "https://"+host, f.credentialHelper())
	case forgeGitLab:
		return fmt.Sprintf(
			`read -r DEVROOM_FORGE_TOKEN
glab auth login --stdin --hostname %s <<< "$DEVROOM_FORGE_TOKEN"
git config --global credential.%q.helper %q
`, host, "https://"+host, f.credentialHelper())
	default:
		return ""
	}
}
