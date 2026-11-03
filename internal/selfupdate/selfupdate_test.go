// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// archive builds a .tar.gz holding the given files.
func archive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// release serves a fake GitHub release: latest redirects to tag, and the
// archive plus checksums are downloadable for it.
func release(t *testing.T, tag string, asset string, data []byte, sums string) *Updater {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if tag == "" {
			http.Redirect(w, r, srv.URL+"/releases", http.StatusFound)
			return
		}
		http.Redirect(w, r, srv.URL+"/releases/tag/"+tag, http.StatusFound)
	})
	mux.HandleFunc("/releases/download/"+tag+"/"+asset, func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	})
	mux.HandleFunc("/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sums))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &Updater{BaseURL: srv.URL, OS: "linux", Arch: "amd64"}
}

func TestLatest(t *testing.T) {
	u := release(t, "v1.2.3", "kubeswitch_linux_amd64.tar.gz", nil, "")
	if got, err := u.Latest(); err != nil || got != "v1.2.3" {
		t.Errorf("Latest = %q, %v; want v1.2.3", got, err)
	}

	u = release(t, "", "x", nil, "")
	if _, err := u.Latest(); err == nil || !strings.Contains(err.Error(), "no release found") {
		t.Errorf("without releases: got %v", err)
	}
}

func TestDownload(t *testing.T) {
	data := archive(t, map[string]string{"kubeswitch": "#!/bin/sh\necho new\n", "README.md": "docs"})
	u := release(t, "v1.2.3", "kubeswitch_linux_amd64.tar.gz", data, sha(data)+"  kubeswitch_linux_amd64.tar.gz\n")

	got, err := u.Download("v1.2.3")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if string(got) != "#!/bin/sh\necho new\n" {
		t.Errorf("binary = %q", got)
	}
	if _, err := u.Download("v9.9.9"); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("unknown version: got %v", err)
	}
}

func TestDownloadRejectsBadChecksum(t *testing.T) {
	data := archive(t, map[string]string{"kubeswitch": "new"})
	u := release(t, "v1.2.3", "kubeswitch_linux_amd64.tar.gz", data, strings.Repeat("0", 64)+"  kubeswitch_linux_amd64.tar.gz\n")
	if _, err := u.Download("v1.2.3"); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("got %v, want a checksum mismatch", err)
	}

	u = release(t, "v1.2.3", "kubeswitch_linux_amd64.tar.gz", data, sha(data)+"  other.tar.gz\n")
	if _, err := u.Download("v1.2.3"); err == nil || !strings.Contains(err.Error(), "not listed") {
		t.Errorf("got %v, want an unlisted-asset error", err)
	}
}

func TestExtractBinary(t *testing.T) {
	if _, err := extractBinary(archive(t, map[string]string{"README.md": "docs"})); err == nil || !strings.Contains(err.Error(), "does not contain") {
		t.Errorf("archive without binary: got %v", err)
	}
	if _, err := extractBinary([]byte("not a tarball")); err == nil {
		t.Error("garbage must be rejected")
	}
	got, err := extractBinary(archive(t, map[string]string{"./kubeswitch": "bin"}))
	if err != nil || string(got) != "bin" {
		t.Errorf("nested name: got %q, %v", got, err)
	}
}

func TestReplace(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "kubeswitch")
	if err := os.WriteFile(exe, []byte("old"), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := Replace(exe, []byte("new")); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if data, _ := os.ReadFile(exe); string(data) != "new" {
		t.Errorf("content = %q", data)
	}
	if info, _ := os.Stat(exe); info.Mode().Perm() != 0o750 {
		t.Errorf("mode = %o, want the original 750", info.Mode().Perm())
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".kubeswitch-update-*")); len(left) != 0 {
		t.Errorf("temp files left behind: %v", left)
	}
}

func TestReplaceReportsPermissionErrors(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "kubeswitch")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	err := Replace(exe, []byte("new"))
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("got %v, want a permission error", err)
	}
	if data, _ := os.ReadFile(exe); string(data) != "old" {
		t.Error("the binary must be untouched after a failure")
	}
}
