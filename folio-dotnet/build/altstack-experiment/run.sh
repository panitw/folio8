#!/usr/bin/env bash
#
# DW-398: WHAT DOES LOADING THE ENGINE DO TO THIS PROCESS THAT KILLS IT?
#
#   ./run.sh [-n N]       # three arms, N iterations each
#
# WHAT IS KNOWN GOING IN. Loading the engine kills the .NET test host on about
# one Linux amd64 run in four; not loading it never does (load-exposure.sh,
# run 35933716928: 134/500 against 0/500). arm64 is clean at 1000. The death
# is an ABORT, not a segfault -- glibc's futex_fatal_error reached from
# pthread_cond_wait inside libcoreclr, UNDER a `<signal handler called>`
# frame, on a `.NET TP Worker` (crash-trace.sh, run 35945107391).
#
# THE ALTERNATE STACK IS RULED OUT, MEASURED, NOT ARGUED. Run 35979184685
# cleared SA_ONSTACK from every signal carrying a handler and the rate did not
# move: 65 deaths/150 against a 69/150 baseline. No handler is dying for want
# of room, and the three earlier arms that cleared it from SEGV/BUS/URG alone
# were not merely incomplete -- they were aimed at the wrong field.
#
# WHAT THAT RUN DID ESTABLISH. Its report block showed dlopen changing 13
# signals, and five of them keep their original handler ADDRESS while their
# flags change: SIG4, SIG5, SIGABRT, SIG15 and SIGRTMIN. Go did not install
# those handlers. It re-flagged the CLR's -- and it does that with sigaction(),
# which replaces sa_flags and sa_mask TOGETHER. sa_mask is the set of signals
# blocked while the handler runs; SA_ONSTACK was just the bit we printed.
#
# SIGRTMIN is CoreCLR's thread-suspension activation signal, and its handler
# does real work. A handler whose author chose its blocking set, re-installed
# by another runtime with a different one, is a re-entrancy bug waiting for
# load -- and condvar state mangled by re-entry is exactly the futex abort we
# have.
#
# THREE ARMS, ONE VARIABLE EACH:
#
#   none             the shipped binding, untouched          -- the baseline
#   restore-foreign  every RELOCATED handler (address
#                    unchanged, flags or mask changed) put
#                    back byte for byte                      -- undoes Go's
#                                                               edits to the
#                                                               CLR's handlers
#   restore-rtmin    the same, SIGRTMIN alone                -- names the one
#                                                               signal, if the
#                                                               arm above works
#
# HOW TO READ IT. If `restore-foreign` is clean, Go's re-flagging of handlers
# it does not own is the cause, and `restore-rtmin` says whether it is the
# activation signal specifically. If both still crash, the signal path is
# finished for DW-398 as well and the next lead is glibc's own stderr, kept
# below, or a minimal console reproducer without vstest.
#
# UNLIKE EVERY EARLIER ARM, THIS ONE'S SHAPE COULD SHIP: it restores handlers
# Go neither installed nor relies on. It is still not a fix -- it is a cause
# test -- but if it works the fix is not required to be a workaround.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../../.." && pwd)"
tests="$root/folio-dotnet/test/Folio8.Tests/Folio8.Tests.csproj"
iterations=150
arms=(none restore-foreign restore-rtmin)

while [ "$#" -gt 0 ]; do
  case "$1" in
    -n|--iterations) iterations="$2"; shift 2 ;;
    --arms) IFS=, read -r -a arms <<<"$2"; shift 2 ;;
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

=== signal experiment ===
  binding   : $(cd "$root" && git log --oneline -1 2>/dev/null || echo unknown)
  native    : $(sha256sum "$native" | awk '{print $1}')
  host      : $(uname -n), $(uname -s) $(uname -r), $(uname -m)
  arms      : ${arms[*]}
  iterations: $iterations per arm

EOF2

# WHAT dlopen ACTUALLY REWRITES -- now including sa_mask, which the previous
# report omitted and which is the field this run exists to examine.
echo "=== what loading the engine changes about this process's signal handlers ==="
FOLIO_EXP=report dotnet test "$tests" -c Release --no-build --nologo -v quiet \
    -p:FolioNativeDir="$stage" --filter "FullyQualifiedName~DocsTests" 2>&1 |
  grep '^\[hook\]' || echo "  (no hook output — check DOTNET_STARTUP_HOOKS)"
echo

declare -a names=() results=()
saved=0
for mode in "${arms[@]}"; do
  export FOLIO_EXP="$mode"
  deaths=0
  printf '  %-16s ' "$mode"
  for i in $(seq 1 "$iterations"); do
    set +e
    out="$(dotnet test "$tests" -c Release --no-build --nologo -v quiet \
        -p:FolioNativeDir="$stage" --filter "$filter" 2>&1)"
    set -e
    case "$out" in
      *"Test host process crashed"*|*"active test run was aborted"*)
        deaths=$((deaths + 1)); printf 'X'
        # KEEP THE FIRST FEW. DW-398's own entry lists reading the dying
        # process's stderr as untried, and every run so far has thrown this
        # away after matching one substring. glibc's __libc_fatal writes its
        # reason here and nobody has looked at it.
        if [ "$saved" -lt 3 ]; then
          saved=$((saved + 1)); printf '%s' "$out" >"$work/death-$saved.txt"
        fi ;;
      *) printf '.' ;;
    esac
  done
  echo "  -> $deaths deaths / $iterations"
  names+=("$mode"); results+=("$deaths")
done

if [ "$saved" -gt 0 ]; then
  echo
  echo "=== what the dying process actually said (first $saved death(s), verbatim) ==="
  for k in $(seq 1 "$saved"); do
    echo "--- death $k ---"
    # Everything vstest did not swallow: glibc's fatal message, the CLR's own
    # complaint, and any Go runtime output. Never truncated to a match.
    sed 's/^/  /' "$work/death-$k.txt" | tail -60
  done
fi

echo
echo "=== RESULT ==="
for i in "${!names[@]}"; do printf '  %-16s %s deaths / %s\n' "${names[$i]}" "${results[$i]}" "$iterations"; done
echo
base="${results[0]}"
if [ "$base" -eq 0 ]; then
  echo "  INCONCLUSIVE — the baseline did not crash, so there was nothing to fix."
  echo "  Raise -n; the rate has been measured between 1-in-60 and 3-in-4."
else
  clean=""; for i in "${!names[@]}"; do
    [ "$i" = 0 ] && continue
    [ "${results[$i]}" -eq 0 ] && clean="$clean ${names[$i]}"
  done
  if [ -n "$clean" ]; then
    echo "  GO'S EDITS TO HANDLERS IT DOES NOT OWN ARE THE CAUSE."
    echo "  Clean arm(s):$clean, against a baseline of $base/$iterations. Restoring the CLR's"
    echo "  own sigaction struct — flags AND mask together — removed the crash where clearing"
    echo "  SA_ONSTACK alone did nothing. Read the mask column in the report block above for"
    echo "  what Go changed; that difference is the defect, and a fix restores it rather than"
    echo "  reaching for stack sizes."
  else
    echo "  NOT THE SIGNAL PATH, AND NOW MEASURED TWICE. Neither clearing SA_ONSTACK"
    echo "  everywhere (run 35979184685) nor restoring the CLR's relocated handlers whole"
    echo "  changes the rate. Loading the engine kills this process by some other means."
    echo "  Next: the death output above, then a minimal console reproducer without vstest,"
    echo "  then strace -f -e futex for the errno."
  fi
fi
echo
echo "  Quote this with the host and binding block above."
