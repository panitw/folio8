#!/usr/bin/env bash
#
# CAP-5: does the fix hold under repetition on real silicon, in a harness that
# has been SHOWN ABLE TO SEE THE DEFECT?
#
#   ./soak.sh --reproduce                  # the validation leg: the PRE-FIX binding
#   ./soak.sh --iterations 100             # the soak: HEAD's binding
#   ./soak.sh --binding 86e7e5a -n 25      # any commit's binding
#   ./soak.sh --self-check                 # the tool's own judgement, any OS
#   ./soak.sh --help
#
# THE BAR IS NOT "THE SUITE PASSED", AND THAT IS THE WHOLE POINT. DW-396 records
# 0-crashes-in-11 being read as a clear, and the same shipped native overflowing
# sigaltstack afterwards. A harness that has never reproduced the defect cannot
# clear it, so this runner refuses to call a clean run a pass until a
# reproduction leg on THIS ARCHITECTURE is on its own ledger. The
# validated/unvalidated state is produced by the tool, never asserted by the
# operator.
#
# NO EMULATION, AND THE RUNNER DECIDES THAT ITSELF. Measured this epic: under
# Rosetta amd64 the pre-fix AND post-fix bindings both die, at a different test
# count every time, so a translated host cannot distinguish them in either
# direction. It is not slower evidence, it is no evidence. The translation
# verdict below is story 1's derivation (folio-dotnet/build/signal-stack-probe/
# Program.cs) re-expressed in shell: kernel machine against this process's own
# ELF e_machine, /proc/cpuinfo's vendor_id, the Rosetta marker, and a
# binfmt_misc interpreter registered for our own architecture. It does not take
# the operator's word for the architecture, because that is exactly what DW-396
# got wrong twice.
#
# amd64 AND arm64 ARE SOAKED SEPARATELY AND NEITHER STANDS IN FOR THE OTHER.
# The spike measured 16 KiB of altstack against 24 KiB for the same handler
# frame: arm64 was never clear, only wider-margined. The ledger is keyed by
# architecture for that reason, and a reproduction on one says nothing about the
# other.
#
# WATCH THE KERNEL, NOT ONLY THE EXIT CODE. `overflowed sigaltstack` in dmesg
# and `Internal CLR error (0x80131506)` are the NAMED signatures. A dead test
# host with NEITHER is the defect's commonest landing on an AVX2-class host
# (DW-398, 2026-09-24: the CLR's activation handler runs off its 16 KiB
# alternate stack and the push faults, with no message anywhere), and it is
# still not attributed by the fact of the death. On the reproduction leg
# such a death sends this tool to the mechanism reproducer
# (build/load-repro) on the SAME host and native: engine loaded against
# engine absent. That independent evidence attributes it, or refuses to.
# host with neither is reported as a crash of unknown cause -- never narrated as
# DW-396, which is the exact misattribution this epic exists to stop making.
#
# THE HOST IS CONTROLLED BEFORE THE BINDING IS. Validating that the harness can
# SEE the defect is only half the question; the other half is whether the host
# INVENTS crashes, and until 2026-09-23 this tool never asked it. On that day a
# WSL2 box crashed the .NET test host in 20 of 80 runs of a workload that never
# loads the native at all -- a higher rate than the engine legs on the same box
# -- and both soak legs were read as measuring the binding. Neither did, and a
# wrong conclusion was committed and pushed on the strength of it (DW-397). So
# every run now opens with a CONTROL LEG: the same iteration count, same host,
# same built suite, against a filter that never enters the engine. Any crash
# there and this tool refuses to produce a verdict at all -- because a figure
# from a host that crashes on its own is worth less than no figure, it looks
# like a measurement.
#
# THE CONTROL RUNS THE SAME NUMBER OF ITERATIONS AS THE SOAK, and that is not
# configurable. A control shorter than the run it certifies buys exactly the
# false clear this tool exists to refuse, one level up.
#
# IT IS NOT A CI GATE AND MUST NOT BECOME ONE. A soak long enough to mean
# anything does not belong on every commit, and a short one would recreate the
# false clear. One workflow invokes this file -- .github/workflows/soak.yml --
# and it is `workflow_dispatch` ONLY: no push, no pull_request, no schedule,
# and it is not a required check. It exists because CAP-5's evidence has to
# come from real hardware and the owner's WSL2 box could not provide it (see
# DW-396: six hypotheses tested and killed). If that workflow ever acquires a
# trigger that fires without a human asking, this sentence is the thing it
# contradicts, and the workflow is wrong rather than this comment.
#
# IT DOES NOT MODIFY src/Folio8. A non-HEAD binding comes from a git worktree,
# exactly as measure-throughput.sh's baseline leg does; there is no bypass
# switch in the shipped binding, and if this tool ever needs one, the tool is
# wrong.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# The commit that moved every crossing onto binding-owned engine threads. ITS
# PARENT IS THE PRE-FIX BINDING -- the one that enters the engine on arbitrary
# CLR thread-pool threads, and the one the reproduction leg must be able to
# kill. Same constant, same meaning, as measure-throughput.sh.
engine_threads_commit="${SOAK_ENGINE_COMMIT:-7f6a936}"

usage() {
  cat <<'USAGE'
soak.sh — CAP-5's soak, and the reproduction leg that makes it mean something

  ./soak.sh --reproduce                # validation leg: the PRE-FIX binding
  ./soak.sh                            # soak HEAD's binding
  ./soak.sh --binding <commit>         # soak any commit's binding
  ./soak.sh --self-check               # exercise this tool's judgement, any OS
  ./soak.sh --help

Options (environment variable in brackets; the flag wins):
  -n, --iterations N   how many times to run the suite
                       [SOAK_ITERATIONS] default 100, or 25 with --reproduce
  -b, --binding REF    'head' (default) or a commit whose binding to soak.
                       A commit is checked out as a git worktree; this tree's
                       native library is staged into it, so the ENGINE is held
                       fixed while the BINDING varies.  [SOAK_BINDING]
      --gc-gen0size HEX  gen0 budget handed to every test host as DOTNET_GCgen0size, so
                       the runtime collects often enough for the defect to have its
                       chance; '' disables it. [SOAK_GC_GEN0SIZE] default 0x100000
      --mechanism-iterations N
                       on the reproduction leg, when the pre-fix binding dies with no
                       named signature: iterations per arm for the mechanism reproducer
                       (build/load-repro, engine loaded vs absent) that attributes it.
                       [SOAK_MECHANISM_ITERATIONS] default 40
      --reproduce      the validation leg. Implies --binding 7f6a936^ unless
                       --binding was given, and inverts the verdict: catching
                       the defect is success, and a clean run is a failure to
                       validate the harness.
      --control-filter EXPR
                       the filter for the CONTROL leg, which must never enter
                       the engine  [SOAK_CONTROL_FILTER]. Default is
                       PackagingTests, DocsTests and SurfaceTests. It runs for
                       the same number of iterations as the soak, before it,
                       and any crash there refuses the whole run.
      --filter EXPR    the vstest filter for the render-heavy suite
                       [SOAK_FILTER] default GoldenTests and FontsTests, the
                       corpus renders that historically triggered it
      --native PATH    the native library to stage  [SOAK_NATIVE] default: the
                       shipped native for this RID, else build/native/host/
      --log-dir DIR    keep every iteration's log here  [SOAK_LOG_DIR]
                       Use it for any run whose result gets written down.
      --ledger FILE    where reproduction results are recorded  [SOAK_LEDGER]
                       default ${XDG_STATE_HOME:-~/.local/state}/folio-soak/ledger.tsv
      --context TEXT   free-text provenance stamped into the header
                       [SOAK_HOST_CONTEXT]
      --self-check     drive everything this tool decides over synthetic inputs
                       and exit: the translation verdict, the binfmt_misc
                       search, the ledger write-then-read round trip, the
                       reading of the two named signatures, the per-run
                       classifier and the final verdict. Runs on any OS; it
                       measures nothing and soaks nothing.

Exit status: 0 only for a soak that was clean AND validated, or a reproduction
that caught the defect. Everything else -- a refusal, a crash, a clean run from
an unvalidated harness -- is non-zero.
USAGE
}

# ---------------------------------------------------------------- arguments

mode="soak"
want_self_check=0
want_reproduce=0
binding="${SOAK_BINDING:-}"
if [ -n "$binding" ]; then binding_given_by_env=1; else binding_given_by_env=0; fi
iterations=""
filter="${SOAK_FILTER:-FullyQualifiedName~GoldenTests|FullyQualifiedName~FontsTests}"
# THE CONTROL FILTER MUST NOT LOAD THE NATIVE, AND THE FIRST ONE DID.
# It was `PackagingTests|DocsTests|SurfaceTests`, checked by reading
# /proc/<pid>/maps during a *DocsTests* run and generalised to all three --
# which was not a check, it was a sample. `PackagingTests` contains
# `TheRecordedEngineVersionMatchesTheEngine`, which asks the engine its
# version and therefore CROSSES THE ABI. Every control iteration was loading
# the Go library, and Go installs its SA_ONSTACK handlers process-wide at
# dlopen, so the "engine absent" leg was exposed to the very defect it existed
# to rule out. Proven by deleting the native and running the filter: that one
# test fails, the other 57 pass.
#
# The exclusion is verified at run time by `assert_control_is_engine_free`
# below rather than trusted, because this is the second filter that looked
# engine-free and was not.
control_filter="${SOAK_CONTROL_FILTER:-(FullyQualifiedName~PackagingTests|FullyQualifiedName~DocsTests|FullyQualifiedName~SurfaceTests)&FullyQualifiedName!~TheRecordedEngineVersionMatchesTheEngine}"
native="${SOAK_NATIVE:-}"
mech_iterations="${SOAK_MECHANISM_ITERATIONS:-40}"
# GC PRESSURE, FOR BOTH LEGS ALIKE. The defect fires on GC suspension -- the
# CLR signals every thread in managed code and the handler overflows -- so
# how often the suite collects is how often it can die. The corpus workload
# collects a handful of times an iteration, and run 35995188797 ran the
# pre-fix binding 150 iterations clean on a host where the mechanism
# reproducer dies 81 times in 100 with collections forced. A small gen0
# budget makes the runtime collect early and often, on the pre-fix leg and
# the soak leg alike, so the instrument provokes what it measures and the
# two legs stay comparable. Hex, as the runtime reads it; empty to disable.
gc_gen0size="${SOAK_GC_GEN0SIZE-0x100000}"
log_dir="${SOAK_LOG_DIR:-}"
ledger="${SOAK_LEDGER:-}"
host_context="${SOAK_HOST_CONTEXT:-}"
binding_given="$binding_given_by_env"

# VALIDATE BEFORE DOING ANY WORK -- nothing is built, no worktree is created and
# no iteration is run until the whole argument list is known good. A run that
# spends two minutes building and then complains about an argument has wasted
# exactly the thing it was asked to produce.
while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --self-check) want_self_check=1; shift ;;
    --reproduce) want_reproduce=1; shift ;;
    --mechanism-iterations) [ "$#" -ge 2 ] || { echo "soak: --mechanism-iterations needs a count" >&2; exit 1; }; mech_iterations="$2"; shift 2 ;;
    --gc-gen0size) [ "$#" -ge 2 ] || { echo "soak: --gc-gen0size needs a hex size, or '' to disable" >&2; exit 1; }; gc_gen0size="$2"; shift 2 ;;
    -n|--iterations) [ "$#" -ge 2 ] || { echo "soak: --iterations needs a number" >&2; exit 1; }; iterations="$2"; shift 2 ;;
    -b|--binding) [ "$#" -ge 2 ] || { echo "soak: --binding needs 'head' or a commit" >&2; exit 1; }; binding="$2"; binding_given=1; shift 2 ;;
    --filter) [ "$#" -ge 2 ] || { echo "soak: --filter needs an expression" >&2; exit 1; }; filter="$2"; shift 2 ;;
    --control-filter) [ "$#" -ge 2 ] || { echo "soak: --control-filter needs an expression" >&2; exit 1; }; control_filter="$2"; shift 2 ;;
    --native) [ "$#" -ge 2 ] || { echo "soak: --native needs a path" >&2; exit 1; }; native="$2"; shift 2 ;;
    --log-dir) [ "$#" -ge 2 ] || { echo "soak: --log-dir needs a directory" >&2; exit 1; }; log_dir="$2"; shift 2 ;;
    --ledger) [ "$#" -ge 2 ] || { echo "soak: --ledger needs a file path" >&2; exit 1; }; ledger="$2"; shift 2 ;;
    --context) [ "$#" -ge 2 ] || { echo "soak: --context needs some text" >&2; exit 1; }; host_context="$2"; shift 2 ;;
    *)
      echo "soak: unknown argument '$1' (see --help)" >&2
      exit 1
      ;;
  esac
done

# --reproduce and --self-check are different jobs, and a command line asking for
# both is asking for two things at once. Say so rather than silently picking
# whichever was parsed last.
if [ "$want_self_check" = "1" ] && [ "$want_reproduce" = "1" ]; then
  echo "soak: --self-check exercises this tool's own judgement and runs nothing; --reproduce runs the pre-fix binding against real hardware. Ask for one or the other." >&2
  exit 1
