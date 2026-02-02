package cmd

import (
	"fmt"
	"os"

	"github.com/bcmister/qk/internal/config"
	"github.com/bcmister/qk/internal/monitor"
	"github.com/bcmister/qk/internal/window"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "qk",
	Short: "Launch terminal windows across monitors with project picker",
	RunE:  runQk,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(monitorsCmd)
	rootCmd.AddCommand(versionCmd)
}

func runQk(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No config found. Run 'qk set' to get started.")
			return nil
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	monitors, err := monitor.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect monitors: %w", err)
	}

	totalWindows := 0
	for _, mc := range cfg.Monitors {
		totalWindows += mc.Windows
	}
	fmt.Printf("Launching %d terminals across %d monitors...", totalWindows, len(cfg.Monitors))

	launched := 0
	for i, mc := range cfg.Monitors {
		if i >= len(monitors) {
			fmt.Printf("\nWarning: Monitor %d not found, skipping\n", i+1)
			continue
		}

		targetMonitor := &monitors[i]
		positions := window.CalculateLayout(targetMonitor, mc.Windows, mc.Layout)

		for j, pos := range positions {
			title := fmt.Sprintf("qk-%d-%d", i+1, j+1)

			err := window.LaunchTerminal(window.LaunchConfig{
				Title:      title,
				WorkingDir: cfg.ProjectsRoot,
				X:          pos.X,
				Y:          pos.Y,
				Width:      pos.Width,
				Height:     pos.Height,
				Command:    cfg.Command,
			})
			if err != nil {
				fmt.Printf("\nWarning: Failed to launch terminal %d on monitor %d: %v\n", j+1, i+1, err)
				continue
			}
			launched++
		}
	}

	fmt.Println("done.")
	return nil
}

var monitorsCmd = &cobra.Command{
	Use:   "monitors",
	Short: "List detected monitors",
	RunE:  runMonitors,
}

func runMonitors(cmd *cobra.Command, args []string) error {
	monitors, err := monitor.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect monitors: %w", err)
	}

	fmt.Printf("Detected %d monitors:\n\n", len(monitors))
	for i, m := range monitors {
		primary := ""
		if m.Primary {
			primary = " (Primary)"
		}
		fmt.Printf("  Monitor %d%s:\n", i+1, primary)
		fmt.Printf("    Resolution: %dx%d\n", m.Width, m.Height)
		fmt.Printf("    Position:   (%d, %d)\n", m.X, m.Y)
		fmt.Println()
	}
	return nil
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("qk v0.2.0")
	},
}
