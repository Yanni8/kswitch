// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/Yanni8/kubeswitch/internal/kubeconfig"
	"github.com/Yanni8/kubeswitch/internal/prompt"
	"github.com/Yanni8/kubeswitch/internal/store"
)

// editTemplate heads the scratch file when editing a stored kubeconfig.
const editTemplate = `# Edit the kubeconfig below, then save and quit.
# Quit without saving to leave it unchanged.
`

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit [name]",
		Short: "Change a kubeconfig's contents, name, environment, or color",
		Long: `Edit a configured kubeconfig.

Without a name, pick the kubeconfig from a menu. You then choose what
to change: the kubeconfig contents (opened in your editor), the name,
the environment, or the terminal background color.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeKubeconfigNames,
		RunE:              runEdit,
	}
}

func runEdit(cmd *cobra.Command, args []string) error {
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
		target, err = pickKubeconfig(p, st, "Which kubeconfig do you want to edit?", prompt.IsTerminal(out))
		if err != nil {
			return err
		}
		if target.Name == "" {
			fmt.Fprintln(out, "No kubeconfigs yet. Add one with: kubeswitch add")
			return nil
		}
	}

	color := target.Color
	if sw := colorSwatch(target.Color); prompt.IsTerminal(out) && sw != "" {
		color += " " + sw
	}
	field, err := p.Select(fmt.Sprintf("What do you want to change for %q?", target.Name), []prompt.Option{
		{Value: "contents", Label: "Kubeconfig contents  (opens the editor)"},
		{Value: "name", Label: fmt.Sprintf("Name  (%s)", target.Name)},
		{Value: "environment", Label: fmt.Sprintf("Environment  (%s)", target.Environment)},
		{Value: "color", Label: fmt.Sprintf("Color  (%s)", color)},
	})
	if err != nil {
		return err
	}

	switch field {
	case "contents":
		return editContents(out, st, target)
	case "name":
		return editName(p, out, st, target)
	case "environment":
		return editEnvironment(p, out, st, target)
	default:
		return editColor(p, out, st, target)
	}
}

// editContents opens the stored kubeconfig in the editor and replaces
// it with the result, if it changed and still parses as a kubeconfig.
func editContents(out io.Writer, st *store.Store, k store.Kubeconfig) error {
	current, err := os.ReadFile(k.Path)
	if err != nil {
		return err
	}
	raw, scratch, err := editInEditor(editTemplate, current)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		os.Remove(scratch)
		return errors.New("aborted: the kubeconfig was emptied; nothing changed")
	}
	if bytes.Equal(raw, current) {
		os.Remove(scratch)
		fmt.Fprintln(out, "Nothing changed.")
		return nil
	}
	// Keep the scratch file on failure so the edits are not lost.
	fail := func(err error) error {
		fmt.Fprintf(out, "Your edited kubeconfig is kept at %s\n", scratch)
		return err
	}
	if _, err := kubeconfig.Parse(raw); err != nil {
		return fail(err)
	}
	if err := st.UpdateKubeconfig(k.Name, raw); err != nil {
		return fail(err)
	}
	os.Remove(scratch)
	fmt.Fprintf(out, "Updated %q: kubeconfig contents replaced\n", k.Name)
	return nil
}

func editName(p *prompt.Prompter, out io.Writer, st *store.Store, k store.Kubeconfig) error {
	name, err := p.Input("New name", prompt.InputOptions{
		Placeholder: k.Name,
		Validate:    func(name string) (string, error) { return checkName(name, st, k.Name) },
	})
	if err != nil {
		return err
	}
	if name == k.Name {
		fmt.Fprintln(out, "Nothing changed.")
		return nil
	}
	if err := st.Rename(k.Name, name); err != nil {
		return err
	}
	fmt.Fprintf(out, "Updated %q: renamed to %q\n", k.Name, name)
	return nil
}

func editEnvironment(p *prompt.Prompter, out io.Writer, st *store.Store, k store.Kubeconfig) error {
	env, err := promptEnvironment(p, k.Environment)
	if err != nil {
		return err
	}
	if env == k.Environment {
		fmt.Fprintln(out, "Nothing changed.")
		return nil
	}
	if err := st.SetEnvironment(k.Name, env); err != nil {
		return err
	}
	fmt.Fprintf(out, "Updated %q: environment is now %s\n", k.Name, env)
	return nil
}

func editColor(p *prompt.Prompter, out io.Writer, st *store.Store, k store.Kubeconfig) error {
	color, err := promptColor(p, k.Color)
	if err != nil {
		return err
	}
	if color == k.Color {
		fmt.Fprintln(out, "Nothing changed.")
		return nil
	}
	if err := st.SetColor(k.Name, color); err != nil {
		return err
	}
	fmt.Fprintf(out, "Updated %q: color is now %s\n", k.Name, color)
	return nil
}
