package kubeconfig

import (
	"strings"
	"testing"
)

func TestParseSuggestedName(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{
			"current-context",
			"current-context: ctx-b\ncontexts:\n  - name: ctx-a\n  - name: ctx-b\n",
			"ctx-b",
		},
		{
			"first context",
			"contexts:\n  - name: ctx-a\n  - name: ctx-b\n",
			"ctx-a",
		},
		{
			"first cluster",
			"kind: Config\nclusters:\n  - name: cl-a\n",
			"cl-a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Parse([]byte(tt.in))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got := cfg.SuggestedName(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	tests := []struct{ name, in, wantErr string }{
		{"invalid yaml", "clusters: [\n", "not valid YAML"},
		{"scalar", "just a string\n", "not valid YAML"},
		{"wrong kind", "kind: Deployment\nclusters:\n  - name: x\n", `kind is "Deployment"`},
		{"not a kubeconfig", "foo: bar\n", "does not look like a kubeconfig"},
		{"empty lists", "clusters: []\ncontexts: []\n", "does not look like a kubeconfig"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.in))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}
