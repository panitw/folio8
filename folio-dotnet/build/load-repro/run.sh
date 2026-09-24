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
# RUN 35993471395: restore-relocated 3/250, +sync=call 10/250,
# restore-rtmin+sync=call 3/250, +sync=poll 4/250, against 206/250 and a
# clean control. This file then said the residual was "NOT a window", and
# that was the wrong reading of a right result: the sync arms do not CLOSE
# the window, they LENGTHEN it -- the restore runs after the sync returns,
# and the workers are running the whole time. Run 36003345966's census
# (strace on the control and on the fixed arm) said the rest: the control
# takes no SIGSEGV at all in 100 iterations, and the fixed arm's one death
# was SEGV_ACCERR eight bytes under a page boundary -- a push into the
# guard page the CLR maps under its own altstack (its 16 KiB mapping is
# 12 KiB of stack over a PROT_NONE page). Something still runs on the
# altstack with the fix in, and the only thing that can is an activation
# delivered between Go's constructor re-flagging the handler and this
# process putting the flag back. Run 36002483670 fits: restore-all 8/250
# with a kernel `overflowed sigaltstack` line, so the residual is not in
# Go's handlers either. The default arms now test the window directly:
#
# THE ENGINE NOW CLOSES THE WINDOW ITSELF (dispositions_linux.c: a
# constructor linked right after Go's puts the flags back), so a plain `load`
# is the shipped configuration and is expected clean. The baseline that shows
# the mechanism is still live on the host -- without which a clean `load`
# means nothing -- sets FOLIO8_SIGNAL_DISPOSITIONS=leave, which the engine
# honours once, at load, and reports as mode=leave. Every load arm prints the
# engine's own report ([repro-engine]) after its hold.
#
#   load:env=FOLIO8_SIGNAL_DISPOSITIONS=leave  baseline: Go's edits left in place
#   noload                                   the control: identical, no dlopen
#   load                                     what ships (PREDICTION: 0, or 1 in 250)
#   load:fix=restore-relocated               the binding's fallback on top: nothing
#                                            left to touch (touched=-), same rate
#
# Earlier arm sets, kept runnable: fix=restore-all (Go's five put back too),
# env=GODEBUG=asyncpreemptoff=1, sync=call, workers=after, delay=MS (the
# window arms; run 36006849443).
#
# Every arm now also reports the kernel's `overflowed sigaltstack` lines that
# appeared DURING it, so the residual's deaths say whether they are
# nested-delivery overflows on an altstack still in use.
#
# --arms a,b,c replaces that list; an arm is
# `load|noload[:fix=MODE][:sync=none|call|poll][:workers=before|after][:delay=MS][:env=K=V]...`.
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
core_arm=""   # --core-arm NAME: keep the first core that arm produces, and read it
strace_iterations=0   # --strace-iterations N: before each arm, N iterations under strace -f -e trace=signal, summarised
strace_arms=""        # --strace-arms a,b: only these arms get the census (default: every arm)
arms=("load:env=FOLIO8_SIGNAL_DISPOSITIONS=leave" noload load "load:fix=restore-relocated")
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
    --core-arm) core_arm="$2"; shift 2 ;;
    --strace-iterations) strace_iterations="$2"; shift 2 ;;
    --strace-arms) strace_arms=",$2,"; shift 2 ;;
    --arms) IFS=, read -r -a arms <<<"$2"; shift 2 ;;
    --self-check) selfcheck=1; shift ;;
    *) echo "unknown argument '$1'" >&2; exit 1 ;;
  esac
done

# The CPU's name, on x86 and on arm: arm's /proc/cpuinfo has no `model name`
# (the arm64 leg printed an empty banner), so fall back to lscpu, then to the
# implementer/part pair the kernel does print.
cpu_name() {
  local n
  n="$(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2- | sed 's/^ *//')"
  [ -n "$n" ] || n="$(lscpu 2>/dev/null | grep -m1 -i '^model name' | cut -d: -f2- | sed 's/^ *//')"
  [ -n "$n" ] || n="$(grep -m1 'CPU implementer' /proc/cpuinfo 2>/dev/null | cut -d: -f2- | sed 's/^ *//') part $(grep -m1 'CPU part' /proc/cpuinfo 2>/dev/null | cut -d: -f2- | sed 's/^ *//')"
  printf '%s' "$n"
}

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

