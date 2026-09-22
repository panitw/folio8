#!/usr/bin/env bash
#
# Runs the DW-396 signal-stack probe: what size alternate signal stack does
# the CLR install on each kind of .NET thread, and what does glibc itself
# think a signal stack should be?
#
#   ./probe-signal-stack.sh                # this host (Linux only)
#   ./probe-signal-stack.sh amd64          # linux/amd64 in a container
#   ./probe-signal-stack.sh arm64          # linux/arm64 in a container
#   ./probe-signal-stack.sh self-check     # row formatting only; any OS
#
# IT IS A DIAGNOSTIC, NOT A TEST, AND IS WIRED INTO NO CI JOB. It prints
# measurements and exits; the judgement stays with the reader. Story 2 of
# SPEC-dotnet-linux owns the assertion that the binding's threads are sized
# correctly -- this only says what an unmodified runtime does.
#
# WHY IT EXISTS AS A COMMITTED SCRIPT. The reading that underpins the whole
# SPEC-dotnet-linux design -- a FIXED 16 KiB (amd64) / 24 KiB (arm64) altstack
# against Go's 32 KiB gsignal stack -- was taken by a throwaway probe in a
# scratch directory. A measurement nobody can re-run is a claim, and DW-396's
# record is already two wrong readings long.
#
# THE ARCHITECTURE ARGUMENT EXISTS FOR THE amd64 LEG. The amd64 row of the
# spike was taken under emulation, where sysconf(_SC_SIGSTKSZ) answers for the
# translator rather than for the silicon. Confirming it is one command on a
# real amd64 box -- which is what this argument is for.
#
# ⚠ A CONTAINER ON THIS MACHINE IS NOT REAL SILICON. `arm64` on an Apple
# Silicon Mac executes natively and is a sound reading; `amd64` there runs
# under Rosetta and is NOT admissible as evidence about real amd64. The probe
# works that out for itself and stamps every row -- it does not take the
# operator's word for it, because that is exactly what DW-396 got wrong twice.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
proj="$here/signal-stack-probe"

# The usage block, repeated from the header so `--help` answers the question
# the header answers -- the probe binary it fronts has one, and a wrapper that
# meets `--help` with "unknown target" teaches the reader to read the source.
usage() {
  cat <<'USAGE'
probe-signal-stack.sh — the DW-396 signal-stack probe (see the header for why)

  ./probe-signal-stack.sh                # this host (Linux only)
  ./probe-signal-stack.sh amd64          # linux/amd64 in a container
  ./probe-signal-stack.sh arm64          # linux/arm64 in a container
  ./probe-signal-stack.sh self-check     # row formatting only; any OS
  ./probe-signal-stack.sh --help

PROBE_SDK_IMAGE overrides the .NET SDK image the container legs use.
USAGE
}

# The SDK image the container legs use. A TAG, NOT A DIGEST, unlike
# build-native.sh's pinned AlmaLinux images -- and the difference is
# principled. There the image's glibc becomes a permanent property of a
# SHIPPED artifact, so it is pinned by digest. Here nothing is shipped, and
# the probe prints the runtime version it actually ran on as part of its own
# report, so the provenance travels with the measurement rather than with the
# script. Override with PROBE_SDK_IMAGE to read a different .NET.
image="${PROBE_SDK_IMAGE:-mcr.microsoft.com/dotnet/sdk:8.0}"

# VALIDATE BEFORE DOING ANY WORK -- no container is started, and nothing is
# built, until the whole argument list is known good. `build-native.sh host
# bogus` used to build and then complain, leaving a half-done run that looks
# like a whole one; the same mistake here would cost an image pull.
if [ "$#" -gt 1 ]; then
  echo "probe-signal-stack: one argument at most (amd64, arm64, self-check, --help, or none for this host)" >&2
  exit 1
fi

target="${1:-host}"
case "$target" in
  host|amd64|arm64|self-check) ;;
  --help|-h)
    usage
    exit 0
    ;;
  *)
    echo "probe-signal-stack: unknown target '$target' (amd64, arm64, self-check, --help, or no argument for this host)" >&2
    exit 1
    ;;
esac

if [ "$target" = "host" ] && [ "$(uname -s)" != "Linux" ]; then
  echo "probe-signal-stack: the measurement is Linux-only and this host is $(uname -s); run './probe-signal-stack.sh arm64' or 'amd64' for a container, or 'self-check' to exercise the row formatting here" >&2
  exit 1
fi

case "$target" in
  amd64|arm64)
    if ! command -v docker >/dev/null 2>&1; then
      echo "probe-signal-stack: $target runs the probe in a container and needs docker on PATH; on a Linux box of that architecture run './probe-signal-stack.sh' with no argument instead" >&2
      exit 1
    fi
    # ON PATH IS NOT THE SAME AS RUNNING. Every other failure here names its
    # remedy; letting the daemon's own "Cannot connect to the Docker daemon"
    # through would be the one that does not.
    if ! docker info >/dev/null 2>&1; then
      echo "probe-signal-stack: docker is installed but its daemon is not reachable — start Docker (Docker Desktop, or 'systemctl start docker') and run this again" >&2
      exit 1
    fi
    ;;
  host|self-check)
    if ! command -v dotnet >/dev/null 2>&1; then
      echo "probe-signal-stack: $target needs the .NET SDK on PATH (https://dotnet.microsoft.com/download); the amd64 and arm64 targets bring their own inside the container" >&2
      exit 1
    fi
    ;;
esac

case "$target" in
  self-check)
    exec dotnet run --project "$proj" -c Release -v quiet -- --self-check
    ;;
  host)
    exec dotnet run --project "$proj" -c Release -v quiet
    ;;
esac

# THE SOURCE IS MOUNTED READ-ONLY, AND ONLY THE SOURCE IS COPIED IN. Building
# in place would write obj/ and bin/ into the repository from inside a
# container, colliding with a host build of the same project for a different
# architecture; copying the directory WHOLE would carry the host's obj/ in and
# make the container build fail about NuGet assets rather than about anything
# that happened here. Two files is the whole project.
# THE COPY BELOW IS A FLAT GLOB, SO SAY SO WHEN IT WOULD BE WRONG. A source
# file added in a subdirectory would be dropped in silence and the container
# legs would build a DIFFERENT PROGRAM than the host leg -- two measurements
# that look comparable and are not, which is this story's whole subject.
nested="$(find "$proj" -mindepth 2 -name '*.cs' -not -path '*/obj/*' -not -path '*/bin/*' 2>/dev/null || true)"
if [ -n "$nested" ]; then
  echo "probe-signal-stack: the container legs copy only the project's top-level sources, but these are in subdirectories and would be silently dropped:" >&2
  echo "$nested" | sed 's/^/  /' >&2
  echo "  Teach the 'cp' line at the end of this script about them before running a container leg." >&2
  exit 1
fi

echo "==> $target: $image (container)"
exec docker run --rm --platform "linux/$target" \
  -v "$proj":/src:ro \
  -e DOTNET_NOLOGO=1 -e DOTNET_CLI_TELEMETRY_OPTOUT=1 \
  "$image" sh -c '
    set -e
    mkdir -p /work
    cp /src/*.csproj /src/*.cs /work/
    cd /work
    dotnet run -c Release -v quiet
  '
