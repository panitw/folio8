#!/usr/bin/env bash
#
# DW-396: IS THE ALTERNATE SIGNAL STACK ACTUALLY THE CAUSE?
#
#   ./run.sh [-n N]       # three arms, N iterations each
#
# WHAT IS KNOWN GOING IN. Loading the engine kills the .NET test host on about
# one Linux amd64 run in four; not loading it never does (load-exposure.sh,
# run 35933716928: 134/500 against 0/500). The death is an ABORT, not a
# segfault -- glibc's futex_fatal_error reached from the CLR's own handler
# under `<signal handler called>`, on a `.NET TP Worker` (crash-trace.sh, run
# 35945107391). That is what a corrupted futex looks like, and a handler
# running out of room on a 16 KiB alternate stack is a way to corrupt one.
#
# WHAT IS NOT KNOWN: whether the alternate stack is the cause or a bystander.
# The mechanism is measured -- Go installs SA_ONSTACK handlers process-wide at
# dlopen, every CLR thread carries 16 KiB, Go sizes 32 KiB for itself -- but a
# mechanism that could explain a crash is not the same as the one that did.
#
# THREE ARMS, ONE VARIABLE EACH, ON A HOST WHOSE SOUNDNESS IS ESTABLISHED:
#
#   none       the shipped binding, untouched                 -- the baseline
#   noonstack  SA_ONSTACK cleared from Go's handlers after
#              load, so they run on the thread's ORDINARY
#              stack, which is megabytes                      -- removes the
#                                                                small stack
#   restore    the CLR's original handlers put back after
#              load, so Go's never run on a .NET thread       -- removes Go
#                                                                from the path
#
# HOW TO READ IT. If `none` crashes and `noonstack` does not, the alternate
# stack's SIZE is the cause and any fix must cover threads the binding does
# not own. If `restore` also fixes it but `noonstack` does not, Go's handler
# is implicated for some reason other than stack size. If all three crash
# alike, the signal path is not the cause at all and this whole line of
# investigation is finished -- which is worth knowing and is why `none` is run
# here rather than quoted from an earlier run on a different day.
#
# NEITHER WORKAROUND IS A SHIPPABLE FIX. Both reach into another runtime's
# signal handlers from a library. They are here to identify a cause.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../../.." && pwd)"
tests="$root/folio-dotnet/test/Folio8.Tests/Folio8.Tests.csproj"
iterations=200

while [ "$#" -gt 0 ]; do
  case "$1" in
    -n|--iterations) iterations="$2"; shift 2 ;;
    *) echo "unknown argument '$1'" >&2; exit 1 ;;
  esac
done

[ "$(uname -s)" = Linux ] || { echo "Linux only." >&2; exit 1; }

case "$(uname -m)" in x86_64) rid=linux-x64 ;; aarch64) rid=linux-arm64 ;; *) echo "unknown arch" >&2; exit 1 ;; esac
native="$root/folio-dotnet/build/native/$rid/libfolio8_native.so"
[ -f "$native" ] || { echo "no native at $native" >&2; exit 1; }

work="$(mktemp -d)"; stage="$work/native"; mkdir -p "$stage/host"
cp "$native" "$stage/host/libfolio8_native.so"

echo "building the hook"
dotnet build "$here" -c Release -v quiet --nologo -o "$work/hook" >"$work/hook.log" 2>&1 || {
  sed 's/^/  /' "$work/hook.log"; exit 1; }
echo "building the suite"
dotnet build "$tests" -c Release -v quiet --nologo -p:FolioNativeDir="$stage" >"$work/build.log" 2>&1 || {
  sed 's/^/  /' "$work/build.log"; exit 1; }

# The workload is load-exposure.sh's arm B: loads the engine once, renders
# nothing. Named identically so the tools cannot drift apart.
filter="FullyQualifiedName~PackagingTests|FullyQualifiedName~DocsTests|FullyQualifiedName~SurfaceTests"

export DOTNET_STARTUP_HOOKS="$work/hook/StartupHook.dll"
export FOLIO_SO="$stage/host/libfolio8_native.so"

cat <<EOF2

=== altstack experiment ===
  binding   : $(cd "$root" && git log --oneline -1 2>/dev/null || echo unknown)
  native    : $(sha256sum "$native" | awk '{print $1}')
  host      : $(uname -n), $(uname -s) $(uname -r), $(uname -m)
  iterations: $iterations per arm

EOF2

declare -a names=() results=()
for mode in none noonstack restore; do
  export FOLIO_EXP="$mode"
  deaths=0
  printf '  %-10s ' "$mode"
  for i in $(seq 1 "$iterations"); do
    set +e
    out="$(dotnet test "$tests" -c Release --no-build --nologo -v quiet \
        -p:FolioNativeDir="$stage" --filter "$filter" 2>&1)"
    set -e
    case "$out" in
      *"Test host process crashed"*|*"active test run was aborted"*)
        deaths=$((deaths + 1)); printf 'X' ;;
      *) printf '.' ;;
    esac
  done
  echo "  -> $deaths deaths / $iterations"
  names+=("$mode"); results+=("$deaths")
done

echo
echo "=== RESULT ==="
for i in 0 1 2; do printf '  %-10s %s deaths / %s\n' "${names[$i]}" "${results[$i]}" "$iterations"; done
echo
base="${results[0]}"; noon="${results[1]}"; rest="${results[2]}"
if [ "$base" -eq 0 ]; then
  echo "  INCONCLUSIVE — the baseline did not crash either, so there was nothing to fix."
  echo "  Raise -n; the rate has been measured between 1-in-60 and 1-in-4."
elif [ "$noon" -eq 0 ] && [ "$rest" -eq 0 ]; then
  echo "  THE SIGNAL PATH IS THE CAUSE. Both workarounds removed the crash."
  echo "  Whichever is responsible, the exposure is on threads the binding does not own."
elif [ "$noon" -eq 0 ]; then
  echo "  THE ALTERNATE STACK'S SIZE IS THE CAUSE. Clearing SA_ONSTACK — which moves the"
  echo "  handler onto the thread's ordinary megabyte-plus stack, changing nothing else —"
  echo "  removed the crash. Enlarging only binding-owned threads cannot fix this."
elif [ "$rest" -eq 0 ]; then
  echo "  GO'S HANDLER IS IMPLICATED, BUT NOT THROUGH STACK SIZE. Restoring the CLR's"
  echo "  handlers removed the crash while clearing SA_ONSTACK did not."
else
  echo "  THE SIGNAL PATH IS NOT THE CAUSE. Every arm crashed. Neither removing Go's"
  echo "  handlers nor moving them off the alternate stack helped, so the crash is"
  echo "  something else that loading the engine provokes. DW-396's mechanism is real"
  echo "  and measured, but it is not what is killing this process."
fi
echo
echo "  Quote this with the host and binding block above."