fi
if [ "$want_self_check" = "1" ]; then
  mode="self-check"
  if [ "$binding_given" = "1" ] || [ -n "$iterations" ]; then
    echo "soak: --self-check exercises this tool's own judgement and runs nothing; it takes no --binding and no --iterations" >&2
    exit 1
  fi
elif [ "$want_reproduce" = "1" ]; then
  mode="reproduce"
fi
# THE ENGINE CLOSES DW-398'S WINDOW ITSELF NOW (dispositions_linux.c), for
# every binding that loads it -- the pre-fix binding included. So the
# reproduction leg, whose whole job is to show this host can SEE the
# defect, runs its test hosts with FOLIO8_SIGNAL_DISPOSITIONS=leave: the
# engine honours it once, at load, leaves Go's edits in place, and reports
# mode=leave. The soak leg runs without it, as shipped. The mechanism check
# below sets it on its own baseline arm for the same reason.
# leave_desc is computed HERE, in a plain assignment, because the banner is an
# unquoted heredoc and an apostrophe inside ${var:-word} there opens a string
# that closes lines later -- run 35998599319 lost a soak to that, and run
# 36009787332 lost another to this very line before it was moved up here.
leave_env=""; leave_desc="none (the engine restores the host signal flags in its own constructor, as shipped)"
if [ "$mode" = "reproduce" ]; then leave_env="FOLIO8_SIGNAL_DISPOSITIONS=leave"; leave_desc="$leave_env on every test host of this leg"; fi

# THE WHOLE ARGUMENT LIST IS KNOWN GOOD BEFORE THE HOST IS EVEN LOOKED AT.
# `--iterations 0` must be refused as a bad argument wherever it is typed, not
# only on a host that got as far as caring.
case "$binding" in
  ""|head|HEAD)
    if [ "$mode" = "reproduce" ] && [ "$binding_given" = "0" ]; then
      binding="${engine_threads_commit}^"
    else
      binding="head"
    fi
    ;;
esac

if [ -z "$iterations" ]; then
  iterations="${SOAK_ITERATIONS:-}"
fi
if [ -z "$iterations" ]; then
  if [ "$mode" = "reproduce" ]; then iterations=25; else iterations=100; fi
fi
case "$iterations" in
  ''|*[!0-9]*)
    echo "soak: --iterations must be a whole number of one or more, not '$iterations'" >&2
    exit 1
    ;;
esac
if [ "$iterations" -lt 1 ]; then
  echo "soak: --iterations must be one or more, not '$iterations'" >&2
  exit 1
fi
# A RUN SHORT ENOUGH TO FINISH QUICKLY IS THE FALSE CLEAR, WRITTEN AGAIN.
# DW-396 records eleven clean runs being read as a clear. Eleven is not refused
# -- a deliberate short run is legitimate for trying the tool out -- but the
# number is never allowed to pass unremarked, IN EITHER MODE: a short
# reproduction leg that comes back clean is the more dangerous of the two,
# because "the pre-fix binding did not crash" is how a host gets written off as
# unable to reproduce when it was only asked three times.
short_run_warning=""
if [ "$iterations" -le 11 ]; then
  if [ "$mode" = "reproduce" ]; then
    short_run_warning="⚠ $iterations iterations is at or below the 11 that produced DW-396's false clear. If this leg comes back NOT REPRODUCED, that is a statement about the run length and not about the host."
  else
    short_run_warning="⚠ $iterations iterations is at or below the 11 that produced DW-396's false clear. Whatever comes out, it is not a soak."
  fi
fi

# =====================================================================
# THE PURE HALF: three derivations, no machine touched.
#
# Every piece of judgement this tool makes lives here, as a function of values
# it is handed, and --self-check drives each one over synthetic inputs. That
# split is story 1's, and it is there because the provenance verdict has a
# demonstrated defect history: its first cut in the probe called a Rosetta
# container NATIVE.
# =====================================================================

# --- 1. the translation verdict --------------------------------------------

# The machine strings a kernel reports, mapped to the architecture names this
# tool uses. Anything unrecognised is left UNMAPPED rather than guessed: an
# unknown pairing must not read as agreement.
arch_of_machine() {
  case "$1" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    i386|i486|i586|i686) echo "x86" ;;
    armv6l|armv7l) echo "arm" ;;
    *) echo "" ;;
  esac
}

# The vendor_id strings real x86 silicon reports, TRIMMED -- the kernel pads
# some of them to twelve characters and hands on a trimmed value, so padded
# entries here could never match and Zhaoxin and VIA silicon would be stamped
# SUSPECT for it. The probe's list exactly, INCLUDING the two that contain
# spaces: a space-separated lookup cannot represent "VIA VIA VIA" or "Geode by
# NSC" at all, so real VIA and Geode silicon would be SUSPECT and, here, HARD
# REFUSED with no override. One entry per line, matched whole.
known_x86_vendors="GenuineIntel
AuthenticAMD
HygonGenuine
CentaurHauls
Shanghai
VIA VIA VIA
GenuineTMx86
Geode by NSC"

is_known_x86_vendor() {
  printf '%s\n' "$known_x86_vendors" | grep -Fxq -- "$1"
}

# The ELF e_machine of a file, as an architecture name. It is how this tool
# reads its OWN architecture (/proc/self/exe, the shell's answer to the probe's
# RuntimeInformation.ProcessArchitecture) and how it checks that the native
# library it is about to stage is the architecture this host can run at all.
# Both callers want the same three lines, and a tool that reads its own
# e_machine rather than trust the operator should not then trust the operator
# about the .so.
elf_arch_of() {
  local f="$1" magic m
  [ -r "$f" ] || { echo "-"; return; }
  magic="$(od -An -c -N4 "$f" 2>/dev/null | tr -d ' ')" || true
  case "$magic" in
    177ELF*) ;;
    *) echo "-"; return ;;
  esac
  m="$(od -An -tu2 -j18 -N2 "$f" 2>/dev/null | tr -d ' ')" || true
  case "$m" in
    62) echo "amd64" ;;
    183) echo "arm64" ;;
    3) echo "x86" ;;
    40) echo "arm" ;;
    *) echo "-" ;;
  esac
}

# The qemu interpreter basenames that would run OUR OWN architecture on some
# other one. Matched against the `interpreter` line of every registration, not
# against the registration's NAME: a registration may be called anything at all
# ("qemu-x86_64-static", "cross-amd64", a distro's own name), and a probe that
# stats one hard-coded filename degrades OPEN -- it finds nothing, SUSPECT never
# fires, and a qemu host is soaked as native. Every other gathering failure in
# this tool degrades into UNKNOWN, which refuses; this one must not be the
# exception.
qemu_interpreter_pattern() {
  case "$1" in
    amd64) echo '(x86[_-]?64|amd64)' ;;
    arm64) echo '(aarch64|arm64)' ;;
    x86) echo '(i[3456]86|x86)' ;;
    arm) echo '(armh?f?|armel)' ;;
    *) echo '' ;;
  esac
}

# Does this interpreter run OUR architecture under emulation? Two shapes are
# accepted, because both are what real hosts register:
#   /usr/bin/qemu-aarch64-static                  -- the basename names qemu
#   /usr/libexec/qemu-binfmt/x86_64-binfmt-P      -- Debian's wrapper; the
#                                                    basename does NOT
# So the test is "somewhere on the path it says qemu, and it names our
# architecture". Requiring the token ALONE would match any interpreter with
# 'amd64' in its path; requiring the 'qemu-<arch>' basename alone missed the
# second shape entirely, which is how this probe came to degrade open.
interpreter_runs_arch() {
  local interp="$1" pattern
  pattern="$(qemu_interpreter_pattern "$2")"
  [ -n "$pattern" ] || return 1
  printf '%s' "$interp" | grep -Eq -- 'qemu' || return 1
  printf '%s' "$interp" | grep -Eq -- "$pattern"
}

# binfmt_hits_in <binfmt-dir> <our-arch>
# Prints one "name (interpreter, state)" line per registration whose interpreter
# would run our own architecture. Driven over a synthetic directory by
# --self-check, because the real one cannot be written to on a developer box.
binfmt_hits_in() {
  local dir="$1" self="$2" pattern entry name interp state
  pattern="$(qemu_interpreter_pattern "$self")"
  [ -n "$pattern" ] || return 0
  [ -d "$dir" ] || return 0
  for entry in "$dir"/*; do
    [ -f "$entry" ] || continue
    name="$(basename "$entry")"
    case "$name" in
      status|register) continue ;;
    esac
    interp="$(sed -n 's/^interpreter[ \t]*//p' "$entry" 2>/dev/null | head -n 1 | tr -d '\r')" || true
    # A registration whose interpreter line cannot be read still counts if its
    # own NAME names our architecture -- that is the hard-coded check this
    # replaces, kept as the fallback rather than as the whole test.
    if [ -z "$interp" ]; then
      if printf '%s' "$name" | grep -Eq -- "^qemu" && printf '%s' "$name" | grep -Eq -- "$pattern"; then
        echo "$name (interpreter unreadable)"
      fi
      continue
    fi
    if interpreter_runs_arch "$interp" "$self"; then
      state="$(head -n 1 "$entry" 2>/dev/null | tr -d '\r')"
      [ -n "$state" ] || state="registered"
      echo "$name ($interp, $state)"
    fi
  done
}

# --- the ledger: the one path that can turn a clean soak into a pass -------

# WRITTEN HERE AND READ HERE, so --self-check can drive the ROUND TRIP rather
# than each half separately. Nothing else in this tool can promote a clean run
# to a pass, and before these were factored out its first execution anywhere
# would have been the owner's hardware leg.
#
# WHAT A LEDGER ROW MEANS, EXACTLY: on THIS architecture, against THIS native
# library, a pre-fix binding was seen to die with a named signature. The read
# below matches on both, so a reproduction taken against a different .so does
# not silently validate today's soak -- in a tool whose whole thesis is that a
# figure without its provenance is worth nothing. Host and kernel are recorded
# and reported but not matched on: they are what a reader needs to judge the
# row, and a kernel upgrade should weaken a claim visibly rather than erase it.
ledger_append() {
  local file="$1" arch_="$2" host_="$3" kernel_="$4" binding_="$5" sha_="$6"
  mkdir -p "$(dirname "$file")" 2>/dev/null || true
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$arch_" "REPRODUCED" "$host_" \
    "$kernel_" "$binding_" "$sha_" >>"$file" 2>/dev/null
}

# ledger_lookup <file> <arch> <native-sha>
# Prints the most recent matching row, or nothing.
ledger_lookup() {
  local file="$1" arch_="$2" sha_="$3"
  [ -f "$file" ] || return 0
  awk -F'\t' -v a="$arch_" -v s="$sha_" \
    '$2 == a && $3 == "REPRODUCED" && $7 == s {line = $0} END {if (line != "") print line}' \
    "$file" 2>/dev/null || true
}

# The near miss: a reproduction on this architecture against a DIFFERENT native.
# It does not validate anything, and saying so is worth more than silence --
# without it the operator sees UNVALIDATED after running the reproduction leg
# and has no idea which of the two runs to change.
ledger_lookup_other_native() {
  local file="$1" arch_="$2" sha_="$3"
  [ -f "$file" ] || return 0
  awk -F'\t' -v a="$arch_" -v s="$sha_" \
    '$2 == a && $3 == "REPRODUCED" && $7 != s {line = $0} END {if (line != "") print line}' \
    "$file" 2>/dev/null || true
}

join_reason() {
  if [ -z "$1" ]; then printf '%s' "$2"; else printf '%s; %s' "$1" "$2"; fi
}

# Join the lines on stdin with a MULTI-CHARACTER separator. `tr '\n' ' | '`
# looks like it does this and does not: tr truncates set2 to one character, so
# the kernel's own words end up space-joined in the one place the owner is meant
# to copy them from -- and it only shows on a real crash, the run nobody gets to
# redo.
join_with() {
  awk -v sep="$1" 'NR > 1 { printf "%s", sep } { printf "%s", $0 } END { if (NR > 0) printf "\n" }'
}

