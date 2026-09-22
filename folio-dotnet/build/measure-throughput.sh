#!/usr/bin/env bash
#
# CAP-10, measured: what did routing every render through the engine-thread
# boundary cost a caller, and does the pool actually buy parallelism?
#
#   ./measure-throughput.sh                # this host (Linux or macOS)
#   ./measure-throughput.sh amd64          # linux/amd64 in a container
#   ./measure-throughput.sh arm64          # linux/arm64 in a container
#
# IT IS A MEASUREMENT, NOT A GATE. No CI job runs it, it bakes in no pass/fail
# threshold, and a human reads the table. CAP-10 is non-regression: the
# question is whether throughput SURVIVED the change, not whether it beat it.
#
# WHAT MAKES IT A COMPARISON RATHER THAN A NUMBER. The owner chose a POOL of
# engine threads over a single one specifically so throughput would survive,
# which makes "it survived" an acceptance criterion rather than a curiosity.
# Two distinct risks are unmeasured without this: that Windows, which took the
# change without needing it, pays for it; and that Linux flattens at one core,
# which would mean the pool is not buying the parallelism it was chosen for.
# So the harness runs TWICE — once against the binding as it is, once against
# the binding as it was BEFORE the engine threads landed — and the two are
# printed side by side.
#
# THE BASELINE IS REAL CODE, NOT A REMEMBERED NUMBER. It comes from a git
# worktree checked out at the parent of the engine-thread commit, and the
# harness links against THAT tree's Folio8.csproj. The alternative — a bypass
# switch inside the shipped binding — was rejected: it would add surface to
# src/Folio8 that exists only for a benchmark. src/Folio8 is not touched by
# this tool at all, and if it ever needs to be, the tool is wrong.
#
# THE ENGINE IS HELD FIXED WHILE THE BINDING VARIES. Both legs stage the SAME
# native library file, from this tree. Letting each leg stage its own
# worktree's native would vary two things at once and the difference would
# mean nothing.
#
# ⚠ A DEVELOPER MACHINE IS INDICATIVE. It is not a controlled environment, and
# a translated host (Rosetta, qemu) measures the translator as much as the
# binding. The harness stamps the host, its architecture and whether it could
# tell it was translated into every run's header, and this script stamps what
# it knows that the harness cannot see from inside a container. Quote a number
# only with that block attached -- the same provenance discipline story 1
# established for the signal-stack readings.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
proj="$here/render-throughput"

# The commit that moved every crossing onto binding-owned engine threads. ITS
# PARENT IS THE BASELINE -- the binding with the direct-DllImport path, which
# is the thing 1.1.0 shipped on Windows and the thing CAP-10 asks us not to
# have regressed against.
engine_threads_commit="${THROUGHPUT_ENGINE_COMMIT:-7f6a936}"

usage() {
  cat <<'USAGE'
measure-throughput.sh — CAP-10's before/after render-throughput comparison

  ./measure-throughput.sh                # this host (Linux or macOS)
  ./measure-throughput.sh amd64          # linux/amd64 in a container
  ./measure-throughput.sh arm64          # linux/arm64 in a container
  ./measure-throughput.sh --help

Environment overrides:
  THROUGHPUT_SECONDS         timed seconds per concurrency level (default 5)
  THROUGHPUT_WARMUP          discarded seconds per level (default 1)
  THROUGHPUT_FIXTURE         fixtures/<slug> to render (default multi-page-statement)
  THROUGHPUT_LEVELS          comma-separated concurrency levels
                             (default 1,2,4,ProcessorCount -- pass e.g.
                             1,2,4,8,32 to probe well above the engine pool)
  THROUGHPUT_REPEATS         how many times to run each leg, ALTERNATING before
                             and after (default 1). Two or more gives the
                             comparison a spread column, which is what
                             separates a real difference from host drift.
  THROUGHPUT_LOG_DIR         keep every leg and build log in this directory
                             instead of discarding them with the work tree.
                             Use it for any run whose numbers get written down.
  THROUGHPUT_ENGINE_COMMIT   the engine-thread commit whose PARENT is the baseline
  THROUGHPUT_SDK_IMAGE       the .NET SDK image the container legs use
  THROUGHPUT_HOST_CONTEXT    free-text provenance stamped into both headers

It prints each leg's provenance-stamped table and then a side-by-side
comparison. It has no pass/fail: a human reads the numbers.
USAGE
}

