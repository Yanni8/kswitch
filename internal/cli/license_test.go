// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"runtime/debug"
	"strings"
	"testing"

	"github.com/Yanni8/kubeswitch/internal/licenses"
)

func TestLicensePrintsWithoutTerminal(t *testing.T) {
	out, err := execute(t, "", "license")
	if err != nil {
		t.Fatalf("license failed: %v", err)
	}
	for _, want := range []string{
		"Copyright",
		"kubeswitch — LICENSE",
		"Apache License",
		"github.com/spf13/cobra",
		"gopkg.in/yaml.v3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
}

// Every module linked into the binary must ship with its license.
func TestEveryLinkedModuleHasALicense(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Skip("no build info")
	}
	deps, err := licenses.Dependencies()
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, d := range deps {
		have[d.Module] = true
	}
	for _, d := range info.Deps {
		if !have[d.Path] {
			t.Errorf("no license embedded for %s; run \"make licenses\"", d.Path)
		}
	}
}
