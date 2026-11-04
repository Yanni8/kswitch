// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeBinary stands in for a released kubeswitch: it reports its version.
const fakeBinary = "#!/bin/sh\necho \"v9.9.9 (fake)\"\n"

// fakeRelease serves a GitHub-like release of tag for this platform, with
// checksums that are valid unless corrupt is set, and points the update
// command at it and at a throwaway "installed" binary, whose path it
// returns.
func fakeRelease(t *testing.T, tag string, corrupt bool) string {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "kubeswitch", Mode: 0o755, Size: int64(len(fakeBinary)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	tw.Write([]byte(fakeBinary))
	tw.Close()
	gz.Close()
	archive := buf.Bytes()
	asset := "kubeswitch_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	sum := sha256.Sum256(archive)
	checksum := hex.EncodeToString(sum[:])
	if corrupt {
		checksum = strings.Repeat("0", 64)
	}

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/releases/tag/"+tag, http.StatusFound)
	})
	mux.HandleFunc("/releases/download/"+tag+"/"+asset, func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	mux.HandleFunc("/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksum + "  " + asset + "\n"))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	exe := filepath.Join(t.TempDir(), "kubeswitch")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	origURL, origExe := updateBaseURL, executablePath
	updateBaseURL = srv.URL
	executablePath = func() (string, error) { return exe, nil }
	t.Cleanup(func() { updateBaseURL, executablePath = origURL, origExe })
	return exe
}

func TestUpdateInstallsLatest(t *testing.T) {
	exe := fakeRelease(t, "v9.9.9", false)

	out, err := execute(t, "", "update")
	if err != nil {
		t.Fatalf("update failed: %v\n%s", err, out)
	}
	for _, want := range []string{
		"Downloading kubeswitch v9.9.9 for " + runtime.GOOS + "/" + runtime.GOARCH,
		"Updated kubeswitch dev to v9.9.9 at " + exe,
		"Now running v9.9.9 (fake)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if data, _ := os.ReadFile(exe); string(data) != fakeBinary {
		t.Errorf("binary not replaced: %q", data)
	}
}

func TestUpdateToGivenVersion(t *testing.T) {
	exe := fakeRelease(t, "v9.9.9", false)

	out, err := execute(t, "", "update", "9.9.9") // the v prefix is optional
	if err != nil {
		t.Fatalf("update failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Updated kubeswitch dev to v9.9.9") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if data, _ := os.ReadFile(exe); string(data) != fakeBinary {
		t.Errorf("binary not replaced: %q", data)
	}
}

func TestUpdateCheckOnly(t *testing.T) {
	exe := fakeRelease(t, "v9.9.9", false)

	out, err := execute(t, "", "update", "--check")
	if err != nil {
		t.Fatalf("update failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "kubeswitch v9.9.9 is available; you have dev.") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if data, _ := os.ReadFile(exe); strings.Contains(string(data), "fake") {
		t.Error("--check must not install anything")
	}
}

func TestUpdateAlreadyCurrent(t *testing.T) {
	// The test binary reports version "dev", so a "dev" release is current.
	fakeRelease(t, "dev", false)

	out, err := execute(t, "", "update")
	if err != nil {
		t.Fatalf("update failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "kubeswitch dev is already the latest version.") {
		t.Errorf("unexpected output:\n%s", out)
	}

	out, err = execute(t, "", "update", "dev")
	if err != nil || !strings.Contains(out, "kubeswitch dev is already installed.") {
		t.Errorf("explicit current version: got %v\n%s", err, out)
	}
}

func TestUpdateRejectsBadChecksum(t *testing.T) {
	exe := fakeRelease(t, "v9.9.9", true)

	_, err := execute(t, "", "update")
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected a checksum error, got %v", err)
	}
	if data, _ := os.ReadFile(exe); string(data) != "#!/bin/sh\necho old\n" {
		t.Error("the binary must be untouched after a failed verification")
	}
}

func TestUpdateSuggestsSudo(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	exe := fakeRelease(t, "v9.9.9", false)
	dir := filepath.Dir(exe)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	_, err := execute(t, "", "update")
	if err == nil || !strings.Contains(err.Error(), "sudo kubeswitch update") {
		t.Fatalf("expected a sudo hint, got %v", err)
	}
}

func TestUpdateLeavesHomebrewInstallsToBrew(t *testing.T) {
	fakeRelease(t, "v9.9.9", false)
	// Homebrew links bin/kubeswitch to the binary in its cellar.
	root := t.TempDir()
	cellar := filepath.Join(root, "Cellar", "kubeswitch", "1.0.0", "bin")
	if err := os.MkdirAll(cellar, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(cellar, "kubeswitch")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "bin", "kubeswitch")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(exe, link); err != nil {
		t.Fatal(err)
	}
	executablePath = func() (string, error) { return link, nil } // restored by fakeRelease's cleanup

	_, err := execute(t, "", "update")
	if err == nil || !strings.Contains(err.Error(), "brew upgrade kubeswitch") {
		t.Fatalf("expected a Homebrew hint, got %v", err)
	}
	if data, _ := os.ReadFile(exe); string(data) != "#!/bin/sh\necho old\n" {
		t.Error("a Homebrew-managed binary must not be replaced")
	}

	out, err := execute(t, "", "update", "--check")
	if err != nil {
		t.Fatalf("update --check failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `Run "brew upgrade kubeswitch" to install it.`) {
		t.Errorf("unexpected output:\n%s", out)
	}
}
