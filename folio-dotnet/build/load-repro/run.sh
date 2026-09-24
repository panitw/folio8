#!/usr/bin/env bash
#
# DW-398 WITHOUT VSTEST.
#
#   ./run.sh [-n N] [--pool N] [--hold MS] [--arms a,b,...] [--native PATH] [--out DIR]
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
# RUN 35990136271 SETTLED THE CAUSE: baseline 83/150, control 0;
# restore-relocated 1, noonstack 1; restore-go, asyncpreemptoff and W^X-off
# at baseline. Removing SA_ONSTACK from the handlers Go re-flagged is the fix.
# The default arms are now the CONFIRMATION set, aimed at the residual 1/150:
#
# RUN 35993471395 ANSWERED THE RESIDUAL'S FIRST QUESTION, IN THE NEGATIVE:
# restore-relocated 3/250, +sync=call 10/250, restore-rtmin+sync=call 3/250,
# +sync=poll 4/250, against 206/250 and a clean control. The synced arms did
# not go to zero, so the 1-4% residual is NOT the init race and NOT a window.
# It is a second, rarer path, only with the engine loaded. The default arms
# now ask what it is:
#
#   load                                     baseline
#   noload                                   the control: identical, no dlopen
#   load:fix=restore-relocated               the fix (expect 1-4%)
#   load:fix=restore-all                     the fix PLUS Go's own five handlers
#                                            put back -- if the residual goes,
#                                            it lives in Go's handler path on
#                                            CLR threads (forwarding on the altstack)
#   load:fix=restore-relocated:env=GODEBUG=asyncpreemptoff=1
#                                            the fix, Go's SIGURG preemption off
#   load:fix=restore-relocated:sync=call     the fix after the first export (what ships)
#
# Every arm now also reports the kernel's `overflowed sigaltstack` lines that
# appeared DURING it, so the residual's deaths say whether they are
# nested-delivery overflows on an altstack still in use.
#
# --arms a,b,c replaces that list; an arm is
# `load|noload[:fix=MODE][:sync=none|call|poll][:env=K=V]...`.
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
native=""
iteration_timeout=60
arms=(load noload "load:fix=restore-relocated" "load:fix=restore-all" "load:fix=restore-relocated:env=GODEBUG=asyncpreemptoff=1" "load:fix=restore-relocated:sync=call")
selfcheck=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    -n|--iterations) iterations="$2"; shift 2 ;;
    --pool) pool="$2"; shift 2 ;;
    --hold) hold="$2"; shift 2 ;;
    --nogc) gcflag=--nogc; shift ;;
    --out) out="$2"; shift 2 ;;
    --native) native="$2"; shift 2 ;;
    --iteration-timeout) iteration_timeout="$2"; shift 2 ;;
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
    124) echo TIMEOUT ;;
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
  arm_load=""; arm_fix=none; arm_sync=none; arm_env=()
  local IFS=: part
  for part in $1; do
    case "$part" in
      load|noload) arm_load="$part" ;;
      fix=*) arm_fix="${part#fix=}" ;;
      sync=*) arm_sync="${part#sync=}" ;;
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
  check 0 clean; check 124 TIMEOUT; check 134 SIGABRT; check 139 SIGSEGV; check 132 SIGILL
  check 135 SIGBUS; check 136 SIGFPE; check 1 exit1; check 2 exit2; check 3 exit3
  check 137 signal9; check 143 signal15
  # The arm parser, on the shapes the default list uses and on two it must refuse.
  parm() {
    if parse_arm "$1" 2>/dev/null; then got="$arm_load fix=$arm_fix sync=$arm_sync env=${arm_env[*]+${arm_env[*]}}"; else got="REFUSED"; fi
    if [ "$got" = "$2" ]; then printf '  ok    %-24s -> %s\n' "arm '$1'" "$got"
    else printf '  FAIL  %-24s -> %s (want %s)\n' "arm '$1'" "$got" "$2"; fails=$((fails+1)); fi
  }
  parm "load"                                  "load fix=none sync=none env="
  parm "noload"                                "noload fix=none sync=none env="
  parm "load:fix=restore-go"                   "load fix=restore-go sync=none env="
  parm "load:fix=restore-rtmin:sync=call"      "load fix=restore-rtmin sync=call env="
  parm "load:env=GODEBUG=asyncpreemptoff=1"    "load fix=none sync=none env=GODEBUG=asyncpreemptoff=1"
  parm "load:fix=noonstack:env=A=1:env=B=2"    "load fix=noonstack sync=none env=A=1 B=2"
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
# --native lets soak.sh hand over the exact file its ledger row will name,
# so the mechanism check and the row it validates cannot drift apart.
[ -n "$native" ] || native="$root/folio-dotnet/build/native/$rid/libfolio8_native.so"
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
  args+=(--pool "$pool" --hold "$hold" "$gcflag" --fix "$arm_fix" --sync "$arm_sync")
  declare -A tally=()
  bad=0; first_line=""
  kl_before="$( (dmesg 2>/dev/null || sudo -n dmesg 2>/dev/null) | grep -c 'overflowed sigaltstack' || true)"; kl_before="${kl_before:-0}"
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
    # ONE ITERATION MAY NOT HANG THE RUN. Run 35993471395 sat in this loop for
    # over an hour with nothing to show; whichever arm it was, a hang is an
    # outcome and gets counted as one (TIMEOUT, exit 124) rather than
    # swallowing every arm after it. The hold is 400ms; a minute is generous.
    err="$(env ${arm_env[@]+"${arm_env[@]}"} ${dumpenv[@]+"${dumpenv[@]}"} timeout -k 5 "${iteration_timeout:-60}" "$work/bin/repro" "${args[@]}" 2>&1 >/dev/null)"
    st=$?
    set -e
    # The first iteration that PRINTED a [repro] line, not the first
    # iteration: on an AVX-512 host the first one can die before it gets
    # that far (run 35992109447), and the arm's touched= and stack numbers
    # would then never be seen.
    if [ -z "$first_line" ]; then
      first_line="$(printf '%s\n' "$err" | grep -m1 '^\[repro\]' || true)"
    fi
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
  [ -n "$first_line" ] || first_line="[repro] (no iteration of this arm lived long enough to print its line)"
  echo "      ${first_line#\[repro\] }"
  # THE KERNEL'S OWN WORD, after the baseline. A SIGSEGV delivered nested onto
  # an altstack the handler has already overflowed cannot get a frame; the x86
  # kernel forces SIG_DFL and logs `<comm>[pid] overflowed sigaltstack` -- the
  # line from the owner's WSL2 box that opened DW-396. Best-effort: needs
  # dmesg readable (the workflow lifts dmesg_restrict; elsewhere, sudo -n).
  # THE KERNEL'S LINES DURING THIS ARM, EVERY ARM. A death that carries
  # `overflowed sigaltstack` is a nested delivery onto an altstack already
  # in use; one that does not is the silent push-fault or a genuine fault.
  # The ratio per arm is part of the residual's description. (NO `| head`
  # under pipefail -- run 35992109447 lost five arms to that; grep -m1.)
  if [ "$bad" -gt 0 ]; then
    kl_after="$( (dmesg 2>/dev/null || sudo -n dmesg 2>/dev/null) | grep -c 'overflowed sigaltstack' || true)"; kl_after="${kl_after:-0}"
    kl=$(( kl_after - kl_before ))
    if [ "$kl" -gt 0 ]; then
      echo "      kernel: $kl 'overflowed sigaltstack' line(s) during this arm's $bad death(s)$( [ "$arm" = load ] && echo '; first:' )"
      [ "$arm" = load ] && (dmesg 2>/dev/null || sudo -n dmesg 2>/dev/null) | grep -m1 'overflowed sigaltstack' | sed 's/^/        /'
    else
      echo "      kernel: no 'overflowed sigaltstack' during this arm's $bad death(s) (silent push-fault, a genuine fault, or dmesg unreadable)"
    fi
  fi
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
  zero=""; reduced=""; nochange=""
  for i in "${!names[@]}"; do
    [ "$i" -le 1 ] && continue
    n="${totals[$i]}"
    if [ "$n" -eq 0 ]; then zero="$zero
      ${names[$i]}   0/$iterations   (${touched[$i]#\[repro\] })"
    elif [ $(( n * 10 )) -lt "$base" ]; then reduced="$reduced
      ${names[$i]}   $n/$iterations   ($(( base / n ))x fewer than baseline)   (${touched[$i]#\[repro\] })"
    else nochange="$nochange
      ${names[$i]}   $n/$iterations"; fi
  done
  echo
  if [ -n "$zero" ]; then
    echo "  ARMS THAT REMOVED THE CRASH ENTIRELY:$zero"
  fi
  if [ -n "$reduced" ]; then
    echo
    echo "  ARMS THAT REMOVED MOST OF IT (a tenfold cut or better). With the cause named --"
    echo "  the CLR's activation handler overflowing its 16 KiB altstack once Go sets"
    echo "  SA_ONSTACK on it -- this is the fix working and a residual to explain, not a"
    echo "  partial result. Run 35990136271's 83 -> 1 was reported as 'partial' by an"
    echo "  earlier version of this text, and that was wrong:$reduced"
  fi
  if [ -z "$zero" ] && [ -z "$reduced" ]; then
    echo "  NO ARM REMOVED THE CRASH. Whatever loading the engine does that kills this process"
    echo "  is not any of the things these arms undo."
  fi
  [ -n "$nochange" ] && { echo; echo "  No effect:$nochange"; }
  echo
  echo "  Believe an arm only if its touched= line shows it changed what it claims to, and"
  echo "  read sync= on the fix arms: an arm that fixed before Go's init finished can be undone."
else
  echo "  DID NOT REPRODUCE WITH THIS WORKLOAD ($gcflag, pool $pool, hold ${hold}ms)."
  echo "  Run 35987979509 got 177/300 from this same workload on the same runner image, so"
  echo "  a clean baseline here is a change in the host or the build, not in the defect."
  echo "  Compare the runtime version and native hash above with that run before anything else."
fi
echo
echo "  Quote this with the host and binding block above."
