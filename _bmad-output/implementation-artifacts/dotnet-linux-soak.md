# CAP-5: the soak, and the reproduction that has to come first

| | |
|---|---|
| **Spec** | `_bmad-output/specs/spec-dotnet-linux/SPEC.md` — **CAP-5** |
| **Story** | `_bmad-output/specs/spec-dotnet-linux/stories/5-the-soak.md` |
| **Tool** | `folio-dotnet/build/soak.sh` |
| **Binding under soak** | `HEAD` — the engine-thread binding from `7f6a936` onward |
| **Pre-fix binding** | `7f6a936^` = `86e7e5a` — every crossing on an arbitrary CLR thread-pool thread |
| **Evidence** | `evidence/dotnet-linux-soak/` |
| **Opened** | 2026-09-23 (local; the runner stamps UTC, so its logs read 2026-09-22T18:4xZ) |

> **NO LEG OF CAP-5 HAS BEEN RUN YET.** Everything in *Legs* below is **PENDING**,
> and the owner runs all of it: the soak's whole value is that a person watched
> real hardware do it. What *has* been done here is the runner, and a
> tool-verification pass that proves the runner works and refuses correctly — on
> a Docker Desktop VM, which is **not** one of the two hardware legs and is
> marked as such everywhere it appears.
>
> **A clean run from this tool is not a pass, by construction.** The runner
> reports `UNVALIDATED` and exits non-zero until a reproduction leg on the **same
> architecture** has caught the defect. DW-396 records 0-crashes-in-11 being read
> as a clear and the same shipped native overflowing sigaltstack afterwards; this
> is the one inference the tool refuses to let anybody make again.

---

## What the runner does, and what makes it different from "the suite passed"

```sh
folio-dotnet/build/soak.sh --reproduce              # validation leg: the PRE-FIX binding
folio-dotnet/build/soak.sh --iterations 100         # the soak: HEAD's binding
folio-dotnet/build/soak.sh --self-check             # the tool's own judgement, any OS
```

Four properties carry the whole claim:

1. **It refuses a host whose answers cannot be trusted, and decides that
   itself.** The translation verdict is story 1's derivation
   (`folio-dotnet/build/signal-stack-probe/Program.cs`) re-expressed in shell:
   `uname -m` against this process's own **ELF `e_machine`**, `/proc/cpuinfo`'s
   `vendor_id`, the Rosetta marker, and a `binfmt_misc` interpreter registered
   for our own architecture. `TRANSLATED`, `SUSPECT` **and** `UNKNOWN` are all
   refused, each naming the signal. **There is no override flag** — an override
   is how a translated tally gets into a record with a footnote nobody reads,
   which has already happened twice in DW-396.
2. **It will not call a clean run a pass until it has been shown able to fail.**
   A reproduction leg that catches the defect writes a line to a ledger, and a
   soak with no matching line prints `UNVALIDATED` and exits non-zero. A row
   validates **one architecture against one native library** — matched on both
   when it is read back — so a reproduction taken months ago against a different
   `.so` cannot quietly validate today's soak; host and kernel are recorded and
   reported beside it but not matched on, so a kernel upgrade weakens a claim
   visibly rather than erasing it. amd64 and arm64 never stand in for each other:
   the spike measured 16 KiB of altstack against 24 KiB for the same handler
   frame, so arm64 was never clear, only wider-margined.
   **A reproduction leg must run a pre-fix binding, and the tool checks rather
   than asks** — the binding has to be an ancestor of `7f6a936^`, because
   otherwise `--reproduce --binding head` is a front door to the very inference
   this tool exists to refuse.
3. **It watches the kernel, not only the exit code.** A dead test host is read
   out of vstest's own words (`Test host process crashed`), because vstest
   survives its child and exits 1 — indistinguishable by status from a failed
   assertion. The two **named** signatures are then looked for separately:
   `overflowed sigaltstack` among the lines `dmesg` gained *during that
   iteration*, and `Internal CLR error (0x80131506)` in the log. A dead host with
   **neither** is reported as `CRASH-UNKNOWN` and is not attributed to DW-396.
   **An `overflowed sigaltstack` printed during an iteration that PASSED is the
   finding, not noise** — that is DW-396's own history — so it is printed as it
   happens, carried into the verdict block, and turns a clean soak into
   `FAILED — DW-396 SIGNATURE, HOST SURVIVED` (and, on the reproduction leg, into
   a validated harness).
