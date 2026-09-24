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
# IT REPRODUCES: run 35987979509, 177/300 SIGSEGV with the load, 0/300
# without, ~1s an iteration. So this is a BISECTION now. The first two arms
# are fixed -- the baseline and the control -- and every arm after them
# undoes ONE thing loading the engine does to the process, in the process
# itself, with what it touched printed to stderr and recorded here per arm.
#
#   load                          baseline
#   noload                        the control: identical, no dlopen
#   load:fix=restore-go           the CLR's handlers back where Go REPLACED them
#   load:fix=restore-relocated    the CLR's structs back where Go only re-flagged them
#   load:fix=noonstack            SA_ONSTACK cleared everywhere, handlers left
#   load:env=GODEBUG=asyncpreemptoff=1   Go's SIGURG preemption off
#   load:env=DOTNET_EnableWriteXorExecute=0   the CLR's W^X double-mapping off
#
# --arms a,b,c replaces that list; an arm is `load|noload[:fix=MODE][:env=K=V]...`.
# `--nogc` re-runs the first version's workload so the two are comparable.
#
# THE FIRST DEATH ALSO YIELDS A RUNTIME DUMP. Every core so far was a kernel
# core, which the .NET DAC refuses to read, so no frame in this defect has a
# name. The baseline arm runs with DOTNET_DbgEnableMiniDump until one dump
# exists; dotnet-dump then names the managed frame the worker was interrupted
# in and, via the symbol server, the CLR handler frames above it.
#
# HOW TO READ IT. The control must be clean or the run is refused. Against a
# baseline in the hundreds, an arm at ZERO has removed the cause and an arm
# at the baseline has not touched it; anything in between is reported as
# partial and is not a finding on its own. Read each arm's `touched=` line
# before believing its number -- an arm that changed nothing proves nothing.
#
# THE EXIT STATUS IS THE RESULT. 134 is SIGABRT, 139 is SIGSEGV, and this
# entry has already conflated an abort with a segfault once. They are counted
# separately and reported separately, never merged into "deaths".
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../../.." && pwd)"
iterations=150
pool=8
hold=400
gcflag=--gc
out=""
arms=(load noload "load:fix=restore-go" "load:fix=restore-relocated" "load:fix=noonstack" "load:env=GODEBUG=asyncpreemptoff=1" "load:env=DOTNET_EnableWriteXorExecute=0")
selfcheck=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    -n|--iterations) iterations="$2"; shift 2 ;;
    --pool) pool="$2"; shift 2 ;;
    --hold) hold="$2"; shift 2 ;;
    --nogc) gcflag=--nogc; shift ;;
    --out) out="$2"; shift 2 ;;
    --arms) IFS=, read -r -a arms <<<"$2"; shift 2 ;;
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

# `load|noload[:fix=MODE][:env=K=V]...` -> sets arm_load, arm_fix, arm_env[].
# One function, so the self-check exercises the same parser the arms use.
parse_arm() {
  arm_load=""; arm_fix=none; arm_env=()
  local IFS=: part
  for part in $1; do
    case "$part" in
      load|noload) arm_load="$part" ;;
      fix=*) arm_fix="${part#fix=}" ;;
      env=*) arm_env+=("${part#env=}") ;;
      *) echo "bad arm token '$part' in '$1'" >&2; return 1 ;;
    esac
  done
  [ -n "$arm_load" ] || { echo "arm '$1' names neither load nor noload" >&2; return 1; }
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
  # The arm parser, on the shapes the default list uses and on two it must refuse.
  parm() {
    if parse_arm "$1" 2>/dev/null; then got="$arm_load fix=$arm_fix env=${arm_env[*]+${arm_env[*]}}"; else got="REFUSED"; fi
    if [ "$got" = "$2" ]; then printf '  ok    %-24s -> %s\n' "arm '$1'" "$got"
    else printf '  FAIL  %-24s -> %s (want %s)\n' "arm '$1'" "$got" "$2"; fails=$((fails+1)); fi
  }
  parm "load"                                  "load fix=none env="
  parm "noload"                                "noload fix=none env="
  parm "load:fix=restore-go"                   "load fix=restore-go env="
  parm "load:env=GODEBUG=asyncpreemptoff=1"    "load fix=none env=GODEBUG=asyncpreemptoff=1"
  parm "load:fix=noonstack:env=A=1:env=B=2"    "load fix=noonstack env=A=1 B=2"
  parm "fix=noonstack"                         "REFUSED"
  parm "load:bogus"                            "REFUSED"
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
[ -n "$out" ] && mkdir -p "$out"
export PATH="$HOME/.dotnet/tools:$PATH"
command -v dotnet-dump >/dev/null 2>&1 || dotnet tool install -g dotnet-dump >/dev/null 2>&1 || echo "  (dotnet-dump not installable; the first dump will be kept unread)"
echo "building the reproducer"
dotnet build "$here" -c Release -v quiet --nologo -o "$work/bin" >"$work/build.log" 2>&1 || {
  sed 's/^/  /' "$work/build.log"; exit 1; }

