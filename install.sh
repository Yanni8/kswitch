#!/bin/sh
# Installs kubeswitch on Linux or macOS with a single command:
#
#   curl -fsSL https://raw.githubusercontent.com/Yanni8/kubeswitch/main/install.sh | sh
#
# It downloads the release archive for this machine, verifies it against
# the release's checksums.txt, and installs the binary.
#
# Settings (environment variables):
#   KUBESWITCH_VERSION      release to install, e.g. v1.2.3 (default: the latest)
#   KUBESWITCH_INSTALL_DIR  where to put the binary (default: /usr/local/bin)
#   KUBESWITCH_REPO         GitHub repository to download from (default: Yanni8/kubeswitch)
#   KUBESWITCH_BASE_URL     base URL of the release assets, for mirrors and tests
set -eu

repo="${KUBESWITCH_REPO:-Yanni8/kubeswitch}"
version="${KUBESWITCH_VERSION:-${1:-}}"
install_dir="${KUBESWITCH_INSTALL_DIR:-/usr/local/bin}"

say() { printf '%s\n' "$*"; }
fail() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

# Which archive fits this machine?
os=$(uname -s)
case "$os" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "unsupported operating system: $os. Linux and macOS are supported; Windows binaries can be downloaded from https://github.com/$repo/releases" ;;
esac
arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported architecture: $arch (amd64 and arm64 are supported)" ;;
esac
asset="kubeswitch_${os}_${arch}.tar.gz"

# Where to download it from.
case "$version" in
  [0-9]*) version="v$version" ;; # tags carry a v prefix
esac
if [ -n "$version" ]; then
  base="https://github.com/$repo/releases/download/$version"
else
  base="https://github.com/$repo/releases/latest/download"
fi
base="${KUBESWITCH_BASE_URL:-$base}"

# Tools: curl or wget, sha256sum (Linux) or shasum (macOS).
if command -v curl >/dev/null 2>&1; then
  download() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  download() { wget -qO "$2" "$1"; }
else
  fail "curl or wget is required"
fi
if command -v sha256sum >/dev/null 2>&1; then
  checksum() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
  checksum() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
  fail "sha256sum or shasum is required to verify the download"
fi

# Runs a command as root when the install directory is not writable.
as_root() {
  if [ "$(id -u)" = 0 ]; then
    "$@"
    return
  fi
  command -v sudo >/dev/null 2>&1 ||
    fail "$install_dir is not writable and sudo is not available; set KUBESWITCH_INSTALL_DIR to a writable directory"
  say "Using sudo to write to $install_dir"
  sudo "$@"
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "Downloading $asset from $base"
download "$base/$asset" "$tmp/$asset" ||
  fail "could not download $base/$asset (is there a release for ${version:-the latest version}?)"
download "$base/checksums.txt" "$tmp/checksums.txt" ||
  fail "could not download $base/checksums.txt"

expected=$(grep " $asset\$" "$tmp/checksums.txt" | cut -d' ' -f1)
[ -n "$expected" ] || fail "$asset is not listed in checksums.txt"
actual=$(checksum "$tmp/$asset")
[ "$actual" = "$expected" ] || fail "checksum mismatch for $asset: expected $expected, got $actual"
say "Checksum verified"

tar -xzf "$tmp/$asset" -C "$tmp" kubeswitch

if [ ! -d "$install_dir" ]; then
  mkdir -p "$install_dir" 2>/dev/null || as_root mkdir -p "$install_dir"
fi
if [ -w "$install_dir" ]; then
  install -m 755 "$tmp/kubeswitch" "$install_dir/kubeswitch"
else
  as_root install -m 755 "$tmp/kubeswitch" "$install_dir/kubeswitch"
fi

say "Installed kubeswitch $("$install_dir/kubeswitch" version) to $install_dir/kubeswitch"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) say "Note: $install_dir is not in your PATH. Add it with: export PATH=\"$install_dir:\$PATH\"" ;;
esac
