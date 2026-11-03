package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Yanni8/kswitch/internal/kubeconfig"
	"github.com/Yanni8/kswitch/internal/palette"
	"github.com/Yanni8/kswitch/internal/prompt"
	"github.com/Yanni8/kswitch/internal/store"
)

// addTemplate heads the scratch file when adding a kubeconfig; it is
// stripped again before the pasted kubeconfig is stored.
const addTemplate = `# Paste your kubeconfig below, then save and quit.
# Quit without saving (or leave the file empty) to abort.
`

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a kubeconfig by pasting it into your editor",
		Long: `Add a new kubeconfig to kswitch.

Opens your editor ($VISUAL or $EDITOR, default vim) on a scratch file.
Paste the kubeconfig, save, and quit. kswitch then asks for a name
(defaulting to the kubeconfig's current context), whether it is a prod
or a dev environment, and which terminal background color it gets.`,
		Args: cobra.NoArgs,
		RunE: runAdd,
	}
}

func runAdd(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()

	st, err := store.Open()
	if err != nil {
		return err
	}

	raw, scratch, err := editInEditor(addTemplate, nil)
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

	name, err := promptName(p, cfg.SuggestedName(), st)
	if err != nil {
		return fail(err)
	}
	env, err := promptEnvironment(p, "")
	if err != nil {
		return fail(err)
	}
	color, err := promptColor(p, "")
	if err != nil {
		return fail(err)
	}

	path, err := st.Add(name, env, color, raw)
	if err != nil {
		return fail(err)
	}
	os.Remove(scratch)

	fmt.Fprintf(out, "\nAdded %q (%s, %s)\n", name, env, color)
	fmt.Fprintf(out, "Saved to %s\n", path)
	return nil
}

// editInEditor opens the editor on a scratch file holding header (which
// is stripped from the result) followed by initial, and returns the
// edited text along with the scratch path. Content is nil if the file
// was left empty apart from comments. The caller owns the scratch file.
func editInEditor(header string, initial []byte) (content []byte, path string, err error) {
	f, err := os.CreateTemp("", "kswitch-*.yaml")
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
func promptColor(p *prompt.Prompter, def string) (string, error) {
	opts := make([]prompt.Option, len(palette.Colors))
	for i, c := range palette.Colors {
		opts[i] = prompt.Option{
			Value: c.Name,
			Label: fmt.Sprintf("%-8s %s", c.Name, c.Swatch()),
		}
	}
	return p.SelectDefault("Which terminal background color should it get?", opts, def)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
