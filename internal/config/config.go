package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ErrNoConfig is returned when no configuration file is found at any level.
var ErrNoConfig = errors.New("no configuration file found")

// AIEntry describes one named AI CLI integration: how to install it into
// the shared base image, what host credentials to mount into every room,
// and how to invoke it non-interactively to generate a room description.
type AIEntry struct {
	Name            string   `toml:"name"`
	Enabled         *bool    `toml:"enabled"` // nil means default true; distinguishes omitted from explicit false
	InstallCommand  string   `toml:"install_command"`
	CredentialPaths []string `toml:"credential_paths"`
	DescribeCommand string   `toml:"describe_command"`
	Env             []string `toml:"env"`
}

// IsEnabled reports whether this entry should be installed into the base
// image and mounted into rooms. Enabled defaults to true when omitted.
func (e AIEntry) IsEnabled() bool {
	return e.Enabled == nil || *e.Enabled
}

// Config holds the resolved devroom configuration.
type Config struct {
	Runtime     string `toml:"runtime"`
	BaseImage   string `toml:"base_image"`
	BuildScript string `toml:"build_script"`
	EnterScript string `toml:"enter_script"`
	LeaveScript string `toml:"leave_script"`
	AIDefault   string `toml:"ai_default"`
	// A file that declares [[ai]] replaces the whole slice rather than
	// merging per-entry with a lower-priority XDG level's [[ai]] list.
	AI []AIEntry `toml:"ai"`
}

// ResolveDefaultAI resolves AIDefault to the matching enabled AIEntry in AI.
func (c *Config) ResolveDefaultAI() (*AIEntry, error) {
	if c.AIDefault == "" {
		return nil, fmt.Errorf("missing required config key 'ai_default' in devroom.toml")
	}
	for i := range c.AI {
		if c.AI[i].Name == c.AIDefault {
			if !c.AI[i].IsEnabled() {
				return nil, fmt.Errorf("ai_default %q names a disabled [[ai]] entry", c.AIDefault)
			}
			return &c.AI[i], nil
		}
	}
	return nil, fmt.Errorf("ai_default %q does not match any configured [[ai]] entry", c.AIDefault)
}

// Load reads and merges configuration from the three XDG levels:
// system-wide < user-wide < per-repo. Each level only overrides keys it
// explicitly sets. Returns ErrNoConfig if no file exists at any level.
func Load(repoRoot string) (*Config, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	found := false
	for _, p := range paths(absRoot) {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if _, err := toml.DecodeFile(p, cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", p, err)
		}
		found = true
	}
	if !found {
		return nil, ErrNoConfig
	}
	return cfg, nil
}

func paths(repoRoot string) []string {
	home, _ := os.UserHomeDir()
	return []string{
		"/etc/xdg/devroom/devroom.toml",
		filepath.Join(home, ".config", "devroom", "devroom.toml"),
		filepath.Join(repoRoot, ".config", "devroom", "devroom.toml"),
	}
}
