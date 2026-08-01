package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/platform"
	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	"github.com/spf13/cobra"
)

var cmdSave = &cobra.Command{
	Use:   "save <name>",
	Short: "Save the current session as a named profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := profile.NewStore()
		if err != nil {
			return err
		}
		return saveProfileWith(args[0], store, platform.Current(), os.Stdout)
	},
}

type saveStore interface {
	Checkpoint(string, string) error
	Inspect(string) profile.Inspection
}

func saveProfileWith(name string, store saveStore, p platform.Platform, out io.Writer) error {
	appData, err := p.AppDataPath()
	if err != nil {
		return err
	}
	running, err := p.IsRunning()
	if err != nil {
		return err
	}
	if running {
		fmt.Fprintln(out, "Stopping Claude Desktop...")
		if err := p.KillApp(); err != nil {
			return fmt.Errorf("stop Claude: %w", err)
		}
	}

	if err := store.Checkpoint(name, appData); err != nil {
		return err
	}
	fmt.Fprintf(out, "Profile %q saved.\n", name)
	warnRenewal(out, name, store.Inspect(name).ExpiresAt, time.Now())

	if running {
		fmt.Fprintln(out, "Starting Claude Desktop...")
		if err := p.LaunchApp(); err != nil {
			return fmt.Errorf("profile %q saved but Claude could not restart; launch manually: %w", name, err)
		}
	}
	return nil
}