cat <<EOF2

=== load reproducer, no vstest ===
  binding   : $(cd "$root" && git log --oneline -1 2>/dev/null || echo unknown)
  native    : $(sha256sum "$native" | awk '{print $1}')
  host      : $(uname -n), $(uname -s) $(uname -r), $(uname -m)
  runtime   : $(dotnet --version)
  cpu       : $(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2- | sed 's/^ *//'); xsave-relevant flags: $(grep -m1 '^flags' /proc/cpuinfo 2>/dev/null | tr ' ' '\n' | grep -E '^(avx2|avx512[a-z_]*|amx[a-z_]*|xsave[a-z]*|pku)$' | sort -u | tr '\n' ' ')
  iterations: $iterations per arm, pool $pool, hold ${hold}ms, $gcflag
  arms      : ${arms[*]}

EOF2

ulimit -c 0 2>/dev/null || true   # a core per death would dominate the runtime

declare -a names=() totals=() breakdowns=() touched=()
saved=0
mkdir -p "$work/dumps"
dump=""
for arm in "${arms[@]}"; do
  parse_arm "$arm" || exit 1
  args=(); [ "$arm_load" = load ] && args=("$native" --load) || args=(--noload)
  args+=(--pool "$pool" --hold "$hold" "$gcflag" --fix "$arm_fix")
  declare -A tally=()
  bad=0; first_line=""
  printf '  %-42s ' "$arm"
  for i in $(seq 1 "$iterations"); do
    # The baseline arm asks the runtime for its own dump on a fatal signal,
    # until one exists. Kernel cores are what every earlier trace read, and
    # the DAC will not open those.
    dumpenv=()
    if [ "$arm" = load ] && [ -z "$dump" ]; then
      dumpenv=(DOTNET_DbgEnableMiniDump=1 DOTNET_DbgMiniDumpType=1 "DOTNET_DbgMiniDumpName=$work/dumps/dump.%p")
    fi
    set +e
    err="$(env ${arm_env[@]+"${arm_env[@]}"} ${dumpenv[@]+"${dumpenv[@]}"} "$work/bin/repro" "${args[@]}" 2>&1 >/dev/null)"
    st=$?
    set -e
    [ -n "$first_line" ] || first_line="$(printf '%s\n' "$err" | grep -m1 '^\[repro\]' || echo '(no [repro] line: the process printed nothing)')"
    word="$(classify "$st")"
    tally["$word"]=$(( ${tally["$word"]:-0} + 1 ))
    if [ "$word" = clean ]; then printf '.'; else
      bad=$((bad + 1)); printf 'X'
      if [ "$saved" -lt 3 ] && [ -n "$err" ]; then
        saved=$((saved + 1)); printf '%s' "$err" >"$work/death-$saved.txt"
      fi
      if [ -z "$dump" ]; then dump="$(ls -t "$work/dumps"/dump.* 2>/dev/null | head -1 || true)"; fi
    fi
  done
  b=""; for k in "${!tally[@]}"; do [ "$k" = clean ] && continue; b="$b $k=${tally[$k]}"; done
  echo "  -> $bad / $iterations${b:+  ($b )}"
  echo "      ${first_line#\[repro\] }"
  names+=("$arm"); totals+=("$bad"); breakdowns+=("${b:- none}"); touched+=("$first_line")
  unset tally
done

