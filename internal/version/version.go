// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

// Package version holds build metadata injected at link time.
package version

import "runtime/debug"

// Overridden via -ldflags, see the Makefile:
//
//	-X github.com/Yanni8/kubeswitch/internal/version.version=v1.2.3
var (
	version = "dev"
	commit  = ""
	date    = ""
)

// Version returns the bare version, such as "v1.2.3", or "dev" for a
// build without one.
func Version() string {
	v := version
	if v == "dev" {
		// Fall back to module info so `go install ...@vX.Y.Z` builds
		// still report their version without ldflags.
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}
	return v
}

// String returns a human-readable version string.
func String() string {
	v := Version()
	if commit != "" {
		v += " (" + commit + ")"
	}
	if date != "" {
		v += " built " + date
	}
	return v
}