# The SDK image the container legs use. A TAG, NOT A DIGEST, for the same
# reason probe-signal-stack.sh uses one: nothing here is shipped, and each run
# prints the runtime it actually ran on, so the provenance travels with the
# measurement rather than with the script.
image="${THROUGHPUT_SDK_IMAGE:-mcr.microsoft.com/dotnet/sdk:8.0}"

seconds="${THROUGHPUT_SECONDS:-5}"
warmup="${THROUGHPUT_WARMUP:-1}"
fixture="${THROUGHPUT_FIXTURE:-multi-page-statement}"
levels="${THROUGHPUT_LEVELS:-}"
host_context="${THROUGHPUT_HOST_CONTEXT:-}"
repeats="${THROUGHPUT_REPEATS:-1}"
log_dir="${THROUGHPUT_LOG_DIR:-}"

# VALIDATE BEFORE DOING ANY WORK -- nothing is built, no worktree is created
# and no image is pulled until the whole argument list is known good. A run
# that spends two minutes building and then complains about an argument has
# wasted exactly the thing it was asked to produce.
if [ "$#" -gt 1 ]; then
  echo "measure-throughput: one argument at most (amd64, arm64, --help, or none for this host)" >&2
  exit 1
fi

target="${1:-host}"
case "$target" in
  host|amd64|arm64) ;;
  --help|-h)
    usage
    exit 0
    ;;
  *)
    echo "measure-throughput: unknown target '$target' (amd64, arm64, --help, or no argument for this host)" >&2
    exit 1
    ;;
esac

# LOOKING LIKE A NUMBER IS NOT BEING ONE, and the difference is minutes. A
# shape-only guard passes `0` and `0.0` while its own message says "positive",
# and the harness then rejects them AFTER a worktree checkout and two Release
# builds -- at which point the failure reads as "the binding did not complete
# the run (exit 2)" and points at the binding rather than at the argument. Every
# value this script forwards is therefore checked for its VALUE here, not only
# for its spelling.
positive_duration() {
  case "$1" in ''|*[!0-9.]*|.|*.*.*) return 1 ;; esac
  awk -v v="$1" 'BEGIN { exit (v + 0 > 0) ? 0 : 1 }'
}
nonnegative_duration() {
  case "$1" in ''|*[!0-9.]*|.|*.*.*) return 1 ;; esac
  return 0
}

if ! positive_duration "$seconds"; then
  echo "measure-throughput: THROUGHPUT_SECONDS must be a number greater than zero, not '$seconds'" >&2
  exit 1
fi
if ! nonnegative_duration "$warmup"; then
  echo "measure-throughput: THROUGHPUT_WARMUP must be a number of zero or more, not '$warmup'" >&2
  exit 1
fi
case "$repeats" in
  ''|*[!0-9]*)
    echo "measure-throughput: THROUGHPUT_REPEATS must be a whole number of one or more, not '$repeats'" >&2
    exit 1
    ;;
esac
if [ "$repeats" -lt 1 ]; then
  echo "measure-throughput: THROUGHPUT_REPEATS must be one or more, not '$repeats'" >&2
  exit 1
