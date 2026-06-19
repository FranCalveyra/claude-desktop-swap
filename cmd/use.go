package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/FranCalveyra/claude-desktop-swap/internal/platform"
	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	"github.com/spf13/cobra"
)

var cmdUse = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a saved profile (kills and restarts Claude Desktop)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := profile.NewStore()
		if err != nil {
			return err
		}

		return switchProfile(args[0], store)
	},
}

func switchProfile(name string, store *profile.Store) error {
	return switchProfileWith(name, store, platform.Current(), os.Stdout)
}

type switchStore interface {
	Exists(string) bool
	Inspect(string) profile.Inspection
	Current() (string, error)
	Checkpoint(string, string) error
	Restore(string, string) error
}

func switchProfileWith(name string, store switchStore, p platform.Platform, out io.Writer) error {
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

	fmt.Fprintln(out, "Starting Claude Desktop...")
	if err := p.LaunchApp(); err != nil {
		return fmt.Errorf("profile %q is active but Claude could not start; launch manually: %w", name, err)
	}

	fmt.Fprintf(out, "Switched to %q.\n", name)
	return nil
}
