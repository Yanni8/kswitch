// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Yanni8/kubeswitch/internal/palette"
	"github.com/Yanni8/kubeswitch/internal/prompt"
	"github.com/Yanni8/kubeswitch/internal/session"
	"github.com/Yanni8/kubeswitch/internal/store"
	"github.com/Yanni8/kubeswitch/internal/terminal"
)

func newUseCmd() *cobra.Command {
	var noBanner, staticBanner bool
	cmd := &cobra.Command{
		Use:     "use [name]",
		Aliases: []string{"switch"},
		Short:   "Switch to a kubeconfig in a new shell",
		Long: `Switch to a kubeconfig.

Starts a new shell ($SHELL) with KUBECONFIG pointing at the chosen
kubeconfig and the terminal background set to its color. Type exit to
return to the previous shell; the background is restored.

A banner stays pinned to the bottom row of the terminal for the whole
session (the shell gets one row less): a quiet one with the kubeconfig's
name, or a flashing PRODUCTION warning for a prod kubeconfig.
--static-banner stops the flashing, --no-banner leaves the banner out.

Without a name, pick the kubeconfig from a menu. The shell also gets
KUBESWITCH_NAME and KUBESWITCH_ENV set, for example for use in your prompt.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeKubeconfigNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUse(cmd, args, noBanner, staticBanner)
		},
	}
	cmd.Flags().BoolVar(&noBanner, "no-banner", false, "do not pin a banner to the bottom of the terminal")
	cmd.Flags().BoolVar(&staticBanner, "static-banner", false, "show the PRODUCTION banner without flashing")
	return cmd
}

// prodBanner is kept on the bottom row of the terminal while a prod
// kubeconfig is in use.
func prodBanner(name string, flash bool) session.Banner {
	return session.Banner{
		Text:  fmt.Sprintf("!!! PRODUCTION !!!   You are connected to the PROD cluster %q   —   type exit to leave   !!! PRODUCTION !!!", name),
		Short: fmt.Sprintf("!!! PRODUCTION: %s — exit to leave !!!", name),
		Flash: flash,
	}
}

// quietBanner is kept on the bottom row for any other kubeconfig: just
// its name, on a background tinted to go with the kubeconfig's color
// (or a neutral gray when it has none).
func quietBanner(name, color string) session.Banner {
	bg := palette.NeutralAccent
	if c, ok := palette.Find(color); ok {
		bg = c.Accent()
	}
	return session.Banner{Text: name, Style: palette.Style(bg, "7")}
}

func runUse(cmd *cobra.Command, args []string, noBanner, staticBanner bool) error {
	out := cmd.OutOrStdout()

	st, err := store.Open()
	if err != nil {
		return err
	}

	var target store.Kubeconfig
	if len(args) == 1 {
		if target, err = st.Get(args[0]); err != nil {
			return err
		}
	} else {
		p := prompt.New(cmd.InOrStdin(), out)
		target, err = pickKubeconfig(p, st, "Which kubeconfig do you want to use?", prompt.IsTerminal(out))
		if err != nil {
			return err
		}
		if target.Name == "" {
			fmt.Fprintln(out, "No kubeconfigs yet. Add one with: kubeswitch add")
			return nil
		}
	}

	if outer := os.Getenv("KUBESWITCH_NAME"); outer != "" {
		fmt.Fprintf(out, "Note: this shell is nested inside the one for %q.\n", outer)
	}

	restore := applyColors(out, target.Color)
	defer restore()

	var banner session.Banner
	switch {
	case noBanner:
	case target.Environment == store.EnvProd:
		banner = prodBanner(target.Name, !staticBanner)
	default:
		banner = quietBanner(target.Name, target.Color)
	}

	if target.Environment == store.EnvProd {
		fmt.Fprintf(out, "Switched to %q. You are now connected to a PRODUCTION cluster. Type exit to go back.\n", target.Name)
	} else {
		fmt.Fprintf(out, "Switched to %q (%s). Type exit to go back.\n", target.Name, target.Environment)
	}
	err = runShell(cmd, target, banner)
	restore()
	fmt.Fprintf(out, "Back from %q.\n", target.Name)
	return err
}

// runShell starts the user's shell with the kubeconfig active and waits
// for it to exit. With a banner, and a terminal to show it on, the shell
// runs inside a session that pins the banner to the terminal's bottom
// row. A non-zero exit status is returned as an exitError.
func runShell(cmd *cobra.Command, k store.Kubeconfig, banner session.Banner) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	c := exec.Command(shell)
	c.Env = append(os.Environ(),
		"KUBECONFIG="+k.Path,
		"KUBESWITCH_NAME="+k.Name,
		"KUBESWITCH_ENV="+string(k.Environment),
	)

	// Ctrl-C and Ctrl-\ are for whatever runs inside the shell. They are
	// caught rather than ignored: an ignored signal would be inherited by
	// the shell and its children, leaving Ctrl-C dead in the session.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGQUIT)
	defer func() { signal.Stop(sigs); close(sigs) }()
	go func() {
		for range sigs {
		}
	}()

	var err error
	in, inOK := cmd.InOrStdin().(*os.File)
	out, outOK := cmd.OutOrStdout().(*os.File)
	if banner.Text != "" && inOK && outOK && prompt.IsTerminal(in) && prompt.IsTerminal(out) {
		err = session.Run(c, in, out, banner)
	} else {
		c.Stdin = cmd.InOrStdin()
		c.Stdout = cmd.OutOrStdout()
		c.Stderr = cmd.ErrOrStderr()
		err = c.Run()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code := exit.ExitCode()
		if code < 0 { // killed by a signal
			code = 1
		}
		return exitError{code: code}
	}
	if err != nil {
		return fmt.Errorf("starting %s: %w", shell, err)
	}
	return nil
}

// applyColors sets the terminal background to the named palette color,
// with a text color that reads well on it, when out is a terminal. The
// returned function restores the previous colors and may be called more
// than once.
func applyColors(out io.Writer, color string) func() {
	if color == palette.None {
		return func() {}
	}
	c, ok := palette.Find(color)
	if !ok || !prompt.IsTerminal(out) {
		return func() {}
	}
	colors, err := terminal.SetColors(c.Text(), c.Hex)
	if err != nil {
		return func() {}
	}
	var once sync.Once
	return func() { once.Do(colors.Restore) }
}

// exitError carries the exit status of the shell started by "use".
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// ExitStatus returns the exit status carried by err, if any. main uses
// it to exit with the shell's status instead of reporting an error.
func ExitStatus(err error) (int, bool) {
	var e exitError
	if errors.As(err, &e) {
		return e.code, true
	}
	return 0, false
}
