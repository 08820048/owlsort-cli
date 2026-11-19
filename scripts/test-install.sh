#!/bin/sh
# Test install.sh end to end, without a network release.
#
#   sh scripts/test-install.sh
#
# It builds a fake release out of dist/ (or from the tree, if dist/ is empty),
# serves it from a local HTTP server, and runs the installer against it. The point
# is the checksum step: install.sh is piped into `sh`, so a mistake there is the
# most expensive bug in the repository, and "it looked right" is not a test.
#
# Needs: go, python3 or python, tar, a POSIX sh.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/owlsort-install-test.XXXXXX")
server=""
cleanup() {
	[ -n "$server" ] && kill "$server" 2>/dev/null || true
	rm -rf "$work"
}
trap cleanup EXIT INT TERM

fail() {
	echo "FAIL: $*" >&2
	exit 1
}
pass() { echo "  ok    $*"; }

# --- a fake release ----------------------------------------------------------

os=$(uname -s | tr '[:upper:]' '[:lower:]')
[ "$os" = "linux" ] || os=darwin
case "$(uname -m)" in
arm64 | aarch64) arch=arm64 ;;
*) arch=amd64 ;;
esac
version="v9.9.9"
bare=${version#v}
archive="owlsort_${bare}_${os}_${arch}.tar.gz"
release="$work/release/$version"
mkdir -p "$release"

echo "Building a binary for the fake release"
(
	cd "$root"
	CGO_ENABLED=0 go build -ldflags "-X main.version=$version" -o "$work/owlsort" ./cmd/owlsort
)
mkdir -p "$work/pkg"
cp "$work/owlsort" "$work/pkg/owlsort"
cp "$root/LICENSE" "$work/pkg/" 2>/dev/null || true
tar czf "$release/$archive" -C "$work/pkg" .
(cd "$release" && sha256sum "$archive" >checksums.txt 2>/dev/null ||
	shasum -a 256 "$archive" >checksums.txt)

# --- serve it ----------------------------------------------------------------

port=8099
python=$(command -v python3 || command -v python || true)
[ -n "$python" ] || fail "no python3 or python to serve the fake release from"
"$python" -m http.server "$port" --directory "$work/release" >"$work/server.log" 2>&1 &
server=$!
# Wait for the port rather than sleeping a fixed time: a fixed sleep is either
# flaky on a slow machine or slow on a fast one.
tries=0
until "$python" -c "import urllib.request,sys; urllib.request.urlopen('http://127.0.0.1:$port/', timeout=1)" 2>/dev/null; do
	tries=$((tries + 1))
	[ "$tries" -lt 50 ] || fail "the fake release server never came up"
	sleep 0.1 2>/dev/null || sleep 1
done
export OWLSORT_BASE_URL="http://127.0.0.1:$port/$version"

# --- 1. a good install -------------------------------------------------------

echo "Installing from the fake release"
target="$work/bin"
if ! sh "$root/install.sh" --version "$version" --dir "$target" >"$work/install.log" 2>&1; then
	cat "$work/install.log" >&2
	fail "the installer failed on a release that is correct"
fi
[ -x "$target/owlsort" ] || fail "no binary was installed"
got=$("$target/owlsort" --version)
case "$got" in
*"$version"*) pass "installed and reports $version" ;;
*) fail "the installed binary reports '$got'" ;;
esac
if ls -a "$target" | grep -q '^\.owlsort\.new$'; then
	fail "a temporary file was left beside the binary"
fi
pass "no temporary file left behind"

# --- 2. a tampered archive is refused ---------------------------------------

echo "Corrupting the archive"
cp "$release/$archive" "$work/archive.bak"
printf 'extra bytes' >>"$release/$archive"
if sh "$root/install.sh" --version "$version" --dir "$work/bin2" >"$work/bad.log" 2>&1; then
	cp "$work/archive.bak" "$release/$archive"
	fail "the installer accepted a download that does not match its checksum"
fi
if [ -e "$work/bin2/owlsort" ]; then
	cp "$work/archive.bak" "$release/$archive"
	fail "a binary was installed from a download that does not match its checksum"
fi
grep -qi "checksum" "$work/bad.log" || fail "the error does not mention the checksum"
pass "a corrupted download is refused and nothing is installed"
cp "$work/archive.bak" "$release/$archive"

# --- 3. a missing checksums file is refused ---------------------------------

echo "Removing checksums.txt"
mv "$release/checksums.txt" "$work/checksums.bak"
if sh "$root/install.sh" --version "$version" --dir "$work/bin3" >"$work/nock.log" 2>&1; then
	mv "$work/checksums.bak" "$release/checksums.txt"
	fail "the installer installed without checksums"
fi
[ ! -e "$work/bin3/owlsort" ] || {
	mv "$work/checksums.bak" "$release/checksums.txt"
	fail "a binary was installed without checksums"
}
pass "no checksums means no install"
mv "$work/checksums.bak" "$release/checksums.txt"

# --- 4. it does not eat its own stdin ---------------------------------------

echo "Piping the script into sh, the way the docs say to"
# A `read` anywhere in install.sh would consume this line and the rest of the
# script with it on some shells. This runs it the documented way to prove it.
if ! printf '' | sh "$root/install.sh" --version "$version" --dir "$work/bin4" >"$work/pipe.log" 2>&1; then
	cat "$work/pipe.log" >&2
	fail "the installer failed when piped into sh"
fi
[ -x "$work/bin4/owlsort" ] || fail "no binary from the piped run"
pass "piped into sh, it still installs"

# --- 5. bad arguments ------------------------------------------------------

if sh "$root/install.sh" --nope >"$work/args.log" 2>&1; then
	fail "an unknown argument was accepted"
fi
pass "an unknown argument is refused"

echo
echo "install.sh: all checks passed"
