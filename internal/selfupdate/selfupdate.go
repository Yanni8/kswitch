// Package selfupdate fetches kswitch releases from GitHub and swaps the
// installed binary for a new one.
package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// maxSize bounds downloads and archive entries.
const maxSize = 256 << 20

// Updater locates and fetches releases.
type Updater struct {
	// BaseURL is the repository URL on GitHub, such as
	// "https://github.com/Yanni8/kswitch".
	BaseURL string
	// OS and Arch select the release asset, in GOOS/GOARCH terms.
	OS, Arch string
	// Client is used for all requests; nil means a default client with
	// a timeout.
	Client *http.Client
}

func (u *Updater) client() *http.Client {
	if u.Client != nil {
		return u.Client
	}
	return &http.Client{Timeout: 2 * time.Minute}
}

// Asset is the name of the release archive for the platform.
func (u *Updater) Asset() string {
	return fmt.Sprintf("kswitch_%s_%s.tar.gz", u.OS, u.Arch)
}

// Latest returns the tag of the latest release, such as "v1.2.3". It
// reads it off the redirect of the repository's releases/latest page,
// which needs no API token and has no rate limit.
func (u *Updater) Latest() (string, error) {
	c := *u.client()
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := c.Do(u.request(u.BaseURL + "/releases/latest"))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	location := resp.Header.Get("Location")
	if resp.StatusCode/100 != 3 || !strings.Contains(location, "/releases/tag/") {
		return "", fmt.Errorf("no release found at %s/releases/latest (HTTP %d)", u.BaseURL, resp.StatusCode)
	}
	return path.Base(location), nil
}

// Download fetches the release archive for version, verifies it against
// the release's checksums.txt, and returns the kswitch binary inside.
func (u *Updater) Download(version string) ([]byte, error) {
	base := fmt.Sprintf("%s/releases/download/%s/", u.BaseURL, version)
	archive, err := u.get(base + u.Asset())
	if err != nil {
		return nil, err
	}
	sums, err := u.get(base + "checksums.txt")
	if err != nil {
		return nil, err
	}
	if err := verify(archive, u.Asset(), sums); err != nil {
		return nil, err
	}
	return extractBinary(archive)
}

func (u *Updater) request(url string) *http.Request {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err) // the URLs are built from constants and validated input
	}
	req.Header.Set("User-Agent", "kswitch-update")
	return req
}

func (u *Updater) get(url string) ([]byte, error) {
	resp, err := u.client().Do(u.request(url))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxSize))
}

// verify checks archive against the SHA-256 listed for name in sums, a
// checksums.txt in sha256sum format.
func verify(archive []byte, name string, sums []byte) error {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != name {
			continue
		}
		sum := sha256.Sum256(archive)
		if got := hex.EncodeToString(sum[:]); got != fields[0] {
			return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", name, fields[0], got)
		}
		return nil
	}
	return fmt.Errorf("%s is not listed in checksums.txt", name)
}

// extractBinary returns the kswitch file inside a .tar.gz archive.
func extractBinary(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("reading the archive: %w", err)
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading the archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && path.Base(hdr.Name) == "kswitch" {
			return io.ReadAll(io.LimitReader(tr, maxSize))
		}
	}
	return nil, errors.New("the archive does not contain a kswitch binary")
}

// Replace swaps the file at exe for binary, keeping its permissions. The
// new file is written next to exe and renamed over it, so a failure
// never leaves a half-written executable behind.
func Replace(exe string, binary []byte) error {
	mode := os.FileMode(0o755)
	if info, err := os.Stat(exe); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".kswitch-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), exe)
}