# derive_translation <kernel-machine> <process-arch> <cpuinfo-readable yes|no> \
#                    <vendor|-> <implementer|-> <rosetta yes|no> <binfmt-hits|->
#
# Sets VERDICT_TAG (native|SUSPECT|UNKNOWN|XLATED) and VERDICT_SUMMARY.
derive_translation() {
  # `implementer` is part of the SIGNAL SET and deliberately not part of the
  # DERIVATION, exactly as in the probe: the arm side is established by the
  # ABSENCE of an x86 vendor_id, not by the presence of a CPU implementer line,
  # because plenty of legitimate arm64 kernels omit that key and a rule keyed on
  # it would refuse them. It is passed here so the two signal sets stay the same
  # shape, and it is reported beside the verdict.
  # shellcheck disable=SC2034  # carried for the signal set; see above
  local machine="$1" self="$2" cpuinfo="$3" vendor="$4" implementer="$5" rosetta="$6" binfmt="$7"
  local how="" unsure="" translated=0 suspect=0
  local kernel_arch
  kernel_arch="$(arch_of_machine "$machine")"

  # AN UNKNOWN VALUE ON EITHER SIDE IS NOT A MISMATCH. If this process's own
  # architecture could not be read, "kernel is x86_64, this process is -" is a
  # gap in the reading, not a translated host; it falls through to UNKNOWN
  # below. Reporting it as TRANSLATED would be a claim about hardware derived
  # from a failure to read a file.
  if [ "$machine" != "-" ] && [ "$self" != "-" ] && [ -n "$kernel_arch" ] && [ "$kernel_arch" != "$self" ]; then
    translated=1
    how="$(join_reason "$how" "kernel is $machine, this process is $self")"
  fi

  local x86_process=0 arm_process=0
  case "$self" in
    amd64|x86) x86_process=1 ;;
    arm64|arm) arm_process=1 ;;
  esac

  if [ "$cpuinfo" != "yes" ]; then
    # "could not be read" is NOT "the file has no such key". Only the second
    # says anything about the CPU, and a host with procfs masked must not be
    # told its CPU "is not x86 at all" on the strength of a permissions error.
    unsure="$(join_reason "$unsure" "/proc/cpuinfo could not be read, so the CPU underneath was not checked")"
  elif [ "$x86_process" = "1" ] && [ "$vendor" = "VirtualApple" ]; then
    translated=1
    how="$(join_reason "$how" "cpuinfo vendor_id is VirtualApple — Apple Silicon running x86 under Rosetta")"
  elif [ "$x86_process" = "1" ] && [ "$vendor" = "-" ]; then
    translated=1
    how="$(join_reason "$how" "cpuinfo carries no vendor_id, so the CPU underneath is not x86 at all")"
  elif [ "$x86_process" = "1" ] && ! is_known_x86_vendor "$vendor"; then
    suspect=1
    how="$(join_reason "$how" "cpuinfo vendor_id '$vendor' is not a vendor real x86 silicon reports")"
  elif [ "$arm_process" = "1" ] && [ "$vendor" != "-" ]; then
    translated=1
    how="$(join_reason "$how" "cpuinfo is x86-shaped (vendor_id $vendor) while this process is $self")"
  fi

  if [ "$rosetta" = "yes" ]; then
    translated=1
    how="$(join_reason "$how" "a Rosetta marker is present")"
  fi

  # Registration is not proof of use -- a real amd64 box with an arm64
  # cross-build setup has qemu-aarch64 registered and is perfectly native -- so
  # only an interpreter for OUR OWN architecture counts, and even that is
  # SUSPECT rather than translation.
  if [ "$binfmt" != "-" ]; then
    suspect=1
    how="$(join_reason "$how" "a binfmt_misc interpreter for this very architecture is registered ($binfmt)")"
  fi

  if [ "$machine" = "-" ]; then
    unsure="$(join_reason "$unsure" "uname gave no machine, so the kernel's architecture could not be compared with this process's")"
  elif [ -z "$kernel_arch" ]; then
    unsure="$(join_reason "$unsure" "uname reports the machine as '$machine', which this tool does not recognise, so it could not be compared with $self")"
  fi
  if [ "$self" = "-" ]; then
    unsure="$(join_reason "$unsure" "this process's own architecture could not be read, so nothing could be compared with the kernel's")"
  fi

  if [ "$translated" = "1" ]; then
    VERDICT_TAG="XLATED"
    VERDICT_SUMMARY="TRANSLATED ($how)"
  elif [ "$suspect" = "1" ]; then
    VERDICT_TAG="SUSPECT"
    VERDICT_SUMMARY="SUSPECT ($how). Nothing proves this host is translated, and nothing proves it is not."
  elif [ -n "$unsure" ]; then
    VERDICT_TAG="UNKNOWN"
    VERDICT_SUMMARY="UNKNOWN — $unsure."
  else
    VERDICT_TAG="native"
    VERDICT_SUMMARY="NATIVE — kernel and process agree on $machine, /proc/cpuinfo is consistent with $self, no Rosetta marker, no matching binfmt_misc interpreter."
  fi
}

# --- 2. reading the two named signatures out of a run ----------------------

# THESE THREE READ TEXT AND NOTHING ELSE, so --self-check can drive them over
# the exact strings vstest and the kernel print. Getting the CLASSIFIER right
# over a set of flags is worth nothing if the flags themselves are computed by
# a grep nobody ever ran against a real crash, and no host this tool is
# developed on can produce one.

# A DEAD TEST HOST DOES NOT KILL THIS SCRIPT, AND THAT IS THE SHAPE DW-396
# ACTUALLY TAKES. vstest survives its own child and reports the death in words,
# exiting 1 -- indistinguishable by status from a failed assertion. So the log
# is read for the death itself, separately from the signatures that would
# explain it.
log_says_host_died() {
  if grep -q -e 'Test host process crashed' -e 'The active test run was aborted' "$1" 2>/dev/null; then
    echo yes
  else
    echo no
  fi
}

# `Internal CLR error. (0x80131506)` -- COR_E_EXECUTIONENGINE, the CLR's
# execution engine reporting its own state as unrecoverable. It is one of the
# two signatures DW-396 names; both spellings are accepted because the numeric
# code and the words have each appeared alone in this entry's history.
log_says_clr_error() {
  if grep -q -e '0x80131506' -e 'Internal CLR error' "$1" 2>/dev/null; then
    echo yes
  else
    echo no
  fi
}

# A FILTER THAT MATCHES NOTHING IS A CLEAN RUN OF NOTHING, and `dotnet test`
# exits 0 for it. Measured while verifying this tool: a mistyped --filter gave
# three "clean" iterations that never crossed the ABI once -- the false clear in
# its purest form, a tally of runs in which the defect had no opportunity to
# fire.
#
# IT IS READ AS THE ABSENCE OF A POSITIVE COUNT, not as the presence of a
# message, because at `-v quiet` .NET 10 prints NOTHING AT ALL for an empty
# filter -- no "No test matches", no summary line. The named messages are kept
# as additional positives for the runners that do print them.
#
# THE CALLER ONLY ASKS THIS OF AN OTHERWISE-CLEAN ITERATION. A log from a dead
# test host has no summary line either, and that is a crash, not an empty
# filter.
log_says_no_tests_ran() {
  if grep -q -e 'No test matches the given testcase filter' -e 'No test is available' "$1" 2>/dev/null; then
    echo yes
  elif grep -Eq 'Passed:[[:space:]]*[1-9][0-9]*' "$1" 2>/dev/null; then
    echo no
  else
    echo yes
  fi
}

# Only lines the kernel added DURING one iteration, and only the two it prints
# for this defect. A broader grep would sweep in every unrelated segfault on the
# box and turn a busy machine into a reproduction; a count rather than the lines
# themselves would leave the report with nothing to quote.
kernel_lines_since() {
  diff "$1" "$2" 2>/dev/null \
    | sed -n 's/^> //p' \
    | grep -e 'overflowed sigaltstack' -e 'unexpected fatal signal' || true
}

# The kernel's half of the pair: `signal: .NET TP Worker[31774] overflowed
# sigaltstack`. `unexpected fatal signal` is collected beside it for the report
# but is NOT the signature -- it is what any fatal signal prints.
lines_say_overflow() {
  if printf '%s' "$1" | grep -q 'overflowed sigaltstack'; then echo yes; else echo no; fi
}

# --- 2b. is the HOST sound enough for any figure to mean anything? ---------

# The control leg's judgement, as a function of what it saw, so --self-check
# can drive it without a machine.
#
# ONE CRASH IS ENOUGH TO REFUSE. Not a rate, not a threshold: a host that can
# kill a test host on a workload with the engine absent can kill it on a
# workload with the engine present, and nothing downstream can tell the two
# apart. DW-397 exists because a 25% background rate was read as a 4-7% defect
# rate, and a threshold would have let a 3% one through on the same reasoning.
#
# A CONTROL THAT RAN NOTHING IS NOT A CLEAN CONTROL. An empty filter exits 0
# and proves nothing, which is the same hole `log_says_no_tests_ran` closes one
# level down.
control_verdict() {
  local crashes="$1" completed="$2" ran_tests="$3"
  if [ "$ran_tests" != "yes" ]; then
    CONTROL_TAG="EMPTY"
    CONTROL_TEXT="the control leg matched no tests, so it certified nothing. Check --control-filter."
    return
  fi
  if [ "$crashes" -gt 0 ]; then
    CONTROL_TAG="UNSOUND"
    CONTROL_TEXT="the host crashed the test host $crashes time(s) in $completed iterations of a workload that NEVER LOADS THE NATIVE. No soak figure from this host is admissible in either direction, and none is produced."
    return
  fi
  CONTROL_TAG="SOUND"
  CONTROL_TEXT="$completed control iterations with the engine absent, no crash. The host is not manufacturing the deaths the soak leg will or will not see."
}

# --- 3. what one iteration's outcome was -----------------------------------

# HOW A RUN DIED IS NOT ONE QUESTION, AND ANSWERING IT AS ONE IS THE EXACT
# MISATTRIBUTION THIS EPIC EXISTS TO PREVENT.
#
#   CLEAN          the suite ran and passed.
#   DW396          the test host died AND a NAMED signature was seen: the
#                  kernel's `overflowed sigaltstack`, or the CLR's
#                  `Internal CLR error (0x80131506)`.
#   CRASH-UNKNOWN  the test host died and NEITHER signature was present. It is
#                  a crash, and it is not evidence about DW-396.
#   SUITE-FAILED   a non-zero exit with no dead host: an assertion, a bad
#                  argument, a missing fixture. Not a crash, and it says
#                  nothing about the defect.
#
# A fifth outcome, KERNEL-OVERFLOW-ONLY, exists at the level of the WHOLE RUN
# rather than of one iteration: the kernel printed `overflowed sigaltstack`
# while every iteration passed. Per-iteration that really is CLEAN -- nothing
# died -- but over the run it is the named signature, and DW-396's own history
# is a kernel message being present while the run looked fine. It is raised
# after the loop, never by this function.
#
# classify_run <exit-status> <host-died yes|no> <clr-signature yes|no> <kernel-signature yes|no>
# Sets RUN_OUTCOME and RUN_DETAIL.
classify_run() {
  local status="$1" died="$2" clr="$3" kernel="$4"
  local signal=""
  if [ "$status" -ge 128 ] 2>/dev/null; then signal="$((status - 128))"; fi

  if [ "$status" = "0" ] && [ "$died" = "no" ]; then
    RUN_OUTCOME="CLEAN"
    RUN_DETAIL="the suite ran to completion"
    return
  fi

  if [ "$died" = "yes" ] || [ -n "$signal" ]; then
    local how="the test host process died"
    if [ -n "$signal" ]; then how="the runner was killed by signal $signal (exit $status)"; fi
    if [ "$kernel" = "yes" ] || [ "$clr" = "yes" ]; then
      RUN_OUTCOME="DW396"
      local sigs=""
      if [ "$kernel" = "yes" ]; then sigs="kernel: 'overflowed sigaltstack'"; fi
      if [ "$clr" = "yes" ]; then sigs="$(join_reason "$sigs" "CLR: 'Internal CLR error (0x80131506)'")"; fi
      RUN_DETAIL="$how — DW-396 signature present [$sigs]"
    else
      RUN_OUTCOME="CRASH-UNKNOWN"
      RUN_DETAIL="$how — NEITHER named signature was present, so this is an unexplained crash and is NOT attributed to DW-396"
    fi
    return
  fi

  RUN_OUTCOME="SUITE-FAILED"
  RUN_DETAIL="the suite exited $status with no dead host: an assertion, a refusal or a setup fault, not a crash"
}

# --- 3b. the mechanism check --------------------------------------------------
#
# WHY A DEATH WITHOUT A SIGNATURE CAN STILL BE ATTRIBUTED, AND BY WHAT. On
# 2026-09-24 DW-398 named the cause behind DW-396: after the engine loads, Go
# re-flags the CLR's own SIGRTMIN handler with SA_ONSTACK, the CLR's GC
# activation handler then runs on the CLR's 16 KiB alternate stack, and it
# overflows. On an AVX-512 host the kernel's own signal frame is large enough
# that the NEXT signal cannot be delivered and the kernel logs `overflowed
# sigaltstack` -- the named signature. On an AVX2 host the frame is smaller,
# the handler itself runs off the end, and the death is a bare SIGSEGV with
# no message anywhere. Same defect, no signature, and this tool was right to
# refuse to attribute it on the death alone.
#
# The attribution comes from build/load-repro instead: a console process
# that holds nothing but pool threads collecting garbage, run with the engine
# loaded and, identically, without. On a host where the mechanism is live
# the loaded arm dies at 55-98% and the control at 0. That is evidence about
# THIS host and THIS native, independent of the binding's death, and it is
# what the reproduction leg now consults when the death carried no name.
#
# mechanism_parse <log>       reads the reproducer's RESULT block into MECH_LOAD,
#                             MECH_CONTROL, MECH_N; returns 1 if there is none.
# mechanism_verdict           derives MECH_TAG from those three.
# mechanism_check             runs the reproducer and does both.
MECH_LOAD=""; MECH_CONTROL=""; MECH_N=""; MECH_TAG="NOT-RUN"; MECH_NOTE=""; MECH_LOG=""

mechanism_parse() {
  local file="$1"
  MECH_LOAD=""; MECH_CONTROL=""; MECH_N=""
  [ -f "$file" ] || return 1
  # Only the RESULT block: the progress lines above it also begin with the
  # arm's name, and they carry the dots.
  local parsed
  parsed="$(awk '
    /^=== RESULT ===/ { in_result = 1; next }
    in_result && $1 == "load"   && $3 == "/" { l = $2; n = $4 }
    in_result && $1 == "noload" && $3 == "/" { c = $2 }
    END { if (l != "" && c != "" && n != "") print l, c, n }' "$file")"
  [ -n "$parsed" ] || return 1
  MECH_LOAD="${parsed%% *}"; parsed="${parsed#* }"
  MECH_CONTROL="${parsed%% *}"; MECH_N="${parsed#* }"
  return 0
}

