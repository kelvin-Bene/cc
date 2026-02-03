package cmd

import (
	"fmt"
	"os"

	"github.com/bcmister/qk/internal/config"
	"github.com/bcmister/qk/internal/ui"
	"github.com/bcmister/qk/internal/window"
	"github.com/spf13/cobra"
)

var tabCmd = &cobra.Command{
	Use:   "tab",
	Short: "Open a new terminal tab in the current window",
	RunE:  runTab,
}

func runTab(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		if os.IsNotExist(err) {
			cfg = &config.Config{
				ProjectsRoot: config.DefaultProjectsRoot(),
			}
		} else {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	fmt.Printf("\n %s%s%s %sOpening new tab...%s\n\n",
		ui.BrCyan, ui.Diamond, ui.Reset, ui.BrWhite, ui.Reset)

	return window.LaunchTab(cfg.ProjectsRoot, config.Command)
}
