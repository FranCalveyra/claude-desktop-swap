package cmd

import (
	"fmt"

	"github.com/FranCalveyra/claude-desktop-swap/internal/platform"
	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	"github.com/spf13/cobra"
)

var cmdStatus = &cobra.Command{
	Use:   "status",
	Short: "Show the currently active profile",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := profile.NewStore()
		if err != nil {
			return err
		}

		appData, err := platform.Current().AppDataPath()
		if err != nil {
			return err
		}
		fmt.Println(statusLine(store, appData))
		return nil
	},
}

type liveMatcher interface {
	MatchLive(string) (string, profile.Health)
}

func statusLine(store liveMatcher, appData string) string {
	name, health := store.MatchLive(appData)
	if name == "" {
		return fmt.Sprintf("Active profile: unknown (live health: %s)", health)
	}
	return fmt.Sprintf("Active profile: %s (%s)", name, health)
}
