---
title: 'The soak — reproduce the defect, then show it closed on real hardware'
type: 'feature'
created: '2026-09-23'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: 'f3b68f5'
context:
  - '{project-root}/_bmad-output/specs/spec-dotnet-linux/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-dotnet-linux/sigaltstack-findings.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** CAP-5, and the decision the whole epic turns on. The Linux RIDs
ship only if the fix is shown to hold under repetition on real silicon. The bar
is deliberately higher than "the suite passed": DW-396 records 0-crashes-in-11
being read as a clear, and the same native overflowing afterwards. A harness
that has never seen the defect cannot clear it.

**Approach:** A soak runner that loops the render-heavy suite against a chosen
binding, watching for a dead host and for the kernel's own overflow message, and
reports a verdict with its provenance. It is run first against the **pre-fix**
binding — where it must catch the defect — and only then against the fix.

## Boundaries & Constraints

**Always:**
- **Reproduce first, and treat a harness that has not reproduced as unvalidated.** A clean result from an unvalidated harness is worth nothing and must be reported as such, not as a pass.
- **No emulation. Not qemu, not Rosetta.** Measured this epic: under Rosetta amd64 the pre-fix and post-fix bindings both die, so a translated host cannot distinguish them in either direction. The runner must detect translation itself and refuse, using story 1's signals rather than trusting the operator.
- **amd64 and arm64 are soaked separately and neither stands in for the other.** The spike measured 16 KiB against 24 KiB of headroom for the same handler frame: arm64 was never clear, only wider-margined.
- **Watch the kernel, not only the exit code.** `overflowed sigaltstack` in `dmesg` and `Internal CLR error (0x80131506)` are the named signatures; a crashed host with neither should be reported as a crash of unknown cause rather than assumed to be this one.
- **Report what was run, not a verdict alone**: binding identity, native identity and SHA-256, host, kernel, architecture, translation verdict, iteration count, and every failure with its signature.
- **The owner runs the hardware legs.** Report only what was observed; never a verdict nobody watched.

**Never:**
- Does not restore the Linux RIDs or touch `PackagingTests`' ban — story 6, and only on this story's evidence.
- Does not claim a pass from a run count alone. The count is necessary and not sufficient; the harness must also have been shown able to fail.
- Does not become a CI gate. A soak long enough to mean anything does not belong on every commit, and a short one would recreate the false clear.
- Does not modify `src/Folio8`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Pre-fix binding, real amd64 | Validation leg | Detects the defect; records the signature and the iteration it died on | Reports reproduction achieved |
| Post-fix binding, real amd64 | After validation | Long clean run reported **with** the reproduction that validates it | Any crash ends the soak and is reported |
| Translated host | Rosetta or qemu detected | **Refuses to run** and says why — a translated result is not evidence either way | Exit non-zero |
| Host crash with no known signature | Dead host, nothing in `dmesg` | Reported as an unexplained crash, not as DW-396 | Non-zero |
| `dmesg` unreadable | Container without privileges | Says so; the run continues but the verdict is marked as missing that signal | Degrade, disclose |
| Clean run, harness never validated | No reproduction leg was run | Reported as UNVALIDATED, never as a pass | Non-zero verdict |

</frozen-after-approval>

## Code Map

- `folio-dotnet/build/signal-stack-probe/Program.cs` -- story 1's translation detection: `uname` vs `ProcessArchitecture`, `/proc/cpuinfo` `vendor_id` (`VirtualApple` is the Rosetta tell), `/run/rosetta`, `binfmt_misc`. The soak runner needs the same verdict; a shell reimplementation is acceptable, a weaker check is not.
- `folio-dotnet/build/measure-throughput.sh` -- story 4's wrapper: the git-worktree baseline mechanism, per-leg build directories, provenance header, `describe_exit` distinguishing a harness refusal from a signal death. Reuse the shape.
- `7f6a936^` -- the pre-fix binding for the reproduction leg.
- `folio-dotnet/test/Folio8.Tests/` -- the render-heavy suite; `GoldenTests` and `FontsTests` are the corpus renders that historically triggered it.
- `_bmad-output/implementation-artifacts/deferred-work.md` DW-396 -- where the result is recorded, as a dated subsection like the others.
- Measured this epic and relevant to the runner's design: under Rosetta amd64 both bindings crash the host at a different test count every time, so "the host crashed" alone does not distinguish them.

