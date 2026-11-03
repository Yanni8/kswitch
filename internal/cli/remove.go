package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Yanni8/kswitch/internal/prompt"
	"github.com/Yanni8/kswitch/internal/store"
)

func newRemoveCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "remove [name]",
		Aliases: []string{"rm"},
		Short:   "Remove a configured kubeconfig",
		Long: `Remove a kubeconfig and delete its stored file.

Without a name, pick the kubeconfig from a menu. You are asked to
confirm unless --yes is given.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(cmd, args, yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "remove without asking for confirmation")
	return cmd
}

func runRemove(cmd *cobra.Command, args []string, yes bool) error {
	out := cmd.OutOrStdout()

	st, err := store.Open()
	if err != nil {
		return err
	}
	p := prompt.New(cmd.InOrStdin(), out)

	var target store.Kubeconfig
	if len(args) == 1 {
		if target, err = st.Get(args[0]); err != nil {
			return err
		}
	} else {
		target, err = pickKubeconfig(p, st, "Which kubeconfig should be removed?", prompt.IsTerminal(out))
		if err != nil {
			return err
		}
		if target.Name == "" {
			fmt.Fprintln(out, "No kubeconfigs to remove.")
			return nil
		}
	}

	if !yes {
		answer, err := p.Select(fmt.Sprintf("Remove %q (%s, %s)?", target.Name, target.Environment, target.Color), []prompt.Option{
			{Value: "no", Label: "No, keep it"},
			{Value: "yes", Label: "Yes, remove it"},
		})
		if err != nil {
			return err
		}
		if answer != "yes" {
			fmt.Fprintln(out, "Nothing removed.")
			return nil
		}
	}

	if err := st.Remove(target.Name); err != nil {
		return err
	}
	fmt.Fprintf(out, "Removed %q\n", target.Name)
	return nil
}
