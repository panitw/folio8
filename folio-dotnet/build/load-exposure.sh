#!/usr/bin/env bash
#
# DW-396: is LOADING the engine enough to expose a thread that never crosses?
#
#   ./load-exposure.sh                # 300 iterations per arm
#   ./load-exposure.sh -n 100
#   ./load-exposure.sh --self-check   # this tool's judgement, any OS
#   ./load-exposure.sh --help
#
# THE QUESTION, AND WHY IT IS NOT THE SOAK'S QUESTION. `soak.sh` asks whether
# the shipped binding survives rendering. This asks something narrower and
# nastier: Go's `c-shared` runtime installs its `SA_ONSTACK` handlers
# PROCESS-WIDE at `dlopen` -- measured 2026-09-23, before any call is made --
# and the CLR gives every thread it creates a fixed 16 KiB alternate signal
# stack against the 32 KiB Go sizes for itself. If that is enough to kill a
# thread that never touches this ABI, then routing every crossing through
# binding-owned threads with enlarged stacks (`EngineThreads`, 7f6a936) cannot
# be sufficient, because the exposed threads are ones the binding does not own.
#
# THE ONLY VARIABLE IS WHETHER THE NATIVE GETS LOADED. Both arms run the SAME
# binding (this tree's, so HEAD's -- the one carrying the fix), the same test
# assembly, on the same host, back to back:
#
#   arm A  a filter that never crosses the ABI          -- the library is never loaded
#   arm B  arm A plus ONE test that asks the engine
#          its version                                  -- the library is loaded, once
#
# Arm B does not render, does not loop through the engine, and makes no further
# crossing. If arm B dies where arm A does not, the cost was paid by LOADING.
#
# A IS VERIFIED, NOT ASSUMED. `soak.sh`'s control leg once claimed to be
# engine-free and was not -- `PackagingTests` contains a test that asks the
# engine its version, so every "engine absent" iteration was loading the Go
# library (DW-396, 2026-09-24). This script therefore proves arm A's claim the
# only way that means anything: it moves the native aside and requires arm A to
# pass without it.
#
# THE KERNEL LINE IS THE FINDING, NOT THE EXIT CODE. `signal: <comm>[pid]
# overflowed sigaltstack` is what distinguishes this defect from a host that
# crashes on its own, and on a runner it is unreadable until
# `kernel.dmesg_restrict` is 0. Without it this script refuses to run at all,
# because a dead test host it cannot name is exactly the reading that has twice
# sent this entry in the wrong direction.
#
# IT IS NOT A GATE. No workflow runs it on push; the soak workflow exposes it
# as a separate manual job.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
tests_csproj="$root/folio-dotnet/test/Folio8.Tests/Folio8.Tests.csproj"

# The single test that crosses. Named once, here, because both arms are defined
# by it: arm A excludes it, arm B includes it.
crossing_test="TheRecordedEngineVersionMatchesTheEngine"
base_filter="FullyQualifiedName~PackagingTests|FullyQualifiedName~DocsTests|FullyQualifiedName~SurfaceTests"
arm_a="(${base_filter})&FullyQualifiedName!~${crossing_test}"
arm_b="${base_filter}"

iterations=300
want_self_check=0
native=""
log_dir=""

usage() {
  cat <<'USAGE'
load-exposure.sh — does loading the engine expose threads that never cross?

  ./load-exposure.sh [-n N] [--native PATH] [--log-dir DIR]
  ./load-exposure.sh --self-check
  ./load-exposure.sh --help

Two arms of N iterations on the same host, same binding, back to back. Arm A
never loads the native (verified by running it with the native moved aside).
Arm B is arm A plus one test that asks the engine its version. Neither renders.

Exit status: 0 if neither arm died, 1 for a refusal or an arm that died.
A death in arm B with the kernel's 'overflowed sigaltstack' line is the
finding this exists to produce, and it exits 1.
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --self-check) want_self_check=1; shift ;;
    -n|--iterations) [ "$#" -ge 2 ] || { echo "load-exposure: --iterations needs a number" >&2; exit 1; }; iterations="$2"; shift 2 ;;
    --native) [ "$#" -ge 2 ] || { echo "load-exposure: --native needs a path" >&2; exit 1; }; native="$2"; shift 2 ;;
    --log-dir) [ "$#" -ge 2 ] || { echo "load-exposure: --log-dir needs a directory" >&2; exit 1; }; log_dir="$2"; shift 2 ;;
    *) echo "load-exposure: unknown argument '$1' (see --help)" >&2; exit 1 ;;
  esac
done

