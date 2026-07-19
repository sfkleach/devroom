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
