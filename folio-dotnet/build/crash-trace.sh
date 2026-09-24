#!/usr/bin/env bash
#
# DW-396: NAME THE CRASH. Not another rate — a stack.
#
#   ./crash-trace.sh              # up to 40 attempts, stop at the first core
#   ./crash-trace.sh -n 100
#   ./crash-trace.sh --self-check
#
# WHY THIS EXISTS. The shipped binding kills the .NET test host on about one
# Linux amd64 run in four (load-exposure.sh, run 35933716928: 134 deaths in 500
# with the engine loaded, 0 in 500 without). Every one of those deaths is
# UNNAMED — no `overflowed sigaltstack` in the kernel buffer, no CLR error in
# the log, just "Test host process crashed". A crash rate cannot distinguish
# DW-396 from anything else that kills a process, and this entry has twice been
# sent the wrong way by reading one as though it could.
#
# THE ONE QUESTION IT ANSWERS: when the host dies, WHAT IS ON THE STACK?
# Specifically, is a Go signal handler (`runtime.sigtramp`, `sigtrampgo`,
# `runtime.sighandler`) on the faulting thread? If it is, this is the signal
# path and DW-396's mechanism is operating. If the faulting frames are the
# CLR's alone, or something unrelated, it is a different defect wearing the
# same symptom and must be filed as one.
#
# IT WANTS A REAL CORE, NOT A MANAGED DUMP. `dotnet-dump` shows managed frames
# and this question is about NATIVE ones — whose handler, on whose stack. So it
# routes the kernel's core to a file and reads it with gdb. That needs
# `kernel.core_pattern` pointed somewhere writable, which needs root, which is
# why this refuses rather than half-works when it cannot.
#
# A RUN THAT PRODUCES NO CORE IS ALSO A RESULT. If the host dies repeatedly and
# the kernel never writes one, the process is NOT dying on a signal — it is
# exiting some other way — and that rules the whole signal-stack family out.
# The verdict says so rather than reporting nothing.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
tests_csproj="$root/folio-dotnet/test/Folio8.Tests/Folio8.Tests.csproj"

# The workload that crashes: arm B from load-exposure.sh -- loads the engine
# once, renders nothing. Named identically so the two tools cannot drift.
filter="FullyQualifiedName~PackagingTests|FullyQualifiedName~DocsTests|FullyQualifiedName~SurfaceTests"

iterations=40
want_self_check=0
native=""
core_dir="${TMPDIR:-/tmp}/folio-cores"

usage() {
  cat <<'USAGE'
crash-trace.sh — when the test host dies, what is on the stack?

  ./crash-trace.sh [-n N] [--native PATH] [--core-dir DIR]
  ./crash-trace.sh --self-check

Runs the engine-loading workload until the host dies, then reads the kernel's
core with gdb and reports every thread's backtrace, with the Go signal-handler
frames called out. Needs root for kernel.core_pattern and gdb on PATH.

Exit status: 0 if a core was captured and read; 1 for a refusal, or for a run
that died without ever producing one (which is itself reported).
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --self-check) want_self_check=1; shift ;;
    -n|--iterations) iterations="$2"; shift 2 ;;
    --native) native="$2"; shift 2 ;;
    --core-dir) core_dir="$2"; shift 2 ;;
    *) echo "crash-trace: unknown argument '$1'" >&2; exit 1 ;;
  esac
done
case "$iterations" in ''|*[!0-9]*) echo "crash-trace: -n must be a whole number" >&2; exit 1 ;; esac

