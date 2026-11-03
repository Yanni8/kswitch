// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// useShell makes "use" start the given program instead of a shell and
// ensures the test does not look nested in a kubeswitch shell.
func useShell(t *testing.T, program string) {
	t.Helper()
	path, err := exec.LookPath(program)
	if err != nil {
		t.Skipf("%s not available: %v", program, err)
	}
	t.Setenv("SHELL", path)
	t.Setenv("KUBESWITCH_NAME", "")
}

func TestUseByNameStartsShellWithKubeconfig(t *testing.T) {
	dir := seedStore(t)
	useShell(t, "env") // prints its environment and exits

	out, err := execute(t, "", "use", "payments prod") // case-insensitive
	if err != nil {
		t.Fatalf("use failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		`Switched to "Payments Prod". You are now connected to a PRODUCTION cluster. Type exit to go back.`,
		"KUBECONFIG=" + filepath.Join(dir, "configs", "payments-prod.yaml") + "\n",
		"KUBESWITCH_NAME=Payments Prod\n",
		"KUBESWITCH_ENV=prod\n",
		`Back from "Payments Prod".`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b]1") {
		t.Error("terminal colors must not be changed when stdout is not a terminal")
	}
	if strings.Contains(out, "nested") {
		t.Errorf("unexpected nesting note:\n%s", out)
	}
}

func TestUsePicksFromMenu(t *testing.T) {
	seedStore(t)
	useShell(t, "env")

	out, err := execute(t, "dev cluster\n", "switch") // the alias
	if err != nil {
		t.Fatalf("use failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		"Which kubeconfig do you want to use?",
		"dev cluster  (dev, green)",
		"KUBESWITCH_NAME=dev cluster\n",
		"KUBESWITCH_ENV=dev\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestUsePropagatesExitStatus(t *testing.T) {
	seedStore(t)
	useShell(t, "false")

	_, err := execute(t, "", "use", "dev cluster")
	if code, ok := ExitStatus(err); !ok || code != 1 {
		t.Fatalf("got %v, want exit status 1", err)
	}
	if _, ok := ExitStatus(errorf("other")); ok {
		t.Error("ExitStatus must not match unrelated errors")
	}
}

func TestUseNotesNesting(t *testing.T) {
	seedStore(t)
	useShell(t, "true")
	t.Setenv("KUBESWITCH_NAME", "outer")

	out, err := execute(t, "", "use", "dev cluster")
	if err != nil {
		t.Fatalf("use failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `nested inside the one for "outer"`) {
		t.Errorf("expected a nesting note:\n%s", out)
	}
}

func TestUseUnknownName(t *testing.T) {
	seedStore(t)
	useShell(t, "true")

	_, err := execute(t, "", "use", "nope")
	if err == nil || !strings.Contains(err.Error(), `kubeconfig "nope" not found`) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestUseEmptyStore(t *testing.T) {
	t.Setenv("KUBESWITCH_CONFIG_DIR", t.TempDir())
	useShell(t, "true")

	out, err := execute(t, "", "use")
	if err != nil {
		t.Fatalf("use failed: %v", err)
	}
	if !strings.Contains(out, "No kubeconfigs yet") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestUseMissingShell(t *testing.T) {
	seedStore(t)
	t.Setenv("SHELL", filepath.Join(t.TempDir(), "no-such-shell"))
	t.Setenv("KUBESWITCH_NAME", "")

	_, err := execute(t, "", "use", "dev cluster")
	if err == nil || !strings.Contains(err.Error(), "starting") {
		t.Fatalf("expected a start error, got %v", err)
	}
}

type errorf string

func (e errorf) Error() string { return string(e) }
