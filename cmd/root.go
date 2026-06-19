package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

var root = &cobra.Command{
	Use:     "claude-desktop-swap",
	Short:   "Switch between Claude Desktop accounts without logging out",
	Version: Version,
}

func Execute() {
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	root.AddCommand(cmdSave, cmdAdd, cmdUse, cmdList, cmdDelete, cmdStatus)
}
