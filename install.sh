#!/bin/sh
set -eu

fail() { printf 'flaky installer: %s\n' "$*" >&2; exit 1; }
download() { curl -fsSL "$@" || fail 'download failed'; }

case "$(uname -s)" in
  Linux) platform=linux ;;
  Darwin) platform=darwin ;;
  *) fail 'unsupported OS; use a release binary for your platform' ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) fail 'unsupported architecture; supported: amd64, arm64' ;;
esac
command -v curl >/dev/null 2>&1 || fail 'curl is required'
command -v tar >/dev/null 2>&1 || fail 'tar is required'
if command -v sha256sum >/dev/null 2>&1; then
  checksum_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  checksum_tool=shasum
else
  fail 'sha256sum or shasum is required'
fi

install_dir=${FLAKY_INSTALL_DIR:-"$HOME/.local/bin"}
case "$install_dir" in /*) ;; *) fail 'FLAKY_INSTALL_DIR must be an absolute path' ;; esac
temp_dir=
staged=
cleanup() {
  if [ -n "$staged" ]; then rm -f "$staged"; fi
  if [ -n "$temp_dir" ]; then rm -rf "$temp_dir"; fi
}
trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
temp_dir=$(mktemp -d)

release=$(download https://api.github.com/repos/dpologdvinity/flaky/releases/latest)
tag=$(printf '%s\n' "$release" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
printf '%s\n' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || fail 'invalid release version'
version=${tag#v}
archive="flaky_${version}_${platform}_${arch}.tar.gz"
base="https://github.com/dpologdvinity/flaky/releases/download/$tag"
download -o "$temp_dir/checksums.txt" "$base/checksums.txt"
download -o "$temp_dir/$archive" "$base/$archive"
expected=$(awk -v name="$archive" '$2 == name { print $1 }' "$temp_dir/checksums.txt")
[ "${#expected}" -eq 64 ] || fail 'missing or invalid release checksum'
case "$expected" in *[!a-fA-F0-9]*) fail 'invalid release checksum' ;; esac
if [ "$checksum_tool" = sha256sum ]; then
  actual=$(sha256sum "$temp_dir/$archive")
else
  actual=$(shasum -a 256 "$temp_dir/$archive")
fi
[ "${actual%% *}" = "$expected" ] || fail 'checksum mismatch; installation unchanged'
tar -xzf "$temp_dir/$archive" -C "$temp_dir" flaky
[ -f "$temp_dir/flaky" ] && [ ! -L "$temp_dir/flaky" ] || fail 'archive does not contain a regular executable'

mkdir -p "$install_dir"
[ ! -d "$install_dir/flaky" ] || fail 'installation target is a directory'
staged=$(mktemp "$install_dir/.flaky.XXXXXX")
cp "$temp_dir/flaky" "$staged"
chmod 0755 "$staged"
mv -f "$staged" "$install_dir/flaky"
staged=
printf 'Installed flaky %s to %s/flaky\n' "$version" "$install_dir"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) printf 'Add %s to your PATH.\n' "$install_dir" ;;
esac
