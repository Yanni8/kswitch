// Package cli defines the kswitch command-line interface.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/Yanni8/kswitch/internal/version"
)

// NewRootCmd builds the root command with all subcommands attached.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kswitch",
		Short:   "kswitch is a fast context switcher",
		Version: version.String(),
		// Errors are reported once by main, not echoed by cobra as well.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newAddCmd(), newListCmd(), newRemoveCmd(), newUseCmd(), newVersionCmd())

	return cmd
}

// Execute runs the root command with os.Args.
func Execute() error {
	return NewRootCmd().Execute()
}
