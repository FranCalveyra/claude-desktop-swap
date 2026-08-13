package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// exitCodeError lets a command pick the process exit status without printing
// an extra error line — `status --check` reports through the exit code.
type exitCodeError int

func (e exitCodeError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

// Version is set at build time via -ldflags.
var Version = "dev"

var root = &cobra.Command{
	Use:     "claude-desktop-swap",
	Short:   "Switch between Claude Desktop accounts without logging out",
	Version: Version,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI()
	},
}

func Execute() {
	root.SilenceErrors = true
	err := root.Execute()
	if err == nil {
		return
	}
	var code exitCodeError
	if errors.As(err, &code) {
		os.Exit(int(code))
	}
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

func init() {
	root.AddCommand(cmdSave, cmdAdd, cmdUse, cmdList, cmdDelete, cmdStatus)
}
