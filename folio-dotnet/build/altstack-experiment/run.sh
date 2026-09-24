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

# WHAT dlopen ACTUALLY REWRITES, printed once before any arm runs. This is the
# diagnostic the first experiment lacked: it assumed Go touches three signals,
# and Go also ADDS SA_ONSTACK to handlers it leaves in place. Whatever appears
# here is the real list.
echo "=== what loading the engine changes about this process's signal handlers ==="
FOLIO_EXP=report dotnet test "$tests" -c Release --no-build --nologo -v quiet \
    -p:FolioNativeDir="$stage" --filter "FullyQualifiedName~DocsTests" 2>&1 |
  grep '^\[hook\]' || echo "  (no hook output — check DOTNET_STARTUP_HOOKS)"
echo

declare -a names=() results=()
# Two arms. `none` is the baseline; `noonstack-all` clears SA_ONSTACK from
# EVERY signal that carries a handler. The earlier three-signal arms
# (noonstack, restore) left SIGRTMIN -- CoreCLR's thread-suspension signal --
# exactly as Go rewrote it, which is very likely why all three crashed alike.
for mode in none noonstack-all; do
  export FOLIO_EXP="$mode"
  deaths=0
  printf '  %-14s ' "$mode"
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
for i in 0 1; do printf '  %-14s %s deaths / %s\n' "${names[$i]}" "${results[$i]}" "$iterations"; done
echo
base="${results[0]}"; all="${results[1]}"
if [ "$base" -eq 0 ]; then
  echo "  INCONCLUSIVE — the baseline did not crash, so there was nothing to fix."
  echo "  Raise -n; the rate has been measured between 1-in-60 and 3-in-4."
elif [ "$all" -eq 0 ]; then
  echo "  THE ALTERNATE STACK IS THE CAUSE AFTER ALL, AND THE EARLIER ARMS MISSED IT."
  echo "  Clearing SA_ONSTACK from every signal removed the crash where clearing it from"
  echo "  SIGSEGV/SIGBUS/SIGURG did not. The signal that matters is one Go did not replace"
  echo "  but DID move onto the alternate stack -- read the report block above for which."
  echo "  A fix must stop the host runtime's OWN handlers being relocated, and cannot be"
  echo "  confined to threads the binding owns."
elif [ "$all" -lt "$base" ]; then
  echo "  PARTIAL. Clearing SA_ONSTACK everywhere reduced the rate from $base to $all but did"
  echo "  not remove it. The alternate stack is implicated and is not the whole story."
else
  echo "  NOT THE ALTERNATE STACK. Clearing SA_ONSTACK from every signal that has a handler"
  echo "  changed nothing, so no handler is dying for want of room. The crash is something"
  echo "  else that loading the engine provokes, and the signal-stack line of investigation"
  echo "  is finished."
fi
echo
echo "  Quote this with the host and binding block above."
