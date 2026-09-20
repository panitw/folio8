# Autonomous decision log — spec-font-sources-and-embedding

Standing instruction, 2026-09-20: *"run to the end of the spec without stop and ask me the
question. log all decision in a file under the spec folder."*

Every decision below was made **without asking**, under that instruction. Each entry records the
choice, the options rejected, and the reasoning — so a decision that turns out wrong can be found
and reversed without re-deriving why it was made. Decisions the owner made in conversation live in
`.memlog.md` and in each story's frozen block; this file holds only what was decided autonomously.

Story `done_checkpoint` flags in `stories.yaml` are overridden by this instruction and are not
honoured during this run. Story 3's flag is the only one affected.

---

## A-0 — Where this log lives, and what goes in it

**Decision:** `DECISIONS.md`, a sibling of `SPEC.md`, not a companion and not listed in
`companions:`.

**Rejected:** appending to `.memlog.md` (it is the owner's decision-of-record and append-only via a
script — mixing autonomous calls into it would blur who decided what); a per-story log (the whole
point is one place to scan).

**Reasoning:** the owner asked for "a file under the spec folder". It is deliberately *not* a
companion, because companions are the what-to-build contract every downstream consumer reads, and
this is a record of how one run behaved.

---

## Story 1 — verification findings, and what was done about them

### A-1 — The one red test is pre-existing, and was proved so rather than assumed

**Decision:** accepted `folio-go/internal/text` `TestCorpusMeetsP6ExerciseFloors/P6g` as red and
unrelated; did not fix it, did not skip it, did not fold it into this story.

**Evidence, not the subagent's word:** checked out the baseline commit `85cdcc0` into a throwaway
worktree and ran that test alone. Identical numbers — `got 7, need >=20`, `P6g:7`. The diff touches
no file under `internal/text`. The worktree was removed afterwards.

**Reasoning:** a red test a story did not cause is not that story's to fix, and fixing it here would
put an unrelated corpus change inside a font-resolution commit.

### A-2 — `npm run lint` reports a false failure in this environment

**Decision:** treated the `folio-js` lint failure as an artifact of this environment's command
proxy, not a code defect, and rewrote the spec's Verification block to invoke the binaries directly.

**Evidence:** `npm run lint` exits 2 printing `ESLint output (JSON parse failed: EOF while parsing
a value at line 1 column 0)` — which is a wrapper trying to parse the script's output as ESLint
JSON. Running the same two tools directly, `./node_modules/.bin/oxlint` and
`./node_modules/.bin/tsc -p tsconfig.test.json --noEmit`, both exit 0.

**Also corrected:** the spec named `npm run typecheck` in `folio-js`, which does not exist — that
script lives in another package and I lifted it from the wrong CI job when writing the spec. My
error, fixed in the spec rather than worked around.

### A-3 — A defensive inconsistency left for review rather than silently patched

**Finding:** `substitutionPool` (`folio-go/render.go:1955`) guards `cache == nil` and returns the
`FontSet` keys, but its only caller `substituteFace` (`:1978`) then calls `cache.get(name, fs)`
unguarded — so the nil guard promises a safety its caller does not honour.

**Decision:** not fixed here. Every production call site of `shapeSegments` passes a real cache
(`render.go:851`, `table_render.go:907/1250/1506`, `page_number.go:463`,
`page_setup.go:1054/1668`), so it is unreachable rather than a live bug.

**Reasoning:** it is a review-shaped question — delete the misleading guard, or honour it — and
step-04's reviewers should see the code as the implementer left it, not as I pre-emptively
adjusted it. Recorded here so it cannot be lost if review misses it.

**CLOSED by the A-6 re-derivation.** All three review layers raised it in pass 1; the revert then
replaced the code entirely, and the re-derived `substitutionPool`/`substituteFace` carry no nil
guard at all, so the inconsistency no longer exists. The line numbers above refer to code that was
discarded and should not be chased.

### A-4 — Accepted the implementer's weaker .NET byte-identity assertion

**Decision:** accepted that the .NET test asserts "a font set the document never consults does not
change the bytes" over the existing `fixtures/embedded-font`, rather than the full "identical bytes
under any font set".

**Reasoning:** that fixture's chain names `"Noto Sans"` alongside its asset, so it is not an
all-embedded document and the stronger claim is not true of it. The stronger claim — the one the
issue reporter measured — is proved in Go against a purpose-built all-asset document
(`TestAnAllEmbeddedDocumentNeedsNoFontSetAtAll`). Proving it once, where it is true, beats proving
a weaker thing twice.

### A-5 — ABI version moved 1 → 2, which is a real break

