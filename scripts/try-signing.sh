#!/bin/sh
# Sign and notarize a build locally, before any tag exists.
#
# This is the one way to find out whether the signing path works without pushing a
# release. It runs the same goreleaser configuration the release workflow uses, on
# this machine, against a snapshot version — so a wrong certificate, a wrong key or
# a rejected notarization shows up here instead of halfway through a release.
#
#   export MACOS_SIGN_P12="$(base64 -i DeveloperID.p12 | tr -d '\n')"
#   export MACOS_SIGN_PASSWORD='the password you set on the .p12'
#   export MACOS_NOTARY_KEY_ID=XXXXXXXXXX
#   export MACOS_NOTARY_ISSUER_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
#   export MACOS_NOTARY_KEY="$(base64 -i AuthKey_XXXXXXXXXX.p8 | tr -d '\n')"
#   make try-signing
#
# Nothing is uploaded. The archives land in dist/ and are deleted by `make clean`.
#
# The `tr -d '\n'` matters: base64 wraps its output, and goreleaser wants one line.
# The same applies when pasting these into the repository's secrets.
set -eu

if [ "$(uname -s)" != "Darwin" ]; then
	echo "try-signing: signing needs macOS (codesign, notarytool)" >&2
	exit 1
fi

missing=""
for name in MACOS_SIGN_P12 MACOS_SIGN_PASSWORD MACOS_NOTARY_KEY_ID \
	MACOS_NOTARY_ISSUER_ID MACOS_NOTARY_KEY; do
	eval "value=\${$name:-}"
	[ -n "$value" ] || missing="$missing $name"
done
if [ -n "$missing" ]; then
	echo "try-signing: these are not set:$missing" >&2
	echo "  See the header of scripts/try-signing.sh, or docs/release.md." >&2
	exit 1
fi

echo "Signing with:"
security find-identity -v -p codesigning | grep -i "Developer ID Application" ||
	echo "  (no Developer ID Application certificate in the keychain — the build will fail here)"

echo
echo "Building, signing and notarizing a snapshot (this waits on Apple)"
goreleaser release --snapshot --clean

echo
echo "Checking what came out"
failed=0
for archive in dist/owlsort_*_darwin_*.tar.gz; do
	sh scripts/verify-macos-signature.sh "$archive" || failed=1
done
if [ "$failed" != 0 ]; then
	echo
	echo "The signing path does not work yet; do not tag a release." >&2
	exit 1
fi
echo
echo "Signed, notarized, and accepted by Gatekeeper. The release workflow should work."