# `load|noload[:fix=MODE][:sync=..][:workers=..][:delay=MS][:env=K=V]...` ->
# sets arm_load, arm_fix, arm_sync, arm_workers, arm_delay, arm_env[].
# One function, so the self-check exercises the same parser the arms use.
parse_arm() {
  arm_load=""; arm_fix=none; arm_sync=none; arm_workers=before; arm_delay=0; arm_env=()
  local IFS=: part
  for part in $1; do
    case "$part" in
      load|noload) arm_load="$part" ;;
      fix=*) arm_fix="${part#fix=}" ;;
      sync=*) arm_sync="${part#sync=}" ;;
      workers=before|workers=after) arm_workers="${part#workers=}" ;;
      workers=*) echo "bad workers value in '$1' (before|after)" >&2; return 1 ;;
      delay=*) arm_delay="${part#delay=}"; [[ "$arm_delay" =~ ^[0-9]+$ ]] || { echo "bad delay in '$1' (milliseconds)" >&2; return 1; } ;;
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
    if parse_arm "$1" 2>/dev/null; then got="$arm_load fix=$arm_fix sync=$arm_sync workers=$arm_workers delay=$arm_delay env=${arm_env[*]+${arm_env[*]}}"; else got="REFUSED"; fi
    if [ "$got" = "$2" ]; then printf '  ok    %-24s -> %s\n' "arm '$1'" "$got"
    else printf '  FAIL  %-24s -> %s (want %s)\n' "arm '$1'" "$got" "$2"; fails=$((fails+1)); fi
  }
  parm "load"                                  "load fix=none sync=none workers=before delay=0 env="
  parm "noload"                                "noload fix=none sync=none workers=before delay=0 env="
  parm "load:fix=restore-go"                   "load fix=restore-go sync=none workers=before delay=0 env="
  parm "load:fix=restore-rtmin:sync=call"      "load fix=restore-rtmin sync=call workers=before delay=0 env="
  parm "load:env=GODEBUG=asyncpreemptoff=1"    "load fix=none sync=none workers=before delay=0 env=GODEBUG=asyncpreemptoff=1"
  parm "load:fix=noonstack:env=A=1:env=B=2"    "load fix=noonstack sync=none workers=before delay=0 env=A=1 B=2"
  parm "load:fix=restore-relocated:workers=after" "load fix=restore-relocated sync=none workers=after delay=0 env="
  parm "load:fix=restore-relocated:delay=400"  "load fix=restore-relocated sync=none workers=before delay=400 env="
  parm "load:workers=sometimes"                "REFUSED"
  parm "load:delay=soon"                       "REFUSED"
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
# WHATEVER EXITS, THE ARTEFACTS GO OUT. Runs 36002483670 and 36003345966
# both died mid-script (a ulimit, a pipe) and uploaded nothing, so every
# copy to --out happens here, on exit, not at the end of a happy path.
previous_pattern=""
on_exit() {
  [ -n "$previous_pattern" ] && { echo "$previous_pattern" | sudo -n tee /proc/sys/kernel/core_pattern >/dev/null 2>&1 || true; }
  if [ -n "$out" ]; then
    cp "$work"/death-*.txt "$out/" 2>/dev/null || true
    [ -f "$work/analysis.txt" ] && cp "$work/analysis.txt" "$out/dump-analysis.txt" 2>/dev/null || true
    [ -f "$work/core-report.txt" ] && cp "$work/core-report.txt" "$out/" 2>/dev/null || true
  fi
}
trap on_exit EXIT
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
  cpu       : $(cpu_name); xsave-relevant flags: $(grep -m1 '^flags' /proc/cpuinfo 2>/dev/null | tr ' ' '\n' | grep -E '^(avx2|avx512[a-z_]*|amx[a-z_]*|xsave[a-z]*|pku)$' | sort -u | tr '\n' ' ')
  iterations: $iterations per arm, pool $pool, hold ${hold}ms, $gcflag
  arms      : ${arms[*]}

EOF2

ulimit -S -c 0 2>/dev/null || true   # a core per death would dominate the runtime

# A CORE FROM THE ARM NAMED BY --core-arm. The residual with the fix is 1-4%
# and unexplained (runs 35993471395, 35999753222); its death has never been
# read. For that one arm cores are enabled and routed to a file, the first
# one is kept, and read-core.sh reads it after the arm. Needs passwordless
# sudo for kernel.core_pattern; without it the arm runs and says so.
core_dir=""
if [ -n "$core_arm" ]; then
  if sudo -n true 2>/dev/null; then
    core_dir="$work/cores"; mkdir -p "$core_dir"; chmod 777 "$core_dir"
    previous_pattern="$(cat /proc/sys/kernel/core_pattern)"
    echo "$core_dir/core.%e.%p" | sudo tee /proc/sys/kernel/core_pattern >/dev/null
    echo "  cores     : enabled for arm '$core_arm' -> $core_dir"
  else
    echo "  cores     : --core-arm '$core_arm' needs passwordless sudo for kernel.core_pattern; running without cores"
    core_arm=""
  fi
