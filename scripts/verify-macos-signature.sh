#!/bin/sh
# Check that a release archive holds a macOS binary that is really signed and
# notarized.
#
#   sh scripts/verify-macos-signature.sh dist/owlsort_0.1.0_darwin_arm64.tar.gz
#
# Why this exists as a script instead of a few lines in the release workflow: the
# obvious check is wrong. `codesign --verify` passes on an *unsigned* Go binary,
# because Go ad-hoc signs darwin binaries so they can run at all. A release guard
# that passes for the wrong reason is worse than none, so the checks here are the
# ones that actually distinguish a Developer ID signature from an ad-hoc one, and
# they are tested against a real ad-hoc build (scripts/test-signature-check.sh).
#
# Three things must all hold:
#   1. The signature is by a Developer ID Application certificate, not ad-hoc.
#   2. It carries a Team Identifier — ad-hoc signatures have none.
#   3. Gatekeeper accepts it (`spctl`), which is only true when Apple has
#      notarized it and the ticket can be found.
#
# Exit code: 0 signed and notarized, 1 not, 2 the archive or the tools are wrong.
set -eu

if [ $# -ne 1 ]; then
	echo "usage: $0 <archive.tar.gz>" >&2
	exit 2
fi
archive=$1
[ -f "$archive" ] || {
	echo "verify-macos-signature: no such archive: $archive" >&2
	exit 2
}

for tool in codesign spctl tar; do
	command -v "$tool" >/dev/null 2>&1 || {
		echo "verify-macos-signature: $tool is needed and not here (macOS only)" >&2
		exit 2
	}
done

work=$(mktemp -d "${TMPDIR:-/tmp}/owlsort-verify.XXXXXX")
trap 'rm -rf "$work"' EXIT INT TERM

tar xzf "$archive" -C "$work"
binary="$work/owlsort"
[ -f "$binary" ] || {
	echo "verify-macos-signature: $archive does not contain owlsort" >&2
	exit 2
}

problems=0
report() {
	echo "  FAIL  $*"
	problems=$((problems + 1))
}

echo "$(basename "$archive")"

# 1 and 2: what kind of signature is on it.
details=$(codesign -dv --verbose=4 "$binary" 2>&1 || true)
echo "$details" | sed 's/^/  | /'

if ! echo "$details" | grep -q "^Authority=Developer ID Application"; then
	report "not signed with a Developer ID Application certificate"
fi
# Go ad-hoc signs darwin binaries so they can run at all; both of these lines are
# what that looks like, so one check covers them.
if echo "$details" | grep -qE "flags=.*adhoc|^Signature=adhoc"; then
	report "the signature is ad-hoc (Go's default), not a real one"
fi
team=$(echo "$details" | sed -n 's/^TeamIdentifier=\(.*\)$/\1/p')
if [ -z "$team" ] || [ "$team" = "not set" ]; then
	report "no Team Identifier: only an ad-hoc or self-signed build looks like this"
fi

# A strict verify is still worth running, but on its own it proves nothing: an
# unsigned Go binary passes it too.
if ! codesign --verify --strict "$binary" 2>&1 | sed 's/^/  | /'; then
	report "codesign --verify --strict failed"
fi

# 3: Gatekeeper's own opinion, which is the one the user's Mac will ask for.
if ! spctl --assess --type execute --verbose=4 "$binary" 2>"$work/spctl.log" >"$work/spctl.out"; then
	cat "$work/spctl.out" "$work/spctl.log" 2>/dev/null | sed 's/^/  | /' || true
	report "Gatekeeper rejects it: signed but not notarized, or notarized but not stapled"
else
	cat "$work/spctl.out" 2>/dev/null | sed 's/^/  | /' || true
fi

if [ "$problems" -gt 0 ]; then
	echo "verify-macos-signature: $problems problem(s) with $archive" >&2
	exit 1
fi
echo "  ok    Developer ID signature, Team Identifier $team, accepted by Gatekeeper"
