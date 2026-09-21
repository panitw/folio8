#!/usr/bin/env bash
#
# Verifies the two SHIPPED Linux natives: that they exist, that each is the
# architecture its RID claims, and that neither demands a newer glibc than the
# pinned build image can possibly produce.
#
#   ./verify-linux-natives.sh [<dir>]
#
# <dir> defaults to build/native beside this script.
#
# IT IS A FILE, NOT A `run:` BLOCK IN ci.yml, ON PURPOSE. The first version of
# this check lived inline in the workflow, matched `readelf`'s machine field
# case-sensitively against `x86-64`, and reddened CI on a native that was
# perfectly correct -- `readelf` prints `Advanced Micro Devices X86-64`. An
# inline check can only be tested by pushing, which is how a wrong check
# survives to fail a release. This one runs anywhere `readelf` and `objdump`
# do, including inside the same container that builds the natives.
#
# THE GLIBC CEILING IS THE POINT OF THE WHOLE EXERCISE. A cgo library records
# the glibc symbol versions of the machine that built it, so a native built
# outside the pinned AlmaLinux 8 image silently raises every consumer's
# minimum glibc. AlmaLinux 8 is glibc 2.28, so nothing built there can require
# more than that; a higher floor means the native did NOT come out of the
# pinned image, whatever the build log said.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
dir="${1:-$here/native}"

# The highest glibc an AlmaLinux 8 build can reference. Not a preference -- a
# property of the image, which is why exceeding it is a provenance failure
# rather than a portability warning.
ceiling="2.28"

# RID -> the substring readelf's `Machine:` line carries for it, matched
# case-INSENSITIVELY. readelf spells them "Advanced Micro Devices X86-64" and
# "AArch64"; neither is worth pinning exactly, and neither is worth matching
# case-sensitively.
check() {
  # SEPARATE STATEMENTS, NOT ONE `local`. A single `local rid="$1"
  # path="$dir/$rid/..."` expands $rid while the builtin is still parsing its
  # arguments, before the assignment has taken effect — which under `set -u`
  # is an unbound-variable abort rather than a quietly wrong path.
  local rid="$1"
  local want="$2"
  local path="$dir/$rid/libfolio8_native.so"

  if [ ! -f "$path" ]; then
    echo "verify-linux-natives: $path does not exist — run build-native.sh $rid" >&2
    return 1
  fi

  # READELF'S OWN FAILURE IS ITS OWN MESSAGE. Letting it fail into an empty
  # $header made every unreadable file — absent, truncated, not an ELF at all
  # — report as "not a shared object", which sends the reader looking at the
  # build's output format instead of at the file that is missing.
  local header
  if ! header="$(readelf -h "$path" 2>/dev/null)"; then
    echo "verify-linux-natives: $path could not be read as an ELF file" >&2
    return 1
  fi

  if ! grep -qi "Type:.*DYN" <<<"$header"; then
    echo "verify-linux-natives: $path is not a shared object (ET_DYN)" >&2
    echo "$header" >&2
    return 1
  fi

  if ! grep -i 'Machine:' <<<"$header" | grep -qi -- "$want"; then
    echo "verify-linux-natives: $path is not $want" >&2
    grep -i 'Machine:' <<<"$header" >&2
    return 1
  fi

  # The highest GLIBC_x.y.z the library references. `sort -V` orders version
  # strings properly, so this is the real maximum and not the lexical one --
  # GLIBC_2.9 must not outrank GLIBC_2.17.
  local floor
  floor="$(objdump -T "$path" | grep -oE 'GLIBC_[0-9.]+' | sort -Vu | tail -1 || true)"
  if [ -z "$floor" ]; then
    echo "verify-linux-natives: $path references no versioned glibc symbol at all, which no cgo build produces — is this the right file?" >&2
    return 1
  fi

  # Compare numerically: the floor is acceptable when sorting it with the
  # ceiling leaves the ceiling last.
  local bare="${floor#GLIBC_}"
  local highest
  highest="$(printf '%s\n%s\n' "$bare" "$ceiling" | sort -V | tail -1)"
  if [ "$highest" != "$ceiling" ] && [ "$bare" != "$ceiling" ]; then
    echo "verify-linux-natives: $path requires $floor, above AlmaLinux 8's own glibc $ceiling." >&2
    echo "  It cannot have come out of the pinned build image. Rebuild with build-native.sh $rid." >&2
    return 1
  fi

  printf '%-13s %s  glibc floor %s (ceiling %s)\n' "$rid" "$want" "$bare" "$ceiling"
}

failed=0
check linux-x64   "x86-64"  || failed=1
check linux-arm64 "aarch64" || failed=1
exit "$failed"
