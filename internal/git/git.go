package git

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// RemoteOrigin returns the URL of the origin remote for the repo at dir.
func RemoteOrigin(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("git remote get-url origin: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// OwnerRepo parses a git remote URL and returns the owner and repository name.
// Handles both HTTPS (https://github.com/owner/repo.git) and
// SSH (git@github.com:owner/repo.git) formats.
func OwnerRepo(remoteURL string) (owner, repo string, err error) {
	if strings.HasPrefix(remoteURL, "git@") {
		return parseSSH(remoteURL)
	}
	return parseHTTPS(remoteURL)
}

func parseSSH(remote string) (owner, repo string, err error) {
	// git@github.com:owner/repo.git
	colon := strings.Index(remote, ":")
	if colon < 0 {
		return "", "", fmt.Errorf("cannot parse SSH remote URL: %s", remote)
	}
	path := strings.TrimSuffix(remote[colon+1:], ".git")
	segs := strings.Split(path, "/")
	if len(segs) < 2 {
		return "", "", fmt.Errorf("cannot parse SSH remote URL: %s", remote)
	}
	return segs[len(segs)-2], segs[len(segs)-1], nil
}

func parseHTTPS(remote string) (owner, repo string, err error) {
	u, err := url.Parse(remote)
	if err != nil {
		return "", "", fmt.Errorf("cannot parse remote URL %s: %w", remote, err)
	}
	path := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
	segs := strings.Split(path, "/")
	if len(segs) < 2 {
		return "", "", fmt.Errorf("cannot parse remote URL: %s", remote)
	}
	return segs[len(segs)-2], segs[len(segs)-1], nil
}
