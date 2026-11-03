package terminal

import "testing"

func TestParseReply(t *testing.T) {
	tests := []struct {
		name  string
		reply string
		want  string
		ok    bool
	}{
		{"BEL terminated", "\x1b]11;rgb:1e1e/1e1e/1e1e\x07", "rgb:1e1e/1e1e/1e1e", true},
		{"ST terminated", "\x1b]11;rgb:1e1e/1e1e/1e1e\x1b\\", "rgb:1e1e/1e1e/1e1e", true},
		{"leading noise", "junk\x1b]11;#000000\x07", "#000000", true},
		{"incomplete", "\x1b]11;rgb:1e1e/1e", "", false},
		{"nothing yet", "", "", false},
		{"unrelated", "\x1b[?1;2c", "", false},
		{"empty spec", "\x1b]11;\x07", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseReply([]byte(tt.reply))
			if got != tt.want || ok != tt.ok {
				t.Errorf("parseReply(%q) = %q, %v; want %q, %v", tt.reply, got, ok, tt.want, tt.ok)
			}
		})
	}
}
