package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/account"
	"github.com/FranCalveyra/claude-desktop-swap/internal/platform"
	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	"github.com/spf13/cobra"
)

var cmdUse = &cobra.Command{
	Use:   "use [name]",
	Short: "Switch to a saved profile (kills and restarts Claude Desktop)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := profile.NewStore()
		if err != nil {
			return err
		}

		name, err := profileNameFromArgs(args, store)
		if err != nil {
			return err
		}
		if name == "" {
			return nil
		}

		return switchProfile(name, store)
	},
}

func profileNameFromArgs(args []string, store *profile.Store) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	profiles, err := store.List()
	if err != nil {
		return "", err
	}
	if len(profiles) == 0 {
		fmt.Println("No profiles saved. Run 'claude-desktop-swap save <name>' to create one.")
		return "", nil
	}

	current := ""
	if appData, err := platform.Current().AppDataPath(); err == nil {
		current, _ = store.MatchLive(appData)
	}
	enrichLiveAccounts(store, profiles)
	return runPicker(profiles, current)
}

func switchProfile(name string, store *profile.Store) error {
	return switchProfileWith(name, store, platform.Current(), os.Stdout, account.Fetch)
}

type switchStore interface {
	Exists(string) bool
	Inspect(string) profile.Inspection
	Current() (string, error)
	Checkpoint(string, string) error
	Restore(string, string) error
}

// accountVerifier checks a live Cookies database against the Claude.ai API.
type accountVerifier func(cookiesPath string) account.Info

func switchProfileWith(name string, store switchStore, p platform.Platform, out io.Writer, verify accountVerifier) error {
	appData, err := p.AppDataPath()
	if err != nil {
		return err
	}

	if !store.Exists(name) {
		return fmt.Errorf("profile %q not found — run 'claude-desktop-swap list' to see available profiles", name)
	}
	inspection := store.Inspect(name)
	if inspection.Health != profile.HealthUsable {
		return fmt.Errorf("profile %q is %s; reauthentication is required before switching", name, inspection.Health)
	}

	fmt.Fprintln(out, "Stopping Claude Desktop...")
	if err := p.KillApp(); err != nil {
		return fmt.Errorf("stop Claude: %w", err)
	}

	current, _ := store.Current()
	if current != "" {
		fmt.Fprintf(out, "Checkpointing profile %q...\n", current)
		if err := store.Checkpoint(current, appData); err != nil {
			return fmt.Errorf("checkpoint outgoing profile: %w", err)
		}
	}

	fmt.Fprintf(out, "Restoring profile %q...\n", name)
	if err := store.Restore(name, appData); err != nil {
		return fmt.Errorf("restore profile: %w", err)
	}

	if info := verify(profile.CookiesPath(appData)); info.Rejected {
		return fmt.Errorf("profile %q was rejected by the server; sign in again in Claude Desktop and re-save it", name)
	}

	fmt.Fprintln(out, "Starting Claude Desktop...")
	if err := p.LaunchApp(); err != nil {
		return fmt.Errorf("profile %q is active but Claude could not start; launch manually: %w", name, err)
	}

	fmt.Fprintf(out, "Switched to %q.\n", name)
	warnRenewal(out, name, inspection.ExpiresAt, time.Now())
	return nil
}

func warnRenewal(out io.Writer, name string, expiresAt, now time.Time) {
	switch profile.ClassifyRenewal(expiresAt, now) {
	case profile.RenewalSoon:
		fmt.Fprintf(out, "⚠ Session for %q expires in %s — use it in Claude Desktop to renew it before it lapses.\n", name, until(expiresAt, now))
	case profile.RenewalExpired:
		fmt.Fprintf(out, "⚠ Session for %q has expired — sign in again in Claude Desktop and re-save it.\n", name)
	}
}