## Tasks & Acceptance

**Execution:**
- [x] `folio-dotnet/build/soak.sh` -- the runner: takes a binding (`head` or a commit), an iteration count, loops the suite, refuses a translated host, watches exit status and `dmesg`, prints a provenance header and a verdict that names whether the harness has been validated.
- [x] `folio-dotnet/build/soak.sh` -- a reproduction mode that runs the pre-fix binding and reports whether the defect was caught, so the validated/unvalidated state is produced by the tool rather than asserted by a person.
- [x] `_bmad-output/implementation-artifacts/` -- a soak record with the provenance fields above, carrying PENDING rows for the legs the owner still has to run.
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- a dated DW-396 subsection recording what was run here and what remains outstanding. DW-396 stays OPEN; story 7 closes it.

**Acceptance Criteria:**
- Given a translated host, when the runner starts, then it refuses before any iteration and names the signal that told it.
- Given the pre-fix binding on a host where the defect reproduces, when the runner finishes, then it reports reproduction with the signature and the iteration count.
- Given a clean run with no reproduction leg behind it, when the verdict prints, then it says UNVALIDATED and exits non-zero.
- Given any crash, when it is reported, then the report distinguishes a named DW-396 signature from an unexplained death.
- Given the soak record, when a reader opens it, then every figure carries its host, architecture and translation verdict, and every leg not yet run is visibly PENDING.

## Implementation Notes

**What landed:** `folio-dotnet/build/soak.sh`, the record
`_bmad-output/implementation-artifacts/dotnet-linux-soak.md` with both hardware legs PENDING, its
console evidence under `evidence/dotnet-linux-soak/tool-verification-2026-09-23/`, and a dated
DW-396 subsection. `folio-dotnet/src/Folio8` is untouched; no CI workflow references the runner.

**Shape reused from story 4's `measure-throughput.sh`:** the git-worktree mechanism for a non-HEAD
binding, one native file staged for every leg so the engine is held fixed while the binding varies,
validate-the-whole-argument-list-before-any-work, provenance stamped into a header, and an exit
status that is never asked to mean more than it does.

**Three decisions worth recording:**

1. **The refusal covers `SUSPECT` and `UNKNOWN`, not only `TRANSLATED`, and there is no override
   flag.** A verdict the tool cannot stand behind is not weaker evidence, it is no evidence; an
   override is how a translated tally reaches the record with a footnote nobody reads, which DW-396
   has already recorded twice.
2. **The validation ledger lives outside the repository** (`${XDG_STATE_HOME:-~/.local/state}/folio-soak/ledger.tsv`)
   and is keyed by **architecture**. It is a fact about a machine, not about the source tree;
   committing one host's ledger would let another host inherit a validation it never earned.
3. **There is no container mode**, unlike `probe-signal-stack.sh` and `measure-throughput.sh`. On the
   development machine a `linux/<arch>` container is either the same silicon (arm64) or Rosetta
   (amd64), and a wrapper offering both would invite the second — exactly what CAP-5 excludes.

**`--self-check` was added beyond the task list, and it earned its place repeatedly:** it found an
unreadable process architecture being reported as TRANSLATED rather than UNKNOWN (a claim about
hardware derived from a failure to read a file), and it is the only way the crash-signature reading
and the **ledger write-then-read round trip** can be exercised at all, since no host available here
can produce a real `overflowed sigaltstack`. 70 cases; it runs on any OS.