**Decision:** let it stand.

**Reasoning:** the cshared entry points gained a parameter, so the ABI genuinely changed and the
version check exists precisely to catch a mismatched pair. The consequence — the managed package
and the native library must ship together — is acceptable inside the coordinated 2.0.0 release this
spec is already scoped to, and it is exactly the kind of break that release exists to absorb.

### A-6 — Took the bad_spec loopback rather than patching, and reverted 41 files to do it

**Decision:** routed review findings #1/#2 to `bad_spec`, reverted every code change to the baseline
commit, amended the spec, and re-derived — instead of patching the one broken arm.

**The finding:** a chain whose *every* member is absent refuses under `FaceFallbackSubstitute`
before shaping is reached, because `verticalModel` (`folio-go/wrap.go:653`) raises on
`len(metrics) == 0` without consulting the selector. Two review layers found it independently. I
proved it with a throwaway probe rather than inferring it: `{"body": ["Brand Face"]}` rendered
leniently over the shipped set was refused. That is the spec's own Success-signal case.

**Why bad_spec and not patch:** the root cause is a sentence *I* wrote in the Code Map — *"This is
the 'no candidate' state; keep it an error."* True under strict, false under lenient, and the
implementer followed it exactly. The workflow says to prefer `bad_spec` when in doubt because a
spec-level fix produces more coherent code, and that applies here: the real fix has to decide what
line metrics a substituted face contributes, which is finding #2 and is a design question, not a
one-line guard.

**Why the revert was worth its cost:** ten further findings were outstanding. Patching them into
code that was about to be replaced would have been wasted work, so all ten are folded into the
amended spec as explicit tasks and get fixed in one re-derivation. The KEEP block preserves the
sixteen things the first attempt got right, so the revert costs the implementation, not the design.

**What would have made this unnecessary:** a test for the single-entry absent chain. Matrix row 4
passed only because every lenient test used a two-entry chain with a present member — the row was
covered in name and not in substance. That is now an explicit task.


### A-7 — Accepted a layout trade-off the owner should know about

**Decision:** accepted that a **lenient** render widens the line box whenever a chain member is
absent — *even when nothing actually substitutes* — so its bytes differ from a strict render of the
same document, with no diagnostic saying so.

**Why it happens:** the table body's vertical model is computed **once per table, before any cell is
shaped**, so the faces that will actually be painted are not knowable at the point the line height
is decided. The implementer chose a data-independent envelope over every pool face, gated on
`absent && lenient`. Sizing from the painted faces instead would make line height depend on row
content, which is a restructuring of the table vertical model, not a fix.

**Why accepted rather than escalated:** strict is byte-for-byte unchanged, so no existing integrator
is affected; lenient is new and opt-in, so there is no prior lenient output to preserve; and the
envelope is never too *small*, so nothing clips — the failure mode is slightly generous leading, not
broken layout. Deciding it was within the standing instruction not to stop and ask.

**What the owner may want to revisit:** if byte-identity between strict and lenient matters for a
document whose chain has an absent member that never actually substitutes, this is the thing to
change, and it costs a table-vertical-model restructure. Flagged because it is the one place this
story's behaviour is *surprising* rather than merely new.

**What was fixed rather than accepted:** the same behaviour was documented in the Go guide only.
That half violates this story's "the three libraries ship the same behaviour" constraint outright
and was sent back as a patch — a .NET or Node caller must be told their leading moves too.

### A-8 — One finding deferred rather than chased

**Decision:** deferred review finding #33 — whether a chain whose every member is a carried asset
with unparseable bytes reaches the substitution pool under lenient, or refuses before consulting it.

**Reasoning:** graded `maybe-false`. The pool arm is gated on `absent`, and it is genuinely unclear
from the diff whether an unparseable *carried* asset sets that flag. It would be settled by one
fixture: a single-entry chain naming a carried asset with corrupt bytes, rendered leniently with a
covering face in the supplied font set. I asked the implementer to check it directly rather than
leaving it purely deferred.

**OUTCOME: refuted, so nothing is deferred.** The refusal is never reached. With text,
`shapeSegments` aborts first on the carried-face parse error (`required table "head" is absent`),
byte-identically under both selectors — because `absent` is `!cache.declares(...)`, not `!present`,
so a declared-but-unparseable asset is a document fault with nothing to substitute *for*. With no
text, `verticalModel`'s empty-metrics arm is not reached at all. Verified across
`{"", "Hi"} x {Strict, Substitute}`. The `deferred-work.md` entry was rewritten to say so rather
than left standing as open work that does not exist.
