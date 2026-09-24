#!/usr/bin/env bash
#
# DW-398 WITHOUT VSTEST.
#
#   ./run.sh [-n N] [--pool N] [--hold MS]
#   ./run.sh --self-check        # the classifier's own judgement, any OS
#
# WHY THIS EXISTS. Every death on record has happened inside `dotnet test`.
# vstest brings its own threads and its own way of reporting a dead child, and
# no experiment has removed it, so it is an unexamined part of every result
# this entry holds. It also costs seconds per iteration, which is why the rate
# is known to ±20 points and not better.
#
# THE FIRST VERSION MEASURED AN EMPTY ROOM, AND THAT IS ON THE RECORD. Run
# 35983829443: 0/300 load, 0/300 noload -- with eight pool threads PARKED for
# 300ms and nothing allocated. Both cores on record show a pool thread
# interrupted by a signal WHILE IN MANAGED CODE, with the CLR's handler dying;
# that is the GC-suspension path, and a process that never allocates never
# GCs, never suspends anything, and never opens the window. Its verdict
# text -- "vstest is part of the trigger" -- was an over-claim and is
# withdrawn: the workload was wrong, not the hypothesis.
#
# TWO ARMS, THE SAME PROCESS, ONE VARIABLE:
#
#   load     pool threads allocating in managed code, dlopen the
#            engine, forced GCs for the hold, leave with the
#            workers still running
#   noload   identical, without the dlopen                  -- the control
#
# `--nogc` re-runs the first version's workload so the two are comparable.
#
# HOW TO READ IT. If `load` dies and `noload` does not, DW-398 reproduces in a
# bare console process: vstest is not part of the trigger, the GC-suspension
# reading of the cores is supported, and the instrument is cheap enough to
# bisect a Go toolchain with. If NEITHER dies with GCs forced and workers in
# managed code, the workload still differs from a testhost in some way that
# matters, and THAT is the next thing to name -- not "vstest is the trigger",
# which the first run already over-claimed once.
#
# THE EXIT STATUS IS THE RESULT. 134 is SIGABRT, 139 is SIGSEGV, and this
# entry has already conflated an abort with a segfault once. They are counted
# separately and reported separately, never merged into "deaths".
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../../.." && pwd)"
iterations=300
pool=8
hold=400
gcflag=--gc
selfcheck=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    -n|--iterations) iterations="$2"; shift 2 ;;
    --pool) pool="$2"; shift 2 ;;
    --hold) hold="$2"; shift 2 ;;
    --nogc) gcflag=--nogc; shift ;;
    --self-check) selfcheck=1; shift ;;
    *) echo "unknown argument '$1'" >&2; exit 1 ;;
  esac
done

# One place that turns a wait status into a word, so the self-check below is
# testing the thing the arms actually use.
classify() {
  case "$1" in
    0)   echo clean ;;
    134) echo SIGABRT ;;
    139) echo SIGSEGV ;;
    132) echo SIGILL ;;
    135) echo SIGBUS ;;
    136) echo SIGFPE ;;
    2|3) echo "exit$1" ;;
    *)   if [ "$1" -gt 128 ] 2>/dev/null; then echo "signal$(( $1 - 128 ))"; else echo "exit$1"; fi ;;
  esac
}

if [ "$selfcheck" = 1 ]; then
  fails=0
  check() {
    got="$(classify "$1")"
    if [ "$got" = "$2" ]; then printf '  ok    %-24s -> %s\n' "status $1" "$got"
    else printf '  FAIL  %-24s -> %s (want %s)\n' "status $1" "$got" "$2"; fails=$((fails+1)); fi
  }
  echo "=== classifier self-check ==="
  # Synthetic, including the two this entry must never merge.
  check 0 clean; check 134 SIGABRT; check 139 SIGSEGV; check 132 SIGILL
  check 135 SIGBUS; check 136 SIGFPE; check 1 exit1; check 2 exit2; check 3 exit3
  check 137 signal9; check 143 signal15
  # And a REAL status, so the arms are not reading a number bash never produces.
  # It must be a separate PROCESS: `( kill -ABRT $$ )` keeps the parent's $$
  # and aborts this script instead of a child, which is how it was first
  # written and what the first run of this self-check did.
  set +e; { bash -c 'kill -ABRT $$'; } 2>/dev/null; real=$?; set -e
  got="$(classify "$real")"
  if [ "$got" = SIGABRT ]; then printf '  ok    %-24s -> %s\n' "a real abort($real)" "$got"
  else printf '  FAIL  %-24s -> %s (want SIGABRT)\n' "a real abort($real)" "$got"; fails=$((fails+1)); fi
  set +e; ( exit 0 ) ; real=$?; set -e
  got="$(classify "$real")"
  if [ "$got" = clean ]; then printf '  ok    %-24s -> %s\n' "a real clean exit" "$got"
  else printf '  FAIL  %-24s -> %s (want clean)\n' "a real clean exit" "$got"; fails=$((fails+1)); fi
  echo
  [ "$fails" = 0 ] && { echo "ALL PASS"; exit 0; } || { echo "$fails FAILED"; exit 1; }
