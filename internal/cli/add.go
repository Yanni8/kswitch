package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Yanni8/kswitch/internal/kubeconfig"
	"github.com/Yanni8/kswitch/internal/prompt"
	"github.com/Yanni8/kswitch/internal/store"
)

// editorTemplate is pre-filled into the scratch file and stripped again
// before the pasted kubeconfig is stored.
const editorTemplate = `# Paste your kubeconfig below, then save and quit.
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

	raw, scratch, err := editKubeconfig()
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
	env, err := promptEnvironment(p)
	if err != nil {
		return fail(err)
	}
	color, err := promptColor(p)
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

// editKubeconfig opens the editor on a scratch file and returns what the
// user pasted (template stripped) along with the scratch path. Content is
// nil if the user pasted nothing. The caller owns the scratch file.
func editKubeconfig() (content []byte, path string, err error) {
	f, err := os.CreateTemp("", "kswitch-*.yaml")
	if err != nil {
		return nil, "", err
	}
	path = f.Name()
	_, werr := f.WriteString(editorTemplate)
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

	text := strings.TrimPrefix(string(raw), editorTemplate)
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
		Validate:    func(name string) (string, error) { return checkName(name, st) },
	})
}

// checkName explains why name cannot be used, or returns "" if it can.
func checkName(name string, st *store.Store) (string, error) {
	if name == "" {
		return "A name is required.", nil
	}
	if err := store.ValidateName(name); err != nil {
		return capitalize(err.Error()) + ".", nil
	}
	exists, err := st.Exists(name)
	if err != nil {
		return "", err
	}
	if exists {
		return fmt.Sprintf("A kubeconfig named %q already exists; choose another name.", name), nil
	}
	return "", nil
}

func promptEnvironment(p *prompt.Prompter) (store.Environment, error) {
	choice, err := p.Select("Is this a prod or a dev environment?", []prompt.Option{
		{Value: string(store.EnvProd), Label: "prod"},
		{Value: string(store.EnvDev), Label: "dev"},
	})
	return store.Environment(choice), err
}

func promptColor(p *prompt.Prompter) (string, error) {
	opts := make([]prompt.Option, len(backgroundColors))
	for i, c := range backgroundColors {
		opts[i] = prompt.Option{
			Value: c.Name,
			Label: fmt.Sprintf("%-8s %s", c.Name, c.swatch()),
		}
	}
	return p.Select("Which terminal background color should it get?", opts)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
