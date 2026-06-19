package cmd

import (
	"fmt"

	"github.com/FranCalveyra/claude-desktop-swap/internal/platform"
	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	"github.com/spf13/cobra"
)

var cmdSave = &cobra.Command{
	Use:   "save <name>",
	Short: "Save the current session as a named profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		p := platform.Current()
		appData, err := p.AppDataPath()
		if err != nil {
			return err
		}

		running, err := p.IsRunning()
		if err != nil {
			return err
		}
		if running {
			force, _ := cmd.Flags().GetBool("force")
			if err := validateSaveState(running, force); err != nil {
				return err
			}
		}

		store, err := profile.NewStore()
		if err != nil {
			return err
		}

		if err := store.Checkpoint(name, appData); err != nil {
			return err
		}

		fmt.Printf("Profile %q saved.\n", name)
		return nil
	},
}

func validateSaveState(running, force bool) error {
	if running {
		return fmt.Errorf("Claude Desktop is running; quit it before saving so Cookies can be checkpointed safely")
	}
	return nil
}

func init() {
	cmdSave.Flags().Bool("force", false, "deprecated; saving while Claude Desktop is running is refused")
}