fi

[ "$(uname -s)" = Linux ] || { echo "Linux only (use --self-check elsewhere)." >&2; exit 1; }
case "$(uname -m)" in x86_64) rid=linux-x64 ;; aarch64) rid=linux-arm64 ;; *) echo "unknown arch" >&2; exit 1 ;; esac
native="$root/folio-dotnet/build/native/$rid/libfolio8_native.so"
[ -f "$native" ] || { echo "no native at $native" >&2; exit 1; }

work="$(mktemp -d)"
echo "building the reproducer"
dotnet build "$here" -c Release -v quiet --nologo -o "$work/bin" >"$work/build.log" 2>&1 || {
  sed 's/^/  /' "$work/build.log"; exit 1; }

cat <<EOF2

=== load reproducer, no vstest ===
  binding   : $(cd "$root" && git log --oneline -1 2>/dev/null || echo unknown)
  native    : $(sha256sum "$native" | awk '{print $1}')
  host      : $(uname -n), $(uname -s) $(uname -r), $(uname -m)
  runtime   : $(dotnet --version)
  iterations: $iterations per arm, pool $pool, hold ${hold}ms, $gcflag

EOF2

ulimit -c 0 2>/dev/null || true   # a core per death would dominate the runtime

declare -a names=() totals=() breakdowns=()
saved=0
for arm in load noload; do
  case "$arm" in
    load)   args=("$native" --load) ;;
    noload) args=(--noload) ;;
  esac
  declare -A tally=()
  bad=0
  printf '  %-8s ' "$arm"
  for i in $(seq 1 "$iterations"); do
    set +e
    err="$("$work/bin/repro" "${args[@]}" --pool "$pool" --hold "$hold" "$gcflag" 2>&1 >/dev/null)"
    st=$?
    set -e
    word="$(classify "$st")"
    tally["$word"]=$(( ${tally["$word"]:-0} + 1 ))
    if [ "$word" = clean ]; then printf '.'; else
      bad=$((bad + 1)); printf 'X'
      # glibc's __libc_fatal writes its reason to stderr and this entry has
      # never once read it. Keep the first few whole.
      if [ "$saved" -lt 3 ] && [ -n "$err" ]; then
        saved=$((saved + 1)); printf '%s' "$err" >"$work/death-$saved.txt"
      fi
    fi
  done
  b=""; for k in "${!tally[@]}"; do [ "$k" = clean ] && continue; b="$b $k=${tally[$k]}"; done
  echo "  -> $bad / $iterations${b:+  ($b )}"
  names+=("$arm"); totals+=("$bad"); breakdowns+=("${b:- none}")
  unset tally
done

if [ "$saved" -gt 0 ]; then
  echo
  echo "=== what the dying process said on stderr (first $saved, verbatim) ==="
  for k in $(seq 1 "$saved"); do echo "--- death $k ---"; sed 's/^/  /' "$work/death-$k.txt"; done
fi

echo
echo "=== RESULT ==="
for i in 0 1; do printf '  %-8s %s / %s  %s\n' "${names[$i]}" "${totals[$i]}" "$iterations" "${breakdowns[$i]}"; done
echo
if [ "${totals[1]}" -gt 0 ]; then
  echo "  REFUSED — the control died too, so this host kills a .NET process that never"
  echo "  loaded the engine and nothing here can be attributed to the library. Do not"
  echo "  quote the load arm. This is DW-397's failure mode and it is why a control is run."
elif [ "${totals[0]}" -gt 0 ]; then
  echo "  DW-398 REPRODUCES WITHOUT VSTEST. A bare console process that dlopens the engine"
  echo "  dies at ${totals[0]}/$iterations while the identical process without the load is clean."
  echo "  vstest is not part of the trigger, every rate in this entry stands, and the"
  echo "  instrument is now cheap enough to bisect the Go toolchain and the runtime with."
else
  echo "  DID NOT REPRODUCE WITH THIS WORKLOAD ($gcflag, pool $pool, hold ${hold}ms)."
  echo "  Loading the engine into a bare console process did not kill it once in $iterations,"
  echo "  where the same load under \`dotnet test\` dies on roughly one run in four. That does"
  echo "  NOT show vstest is the trigger -- the first run of this tool claimed exactly that on"
  echo "  a workload that never GC'd, and withdrew it. It shows this process still lacks"
  echo "  something a testhost has. Name the difference before running again; do not raise"
  echo "  -n and hope."
fi
echo
echo "  Quote this with the host and binding block above."
