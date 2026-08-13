package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	"github.com/spf13/cobra"
)

var cmdDelete = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a saved profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		store, err := profile.NewStore()
		if err != nil {
			return err
		}

		return deleteProfileWith(name, store, os.Stdout)
	},
}

type deleteStore interface {
	Delete(string) error
}

func deleteProfileWith(name string, store deleteStore, out io.Writer) error {
	if err := store.Delete(name); err != nil {
		return err
	}
	fmt.Fprintf(out, "Profile %q deleted.\n", name)
	return nil
}
