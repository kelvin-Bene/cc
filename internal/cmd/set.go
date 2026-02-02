package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bcmister/qk/internal/config"
	"github.com/bcmister/qk/internal/monitor"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Configure project folder and monitor layout",
	RunE:  runSet,
}

func runSet(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Load existing config for defaults
	existing, _ := config.Load("")

	// Determine defaults
	homeDir, _ := os.UserHomeDir()
	defaultRoot := filepath.Join(homeDir, ".1dev")
	defaultCommand := "claude --dangerously-skip-permissions"
	if existing != nil {
		if existing.ProjectsRoot != "" {
			defaultRoot = existing.ProjectsRoot
		}
		if existing.Command != "" {
			defaultCommand = existing.Command
		}
	}

	// Ask for projects root
	fmt.Printf("Projects root [%s]: ", defaultRoot)
	projectsRoot, _ := reader.ReadString('\n')
	projectsRoot = strings.TrimSpace(projectsRoot)
	if projectsRoot == "" {
		projectsRoot = defaultRoot
	}

	// Validate directory exists
	if _, err := os.Stat(projectsRoot); os.IsNotExist(err) {
		fmt.Printf("Directory '%s' does not exist. Create it? [Y/n]: ", projectsRoot)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "" || answer == "y" || answer == "yes" {
			os.MkdirAll(projectsRoot, 0755)
			fmt.Println("Created.")
		}
	}

	// Ask for command
	fmt.Printf("Command [%s]: ", defaultCommand)
	command, _ := reader.ReadString('\n')
	command = strings.TrimSpace(command)
	if command == "" {
		command = defaultCommand
	}

	// Detect monitors
	fmt.Println()
	fmt.Print("Detecting monitors...")
	monitors, err := monitor.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect monitors: %w", err)
	}
	fmt.Printf(" found %d\n", len(monitors))

	for i, m := range monitors {
		primary := ""
		if m.Primary {
			primary = " (Primary)"
		}
		fmt.Printf("  Monitor %d: %dx%d%s\n", i+1, m.Width, m.Height, primary)
	}
	fmt.Println()

	// Configure windows per monitor
	monitorConfigs := make([]config.MonitorConfig, len(monitors))
	for i := range monitors {
		defaultWindows := 1
		if existing != nil && i < len(existing.Monitors) {
			defaultWindows = existing.Monitors[i].Windows
		}

		fmt.Printf("Windows on Monitor %d [%d]: ", i+1, defaultWindows)
		windowsStr, _ := reader.ReadString('\n')
		windowsStr = strings.TrimSpace(windowsStr)
		windows := defaultWindows
		if windowsStr != "" {
			w, err := strconv.Atoi(windowsStr)
			if err == nil && w >= 1 {
				windows = w
			}
		}

		// Auto-assign layout
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

	// Build and save config
	cfg := &config.Config{
		Version:      2,
		ProjectsRoot: projectsRoot,
		Command:      command,
		Monitors:     monitorConfigs,
	}

	configPath := config.DefaultConfigPath()
	if err := config.Save(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\nSaved to %s\n", configPath)
	fmt.Println("Run 'qk' to launch.")
	return nil
}
