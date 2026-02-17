package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/bcmister/cc/internal/config"
	"github.com/bcmister/cc/internal/monitor"
	"github.com/bcmister/cc/internal/ui"
	"github.com/bcmister/cc/internal/window"
	"github.com/spf13/cobra"
)

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Launch terminal windows across all monitors with per-window CLI selection",
	RunE:  runAll,
}

func runAll(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Load existing config for defaults (or start fresh)
	cfg, err := config.Load("")
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to load config: %w", err)
		}
		cfg = &config.Config{
			Version:      4,
			ProjectsRoot: config.DefaultProjectsRoot(),
		}
	}

	// Detect monitors
	monitors, err := monitor.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect monitors: %w", err)
	}

	ui.Logo("")
	ui.Sep()

	ui.Head(fmt.Sprintf("Detected %d monitors", len(monitors)))
	fmt.Println()
	for i, m := range monitors {
		badge := ""
		if m.Primary {
			badge = "Primary"
		}
		ui.BoxStart(fmt.Sprintf("Monitor %d", i+1), badge)
		ui.BoxRow(fmt.Sprintf("%s%d × %d%s", ui.BrWhite, m.Width, m.Height, ui.Reset))
		ui.BoxEnd()
	}

	// Grow or shrink monitor configs to match detected monitors
	for len(cfg.Monitors) < len(monitors) {
		cfg.Monitors = append(cfg.Monitors, config.MonitorConfig{
			Layout:  "full",
			Windows: []config.WindowConfig{{Tool: "cc"}},
		})
	}
	cfg.Monitors = cfg.Monitors[:len(monitors)]

	// Step 1: For each monitor, prompt window count
	for i := range monitors {
		defaultCount := cfg.Monitors[i].WindowCount()
		if defaultCount < 1 {
			defaultCount = 1
		}

		ui.Prompt(fmt.Sprintf("Windows on Monitor %d", i+1), strconv.Itoa(defaultCount))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		count := defaultCount
		if input != "" {
			if n, err := strconv.Atoi(input); err == nil && n >= 1 {
				count = n
			}
		}

		// Resize window configs to match new count
		existing := cfg.Monitors[i].Windows
		windows := make([]config.WindowConfig, count)
		for j := range windows {
			if j < len(existing) {
				windows[j] = existing[j]
			} else {
				windows[j] = config.WindowConfig{Tool: "cc"}
			}
		}
		cfg.Monitors[i].Windows = windows

		// Auto-set layout
		switch count {
		case 1:
			cfg.Monitors[i].Layout = "full"
		case 2:
			cfg.Monitors[i].Layout = "vertical"
		default:
			cfg.Monitors[i].Layout = "grid"
		}
	}

	// Step 2: For each window, prompt tool selection (cc or cx)
	fmt.Println()
	for i := range monitors {
		for j := range cfg.Monitors[i].Windows {
			defaultTool := cfg.Monitors[i].ToolFor(j)
			ui.Inline(fmt.Sprintf("Monitor %d, Window %d", i+1, j+1), defaultTool)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))
			if input == "cc" || input == "cx" {
				cfg.Monitors[i].Windows[j].Tool = input
			} else if input == "" {
				cfg.Monitors[i].Windows[j].Tool = defaultTool
			}
		}
	}

	// Validate selected tools (warn but don't block)
	tools := map[string]bool{}
	for _, mc := range cfg.Monitors {
		for _, wc := range mc.Windows {
			tools[wc.Tool] = true
		}
	}
	fmt.Println()
	for tool := range tools {
		if err := config.ValidateCommand(tool); err != nil {
			ui.Warn(err.Error())
		}
	}

	// Save config
	if err := config.Save(cfg, ""); err != nil {
		ui.Warn(fmt.Sprintf("Could not save config: %v", err))
	}

	// Launch
	return launchAllV3(cfg, monitors)
}

func launchAllV3(cfg *config.Config, monitors []monitor.Monitor) error {
	type monGroup struct {
		monIdx  int
		configs []window.LaunchConfig
	}
	var groups []monGroup
	var allConfigs []window.LaunchConfig

	for i, mc := range cfg.Monitors {
		if i >= len(monitors) {
			break
		}
		positions := window.CalculateLayout(&monitors[i], mc.WindowCount(), mc.Layout)
		g := monGroup{monIdx: i}
		for j, pos := range positions {
			tool := mc.ToolFor(j)
			lc := window.LaunchConfig{
				Title:      fmt.Sprintf("%s-%d-%d", tool, i+1, j+1),
				WorkingDir: cfg.ProjectsRoot,
				X:          pos.X,
				Y:          pos.Y,
				Width:      pos.Width,
				Height:     pos.Height,
				Command:    config.CommandFor(tool),
				Label:      config.LabelFor(tool),
				Profiles:   cfg.Profiles,
			}
			allConfigs = append(allConfigs, lc)
			g.configs = append(g.configs, lc)
		}
		groups = append(groups, g)
	}

	ui.Sep()

	// Use current terminal for first window, spawn others
	launchResult := window.LaunchAllWithCurrent(allConfigs)
	results := launchResult.Results

	newWindows := len(allConfigs) - 1
	if newWindows > 0 {
		ui.Head(fmt.Sprintf("Launching %d new terminals (using current for first)", newWindows))
	} else {
		ui.Head("Using current terminal")
	}
	fmt.Println()

	// Build result lookup
	resultMap := make(map[string]error)
	for _, r := range results {
		resultMap[r.Title] = r.Err
	}

	// Display per-monitor panels
	for _, g := range groups {
		badge := ""
		if monitors[g.monIdx].Primary {
			badge = "Primary"
		}
		ui.BoxStart(fmt.Sprintf("Monitor %d", g.monIdx+1), badge)
		for _, c := range g.configs {
			err := resultMap[c.Title]
			label := fmt.Sprintf("%s%s%s", ui.White, c.Title, ui.Reset)
			ui.BoxRow(fmt.Sprintf("%s  %s%s%s",
				label,
				statusColor(err == nil), statusIcon(err == nil), ui.Reset))
		}
		ui.BoxEnd()
	}

	// Print any warnings
	hasWarn := false
	for _, r := range results {
		if r.Err != nil {
			if !hasWarn {
				fmt.Println()
				hasWarn = true
			}
			ui.Warn(fmt.Sprintf("%s: %v", r.Title, r.Err))
		}
	}

	ui.Fin("Ready")
	fmt.Println()

	// Run picker in current terminal (blocking)
	return launchResult.RunPicker()
}

func statusIcon(ok bool) string {
	if ok {
		return ui.Check
	}
	return ui.Cross
}

func statusColor(ok bool) string {
	if ok {
		return ui.BrGreen
	}
	return ui.BrRed
}
