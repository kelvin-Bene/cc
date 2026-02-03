package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/bcmister/qk/internal/config"
	"github.com/bcmister/qk/internal/monitor"
	"github.com/bcmister/qk/internal/ui"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Configure project folder and monitor layout",
	RunE:  runSet,
}

func runSet(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	existing, _ := config.Load("")

	defaultRoot := config.DefaultProjectsRoot()
	if existing != nil && existing.ProjectsRoot != "" {
		defaultRoot = existing.ProjectsRoot
	}

	ui.Logo("setup")
	ui.Sep()

	// --- Projects root ---
	ui.Prompt("Projects root", defaultRoot)
	projectsRoot, _ := reader.ReadString('\n')
	projectsRoot = strings.TrimSpace(projectsRoot)
	if projectsRoot == "" {
		projectsRoot = defaultRoot
	}

	if _, err := os.Stat(projectsRoot); os.IsNotExist(err) {
		fmt.Printf("   %sDirectory does not exist. Create it?%s %s[Y/n]%s  %s%s%s ",
			ui.White, ui.Reset, ui.DkGray, ui.Reset, ui.BrCyan, ui.Arrow, ui.Reset)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "" || answer == "y" || answer == "yes" {
			os.MkdirAll(projectsRoot, 0755)
			fmt.Printf("   %s%s Created%s\n", ui.BrGreen, ui.Check, ui.Reset)
		}
	}

	// --- Detect monitors ---
	ui.Head("Detecting monitors...")
	monitors, err := monitor.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect monitors: %w", err)
	}
	fmt.Printf("   %sfound %d%s\n", ui.BrWhite, len(monitors), ui.Reset)
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

	// --- Windows per monitor ---
	monitorConfigs := make([]config.MonitorConfig, len(monitors))
	for i := range monitors {
		defaultWindows := 1
		if existing != nil && i < len(existing.Monitors) {
			defaultWindows = existing.Monitors[i].Windows
		}

		ui.Prompt(fmt.Sprintf("Windows on Monitor %d", i+1), strconv.Itoa(defaultWindows))
		windowsStr, _ := reader.ReadString('\n')
		windowsStr = strings.TrimSpace(windowsStr)
		windows := defaultWindows
		if windowsStr != "" {
			w, err := strconv.Atoi(windowsStr)
			if err == nil && w >= 1 {
				windows = w
			}
		}

		layout := "full"
		if windows == 2 {
			layout = "vertical"
		} else if windows >= 3 {
			layout = "grid"
		}

		monitorConfigs[i] = config.MonitorConfig{
			Windows: windows,
			Layout:  layout,
		}
	}

	// --- Save ---
	cfg := &config.Config{
		Version:      2,
		ProjectsRoot: projectsRoot,
		Monitors:     monitorConfigs,
	}

	configPath := config.DefaultConfigPath()
	if err := config.Save(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Sep()
	ui.Ok("Configuration saved")
	fmt.Printf("   %s%s %s%s\n", ui.DkGray, ui.Arrow, configPath, ui.Reset)
	fmt.Println()
	fmt.Printf(" %sRun %sqk%s%s to launch.%s\n\n", ui.DkGray, ui.BrCyan, ui.Reset, ui.DkGray, ui.Reset)
	return nil
}