case "$iterations" in ''|*[!0-9]*) echo "load-exposure: --iterations must be a whole number" >&2; exit 1 ;; esac
[ "$iterations" -ge 1 ] || { echo "load-exposure: --iterations must be one or more" >&2; exit 1; }

# --- the judgement, as a function of what was seen ------------------------
# Kept pure so --self-check can drive it without a machine, the same split
# soak.sh and the signal-stack probe use, and for the same reason: the
# derivation is where the mistakes have been.
#
# ARM A DECIDES ADMISSIBILITY FIRST. A host that kills a test host with the
# library absent cannot answer this question at all, and its arm-B deaths mean
# nothing -- that is the confound that cost this entry a retracted conclusion.
verdict() {
  local a_died="$1" b_died="$2" b_named="$3"
  if [ "$a_died" -gt 0 ]; then
    TAG="UNSOUND"
    TEXT="arm A died $a_died time(s) with the native never loaded. This host crashes test hosts on its own, so nothing about arm B is admissible and no conclusion is drawn."
  elif [ "$b_died" -eq 0 ]; then
    TAG="NO-EXPOSURE"
    TEXT="neither arm died. At this iteration count, loading the engine was not shown to expose a thread that never crosses. That is not proof it cannot -- the defect is probabilistic and its rate is host-dependent -- so quote the count with the result."
  elif [ "$b_named" -gt 0 ]; then
    TAG="EXPOSED"
    TEXT="arm B died with the kernel's 'overflowed sigaltstack' line while arm A, which differs only in that it never loads the native, did not die at all. LOADING the engine is sufficient to kill a thread that never crosses the ABI. Enlarging the alternate signal stack only on binding-owned threads cannot address this."
  else
    TAG="UNEXPLAINED"
    TEXT="arm B died $b_died time(s) and arm A did not, but no death carried the kernel's 'overflowed sigaltstack' line. The difference is real and the cause is not named; it is NOT attributed to DW-396."
  fi
}

if [ "$want_self_check" = 1 ]; then
  checks=0; bad=0
  expect() {
    checks=$((checks + 1))
    if [ "$2" = "$3" ]; then printf '  ok   %-52s %s\n' "$1" "$3"
    else bad=$((bad + 1)); printf '  FAIL %-52s expected %s, got %s\n' "$1" "$2" "$3"; fi
  }
  echo "=== load-exposure.sh --self-check ==="
  echo
  verdict 1 0 0; expect "arm A died: nothing else is admissible"      UNSOUND     "$TAG"
  verdict 2 5 5; expect "arm A died, even with a named arm-B death"   UNSOUND     "$TAG"
  verdict 0 0 0; expect "both arms clean"                             NO-EXPOSURE "$TAG"
  verdict 0 1 1; expect "arm B died, named"                           EXPOSED     "$TAG"
  verdict 0 3 0; expect "arm B died, unnamed"                         UNEXPLAINED "$TAG"
  verdict 0 3 1; expect "arm B died 3x, one named"                    EXPOSED     "$TAG"
  echo
  if [ "$bad" != 0 ]; then echo "$bad of $checks cases WRONG"; exit 1; fi
  echo "all $checks cases rendered as expected. It measures nothing."
  exit 0
fi

[ "$(uname -s)" = "Linux" ] || {
  echo "load-exposure: this is a POSIX signal-stack question and this host is $(uname -s). Run it on the Linux host whose evidence you want; --self-check works anywhere." >&2
  exit 1
}

# THE KERNEL LINE IS THE WHOLE DISCRIMINATOR, SO ITS ABSENCE IS A REFUSAL.
# Ubuntu ships kernel.dmesg_restrict=1. A run that cannot read dmesg can only
# report "the host died", which is the reading that has already been wrong
# twice in this entry.
if ! dmesg >/dev/null 2>&1; then
  echo "load-exposure: dmesg is not readable, so a death could not be named." >&2
  echo "  The kernel's 'overflowed sigaltstack' line is the only thing separating this defect" >&2
  echo "  from a host that crashes on its own. Run: sudo sysctl -w kernel.dmesg_restrict=0" >&2
  exit 1
fi

if [ -z "$native" ]; then
  case "$(uname -m)" in
    x86_64) rid=linux-x64 ;; aarch64|arm64) rid=linux-arm64 ;; *) rid="" ;;
  esac
  native="$root/folio-dotnet/build/native/host/libfolio8_native.so"
  [ -n "$rid" ] && [ -f "$root/folio-dotnet/build/native/$rid/libfolio8_native.so" ] &&
    native="$root/folio-dotnet/build/native/$rid/libfolio8_native.so"
fi
[ -f "$native" ] || { echo "load-exposure: no native library at $native. Run build-native.sh first, or pass --native." >&2; exit 1; }

