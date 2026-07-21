package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sfkleach/devroom/internal/config"
)

// containerEnterScriptPath is the fixed location inside the room container
// where the host's enter_script (if any) is bind-mounted.
const containerEnterScriptPath = "/etc/devroom/enter.sh"

// resolveHostScript resolves a configured script path against root (for a
// relative path, e.g. "scripts/build.sh") or home (for a leading "~/", or
// as the base for the default fallback when configured is empty).
// defaultRelPath names the file under ~/.config/devroom/ used when no path
// is configured at all. Returns "" if the resolved file does not exist, so
// callers can skip mounting/copying it entirely.
func resolveHostScript(configured, root, home, defaultRelPath string) string {
	path := configured
	switch {
	case path == "":
		path = filepath.Join(home, ".config", "devroom", defaultRelPath)
	case strings.HasPrefix(path, "~/"):
		path = filepath.Join(home, path[2:])
	case !filepath.IsAbs(path):
		path = filepath.Join(root, path)
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// resolveEnterScript returns the host-side path to the room's enter_script,
// defaulting to ~/.config/devroom/enter.sh when cfg.EnterScript is unset.
func resolveEnterScript(cfg *config.Config, root, home string) string {
	return resolveHostScript(cfg.EnterScript, root, home, "enter.sh")
}

// enterScriptMountArgs returns the podman/docker run args that bind-mount
// the room's enter_script read-only, or nil if none is configured/present.
func enterScriptMountArgs(cfg *config.Config, root, home string) []string {
	hostPath := resolveEnterScript(cfg, root, home)
	if hostPath == "" {
		return nil
	}
	return []string{"-v", hostPath + ":" + containerEnterScriptPath + ":ro"}
}

// expandHome expands a leading "~/" against home; other paths pass through
// unchanged (credential_paths are host paths, not repo-relative).
func expandHome(path, home string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// aiRunArgs returns the podman/docker run args that bind-mount each enabled
// [[ai]] entry's credential_paths (rw, same host path on both sides —
// mirrors the previous rw ~/.claude mount) and forward its env var names as
// bare "-e VARNAME" (docker/podman propagate the host's current value).
// Shared between enter.go's firstEntry and new.go's runNew so a room
// created either way ends up with identical AI credentials mounted.
//
// A credential_paths entry that doesn't exist on the host is skipped rather
// than passed through to a bind mount: some runtimes silently create an
// empty directory at a missing bind-mount source, which is wrong when the
// entry names a file (e.g. Claude Code's top-level ~/.claude.json, as
// distinct from its ~/.claude/ directory).
func aiRunArgs(cfg *config.Config, home string) []string {
	var args []string
	for _, entry := range cfg.AI {
		if !entry.IsEnabled() {
			continue
		}
		for _, p := range entry.CredentialPaths {
			hostPath := expandHome(p, home)
			if _, err := os.Stat(hostPath); err != nil {
				continue
			}
			args = append(args, "-v", hostPath+":"+hostPath)
		}
		for _, v := range entry.Env {
			args = append(args, "-e", v)
		}
	}
	return args
}
