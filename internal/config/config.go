package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const Command = "claude --dangerously-skip-permissions"
const CodexCommand = "codex --full-auto"

func CommandFor(bin string) string {
	if bin == "cx" {
		return CodexCommand
	}
	return Command // "cc" and "all" both default to claude
}

func LabelFor(bin string) string {
	switch bin {
	case "cx":
		return "cx"
	case "all":
		return "all"
	default:
		return "cc"
	}
}

// ValidateCommand checks if the underlying tool for a given binary name is on PATH.
func ValidateCommand(tool string) error {
	switch tool {
	case "cx":
		_, err := exec.LookPath("codex")
		if err != nil {
			return fmt.Errorf("codex not found on PATH — install: npm i -g @openai/codex")
		}
	default: // "cc", "all"
		_, err := exec.LookPath("claude")
		if err != nil {
			return fmt.Errorf("claude not found on PATH — install: npm i -g @anthropic-ai/claude-code")
		}
	}
	return nil
}

// Profile represents a named Claude Code account profile
type Profile struct {
	Name      string `yaml:"name"`
	ConfigDir string `yaml:"configDir"`
	APIKey    string `yaml:"apiKey,omitempty"`
}

// WindowConfig represents configuration for a single window within a monitor
type WindowConfig struct {
	Tool string `yaml:"tool"` // "cc" or "cx"
}

// Config represents the application configuration (v4)
type Config struct {
	Version      int             `yaml:"version"`
	ProjectsRoot string          `yaml:"projectsRoot"`
	Profiles     []Profile       `yaml:"profiles,omitempty"`
	Monitors     []MonitorConfig `yaml:"monitors"`
}

// HasProfiles returns true if the config has more than one profile
func (c *Config) HasProfiles() bool {
	return len(c.Profiles) > 1
}

// ExpandPath expands a leading ~ to the user's home directory
func ExpandPath(p string) string {
	if len(p) == 0 {
		return p
	}
	if p[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[1:])
	}
	return p
}

// MonitorConfig represents configuration for a single monitor
type MonitorConfig struct {
	Layout  string         `yaml:"layout"`
	Windows []WindowConfig `yaml:"windows"`
}

// WindowCount returns the number of windows configured for this monitor
func (mc *MonitorConfig) WindowCount() int {
	return len(mc.Windows)
}

// ToolFor returns the tool name for the window at index idx, defaulting to "cc"
func (mc *MonitorConfig) ToolFor(idx int) string {
	if idx >= 0 && idx < len(mc.Windows) && mc.Windows[idx].Tool != "" {
		return mc.Windows[idx].Tool
	}
	return "cc"
}

// v2Config is the old format used for migration
type v2Config struct {
	Version      int              `yaml:"version"`
	ProjectsRoot string           `yaml:"projectsRoot"`
	Monitors     []v2MonitorConfig `yaml:"monitors"`
}

type v2MonitorConfig struct {
	Windows int    `yaml:"windows"`
	Layout  string `yaml:"layout"`
}

// DefaultConfigPath returns the default configuration file path
func DefaultConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".cc", "config.yaml")
}

// DefaultProjectsRoot returns the default projects directory
func DefaultProjectsRoot() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".1dev")
}

// upgradeToV4 adds a default profile if none exist and sets version to 4
func upgradeToV4(cfg *Config) *Config {
	if len(cfg.Profiles) == 0 {
		home, _ := os.UserHomeDir()
		cfg.Profiles = []Profile{
			{Name: "Default", ConfigDir: filepath.Join(home, ".claude")},
		}
	}
	cfg.Version = 4
	return cfg
}

// Load reads the configuration from a file, auto-migrating v2→v3→v4 in memory
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Peek at version to decide how to unmarshal
	var peek struct {
		Version int `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &peek); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	var cfg *Config
	if peek.Version < 3 {
		// v2 or unversioned — migrate to v3
		cfg, err = migrateV2(data)
		if err != nil {
			return nil, err
		}
	} else {
		cfg = &Config{}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}
	}

	// Upgrade to v4 if needed
	if cfg.Version < 4 {
		cfg = upgradeToV4(cfg)
	}

	return cfg, nil
}

// migrateV2 converts a v2 config (with Windows as int) to v3 (with []WindowConfig)
func migrateV2(data []byte) (*Config, error) {
	var old v2Config
	if err := yaml.Unmarshal(data, &old); err != nil {
		return nil, fmt.Errorf("failed to parse v2 config: %w", err)
	}

	cfg := &Config{
		Version:      3,
		ProjectsRoot: old.ProjectsRoot,
		Monitors:     make([]MonitorConfig, len(old.Monitors)),
	}

	for i, om := range old.Monitors {
		count := om.Windows
		if count < 1 {
			count = 1
		}
		windows := make([]WindowConfig, count)
		for j := range windows {
			windows[j] = WindowConfig{Tool: "cc"}
		}
		cfg.Monitors[i] = MonitorConfig{
			Layout:  om.Layout,
			Windows: windows,
		}
	}

	return cfg, nil
}

// Save writes the configuration to a file
func Save(cfg *Config, path string) error {
	if path == "" {
		path = DefaultConfigPath()
	}

	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	cfg.Version = 4

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
