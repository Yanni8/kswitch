// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// complete asks cobra's hidden __complete command what it would offer.
func complete(t *testing.T, args ...string) string {
	t.Helper()
	out, err := execute(t, "", append([]string{cobra.ShellCompRequestCmd}, args...)...)
	if err != nil {
		t.Fatalf("completion failed: %v\n%s", err, out)
	}
	return out
}

func TestCompletesKubeconfigNames(t *testing.T) {
	seedStore(t)

	for _, command := range []string{"use", "switch", "edit", "remove", "rm"} {
		out := complete(t, command, "")
		for _, want := range []string{"Payments Prod\tprod, red\n", "dev cluster\tdev, green\n"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: completion lacks %q:\n%s", command, want, out)
			}
		}
		if !strings.Contains(out, ":4\n") { // ShellCompDirectiveNoFileComp
			t.Errorf("%s: file completion should be off:\n%s", command, out)
		}
	}
}

func TestCompletionFiltersByPrefix(t *testing.T) {
	seedStore(t)

	out := complete(t, "use", "pay") // case-insensitive
	if !strings.Contains(out, "Payments Prod\t") || strings.Contains(out, "dev cluster") {
		t.Errorf("unexpected candidates for \"pay\":\n%s", out)
	}
}

func TestCompletionStopsAfterTheName(t *testing.T) {
	seedStore(t)

	out := complete(t, "use", "dev cluster", "")
	if strings.Contains(out, "Payments Prod") || strings.Contains(out, "dev cluster\t") {
		t.Errorf("no further names expected after the first argument:\n%s", out)
	}
}

func TestCompletionWithEmptyStore(t *testing.T) {
	t.Setenv("KUBESWITCH_CONFIG_DIR", t.TempDir())

	out := complete(t, "use", "")
	// Candidates carry a tab before their description; cobra's own
	// directive line and its explanation on stderr do not.
	if strings.Contains(out, "\t") || !strings.HasPrefix(out, ":4\n") {
		t.Errorf("expected no candidates, got:\n%s", out)
	}
}
