#!/bin/sh
# Install owlsort from a GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/08820048/owlsort-cli/main/install.sh | sh
#
#   sh install.sh --version v0.1.0 --dir ~/.local/bin
#
# It downloads the archive for this machine, checks it against the release's
# checksums.txt, and only then puts the binary in place. A download that does not
# match is thrown away and nothing is installed: this tool's whole promise is that
# it does not damage what you already have, and the installer should not be the
# one place that breaks that.
#
# This script never reads its own standard input. It is meant to be piped into `sh`,
# and a `read` here would swallow the rest of the script on some shells.
#
# Set OWLSORT_BASE_URL to download from a mirror instead of GitHub. It is also how
# scripts/test-install.sh exercises this file end to end without a network release.
set -eu

REPO="08820048/owlsort-cli"
BINARY="owlsort"

version=""
dir=""

usage() {
	cat <<'EOF'
Install owlsort.

  --version <tag>   install this version (default: the newest release)
  --dir <path>      install here (default: /usr/local/bin, else ~/.local/bin)
  --help            this text

The binary is checked against the release's checksums.txt before it is installed.
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
	--version)
		[ $# -ge 2 ] || { echo "install.sh: --version needs a value" >&2; exit 2; }
		version="$2"
		shift 2
		;;
	--version=*)
		version="${1#--version=}"
		shift
		;;
	--dir)
		[ $# -ge 2 ] || { echo "install.sh: --dir needs a value" >&2; exit 2; }
		dir="$2"
		shift 2
		;;
	--dir=*)
		dir="${1#--dir=}"
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "install.sh: unknown argument: $1" >&2
		usage >&2
		exit 2
		;;
	esac
done

# --- what machine is this ----------------------------------------------------

os=$(uname -s)
case "$os" in
Darwin) os=darwin ;;
Linux) os=linux ;;
*)
	echo "install.sh: $os is not supported by this script." >&2
	echo "  On Windows, download the .zip from the release page." >&2
	echo "  Everywhere else: go install github.com/$REPO/cmd/owlsort@latest" >&2
	exit 1
	;;
esac

arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*)
	echo "install.sh: $arch is not supported." >&2
	exit 1
	;;
esac

# --- where it goes ------------------------------------------------------------

if [ -z "$dir" ]; then
	if [ -w /usr/local/bin ] 2>/dev/null; then
		dir=/usr/local/bin
	else
		dir="$HOME/.local/bin"
	fi
fi

# --- which version ------------------------------------------------------------

if [ -z "$version" ]; then
	# The redirect on the whole `curl` matters: a 404 without -f would be read as
	# the answer and the tag would come out as an HTML error page.
	version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
		sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
	if [ -z "$version" ]; then
		echo "install.sh: could not find the newest release of $REPO." >&2
		echo "  Pass --version if you know which one you want." >&2
		exit 1
	fi
fi

# The archives are named without the leading "v" of the tag.
bare=${version#v}

archive="${BINARY}_${bare}_${os}_${arch}.tar.gz"
base="${OWLSORT_BASE_URL:-https://github.com/$REPO/releases/download/$version}"

# --- download, check, install --------------------------------------------------

tmp=$(mktemp -d "${TMPDIR:-/tmp}/owlsort-install.XXXXXX")
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "Downloading $archive ($version, $os/$arch)"
if ! curl -fsSL -o "$tmp/$archive" "$base/$archive"; then
	echo "install.sh: could not download $base/$archive" >&2
	echo "  Check the version, and that a release exists for $os/$arch." >&2
	exit 1
fi
if ! curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"; then
	echo "install.sh: could not download the checksums; refusing to install unverified." >&2
	exit 1
fi

echo "Checking the download against checksums.txt"
expected=$(awk -v name="$archive" '$2 == name { print $1 }' "$tmp/checksums.txt")
if [ -z "$expected" ]; then
	echo "install.sh: $archive is not in checksums.txt; refusing to install." >&2
	exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$tmp/$archive" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$tmp/$archive" | awk '{ print $1 }')
else
	echo "install.sh: no sha256sum or shasum to check the download with; refusing to install." >&2
	exit 1
fi
if [ "$actual" != "$expected" ]; then
	echo "install.sh: the download does not match its checksum; nothing was installed." >&2
	echo "  expected $expected" >&2
	echo "  got      $actual" >&2
	exit 1
fi

extract="$tmp/extract"
mkdir -p "$extract"
tar xzf "$tmp/$archive" -C "$extract"
if [ ! -f "$extract/$BINARY" ]; then
	echo "install.sh: the archive does not contain $BINARY" >&2
	exit 1
fi

mkdir -p "$dir"
# Install beside the target and rename over it, so an interrupted install cannot
# leave a half-written binary where a working one used to be.
if ! cp "$extract/$BINARY" "$dir/.$BINARY.new" 2>/dev/null; then
	echo "install.sh: cannot write to $dir." >&2
	echo "  Try: sh install.sh --dir \"\$HOME/.local/bin\"" >&2
	exit 1
fi
chmod 0755 "$dir/.$BINARY.new"
mv -f "$dir/.$BINARY.new" "$dir/$BINARY"

echo
echo "Installed $("$dir/$BINARY" --version) in $dir"

case ":$PATH:" in
*":$dir:"*) ;;
*)
	echo
	echo "$dir is not in your PATH. Add it:"
	echo "  export PATH=\"$dir:\$PATH\""
	;;
esac

case "$os" in
darwin)
	if xattr -p com.apple.quarantine "$dir/$BINARY" >/dev/null 2>&1; then
		echo
		echo "macOS says this binary was downloaded from the internet. If it refuses to"
		echo "run, allow it once with:"
		echo "  xattr -d com.apple.quarantine \"$dir/$BINARY\""
	fi
	;;
esac

echo
echo "Next: owlsort init && owlsort preview"
