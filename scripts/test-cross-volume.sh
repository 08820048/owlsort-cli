#!/bin/sh
# Test moving files between two volumes, which is the one code path in the program
# that can lose a file if it is wrong (docs/safety-model.md, rule 11).
#
#   make cross-volume-test
#
# On macOS this creates a small disk image, mounts it as a second volume, runs the
# tests and takes it down again. On Linux you have to provide one:
#
#   mkdir -p /mnt/scratch && sudo mount -t tmpfs -o size=64m tmpfs /mnt/scratch
#   OWLSORT_SECOND_VOLUME=/mnt/scratch make cross-volume-test
#
# Without a second volume the Go tests skip, and this script says so and exits 0:
# a missing volume is not a failure, but a silent pass would be a lie, so it says
# out loud that nothing was tested.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if [ -n "${OWLSORT_SECOND_VOLUME:-}" ]; then
	echo "Using $OWLSORT_SECOND_VOLUME"
	cd "$root"
	OWLSORT_SECOND_VOLUME="$OWLSORT_SECOND_VOLUME" go test ./internal/apply -run Volume -v -count=1
	exit $?
fi

if [ "$(uname -s)" != "Darwin" ]; then
	echo "No OWLSORT_SECOND_VOLUME set, and this is not macOS, so no second volume"
	echo "can be made here. Mount one and set OWLSORT_SECOND_VOLUME (see above)."
	echo "SKIPPED: the cross-volume path was not tested."
	exit 0
fi

image=$(mktemp -d "${TMPDIR:-/tmp}/owlsort-vol.XXXXXX")/second.dmg
mountpoint=/Volumes/OWLSORT-TEST-$$
mounted=0
cleanup() {
	if [ "$mounted" = 1 ]; then
		hdiutil detach "$mountpoint" -quiet -force 2>/dev/null || true
	fi
	rm -rf "$(dirname "$image")"
}
trap cleanup EXIT INT TERM

echo "Creating a 32 MB disk image and mounting it at $mountpoint"
hdiutil create -size 32m -fs APFS -volname "OWLSORT-TEST-$$" -quiet "$image"
hdiutil attach "$image" -nobrowse -quiet -mountpoint "$mountpoint"
mounted=1

cd "$root"
OWLSORT_SECOND_VOLUME="$mountpoint" go test ./internal/apply -run Volume -v -count=1
