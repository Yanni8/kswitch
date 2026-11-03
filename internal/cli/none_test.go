// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"strings"
	"testing"

	"github.com/Yanni8/kubeswitch/internal/palette"
)

// "none" is offered as the last color and leaves the terminal alone.
func TestAddWithoutColor(t *testing.T) {
	dir := setupAdd(t, paste(testKubeconfig))

	out, err := execAdd(t, "\n2\nnone\n")
	if err != nil {
		t.Fatalf("add failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "none     (keep the terminal's colors)") {
		t.Errorf("the none option is missing from the menu:\n%s", out)
	}
	if !strings.Contains(out, `Added "dev-cluster" (dev, none)`) {
		t.Errorf("unexpected confirmation:\n%s", out)
	}
	if entry := readState(t, dir)["dev-cluster"]; entry["color"] != palette.None {
		t.Errorf("stored color = %q, want none", entry["color"])
	}

	list, err := execute(t, "", "list")
	if err != nil || !strings.Contains(list, "none") {
		t.Errorf("list should show the color as none: %v\n%s", err, list)
	}
	if colorSwatch(palette.None) != "" {
		t.Error("none has no swatch")
	}
}

// Pressing Enter at the color question keeps the terminal's colors.
func TestAddDefaultsToNoColor(t *testing.T) {
	dir := setupAdd(t, paste(testKubeconfig))

	out, err := execAdd(t, "\n1\n\n")
	if err != nil {
		t.Fatalf("add failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "(default: none)") {
		t.Errorf("none should be offered as the default:\n%s", out)
	}
	if !strings.Contains(out, `Added "dev-cluster" (prod, none)`) {
		t.Errorf("unexpected confirmation:\n%s", out)
	}
	if entry := readState(t, dir)["dev-cluster"]; entry["color"] != palette.None {
		t.Errorf("stored color = %q, want none", entry["color"])
	}
}

func TestUseWithoutColorChangesNothing(t *testing.T) {
	setupAdd(t, paste(testKubeconfig))
	if out, err := execAdd(t, "\n2\n9\n"); err != nil { // 9 = none, after the eight colors
		t.Fatalf("add failed: %v\n%s", err, out)
	}
	useShell(t, "true")

	out, err := execute(t, "", "use", "dev-cluster")
	if err != nil {
		t.Fatalf("use failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "\x1b]1") {
		t.Errorf("no color sequences expected:\n%s", out)
	}
	if restore := applyColors(nil, palette.None); restore == nil {
		t.Error("applyColors must return a no-op for none")
	}
}
