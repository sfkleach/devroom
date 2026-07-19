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

// Config holds the resolved devroom configuration.
type Config struct {
	Runtime         string `toml:"runtime"`
	BaseImage       string `toml:"base_image"`
	BuildScript     string `toml:"build_script"`
	SummaryModel    string `toml:"summary_model"`
	EnterScript     string `toml:"enter_script"`
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
