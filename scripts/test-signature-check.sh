#!/bin/sh
# Test scripts/verify-macos-signature.sh against a real, unsigned Go build.
#
#   sh scripts/test-signature-check.sh
#
# The check exists to stop a release that ships an unsigned binary. Its failure
# case is therefore the one that matters: given a binary straight out of `go
# build`, it must report the problems and exit non-zero. If it ever passes there,
# the release guard has become decoration and nobody would know.
#
# Its success case cannot be tested without a Developer ID certificate and Apple's
# notarization service, so it is not pretended here. The first signed release is
# the first real test of that path — docs/release.md says so.
#
# macOS only: codesign and spctl do not exist elsewhere.
set -eu

if [ "$(uname -s)" != "Darwin" ]; then
	echo "test-signature-check: only macOS has codesign and spctl; skipping"
	exit 0
fi

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/owlsort-sigtest.XXXXXX")
trap 'rm -rf "$work"' EXIT INT TERM

fail() {
	echo "FAIL: $*" >&2
	exit 1
}
pass() { echo "  ok    $*"; }

echo "Building an unsigned darwin binary, the way Go does by default"
(
	cd "$root"
	CGO_ENABLED=0 go build -o "$work/owlsort" ./cmd/owlsort
)
tar czf "$work/unsigned.tar.gz" -C "$work" owlsort

echo "Checking an unsigned archive"
if sh "$root/scripts/verify-macos-signature.sh" "$work/unsigned.tar.gz" >"$work/out.log" 2>&1; then
	cat "$work/out.log" >&2
	fail "the check passed an unsigned binary: it cannot be trusted to guard a release"
fi
grep -q "Developer ID" "$work/out.log" || fail "the output does not say what is missing"
grep -q "ad-hoc" "$work/out.log" || fail "the output does not mention the ad-hoc signature"
grep -q "Gatekeeper" "$work/out.log" || fail "the output does not mention Gatekeeper"
pass "an unsigned binary is refused, with the reason"

# The specific trap: `codesign --verify` passes on an unsigned Go binary, so a
# check built on it alone would have accepted this. Prove that the script does not
# rely on it.
codesign --verify --strict "$work/owlsort" >/dev/null 2>&1 ||
	fail "expected codesign --verify to pass on a Go binary (the trap this script exists for)"
pass "codesign --verify alone would have passed — the script catches what it misses"

echo "Checking the ways the script can be given nothing to check"
if sh "$root/scripts/verify-macos-signature.sh" "$work/missing.tar.gz" >/dev/null 2>&1; then
	fail "a missing archive was accepted"
fi
pass "a missing archive is an error"

mkdir -p "$work/empty"
printf 'not the binary' >"$work/empty/README.md"
tar czf "$work/nobinary.tar.gz" -C "$work/empty" README.md
if sh "$root/scripts/verify-macos-signature.sh" "$work/nobinary.tar.gz" >/dev/null 2>&1; then
	fail "an archive without the binary was accepted"
fi
pass "an archive without the binary is an error"

if sh "$root/scripts/verify-macos-signature.sh" >/dev/null 2>&1; then
	fail "no argument was accepted"
fi
pass "no argument is an error"

echo
echo "verify-macos-signature.sh: all checks passed"
