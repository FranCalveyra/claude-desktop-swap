package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/FranCalveyra/claude-desktop-swap/internal/platform"
	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	"github.com/spf13/cobra"
)

var cmdAdd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new account interactively without manual logout",
	Long: `Snapshots your current session, launches Claude Desktop with a clean
state, waits for you to log in as a new account, then snapshots that
new session as <name>. No manual logout required.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		store, err := profile.NewStore()
		if err != nil {
			return err
		}
		p := platform.Current()
		appData, err := beginAddProfileWith(name, store, p, os.Stdout)
		if err != nil {
			return err
		}

		fmt.Printf("\nLog in as the new account in Claude Desktop, then press Enter to snapshot it as %q: ", name)
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		return finishAddProfileWith(name, appData, store, p, os.Stdout)
	},
}

type addStore interface {
	Exists(string) bool
	Current() (string, error)
	Checkpoint(string, string) error
	Restore(string, string) error
	Wipe(string) error
}

func beginAddProfileWith(name string, store addStore, p platform.Platform, out io.Writer) (string, error) {
	appData, err := p.AppDataPath()
	if err != nil {
		return "", err
	}
	if store.Exists(name) {
		return "", fmt.Errorf("profile %q already exists — pick a different name or run 'delete %s' first", name, name)
	}
	running, err := p.IsRunning()
	if err != nil {
		return "", err
	}
	if running {
		fmt.Fprintln(out, "Stopping Claude Desktop...")
		if err := p.KillApp(); err != nil {
			return "", err
		}
	}
	if profile.HasActiveSession(appData) {
		current, _ := store.Current()
		fmt.Fprintf(out, "Snapshotting current session as %q...\n", current)
		if err := checkpointTrackedSession(store, appData); err != nil {
			return "", fmt.Errorf("snapshot current: %w", err)
		}
	}
	fmt.Fprintln(out, "Clearing session state...")
	if err := store.Wipe(appData); err != nil {
		return "", err
	}
	fmt.Fprintln(out, "Launching Claude Desktop with a fresh session.")
	if err := p.LaunchApp(); err != nil {
		return "", err
	}
	return appData, nil
}

func finishAddProfileWith(name, appData string, store addStore, p platform.Platform, out io.Writer) error {
	fmt.Fprintln(out, "Stopping Claude Desktop...")
	if err := p.KillApp(); err != nil {
		return err
	}
	if err := store.Checkpoint(name, appData); err != nil {
		return err
	}
	if err := store.Restore(name, appData); err != nil {
		return fmt.Errorf("post-save cleanup: %w", err)
	}
	fmt.Fprintf(out, "Profile %q saved.\n", name)
	return nil
}

type trackedCheckpointer interface {
	Current() (string, error)
	Checkpoint(string, string) error
}

func checkpointTrackedSession(store trackedCheckpointer, appData string) error {
	current, _ := store.Current()
	if current == "" {
		return fmt.Errorf("active session has no tracked profile; save it before continuing")
	}
	return store.Checkpoint(current, appData)
}
