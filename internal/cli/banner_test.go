// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"strings"
	"testing"

	"github.com/Yanni8/kubeswitch/internal/palette"
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
	if b.Style != "" {
		t.Errorf("the prod banner uses the alarm style, got %q", b.Style)
	}
}

func TestQuietBanner(t *testing.T) {
	t.Setenv("COLORTERM", "truecolor")

	b := quietBanner("dev cluster", "green")
	if b.Text != "dev cluster" || b.Short != "" || b.Flash {
		t.Errorf("the quiet banner should show just the name and never flash, got %+v", b)
	}
	green, _ := palette.Find("green")
	if want := palette.Style(green.Accent(), "7"); b.Style != want {
		t.Errorf("style = %q, want the green accent %q", b.Style, want)
	}
	if b := quietBanner("x", palette.None); b.Style != palette.Style(palette.NeutralAccent, "7") {
		t.Errorf("without a color the neutral accent is used, got %q", b.Style)
	}

	t.Setenv("COLORTERM", "")
	if b := quietBanner("x", "green"); b.Style != "\x1b[0;7m" {
		t.Errorf("without 24-bit color the banner falls back to reverse video, got %q", b.Style)
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

	out, err := execute(t, "", "use", "dev cluster")
	if err != nil {
		t.Fatalf("dev session failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `Switched to "dev cluster" (dev).`) || strings.Contains(out, "\x1b[1;") {
		t.Errorf("unexpected dev output:\n%s", out)
	}
}