mechanism_verdict() {
  if [ -z "$MECH_N" ]; then MECH_TAG="NOT-RUN"; return; fi
  if [ "$MECH_CONTROL" -gt 0 ] 2>/dev/null; then MECH_TAG="HOST-MANUFACTURES"; return; fi
  if [ "$MECH_LOAD" -eq 0 ] 2>/dev/null; then MECH_TAG="DID-NOT-FIRE"; return; fi
  MECH_TAG="ATTRIBUTED"
}

mechanism_check() {
  MECH_LOAD=""; MECH_CONTROL=""; MECH_N=""; MECH_TAG="NOT-RUN"; MECH_NOTE=""
  MECH_LOG="$logs/mechanism.log"
  local tool="$here/load-repro/run.sh"
  if [ ! -x "$tool" ]; then MECH_NOTE="the mechanism reproducer is not at $tool"; return 0; fi
  echo "==> the pre-fix binding died with no named signature. Asking the mechanism reproducer"
  echo "    whether the defect is live on THIS host with THIS native: $mech_iterations iterations per arm,"
  echo "    engine loaded against engine absent."
  # Its exit status is not the answer -- it exits non-zero on a clean baseline
  # too -- the numbers are.
  FOLIO8_SIGNAL_DISPOSITIONS=leave "$tool" --arms load,noload -n "$mech_iterations" --native "$native" >"$MECH_LOG" 2>&1 || true
  if ! mechanism_parse "$MECH_LOG"; then
    MECH_NOTE="its RESULT block could not be read; see $MECH_LOG"
    return 0
  fi
  mechanism_verdict
  echo "    loaded: $MECH_LOAD/$MECH_N died   absent: $MECH_CONTROL/$MECH_N died   -> $MECH_TAG"
  echo "    log: $MECH_LOG"
}

# --- 4. the verdict over the whole run -------------------------------------

# final_verdict <mode soak|reproduce> <iterations-completed> <outcome-of-last-run> <validated yes|no> <kernel-signal-available yes|no>
# Sets FINAL_TAG, FINAL_TEXT and FINAL_STATUS (the process exit code).
final_verdict() {
  local mode="$1" completed="$2" outcome="$3" validated="$4" dmesg_ok="$5"

  # STOPPED BY HAND IS NOT A RESULT, IN EITHER MODE. The owner sits and watches
  # this one, so Ctrl-C at iteration 60 of 100 is the likeliest real ending of
  # any long run; without this it would fall through to the clean branch and be
  # read as 60 clean iterations, which is a tally nobody chose to stop at.
  if [ "$outcome" = "INTERRUPTED" ]; then
    FINAL_TAG="STOPPED — PARTIAL RUN"
    FINAL_TEXT="interrupted after $completed complete iteration(s), none of which crashed. THIS IS NOT A RESULT: nothing is validated, nothing is written to the ledger, and the count above is where somebody pressed Ctrl-C rather than where the run ended. Re-run it whole."
    FINAL_STATUS=130
    return
  fi

  if [ "$mode" = "reproduce" ]; then
    case "$outcome" in
      DW396)
        FINAL_TAG="REPRODUCED"
        FINAL_TEXT="the pre-fix binding died with a named DW-396 signature on iteration $completed. THIS HOST'S HARNESS IS NOW VALIDATED for this architecture and this native library: it has been shown able to see the defect, which is what makes a later clean run worth anything."
        FINAL_STATUS=0
        ;;
      KERNEL-OVERFLOW-ONLY)
        FINAL_TAG="REPRODUCED — KERNEL SIGNATURE, HOST SURVIVED"
        FINAL_TEXT="over $completed iterations the pre-fix binding never died, but the kernel printed 'overflowed sigaltstack' during the run. That is the named signature, so the harness HAS been shown able to see the defect and is validated for this architecture and native. The survival is itself worth recording: the overflow's consequences depend on how deep the stack was when the signal landed."
        FINAL_STATUS=0
        ;;
      CRASH-UNKNOWN)
        case "$MECH_TAG" in
          ATTRIBUTED)
            FINAL_TAG="REPRODUCED — BY MECHANISM"
            FINAL_TEXT="the pre-fix binding died on iteration $completed with NEITHER named signature — and on THIS host, against THIS native, the mechanism reproducer (build/load-repro) died $MECH_LOAD/$MECH_N with the engine loaded and $MECH_CONTROL/$MECH_N without. The death is attributed to DW-396/DW-398 by that independent evidence — the CLR's GC activation handler overflowing the alternate stack Go moved it onto — and not by the fact of the death. THIS HOST'S HARNESS IS NOW VALIDATED for this architecture and this native library."
            FINAL_STATUS=0
            ;;
          HOST-MANUFACTURES)
            FINAL_TAG="NOT REPRODUCED — HOST MANUFACTURES DEATHS"
            FINAL_TEXT="the pre-fix binding died on iteration $completed with NEITHER named signature, and the mechanism reproducer's CONTROL — the engine never loaded — died $MECH_CONTROL/$MECH_N on this host. A host that kills a process which never touched the engine can attribute nothing to it (DW-397). Nothing is validated here; find out what is killing the control first."
            FINAL_STATUS=1
            ;;
          DID-NOT-FIRE)
            FINAL_TAG="NOT REPRODUCED — UNEXPLAINED CRASH"
            FINAL_TEXT="the pre-fix binding died on iteration $completed with NEITHER named signature, and the mechanism reproducer did NOT fire on this host: $MECH_LOAD/$MECH_N with the engine loaded, $MECH_CONTROL/$MECH_N without. The binding's death is therefore something else, and the harness stays unvalidated. Read the iteration's log before running anything else."
            FINAL_STATUS=1
            ;;
          *)
            FINAL_TAG="NOT REPRODUCED — UNEXPLAINED CRASH"
            FINAL_TEXT="the pre-fix binding died on iteration $completed with NEITHER named signature. A crash of unknown cause is not a reproduction of DW-396, and the harness stays unvalidated. Read the iteration's log before running anything else.${MECH_NOTE:+ (The mechanism reproducer could not attribute it: $MECH_NOTE.)}"
            FINAL_STATUS=1
            ;;
        esac
        ;;
      SUITE-FAILED)
        FINAL_TAG="INCONCLUSIVE"
        FINAL_TEXT="the suite itself failed on iteration $completed without a dead host, so the reproduction leg never got a clean look at the defect. Fix the suite failure and run this again."
        FINAL_STATUS=1
        ;;
      *)
        case "$MECH_TAG" in
          ATTRIBUTED)
            FINAL_TAG="NOT REPRODUCED — MECHANISM LIVE, WORKLOAD TOO LIGHT"
            FINAL_TEXT="$completed iterations of the PRE-FIX binding on this host did not produce the defect — yet the mechanism reproducer, on this host against this native, died $MECH_LOAD/$MECH_N with the engine loaded and $MECH_CONTROL/$MECH_N without. The defect is live here; this workload did not collect often enough to meet it. THE HARNESS IS NOT VALIDATED HERE by a clean pre-fix run. Raise the GC pressure (--gc-gen0size) or --iterations, and run this leg again."
            ;;
          HOST-MANUFACTURES)
            FINAL_TAG="NOT REPRODUCED — HOST MANUFACTURES DEATHS"
            FINAL_TEXT="$completed iterations of the PRE-FIX binding stayed clean, and the mechanism reproducer's CONTROL — the engine never loaded — died $MECH_CONTROL/$MECH_N on this host. Nothing on this host can be attributed to the engine (DW-397). Find out what kills the control first."
            ;;
          DID-NOT-FIRE)
            FINAL_TAG="NOT REPRODUCED — MECHANISM NOT LIVE HERE"
            FINAL_TEXT="$completed iterations of the PRE-FIX binding stayed clean, and the mechanism reproducer did not fire either: $MECH_LOAD/$MECH_N with the engine loaded, $MECH_CONTROL/$MECH_N without. This host cannot see the defect and cannot validate a harness for it. Take this leg on a host where the reproducer fires."
            ;;
          *)
            FINAL_TAG="NOT REPRODUCED"
            FINAL_TEXT="$completed iterations of the PRE-FIX binding on this host did not produce the defect. THE HARNESS IS NOT VALIDATED HERE, and a clean soak on this host would therefore prove nothing. Either raise --iterations, or take this leg on a host where the defect is known to fire.${MECH_NOTE:+ (The mechanism reproducer could not say more: $MECH_NOTE.)}"
            ;;
        esac
        FINAL_STATUS=1
        ;;
    esac
    return
  fi

  case "$outcome" in
    DW396)
      FINAL_TAG="FAILED — DW-396"
      FINAL_TEXT="the binding under soak died on iteration $completed with a named DW-396 signature. The fix does not hold on this host."
      FINAL_STATUS=1
      ;;
    KERNEL-OVERFLOW-ONLY)
      FINAL_TAG="FAILED — DW-396 SIGNATURE, HOST SURVIVED"
      FINAL_TEXT="every one of $completed iterations passed, AND the kernel printed 'overflowed sigaltstack' during the run. The suite looking fine is exactly how DW-396 presented the first time; the signature is the finding, not the exit code. This is not a clean soak."
      FINAL_STATUS=1
      ;;
    CRASH-UNKNOWN)
      FINAL_TAG="FAILED — UNEXPLAINED CRASH"
      FINAL_TEXT="the binding under soak died on iteration $completed and NEITHER named signature was present. It is a crash and it is reported as one; nothing here attributes it to DW-396."
      FINAL_STATUS=1
      ;;
    SUITE-FAILED)
      FINAL_TAG="INCONCLUSIVE"
      FINAL_TEXT="the suite failed on iteration $completed without a dead host. That is a test failure, not a soak result; nothing is claimed either way."
      FINAL_STATUS=1
      ;;
    *)
      if [ "$validated" = "yes" ]; then
        FINAL_TAG="CLEAN — VALIDATED HARNESS"
        FINAL_TEXT="$completed iterations, no crash, on a harness that has reproduced the defect on this architecture (see the ledger line above). This is the only shape of clean run this tool will call a pass."
        FINAL_STATUS=0
      else
        FINAL_TAG="UNVALIDATED"
        FINAL_TEXT="$completed iterations came back clean, and THAT IS NOT A PASS. No reproduction leg has run on this architecture, so this harness has never been shown able to see the defect — which is precisely the inference DW-396 records being made wrongly, at 0-crashes-in-11. Run './soak.sh --reproduce' on this architecture first."
        FINAL_STATUS=1
      fi
      ;;
  esac

  if [ "$dmesg_ok" != "yes" ]; then
    FINAL_TEXT="$FINAL_TEXT

  ⚠ ONE SIGNAL WAS MISSING FOR THE WHOLE RUN: dmesg could not be read, so the
    kernel's 'overflowed sigaltstack' line could not be watched for. Only the
    CLR-side signature was available. The verdict above stands on less than it
    normally would; say so wherever it is recorded."
  fi
}

# =====================================================================
# --self-check: drive all three derivations over synthetic inputs.
# =====================================================================

