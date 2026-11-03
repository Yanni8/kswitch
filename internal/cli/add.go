// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Yanni8/kubeswitch/internal/kubeconfig"
	"github.com/Yanni8/kubeswitch/internal/palette"
	"github.com/Yanni8/kubeswitch/internal/prompt"
	"github.com/Yanni8/kubeswitch/internal/store"
)

func newAddCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a kubeconfig by pasting it into your editor or from a file",
		Long: `Add a new kubeconfig to kubeswitch.

Opens your editor ($VISUAL or $EDITOR, default vim) on an empty scratch
file. Paste the kubeconfig, save, and quit. With --file, the kubeconfig
is read from that file instead and stored as is. kubeswitch then asks
for a name (defaulting to the kubeconfig's current context), whether it
is a prod or a dev environment, and which terminal background color it
gets.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAdd(cmd, file)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "read the kubeconfig from this file instead of opening the editor")
	return cmd
}

func runAdd(cmd *cobra.Command, file string) error {
	out := cmd.OutOrStdout()

	st, err := store.Open()
	if err != nil {
		return err
	}
	if file != "" {
		return addFromFile(cmd, st, file)
	}

	fmt.Fprintln(out, "Paste your kubeconfig into the editor, then save and quit (quit without saving to abort).")
	raw, scratch, err := editInEditor("", nil)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		os.Remove(scratch)
		return errors.New("aborted: no kubeconfig pasted")
	}
	// From here on the scratch file is kept whenever something goes
	// wrong, so the pasted config is not lost.
	fail := func(err error) error {
		fmt.Fprintf(out, "Your pasted kubeconfig is kept at %s\n", scratch)
		return err
	}

	cfg, err := kubeconfig.Parse(raw)
	if err != nil {
		return fail(err)
	}

	p := prompt.New(cmd.InOrStdin(), out)
	if err := addKubeconfig(p, out, st, raw, cfg.SuggestedName()); err != nil {
		return fail(err)
	}
	os.Remove(scratch)
	return nil
}

// addFromFile adds the kubeconfig in file, stored as is.
func addFromFile(cmd *cobra.Command, st *store.Store, file string) error {
	out := cmd.OutOrStdout()

	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if strings.TrimSpace(withoutComments(string(raw))) == "" {
		return fmt.Errorf("%s is empty", file)
	}
	cfg, err := kubeconfig.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	if rel := kubeconfig.RelativePaths(raw); len(rel) > 0 {
		fmt.Fprintf(out, "Note: %s refers to files by relative path (%s), which will not resolve from the stored copy. "+
			"Make them absolute first, or use \"kubeswitch import kubeconfig %s\", which does that for you.\n",
			file, strings.Join(rel, ", "), file)
	}

	p := prompt.New(cmd.InOrStdin(), out)
	return addKubeconfig(p, out, st, raw, cfg.SuggestedName())
}

// addKubeconfig asks for a name (suggesting suggested), the environment,
// and the color, then stores raw under them.
func addKubeconfig(p *prompt.Prompter, out io.Writer, st *store.Store, raw []byte, suggested string) error {
	name, err := promptName(p, suggested, st)
	if err != nil {
		return err
	}
	env, err := promptEnvironment(p, "")
	if err != nil {
		return err
	}
	color, err := promptColor(p, palette.None)
	if err != nil {
		return err
	}

	path, err := st.Add(name, env, color, raw)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "\nAdded %q (%s, %s)\n", name, env, color)
	fmt.Fprintf(out, "Saved to %s\n", path)
	return nil
}

// editInEditor opens the editor on a scratch file holding header (which
// is stripped from the result) followed by initial, and returns the
// edited text along with the scratch path. Content is nil if the file
// was left empty apart from comments. The caller owns the scratch file.
func editInEditor(header string, initial []byte) (content []byte, path string, err error) {
	f, err := os.CreateTemp("", "kubeswitch-*.yaml")
	if err != nil {
		return nil, "", err
	}
	path = f.Name()
	_, werr := f.WriteString(header)
	if werr == nil && len(initial) > 0 {
		_, werr = f.Write(initial)
	}
	cerr := f.Close()
	if werr != nil || cerr != nil {
		os.Remove(path)
		return nil, "", errors.Join(werr, cerr)
	}

	if err := openEditor(path); err != nil {
		os.Remove(path)
		return nil, "", fmt.Errorf("editor: %w", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		os.Remove(path)
		return nil, "", err
	}

	text := strings.TrimPrefix(string(raw), header)
	if strings.TrimSpace(withoutComments(text)) == "" {
		return nil, path, nil
	}
	return []byte(strings.TrimLeft(text, "\n")), path, nil
}

// withoutComments drops '#' comment lines. It only serves to decide
// whether the user pasted anything; stored content keeps its comments.
func withoutComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// promptName asks for a human-readable name, suggesting def (if any),
// and accepts only names that are valid and unused.
func promptName(p *prompt.Prompter, def string, st *store.Store) (string, error) {
	return p.Input("Name for this kubeconfig", prompt.InputOptions{
		Placeholder: def,
		Validate:    func(name string) (string, error) { return checkName(name, st, "") },
	})
}

// checkName explains why name cannot be used, or returns "" if it can.
// An existing entry named except does not count as a clash, so an entry
// can keep (or re-case) its own name.
func checkName(name string, st *store.Store, except string) (string, error) {
	if name == "" {
		return "A name is required.", nil
	}
	if err := store.ValidateName(name); err != nil {
		return capitalize(err.Error()) + ".", nil
	}
	existing, err := st.Get(name)
	if errors.Is(err, store.ErrNotFound) || (err == nil && existing.Name == except) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("A kubeconfig named %q already exists; choose another name.", existing.Name), nil
}

// promptEnvironment asks for the environment, with the cursor on def.
func promptEnvironment(p *prompt.Prompter, def store.Environment) (store.Environment, error) {
	choice, err := p.SelectDefault("Is this a prod or a dev environment?", []prompt.Option{
		{Value: string(store.EnvProd), Label: "prod"},
		{Value: string(store.EnvDev), Label: "dev"},
	}, string(def))
	return store.Environment(choice), err
}

// promptColor asks for the background color, with the cursor on def.
// Besides the palette, "none" leaves the terminal's colors alone.
func promptColor(p *prompt.Prompter, def string) (string, error) {
	opts := make([]prompt.Option, 0, len(palette.Colors)+1)
	for _, c := range palette.Colors {
		opts = append(opts, prompt.Option{
			Value: c.Name,
			Label: fmt.Sprintf("%-8s %s", c.Name, c.Swatch()),
		})
	}
	opts = append(opts, prompt.Option{
		Value: palette.None,
		Label: fmt.Sprintf("%-8s (keep the terminal's colors)", palette.None),
	})
	return p.SelectDefault("Which terminal background color should it get?", opts, def)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