# --- the judgement, pure, so --self-check can drive it --------------------
#
# GO FRAMES ARE THE FINDING. `sigtramp` is the entry the kernel jumps to for a
# Go-installed handler; `sigtrampgo` and `sighandler` are what it calls. Any of
# them on the faulting thread means the signal path was taken.
classify() {
  local deaths="$1" cores="$2" go_frames="$3" clr_only="$4"
  if [ "$deaths" -eq 0 ]; then
    TAG="NO-CRASH"
    TEXT="the host did not die in this run. Nothing to trace; raise -n or use a host where it does."
  elif [ "$cores" -eq 0 ]; then
    TAG="NO-CORE"
    TEXT="the host died $deaths time(s) and the kernel wrote NO core. It is therefore not dying on a signal, which rules out the alternate-signal-stack family entirely -- DW-396 included -- and points at an ordinary process exit: an unhandled exception on a background thread, a FailFast, or a runtime abort that was handled before reaching a signal."
  elif [ "$go_frames" -gt 0 ]; then
    TAG="GO-SIGNAL-PATH"
    TEXT="a Go signal handler is on the faulting thread's stack. The signal path is being taken on a thread the binding does not own, which is DW-396's mechanism operating -- and it is NOT addressed by enlarging the alternate stack of binding-owned threads only."
  elif [ "$clr_only" -gt 0 ]; then
    TAG="CLR-ONLY"
    TEXT="a core was read and the faulting thread carries CLR frames with no Go signal handler above them. The signal path is not implicated; this is a different defect wearing DW-396's symptom and must be filed as one."
  else
    TAG="UNREADABLE"
    TEXT="a core was written but no backtrace could be resolved from it. Check that gdb is present and that the binaries are not stripped."
  fi
}

if [ "$want_self_check" = 1 ]; then
  c=0; bad=0
  expect() { c=$((c+1)); if [ "$2" = "$3" ]; then printf '  ok   %-46s %s\n' "$1" "$3"; else bad=$((bad+1)); printf '  FAIL %-46s expected %s got %s\n' "$1" "$2" "$3"; fi; }
  echo "=== crash-trace.sh --self-check ==="; echo
  classify 0 0 0 0; expect "no death at all"                     NO-CRASH       "$TAG"
  classify 5 0 0 0; expect "deaths but no core: not a signal"    NO-CORE        "$TAG"
  classify 1 1 3 0; expect "Go handler on the stack"             GO-SIGNAL-PATH "$TAG"
  classify 1 1 0 9; expect "CLR frames only"                     CLR-ONLY       "$TAG"
  classify 1 1 0 0; expect "core written, nothing resolved"      UNREADABLE     "$TAG"
  classify 9 2 1 40; expect "one Go frame among many CLR frames" GO-SIGNAL-PATH "$TAG"
  echo
  [ "$bad" = 0 ] || { echo "$bad of $c WRONG"; exit 1; }
  echo "all $c cases rendered as expected. It measures nothing."
  exit 0
fi

[ "$(uname -s)" = Linux ] || { echo "crash-trace: Linux only; --self-check works anywhere." >&2; exit 1; }
command -v gdb >/dev/null 2>&1 || { echo "crash-trace: gdb is not on PATH, and reading the core is the entire point." >&2; exit 1; }

# ROOT, OR NOTHING. A core routed to Ubuntu's apport pipe is a core this script
# cannot read, and silently producing no trace is the failure mode it exists to
# remove.
if ! sudo -n true 2>/dev/null; then
  echo "crash-trace: needs passwordless sudo to point kernel.core_pattern at a file." >&2
  echo "  Ubuntu routes cores to apport by default, and this cannot read those." >&2
  exit 1
fi

mkdir -p "$core_dir"; chmod 777 "$core_dir"
previous_pattern="$(cat /proc/sys/kernel/core_pattern)"
restore() { echo "$previous_pattern" | sudo tee /proc/sys/kernel/core_pattern >/dev/null 2>&1 || true; }
trap restore EXIT
echo "$core_dir/core.%e.%p" | sudo tee /proc/sys/kernel/core_pattern >/dev/null
ulimit -c unlimited

