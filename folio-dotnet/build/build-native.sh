#!/usr/bin/env bash
#
# Builds the folio8 engine as a native library from folio-go/cshared/cmd/folio8.
#
#   ./build-native.sh                  # the host library only (the default)
#   ./build-native.sh host win-x64     # a named subset
#   ./build-native.sh all              # host + win-x64 + win-x86
#
# Output layout, which Folio8.Tests.csproj and (in story 7) the NuGet RID
# layout both read:
#
#   build/native/host/libfolio8_native.dylib   (macOS)   — a DEVELOPMENT AID
#   build/native/host/libfolio8_native.so      (Linux)   — a DEVELOPMENT AID
#   build/native/win-x64/folio8_native.dll
#   build/native/win-x86/folio8_native.dll
#
# THE NAME IS `folio8_native`, NOT `folio8`, AND THAT IS LOAD-BEARING. The
# managed assembly is Folio8.dll, Windows and macOS file systems are
# case-insensitive, and `folio8.dll` IS `Folio8.dll` to both of them. Staged
# into one directory — which is what a NuGet RID asset does on modern .NET,
# and what the test project does — one silently overwrites the other, and
# DllImport then loads a file with no folio8_ exports in it. Measured: every
# managed test failed with EntryPointNotFoundException on Windows while the
# DLL itself exported all eight symbols undecorated.
#
# The host library is never packaged and never shipped. It exists so the ABI
# and the managed binding can be developed and tested off Windows, instead of
# every ABI change costing a CI round trip.
#
# Windows targets need a mingw-w64 toolchain PER ARCHITECTURE. On Debian or
# Ubuntu: apt-get install gcc-mingw-w64. On macOS: brew install mingw-w64
# (x86_64 only; the 32-bit leg is a CI concern).
#
# CGO_ENABLED and GOTOOLCHAIN are pinned here rather than inherited: cgo is
# non-negotiable for -buildmode=c-shared, and an unpinned toolchain is AD-22's
# drift class arriving through the back door.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
module="$repo/folio-go"
out="$here/native"
pkg="./cshared/cmd/folio8"

export CGO_ENABLED=1
export GOTOOLCHAIN=go1.26.0

# Expand `all` wherever it appears, not only first, and VALIDATE THE WHOLE
# LIST BEFORE BUILDING ANYTHING: `./build-native.sh host bogus` used to build
# the host library and then fail, leaving a half-done run that looks like a
# whole one.
requested=("${@:-host}")
targets=()
for target in "${requested[@]}"; do
  case "$target" in
    all) targets+=(host win-x64 win-x86) ;;
    host|win-x64|win-x86) targets+=("$target") ;;
    *)
      echo "build-native.sh: unknown target '$target' (host, win-x64, win-x86, all)" >&2
      exit 1
      ;;
  esac
done

# find_cc prints the first existing compiler from its arguments, or nothing.
find_cc() {
  local candidate
  for candidate in "$@"; do
    if [ -x "$candidate" ]; then
      echo "$candidate"
      return 0
    fi
    if command -v "$candidate" >/dev/null 2>&1; then
      command -v "$candidate"
      return 0
    fi
  done
  return 1
}

build() {
  local target="$1" dir="$out/$1" name="$2"
  mkdir -p "$dir"
  # A FAILED BUILD MUST NOT LEAVE THE PREVIOUS ARTIFACT IN PLACE. `go build`
  # writes nothing when it fails, so without this the next `dotnet test` would
  # exercise a stale library as if it were fresh — the most expensive kind of
  # green. The generated header goes too; it is documentation of the same
  # build.
  rm -f "$dir/$name" "$dir/${name%.*}.h"
  echo "==> $target: $dir/$name"
  # -trimpath keeps the build reproducible across checkout locations; the
  # generated header sits beside the library and is documentation only, since
  # the managed binding declares the ABI itself.
  (cd "$module" && go build -trimpath -buildmode=c-shared -o "$dir/$name" "$pkg")
}

for target in "${targets[@]}"; do
  case "$target" in
    host)
      case "$(uname -s)" in
        Darwin) unset GOOS GOARCH CC 2>/dev/null || true; build host libfolio8_native.dylib ;;
        Linux)  unset GOOS GOARCH CC 2>/dev/null || true; build host libfolio8_native.so ;;
        *) echo "build-native.sh: unsupported host $(uname -s); use build-native.ps1 on Windows" >&2; exit 1 ;;
      esac
      ;;
    win-x64)
      cc="$(find_cc x86_64-w64-mingw32-gcc /usr/bin/x86_64-w64-mingw32-gcc)" || {
        echo "build-native.sh: win-x64 needs x86_64-w64-mingw32-gcc on PATH (apt-get install gcc-mingw-w64)" >&2
        exit 1
      }
      GOOS=windows GOARCH=amd64 CC="$cc" build win-x64 folio8_native.dll
      ;;
    win-x86)
      cc="$(find_cc i686-w64-mingw32-gcc /usr/bin/i686-w64-mingw32-gcc)" || {
        echo "build-native.sh: win-x86 needs i686-w64-mingw32-gcc on PATH (apt-get install gcc-mingw-w64)" >&2
        exit 1
      }
      GOOS=windows GOARCH=386 CC="$cc" build win-x86 folio8_native.dll
      ;;
  esac
done
