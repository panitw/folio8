#!/usr/bin/env bash
#
# Builds the folio8 engine as a native library from folio-go/cshared/cmd/folio8.
#
#   ./build-native.sh                  # the host library only (the default)
#   ./build-native.sh host win-x64     # a named subset
#   ./build-native.sh all              # host + the four shipped natives
#
# Output layout, which Folio8.Tests.csproj and the NuGet RID layout both read:
#
#   build/native/host/libfolio8_native.dylib       (macOS)  — a DEVELOPMENT AID
#   build/native/host/libfolio8_native.so          (Linux)  — a DEVELOPMENT AID
#   build/native/win-x64/folio8_native.dll
#   build/native/win-x86/folio8_native.dll
#   build/native/linux-x64/libfolio8_native.so
#   build/native/linux-arm64/libfolio8_native.so
#
# THE `host` LIBRARY IS STILL NEVER PACKAGED, even on Linux, and `linux-x64`
# is not a synonym for it. `host` is whatever this machine happens to be,
# built against whatever glibc it happens to carry; the shipped Linux natives
# are built in a PINNED CONTAINER precisely so that they are not.
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
# LINUX TARGETS NEED DOCKER, AND THE IMAGE IS THE POINT — not a convenience.
# A cgo library records the glibc symbol versions of the machine that built
# it, so building on a modern host silently sets a FLOOR on every consumer:
# the same sources built on Debian bookworm demand GLIBC_2.34 and will not
# load on RHEL 8 or Ubuntu 20.04. Built in the pinned AlmaLinux 8 image below
# they demand GLIBC_2.17, which every distro a .NET consumer is plausibly on
# satisfies. The image is pinned BY DIGEST for the same reason GOTOOLCHAIN is
# pinned: an unpinned base is AD-22's drift class arriving through the back
# door, and here it drifts the consumer's minimum glibc, which nothing in
# this repository would notice.
#
# ⚠ ALPINE / musl IS NOT SUPPORTED AND CANNOT BE BUILT HERE. Go's
# -buildmode=c-shared emits initial-exec TLS relocations that musl's loader
# refuses under dlopen, which is exactly how P/Invoke loads this library:
#
#   Error relocating libfolio8_native.so: free: initial-exec TLS resolves
#   to dynamic definition in libfolio8_native.so
#
# Building WITH musl does not fix it — measured on both x86-64 and arm64. So
# there is no linux-musl-x64 target, deliberately: shipping that RID would
# turn an install-time absence into a first-render crash.
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

# The pinned Linux build base. AlmaLinux 8 is glibc 2.28 and is supported to
# 2029; the Debian release with a comparable floor (bullseye) is archived, so
# its repositories 404. Pinned by digest, not by tag.
linux_image="almalinux@sha256:9f355ae942d6a6c0561f0771dc053a2cfae9580fc45fa4252756db7c7e80c09f"
go_version="1.26.0"

# Expand `all` wherever it appears, not only first, and VALIDATE THE WHOLE
# LIST BEFORE BUILDING ANYTHING: `./build-native.sh host bogus` used to build
# the host library and then fail, leaving a half-done run that looks like a
# whole one.
requested=("${@:-host}")
targets=()
for target in "${requested[@]}"; do
  case "$target" in
    all) targets+=(host win-x64 win-x86 linux-x64 linux-arm64) ;;
    host|win-x64|win-x86|linux-x64|linux-arm64) targets+=("$target") ;;
    *)
      echo "build-native.sh: unknown target '$target' (host, win-x64, win-x86, linux-x64, linux-arm64, all)" >&2
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

# build_linux runs the same `go build` as build(), but inside the pinned
# image, so the glibc floor is a property of this script rather than of
# whoever ran it. GOARCH is passed explicitly and --platform matches it, so a
# cross-arch leg on an Apple Silicon machine emulates rather than silently
# producing the host architecture under a linux-x64 name.
build_linux() {
  local target="$1" arch="$2" platform="$3" dir="$out/$1"
  mkdir -p "$dir"
  rm -f "$dir/libfolio8_native.so" "$dir/libfolio8_native.h"
  if ! command -v docker >/dev/null 2>&1; then
    echo "build-native.sh: $target needs docker (the shipped Linux natives are built in a pinned image so their glibc floor is fixed; see the header)" >&2
    exit 1
  fi
  echo "==> $target: $dir/libfolio8_native.so"
  docker run --rm --platform "$platform" \
    -v "$repo":/src -v "$dir":/out -w /src/folio-go \
    -e CGO_ENABLED=1 -e GOFLAGS=-buildvcs=false \
    "$linux_image" sh -c "
      set -e
      dnf install -y -q gcc tar >/dev/null 2>&1
      curl -sSLo /tmp/go.tgz https://go.dev/dl/go${go_version}.linux-${arch}.tar.gz
      tar -C /usr/local -xzf /tmp/go.tgz
      export PATH=/usr/local/go/bin:\$PATH
      go build -trimpath -buildmode=c-shared -o /out/libfolio8_native.so ./cshared/cmd/folio8
    "
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
    linux-x64)
      build_linux linux-x64 amd64 linux/amd64
      ;;
    linux-arm64)
      build_linux linux-arm64 arm64 linux/arm64
      ;;
  esac
done