if [ -z "$native" ]; then
  case "$(uname -m)" in x86_64) rid=linux-x64 ;; aarch64) rid=linux-arm64 ;; *) rid="" ;; esac
  native="$root/folio-dotnet/build/native/host/libfolio8_native.so"
  [ -n "$rid" ] && [ -f "$root/folio-dotnet/build/native/$rid/libfolio8_native.so" ] &&
    native="$root/folio-dotnet/build/native/$rid/libfolio8_native.so"
fi
[ -f "$native" ] || { echo "crash-trace: no native at $native" >&2; exit 1; }

work="$(mktemp -d)"; stage="$work/native"; mkdir -p "$stage/host"
cp "$native" "$stage/host/libfolio8_native.so"

cat <<EOF
=== crash-trace.sh — name the crash ===
  binding     : $(cd "$root" && git log --oneline -1 2>/dev/null || echo unknown)
  native      : $(sha256sum "$native" | awk '{print $1}')
  host        : $(uname -n), $(uname -s) $(uname -r), $(uname -m)
  core_pattern: $(cat /proc/sys/kernel/core_pattern)   [restored on exit]
  cores       : $core_dir
  attempts    : up to $iterations

EOF

dotnet build "$tests_csproj" -c Release -v quiet --nologo -p:FolioNativeDir="$stage" >"$work/build.log" 2>&1 || {
  sed 's/^/  /' "$work/build.log"; echo "crash-trace: the suite did not build."; exit 1; }

deaths=0
for i in $(seq 1 "$iterations"); do
  set +e
  dotnet test "$tests_csproj" -c Release --no-build --nologo -v quiet \
    -p:FolioNativeDir="$stage" --filter "$filter" >"$work/iteration-$i.log" 2>&1
  set -e
  if grep -q -e 'Test host process crashed' -e 'The active test run was aborted' "$work/iteration-$i.log" 2>/dev/null; then
    deaths=$((deaths + 1))
    printf '\n  attempt %d: HOST DIED\n' "$i"
    # The kernel writes asynchronously; give it a moment before looking.
    sleep 2
    if ls "$core_dir"/core.* >/dev/null 2>&1; then break; fi
    echo "    ...no core yet"
  else
    printf '.'
  fi
done
echo

