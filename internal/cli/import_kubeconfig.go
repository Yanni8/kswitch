// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Yanni8/kubeswitch/internal/kubeconfig"
	"github.com/Yanni8/kubeswitch/internal/prompt"
	"github.com/Yanni8/kubeswitch/internal/store"
)

func newImportKubeconfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kubeconfig [file...]",
		Short: "Import the contexts of your existing kubeconfig",
		Long: `Import contexts from existing kubeconfig files.

Reads the files kubectl uses ($KUBECONFIG, or ~/.kube/config) unless
files are given, and goes through their contexts one by one, asking
whether to keep or skip each. A kept context becomes a kubeconfig of its
own, holding just that context with its cluster and user; you are asked
for a name (defaulting to the context name), the environment, and the
color, as "add" does. Files are merged the way kubectl merges them.`,
		Args: cobra.ArbitraryArgs,
		RunE: runImportKubeconfig,
	}
}

func runImportKubeconfig(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	st, err := store.Open()
	if err != nil {
		return err
	}

	paths := args
	if len(paths) == 0 {
		if paths, err = kubeconfig.DefaultPaths(); err != nil {
			return err
		}
	}
	source := strings.Join(paths, ", ")

	contexts, err := kubeconfig.Split(paths...)
	if err != nil {
		if len(args) == 0 && errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("no kubeconfig found at %s", source)
		}
		return err
	}
	if len(contexts) == 0 {
		fmt.Fprintf(out, "No contexts found in %s.\n", source)
		return nil
	}
	fmt.Fprintf(out, "Found %s in %s.\n", plural(len(contexts), "context"), source)

	p := prompt.New(cmd.InOrStdin(), out)
	imported := 0
	for _, c := range contexts {
		title := fmt.Sprintf("Import %q", c.Name)
		if c.Server != "" {
			title += fmt.Sprintf(" (%s)", c.Server)
		}
		title += "?"
		if exists, err := st.Exists(c.Name); err != nil {
			return err
		} else if exists {
			title += fmt.Sprintf(" A kubeconfig named %q already exists.", c.Name)
		}

		choice, err := p.Select(title, []prompt.Option{
			{Value: "keep", Label: "Keep"},
			{Value: "skip", Label: "Skip"},
		})
		if err != nil {
			return err
		}
		if choice != "keep" {
			continue
		}
		if err := addKubeconfig(p, out, st, c.Config, c.Name); err != nil {
			return err
		}
		imported++
	}

	fmt.Fprintf(out, "\nImported %d of %s.\n", imported, plural(len(contexts), "context"))
	return nil
}

// plural formats a count with its noun: "1 context", "2 contexts".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
