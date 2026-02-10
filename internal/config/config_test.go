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

	if cfg.Version != 3 {
		t.Errorf("expected migrated version 3, got %d", cfg.Version)
	}

	if len(cfg.Monitors) != 1 {
		t.Fatalf("expected 1 monitor, got %d", len(cfg.Monitors))
	}

	if cfg.Monitors[0].WindowCount() != 3 {
		t.Errorf("expected 3 windows after migration, got %d", cfg.Monitors[0].WindowCount())
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

	if cfg.Version != 3 {
		t.Errorf("expected version 3, got %d", cfg.Version)
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
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &Config{
		Version:      3,
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

	if loaded.Version != 3 {
		t.Errorf("expected version 3, got %d", loaded.Version)
	}

	if loaded.Monitors[0].ToolFor(0) != "cx" {
		t.Errorf("expected window 0 tool cx, got %s", loaded.Monitors[0].ToolFor(0))
	}

	if loaded.Monitors[0].ToolFor(1) != "cc" {
		t.Errorf("expected window 1 tool cc, got %s", loaded.Monitors[0].ToolFor(1))
	}
}