4. **It is not a CI gate and no workflow invokes it.** A soak long enough to mean
   anything does not belong on every commit, and a short one would recreate the
   false clear. `--iterations` at or below 11 is warned about by that number, in
   both modes.
5. **It refuses the ways a run can be empty or misattributed rather than counting
   them.** A dirty `src/Folio8` or `test/Folio8.Tests` (a result cannot be
   indexed by a commit that does not contain it), a native whose ELF `e_machine`
   is not this host's, and a `--filter` that selects no tests — measured while
   verifying the tool: a mistyped filter gave three "clean" iterations that never
   crossed the ABI once.
6. **Every caveat travels in the same stream as the number.** The refusals, the
   warnings and the verdict all go to stdout, so `soak.sh -n 100 > record.log`
   cannot keep the figure and lose the conditions. An interrupt prints a partial
   verdict and exits 130 rather than vanishing, and any run that is not clean
   keeps its logs and names the directory.

It does not touch `folio-dotnet/src/Folio8`. A non-HEAD binding is a `git
worktree`, exactly as `measure-throughput.sh`'s baseline leg is, and **both legs
stage the same native file**, so the engine is held fixed while the binding
varies.

---

## Legs — amd64 twice on GitHub's amd64 runners; arm64 on the owner's Apple Silicon and on GitHub's arm runner

Each leg is two runs, in this order, and the order is the point:

```sh
# 1. VALIDATE THE HARNESS: the pre-fix binding must die here.
folio-dotnet/build/soak.sh --reproduce --log-dir ./soak-logs/reproduce

# 2. ONLY THEN, THE SOAK.
folio-dotnet/build/soak.sh --iterations 100 --log-dir ./soak-logs/soak
```

