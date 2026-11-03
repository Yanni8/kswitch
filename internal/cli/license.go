// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Yanni8/kubeswitch/internal/licenses"
	"github.com/Yanni8/kubeswitch/internal/prompt"
)

func newLicenseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "license",
		Short: "Show the license of kubeswitch and of the libraries it uses",
		Long: `Show the license of kubeswitch and of the libraries it is built with.

On a terminal the text opens in your pager ($PAGER, default less);
otherwise it is printed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			text, err := licenses.Text(buildVersions())
			if err != nil {
				return err
			}
			return page(cmd.OutOrStdout(), text)
		},
	}
}

// buildVersions maps the modules compiled into this binary to their
// versions.
func buildVersions() map[string]string {
	versions := map[string]string{}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, d := range info.Deps {
			versions[d.Path] = d.Version
		}
	}
	return versions
}

// page shows text in the pager ($PAGER, default less) when out is a
// terminal, and prints it otherwise or when the pager cannot start.
func page(out io.Writer, text string) error {
	if prompt.IsTerminal(out) {
		args := strings.Fields(os.Getenv("PAGER"))
		if len(args) == 0 {
			args = []string{"less"}
		}
		if pager, err := exec.LookPath(args[0]); err == nil {
			c := exec.Command(pager, args[1:]...)
			c.Stdin = strings.NewReader(text)
			c.Stdout = out
			c.Stderr = os.Stderr
			if err := c.Start(); err == nil {
				c.Wait() // how the pager was quit is not an error
				return nil
			}
		}
	}
	_, err := io.WriteString(out, text)
	return err
}
