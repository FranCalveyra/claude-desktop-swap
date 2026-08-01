package cmd

import (
	"fmt"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/platform"
	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	"github.com/spf13/cobra"
)

var statusCheck bool

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
		now := time.Now()
		expiry := liveSessionExpiry(appData, now)
		fmt.Println(statusLine(store, appData, expiry, now))

		if !statusCheck {
			return nil
		}
		switch profile.ClassifyRenewal(expiry, now) {
		case profile.RenewalSoon:
			return exitCodeError(1)
		case profile.RenewalExpired:
			return exitCodeError(2)
		}
		return nil
	},
}

func init() {
	cmdStatus.Flags().BoolVar(&statusCheck, "check", false, "exit 1 if the session expires soon, 2 if it has expired")
}

type liveMatcher interface {
	MatchLive(string) (string, profile.Health)
}

func statusLine(store liveMatcher, appData string, expiry, now time.Time) string {
	name, health := store.MatchLive(appData)
	if name == "" {
		return fmt.Sprintf("Active profile: unknown (live health: %s, session %s)", health, expiryLabel(expiry, now))
	}

	line := fmt.Sprintf("Active profile: %s (%s, session %s)", name, health, expiryLabel(expiry, now))
	switch profile.ClassifyRenewal(expiry, now) {
	case profile.RenewalSoon:
		line += fmt.Sprintf("\n⚠ Session expires in %s — keep using Claude on this profile to renew it.", until(expiry, now))
	case profile.RenewalExpired:
		line += "\n⚠ Session has expired — sign in again in Claude Desktop and re-save this profile."
	}
	return line
}
