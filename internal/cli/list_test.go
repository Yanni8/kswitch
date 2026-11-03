// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"strings"
	"testing"

	"github.com/Yanni8/kubeswitch/internal/store"
)

// execList runs "kubeswitch list" (or the given alias) and returns its output.
func execList(t *testing.T, name string) (string, error) {
	t.Helper()
	return execute(t, "", name)
}

func TestListEmptyStore(t *testing.T) {
	t.Setenv("KUBESWITCH_CONFIG_DIR", t.TempDir())

	out, err := execList(t, "list")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out, "No kubeconfigs yet") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestListShowsConfigsSortedByName(t *testing.T) {
	t.Setenv("KUBESWITCH_CONFIG_DIR", t.TempDir())
	st, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		env   store.Environment
		color string
	}{
		{"Payments Prod", store.EnvProd, "red"},
		{"dev cluster", store.EnvDev, "green"},
	} {
		if _, err := st.Add(c.name, c.env, c.color, []byte("a: 1\n")); err != nil {
			t.Fatal(err)
		}
	}

	out, err := execList(t, "ls") // the alias
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected a header and two rows, got:\n%s", out)
	}
	if fields := strings.Fields(lines[0]); strings.Join(fields, " ") != "NAME ENVIRONMENT COLOR" {
		t.Errorf("unexpected header: %q", lines[0])
	}
	// Sorted case-insensitively: "dev cluster" before "Payments Prod".
	if !strings.HasPrefix(lines[1], "dev cluster") || !strings.Contains(lines[1], "dev") || !strings.HasSuffix(lines[1], "green") {
		t.Errorf("unexpected first row: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "Payments Prod") || !strings.Contains(lines[2], "prod") || !strings.HasSuffix(lines[2], "red") {
		t.Errorf("unexpected second row: %q", lines[2])
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("swatches must not be printed when stdout is not a terminal")
	}
}
