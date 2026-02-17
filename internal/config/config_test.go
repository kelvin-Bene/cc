package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateV2toV3(t *testing.T) {
	v2yaml := []byte(`version: 2
projectsRoot: /home/test/.1dev
monitors:
  - windows: 2
    layout: vertical
  - windows: 1
    layout: full
`)

	cfg, err := migrateV2(v2yaml)
	if err != nil {
		t.Fatalf("migrateV2 failed: %v", err)
	}

	if cfg.Version != 3 {
		t.Errorf("expected version 3, got %d", cfg.Version)
	}

	if cfg.ProjectsRoot != "/home/test/.1dev" {
		t.Errorf("expected projectsRoot /home/test/.1dev, got %s", cfg.ProjectsRoot)
	}

	if len(cfg.Monitors) != 2 {
		t.Fatalf("expected 2 monitors, got %d", len(cfg.Monitors))
	}

	// Monitor 0: 2 windows, vertical layout
	if cfg.Monitors[0].WindowCount() != 2 {
		t.Errorf("monitor 0: expected 2 windows, got %d", cfg.Monitors[0].WindowCount())
	}
	if cfg.Monitors[0].Layout != "vertical" {
		t.Errorf("monitor 0: expected layout vertical, got %s", cfg.Monitors[0].Layout)
	}
	for j := 0; j < 2; j++ {
		if cfg.Monitors[0].Windows[j].Tool != "cc" {
			t.Errorf("monitor 0, window %d: expected tool cc, got %s", j, cfg.Monitors[0].Windows[j].Tool)
		}
	}

	// Monitor 1: 1 window, full layout
	if cfg.Monitors[1].WindowCount() != 1 {
		t.Errorf("monitor 1: expected 1 window, got %d", cfg.Monitors[1].WindowCount())
	}
	if cfg.Monitors[1].Layout != "full" {
		t.Errorf("monitor 1: expected layout full, got %s", cfg.Monitors[1].Layout)
	}
}

func TestWindowCountAndToolFor(t *testing.T) {
	mc := MonitorConfig{
		Layout: "vertical",
		Windows: []WindowConfig{
			{Tool: "cc"},
			{Tool: "cx"},
			{Tool: "cc"},
		},
	}

	if mc.WindowCount() != 3 {
		t.Errorf("expected WindowCount 3, got %d", mc.WindowCount())
	}

	tests := []struct {
		idx      int
		expected string
	}{
		{0, "cc"},
		{1, "cx"},
		{2, "cc"},
		{3, "cc"},  // out of bounds — default
		{-1, "cc"}, // negative — default
	}

	for _, tt := range tests {
		got := mc.ToolFor(tt.idx)
		if got != tt.expected {
			t.Errorf("ToolFor(%d): expected %s, got %s", tt.idx, tt.expected, got)
		}
	}
}

func TestCommandForAll(t *testing.T) {
	// "all" should map to the default claude command, same as "cc"
	if CommandFor("all") != Command {
		t.Errorf("CommandFor('all') = %s, want %s", CommandFor("all"), Command)
	}
	if CommandFor("cc") != Command {
		t.Errorf("CommandFor('cc') = %s, want %s", CommandFor("cc"), Command)
	}
	if CommandFor("cx") != CodexCommand {
		t.Errorf("CommandFor('cx') = %s, want %s", CommandFor("cx"), CodexCommand)
	}
}

func TestLabelFor(t *testing.T) {
	tests := []struct {
		bin      string
		expected string
	}{
		{"cc", "cc"},
		{"cx", "cx"},
		{"all", "all"},
		{"unknown", "cc"},
	}

	for _, tt := range tests {
		got := LabelFor(tt.bin)
		if got != tt.expected {
			t.Errorf("LabelFor(%q) = %s, want %s", tt.bin, got, tt.expected)
		}
	}
}

func TestLoadV2AutoMigrates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	v2yaml := []byte(`version: 2
projectsRoot: /test
monitors:
  - windows: 3
    layout: grid
`)

	if err := os.WriteFile(path, v2yaml, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// v2 now migrates all the way to v4
	if cfg.Version != 4 {
		t.Errorf("expected migrated version 4, got %d", cfg.Version)
	}

	if len(cfg.Monitors) != 1 {
		t.Fatalf("expected 1 monitor, got %d", len(cfg.Monitors))
	}

	if cfg.Monitors[0].WindowCount() != 3 {
		t.Errorf("expected 3 windows after migration, got %d", cfg.Monitors[0].WindowCount())
	}

	// Should have default profile
	if len(cfg.Profiles) != 1 {
		t.Fatalf("expected 1 default profile, got %d", len(cfg.Profiles))
	}
	if cfg.Profiles[0].Name != "Default" {
		t.Errorf("expected default profile name 'Default', got %s", cfg.Profiles[0].Name)
	}
}