if [ "$saved" -gt 0 ]; then
  echo
  echo "=== what the dying process said on stderr (first $saved, verbatim) ==="
  for k in $(seq 1 "$saved"); do echo "--- death $k ---"; sed 's/^/  /' "$work/death-$k.txt"; done
fi

echo
echo "=== the first death, read by the runtime's own tools ==="
if [ -z "$dump" ]; then
  echo "  no runtime dump was written. Either the baseline never died, or the fatal signal"
  echo "  never reached the CLR's handler (a handler that dies before createdump runs, or a"
  echo "  signal the kernel delivered as SIG_DFL, leaves nothing) -- and that is itself a"
  echo "  fact about how the process dies."
elif ! command -v dotnet-dump >/dev/null 2>&1; then
  echo "  dump at $dump, but dotnet-dump is unavailable to read it."
else
  echo "  dump: $(basename "$dump") ($(du -h "$dump" | cut -f1))"
  # clrstack -f: managed AND native frames of the faulting thread, the
  # native ones symbolised from Microsoft's server. This is the first time
  # any frame in this defect can carry a name.
  timeout 900 dotnet-dump analyze "$dump" -c "setsymbolserver -ms" -c "threads" -c "clrstack -f" -c "exit" \
    >"$work/analysis.txt" 2>&1 || echo "  (dotnet-dump did not complete cleanly; what it wrote follows)"
  if [ -n "$out" ]; then cp "$work/analysis.txt" "$out/dump-analysis.txt" 2>/dev/null || true; fi
  set +o pipefail
  # The faulting thread's stack, trimmed; the whole thing is in the artefact.
  sed -n '/^OS Thread Id/,$p' "$work/analysis.txt" | grep -vE '^\s*$' | head -50 | sed 's/^/  /'
  set -o pipefail
fi
if [ -n "$out" ]; then
  for k in $(seq 1 "$saved"); do cp "$work/death-$k.txt" "$out/" 2>/dev/null || true; done
fi

echo
echo "=== RESULT ==="
for i in "${!names[@]}"; do printf '  %-42s %4s / %s  %s\n' "${names[$i]}" "${totals[$i]}" "$iterations" "${breakdowns[$i]}"; done
echo
if [ "${totals[1]}" -gt 0 ]; then
  echo "  REFUSED — the control died too, so this host kills a .NET process that never"
  echo "  loaded the engine and nothing here can be attributed to the library. Do not"
  echo "  quote the load arm. This is DW-397's failure mode and it is why a control is run."
elif [ "${totals[0]}" -gt 0 ]; then
  base="${totals[0]}"
  echo "  DW-398 REPRODUCES WITHOUT VSTEST: $base/$iterations with the load, 0/$iterations without."
  zero=""; partial=""; nochange=""
  for i in "${!names[@]}"; do
    [ "$i" -le 1 ] && continue
    n="${totals[$i]}"
    if [ "$n" -eq 0 ]; then zero="$zero
      ${names[$i]}   (${touched[$i]#\[repro\] })"
    elif [ $(( n * 3 )) -lt "$base" ]; then partial="$partial
      ${names[$i]}   $n/$iterations"
    else nochange="$nochange
      ${names[$i]}   $n/$iterations"; fi
  done
  echo
  if [ -n "$zero" ]; then
    echo "  ARMS THAT REMOVED THE CRASH ENTIRELY -- each names a cause:$zero"
  else
    echo "  NO ARM REMOVED THE CRASH. Whatever loading the engine does that kills this process"
    echo "  is not any of the things these arms undo."
  fi
  [ -n "$partial" ] && { echo; echo "  Partial (under a third of baseline; suggestive, not a finding):$partial"; }
  [ -n "$nochange" ] && { echo; echo "  No effect:$nochange"; }
  echo
  echo "  Believe an arm only if its touched= line shows it changed what it claims to."
else
  echo "  DID NOT REPRODUCE WITH THIS WORKLOAD ($gcflag, pool $pool, hold ${hold}ms)."
  echo "  Run 35987979509 got 177/300 from this same workload on the same runner image, so"
  echo "  a clean baseline here is a change in the host or the build, not in the defect."
  echo "  Compare the runtime version and native hash above with that run before anything else."
fi
echo
echo "  Quote this with the host and binding block above."
