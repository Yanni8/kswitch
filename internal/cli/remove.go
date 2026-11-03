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
		if target, err = pickKubeconfig(p, st); err != nil {
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

// pickKubeconfig lets the user choose one of the stored kubeconfigs. It
// returns a zero Kubeconfig when there are none.
func pickKubeconfig(p *prompt.Prompter, st *store.Store) (store.Kubeconfig, error) {
	configs, err := st.List()
	if err != nil || len(configs) == 0 {
		return store.Kubeconfig{}, err
	}
	opts := make([]prompt.Option, len(configs))
	for i, c := range configs {
		opts[i] = prompt.Option{
			Value: c.Name,
			Label: fmt.Sprintf("%s  (%s, %s)", c.Name, c.Environment, c.Color),
		}
	}
	name, err := p.Select("Which kubeconfig should be removed?", opts)
	if err != nil {
		return store.Kubeconfig{}, err
	}
	for _, c := range configs {
		if c.Name == name {
			return c, nil
		}
	}
	return store.Kubeconfig{}, fmt.Errorf("kubeconfig %q %w", name, store.ErrNotFound)
}
