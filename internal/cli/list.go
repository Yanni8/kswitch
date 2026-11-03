// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Yanni8/kubeswitch/internal/prompt"
	"github.com/Yanni8/kubeswitch/internal/store"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the configured kubeconfigs",
		Args:    cobra.NoArgs,
		RunE:    runList,
	}
}

func runList(cmd *cobra.Command, _ []string) error {
	st, err := store.Open()
	if err != nil {
		return err
	}
	configs, err := st.List()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(configs) == 0 {
		fmt.Fprintln(out, "No kubeconfigs yet. Add one with: kubeswitch add")
		return nil
	}

	// Swatches are escape sequences; only show them on a terminal so the
	// output stays clean when piped.
	swatches := prompt.IsTerminal(out)

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENVIRONMENT\tCOLOR")
	for _, c := range configs {
		color := c.Color
		if sw := colorSwatch(c.Color); swatches && sw != "" {
			color += " " + sw
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", c.Name, c.Environment, color)
	}
	return w.Flush()
}
