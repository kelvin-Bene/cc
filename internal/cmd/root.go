package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bcmister/quickstart/internal/config"
	"github.com/bcmister/quickstart/internal/monitor"
	"github.com/bcmister/quickstart/internal/window"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "quickstart [profile]",
	Short: "Launch multiple terminal windows across monitors for development",
	Long: `Quickstart is a CLI tool that launches multiple terminal windows
across your monitors, each with an interactive project picker that
starts Claude Code in your selected project.

Example:
  quickstart           # Launch with default profile
  quickstart dev       # Launch with 'dev' profile
  quickstart --init    # Set up configuration`,
	Args: cobra.MaximumNArgs(1),
	RunE: runQuickstart,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ~/.quickstart/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(monitorsCmd)
	rootCmd.AddCommand(versionCmd)
}

func runQuickstart(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w\nRun 'quickstart init' to create a configuration", err)
	}

	// Get profile name (default or specified)
	profileName := "default"
	if len(args) > 0 {
		profileName = args[0]
	}

	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile '%s' not found in config", profileName)
	}

	// Detect monitors
	monitors, err := monitor.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect monitors: %w", err)
	}

	if verbose {
		fmt.Printf("Detected %d monitors:\n", len(monitors))
		for i, m := range monitors {
			fmt.Printf("  %d: %s (%dx%d at %d,%d)\n", i+1, m.Name, m.Width, m.Height, m.X, m.Y)
		}
	}

	// Get the picker script path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	scriptsDir := filepath.Join(filepath.Dir(execPath), "scripts")
	pickerScript := filepath.Join(scriptsDir, "picker.ps1")

	// Check if picker script exists, if not use the one from config dir
	if _, err := os.Stat(pickerScript); os.IsNotExist(err) {
		homeDir, _ := os.UserHomeDir()
		pickerScript = filepath.Join(homeDir, ".quickstart", "scripts", "picker.ps1")
	}

	// Launch terminals on each configured monitor
	totalWindows := 0
	for _, monitorConfig := range profile.Monitors {
		// Find the monitor by name or index
		var targetMonitor *monitor.Monitor
		for i := range monitors {
			if monitors[i].Name == monitorConfig.Name || fmt.Sprintf("%d", i+1) == monitorConfig.Name {
				targetMonitor = &monitors[i]
				break
			}
		}

		if targetMonitor == nil {
			fmt.Printf("Warning: Monitor '%s' not found, skipping\n", monitorConfig.Name)
			continue
		}

		// Calculate window positions based on layout
		positions := window.CalculateLayout(targetMonitor, monitorConfig.Windows, monitorConfig.Layout)

		// Launch terminals
		for i, pos := range positions {
			windowTitle := fmt.Sprintf("Quickstart-%s-%d", monitorConfig.Name, i+1)

			err := window.LaunchTerminal(window.LaunchConfig{
				Title:         windowTitle,
				WorkingDir:    profile.ProjectsDir,
				X:             pos.X,
				Y:             pos.Y,
				Width:         pos.Width,
				Height:        pos.Height,
				Command:       pickerScript,
				PostCommand:   profile.PostSelectCommand,
			})

			if err != nil {
				fmt.Printf("Warning: Failed to launch terminal %d on %s: %v\n", i+1, monitorConfig.Name, err)
				continue
			}

			totalWindows++
			if verbose {
				fmt.Printf("Launched terminal %d on %s at (%d,%d)\n", i+1, monitorConfig.Name, pos.X, pos.Y)
			}
		}
	}

	fmt.Printf("Launched %d terminal windows\n", totalWindows)
	return nil
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize quickstart configuration",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	return config.RunInitWizard()
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
		fmt.Printf("    Name:       %s\n", m.Name)
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
		fmt.Println("quickstart v0.1.0")
	},
}