if [ "$mode" = "self-check" ]; then
  checks=0
  bad=0
  expect() {
    local label="$1" expected="$2" actual="$3"
    checks=$((checks + 1))
    if [ "$expected" = "$actual" ]; then
      printf '  ok   %-58s %s\n' "$label" "$actual"
    else
      bad=$((bad + 1))
      printf '  FAIL %-58s expected %s, got %s\n' "$label" "$expected" "$actual"
    fi
  }

  echo "=== soak.sh --self-check: everything this tool decides, over synthetic inputs ==="
  echo
  echo "control leg verdict (DW-397 -- is the host manufacturing crashes?):"
  control_verdict 0 100 yes
  expect "100 clean control iterations" SOUND "$CONTROL_TAG"
  control_verdict 1 100 yes
  expect "ONE crash in 100 refuses the run" UNSOUND "$CONTROL_TAG"
  control_verdict 20 80 yes
  expect "the 2026-09-23 WSL2 reading" UNSOUND "$CONTROL_TAG"
  control_verdict 0 100 no
  expect "clean but matched no tests" EMPTY "$CONTROL_TAG"
  control_verdict 0 1 yes
  expect "a one-iteration control still reports SOUND" SOUND "$CONTROL_TAG"
  echo

  echo "translation verdict (story 1's derivation, in shell):"
  derive_translation x86_64 amd64 yes VirtualApple - no -
  expect "Rosetta container on Apple Silicon" XLATED "$VERDICT_TAG"
  derive_translation aarch64 amd64 yes - - no -
  expect "qemu: kernel aarch64, process amd64" XLATED "$VERDICT_TAG"
  derive_translation x86_64 amd64 yes - - no -
  expect "amd64 process, cpuinfo has no vendor_id" XLATED "$VERDICT_TAG"
  derive_translation aarch64 arm64 yes GenuineIntel - no -
  expect "arm64 process, x86-shaped cpuinfo" XLATED "$VERDICT_TAG"
  derive_translation x86_64 amd64 yes GenuineIntel - yes -
  expect "Rosetta marker present" XLATED "$VERDICT_TAG"
  derive_translation x86_64 amd64 yes GenuineIntel - no -
  expect "real amd64 silicon" native "$VERDICT_TAG"
  derive_translation aarch64 arm64 yes - 0x41 no -
  expect "native arm64 silicon" native "$VERDICT_TAG"
  derive_translation x86_64 amd64 yes Shanghai - no -
  expect "Zhaoxin's trimmed vendor id is real silicon" native "$VERDICT_TAG"
  derive_translation x86_64 amd64 yes "VIA VIA VIA" - no -
  expect "VIA's multi-word vendor id is real silicon" native "$VERDICT_TAG"
  derive_translation x86_64 amd64 yes "Geode by NSC" - no -
  expect "Geode's multi-word vendor id is real silicon" native "$VERDICT_TAG"
  derive_translation x86_64 amd64 yes "Bochs" - no -
  expect "an x86 vendor no real silicon reports" SUSPECT "$VERDICT_TAG"
  derive_translation x86_64 amd64 yes GenuineIntel - no "qemu-x86_64 (enabled)"
  expect "binfmt_misc interpreter for our own arch" SUSPECT "$VERDICT_TAG"
  derive_translation x86_64 amd64 no - - no -
  expect "/proc/cpuinfo unreadable" UNKNOWN "$VERDICT_TAG"
  derive_translation s390x amd64 yes GenuineIntel - no -
  expect "a machine string this tool cannot map" UNKNOWN "$VERDICT_TAG"
  derive_translation - amd64 yes GenuineIntel - no -
  expect "uname gave no machine" UNKNOWN "$VERDICT_TAG"
  derive_translation x86_64 - yes GenuineIntel - no -
  expect "this process's own architecture unreadable" UNKNOWN "$VERDICT_TAG"

  sig_dir="$(mktemp -d "${TMPDIR:-/tmp}/folio-soak-selfcheck.XXXXXX")"
  trap 'rm -rf "$sig_dir"' EXIT

  echo
  echo "finding a qemu registration in binfmt_misc, over a synthetic directory:"
  # THE REAL DIRECTORY CANNOT BE WRITTEN TO ON A DEVELOPER BOX, which is why the
  # hard-coded-filename version of this probe was never exercised anywhere and
  # degraded open. Every layout below is one a real host produces.
  mkdir -p "$sig_dir/binfmt"
  printf 'enabled\ninterpreter /usr/libexec/qemu-binfmt/x86_64-binfmt-P\nflags: OCF\n' \
    >"$sig_dir/binfmt/cross-amd64"
  expect "a registration NAMED anything, interpreter names our arch" \
    "cross-amd64 (/usr/libexec/qemu-binfmt/x86_64-binfmt-P, enabled)" \
    "$(binfmt_hits_in "$sig_dir/binfmt" amd64)"
  expect "  and that same entry is nothing to an arm64 process" "" \
    "$(binfmt_hits_in "$sig_dir/binfmt" arm64)"
  rm -f "$sig_dir/binfmt/cross-amd64"
  printf 'enabled\ninterpreter /usr/bin/qemu-aarch64-static\n' >"$sig_dir/binfmt/qemu-aarch64"
  expect "the ordinary qemu-aarch64 registration" \
    "qemu-aarch64 (/usr/bin/qemu-aarch64-static, enabled)" \
    "$(binfmt_hits_in "$sig_dir/binfmt" arm64)"
  expect "  an amd64 process is not translated by it" "" \
    "$(binfmt_hits_in "$sig_dir/binfmt" amd64)"
  printf 'enabled\ninterpreter /usr/bin/qemu-riscv64-static\n' >"$sig_dir/binfmt/qemu-riscv64"
  expect "somebody else's cross-build setup is not our translation" \
    "qemu-aarch64 (/usr/bin/qemu-aarch64-static, enabled)" \
    "$(binfmt_hits_in "$sig_dir/binfmt" arm64)"
  : >"$sig_dir/binfmt/status"
  expect "  and 'status' is not read as a registration" \
    "qemu-aarch64 (/usr/bin/qemu-aarch64-static, enabled)" \
    "$(binfmt_hits_in "$sig_dir/binfmt" arm64)"
  rm -rf "$sig_dir/binfmt"
  mkdir -p "$sig_dir/binfmt"
  : >"$sig_dir/binfmt/qemu-aarch64"
  expect "an unreadable registration still counts by its name" \
    "qemu-aarch64 (interpreter unreadable)" \
    "$(binfmt_hits_in "$sig_dir/binfmt" arm64)"

  echo
  echo "the ledger round trip — the ONLY path that can turn a clean soak into a pass:"
  # WRITE THEN READ, through the same two functions the run uses. Before this
  # existed, the first execution of this round trip anywhere would have been the
  # owner's hardware leg.
  led="$sig_dir/ledger.tsv"
  ledger_append "$led" arm64 "soak-host" "6.1.0" "86e7e5a" "sha-AAA"
  expect "a written row reads back for its own arch and native" yes \
    "$([ -n "$(ledger_lookup "$led" arm64 sha-AAA)" ] && echo yes || echo no)"
  expect "the OTHER architecture is not validated by it" no \
    "$([ -n "$(ledger_lookup "$led" amd64 sha-AAA)" ] && echo yes || echo no)"
  expect "nor is the same arch against a different native" no \
    "$([ -n "$(ledger_lookup "$led" arm64 sha-BBB)" ] && echo yes || echo no)"
  expect "  which is reported as a near miss, not as silence" yes \
    "$([ -n "$(ledger_lookup_other_native "$led" arm64 sha-BBB)" ] && echo yes || echo no)"
  expect "a ledger that does not exist validates nothing" no \
    "$([ -n "$(ledger_lookup "$sig_dir/absent.tsv" arm64 sha-AAA)" ] && echo yes || echo no)"
  ledger_append "$led" amd64 "other-host" "5.15.0" "86e7e5a" "sha-CCC"
  expect "a second row does not disturb the first" yes \
    "$([ -n "$(ledger_lookup "$led" arm64 sha-AAA)" ] && echo yes || echo no)"
  expect "  and the second is found on its own terms" yes \
    "$([ -n "$(ledger_lookup "$led" amd64 sha-CCC)" ] && echo yes || echo no)"
  expect "the row carries host, kernel, binding and native" \
    "arm64 REPRODUCED soak-host 6.1.0 86e7e5a sha-AAA" \
    "$(ledger_lookup "$led" arm64 sha-AAA | cut -f2-7 | tr '\t' ' ')"

  echo
  echo "reading the two named signatures out of the text a run leaves behind:"
  # The strings below are the ones actually recorded in DW-396 and in the CI
  # run that opened it — not paraphrases. If vstest or the kernel ever changes
  # its wording, this is where it shows up, on any machine, instead of on the
  # one host in the world that can produce a real crash.
  cat >"$sig_dir/crash.log" <<'LOG'
Test run for /repo/folio-dotnet/test/Folio8.Tests/bin/Release/net10.0/Folio8.Tests.dll
  Passed Folio8Tests.GoldenTests.RendersByteIdenticallyToTheGoldenCorpus
The active test run was aborted. Reason: Test host process crashed
LOG
  cat >"$sig_dir/clr.log" <<'LOG'
Internal CLR error. (0x80131506)
The active test run was aborted. Reason: Test host process crashed
LOG
  cat >"$sig_dir/pass.log" <<'LOG'
Passed!  - Failed:     0, Passed:    38, Skipped:     0, Total:    38
LOG
  cat >"$sig_dir/fail.log" <<'LOG'
  Failed Folio8Tests.GoldenTests.RendersByteIdenticallyToTheGoldenCorpus [12 ms]
  Error Message: Assert.Equal() Failure
Failed!  - Failed:     1, Passed:    37, Skipped:     0, Total:    38
LOG
  expect "vstest's aborted-run wording reads as a dead host" yes "$(log_says_host_died "$sig_dir/crash.log")"
  expect "a passing run does not" no "$(log_says_host_died "$sig_dir/pass.log")"
  expect "a failing ASSERTION does not" no "$(log_says_host_died "$sig_dir/fail.log")"
  expect "Internal CLR error (0x80131506) is read" yes "$(log_says_clr_error "$sig_dir/clr.log")"
  expect "a dead host with no CLR line is not" no "$(log_says_clr_error "$sig_dir/crash.log")"
  cat >"$sig_dir/nothing.log" <<'LOG'
No test matches the given testcase filter `FullyQualifiedName~NoSuchThing` in Folio8.Tests.dll
Passed!  - Failed:     0, Passed:     0, Skipped:     0, Total:     0
LOG
  # What .NET 10 actually printed for an empty filter at -v quiet, measured:
  # two header lines and not one word about tests.
  cat >"$sig_dir/nothing-quiet.log" <<'LOG'
Test run for /repo/folio-dotnet/test/Folio8.Tests/bin/Release/net10.0/Folio8.Tests.dll (.NETCoreApp,Version=v10.0)
A total of 1 test files matched the specified pattern.
LOG
  expect "a filter that matched nothing is not a clean run" yes "$(log_says_no_tests_ran "$sig_dir/nothing.log")"
  expect "  nor is the silent shape .NET 10 prints for it" yes "$(log_says_no_tests_ran "$sig_dir/nothing-quiet.log")"
  expect "  38 renders is a run" no "$(log_says_no_tests_ran "$sig_dir/pass.log")"
  expect "  and so is a run with a failing assertion" no "$(log_says_no_tests_ran "$sig_dir/fail.log")"
  : >"$sig_dir/dmesg.before"
  cat >"$sig_dir/dmesg.after" <<'LOG'
[  510.311940] signal: .NET TP Worker[31774] overflowed sigaltstack
[  510.311943] potentially unexpected fatal signal 11.
LOG
  kernel_new="$(kernel_lines_since "$sig_dir/dmesg.before" "$sig_dir/dmesg.after")"
  expect "the kernel's own overflow line is read" yes "$(lines_say_overflow "$kernel_new")"
  cat >"$sig_dir/dmesg.unrelated" <<'LOG'
[  510.311940] nginx[9182]: segfault at 0 ip 00007f
LOG
  kernel_unrelated="$(kernel_lines_since "$sig_dir/dmesg.before" "$sig_dir/dmesg.unrelated")"
  expect "somebody else's segfault is not this defect" no "$(lines_say_overflow "$kernel_unrelated")"
  # A line the kernel printed BEFORE this iteration must not be read as this
  # iteration's. Without the before/after diff a busy box with one old overflow
  # in its ring buffer would report a reproduction on every single iteration.
  cp "$sig_dir/dmesg.after" "$sig_dir/dmesg.stale"
  kernel_stale="$(kernel_lines_since "$sig_dir/dmesg.stale" "$sig_dir/dmesg.stale")"
  expect "an overflow already in the ring buffer is not new" no "$(lines_say_overflow "$kernel_stale")"

  echo
  echo "per-iteration outcome (exit status is never asked to mean more than it does):"
  classify_run 0 no no no;    expect "exit 0, host alive" CLEAN "$RUN_OUTCOME"
  classify_run 1 yes no yes;  expect "dead host + kernel's sigaltstack line" DW396 "$RUN_OUTCOME"
  classify_run 1 yes yes no;  expect "dead host + Internal CLR error" DW396 "$RUN_OUTCOME"
  classify_run 139 no no no;  expect "runner killed by SIGSEGV, no signature" CRASH-UNKNOWN "$RUN_OUTCOME"
  classify_run 1 yes no no;   expect "dead host, neither signature" CRASH-UNKNOWN "$RUN_OUTCOME"
  classify_run 1 no no no;    expect "a failing assertion is not a crash" SUITE-FAILED "$RUN_OUTCOME"
  classify_run 0 no no yes;   expect "kernel line but nothing died: still clean" CLEAN "$RUN_OUTCOME"

  echo
  echo "final verdict (the ruling that a clean run alone is not a pass):"
  final_verdict soak 100 CLEAN yes yes
  expect "clean soak, harness validated here" "CLEAN — VALIDATED HARNESS" "$FINAL_TAG"
  expect "  and it exits zero" 0 "$FINAL_STATUS"
  final_verdict soak 100 CLEAN no yes
  expect "clean soak, harness NEVER validated" "UNVALIDATED" "$FINAL_TAG"
  expect "  and it exits non-zero" 1 "$FINAL_STATUS"
  final_verdict soak 7 DW396 yes yes
  expect "soak died with a named signature" "FAILED — DW-396" "$FINAL_TAG"
  final_verdict soak 7 CRASH-UNKNOWN yes yes
  expect "soak died with neither signature" "FAILED — UNEXPLAINED CRASH" "$FINAL_TAG"
  final_verdict soak 7 SUITE-FAILED yes yes
  expect "the suite failed; nothing is claimed" "INCONCLUSIVE" "$FINAL_TAG"
  final_verdict reproduce 3 DW396 no yes
  expect "reproduction leg caught it" "REPRODUCED" "$FINAL_TAG"
  expect "  and it exits zero" 0 "$FINAL_STATUS"
  final_verdict reproduce 25 CLEAN no yes
  expect "pre-fix binding stayed clean: no validation" "NOT REPRODUCED" "$FINAL_TAG"
  expect "  and it exits non-zero" 1 "$FINAL_STATUS"
  final_verdict reproduce 4 CRASH-UNKNOWN no yes
  expect "pre-fix binding died of something else" "NOT REPRODUCED — UNEXPLAINED CRASH" "$FINAL_TAG"

  echo
  echo "the mechanism check (a nameless death attributed by independent evidence, or refused):"
  mech_dir="$(mktemp -d)"
  cat >"$mech_dir/result.log" <<'MEOF'
  load                                       ....XX..X  -> 81 / 100  ( SIGSEGV=81 )
      load=yes gc=yes fix=none touched=-
  noload                                     .........  -> 0 / 100