work="$(mktemp -d)"
[ -n "$log_dir" ] || log_dir="$work/logs"
mkdir -p "$log_dir"
stage="$work/native"; mkdir -p "$stage/host"
cp "$native" "$stage/host/libfolio8_native.so"

cat <<EOF
=== load-exposure.sh — does LOADING the engine expose a non-crossing thread? ===
  binding      : $(cd "$root" && git log --oneline -1 2>/dev/null || echo 'not a git tree')
  native       : $native
  native sha256: $(sha256sum "$native" | awk '{print $1}')
  host         : $(uname -n), $(uname -s) $(uname -r), $(uname -m)
  iterations   : $iterations per arm
  arm A        : $arm_a
  arm B        : $arm_b
  dmesg        : readable
  logs         : $log_dir

EOF

echo "==> building the suite"
dotnet build "$tests_csproj" -c Release -v quiet --nologo -p:FolioNativeDir="$stage" >"$log_dir/build.log" 2>&1 || {
  sed 's/^/  /' "$log_dir/build.log"; echo "load-exposure: the suite did not build; nothing was run."; exit 1; }

# ARM A'S ONE CLAIM, PROVEN BEFORE IT IS RELIED ON.
echo "==> proving arm A never loads the native"
mv "$stage/host/libfolio8_native.so" "$stage/host/.hidden.so"
set +e
dotnet test "$tests_csproj" -c Release --no-build --nologo -v quiet \
  -p:FolioNativeDir="$stage" --filter "$arm_a" >"$log_dir/arm-a-engine-free.log" 2>&1
free_status=$?
set -e
mv "$stage/host/.hidden.so" "$stage/host/libfolio8_native.so"
if [ "$free_status" != 0 ]; then
  echo
  echo "=== REFUSED — ARM A IS NOT ENGINE-FREE ==="
  echo "  With the native moved aside arm A did not pass, so it crosses the ABI and the two arms"
  echo "  differ by more than the load. Narrow the filter. Log: $log_dir/arm-a-engine-free.log"
  exit 1
fi
echo "    proven: arm A passes with no native present"
echo

run_arm() {
  local name="$1" filter="$2" n="$3"
  local died=0 named=0 i before after log
  before="$work/dmesg.before"; after="$work/dmesg.after"
  for i in $(seq 1 "$n"); do
    dmesg >"$before" 2>/dev/null || true
    log="$log_dir/$name-$i.log"
    set +e
    dotnet test "$tests_csproj" -c Release --no-build --nologo -v quiet \
      -p:FolioNativeDir="$stage" --filter "$filter" >"$log" 2>&1
    set -e
    if grep -q -e 'Test host process crashed' -e 'The active test run was aborted' "$log" 2>/dev/null; then
      died=$((died + 1))
      dmesg >"$after" 2>/dev/null || true
      if diff "$before" "$after" 2>/dev/null | sed -n 's/^> //p' | grep -q 'overflowed sigaltstack'; then
        named=$((named + 1))
        printf '\n    %s iteration %d: HOST DIED — overflowed sigaltstack\n' "$name" "$i"
        diff "$before" "$after" 2>/dev/null | sed -n 's/^> //p' | grep 'overflowed sigaltstack' | sed 's/^/      /'
      else
        printf '\n    %s iteration %d: HOST DIED — no named signature\n' "$name" "$i"
      fi
    else
      printf '.'
    fi
  done
  echo
  ARM_DIED="$died"; ARM_NAMED="$named"
}

echo "==> arm A: $iterations iterations, the native is NEVER loaded"
run_arm arm-a "$arm_a" "$iterations"; a_died="$ARM_DIED"
echo "    arm A: $a_died death(s) / $iterations"
echo
echo "==> arm B: $iterations iterations, same tests plus ONE crossing (the engine's version)"
run_arm arm-b "$arm_b" "$iterations"; b_died="$ARM_DIED"; b_named="$ARM_NAMED"
echo "    arm B: $b_died death(s) / $iterations, $b_named carrying the kernel signature"
echo

verdict "$a_died" "$b_died" "$b_named"
cat <<EOF
=== VERDICT: $TAG ===

  $TEXT

  arm A (never loads) : $a_died death(s) / $iterations
  arm B (loads once)  : $b_died death(s) / $iterations, $b_named named
  binding             : $(cd "$root" && git log --oneline -1 2>/dev/null || echo unknown)
  native              : $(sha256sum "$native" | awk '{print $1}'), $(uname -m)
  host                : $(uname -n), $(uname -s) $(uname -r)
  logs                : $log_dir

  Quote this verdict with the block above attached.
EOF

case "$TAG" in
  NO-EXPOSURE) exit 0 ;;
  *) exit 1 ;;
esac
