// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEditorCommand(t *testing.T) {
	tests := []struct {
		name           string
		visual, editor string
		want           []string
	}{
		{"default", "", "", []string{"vim"}},
		{"EDITOR", "", "nano", []string{"nano"}},
		{"VISUAL wins", "code --wait", "nano", []string{"code", "--wait"}},
		{"blank is ignored", "  ", "nano", []string{"nano"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("VISUAL", tt.visual)
			t.Setenv("EDITOR", tt.editor)
			if got := editorCommand(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenEditorRunsCommand(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.yaml")
	if err := os.WriteFile(src, []byte("pasted\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// "cp <src>" acts as an editor that fills the scratch file.
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "cp "+src)

	scratch := filepath.Join(dir, "scratch.yaml")
	if err := openEditor(scratch); err != nil {
		t.Fatalf("openEditor: %v", err)
	}
	if got, _ := os.ReadFile(scratch); string(got) != "pasted\n" {
		t.Errorf("scratch content %q", got)
	}
}