fi
case "$fixture" in
  ''|*/*)
    echo "measure-throughput: THROUGHPUT_FIXTURE must be a fixture slug such as 'multi-page-statement', not '$fixture'" >&2
    exit 1
    ;;
esac
if [ -n "$levels" ]; then
  # *,,* catches "1,,2", which the old digits-and-commas shape check let
  # through; each field is then checked as an integer of one or more, so "0"
  # cannot reach the harness either.
  case "$levels" in
    *[!0-9,]*|,*|*,|*,,*)
      echo "measure-throughput: THROUGHPUT_LEVELS must be comma-separated integers of one or more, not '$levels'" >&2
      exit 1
      ;;
  esac
  IFS=',' read -r -a _level_list <<<"$levels"
  for _level in "${_level_list[@]}"; do
    case "$_level" in
      ''|*[!0-9]*)
        echo "measure-throughput: THROUGHPUT_LEVELS holds '$_level', which is not a whole number" >&2
        exit 1
        ;;
    esac
    if [ "$_level" -lt 1 ]; then
      echo "measure-throughput: THROUGHPUT_LEVELS holds '$_level'; a concurrency level is one or more" >&2
      exit 1
    fi
  done
fi

if ! command -v git >/dev/null 2>&1; then
  echo "measure-throughput: git is not on PATH, and the baseline half of this comparison is a git worktree at the commit before the engine threads landed; install git and run this again" >&2
  exit 1
fi

root="$(cd "$here/../.." && pwd)"
if [ ! -d "$root/.git" ] && [ ! -f "$root/.git" ]; then
  echo "measure-throughput: $root is not a git working tree, so the baseline binding cannot be checked out; run this from a clone rather than from an extracted archive" >&2
  exit 1
fi
if [ ! -d "$root/fixtures/$fixture" ]; then
  echo "measure-throughput: there is no fixture at $root/fixtures/$fixture; set THROUGHPUT_FIXTURE to a directory under fixtures/" >&2
  exit 1
fi

# THE CONTAINER LEGS RE-ENTER THIS SAME SCRIPT, at the same absolute path, on
# the repository mounted read-write. Read-only would not do: the baseline leg
# writes a git worktree and both legs write build output. Running as the
# invoking uid keeps every file the container creates owned by the developer
# rather than by root, which is the difference between a clean tree afterwards
# and a sudo to clean up.
#
# safe.directory IS PASSED AS ENVIRONMENT, NOT WRITTEN AS CONFIG. A bind mount
# presents the repository with the host's ownership, which git inside the
# container reads as "dubious ownership" and refuses — and the refusal surfaces
# as "cannot resolve <commit>^", which reads like a shallow clone and sends the
# reader somewhere useless. GIT_CONFIG_COUNT scopes the exception to this one
# container and this one path; `git config --global` would leave it behind on
# whatever machine ran it.
case "$target" in
  amd64|arm64)
    if ! command -v docker >/dev/null 2>&1; then
      echo "measure-throughput: $target runs the harness in a container and needs docker on PATH; on a Linux box of that architecture run './measure-throughput.sh' with no argument instead" >&2
      exit 1
    fi
    # ON PATH IS NOT THE SAME AS RUNNING. Every other failure here names its
    # remedy; letting the daemon's own message through would be the one that
    # does not.
    if ! docker info >/dev/null 2>&1; then
      echo "measure-throughput: docker is installed but its daemon is not reachable — start Docker (Docker Desktop, or 'systemctl start docker') and run this again" >&2
      exit 1
    fi
    # THE WRAPPER KNOWS WHAT THE HARNESS CANNOT SEE. A process inside a
    # linux/<arch> container reads the EMULATED architecture from uname and has
    # no way to tell that Rosetta or qemu is underneath it — that is the trap
    # DW-396 fell into once already. Out here both halves are known, so the
    # verdict is computed and stamped rather than left to the reader's memory.
    # It is an argument about admissibility, not about speed: SPEC.md's soak
    # evidence excludes translated hosts outright, and a throughput number from
    # one is measuring the translator as much as the binding.
    case "$(uname -m)" in
      x86_64|amd64) host_arch="amd64" ;;
      aarch64|arm64) host_arch="arm64" ;;
      *) host_arch="$(uname -m)" ;;
    esac
    if [ "$target" = "$host_arch" ]; then
      translation="native on this host's silicon"
    else
      translation="TRANSLATED (linux/$target on a $host_arch host) — indicative only, and NOT admissible as evidence about real $target"
    fi
    echo "==> linux/$target: $image (container) — $translation"
    exec docker run --rm --platform "linux/$target" \
      --user "$(id -u):$(id -g)" \
      -v "$root":"$root" -w "$root" \
      -e HOME=/tmp -e DOTNET_CLI_HOME=/tmp \
      -e DOTNET_NOLOGO=1 -e DOTNET_CLI_TELEMETRY_OPTOUT=1 \
      -e GIT_CONFIG_COUNT=1 -e GIT_CONFIG_KEY_0=safe.directory -e GIT_CONFIG_VALUE_0="$root" \
      -e THROUGHPUT_SECONDS="$seconds" \
      -e THROUGHPUT_WARMUP="$warmup" \
      -e THROUGHPUT_FIXTURE="$fixture" \
      -e THROUGHPUT_LEVELS="$levels" \
      -e THROUGHPUT_REPEATS="$repeats" \
      -e THROUGHPUT_LOG_DIR="$log_dir" \
      -e THROUGHPUT_ENGINE_COMMIT="$engine_threads_commit" \
      -e THROUGHPUT_HOST_CONTEXT="linux/$target container on $(uname -s)/$(uname -m); $translation${host_context:+ — $host_context}" \
      "$image" "$here/measure-throughput.sh"
    ;;
esac

if ! command -v dotnet >/dev/null 2>&1; then
  echo "measure-throughput: the .NET SDK is not on PATH (https://dotnet.microsoft.com/download); the amd64 and arm64 targets bring their own inside the container" >&2
  exit 1
fi

# WHICH NATIVE BOTH LEGS STAGE. On Linux the SHIPPED artifact for this
# architecture is preferred over the host build: measuring what the package
# actually carries is the point, and build/native/host/ on a developer's box
# may be a different toolchain's output. Either way ONE file is chosen here
# and handed to both legs.
case "$(uname -s)" in
  Darwin) native="$root/folio-dotnet/build/native/host/libfolio8_native.dylib" ;;
  Linux)
    case "$(uname -m)" in
      x86_64|amd64) rid="linux-x64" ;;
      aarch64|arm64) rid="linux-arm64" ;;
      *) rid="" ;;
    esac
    native="$root/folio-dotnet/build/native/host/libfolio8_native.so"
    if [ -n "$rid" ] && [ -f "$root/folio-dotnet/build/native/$rid/libfolio8_native.so" ]; then
      native="$root/folio-dotnet/build/native/$rid/libfolio8_native.so"
    fi
    ;;
  *)
    echo "measure-throughput: this wrapper drives bash and expects Linux or macOS; on Windows run the harness directly with 'dotnet run --project folio-dotnet/build/render-throughput'" >&2
    exit 1
    ;;
esac
if [ ! -f "$native" ]; then
  echo "measure-throughput: no native library at $native. Run folio-dotnet/build/build-native.sh (host) or build-native.sh linux-x64 linux-arm64 first; without it the harness can only measure a DllNotFoundException." >&2
  exit 1
fi

# THE COPY BELOW IS A FLAT GLOB, SO SAY SO WHEN IT WOULD BE WRONG. Each leg
# gets its OWN copy of the harness sources so the two builds cannot share an
# obj/ and hand one leg the other's Folio8.dll -- which would produce two
# measurements that look comparable and are not, this story's whole subject. A
# source file added in a subdirectory would be dropped in silence.
nested="$(find "$proj" -mindepth 2 -name '*.cs' -not -path '*/obj/*' -not -path '*/bin/*' 2>/dev/null || true)"
if [ -n "$nested" ]; then
  echo "measure-throughput: each leg copies only the harness's top-level sources, but these are in subdirectories and would be silently dropped:" >&2
  echo "$nested" | sed 's/^/  /' >&2
  echo "  Teach the 'cp' line in run_leg() about them before running this again." >&2
  exit 1
fi

if ! baseline="$(git -C "$root" rev-parse --verify "${engine_threads_commit}^" 2>/dev/null)"; then
  echo "measure-throughput: cannot resolve ${engine_threads_commit}^, the commit before the engine threads landed, in $root." >&2
  echo "  A shallow clone is the usual cause — run 'git -C $root fetch --unshallow', or set THROUGHPUT_ENGINE_COMMIT to the engine-thread commit in this history." >&2
  echo "  Refusing to measure: without the baseline there is nothing to compare against, and an after-only number is not a comparison." >&2
  exit 1
fi

work="$(mktemp -d "${TMPDIR:-/tmp}/folio-throughput.XXXXXX")"
tree="$work/baseline-tree"

# WHERE THE LOGS GO, AND WHY IT IS NOT ALWAYS THE WORK TREE. Every leg's stdout
# and every build log used to die with the temporary directory, which made the
# recorded tables in _bmad-output retyped from a terminal that no longer
# existed. THROUGHPUT_LOG_DIR keeps them; use it for any run whose numbers get
# written down, so the record has a file behind it rather than a memory.
logs="$work"
if [ -n "$log_dir" ]; then
  mkdir -p "$log_dir"
  logs="$(cd "$log_dir" && pwd)"
fi

cleanup() {
  rm -rf "$work" 2>/dev/null || true
  git -C "$root" worktree prune >/dev/null 2>&1 || true
}
trap cleanup EXIT

# WHAT THE "after" LEG ACTUALLY IS. It links the WORKING TREE's Folio8.csproj,
# not a checkout of HEAD — deliberately, because the point of running this
# during development is to measure the change you are holding. But then calling
# the column "HEAD" would be a lie the moment src/Folio8 is dirty, and an
# uncommitted edit would be recorded as a property of a commit. So the leg is
# described by what it is, and a dirty binding is said out loud rather than
# inferred by the reader.
head_sha="$(git -C "$root" rev-parse --short HEAD 2>/dev/null || echo '?')"
if [ -n "$(git -C "$root" status --porcelain -- folio-dotnet/src/Folio8 2>/dev/null)" ]; then
  after_dirty=1
  after_desc="the WORKING TREE — HEAD $head_sha PLUS UNCOMMITTED CHANGES under folio-dotnet/src/Folio8"
else
  after_dirty=0
  after_desc="HEAD $head_sha (folio-dotnet/src/Folio8 has no uncommitted changes)"
fi

echo "==> baseline worktree: $(git -C "$root" log --oneline -1 "$baseline")"
echo "==> current binding:   $after_desc"
if [ "$after_dirty" = "1" ]; then
  echo
  echo "⚠ measure-throughput: the 'after' leg is NOT a commit. folio-dotnet/src/Folio8 has" >&2
  echo "  uncommitted changes, so these numbers describe your working tree and must not be" >&2
  echo "  recorded against $head_sha. Commit or stash before taking a run for the record." >&2
  echo
fi
if ! git -C "$root" worktree add --detach "$tree" "$baseline" >"$logs/worktree.log" 2>&1; then
  echo "measure-throughput: could not create the baseline git worktree at $tree:" >&2
  sed 's/^/  /' "$logs/worktree.log" >&2
  echo "  Refusing to measure: an after-only number is not a comparison." >&2
  exit 1
fi

# BOTH LEGS ARE BUILT BEFORE EITHER IS RUN. A compile failure is not a
# measurement, and finding one after the first leg has already run would leave a
# half-comparison on the screen. It also means the repeat loop below alternates
# runs only, with no build between them to disturb the machine.
build_leg() {
  local leg="$1"
  local binding="$2"
  local legdir="$work/$leg"
  mkdir -p "$legdir"
  cp "$proj"/*.csproj "$proj"/*.cs "$legdir/"

  echo "==> building the $leg harness against $binding"
  if ! dotnet build "$legdir"/*.csproj -c Release -v quiet --nologo \
      -p:FolioBindingProject="$binding" \
      -p:FolioNativeFile="$native" >"$logs/$leg.build.log" 2>&1; then
    sed 's/^/  /' "$logs/$leg.build.log" >&2
    return 1
  fi
  if [ -z "$(find "$legdir/bin" -name 'render-throughput.dll' -print -quit 2>/dev/null || true)" ]; then
    echo "measure-throughput: the $leg harness built but produced no render-throughput.dll under $legdir/bin" >&2
    return 1
  fi
  return 0
}

# Runs one leg once. Stdout is teed so the reader sees each leg's own provenance
# block as it happens, and kept so the tables can be joined after.
run_leg() {
  local leg="$1"
  local repeat="$2"
  local dll status
  dll="$(find "$work/$leg/bin" -name 'render-throughput.dll' -print -quit)"

  echo
  echo "==> running the $leg leg (run $repeat of $repeats)"
  # An ARRAY, not a ${var:+...} expansion: --host-context carries spaces, and
  # an unquoted conditional expansion would split it into arguments the
  # harness would reject as unknown.
  local -a argv=(
    --repo "$root"
    --fixture "$fixture"
    --seconds "$seconds"
    --warmup "$warmup"
    --label "$leg"
  )
  if [ -n "$levels" ]; then argv+=(--levels "$levels"); fi
  if [ -n "$host_context" ]; then argv+=(--host-context "$host_context"); fi
  # THE BASELINE BINDING HAS NO ENGINE POOL. Letting the harness annotate its
  # table with "above the engine pool" would describe the direct-DllImport path
  # in terms of a mechanism it does not have.
  if [ "$leg" = "before" ]; then argv+=(--pool none); fi

  set +e
  dotnet "$dll" "${argv[@]}" 2>&1 | tee "$logs/$leg.$repeat.log"
  status="${PIPESTATUS[0]}"
  set -e
  return "$status"
}

# HOW A LEG DIED IS NOT ONE QUESTION, AND ANSWERING IT AS ONE IS THE EXACT
# MISATTRIBUTION THE REST OF THIS EPIC EXISTS TO PREVENT.
#
#   exit 1 or 2  the HARNESS refused: a bad argument, a missing fixture, or a
#                render whose digest did not match expected.json. It said why,
#                on stderr, and it is not a crash.
#   exit >= 128  the process was KILLED BY A SIGNAL (128 + signum). On Linux
#                this is the shape DW-396 takes, and it is the ONLY exit status
#                that may be narrated that way.
#   anything else  unknown. Say the number and attribute nothing.
describe_exit() {
  local status="$1"
  if [ "$status" -ge 128 ]; then
    echo "killed by signal $((status - 128)) (exit $status)"
  elif [ "$status" = "1" ] || [ "$status" = "2" ]; then
    echo "refused by the harness itself (exit $status)"
  else
    echo "exit $status"
  fi
}

if ! build_leg before "$tree/folio-dotnet/src/Folio8/Folio8.csproj"; then
  echo >&2
  echo "measure-throughput: the baseline binding at $baseline could not be BUILT (log above)." >&2
  echo "  Refusing to measure: CAP-10 is a non-regression claim, and an after-only number is not a comparison." >&2
  echo "  If the baseline tree needs something this one has, fix that first — do not add a bypass switch to src/Folio8 to avoid it." >&2
  exit 1
fi
if ! build_leg after "$root/folio-dotnet/src/Folio8/Folio8.csproj"; then
  echo >&2
  echo "measure-throughput: the CURRENT binding could not be BUILT (log above)." >&2
  echo "  A compile failure is not a measurement — nothing was run, and there is nothing to read." >&2
  echo "  Fix the build and run this again." >&2
  exit 1
fi

# THE LEGS ALTERNATE, RUN BY RUN. Running all of before and then all of after
# would let any drift on the host over the intervening minutes — thermal,
# another process waking up, the page cache warming — land entirely on one
# column and be read as the binding's doing. Alternating spreads that across
# both, and the spread column below is what lets a reader see how much of it
# there was.
baseline_status=0
after_status=0
for repeat in $(seq 1 "$repeats"); do
  run_leg before "$repeat" || baseline_status=$?
  if [ "$baseline_status" != "0" ]; then break; fi
  run_leg after "$repeat" || after_status=$?
  if [ "$after_status" != "0" ]; then break; fi
done

# A BASELINE THAT DIED STILL LEAVES THE AFTER LEG WORTH RUNNING, and the loop
# above broke before it got there. Without this the "AFTER ONLY" block below
# would announce an after table that was never produced -- a heading describing
# output that does not exist. A refusal (1 or 2) is excluded: that is a setup
# fault which would hit both legs identically, so running the second one only
# repeats the same message.
if [ "$baseline_status" != "0" ] && [ "$baseline_status" != "1" ] && [ "$baseline_status" != "2" ]; then
  echo
  echo "==> the baseline leg did not finish; running the current binding alone so there is"
  echo "    at least something to read. What comes out is NOT a comparison."
  run_leg after 1 || after_status=$?
fi

# A HARNESS THAT REFUSED IS NOT A CRASH, on either leg. Its message is already
# on stderr and says what was wrong; repeating it as a signal death would
# manufacture evidence for the defect this epic is closing.
for leg_status in "$baseline_status" "$after_status"; do
  if [ "$leg_status" = "1" ] || [ "$leg_status" = "2" ]; then
    echo >&2
    echo "measure-throughput: a leg was $(describe_exit "$leg_status") — a bad argument, a missing" >&2
    echo "  fixture, or a render whose digest did not match expected.json. Its own message is above." >&2
    echo "  This is NOT a crash and says nothing about DW-396. Nothing is reported." >&2
    exit 1
  fi
done

if [ "$after_status" != "0" ]; then
  echo >&2
  echo "measure-throughput: the CURRENT binding did not complete the run — $(describe_exit "$after_status")." >&2
  if [ "$after_status" -ge 128 ]; then
    echo "  A signal death here is a result, not a benchmark failure. Read the output above," >&2
    echo "  and see the signal note the baseline branch prints for which signals are, and are" >&2
    echo "  not, the DW-396 shape." >&2
  else
    echo "  The cause is not known from the exit status alone; nothing is attributed to it here." >&2
  fi
  exit 1
fi

echo
if [ "$baseline_status" != "0" ]; then
  echo "=== AFTER ONLY — THIS IS NOT A COMPARISON ==="
  echo
  echo "  The baseline binding built, started, and did not finish: $(describe_exit "$baseline_status")."
  if [ "$baseline_status" -ge 128 ]; then
    signal=$((baseline_status - 128))
    echo
    # WHICH SIGNAL IT WAS IS THE WHOLE QUESTION, and "it died" is not an
    # answer. DW-396 arrives as SIGSEGV (11) — the kernel converting an
    # overflowed alternate signal stack — or as the SIGABRT (6) a runtime
    # raises on the way down. Any OTHER signal is a different event, and
    # narrating it as DW-396 would be manufacturing exactly the evidence this
    # epic spent two wrong readings learning not to manufacture.
    case "$signal" in
      11|6|7)
        echo "  Signal $signal on Linux IS the shape of DW-396: the pre-fix binding enters the"
        echo "  engine on ordinary CLR threads, whose alternate signal stack is too small for"
        echo "  Go's handlers, and the kernel turns the overflow into SIGSEGV. Check 'dmesg'"
        echo "  for an 'overflowed sigaltstack' line to CONFIRM it — the exit status names the"
        echo "  signal, never its cause."
        ;;
      *)
        echo "  Signal $signal is NOT the shape of DW-396, which arrives as SIGSEGV (11) or as"
        echo "  a SIGABRT (6) on the way down. Nothing here is attributed to the sigaltstack"
        echo "  defect. Read the leg's own output above for what it printed before it died."
        ;;
    esac
  else
    echo
    echo "  That status is not a signal death and not a refusal the harness explained, so"
    echo "  nothing is attributed to it here. Read the leg's output above."
  fi
  echo
  echo "  No before column exists, so none is printed. A Windows host, where the"
  echo "  sigaltstack defect cannot fire at all, is where CAP-10's before/after comparison"
  echo "  can be taken if this host cannot hold the baseline up."
  exit 1
fi

echo "=== before / after, over $repeats run(s) per leg ==="
echo "  before = $(git -C "$root" log --oneline -1 "$baseline")"
echo "  after  = $after_desc"
echo
# Joined on the concurrency level over every RESULT line both legs emitted.
# awk rather than a shell loop because the join, the averaging and the union of
# levels are the whole job, and a loop doing them would be longer and no clearer.
awk -F'\t' '
  /^RESULT\t/ {
    lab = $2; c = $3 + 0
    if (!(c in seen)) { seen[c] = 1; levels[++nl] = c }
    k = lab SUBSEP c
    n[k]++; rps[k] += $6; med[k] += $8; p95[k] += $9
    if (!(k in lo) || $6 + 0 < lo[k]) lo[k] = $6 + 0
    if (!(k in hi) || $6 + 0 > hi[k]) hi[k] = $6 + 0
  }
  function avg(k, field) { return n[k] > 0 ? field[k] / n[k] : 0 }
  function spread(k,   a) {
    if (n[k] < 2) return "n=1"
    a = avg(k, rps)
    return a > 0 ? sprintf("+/-%.1f%%", (hi[k] - lo[k]) / 2 / a * 100) : "n/a"
  }
  END {
    # THE UNION OF LEVELS, not just the levels one leg produced. A level that one leg
    # produced and the other did not is a fact about the run and is printed as
    # a gap rather than dropped from the table.
    for (i = 2; i <= nl; i++) { v = levels[i]; j = i - 1
      while (j >= 1 && levels[j] > v) { levels[j + 1] = levels[j]; j-- }
      levels[j + 1] = v }

    printf "  %-11s %11s %9s %11s %9s %9s\n", "concurrency", "before r/s", "spread", "after r/s", "spread", "change"
    printf "  %-11s %11s %9s %11s %9s %9s\n", "-----------", "----------", "------", "---------", "------", "------"
    gaps = ""
    for (i = 1; i <= nl; i++) {
      c = levels[i]; kb = "before" SUBSEP c; ka = "after" SUBSEP c
      hb = (kb in n); ha = (ka in n)
      if (!hb || !ha) gaps = gaps sprintf("  level %s is missing from the %s leg.\n", c, (hb ? "after" : "before"))
      ab = hb ? avg(kb, rps) : 0; aa = ha ? avg(ka, rps) : 0
      change = (hb && ha && ab > 0) ? sprintf("%+.1f%%", (aa - ab) / ab * 100) : "-"
      printf "  %-11s %11s %9s %11s %9s %9s\n", c,
        (hb ? sprintf("%.2f", ab) : "-"), (hb ? spread(kb) : "-"),
        (ha ? sprintf("%.2f", aa) : "-"), (ha ? spread(ka) : "-"), change
    }

    # MEDIAN AND p95 TOO, because the mean alone hid the most load-bearing
    # reading this harness has produced: a leg whose median render was unchanged
    # and whose p95 had doubled -- a handoff tail, not a slower render. A table
    # that drops them cannot tell those two apart.
    printf "\n  %-11s %11s %11s %11s %11s\n", "concurrency", "before med", "after med", "before p95", "after p95"
    printf "  %-11s %11s %11s %11s %11s\n", "-----------", "----------", "---------", "----------", "---------"
    for (i = 1; i <= nl; i++) {
      c = levels[i]; kb = "before" SUBSEP c; ka = "after" SUBSEP c
      printf "  %-11s %11s %11s %11s %11s\n", c,
        ((kb in n) ? sprintf("%.2f", avg(kb, med)) : "-"),
        ((ka in n) ? sprintf("%.2f", avg(ka, med)) : "-"),
        ((kb in n) ? sprintf("%.2f", avg(kb, p95)) : "-"),
        ((ka in n) ? sprintf("%.2f", avg(ka, p95)) : "-")
    }
    if (gaps != "") { printf "\n%s", gaps }

    if (nl > 0) {
      first = levels[1]; last = levels[nl]
      kaf = "after" SUBSEP first; kal = "after" SUBSEP last
      kbf = "before" SUBSEP first; kbl = "before" SUBSEP last
      printf "\n  Scaling (each leg divided by its OWN concurrency-%s figure):\n", first
      if ((kaf in n) && (kal in n) && avg(kaf, rps) > 0)
        printf "    after:  concurrency %s over concurrency %s is %.2fx\n", last, first, avg(kal, rps) / avg(kaf, rps)
      if ((kbf in n) && (kbl in n) && avg(kbf, rps) > 0)
        printf "    before: concurrency %s over concurrency %s is %.2fx\n", last, first, avg(kbl, rps) / avg(kbf, rps)
      printf "    These two are NOT comparable with each other: each divides by its own\n"
      printf "    concurrency-%s throughput, so a leg that is slower at %s scores a HIGHER\n", first, first
      printf "    multiple for the same absolute throughput. Read the change column for that.\n"
    }
  }
' "$logs"/before.*.log "$logs"/after.*.log

echo
if [ "$repeats" -lt 2 ]; then
  echo "  SPREAD IS UNMEASURED at THROUGHPUT_REPEATS=1. This host's run-to-run drift is"
  echo "  several percent, so a single-run difference of a few percent is not a signal."
  echo "  Set THROUGHPUT_REPEATS=3 or more before writing any of these numbers down."
fi
echo "  A developer machine is INDICATIVE. Both legs ran on the host stamped in the"
echo "  headers above, against the same native library, on the same fixture, alternating"
echo "  run by run. Record those lines with any number quoted from this table."
if [ -n "$log_dir" ]; then
  echo "  Logs kept in $logs"
else
  echo "  Logs are being discarded — set THROUGHPUT_LOG_DIR to keep them for the record."
fi
