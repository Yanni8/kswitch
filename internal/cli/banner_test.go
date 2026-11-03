// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"strings"
	"testing"
)

func TestProdBanner(t *testing.T) {
	b := prodBanner("Payments Prod", true)
	for _, want := range []string{"PRODUCTION", "connected to the PROD cluster", `"Payments Prod"`, "exit"} {
		if !strings.Contains(b.Text, want) {
			t.Errorf("banner %q lacks %q", b.Text, want)
		}
	}
	if !strings.Contains(b.Short, "PRODUCTION") || !strings.Contains(b.Short, "Payments Prod") || len(b.Short) >= len(b.Text) {
		t.Errorf("short banner %q should be a shorter form of the full one", b.Short)
	}
	if !b.Flash || prodBanner("x", false).Flash {
		t.Error("Flash must follow the argument")
	}
}

// Without a terminal the shell runs plainly, whatever the banner flags.
func TestUseWithoutTerminalRunsPlainly(t *testing.T) {
	seedStore(t)
	useShell(t, "env")

	for _, args := range [][]string{
		{"use", "payments prod"},
		{"use", "payments prod", "--no-banner"},
		{"use", "payments prod", "--static-banner"},
	} {
		out, err := execute(t, "", args...)
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
		if !strings.Contains(out, "You are now connected to a PRODUCTION cluster") {
			t.Errorf("%v: the prod warning is missing:\n%s", args, out)
		}
		if !strings.Contains(out, "KUBESWITCH_ENV=prod\n") || strings.Contains(out, "\x1b[1;") {
			t.Errorf("%v: unexpected output:\n%s", args, out)
		}
	}
}
