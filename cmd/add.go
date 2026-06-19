package cmd

import (
	"bufio"
	"fmt"
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

		p := platform.Current()
		appData, err := p.AppDataPath()
		if err != nil {
			return err
		}

		store, err := profile.NewStore()
		if err != nil {
			return err
		}

		if store.Exists(name) {
			return fmt.Errorf("profile %q already exists — pick a different name or run 'delete %s' first", name, name)
		}

		running, err := p.IsRunning()
		if err != nil {
			return err
		}
		if running {
			fmt.Println("Stopping Claude Desktop...")
			if err := p.KillApp(); err != nil {
				return err
			}
		}

		if profile.HasActiveSession(appData) {
			current, _ := store.Current()
			fmt.Printf("Snapshotting current session as %q...\n", current)
			if err := checkpointTrackedSession(store, appData); err != nil {
				return fmt.Errorf("snapshot current: %w", err)
			}
		}

		fmt.Println("Clearing session state...")
		if err := store.Wipe(appData); err != nil {
			return err
		}

		fmt.Println("Launching Claude Desktop with a fresh session.")
		if err := p.LaunchApp(); err != nil {
			return err
		}

		fmt.Printf("\nLog in as the new account in Claude Desktop, then press Enter to snapshot it as %q: ", name)
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')

		fmt.Println("Stopping Claude Desktop...")
		if err := p.KillApp(); err != nil {
			return err
		}

		if err := store.Checkpoint(name, appData); err != nil {
			return err
		}
		if err := store.Restore(name, appData); err != nil {
			return fmt.Errorf("post-save cleanup: %w", err)
		}

		fmt.Printf("Profile %q saved.\n", name)
		return nil
	},
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