fi

declare -a names=() totals=() breakdowns=() touched=()
saved=0
mkdir -p "$work/dumps"
dump=""
for arm in "${arms[@]}"; do
  parse_arm "$arm" || exit 1
  args=(); [ "$arm_load" = load ] && args=("$native" --load) || args=(--noload)
  args+=(--pool "$pool" --hold "$hold" "$gcflag" --fix "$arm_fix" --sync "$arm_sync" --workers "$arm_workers" --delay "$arm_delay")
  declare -A tally=()
  bad=0; first_line=""; engine_line=""; saved_arm=0
  if [ -n "$core_arm" ] && [ "$arm" = "$core_arm" ]; then ulimit -S -c unlimited; else ulimit -S -c 0 2>/dev/null || true; fi
  # WHICH SIGNALS A THREAD OF THIS PROCESS ACTUALLY TAKES. The residual with
  # the fix is a SIGSEGV that only kills the process with Go's handler in
  # front of the CLR's (run 36000448980: restore-all 0/250). A signal the CLR
  # takes and SURVIVES is invisible in the control -- nothing dies -- so the
  # question "does this workload raise SIGSEGV at all, and with what si_code
  # and si_addr" has never been asked. strace answers it per thread; the
  # timing under strace is different, so these iterations are not counted
  # in the arm's rate, only their signals are summarised.
  if [ "$strace_iterations" -gt 0 ] && command -v strace >/dev/null 2>&1 \
     && { [ -z "$strace_arms" ] || [[ "$strace_arms" == *",$arm,"* ]]; }; then
    sdir="${out:-$work}/strace-$(echo "$arm" | tr ':=/' '___')"; mkdir -p "$sdir"
    sdeaths=0
    for i in $(seq 1 "$strace_iterations"); do
      set +e
      env ${arm_env[@]+"${arm_env[@]}"} strace -f -qq -e trace=signal -e signal=all -o "$sdir/it-$i.txt" \
        timeout -k 5 "$iteration_timeout" "$work/bin/repro" "${args[@]}" >/dev/null 2>&1
      st=$?
      set -e
      [ "$st" -eq 0 ] || { sdeaths=$((sdeaths + 1)); echo "$st" >"$sdir/it-$i.status"; }
    done
    # Everything below reads FILES. `cat … | grep -m1` killed run 36003345966
    # (grep closed the pipe on cat; pipefail; exit 1) after the census had
    # already found what it was sent for.
    cat "$sdir"/it-*.txt >"$sdir/all.txt"
    echo "      strace ($strace_iterations iterations, $sdeaths died under it): signals delivered, by signal and si_code:"
    grep -oE -- '--- SIG[A-Z0-9_+]+ \{si_signo=[A-Z0-9_+]+, si_code=[A-Z_0-9]+' "$sdir/all.txt" \
      | sed -E 's/^--- ([A-Z0-9_+]+) \{si_signo=[A-Z0-9_+]+, si_code=([A-Z_0-9]+)/\1(\2)/' | sort | uniq -c | sort -rn \
      | awk '{printf "        %6d  %s\n", $1, $2}'
    grep -oE -- '--- SIGSEGV \{[^}]*\}' "$sdir/all.txt" >"$sdir/segv.txt" || true
    if [ -s "$sdir/segv.txt" ]; then
      echo "      SIGSEGV details (si_code, si_addr; distinct, first 8):"
      sed -E 's/.*si_code=([A-Z_0-9]+).*/\1 &/; s/^([A-Z_0-9]+) .*si_addr=(0x[0-9a-f]+).*/\1 \2/; s/^([A-Z_0-9]+) ---.*/\1 -/' "$sdir/segv.txt" \
        | sort | uniq -c | sort -rn | awk 'NR<=8 {printf "        %6d  %s %s\n", $1, $2, $3}'
      # THE FAULTING THREAD'S OWN SEQUENCE, per dying iteration. strace's
      # trace=signal also logs rt_sigreturn, so two deliveries to one LWP
      # with no rt_sigreturn between them mean the second fault happened
      # inside the first handler -- on whatever stack that handler was on.
      for f in "$sdir"/it-*.status; do
        [ -f "$f" ] || continue
        t="${f%.status}.txt"; it="${t##*/it-}"; it="${it%.txt}"
        lwp="$(grep -m1 -E -- '--- SIGSEGV' "$t" | awk '{print $1}')"
        [ -n "$lwp" ] || continue
        echo "      iteration $it died (status $(cat "$f")); LWP $lwp's last signal-related lines, and the exit:"
        grep -E "^$lwp " "$t" | grep -v '+++' | tail -14 | sed 's/^/        /'
        grep -E -- '\+\+\+ killed by|overflowed' "$t" | tail -2 | sed 's/^/        /'
      done
    else
      echo "      no SIGSEGV was delivered to any thread in $strace_iterations strace'd iterations"
    fi
  elif [ "$strace_iterations" -gt 0 ] && ! command -v strace >/dev/null 2>&1; then
    echo "      (strace not on PATH; the signal census is skipped)"
  fi
  kl_before="$( (dmesg 2>/dev/null || sudo -n dmesg 2>/dev/null) | grep -c 'overflowed sigaltstack' || true)"; kl_before="${kl_before:-0}"
  printf '  %-42s ' "$arm"
  for i in $(seq 1 "$iterations"); do
    # The baseline arm asks the runtime for its own dump on a fatal signal,
    # until one exists. Kernel cores are what every earlier trace read, and
    # the DAC will not open those.
    dumpenv=()
    if [ "$arm" = "${arms[0]}" ] && [ -z "$dump" ]; then
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
    if [ -z "$engine_line" ]; then
      engine_line="$(printf '%s\n' "$err" | grep -m1 '^\[repro-engine\]' || true)"
    fi
    word="$(classify "$st")"
    tally["$word"]=$(( ${tally["$word"]:-0} + 1 ))
    if [ "$word" = clean ]; then printf '.'; else
      bad=$((bad + 1)); printf 'X'
      if [ "$saved_arm" -lt 2 ] && [ -n "$err" ]; then
        saved=$((saved + 1)); saved_arm=$((saved_arm + 1))
        printf '%s' "$err" >"$work/death-$saved.txt"; echo "$arm" >"$work/death-$saved.arm"
      fi
      if [ -z "$dump" ]; then dump="$(ls -t "$work/dumps"/dump.* 2>/dev/null | head -1 || true)"; fi
      if [ -n "$core_arm" ] && [ "$arm" = "$core_arm" ] && ls "$core_dir"/core.* >/dev/null 2>&1; then ulimit -S -c 0 2>/dev/null || true; fi
    fi
  done
  b=""; for k in "${!tally[@]}"; do [ "$k" = clean ] && continue; b="$b $k=${tally[$k]}"; done
  echo "  -> $bad / $iterations${b:+  ($b )}"
  [ -n "$first_line" ] || first_line="[repro] (no iteration of this arm lived long enough to print its line)"
  echo "      ${first_line#\[repro\] }"
  [ -n "$engine_line" ] && echo "      engine: ${engine_line#\[repro-engine\] }"
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
      echo "      kernel: $kl 'overflowed sigaltstack' line(s) during this arm's $bad death(s)$( [ "$arm" = "${arms[0]}" ] && echo '; first:' )"
      if [ "$arm" = "${arms[0]}" ]; then (dmesg 2>/dev/null || sudo -n dmesg 2>/dev/null) >"$work/dmesg.txt" || true; grep -m1 'overflowed sigaltstack' "$work/dmesg.txt" | sed 's/^/        /'; fi
    else
      echo "      kernel: no 'overflowed sigaltstack' during this arm's $bad death(s) (silent push-fault, a genuine fault, or dmesg unreadable)"
    fi
  fi
  names+=("$arm"); totals+=("$bad"); breakdowns+=("${b:- none}"); touched+=("$first_line")
  unset tally
  if [ -n "$core_arm" ] && [ "$arm" = "$core_arm" ]; then
    echo
    echo "=== a residual death from arm '$arm', read from its core ==="
    core="$(ls -t "$core_dir"/core.* 2>/dev/null | head -1 || true)"
    if [ -z "$core" ]; then
      echo "  no core was written by this arm ($bad death(s)). A death the kernel delivers as SIG_DFL"
      echo "  after failing to build a signal frame writes no core either; check dmesg above."
    else
      "$here/../read-core.sh" "$core" "$work/bin/repro" "${out:-$work}" || echo "  (read-core.sh did not complete cleanly)"
    fi
  fi
done

if [ "$saved" -gt 0 ]; then
  echo
  echo "=== what the dying process said on stderr (first two per arm, verbatim) ==="
  for k in $(seq 1 "$saved"); do echo "--- death $k, arm $(cat "$work/death-$k.arm") ---"; sed 's/^/  /' "$work/death-$k.txt"; done
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
  set +o pipefail
  # The faulting thread's stack, trimmed; the whole thing is in the artefact.
  sed -n '/^OS Thread Id/,$p' "$work/analysis.txt" | grep -vE '^\s*$' | head -50 | sed 's/^/  /'
  set -o pipefail
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
  echo "  DW-398 REPRODUCES WITHOUT VSTEST: $base/$iterations on '${names[0]}', 0/$iterations without the engine."
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
