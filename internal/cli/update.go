package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/Yanni8/kubeswitch/internal/selfupdate"
	"github.com/Yanni8/kubeswitch/internal/version"
)

// releaseRepoURL is where releases are published.
const releaseRepoURL = "https://github.com/Yanni8/kubeswitch"

// Overridable in tests.
var (
	updateBaseURL  = releaseRepoURL
	executablePath = os.Executable
)

func newUpdateCmd() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "update [version]",
		Short: "Update kubeswitch to the latest release",
		Long: `Update kubeswitch itself.

Downloads the latest release (or the given version, such as v1.2.3) for
this platform, verifies it against the release's checksums, and replaces
the running binary in place. If the binary's directory is not writable,
run "sudo kubeswitch update".`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd, args, check)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "only report whether a newer release exists")
	return cmd
}

func runUpdate(cmd *cobra.Command, args []string, check bool) error {
	out := cmd.OutOrStdout()
	if runtime.GOOS == "windows" {
		return fmt.Errorf("updating in place is not supported on Windows yet; download the new release from %s/releases", releaseRepoURL)
	}

	base := updateBaseURL
	if v := os.Getenv("KUBESWITCH_UPDATE_URL"); v != "" { // mirrors and tests
		base = v
	}
	u := &selfupdate.Updater{BaseURL: base, OS: runtime.GOOS, Arch: runtime.GOARCH}
	current := version.Version()

	var target string
	if len(args) == 1 {
		target = args[0]
		if target != "" && target[0] >= '0' && target[0] <= '9' {
			target = "v" + target // tags carry a v prefix
		}
		if target == current {
			fmt.Fprintf(out, "kubeswitch %s is already installed.\n", current)
			return nil
		}
	} else {
		latest, err := u.Latest()
		if err != nil {
			return fmt.Errorf("checking for the latest release: %w", err)
		}
		if latest == current {
			fmt.Fprintf(out, "kubeswitch %s is already the latest version.\n", current)
			return nil
		}
		target = latest
	}

	if check {
		fmt.Fprintf(out, "kubeswitch %s is available; you have %s. Run \"kubeswitch update\" to install it.\n", target, current)
		return nil
	}

	exe, err := executablePath()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	fmt.Fprintf(out, "Downloading kubeswitch %s for %s/%s...\n", target, runtime.GOOS, runtime.GOARCH)
	binary, err := u.Download(target)
	if err != nil {
		return err
	}
	if err := selfupdate.Replace(exe, binary); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf("cannot replace %s: %w\nTry: sudo kubeswitch update", exe, err)
		}
		return err
	}
	fmt.Fprintf(out, "Updated kubeswitch %s to %s at %s\n", current, target, exe)

	// Show that the new binary runs.
	if v, err := exec.Command(exe, "version").Output(); err == nil {
		fmt.Fprintf(out, "Now running %s", v)
	}
	return nil
}