| Leg | Host | Reproduction (pre-fix `86e7e5a`) | Soak (HEAD) | Where |
|---|---|---|---|---|
| **real amd64**, binding-only fix (managed restore after the first export; native `9f1296a3…`) | GitHub runner `runnervmtr4k5`, Linux 6.17.0-1022-azure, x86_64, translation verdict NATIVE, `DOTNET_GCgen0size=0x100000` | **REPRODUCED** — died on iteration 95 with the kernel's `overflowed sigaltstack`; 150 control iterations clean | **CLEAN — VALIDATED HARNESS**, 100 / 100, 100 control iterations clean | [run 36001723693](https://github.com/panitw/folio8/actions/runs/36001723693), artifact `soak-linux-x64-36001723693` (2026-09-24) |
| **real amd64**, engine-side closure (`dispositions_linux.c`; binding `529da0d`; native `cdcc1c5b875ea3068a15c0b51e74491a399a5899ba19f98e426c66af20f8c81a`; the reproduce leg sets `FOLIO8_SIGNAL_DISPOSITIONS=leave`, the soak leg runs as shipped) | GitHub runner `runnervmtr4k5`, Linux 6.17.0-1022-azure, x86_64, translation verdict NATIVE, `DOTNET_GCgen0size=0x100000` | **REPRODUCED** — died on iteration 27 with the kernel's `overflowed sigaltstack`; 150 control iterations clean | **CLEAN — VALIDATED HARNESS**, 100 / 100, 100 control iterations clean; load-exposure 0 / 100 in both arms | [run 36011142609](https://github.com/panitw/folio8/actions/runs/36011142609), artifact `soak-linux-x64-36011142609` (2026-09-24T14:26Z) — **the amd64 half of the pack assertion** |
| **real arm64**, engine-side closure (binding `ce6ec81`; native `826ac11253f5ee62595dacffe2a9ceea912474852c3ec9f54e56d1bf3da36b86`, built in the pinned image; the reproduce leg sets `FOLIO8_SIGNAL_DISPOSITIONS=leave`) | the owner's Apple Silicon Mac, `linux/arm64` container under Docker Desktop (Linux 7.0.12-linuxkit, executing natively: translation verdict NATIVE, CPU implementer 0x61, no SVE), 10 CPUs; dmesg NOT readable | **NOT REPRODUCED — MECHANISM NOT LIVE HERE**: pre-fix binding 25 / 25 clean, 25 control iterations clean, mechanism reproducer with the switch **0 / 40 loaded, 0 / 40 absent** | **UNVALIDATED**, 100 / 100 clean, 100 control iterations clean — the tool's own verdict, because no harness on this silicon has seen the defect | `evidence/dotnet-linux-soak/arm64-apple-silicon-docker/{reproduce,soak}/` (local; logs are gitignored), 2026-09-24 |
| **real arm64**, same binding and native | GitHub runner `runnervmoyp6c`, `ubuntu-24.04-arm`, Linux 6.17.0-1022-azure, CPU implementer 0x41, translation verdict NATIVE, **dmesg readable** | **NOT REPRODUCED — MECHANISM NOT LIVE HERE**, twice: pre-fix binding **150 / 150 clean**, 150 control iterations clean, mechanism reproducer **0 / 40 loaded, 0 / 40 absent** ([run 36019302426](https://github.com/panitw/folio8/actions/runs/36019302426)); the same at 25 / 25 in [run 36016056509](https://github.com/panitw/folio8/actions/runs/36016056509) | **not run on this host**: the workflow's job validates first and stops at that verdict, by design — the shipped configuration's clean hundred on arm64 is the Mac row's | both runs 2026-09-24 |

The first amd64 row is the binding-only fix and stays on the record as what it is: a validated
harness and a clean hundred, on a fix that the reproducer showed still loses one to four processes in
a hundred to the window before its restore (DW-398, runs 36002483670 and 36006849443). The second row
is the shipped configuration, and it is the row the amd64 half of `FolioLinuxSoakEvidence` rests on.
Its reproduce leg needs the engine's `leave` switch because the engine now protects every binding that
loads it, the pre-fix one included; with the switch the pre-fix binding died on iteration 27, which is
the same mechanism at the same order of rate as before (95 on the first row). The arm64 row is still
the owner's: no arm64 host has run either leg, and on arm64 the reproduce leg is expected NOT to fire
(the kernel's signal frame is small there; load-exposure was 0 / 1000), which the tool reports as a
harness that cannot validate a soak — the owner decides what that is worth.

**The arm64 rows say why, in numbers.** On both arm64 hosts the reproducer printed
`minsigstksz=4720 sigstksz=20480 clr_altstack=24576`: the runtime's alternate stack is 24 KiB there
(20 KiB usable over its guard page) under a 4.7 KB kernel frame, and the activation handler's
roughly 10.5 KB fits with about 5 KB to spare. On amd64 the same handler has 12 KiB usable under a
1.8 KB frame (AVX2) or 3.6 KB (AVX-512), which is marginal and fatal respectively — DW-398's arithmetic.
So on arm64 the mechanism cannot fire, the reproduce leg cannot validate, and every clean run is
`UNVALIDATED` by the tool's rule. What the two rows establish is exactly that: on the owner's silicon
and on Azure's, with the engine's protection switched OFF, the pre-fix binding does not die, and with it
ON the shipped configuration ran a clean hundred. A validated arm64 harness is not obtainable, and the
assertion for `linux-arm64` rests on the mechanism's absence rather than on a reproduction.

**Accepted by the owner, 2026-09-25.** With the four rows above in front of them the owner accepted
that reading — amd64 validated and clean; arm64 clean with the mechanism shown absent on two hosts —
as the evidence `FolioLinuxSoakEvidence` claims. The assertion is typed at the pack, by a person, as the
gate requires; this line records who accepted what it stands for, and on which runs.

Fill each cell with the runner's own verdict block: it already carries binding
identity, native path and **SHA-256**, host, kernel, architecture, translation
verdict, iteration count and every failure with its signature. Keep the
`--log-dir` output under `evidence/dotnet-linux-soak/<leg>/` and cite it here, so
no figure in this file is retyped from a terminal that no longer exists.

**What "reproduced" is allowed to mean.** Only `REPRODUCED` — a dead host **with**
a named signature. If the pre-fix binding dies with neither, the runner says
`NOT REPRODUCED — UNEXPLAINED CRASH` and the harness stays unvalidated: an
unexplained death is not this defect, and treating it as one is the mistake this
entry has twice recorded.

**If the amd64 reproduction leg will not fire**, raise `--iterations` before
concluding anything. The defect is marginal by nature — whether the frame fits
depends on how deep the stack is when the signal lands — and the pre-fix binding
is a lower-frequency crasher on the shipped AlmaLinux-built native than on a
modern-glibc one. A pre-fix binding that stays clean over a long run is itself a
finding worth recording here, because it would mean **this host cannot validate a
soak at all**.

---

## Tool verification — 2026-09-22T18:4xZ, and it is NOT a leg

These runs establish that the runner behaves, not that the fix holds. The host is
an Apple Silicon Mac; the Linux legs are Docker containers on Docker Desktop's
VM. Source: `evidence/dotnet-linux-soak/tool-verification-2026-09-22Z/`.

**On the two dates in this file.** The runner stamps **UTC**; the story and the
DW-396 subsection are dated by the **local** working day. These runs are stamped
`2026-09-22T18:4xZ`, which is 2026-09-23 locally. The evidence directory is named
for the UTC date, so it matches what is inside it.

Every row below cites the console log it was read out of. Nothing here is retyped
from a terminal that no longer exists, and no row rests on an unpreserved one.
**Every one of those logs stamps the same tool:** `soak.sh` sha256
`5ee2862d58a1fc1daef8084d7264b1ae4603381c6348f3da81cefaeee8305720`, which is the
content hash of the committed file — the runner stamps what it actually ran, not
what `git rev-parse HEAD` says.

| What was checked | How | Result | Log |
|---|---|---|---|
| **A translated host is refused before any iteration** | `linux/amd64` container on Apple Silicon (Rosetta) | **Refused**, naming the signal: `cpuinfo vendor_id is VirtualApple — Apple Silicon running x86 under Rosetta`. Exit 1, nothing built, nothing run | `refusal-rosetta-amd64.console.log` |
| **A native host is let through** | `linux/arm64` container, same machine | Verdict `NATIVE — kernel and process agree on aarch64`; proceeds (and then stops on the absent SDK in that minimal image, which is the next check, not this one) | `gate-passes-native-arm64.console.log` |
| **A reproduction leg against a POST-fix binding is refused** | `--reproduce --binding head` | **Refused**: `head` is not an ancestor of `7f6a936^`. Exit 1 | `refusal-reproduce-post-fix-binding.console.log` |
| **A foreign-architecture native is refused** | `--native …/linux-x64/libfolio8_native.so` on arm64 | **Refused**: "the staged native is amd64 and this host is arm64". Exit 1, nothing run | `refusal-foreign-arch-native.console.log` |
| **A filter that selects nothing is refused** | `--filter FullyQualifiedName~NoSuchTestClassAnywhere` | **Refused on iteration 1**: it would be "a tally of iterations in which the engine was never entered" | `refusal-filter-matched-nothing.console.log` |
| **A clean run with no reproduction behind it is not a pass** | 12 iterations of HEAD's binding, `linux/arm64`, `linux-arm64` native `aa3051b8…` | 12 clean, verdict **`UNVALIDATED`**, **exit 1** | `soak-arm64-12-iterations.console.log`, and **all twelve** iteration logs plus the build log in `soak-arm64-12-iterations/` |
| **An unreadable `dmesg` degrades and discloses** | same run — a container cannot read the kernel ring buffer | Header says `NOT READABLE`; the verdict carries `⚠ ONE SIGNAL WAS MISSING FOR THE WHOLE RUN` | same |
| **The pre-fix binding builds and runs from a worktree** | `--reproduce -n 2`, `linux/arm64` | `86e7e5a` checked out, built against the **same** native, ran clean → `NOT REPRODUCED`, exit 1, worktree pruned, **nothing written to the ledger** | `reproduce-arm64-2-iterations.console.log`, its iteration logs and `worktree.log` in `reproduce-arm64-2-iterations/`. **No `ledger.tsv` exists in that directory**, which is the check: a leg that did not reproduce writes nothing |
| **Ctrl-C produces a verdict rather than silence** | SIGTERM at iteration 3 of 40 | `STOPPED — PARTIAL RUN`, **exit 130**, logs KEPT and named, worktree cleaned | `interrupt-mid-soak.console.log` |
| **Every judgement the tool makes** | `--self-check`, 70 cases | all as expected, exit 0 | `self-check.console.log` |
| `--help` | on macOS | usage, exit 0 | `help.console.log` |

**`--self-check` is where the tool's judgement is actually tested**, and it runs
on any OS because the host that can produce a real crash is not the host this was
written on. It drives six things over synthetic inputs, each printing its expected
rendering beside the actual one:

- **the translation verdict** — Rosetta, a qemu arch mismatch, an x86 process
  with no `vendor_id`, an arm process with x86-shaped `cpuinfo`, the Rosetta
  marker, real amd64, native arm64, the three multi-word or unusual vendor ids
  (Zhaoxin, VIA, Geode), an unknown vendor (SUSPECT), a `binfmt_misc`
  registration for our own arch (SUSPECT), unreadable `cpuinfo`, an unmappable
  machine string, no `uname`, and no readable process architecture — the last
  four all UNKNOWN, **never** agreement and never translation;
- **the binfmt_misc search**, over a synthetic directory: a registration whose
  *name* says nothing and whose *interpreter* names our architecture (Debian's
  `…/qemu-binfmt/x86_64-binfmt-P`), the ordinary `qemu-aarch64-static` shape,
  somebody else's cross-build registration (not ours), `status` (not a
  registration), and an unreadable entry;
- **the ledger round trip** — written through `ledger_append` and read through
  `ledger_lookup`: the row validates its own architecture *and* its own native,
  the other architecture is not validated by it, the same architecture against a
  different native is not either but is reported as a near miss, and an absent
  ledger validates nothing;
- **the reading of the two signatures**, over the exact strings vstest and the
  kernel print — including an unrelated segfault (not this defect), an overflow
  line **already in the ring buffer before the iteration started**, and the
  silent shape .NET 10 prints when a filter matches nothing;
- **the per-iteration classifier** — clean, DW-396 by either signature, a dead
  host with neither, a signal death, and a failing assertion (which is not a
  crash);
- **the final verdict**, including that a clean-but-unvalidated soak exits
  non-zero, a caught reproduction exits zero, a kernel overflow during
  iterations that all passed is a FAILED soak and a VALIDATED harness, and an
  interrupt is neither.

⚠ **Nothing above is admissible as CAP-5 evidence.** The arm64 container executes
natively, but it is a Docker Desktop VM rather than a host, it cannot read
`dmesg`, and — decisively — **the harness has never reproduced the defect on this
machine**, which is exactly the state the tool reports as UNVALIDATED.

---

## What this establishes

1. **A runner exists that cannot produce a false clear by the route DW-396
   took.** A clean tally is reported as `UNVALIDATED` and exits non-zero unless a
   reproduction on the same architecture is on the ledger.
2. **Translated hosts are refused by the tool, not by the operator's memory** —
   demonstrated against a real Rosetta container, with the signal named.
3. **A crash is attributed only when a named signature is present**, and the
   distinction is exercised rather than asserted.

## What this does NOT establish

- **That the fix holds.** No leg has run. CAP-5 is not met, and story 6 must not
  restore the RIDs on the strength of this file.
- **Anything about real amd64 or real arm64 silicon.** Both legs are PENDING.
- **That the pre-fix binding reproduces within any particular iteration count.**
  Unknown until the amd64 leg runs; the 2-iteration arm64 container run above is
  far too short to say anything, and arm64 was always the wider-margined side.
- **That the kernel-side signature reading works against a real kernel.** It is
  verified against the exact recorded strings, not against a live overflow —
  because no host available here can produce one.

---

## Re-running this

```sh
folio-dotnet/build/soak.sh --self-check                 # any OS; verifies the tool, not the fix
folio-dotnet/build/soak.sh --reproduce --log-dir ...    # Linux, real silicon: validate the harness
folio-dotnet/build/soak.sh -n 100 --log-dir ...         # Linux, real silicon: the soak
```

It needs a native library staged (`folio-dotnet/build/build-native.sh`), a full
clone (the pre-fix binding is a `git worktree` at `7f6a936^`), a committed
`src/Folio8` and `test/Folio8.Tests`, and the .NET SDK the test project targets.
It prefers the **shipped** native for the host's RID over `build/native/host/` —
soaking what the package actually carries is the point — and `--native <path>`
overrides that, with the file's ELF `e_machine` checked against the host either
way.

**Where validation is remembered, and how narrow it is.** The ledger lives
outside the repository (`${XDG_STATE_HOME:-~/.local/state}/folio-soak/ledger.tsv`
by default): it is a fact about a machine, not about the source tree, and
committing one host's ledger would let another host inherit a validation it never
earned. A row reads back as a validation **only for its own architecture and its
own native library**, matched on the SHA-256. It is **not** narrowed further than
that: the host name and kernel release are written into the row and reported in
the header, but a soak on a different machine of the same architecture, or after
a kernel upgrade, still reads as validated. That is a deliberate limit and a soft
one — if the amd64 reproduction leg runs on one box and the soak on another, say
so in this file, because the tool will not.