=== RESULT ===
  load                                         81 / 100   SIGSEGV=81
  noload                                        0 / 100   none
MEOF
  mechanism_parse "$mech_dir/result.log"; expect "the RESULT block parses: loaded" 81 "$MECH_LOAD"
  expect "  control" 0 "$MECH_CONTROL"; expect "  per arm" 100 "$MECH_N"
  printf '  load   ....X -> 3 / 5\n' >"$mech_dir/noresult.log"
  if mechanism_parse "$mech_dir/noresult.log"; then expect "a log with no RESULT block is refused" refused accepted; else expect "a log with no RESULT block is refused" refused refused; fi
  MECH_LOAD=22; MECH_CONTROL=0; MECH_N=40; mechanism_verdict; expect "loaded dies, control clean" ATTRIBUTED "$MECH_TAG"
  MECH_LOAD=22; MECH_CONTROL=1; MECH_N=40; mechanism_verdict; expect "control died: the host manufactures deaths" HOST-MANUFACTURES "$MECH_TAG"
  MECH_LOAD=0;  MECH_CONTROL=0; MECH_N=40; mechanism_verdict; expect "nothing died: the mechanism did not fire here" DID-NOT-FIRE "$MECH_TAG"
  MECH_LOAD=""; MECH_CONTROL=""; MECH_N=""; mechanism_verdict; expect "no numbers: not run" NOT-RUN "$MECH_TAG"
  MECH_LOAD=22; MECH_CONTROL=0; MECH_N=40; MECH_TAG=ATTRIBUTED
  final_verdict reproduce 2 CRASH-UNKNOWN no yes
  expect "a nameless death, attributed by the mechanism" "REPRODUCED — BY MECHANISM" "$FINAL_TAG"
  expect "  and it exits zero" 0 "$FINAL_STATUS"
  MECH_TAG=HOST-MANUFACTURES; MECH_CONTROL=3
  final_verdict reproduce 2 CRASH-UNKNOWN no yes
  expect "a nameless death on a host whose control dies" "NOT REPRODUCED — HOST MANUFACTURES DEATHS" "$FINAL_TAG"
  expect "  and it exits non-zero" 1 "$FINAL_STATUS"
  MECH_TAG=DID-NOT-FIRE; MECH_LOAD=0; MECH_CONTROL=0
  final_verdict reproduce 2 CRASH-UNKNOWN no yes
  expect "a nameless death where the mechanism is not live" "NOT REPRODUCED — UNEXPLAINED CRASH" "$FINAL_TAG"
  expect "  and it exits non-zero" 1 "$FINAL_STATUS"
  MECH_LOAD=22; MECH_CONTROL=0; MECH_N=40; MECH_TAG=ATTRIBUTED
  final_verdict reproduce 150 CLEAN no yes
  expect "clean pre-fix run where the mechanism IS live: workload too light" "NOT REPRODUCED — MECHANISM LIVE, WORKLOAD TOO LIGHT" "$FINAL_TAG"
  expect "  and it exits non-zero: a clean pre-fix run validates nothing" 1 "$FINAL_STATUS"
  MECH_LOAD=0; MECH_CONTROL=0; MECH_TAG=DID-NOT-FIRE
  final_verdict reproduce 150 CLEAN no yes
  expect "clean pre-fix run where the mechanism is NOT live" "NOT REPRODUCED — MECHANISM NOT LIVE HERE" "$FINAL_TAG"
  MECH_LOAD=""; MECH_CONTROL=""; MECH_N=""; MECH_TAG=NOT-RUN
  final_verdict reproduce 25 CLEAN no yes
  expect "clean pre-fix run, mechanism not consulted: the plain refusal" "NOT REPRODUCED" "$FINAL_TAG"
  rm -rf "$mech_dir"
  final_verdict soak 100 KERNEL-OVERFLOW-ONLY yes yes
  expect "kernel overflowed while every iteration passed" "FAILED — DW-396 SIGNATURE, HOST SURVIVED" "$FINAL_TAG"
  expect "  and it exits non-zero" 1 "$FINAL_STATUS"
  final_verdict reproduce 25 KERNEL-OVERFLOW-ONLY no yes
  expect "the same, on the pre-fix leg, validates the harness" "REPRODUCED — KERNEL SIGNATURE, HOST SURVIVED" "$FINAL_TAG"
  expect "  and it exits zero" 0 "$FINAL_STATUS"
  final_verdict soak 60 INTERRUPTED yes yes
  expect "Ctrl-C is not 60 clean iterations" "STOPPED — PARTIAL RUN" "$FINAL_TAG"
  expect "  and it exits 130" 130 "$FINAL_STATUS"
  final_verdict reproduce 12 INTERRUPTED no yes
  expect "nor is it a failure to reproduce" "STOPPED — PARTIAL RUN" "$FINAL_TAG"
  final_verdict soak 100 CLEAN yes no
  case "$FINAL_TEXT" in
    *"dmesg could not be read"*) expect "an unreadable dmesg is disclosed in the verdict" yes yes ;;
    *) expect "an unreadable dmesg is disclosed in the verdict" yes no ;;
  esac

  # THE BANNER ITSELF, RENDERED. Twice now (runs 35998599319 and 36009787332)
  # an apostrophe inside ${var:-word} in the unquoted HEADER heredoc opened a
  # string that closed lines later, and the soak exited after its banner with
  # `bash -n` green and every case above green. No host this tool runs on
  # reaches the banner without a Linux SDK, so it is rendered here from the
  # script's own text with every variable empty: the render must succeed and
  # print no literal `${`.
  _banner="$(sed -n '/^cat <<HEADER$/,/^HEADER$/p' "${BASH_SOURCE[0]}")"
  if [ -z "$_banner" ]; then
    expect "the HEADER banner is where the self-check looks for it" found missing
  else
    _rendered="$(bash -c "set +u; $_banner" 2>&1)" && _rc=0 || _rc=$?
    expect "the HEADER banner renders (no apostrophe inside \${var:-word})" 0 "$_rc"
    case "$_rendered" in
      *'${'*) expect "the rendered banner carries no literal \${" none literal ;;
      *) expect "the rendered banner carries no literal \${" none none ;;
    esac
  fi

  echo
  if [ "$bad" != "0" ]; then
    echo "$bad of $checks cases did not render as expected." >&2
    exit 1
  fi
  echo "all $checks cases rendered as expected. This says the tool's JUDGEMENT is sound;"
  echo "it measures nothing and soaks nothing."
  exit 0
fi

# =====================================================================
# THE IMPURE HALF: read the host, then act on what the derivations above say.
# =====================================================================

if [ "$(uname -s)" != "Linux" ]; then
  echo "soak: DW-396 is a POSIX signal-stack defect and this host is $(uname -s), where it cannot occur." >&2
  echo "  There is deliberately no container mode: a linux/<arch> container on this machine is either" >&2
  echo "  the same silicon (arm64, admissible) or Rosetta (amd64, NOT admissible), and a wrapper that" >&2
  echo "  offered both would invite the second. Run this ON the Linux host whose evidence you want." >&2
  echo "  './soak.sh --self-check' exercises this tool's own judgement here." >&2
  exit 1
fi

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'
  else echo "unavailable (no sha256sum or shasum on PATH)"; fi
}

# ------------------------------------------------- the translation verdict

# This process's OWN architecture is the ELF e_machine of /proc/self/exe. It is
# the shell's answer to the probe's RuntimeInformation.ProcessArchitecture, and
# it is what makes the comparison with uname meaningful: under qemu-user the
# kernel's machine string is faked to the target, but the binary being run
# really is the target's.
kernel_machine="$(uname -m 2>/dev/null || echo '-')"
[ -n "$kernel_machine" ] || kernel_machine="-"
self_arch="$(elf_arch_of /proc/self/exe)"

cpuinfo_readable="no"; cpu_vendor="-"; cpu_implementer="-"
if [ -r /proc/cpuinfo ] && cpuinfo="$(cat /proc/cpuinfo 2>/dev/null)"; then
  cpuinfo_readable="yes"
  v="$(printf '%s\n' "$cpuinfo" | awk -F: '/^vendor_id[ \t]*:/ {gsub(/^[ \t]+|[ \t]+$/, "", $2); if ($2 != "") {print $2; exit}}')"
  i="$(printf '%s\n' "$cpuinfo" | awk -F: '/^CPU implementer[ \t]*:/ {gsub(/^[ \t]+|[ \t]+$/, "", $2); if ($2 != "") {print $2; exit}}')"
  [ -n "$v" ] && cpu_vendor="$v"
  [ -n "$i" ] && cpu_implementer="$i"
fi

rosetta="no"
if [ -d /run/rosetta ] || [ -e /proc/sys/fs/binfmt_misc/rosetta ]; then rosetta="yes"; fi

binfmt_hits="-"
binfmt_state="not mounted"
if [ -d /proc/sys/fs/binfmt_misc ]; then
  binfmt_state="no interpreter registered that would run $self_arch"
  hits="$(binfmt_hits_in /proc/sys/fs/binfmt_misc "$self_arch" | join_with ", ")"
  if [ -n "$hits" ]; then
    binfmt_hits="$hits"
    binfmt_state="$hits"
  fi
fi

derive_translation "$kernel_machine" "$self_arch" "$cpuinfo_readable" "$cpu_vendor" "$cpu_implementer" "$rosetta" "$binfmt_hits"

arch="$(arch_of_machine "$kernel_machine")"
[ -n "$arch" ] || arch="$kernel_machine"

echo "=== soak.sh — translation verdict, taken before anything is run ==="
echo "  uname machine        : $kernel_machine"
echo "  process architecture : $self_arch (ELF e_machine of /proc/self/exe)"
if [ "$cpuinfo_readable" != "yes" ]; then
  echo "  /proc/cpuinfo        : could not be read"
elif [ "$cpu_vendor" != "-" ]; then
  echo "  /proc/cpuinfo        : vendor_id $cpu_vendor (x86-shaped)"
elif [ "$cpu_implementer" != "-" ]; then
  echo "  /proc/cpuinfo        : CPU implementer $cpu_implementer (arm-shaped)"
else
  echo "  /proc/cpuinfo        : readable, but neither vendor_id nor CPU implementer"
fi
echo "  /run/rosetta         : $([ "$rosetta" = yes ] && echo present || echo absent)"
echo "  binfmt_misc for self : $binfmt_state"
echo "  verdict              : $VERDICT_SUMMARY"
echo

# THE REFUSAL, AND WHY IT COVERS MORE THAN "TRANSLATED". A verdict this tool
# cannot stand behind is not weaker evidence, it is no evidence: the whole value
# of a soak result is that the host it came from is known. SUSPECT and UNKNOWN
# are therefore refused alongside TRANSLATED, each naming the signal that
# produced it. There is no override flag, deliberately -- an override is how a
# translated tally gets into the record with a footnote nobody reads, which has
# already happened twice in DW-396.
#
# EVERY CAVEAT GOES TO STDOUT, WITH THE HEADER AND THE VERDICT. `soak.sh -n 100
# > record.log` is how a result gets written down, and a refusal, a warning or a
# caveat on stderr is exactly the line that would be lost from that file while
# the number survives -- the opposite of this tool's whole thesis. Nothing here
# is on stderr; the exit status carries the failure.
if [ "$VERDICT_TAG" != "native" ]; then
  echo "soak: REFUSING TO RUN on this host."
  echo "  $VERDICT_SUMMARY"
  echo
  case "$VERDICT_TAG" in
    XLATED)
      echo "  A translated host cannot distinguish the pre-fix binding from the post-fix one in"
      echo "  EITHER direction: measured this epic, under Rosetta amd64 both bindings die, at a"
      echo "  different test count every time. CAP-5 excludes emulated and translated hosts"
      echo "  outright, and a result from one would not be admissible however it came out."
      ;;
    SUSPECT)
      echo "  Nothing here proves translation, and nothing rules it out. A soak result is only"
      echo "  worth the certainty about the host it ran on, so this is refused rather than"
      echo "  recorded with a caveat."
      ;;
    UNKNOWN)
      echo "  The signals that would establish this host's architecture could not all be read, so"
      echo "  the runner cannot say what it would be soaking. Restore the missing signal (procfs"
      echo "  is usually the one a container masks) and run this again."
      ;;
  esac
  echo
  echo "  Real amd64 and real arm64 are soaked separately and neither stands in for the other:"
  echo "  the spike measured 16 KiB of altstack against 24 KiB for the same handler frame."
  exit 1
fi

# ------------------------------------------------------------ what we soak

# THE TOOLS COME AFTER THE HOST VERDICT, DELIBERATELY. A translated host is
# refused whether or not it has an SDK on it, and a missing SDK must never be
# the reason a Rosetta box was let past the gate above.
if ! command -v dotnet >/dev/null 2>&1; then
  echo "soak: the .NET SDK is not on PATH (https://dotnet.microsoft.com/download); the test project targets net10.0" >&2
  exit 1
