package cli

import (
	"bytes"
	"strings"
	"testing"
)

// execute runs the CLI with stdin feeding its prompts and returns the
// combined output.
func execute(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(stdin))
	// Must be non-nil, otherwise cobra falls back to os.Args.
	if args == nil {
		args = []string{}
	}
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}

// run executes the CLI and fails the test if the command errors.
func run(t *testing.T, args ...string) string {
	t.Helper()
	out, err := execute(t, "", args...)
	if err != nil {
		t.Fatalf("kubeswitch %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func TestRootShowsHelp(t *testing.T) {
	out := run(t)
	if !strings.Contains(out, "Usage:") {
		t.Errorf("expected help output, got:\n%s", out)
	}
}

func TestVersionCommand(t *testing.T) {
	out := run(t, "version")
	if strings.TrimSpace(out) == "" {
		t.Error("version output is empty")
	}
}
