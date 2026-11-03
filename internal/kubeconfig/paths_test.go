// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package kubeconfig

import (
	"strings"
	"testing"
)

func TestRelativePaths(t *testing.T) {
	got := RelativePaths([]byte(sampleKubeconfig))
	if want := "certs/admin.key,certs/ca.crt"; strings.Join(got, ",") != want {
		t.Errorf("RelativePaths = %v, want %s (absolute and inline data must be ignored)", got, want)
	}

	if got := RelativePaths([]byte("clusters:\n- name: x\n  cluster:\n    server: https://x\n")); len(got) != 0 {
		t.Errorf("no file references: got %v", got)
	}
	if got := RelativePaths([]byte("not: [valid")); got != nil {
		t.Errorf("invalid YAML: got %v, want nil", got)
	}
}