fi
if ! command -v git >/dev/null 2>&1; then
  echo "soak: git is not on PATH, and a non-HEAD binding is a git worktree; install git and run this again" >&2
  exit 1
fi

root="$(cd "$here/../.." && pwd)"
if [ ! -d "$root/.git" ] && [ ! -f "$root/.git" ]; then
  echo "soak: $root is not a git working tree; run this from a clone rather than from an extracted archive" >&2
  exit 1
fi

if [ -z "$native" ]; then
  # THE SHIPPED NATIVE FOR THIS RID IS PREFERRED over the host build: soaking
  # what the package actually carries is the point, and build/native/host/ on a
  # developer's box may be a different toolchain's output.
  case "$arch" in
    amd64) rid="linux-x64" ;;
    arm64) rid="linux-arm64" ;;
    *) rid="" ;;
  esac
  native="$root/folio-dotnet/build/native/host/libfolio8_native.so"
  if [ -n "$rid" ] && [ -f "$root/folio-dotnet/build/native/$rid/libfolio8_native.so" ]; then
    native="$root/folio-dotnet/build/native/$rid/libfolio8_native.so"
  fi
fi
if [ ! -f "$native" ]; then
  echo "soak: no native library at $native."
  echo "  Run folio-dotnet/build/build-native.sh (host, or linux-x64 / linux-arm64) first, or pass"
  echo "  --native <path>. Without it the suite can only report a DllNotFoundException, which is"
  echo "  not a soak result."
  exit 1
fi
native="$(cd "$(dirname "$native")" && pwd)/$(basename "$native")"
native_sha="$(sha256_of "$native")"

# THE STAGED NATIVE IS READ, NOT TRUSTED. `--native` and a stale
# build/native/host/ can both hand this tool the other architecture's .so, and
# the only symptom is SUITE-FAILED on every iteration with a DllNotFoundException
# buried in a log -- an hour of a hardware leg spent on a staging mistake. A tool
# that reads its own e_machine rather than take the operator's word for the
# architecture has no business taking it for the library either.
native_arch="$(elf_arch_of "$native")"
if [ "$native_arch" = "-" ]; then
  echo "soak: $native is not an ELF shared library this tool can read an e_machine from."
  echo "  Point --native at a libfolio8_native.so built by folio-dotnet/build/build-native.sh."
  exit 1
fi
if [ "$native_arch" != "$arch" ]; then
  echo "soak: the staged native is $native_arch and this host is $arch."
  echo "  $native"
  echo "  Nothing was run: every iteration would fail to load it, and a suite that cannot reach"
  echo "  the engine says nothing about a defect at the ABI boundary. Build or pass the $arch"
  echo "  native — folio-dotnet/build/build-native.sh, or --native <path>."
  exit 1
fi

# A DIRTY BINDING IS REFUSED, NOT WARNED ABOUT. The warning this replaces said
# the result "must not be recorded against <commit>" and then recorded exactly
# that in its own header, which leaves the operator holding a provenance block
# that is wrong in the one field the whole record is indexed by. The suite is in
# the check beside the binding: a modified test under a clean-commit header is
# the same error, and the suite is what decides whether an iteration was clean.
dirty="$(git -C "$root" status --porcelain -- folio-dotnet/src/Folio8 folio-dotnet/test/Folio8.Tests 2>/dev/null || true)"
if [ -n "$dirty" ]; then
  echo "soak: REFUSING TO RUN — the binding or the suite has uncommitted changes:"
  printf '%s\n' "$dirty" | sed 's/^/    /'
  echo
  echo "  A soak result is indexed by the commit in its header, and these changes are not in any"
  echo "  commit. Commit or stash them and run this again. (A modified suite counts: it is what"
  echo "  decides whether an iteration was clean.)"
  exit 1
fi

if [ "$binding" = "head" ]; then
  binding_commit="$(git -C "$root" rev-parse --short HEAD 2>/dev/null || echo '?')"
  binding_desc="HEAD $binding_commit"
else
  if ! binding_commit="$(git -C "$root" rev-parse --verify "$binding" 2>/dev/null)"; then
    echo "soak: cannot resolve '$binding' in $root."
    echo "  A shallow clone is the usual cause — run 'git -C $root fetch --unshallow' — or pass a"
    echo "  commit that exists in this history. The pre-fix binding is ${engine_threads_commit}^."
    exit 1
  fi
  binding_desc="$(git -C "$root" log --oneline -1 "$binding_commit") [requested as '$binding', checked out as a git worktree]"
fi

# A REPRODUCTION LEG MUST BE A PRE-FIX BINDING, AND THE TOOL CHECKS RATHER THAN
# ASKS. Without this, `--reproduce --binding head` is the front door to the one
# inference this whole tool exists to refuse: a POST-fix binding that dies for
# any reason carrying a named signature writes REPRODUCED to the ledger, and
# every later soak on that architecture reads VALIDATED. The test is ancestry,
# not equality, so any commit from before the engine threads landed qualifies
# and nothing after it does.
if [ "$mode" = "reproduce" ]; then
  prefix_tip=""
  if ! prefix_tip="$(git -C "$root" rev-parse --verify "${engine_threads_commit}^" 2>/dev/null)"; then
    echo "soak: cannot resolve ${engine_threads_commit}^, the pre-fix binding, in $root."
    echo "  A shallow clone is the usual cause — run 'git -C $root fetch --unshallow'."
    exit 1
  fi
  reproduce_ref="$binding_commit"
  if [ "$binding" = "head" ]; then
    reproduce_ref="$(git -C "$root" rev-parse --verify HEAD 2>/dev/null || echo '')"
  fi
  if [ -z "$reproduce_ref" ] || ! git -C "$root" merge-base --is-ancestor "$reproduce_ref" "$prefix_tip" 2>/dev/null; then
    echo "soak: REFUSING a reproduction leg against '$binding'."
    echo "  $binding_desc"
    echo
    echo "  A reproduction leg exists to show the harness can SEE the defect, so it has to run a"
    echo "  binding that still HAS it: one at or before ${engine_threads_commit}^, the commit"
    echo "  before every crossing moved onto binding-owned engine threads. '$binding' is not an"
    echo "  ancestor of it."
    echo
    echo "  This is refused rather than warned about because it is the front door to the exact"
    echo "  inference this tool exists to refuse: a post-fix binding that died for any reason at"
    echo "  all would write REPRODUCED to the ledger, and every later soak on this architecture"
    echo "  would then read VALIDATED."
    exit 1
  fi
fi

if [ -z "$ledger" ]; then
  ledger="${XDG_STATE_HOME:-$HOME/.local/state}/folio-soak/ledger.tsv"
fi

work="$(mktemp -d "${TMPDIR:-/tmp}/folio-soak.XXXXXX")"

# THE LOGS LIVE OUTSIDE THE WORK TREE, so keeping them does not mean keeping a
# git worktree behind as well. With no --log-dir they used to die with $work --
# including on the run that finally caught the defect, whose failure block named
# `.../iteration-7.log` for a path the trap had just deleted. Anything but a
# clean run keeps them now, and the verdict says where.
logs="$(mktemp -d "${TMPDIR:-/tmp}/folio-soak-logs.XXXXXX")"
logs_are_temporary=1
if [ -n "$log_dir" ]; then
  mkdir -p "$log_dir"
  logs="$(cd "$log_dir" && pwd)"
  logs_are_temporary=0
fi

keep_logs=0
cleanup() {
  rm -rf "$work" 2>/dev/null || true
  git -C "$root" worktree prune >/dev/null 2>&1 || true
  if [ "$logs_are_temporary" = "1" ] && [ "$keep_logs" = "0" ]; then
    rm -rf "$logs" 2>/dev/null || true
  fi
}
trap cleanup EXIT

# CTRL-C IS THE LIKELIEST REAL ENDING OF THE ONE ACTIVITY THE OWNER SITS AND
# WATCHES. Without this, an interrupt at iteration 60 of 100 on real hardware
# tore down the worktree through the EXIT trap and printed nothing at all: no
# verdict, no ledger line, no record that sixty iterations had been clean. The
# handler only raises a flag -- the loop notices it, stops, and the ordinary
# verdict path prints a partial result and cleans up exactly as it would have.
interrupted=0
on_interrupt() {
  interrupted=1
  echo
  echo "==> interrupt received; stopping after the current iteration and printing a partial verdict"
}
trap on_interrupt INT TERM

# THE ENGINE IS HELD FIXED WHILE THE BINDING VARIES. One native file is chosen
# above and staged into a directory of this runner's own for every leg, so the
# reproduction leg and the soak leg cross into the SAME engine. Letting each
# leg stage its own tree's native would vary two things at once and neither
# result would mean anything.
stage="$work/native"
mkdir -p "$stage/host"
cp "$native" "$stage/host/libfolio8_native.so"

if [ "$binding" = "head" ]; then
  tests_csproj="$root/folio-dotnet/test/Folio8.Tests/Folio8.Tests.csproj"
else
  tree="$work/binding-tree"
  if ! git -C "$root" worktree add --detach "$tree" "$binding_commit" >"$logs/worktree.log" 2>&1; then
    echo "soak: could not create the git worktree at $tree:"
    sed 's/^/  /' "$logs/worktree.log"
    exit 1
  fi
  tests_csproj="$tree/folio-dotnet/test/Folio8.Tests/Folio8.Tests.csproj"
  if [ ! -f "$tests_csproj" ]; then
    echo "soak: $binding_commit has no folio-dotnet/test/Folio8.Tests/Folio8.Tests.csproj, so there is no suite to loop there"
    exit 1
  fi
fi

# ------------------------------------------------------------ the ledger

# WHAT THE LEDGER IS FOR, AND WHAT IT IS NOT. It is this tool's own record that
# a reproduction leg has run HERE and caught the defect — the thing that turns a
# later clean run from a tally into evidence. It is keyed by ARCHITECTURE
# because the spec's ruling is that amd64 and arm64 are soaked separately and
# neither stands in for the other. It lives outside the repository: it is a fact
# about a machine, not about the source tree, and committing one host's ledger
# would let another host inherit a validation it never earned.
ledger_line="$(ledger_lookup "$ledger" "$arch" "$native_sha")"
ledger_near_miss=""
if [ -n "$ledger_line" ]; then
  validated="yes"
else
  validated="no"
  ledger_near_miss="$(ledger_lookup_other_native "$ledger" "$arch" "$native_sha")"
fi

record_reproduction() {
  if ! ledger_append "$ledger" "$arch" "$(uname -n)" "$(uname -r)" "$binding_commit" "$native_sha"; then
    echo "  ⚠ the reproduction could not be written to $ledger; a later soak on this host will"
    echo "    report UNVALIDATED until it can be. Pass --ledger <writable path>."
    return 1
  fi
  return 0
}

# --------------------------------------------------------------- provenance

started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# THE TOOL IS STAMPED BY ITS OWN CONTENT, NOT BY HEAD. The first cut printed
# `git rev-parse HEAD` as "the tool at <sha>", which the committed evidence
# already falsified: it recorded `soak.sh at f3b68f5` for a file that is not in
# f3b68f5 at all. A content hash is true whether this file is committed,
# uncommitted, or copied onto a hardware box on its own — which is how it will
# usually arrive on the WSL2 leg.
tool_sha="$(sha256_of "${BASH_SOURCE[0]}")"

dmesg_available="yes"
if ! dmesg >/dev/null 2>&1; then
  dmesg_available="no"
fi

# Computed OUTSIDE the heredoc. Inside `${var:-word}` bash parses quotes even
# in a heredoc, and run 35998599319 died on an apostrophe in that word: the
# substitution failed under set -e right after printing the banner, and no
# leg ran. Plain assignments have no such edge.
if [ -n "$gc_gen0size" ]; then
  gc_pressure_desc="DOTNET_GCgen0size=$gc_gen0size on every test host"
else
  gc_pressure_desc="none; the runtime collects at its own pace"
fi

cat <<HEADER
=== soak.sh — what is being run ===
  mode                 : $mode
  binding              : $binding_desc
  native               : $native
  native sha256        : $native_sha
  suite                : $tests_csproj
  filter               : $filter
  gc pressure          : $gc_pressure_desc
  engine switch        : $leave_desc
  iterations requested : $iterations
  host                 : $(uname -n)
  kernel               : $(uname -s) $(uname -r)
  architecture         : $arch
  translation verdict  : $VERDICT_SUMMARY
  dmesg                : $([ "$dmesg_available" = yes ] && echo "readable — the kernel's 'overflowed sigaltstack' line is being watched for" || echo "NOT READABLE — the kernel signature cannot be watched for; only the CLR-side one")
  harness validated    : $([ "$validated" = yes ] && echo "YES for $arch against this native — $ledger_line" || echo "NO for $arch against this native — no reproduction leg is on the ledger")
  ledger               : $ledger
  logs                 : $logs$([ -n "$log_dir" ] || echo " (temporary; kept if anything but a clean run comes out)")
  tool                 : folio-dotnet/build/soak.sh, sha256 $tool_sha
  started              : $started
  context              : ${host_context:-(none given)}
HEADER
echo

if [ -n "$ledger_near_miss" ]; then
  echo "note: this architecture HAS a reproduction on the ledger, but against a different native"
  echo "      library, so it does not validate this run:"
  echo "        $ledger_near_miss"
  echo "      Re-run the reproduction leg against the native staged today, or soak the native that"
  echo "      reproduction was taken against."
  echo
