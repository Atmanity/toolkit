#!/bin/sh
# Installs the atmee CLI from Atmanity/toolkit's releases, checked against the
# release's checksums.txt.
#
#   curl -fsSL https://raw.githubusercontent.com/Atmanity/toolkit/main/install.sh | sh
#
# While the repo is members-only, plain downloads fail; a signed-in gh fetches
# the release instead, and the script itself comes from:
#
#   gh api repos/Atmanity/toolkit/contents/install.sh -H 'Accept: application/vnd.github.raw' | sh
#
#   ATMEE_VERSION      a release such as 0.11.0 (default: the latest)
#   ATMEE_INSTALL_DIR  where the binary goes (default: ~/.local/bin)
set -eu

repo="Atmanity/toolkit"
dir="${ATMEE_INSTALL_DIR:-$HOME/.local/bin}"

fail() { echo "atmee install: $*" >&2; exit 1; }

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "no build for $(uname -s); on Windows download atmee_windows_amd64.zip from https://github.com/$repo/releases/latest" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "no build for $(uname -m)" ;;
esac

tag=""
[ -n "${ATMEE_VERSION:-}" ] && tag="atmee--v${ATMEE_VERSION#v}"
if [ -n "${ATMEE_DOWNLOAD_BASE:-}" ]; then
  base="$ATMEE_DOWNLOAD_BASE"   # tests: a folder of release assets
elif [ -n "$tag" ]; then
  base="https://github.com/$repo/releases/download/$tag"
else
  base="https://github.com/$repo/releases/latest/download"
fi
asset="atmee_${os}_${arch}.tar.gz"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

# A plain download, else (a members-only repo) the release through gh's sign-in.
if command -v curl > /dev/null 2>&1 &&
  curl -fsSL "$base/$asset" -o "$tmp/$asset" 2> /dev/null &&
  curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" 2> /dev/null; then
  :
elif [ -z "${ATMEE_DOWNLOAD_BASE:-}" ] && command -v gh > /dev/null 2>&1 &&
  gh release download $tag -R "$repo" -p "$asset" -p checksums.txt -D "$tmp" --clobber > /dev/null 2>&1; then
  :
else
  fail "could not download $base/$asset; while $repo is members-only, install gh and run gh auth login, then try again"
fi

want="$(awk -v f="$asset" '$2 == f { print $1 }' "$tmp/checksums.txt")"
[ -n "$want" ] || fail "checksums.txt has no line for $asset"
if command -v sha256sum > /dev/null 2>&1; then
  got="$(sha256sum "$tmp/$asset" | awk '{ print $1 }')"
else
  got="$(shasum -a 256 "$tmp/$asset" | awk '{ print $1 }')"
fi
[ "$want" = "$got" ] || fail "checksum mismatch for $asset"

tar -xzf "$tmp/$asset" -C "$tmp" atmee
mkdir -p "$dir"
mv "$tmp/atmee" "$dir/atmee"
chmod +x "$dir/atmee"

echo "Installed $("$dir/atmee" --version) to $dir/atmee"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) echo "Add $dir to your PATH, for example: export PATH=\"$dir:\$PATH\"" ;;
esac