cores=0; go_frames=0; clr_only=0
for core in "$core_dir"/core.*; do
  [ -e "$core" ] || continue
  cores=$((cores + 1))
  echo "=== backtrace from $(basename "$core") ($(du -h "$core" | cut -f1)) ==="

  # THE EXECUTABLE, OR GDB LOADS NO SHARED LIBRARIES AND EVERY FRAME IS `??`.
  # The first cut passed --core alone and got exactly that, twice: once on the
  # owner's WSL2 box and once here, where it produced an UNREADABLE verdict on
  # a core that was perfectly good. `file` reports the core's execfn; without
  # it gdb has no library list to attribute an address to, which is the whole
  # analysis.
  exe="$(file "$core" 2>/dev/null | sed -n "s/.*execfn: '\([^']*\)'.*/\1/p")"
  if [ -z "$exe" ] || [ ! -f "$exe" ]; then
    exe="$(file "$core" 2>/dev/null | sed -n "s/.*from '\([^ ']*\).*/\1/p" | head -1)"
  fi
  [ -n "$exe" ] && [ -f "$exe" ] || exe="$(command -v dotnet || true)"
  echo "  executable: ${exe:-<unresolved>}"

  gdb -q -batch -ex "set pagination off" \
      -ex "thread apply all bt" \
      -ex "echo \n===MAPPINGS===\n" -ex "info proc mappings" \
      -ex "echo \n===REGISTERS===\n" -ex "info registers" \
      ${exe:+"$exe"} --core="$core" \
      >"$work/backtrace.txt" 2>"$work/gdb.err" || true

  # ATTRIBUTION WITHOUT SYMBOLS, WHICH IS ALL THIS QUESTION NEEDS. The engine
  # is stripped Go and the CLR is not shipped with symbols, so frame NAMES may
  # never resolve -- but a return address inside libfolio8_native.so is a Go
  # frame whatever it is called, and that is the finding. gdb prints `from
  # <path>` for any frame it can place in a shared object even with no symbols.
  echo "--- frames placed in the engine (Go) ---"
  grep -nE "libfolio8_native" "$work/backtrace.txt" | head -10 || echo "  (none)"
  # KEPT BEFORE IT IS PRINTED. The first version copied the backtrace out at
  # the END of this block and a `sed | head` in the middle took SIGPIPE, which
  # under `pipefail` killed the script with exit 4 -- losing the trace it had
  # just spent 150 attempts earning. The artefact is written first now, and
  # every pipe below tolerates a closed reader.
  cp "$work/backtrace.txt" "$core_dir/backtrace-$(basename "$core").txt" 2>/dev/null || true
  cp "$work/gdb.err" "$core_dir/gdb-$(basename "$core").err" 2>/dev/null || true

  # The faulting thread first, then anything naming a Go handler anywhere.
  set +o pipefail
  sed -n '/^Thread 1 /,/^Thread 2 /p' "$work/backtrace.txt" | head -40
  set -o pipefail
  echo
  echo
  echo "--- frames naming a Go signal handler, any thread ---"
  if grep -nE "sigtramp|sigtrampgo|runtime\.sighandler|cgoSigtramp" "$work/backtrace.txt"; then
    :
  else
    echo "  (none by name — stripped Go resolves no symbols; the library placement above is the evidence)"
  fi
  # EITHER IS THE SIGNAL PATH: a handler named, or ANY frame inside the engine
  # on a thread that died. The binding never renders in this workload, so a
  # frame in libfolio8_native.so on the faulting stack can only have arrived
  # through a signal.
  go_frames=$(( $(grep -cE "sigtramp|sigtrampgo|runtime\.sighandler|cgoSigtramp" "$work/backtrace.txt" || true) \
              + $(sed -n '/^Thread 1 /,/^Thread 2 /p' "$work/backtrace.txt" | grep -cE "libfolio8_native" || true) ))
  echo
  echo "--- frames naming libcoreclr or the engine ---"
  grep -nE "libcoreclr|libfolio8_native" "$work/backtrace.txt" | head -10 || echo "  (none)"
  clr_only=$(grep -cE "libcoreclr" "$work/backtrace.txt" || true)

  # ABORT IS A DIFFERENT DEATH FROM A SEGFAULT, AND THIS RUN FOUND ONE.
  # Run 35945107391's faulting thread was inside glibc's futex_fatal_error --
  # __libc_fatal, i.e. abort() -- reached from the CLR's handler under
  # `<signal handler called>`. That is why no kernel line ever appeared for
  # any of these deaths: the kernel logs an unhandled fatal signal, not a
  # process that aborts itself. Surfaced by name so the next reader does not
  # have to rediscover it from a stack.
  if grep -qE "futex_fatal_error|__libc_fatal|abort \(\)" "$work/backtrace.txt" 2>/dev/null; then
    echo
    echo "--- NOTE: this is an ABORT, not a segfault ---"
    grep -nE "futex_fatal_error|__libc_fatal|abort \(\)|<signal handler called>" "$work/backtrace.txt" | head -5
  fi
  break
done

classify "$deaths" "$cores" "$go_frames" "$clr_only"
cat <<EOF

=== VERDICT: $TAG ===

  $TEXT

  deaths      : $deaths of $iterations attempts
  cores read  : $cores
  Go handler frames : $go_frames
  CLR frames        : $clr_only
  binding     : $(cd "$root" && git log --oneline -1 2>/dev/null || echo unknown)
  host        : $(uname -n), $(uname -s) $(uname -r), $(uname -m)
  artefacts   : $core_dir

  Quote this verdict with the block above attached.
EOF

[ "$TAG" = "NO-CORE" ] && exit 1
[ "$TAG" = "NO-CRASH" ] && exit 1
exit 0
