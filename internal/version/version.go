// Package version holds build metadata injected at link time.
package version

import "runtime/debug"

// Overridden via -ldflags, see the Makefile:
//
//	-X github.com/Yanni8/kswitch/internal/version.version=v1.2.3
var (
	version = "dev"
	commit  = ""
	date    = ""
)

// String returns a human-readable version string.
func String() string {
	v := version
	if v == "dev" {
		// Fall back to module info so `go install ...@vX.Y.Z` builds
		// still report their version without ldflags.
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}
	if commit != "" {
		v += " (" + commit + ")"
	}
	if date != "" {
		v += " built " + date
	}
	return v
}