func TestLoadV3Direct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	v3yaml := []byte(`version: 3
projectsRoot: /test
monitors:
  - layout: vertical
    windows:
      - tool: cc
      - tool: cx
`)

	if err := os.WriteFile(path, v3yaml, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// v3 auto-upgrades to v4
	if cfg.Version != 4 {
		t.Errorf("expected version 4 (auto-upgraded), got %d", cfg.Version)
	}

	if cfg.Monitors[0].WindowCount() != 2 {
		t.Errorf("expected 2 windows, got %d", cfg.Monitors[0].WindowCount())
	}

	if cfg.Monitors[0].ToolFor(0) != "cc" {
		t.Errorf("expected window 0 tool cc, got %s", cfg.Monitors[0].ToolFor(0))
	}

	if cfg.Monitors[0].ToolFor(1) != "cx" {
		t.Errorf("expected window 1 tool cx, got %s", cfg.Monitors[0].ToolFor(1))
	}

	// Should have default profile after upgrade
	if len(cfg.Profiles) != 1 {
		t.Fatalf("expected 1 default profile, got %d", len(cfg.Profiles))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &Config{
		Version:      4,
		ProjectsRoot: "/test/projects",
		Monitors: []MonitorConfig{
			{
				Layout: "vertical",
				Windows: []WindowConfig{
					{Tool: "cx"},
					{Tool: "cc"},
				},
			},
		},
	}

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Version != 4 {
		t.Errorf("expected version 4, got %d", loaded.Version)
	}

	if loaded.Monitors[0].ToolFor(0) != "cx" {
		t.Errorf("expected window 0 tool cx, got %s", loaded.Monitors[0].ToolFor(0))
	}

	if loaded.Monitors[0].ToolFor(1) != "cc" {
		t.Errorf("expected window 1 tool cc, got %s", loaded.Monitors[0].ToolFor(1))
	}
}

func TestUpgradeToV4(t *testing.T) {
	cfg := &Config{
		Version:      3,
		ProjectsRoot: "/test",
		Monitors: []MonitorConfig{
			{Layout: "full", Windows: []WindowConfig{{Tool: "cc"}}},
		},
	}

	upgraded := upgradeToV4(cfg)

	if upgraded.Version != 4 {
		t.Errorf("expected version 4, got %d", upgraded.Version)
	}

	if len(upgraded.Profiles) != 1 {
		t.Fatalf("expected 1 default profile, got %d", len(upgraded.Profiles))
	}

	if upgraded.Profiles[0].Name != "Default" {
		t.Errorf("expected profile name 'Default', got %s", upgraded.Profiles[0].Name)
	}

	// ConfigDir should end with .claude
	if !filepath.IsAbs(upgraded.Profiles[0].ConfigDir) {
		t.Errorf("expected absolute configDir, got %s", upgraded.Profiles[0].ConfigDir)
	}
}

func TestLoadV4Direct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	v4yaml := []byte(`version: 4
projectsRoot: /test
profiles:
  - name: Personal
    configDir: ~/.claude-personal
  - name: Work
    configDir: ~/.claude-work
monitors:
  - layout: grid
    windows:
      - tool: cc
`)

	if err := os.WriteFile(path, v4yaml, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Version != 4 {
		t.Errorf("expected version 4, got %d", cfg.Version)
	}

	if len(cfg.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(cfg.Profiles))
	}

	if cfg.Profiles[0].Name != "Personal" {
		t.Errorf("expected profile 0 name 'Personal', got %s", cfg.Profiles[0].Name)
	}
	if cfg.Profiles[1].Name != "Work" {
		t.Errorf("expected profile 1 name 'Work', got %s", cfg.Profiles[1].Name)
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"~/.claude", filepath.Join(home, ".claude")},
		{"~/.claude-work", filepath.Join(home, ".claude-work")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}

	for _, tt := range tests {
		got := ExpandPath(tt.input)
		if got != tt.expected {
			t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestHasProfiles(t *testing.T) {
	tests := []struct {
		name     string
		profiles []Profile
		expected bool
	}{
		{"no profiles", nil, false},
		{"one profile", []Profile{{Name: "Default"}}, false},
		{"two profiles", []Profile{{Name: "A"}, {Name: "B"}}, true},
		{"three profiles", []Profile{{Name: "A"}, {Name: "B"}, {Name: "C"}}, true},
	}

	for _, tt := range tests {
		cfg := &Config{Profiles: tt.profiles}
		got := cfg.HasProfiles()
		if got != tt.expected {
			t.Errorf("HasProfiles() with %s: got %v, want %v", tt.name, got, tt.expected)
		}
	}
}

func TestSaveV4RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &Config{
		Version:      4,
		ProjectsRoot: "/test/projects",
		Profiles: []Profile{
			{Name: "Personal", ConfigDir: "~/.claude-personal"},
			{Name: "Work", ConfigDir: "~/.claude-work", APIKey: "sk-test-123"},
		},
		Monitors: []MonitorConfig{
			{
				Layout: "grid",
				Windows: []WindowConfig{
					{Tool: "cc"},
				},
			},
		},
	}

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Version != 4 {
		t.Errorf("expected version 4, got %d", loaded.Version)
	}

	if len(loaded.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(loaded.Profiles))
	}

	if loaded.Profiles[0].Name != "Personal" {
		t.Errorf("expected profile 0 name 'Personal', got %s", loaded.Profiles[0].Name)
	}
	if loaded.Profiles[0].ConfigDir != "~/.claude-personal" {
		t.Errorf("expected profile 0 configDir '~/.claude-personal', got %s", loaded.Profiles[0].ConfigDir)
	}

	if loaded.Profiles[1].Name != "Work" {
		t.Errorf("expected profile 1 name 'Work', got %s", loaded.Profiles[1].Name)
	}
	if loaded.Profiles[1].APIKey != "sk-test-123" {
		t.Errorf("expected profile 1 apiKey 'sk-test-123', got %s", loaded.Profiles[1].APIKey)
	}
}