**Review round 1 closed fifteen findings.** The load-bearing ones: `--reproduce` accepted any
binding, so a post-fix binding dying for any reason could have written REPRODUCED to the ledger (now
an ancestry check against `7f6a936^`); the ledger was written with seven fields and read on two (now
matched on the native's SHA-256 as well, with the narrowness of that stated in the record); the
binfmt_misc probe stat'd one hard-coded filename and so degraded **open** where every other
gathering failure degrades into UNKNOWN (now enumerates the directory and matches interpreters); an
`overflowed sigaltstack` during an iteration that passed was classified CLEAN and discarded, which
is DW-396's own history (now a verdict of its own); a dirty tree was warned about and then recorded
anyway (now refused, suite included); and every caveat went to stderr while the number went to
stdout. A sixteenth was found while verifying the fixes: a `--filter` matching nothing gave three
"clean" iterations that never entered the engine, and is now refused on iteration 1.

**Known limitation, disclosed in the record:** the `DW396` and `CRASH-UNKNOWN` paths are verified
against the exact strings vstest and the kernel print, not against a live crash. The first real
exercise of them is the owner's amd64 reproduction leg.

## Spec Change Log

## Review Triage Log

Three layers, launched together. Verdicts mine. This tool is the gate the epic's
irreversible step depends on, so the bar for its own correctness is the bar it
sets for everything else.

| # | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|
| 1 | `--reproduce` honoured any binding, including `head` — a post-fix binding dying for any reason would write REPRODUCED and validate the harness permanently (all three layers) | **high** | patch | Verified by reading the mode/binding resolution: nothing checked ancestry. This is the one inference the tool exists to refuse, reachable through the front door. Now refused via `git merge-base --is-ancestor`, and confirmed: `--reproduce --binding head` exits 1 naming why. |
| 2 | The ledger write→read round trip — the only path that turns a clean soak into a pass — was executed by nothing (verification-gap, pre-verified) | **high** | patch | Confirmed: `--self-check` passed `validated` in as a literal, and `record_reproduction` was defined after the self-check's exit. Its first execution anywhere would have been the owner's hardware leg. Now eight self-check cases write and read real rows in a temp ledger. |
| 3 | The ledger was written with seven provenance fields and read on two (all three layers) | **medium** | patch | Real: a reproduction from a different kernel and a different native silently validated today's soak, in a tool whose thesis is that a figure without provenance is worth nothing. Now matches architecture **and** native SHA-256, reports a near miss otherwise, and the record states the per-machine limit in words. |
| 4 | The `binfmt_misc` probe stat'd one hard-coded filename and so degraded **open** (verification-gap, edge-case) | **medium** | patch | Every other gathering failure lands in UNKNOWN and refuses; this one let a host through as native. Debian's own `qemu-binfmt/x86_64-binfmt-P` naming would have been missed. Now enumerates the directory and matches interpreter lines, with seven cases over a synthetic dir. |
| 5 | A `overflowed sigaltstack` line printed during an iteration where nothing died was classified CLEAN and discarded (edge-case, verification-gap) | **medium** | patch | DW-396's own history is a kernel message present while the run looked fine, so dropping it loses the one signal the tool was built to watch. Now surfaced as it happens and raised to a distinct outcome: DW-396 signature, host survived. |
| 6 | Default logs deleted on exit, so the failure block cited a path that no longer existed (blind-hunter) | **medium** | patch | The run that finally catches the defect was the run whose evidence was discarded. Logs now live outside the temp tree and are kept on any non-clean outcome. |
| 7 | No INT/TERM trap (blind-hunter, edge-case) | **medium** | patch | A Ctrl-C at iteration 60 of 100 produced no verdict, no partial record and left the worktree behind — the likeliest real ending for the one activity the owner sits and watches. Now prints a partial verdict, exits 130, keeps logs, prunes. |
| 8 | Caveats on stderr while the verdict was on stdout; the short-run warning gated to soak mode (blind-hunter, edge-case) | **medium** | patch | `soak.sh -n 100 > record.log` kept the number and lost the disclaimers — for a tool whose thesis is that a number without its caveats is DW-396's mistake. Both fixed. |
| 9 | A dirty tree warned "must not be recorded against <commit>" and then recorded exactly that (blind-hunter) | **medium** | patch | The warning contradicted what the code did. Now refuses, and covers `test/Folio8.Tests` too, since a modified suite under a clean-commit header is the same error. |
| 10 | The tool stamped `rev-parse HEAD` as its own version (verification-gap) | **medium** | patch | Demonstrably false in the committed evidence: a log recorded "soak.sh at f3b68f5" for a file not in f3b68f5. Now stamped by content hash. |
| 11 | The header and verdict printed a hardcoded "real silicon", dropping the derived summary and the mode (blind-hunter) | **medium** | patch | A pasted verdict block could not be told apart between two native hosts, and did not say which leg produced it. Both now printed. |
| 12 | The shell vendor list dropped `VIA VIA VIA` and `Geode by NSC`, and space-joining could not represent them (edge-case) | **medium** | patch | Story 1 fixed this exact defect in the probe; the shell re-expression reintroduced it. Real VIA/Geode silicon would be SUSPECT and hard-refused with no override. Now newline-separated with `grep -Fx`. |
| 13 | A staged native of the wrong architecture surfaced only as SUITE-FAILED (edge-case) | **medium** | patch | In a tool that reads its own ELF `e_machine` rather than trust the operator, not reading the native's was inconsistent. Now refused before anything runs. |
| 14 | `tr '
' ' | '` truncates set2 to one character (blind-hunter, verification-gap) | **low** | patch | Reviewer demonstrated it; kernel lines were space-joined in the one place the owner copies the kernel's own words. Replaced. |
| 15 | Evidence gaps: three verification rows citing no log, only 2 of 12 iteration logs, and a date skew against the directory name (blind-hunter, verification-gap) | **medium** | patch | All confirmed, in a file whose stated rule is that no figure is retyped from a vanished terminal. Table rewritten with eleven rows each citing a committed log; dates reconciled as UTC-vs-local and the directory renamed to say so. |
| 16 | `deferred-work.md` said DW-396 closes in story 6 and in story 7, three lines apart (blind-hunter) | **low** | patch | Confirmed. Now one statement: story 7 closes it, on CAP-5's legs. |
| 17 | `--iterations` overflow, `--binding ''`, `seq` materialising a huge list, stale `--log-dir` reuse, exit status exactly 128 (edge-case) | **low** | rejected | All real and all confined to a hand-invoked developer tool; each fix adds a branch for a case nobody meets. Rejected under the low-severity rule, recorded so the reasoning is on record rather than absent. |
| 18 | The same rationale paragraphs now exist near-verbatim in four files (blind-hunter) | **low** | rejected | A fair observation and the repository has DW-148 open on exactly this pattern. Rejected here because the duplication is between a tool's header, a record, a ledger entry and a spec — four audiences who each read only one — and consolidating to citations would make the tool's own header depend on a file it cannot assume is present. Recorded as a known cost. |
| 19 | In `--reproduce` the **suite** reverts with the binding, so the reproduction leg and the soak leg may render different work (verification-gap) | **medium** | defer | Correct and not cheaply fixable: the worktree is how the pre-fix binding is obtained at all, and the tests travel with it. The corpus filter is the same, and no fixture changed between those commits, but nothing asserts it. Filed. |

**Found while verifying the patches, and the most valuable item in the story:** a
`--filter` matching nothing exited 0 and printed nothing at `-v quiet` on .NET
10, producing iterations that tallied as CLEAN while the engine was never
entered. The false clear in its purest form, inside the tool built to prevent
it. Now detected on the first iteration and refused.


## Design Notes

The runner's job is to make a false clear hard. Two things do that: it refuses
hosts whose answers cannot be trusted, and it refuses to call a clean run a pass
unless it has been shown able to fail. Everything else is bookkeeping.

## Verification

**Commands:**
- `folio-dotnet/build/soak.sh --help` -- expected: usage, exits zero
- The runner invoked on a translated host -- expected: refuses, names the signal
- A short run against the fix on native arm64 -- expected: clean, and reported as UNVALIDATED because the reproduction leg has not run on this architecture