fi
if [ -n "$short_run_warning" ]; then
  echo "$short_run_warning"
  echo
fi
if [ "$mode" = "reproduce" ] && [ "$validated" = "yes" ]; then
  echo "note: this architecture is already validated on this ledger. Running the reproduction leg"
  echo "      again is fine — it re-proves the harness against the native staged today."
  echo
fi

# ------------------------------------------------------------- the loop

echo "==> building the suite"
if ! dotnet build "$tests_csproj" -c Release -v quiet --nologo \
    -p:FolioNativeDir="$stage" >"$logs/build.log" 2>&1; then
  sed 's/^/  /' "$logs/build.log"
  echo
  echo "soak: the suite did not BUILD, so nothing was run and nothing is claimed."
  exit 1
fi

# ------------------------------------------------------- the control leg
#
# BEFORE the binding is measured, the HOST is. See the header: this exists
# because a WSL2 box once crashed the test host in 20 of 80 runs of a workload
# with the native absent, and both soak legs on it were read as measurements of
# the binding (DW-397).

control_logs="$logs/control"
mkdir -p "$control_logs"

# THE CONTROL LEG'S ONE CLAIM, CHECKED RATHER THAN ASSERTED. "With the engine
# absent" is the entire value of this leg: if the filter loads the native, the
# control is exposed to DW-396 and a crash there proves the opposite of what it
# is read as proving. The check is empirical and costs one iteration -- move
# the staged native aside, run the filter once, put it back. A filter that
# needs the engine fails without it.
echo "==> control leg: asserting the filter does not load the native"
_cf_hidden="$stage/host/.hidden-libfolio8_native.so"
mv "$stage/host/libfolio8_native.so" "$_cf_hidden"
set +e
env ${gc_gen0size:+DOTNET_GCgen0size=$gc_gen0size} dotnet test "$tests_csproj" -c Release --no-build --nologo -v quiet \
  -p:FolioNativeDir="$stage" --filter "$control_filter" >"$control_logs/engine-free-check.log" 2>&1
_cf_status=$?
set -e
mv "$_cf_hidden" "$stage/host/libfolio8_native.so"
if [ "$_cf_status" != 0 ] || [ "$(log_says_no_tests_ran "$control_logs/engine-free-check.log")" = "yes" ]; then
  keep_logs=1
  echo
  echo "=== REFUSED — THE CONTROL FILTER IS NOT ENGINE-FREE ==="
  echo
  echo "  With the native moved aside the control filter did not pass, so it CROSSES THE ABI."
  echo "  A control leg that loads the Go library is exposed to the defect it exists to rule"
  echo "  out -- Go installs its SA_ONSTACK handlers process-wide at dlopen -- and a crash"
  echo "  there would be read as host noise when it may be DW-396."
  echo
  echo "  filter : $control_filter"
  echo "  log    : $control_logs/engine-free-check.log"
  echo
  echo "  Narrow --control-filter until it passes with no native present."
  exit 1
fi
echo "    it does not: the filter passes with no native present"

echo "==> control leg: $iterations iterations with the engine ABSENT"
echo "    filter: $control_filter"
control_crashes=0
control_completed=0
control_ran_tests="no"
for c in $(seq 1 "$iterations"); do
  if [ "$interrupted" = "1" ]; then break; fi
  clog="$control_logs/control-$c.log"
  set +e
  env ${gc_gen0size:+DOTNET_GCgen0size=$gc_gen0size} dotnet test "$tests_csproj" -c Release --no-build --nologo -v quiet \
    -p:FolioNativeDir="$stage" --filter "$control_filter" >"$clog" 2>&1
  cstatus=$?
  set -e
  if [ "$interrupted" = "1" ]; then break; fi
  control_completed=$((control_completed + 1))
  if [ "$(log_says_host_died "$clog")" = "yes" ]; then
    control_crashes=$((control_crashes + 1))
    printf '\r    iteration %d of %d ... HOST DIED\n' "$c" "$iterations"
  else
    if [ "$(log_says_no_tests_ran "$clog")" = "no" ]; then control_ran_tests="yes"; fi
    printf '\r    iteration %d of %d ... clean' "$c" "$iterations"
  fi
  if [ "$cstatus" = 0 ]; then :; fi
done
echo

if [ "$interrupted" = "1" ]; then
  echo
  echo "soak: interrupted during the control leg. Nothing is claimed."
  exit 1
fi

control_verdict "$control_crashes" "$control_completed" "$control_ran_tests"
if [ "$CONTROL_TAG" != "SOUND" ]; then
  keep_logs=1
  echo
  echo "=== REFUSED — THE CONTROL LEG DID NOT PASS ($CONTROL_TAG) ==="
  echo
  echo "  $CONTROL_TEXT"
  echo
  echo "  control filter : $control_filter"
  echo "  control logs   : $control_logs"
  echo "  host           : $(uname -n), $(uname -s) $(uname -r), $arch"
  echo "  translation    : $VERDICT_SUMMARY"
  echo
  echo "  The soak leg was NOT run. Nothing is written to the ledger. Fix the host --"
  echo "  or move to one that passes this leg -- before reading anything into a crash"
  echo "  from the binding, because on this host the two cannot be told apart."
  exit 1
fi
echo "    $CONTROL_TEXT"
echo

before_dmesg="$work/dmesg.before"
after_dmesg="$work/dmesg.after"
: >"$before_dmesg"

outcome="CLEAN"
detail=""
completed=0
failures=""
kernel_notes=""
kernel_overflow_seen=0

for i in $(seq 1 "$iterations"); do
  # Checked at the top as well as after the run, so an interrupt that arrives
  # during the BUILD -- or between iterations -- does not buy one more full
  # iteration before it is noticed.
  if [ "$interrupted" = "1" ]; then
    outcome="INTERRUPTED"
    detail="stopped by hand before iteration $i"
    break
  fi

  if [ "$dmesg_available" = "yes" ]; then
    dmesg >"$before_dmesg" 2>/dev/null || true
  fi

  printf '==> iteration %d of %d ... ' "$i" "$iterations"
  log="$logs/iteration-$i.log"
  set +e
  env ${gc_gen0size:+DOTNET_GCgen0size=$gc_gen0size} ${leave_env:+$leave_env} dotnet test "$tests_csproj" -c Release --no-build --nologo -v quiet \
    -p:FolioNativeDir="$stage" --filter "$filter" >"$log" 2>&1
  status=$?
  set -e

  # AN INTERRUPTED ITERATION IS NOT A CRASHED ONE. Ctrl-C kills the child, which
  # comes back as a signal death and would otherwise be classified as an
  # unexplained crash and reported as a finding. The flag is checked here, before
  # anything is read out of that iteration, and the iteration is discarded.
  if [ "$interrupted" = "1" ]; then
    echo "interrupted"
    outcome="INTERRUPTED"
    detail="stopped by hand during iteration $i"
    break
  fi
  completed="$i"

  # CHECKED ON THE FIRST ITERATION ONLY, AND ONLY IF IT EXITED 0: whether the
  # filter selected anything is a property of the filter, not of the run, and a
  # log from a dead test host is missing its summary line for a quite different
  # reason.
  if [ "$i" = "1" ] && [ "$status" = "0" ] && [ "$(log_says_no_tests_ran "$log")" = "yes" ]; then
    echo "no tests matched"
    echo
    echo "soak: REFUSING — the filter selected no tests, so this would be a tally of iterations in"
    echo "  which the engine was never entered. That is the false clear in its purest form."
    echo "    filter: $filter"
    echo "    log:    $log"
    keep_logs=1
    exit 1
  fi

  host_died="$(log_says_host_died "$log")"
  if [ "$status" -ge 128 ]; then host_died="yes"; fi
  clr_signature="$(log_says_clr_error "$log")"

  kernel_signature="no"
  new_kernel_lines=""
  if [ "$dmesg_available" = "yes" ]; then
    dmesg >"$after_dmesg" 2>/dev/null || true
    new_kernel_lines="$(kernel_lines_since "$before_dmesg" "$after_dmesg")"
    kernel_signature="$(lines_say_overflow "$new_kernel_lines")"
  fi

  classify_run "$status" "$host_died" "$clr_signature" "$kernel_signature"
  outcome="$RUN_OUTCOME"
  detail="$RUN_DETAIL"

  case "$outcome" in
    CLEAN)
      echo "clean"
      # A KERNEL MESSAGE DURING AN ITERATION THAT PASSED IS THE FINDING, NOT
      # NOISE. DW-396's own history is exactly that: the suite looked fine and
      # the kernel had already said `overflowed sigaltstack`. Printing these
      # only in the failure branch discarded the one observation this tool is
      # most specifically here to catch.
      if [ -n "$new_kernel_lines" ]; then
        echo "    ⚠ the iteration PASSED, and the kernel still said:"
        printf '%s\n' "$new_kernel_lines" | sed 's/^/      /'
        kernel_notes="$kernel_notes
  iteration $i (the suite passed): $(printf '%s\n' "$new_kernel_lines" | join_with ' | ')
    log: $log"
        if [ "$kernel_signature" = "yes" ]; then
          kernel_overflow_seen=1
          echo "      ^ that is DW-396's NAMED signature. The soak is not clean, whatever the"
          echo "        exit codes say; the run continues so the whole picture is on record."
        fi
      fi
      ;;
    *)
      echo "$outcome"
      echo "    $detail"
      failures="$failures
  iteration $i: $outcome — $detail
    log: $log"
      if [ -n "$new_kernel_lines" ]; then
        echo "    kernel said, during this iteration:"
        printf '%s\n' "$new_kernel_lines" | sed 's/^/      /'
        failures="$failures
    kernel: $(printf '%s\n' "$new_kernel_lines" | join_with ' | ')"
      fi
      # ANY CRASH ENDS THE SOAK. Continuing past one would be collecting
      # iterations after the question has already been answered, and on a host
      # whose kernel has just logged a fatal signal.
      break
      ;;
  esac
done

# THE KERNEL'S WORD OUTRANKS A CLEAN EXIT CODE. Per iteration nothing died, so
# each one really was CLEAN; over the run the named signature was present, and
# that is a different verdict in both modes -- a failed soak, and a validated
# harness on the reproduction leg.
if [ "$outcome" = "CLEAN" ] && [ "$kernel_overflow_seen" = "1" ]; then
  outcome="KERNEL-OVERFLOW-ONLY"
fi

echo

# ----------------------------------------------------------- the verdict

# A DEATH WITH NO NAME GOES TO THE MECHANISM REPRODUCER, ON THIS HOST, NOW.
# Linux only: the reproducer is, and so is the defect.
# ...AND SO DOES A PRE-FIX BINDING THAT STAYED CLEAN. Then the numbers say
# whether the mechanism is live on this host at all (the workload was too
# GC-light to show it) or not (this host cannot validate anything), which
# are different next steps; neither validates the harness.
if [ "$mode" = "reproduce" ] && { [ "$outcome" = "CRASH-UNKNOWN" ] || [ "$outcome" = "CLEAN" ]; } && [ "$(uname -s)" = "Linux" ]; then
  mechanism_check
  echo
fi

if [ "$mode" = "reproduce" ] && { [ "$outcome" = "DW396" ] || [ "$MECH_TAG" = "ATTRIBUTED" ]; }; then
  record_reproduction || true
  validated="yes"
fi

final_verdict "$mode" "$completed" "$outcome" "$validated" "$dmesg_available"

# ANYTHING BUT A CLEAN RUN KEEPS ITS LOGS. The run that finally catches the
# defect is the run whose evidence must not be swept up by the exit trap, and
# the failure block below names those files by path.
if [ "$outcome" != "CLEAN" ]; then keep_logs=1; fi

echo "=== VERDICT: $FINAL_TAG ==="
echo
printf '  %s\n' "$FINAL_TEXT"
echo
echo "  mode      : $mode$([ "$mode" = reproduce ] && echo " (the validation leg — the PRE-FIX binding)" || echo " (the soak)")"
echo "  binding   : $binding_desc"
echo "  native    : $native"
echo "              sha256 $native_sha, $native_arch"
echo "  host      : $(uname -n), $(uname -s) $(uname -r), $arch"
echo "  translation: $VERDICT_SUMMARY"
echo "  iterations: $completed of $iterations requested"
echo "  started   : $started"
echo "  finished  : $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "  tool      : folio-dotnet/build/soak.sh, sha256 $tool_sha"
if [ -n "$failures" ]; then
  echo "  failures  :$failures"
else
  echo "  failures  : none"
fi
if [ -n "$kernel_notes" ]; then
  echo "  kernel, during iterations that PASSED:$kernel_notes"
fi
if [ "$logs_are_temporary" = "0" ]; then
  echo "  logs      : $logs"
elif [ "$keep_logs" = "1" ]; then
  echo "  logs      : $logs"
  echo "              KEPT because this run was not clean — nothing here is swept up on exit."
  echo "              They are in a temporary directory, so move them somewhere they will survive"
  echo "              a reboot before writing this run down."
else
  echo "  logs      : discarded — the run was clean and no --log-dir was given. Pass --log-dir for"
  echo "              any run whose result gets written down."
fi
echo
echo "  Quote this verdict with the block above attached. A soak figure without its host,"
echo "  architecture and translation verdict is the reading DW-396 has already recorded twice."

exit "$FINAL_STATUS"
