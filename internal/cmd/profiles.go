package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bcmister/cc/internal/config"
	"github.com/bcmister/cc/internal/ui"
	"github.com/spf13/cobra"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage account profiles",
	RunE:  runProfilesList,
}

var profilesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE:  runProfilesList,
}

var profilesAddCmd = &cobra.Command{
	Use:   "add [name] [configDir]",
	Short: "Add a new profile",
	RunE:  runProfilesAdd,
}

var profilesRemoveCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove a profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfilesRemove,
}

func init() {
	profilesCmd.AddCommand(profilesListCmd)
	profilesCmd.AddCommand(profilesAddCmd)
	profilesCmd.AddCommand(profilesRemoveCmd)
}

func runProfilesList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("\n %sNo config found. Run %scc set%s%s to initialize.%s\n\n",
				ui.DkGray, ui.BrCyan, ui.Reset, ui.DkGray, ui.Reset)
			return nil
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Profiles) == 0 {
		fmt.Printf("\n %sNo profiles configured.%s\n\n", ui.DkGray, ui.Reset)
		return nil
	}

	ui.Head(fmt.Sprintf("%d profile(s)", len(cfg.Profiles)))
	fmt.Println()

	for i, p := range cfg.Profiles {
		expandedDir := config.ExpandPath(p.ConfigDir)
		credPath := filepath.Join(expandedDir, ".credentials.json")
		authStatus := fmt.Sprintf("%s%s not authenticated%s", ui.BrRed, ui.Cross, ui.Reset)
		if _, err := os.Stat(credPath); err == nil {
			authStatus = fmt.Sprintf("%s%s authenticated%s", ui.BrGreen, ui.Check, ui.Reset)
		}

		ui.BoxStart(fmt.Sprintf("%d. %s", i+1, p.Name), "")
		ui.BoxRow(fmt.Sprintf("%sDir%s    %s%s%s", ui.DkGray, ui.Reset, ui.White, p.ConfigDir, ui.Reset))
		ui.BoxRow(fmt.Sprintf("%sAuth%s   %s", ui.DkGray, ui.Reset, authStatus))
		if p.APIKey != "" {
			preview := p.APIKey
			if len(preview) > 8 {
				preview = preview[:8]
			}
			ui.BoxRow(fmt.Sprintf("%sAPI%s    %s%s...%s", ui.DkGray, ui.Reset, ui.White, preview, ui.Reset))
		}
		ui.BoxEnd()
	}

	fmt.Println()
	return nil
}

func runProfilesAdd(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		if os.IsNotExist(err) {
			cfg = &config.Config{
				Version:      4,
				ProjectsRoot: config.DefaultProjectsRoot(),
			}
		} else {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	reader := bufio.NewReader(os.Stdin)

	var name, configDir string

	if len(args) >= 1 {
		name = args[0]
	} else {
		fmt.Printf("\n %s%s%s %sProfile name%s\n   %s%s%s ",
			ui.BrCyan, ui.Diamond, ui.Reset, ui.BrWhite, ui.Reset, ui.BrCyan, ui.Arrow, ui.Reset)
		input, _ := reader.ReadString('\n')
		name = strings.TrimSpace(input)
		if name == "" {
			return fmt.Errorf("profile name is required")
		}
	}

	// Check for duplicate names
	for _, p := range cfg.Profiles {
		if strings.EqualFold(p.Name, name) {
			return fmt.Errorf("profile %q already exists", name)
		}
	}

	defaultDir := "~/.claude-" + strings.ToLower(name)

	if len(args) >= 2 {
		configDir = args[1]
	} else {
		fmt.Printf("\n %s%s%s %sConfig directory%s %s[%s]%s\n   %s%s%s ",
			ui.BrCyan, ui.Diamond, ui.Reset, ui.BrWhite, ui.Reset,
			ui.DkGray, defaultDir, ui.Reset, ui.BrCyan, ui.Arrow, ui.Reset)
		input, _ := reader.ReadString('\n')
		configDir = strings.TrimSpace(input)
		if configDir == "" {
			configDir = defaultDir
		}
	}

	cfg.Profiles = append(cfg.Profiles, config.Profile{
		Name:      name,
		ConfigDir: configDir,
	})

	if err := config.Save(cfg, ""); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	ui.Ok(fmt.Sprintf("Profile %q added", name))
	fmt.Printf("   %s%s %s%s\n", ui.DkGray, ui.Arrow, configDir, ui.Reset)

	// Check if auth exists
	expandedDir := config.ExpandPath(configDir)
	credPath := filepath.Join(expandedDir, ".credentials.json")
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		fmt.Println()
		fmt.Printf(" %sTo authenticate this profile, run:%s\n", ui.DkGray, ui.Reset)
		fmt.Printf("   %sCLAUDE_CONFIG_DIR=%s claude auth%s\n\n",
			ui.BrCyan, config.ExpandPath(configDir), ui.Reset)
	}

	return nil
}

func runProfilesRemove(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	name := args[0]

	if len(cfg.Profiles) <= 1 {
		return fmt.Errorf("cannot remove the last profile")
	}

	found := -1
	for i, p := range cfg.Profiles {
		if strings.EqualFold(p.Name, name) {
			found = i
			break
		}
	}

	if found == -1 {
		return fmt.Errorf("profile %q not found", name)
	}

	cfg.Profiles = append(cfg.Profiles[:found], cfg.Profiles[found+1:]...)

	if err := config.Save(cfg, ""); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	ui.Ok(fmt.Sprintf("Profile %q removed", name))
	fmt.Println()
	return nil
}
