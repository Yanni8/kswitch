// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package terminal

import "testing"

func TestParseReply(t *testing.T) {
	tests := []struct {
		name  string
		reply string
		code  int
		want  string
		ok    bool
	}{
		{"BEL terminated", "\x1b]11;rgb:1e1e/1e1e/1e1e\x07", 11, "rgb:1e1e/1e1e/1e1e", true},
		{"ST terminated", "\x1b]11;rgb:1e1e/1e1e/1e1e\x1b\\", 11, "rgb:1e1e/1e1e/1e1e", true},
		{"text color", "\x1b]10;rgb:ffff/ffff/ffff\x07", 10, "rgb:ffff/ffff/ffff", true},
		{"leading noise", "junk\x1b]11;#000000\x07", 11, "#000000", true},
		{"incomplete", "\x1b]11;rgb:1e1e/1e", 11, "", false},
		{"nothing yet", "", 11, "", false},
		{"unrelated", "\x1b[?1;2c", 11, "", false},
		{"other code", "\x1b]10;#ffffff\x07", 11, "", false},
		{"empty spec", "\x1b]11;\x07", 11, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseReply([]byte(tt.reply), tt.code)
			if got != tt.want || ok != tt.ok {
				t.Errorf("parseReply(%q, %d) = %q, %v; want %q, %v", tt.reply, tt.code, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestRestoreSequences(t *testing.T) {
	if got, want := restore(oscBackground, "rgb:1e1e/1e1e/1e1e"), "\x1b]11;rgb:1e1e/1e1e/1e1e\x07"; got != want {
		t.Errorf("restore with a known color = %q, want %q", got, want)
	}
	if got, want := restore(oscText, ""), "\x1b]110\x07"; got != want {
		t.Errorf("restore with an unknown color = %q, want %q", got, want)
	}
}
