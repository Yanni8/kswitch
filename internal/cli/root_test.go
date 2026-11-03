package cli

import (
	"bytes"
	"strings"
	"testing"
)

// run executes the CLI with the given args and returns its combined output.
func run(t *testing.T, args ...string) string {
	t.Helper()

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Must be non-nil, otherwise cobra falls back to os.Args.
	if args == nil {
		args = []string{}
	}
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("kswitch %s: %v", strings.Join(args, " "), err)
	}
	return out.String()
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
