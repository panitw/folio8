# Decision log — Epic 7 & 8 delivery run (with Story 15.1 as story zero)

**Run started:** 2026-08-30
**Target:** Story 15.1, then Epic 7 (7.1–7.7), then Epic 8 (8.1–8.6) — 14 stories.
**Baseline at run start:** `09bb30e` on `main`.
**Orchestrator:** run-dev-cycle. **Per-story loop:** `bmad-build-auto` via `bmad-story-builder`.

This log records decisions that outlive the story that raised them. Per-story narrative lives in
each spec's `## Delivery Log`; findings consciously not fixed live in `deferred-work.md`. Project
decisions predating this run are in `folio-mvp-decision-log.md`, which this log does not duplicate.

---

## Lead Grounding

Filed by the orchestrator 2026-08-30 from the engineering lead's one-time grounding report, at
baseline `09bb30e`. The lead grounded from `ARCHITECTURE-SPINE.md` (AD-1…AD-26 in full), `epics.md`
(inventory, coverage map, Epics 7 and 8 in full, 9/10/11/15 read), both SPECs and their five
companions, this log, targeted greps of the 14k-line MVP decision log, and `deferred-work.md`.

**Correction to the run brief, recorded first because it changes Story 15.1.** There are no separate
ADR files. `_bmad-output/planning-artifacts/architecture/architecture-folio-2026-08-23/ARCHITECTURE-SPINE.md`
is the only architecture document, and the `AD-N` invariants inside it *are* the ADRs. Cite `AD-N`
plus a SPEC clause or story AC — never an ADR path.

### The spine, as it constrains this run

- **Byte identity is the product, not a quality bar.** AD-2 (one fixed-point unit — `geom.Length`,
  int64 millipoints), AD-3 (numbers reach the PDF through one file in two representations), AD-23
  (no `float64` under `internal/`, geometry or data), AD-1 (determinism is a *directory* boundary),
  AD-21 (four-target matrix; a hash change is a defect until proven intended), AD-22 (any change to
  layout, subsetting, emission, the locale table or the toolchain is breaking, and ships as such).
  Counter-metric C6 — "golden hashes regenerated rather than investigated" — is what Story 15.1 exists
  to prevent.
- **One authority per fact,** recurring at every layer: one number→text emitter (AD-3), one
  coordinate-flip function (AD-24), one canonical serializer (AD-9), no TypeScript model of a
  document (AD-15), the browser never measures text (AD-17).
- **Command / projection** (AD-15, AD-16): the UI paints from an immutable snapshot and sends every
  committed mutation as one opaque command. This binds 7.4, 7.5, 7.6, 8.1, 8.2 and 8.6.
- **The window model** (D-2.6.1): pages are sliding windows onto one unbounded content column, which
  is never mutated. Epic 7's own header makes this an *input* to the epic, not a target.
- **The overflow scope line:** content exceeding its declared *box* is FR44/Story 2.8 — clip, diagnose,
  still return bytes. An item taller than the *window* is a located template error. Route every new
  overflow question through that distinction first.
- **AD-14's diagnostic contract:** one closed, additive SCREAMING_SNAKE registry; clipped content and
  over-tall rows are Warnings returned *alongside* PDF bytes, never fatal.
- **AD-9's P1** (`Parse(Serialize(d)) == d`) is forced, not chosen — the serializer preserves what it
  does not understand, orphaned assets included. This matters more in Epic 8 than it ever has.

### Risks the lead flagged before any story is specced

**Epic 7.** 7.1 and 7.2 are load-bearing: both change the vertical model everything downstream
measures against. 7.2's trap is in its own AC — `lineSpacing` scales `Advance` and *nothing else*;
`FirstBaseline` is unchanged, which is D-2.5a/DW-15's deliberate two-model split. 7.3 is where a spec
is most likely to drift: slack must be distributed in integer millipoints by one stated ordered rule
(a per-gap float divide is a second rounding site, banned by AD-2/AD-3), and a Thai line with no
interior break opportunities (AD-25) must fall back to its natural start edge — an underflow case the
AC does not cover. 7.3 also *depends* on 7.1: a line ended by a mandatory break must be
distinguishable at pack time. 7.5 is a designer-side command-validation change and must not touch
`internal/layout/paginate.go`. 7.7 extends `ItemGroup` past table rows for the first time, which is
exactly the co-extensiveness D-4.6.2 left a tripwire for.

**Epic 8.** 8.3 is load-bearing in a way the others are not — it changes the *format*, the one public
contract between two users of equal standing. Three findings, all pulled forward:

1. **The `.folio` version bump is a genuine owner fork.** D-1.4.12 (extending a closed set is MAJOR)
   points at MAJOR; but a higher MAJOR is a load error, and Story 15.3 is about to cut
   `folio-go/v0.1.0`, so a 2.0 document would be unreadable by the first tagged release forever.
   Materially cheaper to settle before the tag than after. Raised at 8.3's plan gate.
2. **8.3's "closed set of media types" contradicts D-1.8.1 (amended)**, which ruled `mediaType` must
   *not* be a closed set and an unrecognised one is never a load error — and whose own note predicted
   this recurrence "later for font formats". The lead will rule rather than escalate: unrecognised
   media type is preserved at load and errors at *render*; a *recognised* type whose bytes do not
   decode stays a load error, as does a chain entry naming an absent asset key.
3. **8.6's "unreferenced font assets are dropped on save" contradicts a forced invariant.**
   `folio-go/internal/template/serialize.go:420-427` preserves orphans unconditionally because P1
   forces it — its comment states there is "no policy latitude to drop one" and that collecting
   orphans is a designer *command*, never a serializer side effect. D-5.13.3's image-asset precedent
   already gives the correct shape. `spec-fonts/font-catalogue.md` needs amending in the same story.

**A data-loss trap for 8.1 and 8.6.** `assetKeyReferenced` in `folio-go/component_commands.go` matches
image elements only; a font asset is referenced from the `fonts` chain map, not from any element, so
that function returns false for *every* font asset. Any orphan check that reuses it deletes live
faces, with no compile error to announce it.

**Bold/italic is not unruled — this closes the run brief's open flag.** `epics.md` Epic 11 (FR57) owns
it with Stories 11.1–11.3. SPEC-fonts' open-question list predates Epic 11 and is stale, not open. No
Epic 8 story depends on the answer; the only residue is 8.5's catalogue shipping Regular-only, which
Epic 11 later widens. A weighted catalogue face proposed in 8.5 is a collision with Epic 11.

**Also noted:** 8.4 lands on DW-14 (unbounded `beginbfchar`) and DW-13 (uncompressed `FontFile2` — an
Epic 8 document pays for whole faces *and* an embedded subset). DW-21 gates the heavy statement legs
behind `FOLIO_HEAVY=1`; unset, they `--- SKIP` rather than silently pass, so Story 15.1 must set it or
its last AC passes on three suites that never ran.

---

## Standing decisions (settled at setup, 2026-08-30)

### D-R7.0 — Story 15.1 runs first, as story zero of this run
**Owner decision** (asked at setup; the owner chose "explain the move first" over two proceed-anyway
options).

**Verdict.** The run does not start at Story 7.1. It starts at Story 15.1 — explain the moved golden
hash — and Epic 7 does not begin until the golden corpus is green or its move has been deliberately
re-recorded with fresh human sign-off.

**Situation.** The four golden statement fixtures (`statement-1/5/20/50`) are the project's primary
acceptance evidence: each is a `.folio` template whose rendered PDF bytes are frozen and committed,
so any later change that moves those bytes is caught the moment it lands. At run start all four were
RED. Verified by the orchestrator before the run began, not taken from the sprint note:

| Commit | Golden corpus |
|---|---|
| `be40f24` (before Epic 9) | green |
| `791ed00` (Epic 9 — "a component's box prints") | **RED — all four, +4 bytes per page** |
| `304442f` (Epic 10 — text colour) | RED, byte counts unchanged from `791ed00` |

So **Epic 9 moved the corpus and Epic 10 did not.** This corrects two records: `sprint-status.yaml`
named only `statement-1` (all four moved) and named Epics 9 and 10 jointly as suspects (only 9 is
implicated). Epic 9's own sprint-status note claims "a document declaring no element background or
border must hash identically and the golden corpus stays a witness" — that claim is false as shipped.

The separate red, `TestCorpusMeetsP6ExerciseFloors` (P6g 7 < 20), is **not** part of this: it is the
permanent, mandated red D-000.17 / D-2.1.14 require to stay unmet. It is expected and stays red.

**In simple terms.** The goldens are a tripwire: a set of documents whose output bytes were checked
by a human once and then frozen, so the build can shout if anything ever changes them. Somebody has
already tripped it, and nobody has said whether they meant to. Story 7.1's central promise is "the
whole existing fixture corpus re-renders with every hash unchanged" — but you cannot demonstrate that
nothing moved when the alarm is already sounding for a different reason. Build thirteen more stories
on top and the question stops being "what did Epic 9 change?" and becomes "which of fourteen stories
changed this?", with the four-target matrix re-run needed to answer it either way.

**Options considered.** (a) *Proceed and re-point every corpus AC at current HEAD bytes* — the guard
would still stop a new story moving bytes, but the Epic 9 delta stays unexplained and Story 15.1 gets
thirteen stories harder; also it silently ratifies a byte move no human signed off, which is exactly
what AD-22 and the golden test's own failure message forbid. (b) *Proceed with the ACs as written* —
every story's corpus check fails for a pre-existing reason, so the guard is off for the whole run and
a real regression hides inside the existing red. (c) *Explain it first* — chosen by the owner.

**Why this wins.** It is the only option under which Epic 7 and Epic 8's hash-identity criteria mean
anything when they are evaluated, and it matches the sprint sequence already agreed on 2026-08-30,
which named 15.1 "S1 UNBLOCK … doing this after building on top is strictly harder". The accepted
cost is one story of delay before any Epic 7 work starts.

**Consequences.** Epic 7 dispatch is gated on 15.1 reaching `done`. If 15.1 concludes the move was
*wanted*, re-recording the digests re-invalidates the human sign-off across all four documents in
whole — `statement-signoff.json` and the semantic acceptance step must be re-run, not patched. If it
concludes the move was a defect, the fix lands in Epic 9's code and the goldens are left untouched.
Either outcome is a decision this log records before Epic 7 begins.

**How we'd know it was wrong.** If 15.1 finds the +4 bytes/page are inseparable from a change Epic 7
is about to rewrite anyway, the sequencing cost bought nothing — visible as 15.1 halting on an intent
gap that only Epic 7 can settle.

### D-R7.1 — Heavy-test cadence is per-epic, overridden to every-story where correctness is byte-shaped
**Owner decision** (cadence), **orchestrator decision** (the override rule).

**Verdict.** Integration and e2e — Playwright and the four-target hash matrix — run at each epic
boundary: after Story 15.1, after Story 7.7, and after Story 8.6. Unit tests, lint and build run on
every story. **Overridden to every-story for any story whose own correctness is byte-identity-shaped**
— 15.1 by construction, and every story carrying a "the corpus hashes identically" criterion, which
is 7.1, 7.2, 7.3, 7.5, 7.7, 8.3 and 8.4.

**Why.** The override is not caution for its own sake: Epic 9 moved the corpus and it went undetected
across two epics precisely because the heavy check was deferred. A story that promises byte identity
must measure it, or the promise is unfalsifiable. Under a per-epic cadence every deferred suite must
still compile — a Go package that fails to build under its integration tag silently skips its tests,
so the catch-up run would otherwise open with a compile error from ten stories ago.

**Consequences.** Each story's Delivery Log names which suites were measured and which were deferred.
"Green" without that qualifier is a misreport. An epic is not marked `done` until its catch-up run
passes.

### D-R7.2 — Answer channel is the terminal; checkpoints are continuous
**Owner decision.** Questions are asked inline via `AskUserQuestion`, not Telegram, for this run. The
run does not pause at story or epic boundaries; it pauses only for a decision that genuinely needs the
owner. Per-story results are still reported as they land.

### D-R7.3 — Build order is numeric within each epic
**Orchestrator decision** (routine — numeric order already is the dependency order here).

**Verdict.** 15.1 → 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7 → 8.1, 8.2, 8.3, 8.4, 8.5, 8.6.

**Why.** Both epics' stories are written in dependency order. In Epic 7 the typography trio (7.1–7.3)
lands in the engine before 7.4 exposes it in the inspector, and 7.5 must lift the one-page bound before
7.6 can draw the resulting pages or 7.7 can keep a group inside one of them. In Epic 8, 8.1 makes the
chains editable before 8.2 gives them an editor, and 8.3 puts a face in the file before 8.4 renders
from it or 8.6 makes picking one do both. No re-ordering is needed.

**Consequences.** Stories run strictly one at a time; build-auto's previous-story continuity scan reads
the committed, `done` spec of the preceding story, so two in flight would halt the run.

### D-R7.4 — Dispatch mode is classic intent, not folder+id
**Orchestrator decision** (routine — determined by what is on disk).

**Verdict.** Stories are dispatched to `bmad-build-auto` as freeform intent naming the story number.

**Why.** Folder+id dispatch needs `{spec_folder}/stories.yaml`; neither `_bmad-output/specs/spec-folio`
nor `_bmad-output/specs/spec-fonts` has one — they carry `SPEC.md` and companions only. Build-auto
therefore resolves epic context from `epics.md` and writes the spec into
`_bmad-output/implementation-artifacts/`, which is where all 43 existing story files already live.

**Consequences.** Spec filenames follow build-auto's classic derivation (`spec-<slug>.md`), which
differs from the hand-made `<story-key>.md` convention of Epics 1–6. The orchestrator renames each spec
to the existing `<story-key>.md` convention at the plan gate so `sprint-status.yaml` keys and story
files continue to correspond one-to-one.

---

## Run decisions

### D-R7.5 — The corpus move is text alignment, not the element-box paint
**Orchestrator decision** (a correction of fact, not a choice — recorded because D-R7.0 asserts the
wrong mechanism and Story 15.1 would search the wrong file on that brief).

**Verdict.** The +4 bytes per page is **one `Tm` x-coordinate** on the page-footer element, caused by
`folio-go/text_alignment.go` — a file created in `791ed00` with no story, no AC and no FR. Epic 9's
element-box paint moved nothing. D-R7.0's phrase "Epic 9's claim that the corpus stays a witness is
false" is right in outcome and **wrong in mechanism**, and this entry supersedes it on that point.

**The measurement.** The lead attributed it and the orchestrator independently reproduced it — rendered
`statement-1` at HEAD and diffed the content stream against the committed `expected.pdf`. Content
stream `/Length` moves 4548 → 4552, and exactly one run of bytes differs in the whole file:

```
recorded: 1 0 0 1 436     53.88 Tm   <001b002200280026000100060001002e002700010006> Tj
HEAD:     1 0 0 1 522.474 53.88 Tm   <001b002200280026000100060001002e002700010006> Tj
```

Same ten glyphs — `Page 1 of 1` — moved, not changed. `"436"` (3 chars) → `"522.474"` (7 chars) is the
whole delta. The element is `pageFooter` `e4`: `x=400`, `width=123`, `style.align="right"`. The page
footer repeats on every page, which is why the delta is +4 **per page** and scales exactly across the
1-, 5-, 20- and 50-page fixtures.

**In simple terms.** The footer says `Page 1 of 1` and the template has always asked for it to sit
flush right. Until `791ed00` the engine read that request, validated it, round-tripped it, showed it in
the inspector — and then drew the text at the *left* edge of its box anyway, because nothing had ever
been wired to act on it. Epic 9 wired it up. The text moved to where the template always said it
should go, and the number describing its position got four characters longer. Nothing about the
document changed except that one instruction was finally obeyed.

**The arithmetic, which is the test of the attribution.** Element coordinates are *band-relative*; the
absolute position is the 36pt page margin plus `x`. So `e4`'s box spans 436 → 559 absolute, and 559 is
exactly the content right edge (A4's 595.276 less the 36pt right margin, to within the box's own
rounding). Old position 436 = the box's **left** edge — alignment ignored. New position 522.474 leaves
36.526pt of text before that right edge, which is 0.52em per character for ten characters at 7pt: the
correct width for a proportional face. **The new rendering is right and the old was the defect.**

**Population, which closes the red set.** Exactly four templates in the whole corpus declare a
non-`left` `style.align`: `fixtures/statement-{1,5,20,50}/input.folio`. That is precisely the set that
went red — no other fixture could have. And **no fixture anywhere declares `valign`**, so the vertical
half of the same commit shipped with zero corpus coverage. That is a second finding and it is Story
15.1's fourth AC in the literal: "a fixture is added that would have caught it — because no document in
the corpus exercised the path that moved."

**Consequences.** Story 15.1 is briefed with the two byte strings above so it verifies rather than
searches. It must still perform its own diff (AC1 forbids attribution from the commit log alone), but
it must not go looking for a defect in the box paint or in `rectdoc.go` — the `appendEdge` prefix→postfix
fix is length-neutral, and no corpus document strokes an edge. The intended-or-not ruling is the owner's
and is recorded in D-R7.6.

**How we'd know it was wrong.** If re-recording the four goldens does not produce exactly the byte
counts already measured (76,744 / 127,363 / 269,884 / 555,829), something other than alignment is also
moving and the attribution is incomplete.

### D-R7.6 — The alignment change is intended; the goldens are re-recorded and the sign-off re-attested
**Owner decision** (asked at the terminal 2026-08-30, on the evidence in [[D-R7.5]]; the owner chose
"intended, re-record now" over reverting, and over deferring the re-land into Story 7.3).

**Verdict.** `style.align` should render. The pre-`791ed00` output — text drawn at its box's left edge
while the template asked for right — was the defect, and the current output is correct. All four
golden statements are re-recorded to the bytes the engine now produces, in one commit, together with
their `expected.json`, their READMEs and the digest literal in `byte_neutrality_test.go`. The human
sign-off is **re-attested by the owner**, not edited.

**Situation.** Re-recording a golden is the one operation the golden test's own failure text forbids
("DO NOT UPDATE A DIGEST TO MAKE A TEST GO GREEN"), because a digest records that a human once looked
at these pages and accepted them. AD-22 makes a moved digest a versioned behaviour change and C6 makes
an unexplained one a defect. The prohibition is against regenerating *instead of* investigating — it is
not a prohibition on ever moving a golden. The investigation is done, so the question became whether
the new bytes are the ones that should be on record.

**In simple terms.** The reference PDFs are photographs of what the product should look like, and each
one has a signature on the back saying a person checked it. The product has changed in one small way —
a page number that used to sit in the wrong place now sits in the right one — so the photographs are
out of date. You can either take new photographs or undo the improvement. Taking new photographs is
correct, but the old signature was given against the old pictures, so it does not transfer: somebody
has to look again and sign again. What you must never do is take the new photograph and move the old
signature across, because then the signature stops meaning anything.

**Options considered.** (a) *Unintended — revert the alignment rendering.* Rejected: the new output is
demonstrably correct (the arithmetic in D-R7.5 shows the text now ends exactly at the content right
edge), and reverting would make Story 7.3's premise false — 7.3 is written assuming `align` already
renders and adds `justify` to that same closed set, so Epic 7 would inherit an alignment feature it
currently gets for free. (b) *Intended, but revert now and re-land inside Story 7.3*, keeping the
goldens green in the interim. Rejected by the owner: it costs a revert and a re-land to buy a delay,
and it defers the sign-off past the point where `folio-go/v0.1.0` gets cut. (c) *Intended, re-record
now* — chosen.

**Why this wins.** The cost of re-recording is lowest right now and rises sharply: `version.go` still
reads `0.0.0-dev` and `folio-go/v0.1.0` has never been cut, so this is not yet an AD-22 release event.
Once Story 15.3 tags v0.1.0, the same move becomes a breaking change against a published artifact. The
accepted cost is one genuine visual pass over four documents by the owner, which cannot be delegated,
automated, or carried forward.

**Consequences.**
- The sign-off is invalidated **in whole across all four documents** (D-4.7.1), not per-document. The
  story re-records the goldens and then **HALTS** with blocking condition `human sign-off required`,
  leaving `fixtures/statement-signoff.json` byte-identical to `09bb30e` and its test red. That red is
  the story working. No agent may write, edit or synthesise the `reader`, `date` or `examined` fields —
  a filled-in attestation nobody performed is a fabricated record, and the orchestrator obtains the
  real one.
- The four PDFs are rendered to `_bmad-output/implementation-artifacts/evidence/15-1/` for the owner to
  open. The only visible change is the footer's `Page N of M` moving ~86pt right to sit flush with the
  right margin.
- **`valign` still has zero corpus coverage.** The same commit shipped `textValignOffset` and no fixture
  anywhere declares `valign`, so nothing would catch a regression in it. AC4 obliges either a fixture or
  a deferral with a named owner and trigger.
- Epic 7's stories may now rely on `style.align` rendering. Story 7.3 extends the closed align set with
  `justify` rather than introducing alignment.

**How we'd know it was wrong.** If the owner's visual pass finds anything wrong on the re-recorded
pages beyond the footer's new position, the attribution was incomplete and the re-record must stop —
D-R7.5 predicts exactly four moved goldens with byte counts 76,744 / 127,363 / 269,884 / 555,829, and
any other difference falsifies it.

### D-R7.7 — The sign-off gate is unfalsifiable, and this run does not fix it
**Orchestrator decision** (routing, not remediation — the fix belongs to a story this run does not own).

**Verdict.** Story 15.1's review found, and demonstrated, that
`fixtures/statement-signoff.json`'s gate cannot distinguish a genuine re-attestation from someone
pasting four new digests over the old prose. It is **deferred**, not fixed, and this run does not
touch it. Its existence is disclosed to the owner **before** the attestation is requested, which is the
only mitigation available inside this run.

**Situation.** `statementSignOffFieldProblems` (`folio-go/statement_golden_fixture_test.go:237-252`)
checks only that `reader`, `date` and `examined` are **non-empty**. `statementSignOffStaleness` and
`byte_neutrality_test.go` compare only digest equality. A reviewer edited nothing but the four digest
strings — leaving `reader "Panit Wechasil"`, `date 2026-08-27`, and prose describing pages whose footer
sat at the box's *left* edge — and **both gates went green**. The file was restored. Nothing asserts the
record is new: no check that the date advanced, that `examined` or `reader` changed, or that the record
names what it supersedes.

**In simple terms.** The signature on the back of the photograph is checked for being *present*, never
for being *recent* or *about this photograph*. So the cheapest way past the gate is to keep the old
signature and swap the picture — which is exactly the failure the gate exists to prevent, and exactly
what an agent under pressure to turn a test green would reach for. The gate has been this way since it
was written; Story 15.1 is simply the first time anything load-bearing has rested on it.

**Why it is not fixed here.** The intent contract's `Never` assigns CI and gate restructuring to Story
15.2, and this is pre-existing code untouched by 15.1's diff. Widening 15.1 to fix it would mean editing
the very gate whose verdict 15.1 is currently waiting on — a conflict of interest in the literal sense.
The accepted cost is that the gate stays weak until 15.2, and that the integrity of *this* attestation
rests on process rather than on a check.

**Consequences.**
- **No agent in this run may write `reader`, `date` or `examined`.** Recorded in the intent contract as a
  `Never`, and honoured: `git diff 09bb30e HEAD -- fixtures/statement-signoff.json` is 0 lines.
- The orchestrator disclosed the weakness to the owner in the same message that requested the pass, so
  the owner knew the record rests on their actually opening the files.
- Story 15.2 inherits it. It should assert the record is *newer* than the goldens it covers and names
  what it supersedes — not merely that three strings are non-empty.

**How we'd know it was wrong.** A future sign-off whose `date` is older than the goldens it attests to,
or whose `examined` prose describes a rendering the current bytes no longer produce.

### D-R7.8 — The "second permanent red" deferral is transient and closes with the sign-off
**Orchestrator decision** (a correction to a finding, recorded so the closer does not carry it forward).

**Verdict.** Story 15.1's second high deferral — that the story leaves a second permanently-expected red
which `.github/workflows/ci.yml`'s single-scalar `KNOWN_RED_TEST` rule forbids appending to — describes
the **halt state, not the closed state**. `TestGoldenDigestAgreesAtEveryDeclaredSite` is red only while
the sign-off is outstanding. A genuine attestation turns it green, and no second permanent red exists.
The reviewer was right about the halt and wrong about the consequence.

**Consequences.** The closer must verify this empirically — the suite must show **exactly one** red
(`TestCorpusMeetsP6ExerciseFloors`, the mandated P6g floor) once the sign-off lands — and must not carry
the deferral into `deferred-work.md` as an open item against Story 15.2. `KNOWN_RED_TEST` stays a single
scalar and is not touched.

**How we'd know it was wrong.** The sign-off lands and `TestGoldenDigestAgreesAtEveryDeclaredSite` is
still red — which would mean the digests, not the attestation, are the problem.

---

## Story 7.1 — rulings

Story 7.1's plan dispatch halted `blocked` / `intent gap` on three forks. All five entries below are
the engineering lead's RULINGs, verified by the orchestrator where a ruling turned on a quoted fact,
and applied to the spec before re-dispatch. **No owner escalation was needed on any of them.**

### D-7.1.1 — A mandatory break survives inside a declared-unbreakable value, and AD-25 is NOT amended
**Lead ruling**, orchestrator-verified. This was the load-bearing fork and the one that looked like it
needed the owner.

**Verdict.** Exempt **mandatory** breaks from `spansCover`; every **optional** opportunity inside an
atomic span stays suppressed exactly as today. The exemption is keyed on the opportunity's **kind**, at
the filter site (`folio-go/internal/text/opportunity.go:170`) — never on "is this rune `\n`", and never
by shrinking or excluding the span. `atomicSpansFor` (`wrap.go:262`) is unchanged.

**The situation.** `atomicSpansFor` builds a span for every substituted value on a declared
`unbreakableValues` path — by construction, "whatever its script and whatever its content". `spansCover`
then drops any opportunity strictly interior to such a span. So the naive implementation **silently
deletes** the author's break, straight against Story 7.1's AC2. Verified by the orchestrator at
`opportunity.go:170` (`if spansCover(atomic, o) { continue }`) and `opportunity.go:182`.

The apparent conflict is inside 7.1's own `Covers: FR46 · AD-25` line. AD-25's third override says
"no break opportunity survives inside a substituted value from a listed path"; FR46 says a typed break
must survive.

**In simple terms.** A template can mark a field "never break this" — it exists so a Thai surname is not
split down the middle, because 83% of Thai surnames are two ordinary words joined and no algorithm can
tell the join from a word boundary. The question was whether that mark also throws away a line break
somebody deliberately put in the text. It does not, and the distinction is the ordinary one between
*guessing* and *being told*: the rule stops the engine inventing a break where it is unsure, and a line
feed sitting in the data is not the engine being unsure about anything.

**Why no amendment, and why no owner.** D-2.1.6 is an OWNER decision, so amending it would have been an
escalation. It does not need amending. Its Verdict sentence — read at
`folio-mvp-decision-log.md:4959` and confirmed verbatim by the orchestrator — is:

> "A template may declare that a bound value must never be broken. **The engine never proposes a break
> inside such a field.** Declarative, not inferred."

It binds what the engine **proposes**. A line feed present in the input is not the engine proposing
anything, so honouring it is outside what the owner ruled. AD-25's own text scopes the same way: its
three constraints "sit **under** whatever the dictionary proposes, and all override it", and it closes
"the engine's contract is **break opportunities**, not word segmentation" — the third override binds the
opportunity SET, and AC2 defines a mandatory break as expressly not a member of it. The two texts never
meet. Third support: the alternative is a **silent data mutation**, which AD-14 ("never silent") and
D-1.4.9 ("nothing is dropped, nothing is refused") treat as fatal.

**The correction that changes the fixture — this is the part worth the routing.** `bind.Substitution`'s
`Start, End` are rune indices covering **only what a placeholder substituted**
(`folio-go/internal/bind/text.go:37`, confirmed by the orchestrator). So **a line feed the template
author types into an element's literal text is never inside an atomic span at all** — the collision
cannot arise for it. The conflict exists only for a `\n` arriving **through data** on a declared path,
where the "author" is the data supplier. **A fixture that puts `\n` in template literal text proves
nothing and passes vacuously.** The discriminating fixture must supply the line feed through data on an
`unbreakableValues`-declared path.

**Consequences / guardrails.**
- One fixture red-proves both directions: a declared-unbreakable value containing **both** a `\n` and a
  space; assert the `\n` breaks and the space does not. Either assertion alone cannot distinguish
  "exemption works" from "span suppression broke".
- The exemption must be red-provable by flipping the kind test, not by deleting the span — deleting it
  reddens for the wrong reason.
- 7.1 carries a D-000.6 **clarifying** edit: AD-25's third override gains a scope clause saying it binds
  *inferred* break opportunities, not literal control characters in the input. **State the clause; do
  not change the rule.** Same sentence into `folio-format.md`'s `unbreakableValues` prose.
- D-2.1.6's disclosed cost is untouched: a Thai name in free-form text is still guarded only by the
  atomic-unknown-run rule.

**How we'd know it was wrong.** A declared-unbreakable value splitting at anything other than a literal
control character the caller supplied.

### D-7.1.2 — A leading or trailing line feed produces its empty line
**Lead ruling.**

**Verdict.** The separator model applies uniformly. `"a\n"` is two lines, the second empty and occupying
one full `Advance`; `"\na"` is two lines, the first empty. Achieved by **scoping** the two blockers, not
by special-casing: `opportunity.go:124`'s `i > 0 && j < n` guard stays as-is for whitespace and does not
extend to mandatory breaks, and `packLines`' `for start < totalRunes` loop gains the ability to emit a
final zero-length line when the input ends on a mandatory break.

**In simple terms.** If a pasted clause ends with a blank line, the author gets a blank line. Under the
alternative, a trailing blank line is *inexpressible* — you could never produce one — and the character
is thrown away without saying so. Under this rule, an author who wants no blank line just deletes the
newline. Both outcomes stay reachable, which is the test.

**Why this wins.** Expressibility: a reading that leaves an input inexpressible loses to one that does
not. `opportunity.go:124`'s guard is scoped by its own comment to a break that "gains nothing" — for a
mandatory break the empty side is the entire point, so the guard's rationale does not reach it, and
extending it would be the packer *declining* a mandatory break, which AC2 forbids in terms. Story 7.4
makes it concrete: pasted word-processor text routinely ends in a newline.

**Consequences.** Corpus-neutral by construction (no existing fixture value contains a line feed) —
assert that rather than assume it. The empty line occupies one `Advance` and does **not** move
`FirstBaseline`: D-2.5a/DW-15's two-model split must hold identically for empty lines. `textBlockHeight`
must count it — that is the number `textValignOffset` distributes slack against and the quiet path to a
wrong page break. A value that is a single `"\n"` (two empty lines) must not become a zero-line element
or a nil deref.

**How we'd know it was wrong.** An element whose rendered height disagrees with its line count, or a
`\n`-only value producing no line box at all.

### D-7.1.3 — The change lands in the shared packer, for every caller
**Lead ruling.**

**Verdict.** Every caller. One packer, one mandatory-break rule. A `\n` in a table cell's bound data
breaks that cell's line exactly as it does in a text element. No text-element-only flag, no second
packer.

**The question behind it.** Epic 7's header says it "changes nothing about pagination". Does that mean
"does not change the pagination MODEL", or "no template's page breaks may move"?

**Why the model reading wins, from inside the epic.** Story 7.2's fifth AC reads: "the wider line extents
feed `internal/layout` unchanged, **so page breaks follow the spacing rather than ignoring it**." Under
the strict reading that AC is unsatisfiable — an in-epic contradiction. The reading that leaves every
clause standing is that `paginate.go`'s four rules and D-2.6.1's window model are inputs and are
untouched; a page break moving because a line count changed is the window model **working**, not being
changed. Restricting by caller would also mean a second rule for one character, when
`table_render.go:878` already declares it is "the SAME packer text elements use" — a second source of
truth for what `\n` means.

**Consequences.** The spec states the consequence rather than discovering it: a `\n` in bound data
changes a cell's line count, hence its row height, hence where the table breaks. That is intended.
**Re-assert, do not assume, that a row stays atomic** — a `\n` must never become a way to split a row
across pages, and that property is currently held by code nobody is changing, which is exactly when it
breaks. A cell with enough line feeds to exceed the content window is Story 4.6's existing case:
`Pagination.Clipped`, a Warning beside the bytes, **no new diagnostic code**.

**How we'd know it was wrong.** A row splitting across a page boundary, or a new diagnostic code minted
for over-tall cells.

### D-7.1.4 — DW-24 is declined by 7.1, moves to Story 7.3 with two addresses, and its scope is amended now
**Lead ruling**, overriding the orchestrator's initial assignment of DW-24 to 7.1.

**Verdict.** 7.1 declines DW-24. New owner: **Story 7.3**, with the **orchestrator's gate checklist as a
second standing address** — explicitly **not** "Epic 7 close". 7.1 amends DW-24's text in its own record
to add the four `table_render.go` rounding sites.

**Why declined, on the criterion rather than the cost.** DW-24's stated hazard is the unexercised
**rounding** branch — `center`/`middle`, `geom.ScaleRound(slack, 1, 2)` at `text_alignment.go:56`/`:74`,
where a half-to-even tie is what the four-target matrix exists to catch. **7.1 touches neither the
rounding nor the population that reaches it**: no corpus document declares `center` or `valign`, and 7.1
adds none. 7.1 leaves the gap exactly as it found it, so closing it there discharges nothing 7.1
endangered. The `multiple-goals` cost is real but secondary — "a criterion that yields to a budget stops
being a criterion".

**Why not "Epic 7 close".** Precedent: DW-14's owner was "the Epic 2 boundary gate", which ran and closed
without re-owning it, and it survived a whole epic with nobody holding it — D-000.73's class, where an
owner that is an *event* stops existing the moment the event passes. DW-21 already fixed this by naming
two addresses. Use that shape.

**Why 7.3 is a real forcing function.** It extends the closed align set, must author an aligned fixture
anyway, and its slack-remainder rule is itself new integer rounding in the same neighbourhood. A
re-deferral owes a new trigger, and that is one.

**Consequences.** The **scope amendment lands in 7.1, not 7.3** — a deferral whose scope is known wrong
is worse than one merely open, because the literal closing fixture would satisfy DW-24's text while
missing `table_render.go:687/698/1017/1193` and the entry would be marked closed. A centred *text*
element does not cover the table sites: different code, same rounding. **At closure the enumeration must
be re-derived by grep and recorded, not read off the amended hand-list** — the hand-list is being amended
today precisely because it had already rotted once. 7.1's Delivery Log names DW-24 as
inspected-and-declined with this ground, so the next reader sees a decision rather than an omission.

### D-7.1.5 — The break-kind seam is two fields, and Story 7.1 lands both
**Lead ruling**, on an orchestrator finding.

**Verdict.** Two fields, not one. (1) `Opportunity` gains the break **kind** — 7.1 needs it, reads it,
and it is what makes AC2 true. (2) `wrappedLine` gains **the kind of break that ended this line** — 7.1
writes it, Story 7.3 reads it. Both land in 7.1. The last-line case stays **derived** from the index at
7.3's call site and is never stored, so there is one source for it.

**The finding.** `wrappedLine` is `{from, to, width}` (`wrap.go:152`, orchestrator-verified) and
`packLines` holds the winning opportunity in `op`/`chosen` then writes only those three fields
(`wrap.go:217-234`). The information is **destroyed after packing**, and 7.3's AC needs it: 7.3 must not
justify "the last line of a paragraph, **or a line ended by a mandatory break**".

**Why 7.1 carries a field it never reads.** Reconstructing it in 7.3 would be a second derivation of
where a break came from — the hazard `verticalMetrics`' own doc comment says that type exists to close.
It lands at the site that already knows. Precedent and its handling are established:
`verticalMetrics.LastDescent` shipped stating honestly in its comment that nothing in production consumed
it, and was asserted directly by a test over fabricated metrics in the meantime.

**Consequences / guardrails.** 7.1 asserts the `wrappedLine` field **directly** in a test over fabricated
input, and its comment names Story 7.3 / FR47 as the arriving consumer — an unread, unasserted field is
D-000.46's shape and is not authorised. A **named kind, not a bool**: 7.3's rule is two independent
conditions and a bare `!hardBreak` invites the third case to be re-derived wrongly. 7.1's spec states
that 7.3 depends on this field — D-R7.3's numeric order satisfies the dependency by accident, and relying
on an accident is how it gets reordered later.

### D-7.1.6 — Whitespace adjacent to a mandatory break is consumed with it
**Orchestrator decision** (routine — Story 7.1's AC5 settles it unambiguously, so it took the fast path
rather than a lead round-trip. Recorded here because it changes rendered output and the planner
explicitly invited pushback on it).

**Verdict.** A whitespace run containing one or more line feeds emits **mandatory** breaks *instead of*
rule 1's single optional opportunity, and inherits rule 1's consumption at the run's outer edges. So
`"a \n b"` renders as `a` / `b` with neither the space before nor the space after drawn on either line.

**Situation.** The three ruled forks were silent on whitespace *adjacent* to a line feed. Rule 1 in
`internal/text/opportunity.go` walks a whitespace run and emits one opportunity spanning the whole run,
so the run is drawn on neither line. A line feed is itself whitespace, so a run like `" \n "` is one run
today.

**Why this and not "consume only the line feed".** Story 7.1's AC5 says the break character "is consumed
and drawn on neither line, **exactly as a whitespace break already is**" — the comparison is the
instruction. `Opportunity`'s own doc comment already models this: `LineEnd` and `NextStart` "differ only
where the break CONSUMES text — today exactly one case, a run of whitespace", and it states that
modelling the consumed range explicitly is what keeps "the trailing space does not count toward the
line's width" a property of the break rather than a special case at the measuring site. Consuming only
the `\n` would leave a trailing space measured into the previous line — a new polarity nothing asked for,
and precisely the special-case-at-the-measuring-site that comment exists to prevent.

**Consequences.** No new consumption model: the existing `{LineEnd, NextStart}` pair already expresses
it. A run holding *k* line feeds emits *k* mandatory breaks, producing *k−1* empty lines between them,
which is what D-7.1.2's separator model requires.

**How we'd know it was wrong.** A justified or right-aligned line whose measured width includes a
trailing space — the polarity error this avoids.

### D-7.1.7 — Two silent-failure sites the rulings did not name, found at the plan gate
**Orchestrator finding**, verified in the code before accepting the spec. Recorded because both are
places where Story 7.1 would pass its tests while failing its ACs.

1. **A leading empty line is unreachable, and D-7.1.2 did not name the site.** The ruling scoped
   `opportunity.go:124`'s `i > 0 && j < n` guard and `packLines`' loop, which together cover `"a\n"`. They
   do **not** cover `"\na"`: the collection loop at `opportunity.go:165` is `for i := 1; i < n; i++`, so
   `LineEnd == 0` is unreachable and the leading empty line can never be emitted. Verified.
2. **`packLines` bypasses the opportunity list entirely when the remainder fits.** `wrap.go:190` reads
   `if w := measureRuneRange(segs, start, totalRunes, fontSize); w <= maxWidth { … break }`. Verified.
   **This is exactly where AC1 is won or lost** — AC1 requires the break be taken "regardless of how much
   width remained on the line before it", and this short-circuit is the path on which a mandatory break
   inside text that otherwise fits would be silently ignored. A test using text too wide for its box
   would never reach it and would pass green.

**Consequences.** Both are carried in the spec as explicit hazards and task steps. The AC1 test must use
a value that **fits** its declared width, so it exercises the short-circuit rather than the packing loop.

**How we'd know it was wrong.** A `\n` in a short value rendering as one line.

---

## Canvas projection bounds — rulings carried into Story 7.4's plan gate

Raised by the orchestrator at Story 7.1's close, after the implementer deferred `maxCanvasTextLines`
as `low`. The lead **widened the predicate** and found a bound that binds far earlier. Filed as
**DW-25**. All code claims below were verified in the tree by the orchestrator.

### D-7.4.1 — Story 7.4 owns the canvas bounds, and the obligation is not "raise 256"
**Lead ruling**, orchestrator-verified.

**Verdict.** Owner is **Story 7.4**, with the orchestrator's 7.4 plan-gate checklist as a second
standing address — not 7.6, not a new story, and explicitly not "Epic 7 close" (D-000.73: an owner that
is an *event* stops existing when the event passes). The obligation is: **no canvas projection bound may
abort the projection, and the bounds Epic 7's own input makes routinely reachable must admit that
input.** At minimum `page_setup.go:558` and `:456`.

**The finding that changed the question.** The orchestrator raised `maxCanvasTextLines = 256`
(`page_setup.go:27`, enforced `:456`). The lead found that **it is not the bound that stops Story 7.4**:

```go
// page_setup.go:557-560, in canvasComponents — a DIFFERENT function from the paint loop
if element.Type == template.ElementText && element.Value.Set && !element.Value.Null {
    if len(element.Value.Value) > maxCanvasPropertyString {
        return nil, fmt.Errorf("folio: component value exceeds the projection bound")
```

A text element's value is capped at **512 bytes**, and overrunning it returns `nil` for the **entire
component list** — the whole canvas with no components, worse than the `:456` case and firing far
earlier. Verified.

**In simple terms.** 512 bytes is about eighty English words — less than one numbered contract clause.
Story 7.4's promise is "type and paste a multi-paragraph clause". And because `len()` on a Go string
counts **bytes**, not characters, Thai and CJK cost three bytes per character, so the ceiling lands near
170 characters for exactly the two scripts NFR3 makes first-class. An author writing a Thai contract hits
the wall at a third of the length an English author does. To reach the 256-line cap with real prose you
must pass 512 bytes first, so in practice the value cap fires first for every realistic Epic 7 input; 256
is reachable only by a value that is almost entirely line feeds.

**The diagnosis underneath both.** `maxCanvasPropertyString` does **two different jobs**. At `:211`,
`:567`, `:573`, `:617`, `:642`, `:648`, `:663` it bounds **identifiers, colours and expressions** —
legitimately short. At `:558` it bounds **document body text**, which is not. One constant, two
categories. **The fix is to split it, not to change its value.**

**Why 7.4 and not elsewhere.** 7.4 is where the condition first becomes reachable through the product —
the same logic D-000.65 applies to minting. Before 7.4 the properties surface is single-line; 7.4 hands
the author the input, and its first AC is "the editor accepts and preserves multiple lines". An AC that
cannot be demonstrated without lifting these bounds is not a neighbouring concern, it is 7.4's own
acceptance. 7.6 is wrong — it draws pages and neither bound is a function of page count. 7.1's
implementer deferred correctly (7.1's contract forbade designer-surface work); 7.4 is a designer-surface
story, so that constraint is gone. **The severity was wrong, not the deferral.**

**Consequence.** 7.4's Delivery Log must state that it verified an **actual pasted multi-paragraph clause
reaches the canvas** — not that a constant was changed. A constant edit with no end-to-end demonstration
is the shape that lets this survive.

### D-7.4.2 — Degrade per element; never abort; mint no diagnostic code
**Lead ruling.**

**Verdict.** Shape **(b)** — degrade the offending element, keep the projection. The pattern already
exists in the tree eleven lines above the site in question and must be reused, not reinvented:
`page_setup.go:428-435` handles a `fontChain` failure by setting
`component.TextPaint = &CanvasTextPaint{Lines: []CanvasTextLine{}}` and `continue`ing, with the rationale
"Existing designer documents can be structurally valid while incomplete for production rendering … They
remain loadable; there is simply no honest measured paint to display yet." Verified; that reasoning
covers this case verbatim.

**Options rejected.** (a) *Raise the constant* — rejected on the criterion, not the cost: moving a
threshold so a symptom stops reporting is the twin of manufacturing data to meet a floor. **The cliff is
the defect; its position is not.** (c) *A document-level limit with a located diagnostic* — rejected
because there is no code to mint. The render path has **no such cap**: a 400-clause document renders
correctly to PDF, so there is no render-time condition and D-000.65 never fires. Worse, it would let the
**canvas refuse a document the engine renders**, inverting AD-15/AD-16 and the
canvas-approximate/preview-exact asymmetry that is the product concept. **A canvas projection bound must
never become a document validity rule.**

**Guardrails — the first two carry most of the weight.**
1. **Truncate the paint, never the value.** `CanvasTextPaint.Lines` is regenerated every projection and
   never written back, so bounding it is safe. `component.Value` is what the properties panel edits and
   **saves** — truncating it would write the truncation back into the document and destroy the author's
   text. So `:558` gets body text its own genuinely large bound (a channel-size concern, megabytes, not
   an identifier bound), and the degradation lives on the paint side alone.
2. **The degraded state must be distinguishable from the empty one.** As written, a 400-line element and
   an element with no text both project `Lines: []` — the all-clear wearing the face of could-not-look.
   Add a field beside the existing `Overflow bool` (already the precedent for a paint-plan flag naming a
   degraded state) and **paint the first N lines rather than none**, so the author sees their text and is
   told it is cut. A projection field, not a `diag.Diagnostic`; no registry entry.
3. **Split `maxCanvasPropertyString`.** It stays 512 for identifiers, colours and expressions (all seven
   other sites are correct at that size). Body text gets its own named constant.
4. **Whatever number replaces 256 must be derived and recorded, not chosen round**, with the criterion in
   the constant's comment. Epic 7's narrative target is "forty pages"; at ~45 lines per A4 page at 11pt,
   256 lines is about six pages — short of the epic's own stated target by most of an order of magnitude.
5. **Verify, do not assume, that truncating the paint leaves Story 7.5's window count intact.** 7.5's AC
   has the projection report "how many windows the column currently occupies"; that must come from layout,
   not from `CanvasTextPaint`. If a truncated paint can shorten the reported column, 7.6 draws the wrong
   number of sheets and **the canvas lies about pagination**. Assert the independence.
6. The other seven `maxCanvasPropertyString` sites keep aborting, and that is **recorded, not fixed**:
   Epic 7 makes none of them newly reachable. Naming the population is what stops the next reader
   believing the whole class was closed.

### D-7.4.3 — The three canvas constants are not a coordinated budget, and 256 protects nothing
**Lead ruling**, answering the orchestrator's question of whether raising one number was safe.

**Verdict.** `maxCanvasTextLines`, `maxCanvasPropertyString` and `maxCanvasTextFragments` are **three
independent defensive caps that happen to share a const block** — not a projection-size budget.
`maxCanvasTextLines` protects no invariant and may be raised or replaced on its own merits.

**Grounding.** They bound three different axes at three different granularities: a string length at eight
sites; fragments **per line** (`:499`); lines **per element** (`:456`). Their product, 256 × 512 × 512, is
not a coherent total anyone budgeted. The only representational invariant in the file — nothing
unrepresentable crosses the JS boundary (AD-15/AD-16) — is already enforced **per value** by
`canvasLineTop` and `canvasDerived` against `MaxCanvasMillipoints`, each with its own error. 256 lines at
a typical ~14,000 mp advance is ≈3.6e6 against 9.007e15: **ten orders of magnitude of headroom**. What 256
does bound is projection payload size across the worker channel — a real concern and a legitimate reason
to keep *a* cap, but a payload budget **degrades, it does not abort**, which is D-7.4.2.

### D-7.4.4 — Watch item: the canvas breaks the raw template string, not bound data
**Lead observation**, not a decision. Recorded so it is not discovered inside a story that assumed
otherwise.

`page_setup.go:449` calls `atomicSpansFor(t.doc.UnbreakableValues, nil)` — the canvas passes **nil**
substitutions, so **no atomic span ever exists on the canvas**, and the canvas breaks the raw template
string (`{{...}}` placeholders and all) rather than the bound data. Correct as designed, and untouched by
D-7.1.1, which has no canvas-side effect.

**Consequence.** Stories 7.4 and 7.6 both lean on canvas/PDF agreement. **If any story claims the canvas
shows where the engine will break a _bound_ value, that claim is false today.** To be raised at 7.6's plan
gate rather than decided now.

---

## The `.folio` format version — rulings and the owner's reframing

### D-R7.9 — Nothing is tagged, so the MAJOR bump is free and no release is scheduled
**Owner decision** (asked at the terminal 2026-08-30). **The owner REFRAMED the question rather than
choosing an option, and the reframing is better than any of the four offered.**

**Verdict, in the owner's words:** *"We haven't released anything to production yet so no need to tag
now."* So Epic 7 takes the MAJOR bump to `.folio` 2.0 freely at Story 7.3, Epic 8's format change joins
the same 2.0 at Story 8.3, and **no release is cut in or around this run**. Story 15.3 is not imminent
and must not be treated as a deadline.

**Why the reframing beat the options.** All four options I offered assumed a tag was coming and argued
about where to put it relative to the epics — the lead's recommendation was "land both epics, then tag",
and two of the remaining three were about paying to avoid a MAJOR that a tag would make permanent. The
owner removed the premise. **There is nothing to sequence against.** The load-bearing fact the lead
found — that a MAJOR costs per *released version*, not per feature — becomes unconditional rather than a
race: with no tag pending, the second format extension is free, and so is the third. That dissolves the
"cheaper before the tag" urgency that framed the whole question, and it removes the trade-off the lead's
own recommendation had to accept (a slipped tag).

**In simple terms.** A format MAJOR is expensive because it orphans everyone already using the old one.
Nobody is using this yet — no tag, no consumers, `version.go` still reads `0.0.0-dev`. So the change that
would be costly later is free now, and the right move is to make it properly rather than to design around
a cost that does not exist yet.

**Consequences.**
- Story 7.3 raises `SupportedMajor` to 2 without further escalation. Story 8.3 joins the same 2.0.
- **The build order does not change.** 15.3 was never in this run's scope; this only removes the pressure
  to reach it.
- Options 3 (`style.justified` as an additive key) and 4 (cut `justify`) are **rejected** and should not
  be re-proposed: both existed only to dodge a cost the owner has just said is not being incurred, and
  option 3 additionally reintroduces the silently-wrong render D-1.4.12 exists to prevent.
- When 15.3 is eventually planned it must re-make the release decision from scratch — its own AC already
  says the "cut after Epic 6" trigger has been overtaken by events. This entry is now part of what it
  re-makes.

**How we'd know it was wrong.** Someone outside the project starts depending on `folio-go` from `main`
before a tag exists — at which point the MAJOR stops being free and this decision needs revisiting.

### D-7.3.1 — `justify` extends a closed set named literally in D-1.4.12, so Story 7.3 is a MAJOR-bump story
**Lead ruling**, orchestrator-verified. **This also corrects the lead's own grounding report.**

**Verdict.** D-1.4.12 reaches `align` **by name**, with nothing to interpret. Its verdict is the mandatory
`folio-format.md` sentence and it enumerates the eight closed sets literally: element `type`, `locale`,
**`align`**, **`valign`**, `columns[].footer`, `border.edges`, `page.orientation`, `page.size`.
Extending any of them is MAJOR "because every existing library validates those sets as load errors".
Verified in code: `closedAligns = {left, center, right}` at `internal/template/closedsets.go:30`,
enforced as a load error at `parse_bands.go:399` and `:531`. A 1.0 library hits `justify` and refuses the
file — D-1.4.12's stated mechanism exactly.

**The correction.** The orchestrator hypothesised a distinction between extending a **value** set and
extending a **shape** set, which would have let `align` through as MINOR. **It does not exist, and the
asymmetry runs the other way:** all eight named sets are value sets, so `align` is the textbook case,
while Epic 8's chain-entry *shape* set is not on the list and reaches D-1.4.12 only by analogy. The lead's
grounding report had framed 8.3 as the D-1.4.12 instance and missed that 7.3 is the cleaner one — the
lead owns that correction. **The real instance is four stories earlier than anyone thought.**

D-1.4.12 had already considered and rejected per-set treatment, with `align` named in the rejection: "on
inspection every one of the eight genuinely cannot be faked by an older library … an unknown `align`
would be silently wrong." And its own falsifier fires precisely here: "How we'd know it was wrong. A
MINOR release adding a closed-set value."

**Guardrails for Story 7.3, both concrete and both verified.**
1. **`closedAligns` is a single map SHARED between `style.align` (`parse_bands.go:531`) and
   `columns[].align` (`:399`).** Adding `justify` to it silently legalises `columns[].align: "justify"` —
   justified table cells, which Epic 7 never specified and nothing implements. **Split the set:**
   `columns[].align` keeps `{left, center, right}`; only the style set gains `justify`. Justified columns
   would be a separate scope decision, not a side effect of a map edit.
2. Both call sites carry a **hand-written** error string, "not one of the closed set left, center, right".
   Derive the message from the set, or 7.3 ships two error messages that lie about what is legal.

### D-7.2.1 — `style.lineSpacing` is MINOR, and Story 7.2 inherits an unrecorded version debt it must discharge
**Lead ruling**, orchestrator-verified.

**Verdict.** `style.lineSpacing` is a new optional key with no closed set, so D-1.4.12 does not reach it
and D-1.4.9's "may add new optional keys only" makes it **MINOR**. But 7.2 cannot merely declare that —
it is the first story able to discharge three things nobody has, and it must.

**The debt, measured in the tree.**
1. `SupportedVersion = "1.0"`, `SupportedMajor = 1` (`internal/template/version.go:24-25`), and **all 18
   fixtures declare `"1.0"`. The format version has never moved since Story 1.4.**
2. **Epic 10 shipped `style.color` and bumped nothing** — its own prose says "This epic adds one optional
   field, `style.color`". So documents using colour **declare 1.0 while requiring 1.1**. An older 1.0
   library loads them, ignores the key, and renders the text **black**. That is a silently-wrong render
   arriving through the back door — the exact failure D-1.4.12's rejected option (b) would have allowed.
3. **`versionForSave` is still a stub.** `version.go:88` returns `loaded` unchanged, and its own comment
   says why: "At Story 1.4 the raise path is unreachable — no 1.1 exists yet … so that the day a future
   MINOR exists, the raise path has exactly one place to be filled in." **That day is Story 7.2.**
   Bumping `SupportedVersion` alone changes nothing; without the raise path no document is ever raised and
   the constant is decorative.

**Consequences.** Story 7.2 owns: implement D-1.4.13's raise path at the one place reserved for it; raise
a document to 1.1 when it introduces `lineSpacing`; and **retrofit the same for `style.color`** so Epic
10's documents stop misdeclaring. Cheap now.

**A worry that dissolves — Epic 7 does NOT move the version twice.** Under D-1.4.13 `version` is a
property of the **document**, raised only by the content it actually contains: a document using only
`lineSpacing` is 1.1, one using `justify` is 2.0, one using neither stays 1.0, and all three coexist. What
moves twice is the *library's* `SupportedMajor`/`SupportedVersion`, which is a library fact, not a format
event.

**Two version systems, not to be conflated** (they were being, in this run and elsewhere):
- **AD-22 governs the LIBRARY version** (`folio-go`): any change to layout, subsetting, emission or the
  toolchain is breaking. Epic 9's alignment change was an AD-22 breaking change — it moved goldens and
  touched no format key.
- **D-1.4.9 / D-1.4.12 / D-1.4.13 govern the FORMAT version** (`.folio`). Epic 9 changed no format version
  and was right not to.

**Guardrail for whichever story bumps `SupportedMajor`.** Its doc comment calls it "the full MAJOR.MINOR
this library itself would author for a brand-new document". Once that reads `"2.0"`, a blank document
would declare 2.0 while using no 2.0 feature — needlessly orphaning it from every 1.x reader and
contradicting D-1.4.13's raise-only-by-content rule. **A new document must declare the lowest version its
content requires, never the library's ceiling.**

---

## Story 7.2 — rulings

Story 7.2's plan halted `intent gap`: AC7 rejects a `lineSpacing` "outside the supported range" and
**nothing anywhere says what that range is**. Four lead rulings, all orchestrator-verified in code.

### D-7.2.2 — The canvas clause that forbids tight leading is the defect; delete it
**Lead ruling**, orchestrator-verified. This one removes a guard, so it was checked before acting.

**Verdict.** In `folio-designer/src/engine-protocol.ts:207`, remove **`paint.baseline > paint.top +
paint.advance`** from the predicate. Everything else on that line stays.

**Situation.** The canvas emits `baseline = top + FirstBaseline` while `advance` is the scaled value, so
that clause reduces to **`FirstBaseline <= Advance`** — the unscaled relationship `wrap.go:600-612`
guarantees by clamping `maxDescent`/`maxLineGap` at zero. Verified at `page_setup.go:485-497`:
`baseline = canvasDerivedSum(top, vm.FirstBaseline)`, `advance = vm.Advance`. It is **not an independent
property**; it is a restatement of an engine invariant that `lineSpacing` deliberately dissolves, sitting
on the wrong side of the channel. The predicate lives inside `isTextPaint`, so one bad line fails
`isTextPaint` → `isCanvas` → `isSnapshot` and blanks the **whole projection**. Measured cliff on the
shipped chain at 11pt (`FirstBaseline: 11759`, `Advance: 14982`): **784 thousandths rejects, 785 passes.** *(Corrected 2026-08-30 at Story 7.4's plan gate: this line originally read "12pt". Those are the **11pt** metrics. The error propagated — D-7.4.2 §4's "~45 lines per A4 page at 11pt" is in fact the 12pt figure; at 11pt it is 48, which is what Story 7.4's derived line bound uses.)*

**In simple terms.** `FirstBaseline > scaledAdvance` means one line's baseline sits below the next line's
top — the line boxes overlap. **That is what tight leading is**, and it is exactly what the PDF will
draw. A guard refusing it is stating a typographic opinion, not checking a consistency property.

**Why removing it is the AD-17 fix, not an AD-17 breach.** AD-17 says the canvas takes every text metric
from the engine and the browser contributes rasterization only. This clause has the **browser refusing
the engine's own honest measurement** and blanking the canvas — the invariant inverted. Story 5.9's guard
is about *where metrics come from*, not about the browser adjudicating them.

**What replaces it: nothing new. The real invariants are already in the same predicate and all survive.**
`paint.advance <= 0` (genuinely required, unaffected); `paint.baseline < paint.top` (still holds —
`FirstBaseline` is `maxAscent` clamped at zero and `lineSpacing` scales only `Advance`); the
`Number.isSafeInteger` checks (the actual JS-boundary concern); and `paint.top < priorTop + priorAdvance`
— **orchestrator-verified not to become the next cliff**: `top = canvasLineTop(originY, i, vm.Advance)`
so `top_i = originY + i·A`, making the test `originY+i·A < originY+i·A`, false for any positive advance.

**How we'd know it was wrong.** If browser paint code ever derived a line's top from `baseline + …`
rather than from the engine's supplied `top`, overlap could corrupt layout. It does not — `top` is
supplied per line.

### D-7.2.3 — The load-time range is representational and encodes no typographic opinion
**Lead ruling.**

**Verdict.** `style.lineSpacing` is a whole number of thousandths in **[1, 1000000]** (0.001–1000.0
inclusive). Outside that, or not a whole number of thousandths, is a located load error naming the
element. **No lower bound at 1.0.**

**Why not a typographic bound.** A load-time check **cannot see the font size**, so it cannot express
one: the canvas cliff is 785 thousandths at 12pt on the shipped chain and *moves with face and size*. A
bound pretending to be typographic while blind to the input that determines it is wrong at every size but
one. So the load-time range is the only thing load time can see — the value's own domain. A 1.0 minimum
was also rejected on product grounds: Epic 7 exists to produce a filed contract's house style, and tight
leading is part of that. Asymmetry with `fontSize` (exactness and overflow checks, no range) is a reason
*not* to invent a bound, not a reason to add one. The 1000.0 ceiling is a **stated sanity ceiling, not a
derived safety bound**, and must say so in the constant's comment.

**Guardrail — the real lower-bound failure is not load-time checkable either.** `ScaleRound(400, 1, 1000)`
is **0**: a small face at `lineSpacing: 0.001` yields `advance = 0`, which the canvas correctly rejects
and which gives layout zero-height lines. Check it **where both operands exist**, in the leading model, as
a located error naming the element and the resolved size. Do not raise the minimum to prevent it — that
just moves the blindness. One validation function, called from both the load path and the property-command
path, so a value refused in a file is refused in the inspector for the same reason.

### D-7.2.4 — Overflow is a separate guard at the computation site, and it is not a number
**Lead ruling.**

**Verdict.** Not the same bound and not a load-time bound at all. Guard `ScaleRound`'s precondition
**where it is called**: check `int64MulOverflows(Advance, r)` before the call in the leading model and
return a located error. **A panic must never be reachable from authored input** — `geom.ScaleRound` panics
on int64 overflow (`internal/geom/scale.go:67-69`, verified).

**Why not at load.** The precondition cannot be discharged there: `Advance` is unknown and `fontSize` is
unbounded. Deriving a load-time ceiling honestly from the extreme case gives `r <= 1023` — i.e. it would
forbid `lineSpacing` above 1.0, which is absurd. That reductio is why D-7.2.3's ceiling is a sanity bound
and nothing more. This is **D-1.5.2's shape** (`unitsPerEm`): validate at the trust boundary, and add **no
non-panicking variant** of `ScaleRound` — AD-2 says scaling is one function and a second door drifts.

**Stakes.** Story 7.1's closer found that a Go panic aborts the package binary, so every other test in
`folio-go` silently stops reporting. **A panic reachable from a template is not a crash, it is a
suite-wide blindfold.**

**Guardrail.** `fontSize`'s missing range check is the pre-existing half of this. 7.2 **records it in
`deferred-work.md` rather than closing it** — closing it is a format-domain decision on a second field and
would earn `multiple-goals`. Name it, noting the overflow site is shared.

### D-7.2.5 — Mint `STYLE_LINE_SPACING_INVALID`
**Lead ruling** under D-000.65 (mint where the condition first occurs).

**Verdict.** Mint `STYLE_LINE_SPACING_INVALID` in `internal/diag`, `SeverityError`, with the public bridge
`folio.DiagCodeStyleLineSpacingInvalid`, raised from the single validation function both the load path and
the property-command path call.

**Why minting is what makes AC7 reachable at all.** `wasm/cmd/engine/main.go:271-280`'s
`reportableMessage` replaces the message with "The template could not be processed" **only** for
`DiagCodeTemplateMalformed`; every other code gets `bounded(message, 512)`. Every load-time style
rejection today is uncoded and becomes `TemplateMalformed`, so **an uncoded `lineSpacing` error is
destroyed before it reaches the author** and AC7's "a located load error naming the element" is
unsatisfiable through the designer. A dedicated code is not registry hygiene here — it is the mechanism
that delivers the AC. The destruction rule's own stated reason does not reach this case: it exists because
a malformed-template message "quotes the offending document back", and an engine-authored message naming
an element id and a numeric range quotes nothing back. `STYLE_COLOR_INVALID` is the standing precedent.

**Guardrails.** Assert the code is **not** `TemplateMalformed` and that its message survives
`reportableMessage` intact — without that, someone later routes it through the generic load-error path and
AC7 fails silently, invisibly from the Go side where the tests are. The code names **the field's value
being outside its declared domain** — the zero-advance (D-7.2.3) and overflow (D-7.2.4) errors are
different conditions at a different stage and **must not be folded into it to save a mint**.

**Forward note, not a decision.** `STYLE_COLOR_INVALID` plus this makes two per-field style codes. Before
a **third** is minted, someone must decide whether the general form is right or AD-14's closed registry
accretes one entry per style field forever. Raise at Epic 11 or 12 planning, whichever first proposes one.

### D-7.2.6 — DW-24 declined a second time, on a narrower ground, and is not deferrable a third
**Lead ruling**, correcting the orchestrator's and planner's reasoning while accepting the outcome.

**Verdict.** DW-24 is inspected-and-declined for 7.2. **But not for the reason given.** The planner argued
"a different call site, denominator and file". In fact **7.2 does change the input to the unexercised
rounding site**: `textBlockHeight` is built from `Advance` and feeds the slack that `ScaleRound(slack, 1,
2)` halves for `valign: middle`. The exposure is unchanged only because **no fixture declares `valign` at
all** — which is the same absence DW-24 exists to record. The decline holds on that narrower ground.

**Condition.** **DW-24 is not deferrable a third time.** Story 7.3 is its owner (D-7.1.4); 7.3's plan gate
treats it as an **acceptance criterion, not a deferred item**, and a decline there is an escalation to the
lead rather than another entry. A trigger that has failed to fire twice stops being a trigger.

### D-7.4.5 — The TypeScript mirror is three hand-copied constants, not one
**Lead ruling**, extending D-7.4.2 after an orchestrator observation.

**Verdict.** `engine-protocol.ts:201-212` mirrors **three** Go constants with no shared source:
`value.lines.length > 256`, `fragments <= 512`, and `fragment.text.length <= 512`. This is the drift
pattern in pure form — the Go side can be raised and the TS side will silently keep rejecting, blanking
the projection with **no error anyone can attribute**.

**Consequence, added to DW-25.** Whichever story changes a Go bound changes its TS mirror **in the same
commit**, and lands **an assertion tying the two** — a test that reads both, not a comment asking the next
person to remember.

### D-R7.10 — The run continues story-to-story without check-ins, through Epic 8
**Owner decision** (given 2026-08-30, mid-run): *"make sure the dev cycles continues after each user
story until finishing epic 8 unless there are important decisions I need to make."*

**Verdict.** The orchestrator chains plan → build → close for every remaining story (7.4–7.7, then
8.1–8.6) without pausing to report between them. It stops **only** for a decision that genuinely
requires the owner. This supersedes the "run continuously, pause only at design decisions" setting from
D-R7.2 by making the endpoint explicit: **Epic 8 complete.**

**Why it was given.** The orchestrator twice ended a turn saying it would continue to the next story and
did not dispatch it — the run stalled with nothing running. That is the failure this instruction closes.

**What still counts as worth stopping for**, so the instruction is not read as "never ask":
- A ruling that touches the owner's own artifacts — the golden sign-off is the standing example, since
  no agent may write a `reader`/`date`/`examined` attestation.
- A scope or release decision with lasting consequence (the D-R7.9 class).
- A defect that falsifies a decision already made.
- An `OWNER-DECISION-NEEDED` block the engineering lead declines to rule on.

Everything else — lead rulings, spec amendments, deferrals, halts the lead can settle — is applied and
logged without interrupting the owner, and reported when the run next surfaces.

**Consequence.** Reports become epic-boundary-shaped rather than story-shaped. The per-story record does
not degrade: each story still gets its `## Delivery Log` entry and its decision-log entries, so the audit
trail is unchanged — only the interruptions stop.

---

## Corrections to this log, filed at Story 7.4's close (2026-08-30)

**Appended, not rewritten.** The rulings below stand exactly as given; what is corrected is three
measurements quoted inside them, each of which later stories were going to reuse.

### 1. `:891`'s vertical metrics are the **11pt** figures, not 12pt

`FirstBaseline: 11759, Advance: 14982` on the shipped chain are **11pt**. The fixture they came
from (`line_spacing_template.go:60-75`) declares `"fontSize": 11`. Re-measured at HEAD on
`["Noto Sans"]`:

| size | FirstBaseline | Advance | LastDescent |
|---|---|---|---|
| 11pt | 11759 | 14982 | 3223 |
| 12pt | 12828 | 16344 | 3516 |

The line in D-7.2.3's neighbourhood already carries an inline correction; this entry is the record
of it, because the error propagated.

### 2. D-7.4.2 §4's "~45 lines per A4 page at 11pt" is the **12pt** figure

A4 content-band height is 729890 mp for the canonical 36pt margins with a 20pt header and footer
(`internal/layout/band.go`, the value already shipped in `App.test.tsx`). So
`⌊729890 ÷ 14982⌋ = **48** lines/page at 11pt` and `⌊729890 ÷ 16344⌋ = 44` at 12pt. Story 7.4's
`maxCanvasBodyTextLines` is derived from **48**, which is the admitting figure of the two, giving
40 × 48 = **1920**.

The §4 argument is unaffected in substance and gets stronger: 256 lines is about **five** pages at
11pt, not six — further short of the epic's own forty-page target, not nearer.

### 3. D-7.4.5's "three hand-copied constants" is **four**

The fourth is `engine-protocol.ts:152-154`'s `optionalString`, which capped an element's `value` at
512 alongside seven identifier and colour keys — `maxCanvasPropertyString`'s two-jobs conflation
reproduced exactly on the browser side. **Without splitting it too, a correct Go-side split changes
nothing observable**: the browser goes on dropping the whole response at 512 bytes of clause text.
D-7.4.5's consequence is otherwise unchanged, and Story 7.4 discharged it: all four are hoisted to
named constants and tied by `folio-designer/src/engine-bounds-mirror.test.ts`, which reads both
files, asserts the pairs non-vacuously, and red-proofs a one-sided edit in either direction.

### 4. One measurement Story 7.4 added, because a later story will want it

**Cumulative fragments ≈ the value's WORD COUNT, at any column width.** Measured through
`CanvasWithTextPaint` on justified English contract prose at 11pt in the **shipped `["Noto Sans"]`
chain** — the same face and size `maxCanvasBodyTextLines` is derived from: **18.05 fragments/line at
523.276pt** (full A4 content width, over 101 lines) and **8.10 at 240pt**, with cumulative totals of
1823 for 1824 words and 1822 for 1824. A short-word worst case at the same width measures 30.86.
The "~73 justified lines" figure in DW-25's Story 7.3 amendment is a 240pt-column figure that was
quoted as a general one; at full A4 width the old browser cumulative cap of 512 was crossed at ~28
lines. Quote the law, never a lines figure.

**This measurement moved a constant.** 1920 × 18.05 = 34 656 is above 32 768, so
`maxCanvasBodyTextFragments` as first shipped did not cover the forty-page criterion its own comment
claimed. It is **65 536** — the next power of two above the measured product, which also clears the
short-word case at 1920 × 30.86 = 59 251. `page_setup.go`, `engine-protocol.ts`,
`engine-bounds-mirror.test.ts` and `deferred-work.md` all carry the one figure. Earlier numbers in
circulation (16.72, a thirteen-line sample; 19.35, the `Roboto-Regular` test face) are superseded.

### 5. A pre-existing performance characteristic, observed and not fixed

`packLines` is **superlinear in a value's break-opportunity count**: measured through
`CanvasWithTextPaint` on one justified element, 1.2 s at 4,000 word opportunities and 9.8 s at
8,000. This is why Story 7.4's cumulative-fragment assertion exercises the projection's own budget
rule rather than a document that reaches 32,768 fragments — such a fixture would cost minutes of
wall clock in the ordinary suite. Nothing in Epic 7 makes it reachable through the product; recorded
so the next reader does not conclude the test was written that way out of convenience.

---

## Story 7.7 gate + two cross-cutting rulings (2026-08-31)

### D-7.7.1 — The Epic 7 scope fence is re-anchored on the invariant, not on a filename
**Lead ruling.** 7.7 is the deliberate exception to "`paginate.go` absent from the diff": its AC says the
group *"reuses that machinery"* — `ItemGroup` — *"rather than adding a second grouping model to
`internal/layout`"*, which is an instruction to work **inside** it.

**But file-absence was always a PROXY.** D-7.1.3 ruled that the epic header's "changes nothing about
pagination" means the **model**, proven by 7.2's AC5 requiring page breaks to move *with `internal/layout`
unchanged*. `paginate.go`-absent was a cheap stand-in that happened to be correct while 7.1–7.6 had no
business in the file. **At 7.7 the proxy and the purpose come apart — and a tripwire keyed on a proxy is
the shape the lead has twice refused.**

**The fence for 7.7 is the property, not the filename:** (a) the four pagination rules, (b) window advance
to the first unplaced item and **no page is ever empty**, (c) one `Shift` per page, (d) the column is never
mutated. Those already have tests. **`TestPaginateNeverProducesAnEmptyPage` and the four-rule tests must
stay green AND unmodified** — an anchor the story cannot move, unlike a filename.

**The co-extensiveness audit has measured teeth.** `ItemGroupKey` carries `IsHeader` and an `ElementID`
meaning *the table*. Four sites assume it: `paginate.go:833` (`!it.Group.Key.IsHeader`), `:839`
(`tbl := it.Group.Key.ElementID`), `:949-950` (`headerPageOf[...]`), `:960-962`
(`headerExtent(...)`). A keep-together group has **no header and is not a table**, so each must be
re-examined, and `:783`'s comment ("Keyed on `Group.Present`, and NOT on the item's kind") is the decision
7.7 puts under load. **Enumerate them in the story record; a group that silently acquires header semantics
is the failure mode.**

### D-7.7.2 — The group key gets its own rank at 1.2 — not 1.1, not 2.0
**Lead ruling**, agreeing with the plan gate's independent conclusion.

Applying D-7.3.1's test — *would a pre-V reader refuse it or render it wrong*:
- **Not 1.1.** A 1.1 reader predates the group key: it loads the file and **silently splits the signature
  block**. Declaring 1.1 would claim a reader sufficient for content it cannot render — a version that
  lies, the defect option 3 was rejected for at D-R7.9.
- **Not 2.0.** The key is additive; a 1.x reader can load and ignore it. Declaring 2.0 **overstates** the
  requirement and orphans the document from every 1.x reader for a feature that never needed a MAJOR.
  `style.color` is the standing precedent: an additive key whose absence renders *wrong* is still MINOR.

So `baseVersion "1.0"`, `minorFeatureVersion "1.1"`, **a new `"1.2"`**, `majorFeatureVersion "2.0"`. A
document with a group only is **1.2**; group + `justify` is 2.0 by the max 7.3's rank fix already built.
Note the key is **element-level**, so `styleVersionRank` is the wrong site — `versionRequiredByContent`
needs a new probe in its element loop.

**Guardrail, because this fails silently.** `versionForRank` is `[...]string` **indexed by an `iota`
rank**; inserting `1.2` renumbers `rankMajorFeature`. An array literal keyed by constant names follows
correctly, anything assuming a numeric rank does not. **Assert `versionForRank` is strictly ascending by
parsed version** — that catches a mis-ordered insertion now and every future one, and it is the anchor
that table currently lacks. The hand-enumerated lists at `linespacing_test.go:229` and `:246` go vacuous
otherwise.

### D-7.7.3 — DW-38: the engine owns per-kind creation defaults; a PROJECTION is not a MIRROR
**Lead ruling.** Engine-side constants exposed through the existing projection channel alongside
`DefaultFontSize`. **Not** a browser-side table, tied or otherwise.

**AD-15 settles ownership:** a creation default ends up *in the document*, and AD-15 gives the engine the
document while the UI only sends commands. A browser-side default table would be the browser deciding
document content.

**The distinction worth keeping:** *a projection is not a mirror.* A mirror is a second hardcoded spelling
that can drift; a projected value is transmitted at runtime from one source, so there is no second
spelling. Projecting per-kind defaults adds **zero** mirrored numerals; hardcoding five kinds browser-side
would add **ten** to a set of six that has already produced five defects.

**Guardrails.** **No browser-side `?? 72` fallback** — it re-creates the mirror and stays silently correct
until the engine changes; an absent projection must fail visibly. **Characterize before fixing:** the
reported symptom (a later-sheet image getting a different box) does not follow from a constant 72×24 on
its face, so something else — containment, or grid snapping at a window edge — is contributing, and the
story must reproduce it in a test *first*. Also worth naming: 72×24 makes a **Line** a **24pt-thick bar**
under Story 9.2's "declared height is its thickness" — the sharpest instance and the one an author notices
first.

**Owner: Epic 14.2**, which already owns the two kinds whose defaults are most obviously wrong. **Not 7.7**
— that is a layout story and this is creation/designer.

### D-7.7.4 — DW-42: story-owned, and the deliverable is an inventory WITNESS, not 29 tests
**Lead ruling.**

**Opportunistic discharge cannot reach the seams that matter** — its trigger is unrelated activity, so it
covers exactly what someone happened to touch and never what nobody touches, which are the ones that rot.
That is the deferral shape refused three times this run (DW-14's spent event owner, DW-24's twice-renewed
trigger). **A rule whose trigger is "someone walks past" is not a trigger.**

**But "tie the remaining 29" solves the instances, not the class.** What made 7.6's defect possible was not
that 29 were untied — it was that **nobody knew which were untied**. Six of ~35 is a number somebody had to
go and count. **The artifact that closes this is the count becoming automatic.**

**The shape:** a build-failing check enumerating duplicated Go/TS invariants, requiring each to be
**either tied by an assertion or explicitly listed as exempt with a reason** — an unlisted, untied
duplicate fails. That is the coverage-witness form that closed `ScanAbsences` and the licence manifest's
extension allowlist, and it is the anchor DW-24's hand-list never had (three rots, the last inside its own
closing commit). Ties then land as the witness demands them.

**Guardrails.** The **exemption list is the dangerous half** — every exemption carries a reason and the
reasons are re-read at each boundary gate; an exemption list nobody re-reads is an allowlist, and
allowlists rot in both directions. Tie DW-42's own instance (`SHEET_STACK_GAP` ↔ its CSS gap) **in the same
story**, as the witness's first customer, so the mechanism is proved by use. Fold in the five known mirrors
so the enumeration starts from measurement.

**Placement: a named story in Epic 15, before the v0.1.0 tag** — this is build integrity, not UI coherence,
so Epic 14 is the wrong home. Under D-R7.9 (nothing released, no tag scheduled) there is slack. **It
becomes the owner's only if adding it would move the tag.**

### D-7.7.5 — Two lessons recorded as general rules, not story notes
**Lead ruling**, on the run's two sharpest findings.

1. **When a consumer needs a datum the producer computed and dropped, the fix is to stop dropping it —
   never to recompute it.** The banned closed form reappeared at 7.6 precisely because
   `addCanvasWindowCount` discarded `PageAssignment.Shift` while keeping `len(plan.Pages)` from the same
   value: *the second derivation was created by throwing away the first.* The control fixture asserting the
   true origins **and separately pinning the closed form's** is the construction that stops a
   discrimination decaying into a coincidence.
2. **A round-trip test over a sampled domain proves nothing about the region the samples avoid, and the
   interesting region is the one a declared gap skips.** 7.6's zero-pixel drag committed a nine-window move
   because every offset the shipped round-trip sampled sat inside its own window. **Sample the
   discontinuities, not the interior.**

## Story 7.7's deferrals — five rulings, and one story added to Epic 7 (2026-08-31)

Story 7.7 closed at `c9039a3` with seven deferrals filed (DW-46 … DW-52). Three of them were
routed to the engineering lead as decisions rather than patches. It ruled on all three plus two
consequences. The rulings are recorded here in the lead's own reasoning, because in two places the
ground matters more than the verdict — the verdicts are small and the grounds are reusable.

### D-7.7.6 — DW-46 is a DEFECT, not a shortfall: the canvas is FIXED, and no fourth floor cause is registered

**Ruling.** The canvas must tag its `layout.ColumnItem`s with the same keep-together groups the
render path already uses, so that the window **count** and the window **origins** become correct by
construction. `ContentWindowCountIsFloor` keeps **exactly its three existing causes**. Nothing is
added to the disclosure surface.

I had offered the lead two options — register grouping as a fourth floor cause, or something else —
and it took neither, which is the interesting part.

**The ground, and it is one measured fact.** `keepTogetherTags(t *Template) keepTogetherIndex`
(`folio-go/render.go:2017`) takes **the Template and nothing else**. No data, no params, no
`FontSet`. Grouping is therefore a **pure template property**, and the canvas — which holds the
template — already holds every input it needs to be right.

**That fact is the whole line between a floor cause and a defect.** The floor flag's three existing
causes are each things the canvas *genuinely cannot know*: a bound table's row count needs the data,
the taller-than-one-window degradation needs the render, a failed font chain needs the face. Those
are shortfalls, and a disclosure is the honest response to a shortfall. Grouping is knowable
canvas-side, so reporting it as unknowable would **park a defect inside a disclosure mechanism**.
The lead gave the test for telling these apart, and it is worth keeping:

> the test for that is whether the user could avoid it. They cannot: they declared a group in the
> file and the canvas is simply wrong about it.

**A plain-language way to say it.** A weather forecast that says "I can't see past the mountain" is
honest. A forecast that says "clear skies, certain" while looking at the wrong valley is not — and
adding "I can't see past the mountain" to it does not fix which valley it is looking at. DW-46 is
the second kind. The canvas is not short-sighted here; it is reading the wrong document.

**One fix closes both halves.** I had argued separately that a wrong *origin* is a different failure
from a wrong count, because a floor flag is a claim about a **count** and says nothing about **where
a window begins** — there is no flag on the origins array at all. The lead agreed with the argument
and then dissolved it: once the items carry their groups, the real `Paginate` produces the true
origins **as a by-product**, because Story 7.6 projected origins from `pages[page].Shift` rather
than computing them. So the same wiring fixes count and origins together, and adds no new
disclosure surface for either.

**Guardrails carried into the story:**

- **Assert the equality directly** — for a grouped document, the canvas's window count and origins
  **equal the render path's**. That absent assertion is what let this ship, and it is stronger than
  any test of the flag.
- Confirm afterwards that grouping introduces **no** new floor cause. A group's aggregate height
  still varies with bound text length, but that is the canvas-versus-bound-data divergence the
  existing causes already name. **If the implementation finds a genuinely unknowable grouping case,
  it returns to the lead before a fourth cause is added** — it does not add one.
- `parse_bands.go` already refuses the tag on a table, so a group cannot inherit a table's data
  dependency. **Keep that refusal asserted**, because it is what makes the previous bullet true.

The mechanism needs no new code beyond the call: `keepTogetherGroup` and `orKeepTogether` are
already methods in package `folio`, which is `page_setup.go`'s own package. This is one authority
gaining a second caller — the same shape as Story 7.5's extents.

### D-7.7.7 — DW-46 becomes a named Story 7.9, not a bare fix and not a passenger on 7.8

**Ruling.** **Story 7.9**, in Epic 7, named for its subject: *the designer tells the truth about
keep-together groups*. It carries D-7.7.6 and D-7.7.10.

**Not folded into 7.8.** I raised that 7.8 and this work are unrelated in subject, and the lead
made that the whole objection rather than a preference: folding unrelated work into a story's record
**destroys the attribution the per-story log exists to preserve**. It named the precedent — the two
unstoried commits that landed after the Epic 6 gate — and said it would not authorise "its tidier
cousin". A story that quietly contains a second story's work is the same loss of provenance,
arrived at politely.

**Not a bare fix either.** It changes projected values the canvas draws from, it needs a grouped
canvas fixture, and it needs the count-and-origins equality assertion. Work that ships a fixture and
an assertion is a story; **calling it a fix is how it lands without a Delivery Log entry**.

**On adding a story to an epic that was scoped closed.** The lead drew the line explicitly, and it
is the line this run should keep using: a new story that adds **capability** to a closed epic is
scope creep and would be refused; a new story that **restores a guarantee the epic already shipped**
is the epic finishing its own work. 7.9 is entirely the second kind.

### D-7.7.8 — DW-46 gates `epic-7: done`, and that call is the lead's, not the owner's

**Ruling.** `epic-7: done` is not written until Story 7.9 lands. The heavy-suite catch-up is
satisfied on evidence — every Epic 7 story ran the suites under the D-R7.1 override, measured
per story at 7.7's close — but the boundary gate is not only about suites.

**The ground is a false AC at HEAD.** Story 7.6's AC2 requires the window boundary to be *"marked
where the engine will actually break, taken from the projection rather than computed in the
browser."* For a grouped document the projection now reports boundaries the engine **will not
take**. That is not a new deferral filed against a new feature; it is a **regression of an already
accepted criterion**. A boundary gate that passes while a shipped story's AC is false is not a gate
but a formality, and D-000.4 exists so that `done` means something.

**The disclosure makes it worse rather than better**, which is what removed the lead's hesitation.
Before 7.6 the canvas made no claim at all. Now it asserts exactness in exactly the case where the
number is wrong. A confidently wrong disclosure is a liability the silence was not — and shipping
one under a green gate teaches the next reader that the flag can be trusted when it cannot.

**Why this is not the owner's call.** The owner decides what to build and what to trade away. This
is not a trade: nobody chose to ship a false claim, and the fix is small and inside the epic. It is
"does the epic meet its own criteria", which is the lead's.

**The one condition that flips it back to the owner**, recorded so it is recognised if it happens:
if Story 7.9 turns out **materially larger** than D-7.7.6 implies — for instance if the canvas
cannot reuse `keepTogetherTags` for a reason not yet seen — then holding the epic starts costing
schedule against the still-open v0.1.0 sequencing, and **that** trade is the owner's. It returns to
the lead first, at 7.9's plan gate, and to the owner only if the plan gate says so.

### D-7.7.9 — DW-47/DW-50: an over-tall ELEMENT stays fatal, tagged or not; the discriminator was split on the wrong axis

**Ruling.** The question is **not** "is it grouped". It is **what is over-tall**:

- an over-tall **individual element** is a located `OverflowError`, **fatal**, whether or not it
  carries a group tag;
- a group that exceeds a window **only in aggregate** — every member fitting, the sum not — takes
  Story 4.6's clip-and-warn.

Matrix rows 3 and 5 stop colliding because they were never really in conflict; they had been split
on the wrong axis.

**The ground is D-4.6.2's own ratio: leniency follows AUTHORSHIP**, and the direction of that ratio
decides this. A table row's height is driven by **data the author cannot fix**, so failing them
fatally is unjust — clip and warn. A loose element's height is **declared by the author**,
determinable from the template alone, and fixable by them, which is exactly why D-2.6.1 made
page-edge overflow a located template error in the first place. As the lead put it: **a group tag
does not launder authorship.** The author who declared a 900pt box still declared it.

**And a group of one is a no-op** — there is nothing to keep it together with — so it must be
indistinguishable from no group. Any other answer hands the author an **escape hatch from a fatal
error via an unrelated feature**: tag the element into a group of one and a hard error becomes a
warning. The lead's line is worth keeping as a general rule: *rules that can be switched off by an
unrelated declaration do not survive contact with a deadline.*

This keeps Story 7.7's AC true **as written**, which is why it is a defect in the matrix rather than
in the shipped story: the AC's clause is about **a group** being too tall, and a group whose single
member is individually too tall was already fatal before the tag existed.

**Guardrail: assert both halves in ONE fixture** — a single over-tall element tagged into a group is
refused fatally, **and** a two-member group whose members each fit but whose sum does not is clipped
and warned. Either half alone proves nothing about the discriminator, because either half alone is
consistent with the old rule.

**Home: Story 7.10** (D-7.7.12). By D-7.7.7's own reasoning it cannot ride Story 7.9 — 7.9's subject
is the canvas telling the truth, and an element's fatality is a different subject — so it was routed
back to the lead as a placement question and answered as a story of its own.

**One correction the lead filed against its own ruling**, and it is the kind worth keeping visible.
It had defended Story 7.7's third AC as "true as written" on the reading that the clause is *about a
group being too tall*. It then withdrew that: read literally, *"Given a group taller than one window
… never a fatal error"* **does** cover a single-member group whose member is individually over-tall,
so the ruling makes the AC text stale. Its own words: *"That was me narrowing the AC to fit my
ruling."* **Correcting the text is therefore part of the ruling, not follow-up** — 7.7's AC gains
the qualifier *"taller than one window **in aggregate**"*, landing with Story 7.10, because a story
paragraph left contradicting the ruling that answered it is how the next reader re-derives the
collision.

### D-7.7.10 — DW-48: `duplicateComponent` must NOT copy the group tag

**Ruling.** A duplicated component joins **no** group. Drop the tag on copy. Rides in Story 7.9.

**The ground is not "the designer should have a grouping UI".** The lead was explicit that
file-only authoring is **in scope** — Epic 7 has no story making groups authorable, and FR51 says
only that a group can be *declared*. What is **out** of scope is **creating state the author cannot
reach or undo**. A duplicated signature block silently joining the original's group can force an
unexpectedly large keep-together set, with no control anywhere in the product to explain it or
remove it.

The project refuses orphaned or unreachable document state consistently — Story 8.1's own AC refuses
a chain delete that would orphan elements, *"never accepted with the orphaned elements left to fail
at render"* — and this is that same principle applied at the copy path.

**Guardrails:**

- Record explicitly that **designer-side group authoring is out of Epic 7**, so the gap is a
  **stated scope boundary** rather than an accident. If it is wanted it belongs with the inspector
  work in Epic 12 or 14, and scheduling it is the owner's.
- **The drop must be asserted, not incidental**: duplicate a tagged element, assert the copy carries
  **no** tag and the original is **unchanged**.
- If 7.9's plan gate calls the pair `multiple-goals`, **D-7.7.6 is the half that gates the epic**
  and this one splits out. The split must not reverse.

### D-7.7.11 — The mutation finding goes into the EPIC's boundary record, as evidence about the suite

Story 7.7's builder mutated its own work and found that removing **all three**
`contentColumnItems` substitutions left the **entire test suite green** — a wired page-count pass
asserted by nothing, so `{{pages}}` could have printed the **ungrouped** total on every grouped
document and no test anywhere would have noticed.

The lead asked for this to be carried into Epic 7's boundary record as evidence **about the suite**,
not as a story note, because it is the **second time this run** that a whole path turned out to be
asserted by nothing. Recorded here for that purpose, with the standard it stated:

> the person who writes the code must not be the only one who chooses the mutation, and deletion is
> the cheapest screen.

Deletion-mutation — remove the call entirely and see whether anything reddens — is cheaper than
value-mutation and catches the failure class value-mutation cannot: a subject the tests never
**reach**. It belongs at every gate from here, and the choice of what to delete belongs to someone
other than the implementer.

### D-7.7.12 — Story 7.10 is confirmed in Epic 7, under a criterion that bounds any further additions

**Ruling.** **Story 7.10 — an over-tall element is refused whether or not it is grouped** — in
Epic 7, carrying DW-47 and DW-50 with the single two-arm fixture D-7.7.9 mandates. Story 7.7
created the collision, so Epic 7 owns it.

I had flagged discomfort at a **second** story being added to an epic that was scoped closed, and
rather than answering only this case the lead drew a line that answers every future one:

> **Epic 7 may add a story that REPAIRS Epic 7. It may not add a story that EXTENDS Epic 7.**

7.9 restores a guarantee 7.6 shipped and 7.7 broke. 7.10 corrects a disposition 7.7 introduced.
Neither adds capability, neither covers a new FR, and both close on the epic's own regressions. **A
proposed 7.11 that adds behaviour nobody shipped is a different epic's problem, and this orchestrator
can refuse it on this line without asking again.**

### D-7.7.13 — Story 7.10 does NOT gate `epic-7: done`, but it DOES gate the v0.1.0 tag

**Ruling.** Write `epic-7: done` when Stories 7.8 and 7.9 land. Story 7.10 escapes the boundary
gate — and acquires a **harder** deadline than the one it escaped.

**Why the two defects gate differently, which is the distinction to keep:**

- **DW-46 is a LIE.** `ContentWindowCountIsFloor` asserts exactness where the number is wrong, so
  the product tells the author something false. That falsifies a shipped AC, and a gate that passes
  over it is a formality.
- **DW-47 is an INCONSISTENCY.** Both behaviours are self-consistent renders with correct
  diagnostics — the tagged case clips and warns, and the warning is **true**. Nothing lies to
  anyone. The harm is that a tag can launder a fatal error, which is real, but it is not "a
  criterion of this epic is unmet". A documented matrix contradicting itself is a **specification**
  defect; the ruling that resolves it is the repair, and it does not retroactively make the epic's
  acceptance false.

**The constraint that actually binds 7.10 is the TAG, not the epic boundary.** It changes **when a
render fails**: documents that render today — clipped, with a true Warning — will fail **fatally**
afterwards. Under AD-22 that is a breaking change for every downstream suite, and it is free **only
while nothing is released**.

> **Story 7.10 must land before `folio-go/v0.1.0` is cut.**

That puts it in the same bucket as Story 7.8's load rejection, which the lead had already ruled must
precede the tag for the identical reason: **narrowing what is accepted is free exactly once.** Under
the sequencing the lead recommended to the owner — both epics before the tag — there is ample slack.
If the owner instead tags early, 7.8 and 7.10 are **inside** that decision and go in front of it,
rather than being discovered afterwards.

### D-7.7.14 — DW-49 splits in two, and half of it is already overdue

**Ruling.** Two edits, two homes, **neither held for the other**.

- **(a) The half that describes HEAD lands with Story 7.9.** AD-14's carve-out is scoped to
  over-tall **rows**, and that is **already false at HEAD** — Story 7.7 shipped clip-and-warn for
  keep-together groups, which are not rows. This half is not waiting on 7.10 at all; it is a stale
  spine sentence that has been stale since `ed485eb`. Widen the carve-out to cover rows **and
  author-declared groups**, describing what the engine does today.
- **(b) The half that states the discriminator rides Story 7.10** — that an individually over-tall
  element is fatal regardless of tagging. That sentence describes behaviour **that does not exist
  yet**, and a spine running ahead of the code is the same defect as one lagging it.

**Why split rather than take either simple option.** Editing it all now would put a sentence in the
spine that HEAD contradicts, on the promise that 7.10 will make it true — a doc correction resting
on a deferral, which is a weak trigger holding a canonical document hostage. Waiting for 7.10 leaves
the spine wrong about **shipped** behaviour for an unbounded interval, and this project has been
bitten repeatedly by canonical documents that were confidently wrong. They compose cleanly — (b)
adds a clause and does not undo (a) — so splitting costs no rework.

**Half (a) is done inside Story 7.9's record, not as a bare orchestrator edit** — not for want of
authority, but because D-000.6's amendments execute in a story so the doc change and its evidence
sit together. An amendment recording what the epic already shipped is Epic 7's own bookkeeping, not
a second subject, so it does not trip D-7.7.7's attribution concern.

### Sequencing set by these rulings

**7.8 → 7.9 → the Epic 7 boundary gate → 7.10, before the v0.1.0 tag.** 7.8 first because it is
already specified and small — and, independently, because it is the load rejection that must precede
the tag, so front-loading it removes the item most likely to be squeezed if the owner chooses to tag
early. 7.9 last before the gate because the gate waits on it. `epic-7` stays `in-progress` until 7.9
lands.

## Story 7.8 — the reserved diagnostic-code decision, ruled at the plan gate (2026-08-31)

Story 7.8's plan dispatch halted `blocked` / `intent gap` exactly where it was told to: on the
decision `internal/diag/diag.go:249-252` reserves. That halt is the correct outcome and not a
failure — the spec stated three options and picked none, which is what a plan gate is for.

### D-7.8.1 — ONE code, `TEMPLATE_FIELD_INVALID`, supplied by the CONSTRUCTOR

**The reserved question, verbatim from the code:** *"Before a THIRD is minted, someone must decide
whether the general form is right or whether AD-14's closed registry accretes one entry per style
field forever."*

**Ruling.** Mint **exactly one** code — `TEMPLATE_FIELD_INVALID`, for the category *a well-formed
template carries a field value that is not acceptable* — and have **`newLoadError` itself supply
it**. Every uncoded load-error site becomes coded at once, by construction: no enumeration, no
per-site decision, no accretion. `newLoadErrorCoded` remains the override for conditions that
genuinely need discrimination.

**The lead took the general option, but by a narrower route than the three I framed, and it is
cheaper than any of them because the structure to hold it already exists.** What it measured in
`internal/template/errors.go:42-75`:

- **`LoadError` is already structured** — `{Field, ElementID, Value, Reason, Code}`. The general
  option's selling point, "carry element/field/value as data", **is already true today**. The only
  missing piece is the `Code`.
- **Its message never quotes the document.** `Error()` renders `"template: field %s (element %s):
  %s (value: %s)"` — one field, one bounded value. The reflection hazard `reportableMessage` guards
  against is a **document-quoting** message, and a `LoadError` is not one. **The boundary rule is
  over-broad by accident, not by design** — which is the observation the whole ruling turns on.
- **`newLoadError` is a SINGLE constructor** called by every uncoded site in the package, and
  **AC41's enumeration test already walks those call sites.** The instrument that would otherwise
  have to be built is already shipped.
- **`newLoadErrorCoded` already exists**, already coding three footer-source conditions. Coding a
  load error is established practice, not a new mechanism.

**The registry-policy rule this establishes**, which is the part with a life beyond this story and
which replaces the reservation in `diag.go`:

> **The general code is the default. A specific code is minted only when a named consumer must
> BRANCH on it to behave differently.** Everything else discriminates on the `Field` data, which can
> grow freely without touching a closed registry.

The lead confirmed this is D-7.3.1's own lesson applied one level up — *partition by what the
consumer does, not by where the value is written* — and said so explicitly when I offered that
reading tentatively. A designer receiving any of these does exactly one thing: locate the element,
name the field, show the value and the reason. **One behaviour, one code.** `STYLE_COLOR_INVALID`
versus `STYLE_LINE_SPACING_INVALID` buys a consumer nothing it cannot get from `Field`, which is
why the registry was heading for one entry per style field forever.

**Why the other two were rejected.** Minting a third per-field code answers *"is the general form
right?"* with *"not yet"* — and the third mint is the moment the pattern becomes a policy by
default, which is exactly the reservation's own worry coming true. Changing the wasm boundary rule
was rejected too, **and this ruling gets its benefit without its blast radius**: `TEMPLATE_MALFORMED`
keeps destroying its own messages, for the good reason it was written; what changes is that
`LoadError`s **stop being bucketed there**. Nothing else reaching `reportableMessage` is affected,
so there is no unaudited population — which is precisely what made option 3 expensive.

**Guardrails, carried into the spec's Part 0:**

1. **Assert the property END TO END, through the wasm entry point.** The requirement is that *a
   `LoadError` must not reach the wasm boundary carrying `TEMPLATE_MALFORMED`* — a requirement, not
   a mechanism. The defect lives **at the boundary**, so a Go-side-only assertion would pass while
   the author still sees one sentence.
2. **AC41's enumeration test must be RE-POINTED AND RE-MEASURED, not re-run.** Its subject is "call
   sites of the uncoded constructor", and after this change that set is empty or means something
   different. The lead named this against itself: it is **the dead-detector failure it caused at
   D-7.4.2** — a test that keeps passing while measuring nothing.
3. **Load-stage only.** `diag.go`'s own scope note for `STYLE_LINE_SPACING_INVALID` already draws
   the line: a zero resolved advance and int64 overflow are render-stage conditions with a different
   remedy.
4. **The code names the CONDITION**, not the field and not the call site — the same discipline that
   gave one code to two `TABLE_FOOTER_SOURCE_FORBIDDEN` sites.

### D-7.8.2 — the two existing style codes are audited before the TAG, and not by Story 7.8

**Ruling.** Story 7.8 establishes the rule; it does **not** retire `STYLE_COLOR_INVALID` or
`STYLE_LINE_SPACING_INVALID`, and it must not do so by accident.

At least `STYLE_COLOR_INVALID` is a **render** error by Epic 10's own AC (*"When the document
renders, Then it is a located render error"*), so it is not in `TEMPLATE_FIELD_INVALID`'s load-stage
category at all and may be correctly specific. `STYLE_LINE_SPACING_INVALID` spans a load path and a
property-command path. Neither is a clean migration candidate on today's evidence, and 7.8 has no
business auditing them.

**But leaving them unexamined is how an arbitrary distinction ossifies.** So it becomes a **named
obligation triggered by the `folio-go/v0.1.0` tag**: audit both against D-7.8.1's rule — *does any
named consumer branch on them?* — and retire whichever fails. Not an event owner and not
"opportunistically": AD-14 makes removing a code a breaking change, so this is free exactly once and
the tag is the hard, dated boundary.

**AMENDED 2026-09-17 (SPEC-client-libraries story 2) — discharged before the tag.** The audit ran
under the owner's ruling that it gates `folio-go/v1.0.0`: no consumer branched on either code, so
**both** `STYLE_COLOR_INVALID` and `STYLE_LINE_SPACING_INVALID` were retired by client-libraries
story 3 (commit `0260ba6`) — line spacing now reports `TEMPLATE_FIELD_INVALID`, and colour is
validated at load with `TEMPLATE_FIELD_INVALID`. Neither `DiagCode*` constant exists on the release
tree, and the v1.0.0 surface census pins the 27 that remain.

### D-7.8.3 — the growing BEFORE-THE-TAG set, recorded in one place

Three obligations now share one trigger, and they are recorded together so that a decision to tag
early is made **with** them rather than discovering them afterwards. All three are free before
`folio-go/v0.1.0` and breaking after it, for the same AD-22 reason: **narrowing what is accepted,
or removing what is published, is free exactly once.**

1. **Story 7.8** — a justified table is refused at load. Narrows what loads.
2. **Story 7.10** (D-7.7.13) — an individually over-tall element is fatal whether or not it is
   grouped. Narrows what renders.
3. **D-7.8.2's audit** — retire whichever of the two existing style codes no consumer branches on.
   Removes a published code.

**AMENDED 2026-09-17 (SPEC-client-libraries story 2) — the set at the `folio-go/v1.0.0` tag.** The tag
is `v1.0.0`, not `folio-go/v0.1.0` (owner decision, 2026-09-16). Every item that joined this set is
recorded here against it, read from the tracker on the release tree:

1. **Story 7.8** (justified table refused at load) — `done`.
2. **Story 7.10** (over-tall element fatal, grouped or not) — `done`.
3. **D-7.8.2's audit** — discharged by client-libraries story 3: both style codes retired.
4. **DW-32 / Story 15.2a** (D-8.2.7, command-shape injection) — `done`. Its exported
   `ApplyComponentCommand` has since left the public API entirely (client-libraries story 1).
5. **Story 8.4 AC4** (D-8.4.3, non-font-asset render refusal) — `done`.
6. **Story 16.1's nameID 13 licence tie** (D-16.R.5) — Story 16.1 `done`. Its hand-authored
   "second door" at load is deliberately not closed and ships open; see
   `folio-go/internal/template/parse.go`.

**Tag-gate rulings recorded with the set (owner):** DW-147 gated the tag and is met
(`fixtures/colour-strokes/`, signed). **DW-68 is ruled: v1.0.0 ships the clip**, so making the
aggregate-only case fatal is now a `/v2` choice. 8.4d, 8.4k and DW-230 are released from the gate.

### D-7.8.4 — three corrections the plan gate found in the epic's own text

The epic text for Story 7.8 was written at Story 7.4's plan gate and carried anchors that had not
been re-verified. The plan dispatch checked all of them against baseline `53b2c1f` and found:

- **Two of the three test anchors were wrong.** `folio-go/line_spacing_test.go:168-175` with a const
  at `:311-331` **does not exist as described** — that file contains **zero** occurrences of
  `justify`, and its `:168-175` is `TestNeutralRatioNeverRefusesADegeneracyItDidNotCause`. The real
  site is `folio-go/internal/template/linespacing_test.go:230-237`, const `justifyHeaderStyleDoc` at
  `:479-503`. `epics.md` is corrected.
- **Two further tests are affected that the epic did not list**, and they **rename-and-widen**
  rather than invert: `closedsets_test.go:215-275`
  `TestAlignSetsAreTwoSetsPinnedAgainstTheirMaps`, whose name asserts two sets and whose `:261-263`
  asserts `len(StyleAlignTokens) == len(ColumnAlignTokens)+1` — a third set breaks both; and
  `component_properties_test.go:217-249`
  `TestStyleAlignPropertyValidatesAgainstTheStyleSetOnly`, whose name becomes false.
- **The version half is not where the epic said, and there is a SECOND DOOR.**
  `versionRequiredByContent` runs **only at save**, on a fully validated `*Document`, so a loader
  refusal closes the 2.0 raise **by construction** with no version code to change. But
  `component_commands.go:909` allows `align` on a table and its `case "align":` arm validates with
  **no element-type check** — so the designer's *engine* can still stamp a table document 2.0.
  Story 7.4 closed only the **UI** door. AC1 is not satisfiable without closing this one, so it is
  in scope, selected by a rule already written in the code: `IsStyleAlign`'s own doc comment
  requires the property-command path to *"validate it against the same single source the loader
  does."* It is Go engine code, so the Epic 7 designer fence holds.

**Also verified, and it de-risks the story:** **no golden is at risk.** Every table-bearing fixture
has a `justify` count of **0**; the only justify-bearing fixtures are the two text goldens the story
must preserve.

**The general lesson, since this is the second time this run an epic's anchors had rotted** (DW-24's
hand-list rotted three times, once inside its own closing commit): **an anchor written at a plan
gate is a claim with an expiry date.** Anchors are re-verified against the current baseline at the
gate that consumes them, never trusted from the gate that wrote them.

## Story 7.8 — D-7.8.1's premise corrected at the build gate (2026-08-31)

The first build dispatch implemented D-7.8.1 exactly as ruled, ran the whole verification block
green, and then **halted `blocked` / `intent gap` on the ruling's own factual ground**. That is the
most useful halt this run has produced, and it is worth reading as a process result before it is
read as a technical one: the implementer found the decision-maker's premise false, measured the
consequence in both directions, and refused to resolve it in the diff. HEAD stayed at `7c892f1`.

### D-7.8.5 — `LoadError`'s value is bounded where it is RENDERED, not where it is constructed

**The false premise.** D-7.8.1 argued the wasm boundary rule was *"over-broad by accident, not by
design"*, on the ground that *"its message never quotes the document — one field, one bounded
value."* That is not true of the code. The lead had read `LoadError.Error()`'s **format string** and
concluded about **the data flowing through it**, without checking what callers pass as `value`.

**Measured at `7c892f1`, by this orchestrator rather than relayed:** `newLoadError` has **105
non-test call sites**, and **7** pass `string(raw)` — an arbitrary JSON sub-object — as `value`:
`parse_bands.go:583` (`style`), `:718` (`padding`), `:749` (`border`), `decodehelpers.go:158`
(`unbreakableValues`), `parse.go:82` (`locale`), and two more. The build reported ~19; only 7 are
confirmable by the literal `string(raw)` spelling, and the conclusion does not turn on the number.

**The consequence, measured in both directions** on a well-formed document whose `style` key is a
2048-byte string:

| | code | message length | reflects the document |
|---|---|---|---|
| baseline `7c892f1` | `TEMPLATE_MALFORMED` | 35 | **no** — replaced with *"The template could not be processed"* |
| with D-7.8.1 as ruled | `TEMPLATE_FIELD_INVALID` | **512** | **yes** |

So the ruling's *"no enumeration, no per-site judgement"* — the property that made it elegant — is
also what switched `reportableMessage`'s reflection guard off for an entire population. And
`reportableMessage`'s doc comment had said so all along: *"that message quotes the offending
document back, so a large or hostile one would be reflected instead of described."* **The comment
was right and the ruling was wrong about it.**

**The lead's own account of the error, recorded because it names the shape:** *"I had read
`LoadError.Error()`'s format string and concluded about the data flowing through it. I never checked
what callers pass as `value`. That is the same failure I recorded against myself at Epic 4 —
measuring one population and reporting on a wider one — and it is worse here because I used the
conclusion to argue the boundary rule was over-broad by accident."*

**Ruling.** `LoadError.Value` stays **complete** in the struct. `LoadError.Error()` bounds the value
**as it renders it into the sentence**. One method, all 105 sites by construction, so D-7.8.1's
no-per-site-judgement property survives intact.

**The 7 raw-carrying sites lose nothing.** The full sub-object remains on the error as structured
data; it is **relocated from the prose to the field**, not destroyed.

**Why not the constructor** — which is what I proposed, and what the lead said it would otherwise
have picked: **bounding `Value` itself repeats the exact mistake being corrected.** A Go
integrator's CI log legitimately wants the whole offending JSON — that is the right place for it —
and the hazard exists only where the message is rendered **to a person who may not have authored the
document**. Truncating the struct field to fix a presentation problem is over-broad in precisely the
way the boundary rule was. *One over-broad rule is not corrected by writing another.*

**And it makes the premise TRUE rather than working around its falsity.** The premise was a claim
about **the message**. Bounding in `Error()` makes that sentence true of the message, which is the
thing that gets reflected.

**Guardrails:**

- **Bound in RUNES, never bytes, and never split a rune.** `len()` on a Go string counts bytes, so a
  byte bound hands Thai and CJK authors a third of the budget — **the same script-dependence defect
  ruled on at Story 7.4, two floors down.** Assert it with a multi-byte value so an ASCII-only test
  cannot pass a byte-counting regression.
- **The elision must be VISIBLE.** A truncated value that looks like a whole value is a new lie;
  a marker keeps *"this is all of it"* distinguishable from *"there is more"*.
- **Derive the bound from a stated criterion, not a round number:** *the message must stay dominated
  by the engine's own words — field, element, reason — inside `bounded(message, 512)`.* The
  criterion goes in the comment and one measured example in the story record. ~96 runes is the
  lead's expectation, **explicitly not the requirement**; a different number derived from the
  criterion is better than this one taken on faith.
- **`Value` is the author-supplied component identified so far, and the list is not assumed
  complete.** If the story finds another — a `Field` path or a `Reason` interpolating author content
  — it gets the same treatment and the story says so. The lead's words: *"do not assume my list is
  complete; I have already been wrong about this once in this thread."*

**The `TEMPLATE_MALFORMED` fence stands.** `reportableMessage`'s treatment of that code is
unchanged. Genuinely unparseable documents keep their destroyed message and the good reason it was
written for.

### D-7.8.6 — why this was NOT escalated to the owner, and what would reopen it

The lead nearly escalated, and said so: a reflection question with **no threat model** — PRD §13
records that the MVP deliberately has none — is normally the owner's. Two measured facts kept it.

1. **The project has already decided this exact question, in this exact file.** `main.go`'s
   `ENGINE_REJECTED` path does `bounded(err.Error(), 512)` and returns it, with the rationale beside
   it: *"The engine authored this text about a template the caller already holds. Withholding it
   left the panel with nothing to act on, so report it bounded, exactly as an ordinary render
   diagnostic's message is reported."* So **bounded, engine-authored text about a document the
   caller already holds is reported** — that is established direction, not a new concession.
   `TEMPLATE_MALFORMED`'s exception is not an exception to the principle: it exists because that
   message **QUOTES** the document, which is a different thing from **MENTIONING** it. D-7.8.5
   brings `LoadError` into compliance with the principle rather than asking for an exception to it,
   **so no new risk is accepted and there is nothing for the owner to accept.**
2. **Measured: no injection vector.** `grep -rn "dangerouslySetInnerHTML\|innerHTML"
   folio-designer/src/` returns nothing — diagnostics reach React as text nodes and are escaped. The
   residual reflection is inert display of **the user's own file on the user's own screen**, with no
   server and no third party anywhere in the product.

**What would reopen this as an owner question** — recorded, deliberately not acted on: a rendering
surface that **interprets** rather than escapes (any `innerHTML`, a Markdown renderer, a
`title`/attribute sink), or **FR45's REST service**, which PRD §13 names as the thing that brings a
threat model with it. At that point bounded reflection of author content acquires a standard to be
judged against.

**Two things explicitly NOT claimed**, so this is not over-read: only two injection spellings were
audited, not every sink in the designer; and `cmd/folio` **already** prints `err.Error()` with the
full value to a terminal today, so terminal-escape content in a `.folio` is a **pre-existing**
property of the CLI that this ruling neither creates nor fixes. A `deferred-work.md` line, not work
in Story 7.8.

### D-7.8.7 — a general obligation about guards, after the THIRD instance in one epic

The first dispatch also changed `TestWasmHostSanitizesTemplateDiagnostics`'s payload from a
**parseable** object to **unparseable** bytes — the only shape still reaching `TEMPLATE_MALFORMED`
after the change. The test stayed green while measuring the one class that could no longer reflect.

That is the D-7.4.2 shape for the **third** time in this epic, after DW-24's rotted hand-list and
the `contentColumnItems` page-count pass that no test reached. All three share one shape: **a test
that kept passing because its SUBJECT moved, not because the property held.** Three in one epic is a
rate, not an anecdote. So:

> **When a change moves a population OUT of a guard's scope, the guard's test must be re-pointed to
> a member still in scope AND the departed population asserted under its new treatment.** Narrowing
> the fixture to whatever still trips the old guard leaves a green test measuring the residue.

Applied here, `TestWasmHostSanitizesTemplateDiagnostics` gets **two arms**: *unparseable bytes* →
`TEMPLATE_MALFORMED`, message destroyed (still in scope, keep it); **and** *parseable-but-invalid
with a large `style` value* → the new code, message survives, **and the reflected fragment is
bounded and visibly elided**. The second arm is the one that witnesses the property the test is
named for.

**This goes into the Epic 7 boundary record beside the `contentColumnItems` finding** (D-7.7.11),
as evidence about **the suite** rather than about any one story, with the one thing all three have
in common named so the Epic 8 gate has something specific to look for rather than a general
exhortation to be careful.

## DW-28 found in production — Epic 8 gets an opening story (2026-08-31)

The owner pasted a contractor-liability clause from a real Thai contract into a text element. The
design canvas rendered it; the PDF preview failed outright with
`internal/pdf: face Noto Sans Thai: CID 27 carries a non-zero vertical offset (-2)…`, surfaced as
`Render failure · ENGINE_REJECTED`. That is DW-28, filed at Story 7.3 as MEDIUM and never picked up.

Reproduced by this orchestrator through the shipped CLI at `31d6cc6`, not relayed: the full clause
fails at **CID 27, offset −2**; the single word `ทั้งสิ้น` fails at **CID 3, offset −57**; the
same-script control `สัญญา` renders exit 0. A failed render writes **no** output file, so there is
no partial-artifact defect beside it.

### D-8.0.1 — a code comment asserted the branch was unreachable, and that is why nobody looked

`textdoc.go` read, verbatim: *"Measured at Story 2.3: YOffset is 0 for every glyph of every sample
across all three shipped faces, so this branch is UNREACHABLE through the render path with the
shipped set and cannot be red-proved through it."*

**Both halves are false.** The owner's clause reaches the branch on a shipped face through the
render path. Story 2.3 measured **its own samples** and reported on **the shipped set** — two
different populations.

**That is the third instance of one error shape in this run**, and the third is the worst because it
was written into a canonical comment rather than into a test: D-7.4.2's dead detector; D-7.8.5's
*"its message never quotes the document"*, where the lead read a format string and concluded about
the data flowing through it; and this. The lead named it against itself both times.

**Why this one cost the most.** The comment was **load-bearing in two directions**: it justified the
fail-closed choice *and* the absence of a render-path test, and it is what a reader hitting the
refusal in production would have checked first. **It protected itself.** Corrected immediately and
independently of the fix, in `textdoc.go` and in `textdoc_test.go`'s parallel claim, because a
comment asserting unreachability is exactly what stops the next reader from looking.

**The general rule, stated so it is not re-derived a fourth time:** *a comment that asserts a
negative — unreachable, never, impossible — is a claim with the same evidentiary burden as a test,
and it must name the population it measured, not the population it concluded about.*

### D-8.0.2 — HIGH, and NOT on the "the product lies" ground

Raised from MEDIUM on the criterion *blocks a supported use case, with no workaround, for a real
user.* The shipped Thai face is the only Thai face; the document is the owner's real work; and
"avoid the character sequence" is mutilating the document, not a workaround.

The lead explicitly refused the ground I had offered — that the canvas rendering correctly while the
PDF refuses is the same *product lies to the author* shape that gated Epic 7 at D-7.7.8 — and the
distinction matters for future triage. **AD-5 makes the page model blind to the emission stage**, so
the canvas cannot see a refusal that happens downstream of it: it is not lying, it is blind **by
design**. D-7.7.8's canvas floor was different in kind — that one asserted a value it had no basis
for. Grounding this on the lie framing would make a deliberate invariant look like a defect.

**The severity comes from the outcome — no bytes at all — not from the canvas.**

### D-8.0.3 — it opens Epic 8, and that is FORCED rather than traded

Not Epic 7: by D-7.7.12's line, *Epic 7 may add a story that repairs Epic 7, not one that extends
it* — the refusal is pre-existing, nothing Epic 7 shipped caused it, and emitting `Ts` is new
capability. The lead confirmed my reading and added a caveat worth keeping: **that line was written
to stop scope creep, not to triage blockers.** If a blocker ever genuinely needs to jump into a
closed epic, that is a trade for the owner, not a rule the lead bends silently.

**It is Epic 8's opening story because Epic 8 WIDENS the defect**, which turns "blocker versus epic"
into "precondition of the epic":

- Epic 8 lets an author embed **arbitrary faces**, whose mark positioning is arbitrary. Noto Sans
  Thai reaches this branch; a face picked from a catalogue can reach it far more often, on scripts
  nobody tested.
- **Story 8.4's own AC** — *a template with embedded faces renders on a machine that has never seen
  them* — is at risk from the first embedded face whose glyphs carry a vertical offset. Shipping 8.4
  over an unfixed fail-closed branch ships a feature that can **newly stop documents rendering**.

Numbered **8.0** so that 8.1–8.6 keep the keys they already carry in `sprint-status.yaml` and in
cross-references.

### D-8.0.4 — it does NOT join the before-the-tag set, and the reasoning runs opposite to 7.8 and 7.10

Emitting `Ts` for glyphs that currently **refuse** can move no existing golden **by construction**:
a document containing such a glyph produces no bytes today, so **no fixture can contain one**. And
the change **widens** what renders rather than narrowing it, so a consumer upgrading past
`folio-go/v0.1.0` gets more documents rendering, never fewer. Stories 7.8 and 7.10 are in
D-7.8.3's set because they **narrow**; this one is outside it because it **widens**. **The tag is
not the constraint here — the product is.**

**The one byte-identity guardrail, and it is the whole of the risk:** the `Ts` path must be entered
**only** when `YOffset != 0`. Every document whose glyphs all carry zero offset — which is the
entire corpus — must emit byte-identically, and the 21 digests are the assertion.

Also settled: this is a **gap, not a format limit**. `Ts` is inside AD-6's pinned profile — AD-6's
exclusion list (encryption, annotations, forms, transparency groups, shading, ICC, tagging) does not
contain the text-state operators — and the refusal's own comment concedes it: *"the alternative … is
not built here."* Story 8.0 does not have to argue that point.

### D-8.0.5 — the owner's sequencing call, and a checklist owner that failed for the third time

**Put to the owner** because it traded their own blocked document against two stories that close
Epic 7 cleanly, and only they can price their own work. The lead recommended splitting the
difference; **the owner chose to unblock sooner**: **7.9 → 8.0 → 7.10 → 8.1–8.6.** Story 7.9 still
closes Epic 7 on a clean gate; Story 7.10 moves behind 8.0 and is **placed in Epic 8's sequence
explicitly, not promised** — it is already pinned to the v0.1.0 tag by D-7.7.13 and must not be
forgotten across an epic boundary.

**The owner also chose to land the characterization NOW and separately**, rather than as Story 8.0's
first step as the lead had proposed. Done the same day: `folio-go/thai_mark_stacking_test.go`, three
arms, mutation-proved. That was the better call — with the fix a story away, the branch would
otherwise have been free to drift for exactly as long as it took to get to it.

**And DW-28's owner clause had failed.** It read *"the next story that touches `internal/pdf`'s
glyph-positioning refusal, plus the Epic 7 and Epic 8 plan-gate checklists as a second standing
address"*, and **Epic 7 ran eight plan gates without one picking it up.** That is the **third**
checklist-as-owner failure in this run.

> **A checklist is not an owner. A named story with a position in the sequence is the only owner
> that has worked on this project.**

Recorded in DW-28 itself as evidence rather than deleted, so the next reader sees the failure mode
and not just the correction.

## Story 7.9 halted on its own block condition — three rulings, and an escalation withdrawn (2026-08-31)

Story 7.9's build dispatch halted `blocked` / `grouping case the canvas cannot know`, which is the
`Block If` D-7.7.6 asked for, firing exactly as designed. **No fourth cause was added**, the
implementation was otherwise correct and mutation-proved, and commit `6a31a7f` was **retained**
rather than reverted — the block condition prescribed a halt and a return to the lead, not a revert,
and the commit contains real work a revert would throw away.

### D-7.9.1 — the `visibleIf` case is a genuine cause, but it is not about grouping and it is not a floor

**What was found.** A tagged group member carrying `visibleIf`: the canvas answers **3** with no
data and reports it **exact**, while the real render is **3** with `{"showRule": true}` and **2**
with `{"showRule": false}`.

**The ruling, and the correction to D-7.7.6 it carries.** `page_setup.go` only **projects**
`VisibleIf` as a string; nothing evaluates it. So the canvas cannot resolve it without data and this
passes the knowability test honestly. D-7.7.6's deciding fact — that `keepTogetherTags` takes only
the `*Template` — was **true**, and the conclusion drawn from it was **too wide**. The distinction
that was missing:

> **The tag INDEX is knowable. Whether a member is PLACED is a property of the data.**

The lead accepted that correction in those terms, and it is the second time in this run that a
ruling's ground was true as far as it went and the conclusion reached past it — D-7.8.5 being the
first.

**Correction A — register it for CONDITIONAL VISIBILITY, not for grouping, because it predates this
story.** An **ungrouped** element carrying `visibleIf` has the identical problem: the canvas places
it, the render omits it, and AD-24 makes a hidden element **absent with no gap**, so the counts
differ. That has been true since **Story 7.5** shipped the count and **7.6** shipped the flag.
**Grouping did not create this cause; it is how it was found.**

**Correction B — it is a CEILING, and that is what changes a name.** The directions do not agree:

| cause | mechanism | direction |
|---|---|---|
| bound table | canvas shows header height only, render adds rows | canvas ≤ render — **floor** |
| conditional visibility | canvas places an element the render omits | canvas ≥ render — **ceiling** |
| unstyled non-text element (D-7.9.2) | same | canvas ≥ render — **ceiling** |

**A boolean named `IsFloor` set true on a ceiling case is a second confidently-wrong disclosure** —
the exact failure D-7.7.8 gated the epic on, one layer down. And a document carrying **both** a
bound table and a `visibleIf` element can be wrong in **either** direction, so no single direction is
honest for the general case.

**The fix is a RENAME, not a redesign:** `ContentWindowCountIsFloor` →
**`ContentWindowCountIsApproximate`**. One Go field, its TypeScript mirror, one UI string. **No
enum** — direction is information nobody reads, because Story 7.6's own AC already has the UI say
only that the count *"can change when the data does"*, which is direction-free.

### D-7.9.2 — the unstyled non-text element is a DEFECT, in scope, and must not be disclosed

**What was found.** Measured at both revisions against the real `buildPageModel`:

| document | canvas @`26bfd49` | canvas @`6a31a7f` | real render |
|---|---|---|---|
| untagged + styled | 2 `[0 740000]` | 2 `[0 740000]` | 2 |
| untagged + unstyled | 2 `[0 740000]` | 2 `[0 740000]` | 2 |
| tagged + styled | 2 `[0 740000]` | **3** `[0 700000 1440000]` | 3 — **fixed** |
| tagged + unstyled | 2 `[0 740000]` | **3** `[0 700000 1440000]` | 2 — **REGRESSED** |

**Ruling: fix it, do not add a cause for it.** `element_box.go:69-74`'s rule is
`style.Background.Set || style.Border.Set` — **both pure template properties** — so the canvas can
apply the identical rule with **no data**. It fails the knowability test in the other direction, and
parking it in the disclosure is precisely what was refused when the fourth cause was refused.

**I argued it was out of scope because it is pre-existing, and I was wrong** — on a point I had
already made myself and then argued past. This story is what **propagates** the asymmetry into a
number the flag calls exact. I had written *"the story as built trades one wrong count for another"*
and then reasoned toward shipping it anyway. **A story whose gate is "the canvas is honest" cannot
ship a fresh dishonest row.** Recorded because the failure was mine and it is a legible shape:
having identified the disqualifying fact, I let scope-protection reason past it.

**Guardrails:** reuse `element_box.go`'s predicate, never restate it — one authority, two callers,
the same shape as `keepTogetherTags`, and a second copy of *"styled means background or border"* is
the drift this project keeps paying for. **Verify all four rows**, not just the regressed one: the
two untagged rows are correct today and changing what the canvas places can move them.

### D-7.9.3 — the story's subject widens, and D-7.7.12's line still holds

Story 7.9 becomes *the canvas tells the truth about **the window count***. It is still a **repair**
of Epic 7 under D-7.7.12, not an extension: it adds no capability and covers no new FR.

### D-7.9.4 — D-7.7.8's escalation TRIGGER fired, but its REASON had gone, so no question was put to the owner

The lead judged 7.9 materially larger than D-7.7.6 implied and moved to escalate, framing four
sequencing options for the owner. **I withdrew the escalation, because its premise was stale.**

Every option priced *the owner's blocked Thai document* against *Epic 7 closing cleanly*. But
**Story 8.0 had already shipped** — planned, built and closed at `26e3ba1` / `89df23b` while the
lead was ruling — and the owner's clause renders at exit 0, 70,448 bytes. **The blocked document was
already unblocked.** What remained on the other side was, in the lead's own word, bookkeeping: Epic 7
sitting `in-progress` blocks no user and nothing ships from that state.

**The distinction worth keeping, because it is about how these gates behave rather than about this
story:** D-7.7.8 reserved a return to the owner because *holding the epic starts costing schedule
against the still-open v0.1.0 sequencing.* The **trigger** fired; the **reason** did not survive.

> **Putting a decision to the owner whose stated trade has evaporated is worse than not asking** —
> it presents a fork that is not one, and spends the owner's attention on pricing a cost that is no
> longer there.

So the options collapsed rather than being chosen between: **7.9 whole → close Epic 7 → 7.10.** The
orchestrator proceeded on that and told the owner plainly rather than presenting it as their call.

**My own error in the same exchange, recorded:** the lead ruled on DW-28's sequencing, the owner then
reported the defect a third time and chose to take it immediately, and **I did not tell the lead the
sequencing had moved.** It then spent a full ruling pricing a trade that no longer existed. A
decision authority reasoning from stale facts is the orchestrator's failure, not the authority's:
**whoever changes a premise owes an update to everyone still reasoning from it.**

### D-7.9.5 — a cached green is a gate that measured nothing

`cd lint && go test ./...` was returning a **cached** four-`ok`. With `-count=1` it was **2 RED**,
and both reds had been introduced by the two preceding commits — so **two stories closed reporting a
gate green that was not.** One was Story 8.0's close drifting a hand-pinned line number in the
float-typed inventory; **one was the orchestrator's own** stage-rank violation, a red-proof written
under `internal/text` that imported `internal/fontset` — rank 5 importing rank 6, where D-000.16
makes the pipeline strictly forward. `lint` caught exactly that, which is the rule working.

Both fixed at `de7b126`. The test moved to `internal/fontset`, which already imports `text` and
constructs the Shaper, and was re-proved to still discriminate after the move.

**This is the fifth check this run that passed while measuring nothing, and it is a different
mechanism from the other four.** Those were tests whose **subject** had moved — DW-24's rotted
hand-list, the `contentColumnItems` page-count pass, `TestWasmHostSanitizesTemplateDiagnostics`'s
swapped payload, `renderPathWindows`' nil oracle. This one is a test whose subject was fine and
whose **result** was stale.

> **`-count=1` is the anchor. A cache is not one.** Every gate procedure in this run carries it, and
> a story must not close on a green that a cache produced.

### D-7.9.6 — the flag is `ContentWindowCountIsExact`, and the SENSE inverts on purpose

The lead corrected its own D-7.9.1 rename before it shipped, on the field's **zero value**, and the
reasoning generalises past this field.

- **`ContentWindowCountIsApproximate`** has zero value `false`, which reads **"this count is
  exact"**. A projection path that forgets to set it therefore **claims exactness** — which is
  *precisely the bug that started this entire thread*, rebuilt into the field's default. **A hazard
  indicator that fails toward the quiet variant is not an indicator.**
- **`ContentWindowCountIsExact`** has zero value `false`, which reads **"do not trust this count"**.
  A forgotten set degrades to the **honest** claim.

Same one field, same TypeScript mirror, same single UI string, still no enum — the only change is
which way the boolean points. **Free to decide now and expensive once 7.6's UI is built against it.**

> **A boolean disclosing a hazard must be named so that its ZERO VALUE is the safe claim.**

**Guardrail: mutation-prove the flip in BOTH directions.** Inverting a boolean flips every call site
and every assertion, it is the easiest thing in the world to get backwards, and **the corpus will
not catch it, because most documents are exact.** Required: a document that *should* be exact
reddens when the field is forced `false`, **and** a document carrying any registered cause reddens
when it is forced `true`.

**Direction is dropped deliberately, and that must be written where the field is declared.**
Direction informs no decision the canvas makes — a floor means there may be more sheets than drawn,
a ceiling fewer, and neither is a safe side to act on. If the projection carries the cause set,
direction belongs **there**, since a cause knows its own direction and a future consumer can derive
it without the flag re-acquiring a claim. If the projection carries only the boolean, the field's
comment says so explicitly: *direction was deliberately dropped; it lives with the causes if anyone
ever needs it.* **Without that line a future reader restores the floor claim, mistaking a deliberate
choice for lost fidelity** — which is how this project keeps re-acquiring claims it decided to give
up.

### D-7.9.7 — discharging a trigger without escalating, and the word that keeps it honest

D-7.7.8's escalation was **discharged, not un-fired**, and the distinction is recorded so a later
reader sees it was assessed rather than missed. The clause read: *"flips to the owner only if 7.9
proves materially larger than the reuse implies, **since then** it costs schedule against the open
v0.1.0 fork."* The chain is **size → schedule cost → owner**, and with Story 8.0 shipped at
`89df23b` the **second link** is gone.

**The rule, stated because it is about how these gates behave rather than about this story:**

> **A trigger may be discharged without escalating when its stated reason is MEASURABLY absent.
> "Measurably" is the load-bearing word.**

Both halves matter. A trigger is a **proxy** for the condition it was written to detect, so when
proxy and purpose diverge the purpose governs — the same re-anchoring that moved Story 7.7's fence
from a filename to an invariant. But **the failure in the other direction quietly destroys gates**:
*"the reason went away"* is easy to assert and hard to falsify, and a trigger any orchestrator can
retire by declaring its reason gone **binds nothing**. Here the reason's absence is a **fact** — the
owner's clause renders at exit 0, 70,448 bytes, and nothing is queued behind 7.9 that anyone waits
on — not a judgement. That is why this discharge is sound and why the rule does **not** generalise
to *reasons can be argued away*.

**The trigger is LIVE AGAIN if anything later queues behind 7.9.** That is why this is filed as
discharged rather than as never-fired.

**And the mitigation for the stale premise is the lead's own, filed here so it is not lost:** it
will **date and attribute state claims inside escalation blocks** from here (*"as of your message of
X"*), so a stale one reads as stale instead of authoritative. A lead resumed from a transcript holds
a **snapshot**, and a snapshot presented as current is how a correct ruling gets made about a world
that has moved. The gap that let it happen was the orchestrator's — see D-7.9.4.

## EPIC 7 BOUNDARY GATE — run 2026-08-31, PASSED

`epic-7: done` written at the closing revision below. The gate had three conditions, all measured
rather than asserted, and one of them was the reason the epic stayed open for two extra stories.

### Condition 1 — Story 7.6's AC2 is TRUE at HEAD (D-7.7.8's whole condition)

AC2: *"the boundary … is marked where the engine will actually break, taken from the projection
rather than computed in the browser."* It was **false at HEAD** for a grouped document from
`ed485eb` until Story 7.9 landed — the canvas reported a count too low, origins at wrong column
positions, and reported the count as **exact**.

Measured at the closing revision: for the grouped template the canvas projects **count 3, origins
`[0 700000 1440000]`** against a real `buildPageModel` render of **3 pages with the identical origin
sequence**; for the shipped `fixtures/keep-together/` document, canvas and render both begin window
two at **`706000`**. The mutants prove this is asserted rather than coincidental — **crippling
either tagging arm drives the canvas to `734000`**. Nothing is derived in the browser; origins come
from `plan.Pages[i].Shift`.

**The gate held the epic for two stories and that was right.** A boundary gate that passes while a
shipped story's acceptance criterion is false is a formality, not a gate.

### Condition 2 — no heavy-test arrears (D-R7.1)

Every Epic 7 story ran the heavy suites in-story, read from each story's own Delivery Log:

| Story | 4 legs exported | unset control | CrossTarget | lint | designer | digests |
|---|---|---|---|---|---|---|
| 7.1 | 4 PASS, none "asserts NOTHING" | not stated | PASS | 4 pkgs | 30f/213 | 5 + pre-story byte compare |
| 7.2 | 4 PASS, same sha | not stated | ok 21.76s | 4 pkgs | 30f/214 | 5 quoted |
| 7.3 | 4 PASS | **ran** | PASS 21.45s | 4 pkgs | 30f/215 | 9 by `shasum` |
| 7.4 | 4 PASS w/ timings | grep=0 | ok 20.8s | 4 pkgs | 32f/239 | all 20 hold |
| 7.5 | 4 PASS, grep=0/leg | not stated | PASS 20.15s | 4 pkgs | 32f/248 | all 20 hold |
| 7.6 | 4 PASS w/ timings | not stated | PASS 19.87s | 4 pkgs | 33f/280 | 20 declared |
| 7.7 | 4 PASS, grep=0/leg | **ran** | PASS 21.2s | 4 pkgs | 33f/280 | 20→21, insertions only |
| 7.8 | 4 PASS, 0 "asserts NOTHING" | **printed** | PASS | 4 pkgs | 33f/280 | all 21 identical |
| 7.9 | 4 PASS w/ timings | **ran, printed** | PASS 22.10s | 4 pkgs | 34f/284 | all 22, 0 moved |

**Epic 7 owes nothing to a catch-up run.** Note the unset control was *not stated* for 7.1, 7.2,
7.5 and 7.6 — those legs are credited on their own PASS lines rather than on a proof the legs were
not no-ops. From 7.3 onward the control was run and printed. That is a real, if minor, gap in the
early record, and it is recorded rather than smoothed over.

### Condition 3 — nothing left that gates

**Story 7.10 is open and does NOT gate** (D-7.7.13): both its behaviours are self-consistent renders
with truthful diagnostics, so no shipped AC is false. It **must precede the `folio-go/v0.1.0` tag**,
since it narrows what renders and that is free only while nothing is released (AD-22). It owns
DW-47, DW-49(b), DW-50 and DW-64.

**The before-the-tag set (D-7.8.3)** at this boundary: (1) Story 7.8's justified-table load
refusal — **landed**; (2) Story 7.10's over-tall fatality — **open**; (3) D-7.8.2's audit retiring
whichever of the two existing style codes no consumer branches on — **deliberately not done**, both
verified unretired at 7.8's close.

### THE FINDING THIS BOUNDARY RECORDS ABOUT THE SUITE, NOT ABOUT ANY STORY

**Five checks in this epic passed while measuring nothing.** One is an anecdote; five is a rate, and
the lead asked for what they have in common rather than a general exhortation to be careful.

1. **DW-24's hand-list** — rotted three times, the third **inside its own closing commit**.
2. **The `contentColumnItems` page-count pass** — removing all three grouping substitutions left the
   **entire suite green**, so `{{pages}}` could have printed the ungrouped total on every grouped
   document.
3. **`TestWasmHostSanitizesTemplateDiagnostics`** — its payload was swapped to the one shape still
   reaching the old guard, leaving it green while measuring the residue.
4. **`renderPathWindows`' `nil` oracle** — it passed `nil` for the `keepTogether` parameter, so
   grouping was disabled on the **render** side too. Both sides were wrong identically, and *"agrees
   with the render path"* asserted nothing about groups **for two stories**.
5. **`lint`'s cached green** — two tests red while `go test ./...` returned a cached `ok`. **Two
   stories closed on it.** The mechanism, measured at the close: the `rules` package **walks a
   directory**, and Go's build cache does not track `ReadDir`, so a **new file never invalidates
   it**.

**Items 1–4 are tests whose SUBJECT moved. Item 5 is a test whose RESULT was stale.** Same outcome
by two different mechanisms, and the second is the more dangerous because nothing in the diff looks
wrong.

**The three obligations this leaves on every gate from here:**

> **(a) Deletion is the cheapest screen, and the implementer must not be the only one choosing the
> mutation.** Removing a call entirely and seeing whether anything reddens catches the class that
> value-mutation cannot: a subject the tests never *reach*.
>
> **(b) When a change moves a population OUT of a guard's scope, re-point the guard at a member
> still in scope AND assert the departed population under its new treatment.** Narrowing the fixture
> to whatever still trips the old guard leaves a green test measuring the residue.
>
> **(c) `-count=1` on every Go gate, without exception. A cache is not an anchor.**

**And a fourth, from D-8.0.1, which is the same disease in prose rather than in code:** a comment
that asserts a negative — *unreachable*, *never*, *impossible* — carries the same evidentiary burden
as a test, and must name the population it **measured** rather than the population it **concluded
about**. That one cost the most, because it justified the absence of the test that would have caught
it.

### What this gate did NOT verify, stated rather than smoothed over

**The browser end-to-end suite has never executed anywhere in this run.** `test:e2e:compile` is
`tsc --noEmit` only, and D-000.4 defers the Playwright run. So Story 7.6's and Story 7.9's canvas
work — the whole subject of two stories — **remains unproven at the surface an author actually
touches.** Every canvas claim in this epic is asserted against the projection in Go, which is where
the authority lives (AD-17), but "the projection is correct" and "the author sees it" are different
claims and only the first is measured. Recorded as a limit of this gate, not as a defect of the
stories.

### Verdict

**PASSED.** `epic-7: done`. Nine stories delivered, two of them repairs the epic wrote against
itself, and the epic closed on a criterion it had falsified and then restored.

## Story 7.10 halted on an intent gap in the ruling itself — six rulings (2026-08-31)

Story 7.10's plan gate halted `blocked` / `intent gap`, and the gap was in **D-7.7.9's own
mechanism** rather than in the spec derived from it. The lead opened with the error attributed to
itself: *"I was reasoning in **elements** while the mechanism groups **line items**."*

**What the gate measured**, in both pagination passes (`render.go:2196-2250`,
`page_number.go:89-110`): column items are built **one per shaped line**, with the tag stamped on
**every** line item. So for a tagged **multi-line text element every member fits** and only the
union exceeds. Read literally, D-7.7.9 classifies DW-50's document as **aggregate-only** and leaves
it exactly as it is today — **still silently losing content**. The ruling written to fix a pair
reached only half of it.

**And its stated premise was false for that kind.** *"A group of one is a no-op — there is nothing
to keep it together with"* is untrue for multi-line text: **there is something — its own lines.**

### D-7.10.1 — the member unit is the TEMPLATE ELEMENT

A group's members are the **template elements carrying the tag**, not the column items they
decompose into. The tag is declared on an element; the split into one item per shaped line is how
the paginator **implements** keeping things together — an internal decomposition, not a statement
about what the author grouped.

> **Reading the discriminator in items lets an implementation detail decide a product behaviour.**

That is precisely how a one-element group came to be classified "aggregate-only" and DW-50 fell
through. Consequences: **a tagged element whose own extent exceeds the window → FATAL** (DW-50
reached and fixed); **a tagged multi-element group, each fitting, the sum not → clip-and-warn**
(Story 7.7's AC preserved unchanged). The literal item-reading was rejected as *the literal reading
of a ruling whose units were wrong*, and exempting multi-line text was rejected outright: it
**widens** what renders, contradicting the whole before-the-tag premise, and it silently ignores an
author declaration, which this project refuses everywhere else.

### D-7.10.2 — why fatal is right, and why the obvious counter-argument does not apply

The chain matters, because the reflex is to reach for AD-25's clip precedent:

1. **D-2.6.1 governs height overflow** — an item that fits in *no* window is a located template
   error, **fatal**. That is the default for anything too tall.
2. **AD-25's clip precedent is WIDTH-only.** FR44 / Story 2.8 clip against the declared *width*, and
   D-2.6.1 **explicitly excluded page-edge overflow from FR44**. So *"the author declared this
   atomic and it does not fit, therefore clip"* **does not transfer** from a value overflowing its
   box to an element overflowing a page.
3. **Story 4.6's clip is the exception to D-2.6.1**, and its justification is specific: a table
   row's height is **data-driven and the author cannot fix it**, so failing them fatally is unjust.
4. **A tagged element always has a fix in the author's hands: remove the tag.** The grouping exists
   only because they asked for it. So 4.6's exception does not reach it — **regardless of whether
   the element's height is data-driven, because the TAG never is.**

> **The real discriminator, sharper than D-7.7.9's: WHO CREATED THE GROUPING.** Engine-created
> (table rows, atomic by construction) → lenient. Author-created (a keep-together tag) → strict,
> because the author can dissolve it.

### D-7.10.3 — AC1 is replaced, because "the same as untagged" was a PROXY

*"The same located fatal `OverflowError` the untagged element receives"* was never the rule. It was
a **proxy** for *the author declared this and can fix it*, and it was **true for the population the
ruling had in front of it** — rects and images, fatal untagged — and **false for the one it did
not**: a multi-line text element, which untagged renders perfectly well across pages.

**This is the third proxy-versus-purpose instance in this run**, after Story 7.7's fence
re-anchoring from a filename to an invariant (D-7.7.1) and D-7.7.8's trigger discharged when its
reason went absent (D-7.9.7). All three fail the same way: **the proxy holds for the population that
was in view and breaks on the one that was not.**

The replacement states the difference from untagged rather than hiding it: the element is refused
**because the author declared an atomic block that cannot fit**, which is **unsatisfiable** rather
than merely degraded — and the tag is what makes it so.

**The story must NOT assert message-equality with the untagged case.** Under this ruling the
untagged document **renders**, so there is no untagged error to be equal to. Such an assertion would
be **false**, and writing one is the most likely way an implementer "reconciles" the old AC.

### D-7.10.4 — the aggregate-only case stays as 7.7 shipped it, and the boundary is named honestly

A tagged multi-element group over-tall only in aggregate **keeps clipping**. Story 7.10 does not
touch it. The lead said the uncomfortable part plainly rather than dressing it up:

> D-7.10.2's fixability argument, **pushed all the way, would make that case fatal too** — the author
> can untag it just the same. **Story 7.7 chose clip by importing Story 4.6's TREATMENT without
> importing its REASON**, and the lead confirmed it at the time. It is left because it is shipped,
> deliberate and outside this story's subject — **not because it is obviously right.**

**What would reopen it:** a real document losing content that way, as DW-50 came from a real case.
**Raise it now if anyone sees one** — one ruling would cover both cases, and it is cheaper before
the `folio-go/v0.1.0` tag than after.

### D-7.10.5 — the `Kind` defect is in scope, and it is load-bearing rather than adjacent

`paginate.go:241-249` enumerates *"exactly one of three sites"* and asserts `Kind == "table"` is
*"NO LONGER PRODUCED FROM PACKAGE FOLIO"*. `collectElementBoxRects`, added by Epic 9, is a
**fourth, ungrouped source**, so the claim is **false at HEAD** — which is exactly why an over-tall
rectangle is told, verbatim, *"element e1: **table** is taller than the content window (900000mp
against a content height of 729890mp)"*.

**In scope because it is load-bearing for this story:** AC1 requires the message to **name the
element**, and **7.10 is the story that asserts that message for the first time.** Asserting a wrong
message **pins** the defect.

**Guardrail: `Kind` is derived — fix the DERIVATION, never special-case the string** — and correct
the comment to state what is actually produced.

**Sixth instance of a comment asserting a negative that measurement falsifies, and the FIRST found
by looking rather than by being burned.** That is worth recording on its own: **the search is now
cheaper than the incident.** The rule from D-8.0.1 is doing work prospectively rather than
retrospectively for the first time.

### D-7.10.6 — the obligation discharges by ADDING, and both arms are required

There is nothing to re-point: **today's clip-and-warn for a tagged over-tall element is asserted by
nothing.** DW-47 was reproduced ad hoc at Story 7.7's close and never captured. **Seventh instance
this run of a behaviour asserted by nothing** — and, unlike the five in Epic 7's boundary record,
this one was found *before* it could hide a regression rather than after.

Both arms are required, and they **are the discriminator, not a demonstration**: the **tagged single
element** → fatal, message naming the element and its **true kind**; the **tagged multi-element
aggregate group** → still clipped and warned, unchanged. **Without the second arm the change reads
as "tagging makes things fatal", and the next story generalises it in exactly the wrong direction.**

**The two-document fixture shape is confirmed.** AD-14 makes an `Error` abort the render and a
`Warning` accompany a successful one, so **no single rendered document can yield both arms** — that
is a property of the contract, not a gap in the fixture. Two documents differing in exactly which
element is tagged, following the shipped `keepTogetherTemplateJSON` /
`keepTogetherUngroupedTemplateJSON` pair, whose own comment sets the standard the new pair is held
to: *"IT IS A DISCRIMINATOR, NOT A DEMONSTRATION."*

**DW-64 and DW-65 ride here**, since the implementation touches `internal/layout` and **Epic 7's
fence is closed** — the epic is `done` as of `9844e6d`.

## FR52's "reorder" — ruled inapplicable, not deferred (2026-08-31)

Story 8.1's plan gate found that FR52 promises *"reorder the document's font chains"* and the format
cannot express it: `Fonts` is a Go map with no stored order, and the serializer sorts its keys
**twice**. I routed it as a scope gap with three ways out. **The lead took none of them as framed:
under the correct reading nothing is being cut, and Story 8.1 delivers FR52 in full.**

### D-8.1.1 — chain order is INAPPLICABLE, and the wording is what needed fixing

**Ruling.** `fonts` stays a mapping with no authored key order. FR52's *"reorder"* is satisfied by
**entry-level** reordering — the order of faces *within* a chain — which is the only ordering the
format has ever expressed and which 8.1 delivers. **Amend the loose spellings; add nothing to the
format; record no gap.**

**Ground 1 — the absence of chain order is LOAD-BEARING for a different decision, and neither of us
had this.** `folio-format.md:390`, verbatim:

> *"There is no font default. An element with text and no `style.fontFamily` is a located error
> naming the element. A default was documented here from the format's first draft and never
> implemented; **`fonts` is a mapping with no authored key order, so "the first key" was never
> well-defined.** If a default is added later it will name its rule explicitly."*

So the format doc **reasons from** the no-order property to kill the font-default idea. Adding an
authored order would not merely reverse D-4.1.1's discharged debt — **it would supply the "first
key" that sentence depends on not existing, reopening the font-default question.** A far larger
blast radius than the option looked like, and the fact that settles it.

**Ground 2 — FR52's grammar gives "reorder" a referent, and it is entries.** Verbatim at
`epics.md:113`: *"Create, rename, reorder and delete the document's font chains **and their
entries**."* The verbs distribute over both nouns. Create/rename/delete have referents in both;
**`reorder` has a referent only in entries**, because a chain is an ordered list and the map is not.
**The reading under which every verb means something is the reading that wins** — the same rule
applied when a Given/When/Then AC beat a loose one-liner at Story 4.4.

**Ground 3 — chain order is semantically INERT.** `fontChain` (`render.go:1097`) resolves by name,
there is no default-chain rule, and an element without `style.fontFamily` is a located error. Chain
order reaches **no byte, no lookup and no render.** The only thing it could mean is the order names
appear in a panel — **presentation, not document**.

**And that presentation need is still fully servable**, which is why nothing is lost: the designer
may order the family control however it likes — alphabetical, recently-used, pinned — as **UI
state**. AD-15 keeps the *document* in the engine; it does not require a document field for every
list order a panel shows. **If persistent pinned ordering is ever wanted it is a small designer
feature needing no format change at all.** Recorded so the option stays visible rather than being
forgotten as "the thing we decided against".

**Why not add the order** — argued explicitly because I asked for it argued. Beyond reopening the
font-default reasoning, both mechanisms are bad: **abandoning sorted keys for `fonts`** breaks
AD-9's one-canonical-form directly, for one object, creating exactly the *two ways to serialize a
document* the invariant exists to prevent; **a parallel `fontOrder` array** is a second structure
that must stay in sync with the map through every add, rename and delete, with a new failure class
on each — an order naming a chain that does not exist, or omitting one that does. **That is a mirror
inside a single document**, and this run has spent a great deal of time on mirrors. Either way it is
a versioned format change carrying a rank under D-1.4.9/12 **for a benefit that is zero at render
time.**

**And recording it as a partially-delivered FR was rejected too:** an FR that is **fully delivered
under the correct reading** recorded as partial is a **false record**, and a false record is exactly
the kind of thing that becomes precedent.

**Three loose spellings amended, all in Story 8.1 because it is the story whose scope the reading
defines** — leaving any one makes a future reader re-derive this: FR52's own wording
(`epics.md:113`); **SPEC-fonts' CAP-1**, which promised chain reordering and **contradicted its own
companion**, whose `fonts` Order row reads *"Unchanged"* — the field-level companion is the precise
statement and governs, CAP-1's prose was the loose one; and Story 8.1's AC1 enumeration.

### D-8.1.2 — `headerStyle` is a standing check, not a catch

`headerStyle.fontFamily` being missed by Story 8.1's ACs is the **second** instance: the lead hit
the same shape at Story 7.3, where `headerStyle.align` fed `alignFallback` and its own closed-set
split failed to reach it. **`headerStyle` is a shadow copy of `style` that acceptance criteria keep
forgetting because it is not spelled `style`.**

> **Any story that walks a `style.X` must state whether it also walks `headerStyle.X`, and say why
> if not.**

A catch is what stopped the second instance. **A standing check is what stops the third.**

### D-8.1.3 — "route through the single authority X" is a CLAIM, never a premise

Story 8.1's dispatch instructed the plan gate to *"identify the single place that already decides
what a valid chain name is, and route through it."* **There was no such place.** The loader
validates nothing about a chain-map key — empty, whitespace, case variants and duplicate JSON keys
all load, last-wins — and the nearest rule is open-coded **five** times, **one of them under a
comment claiming it projects exactly what another accepts, three lines before re-implementing the
test.**

That is the **seventh** time in this run that a stated single authority turned out to be several,
and the rate has stopped looking like a series of accidents. So, for this orchestrator's own
dispatches:

> **"Route through the single authority X" is a CLAIM the plan gate VERIFIES, never a premise it
> accepts.** If X does not exist or is not sole, the story's **first task is to create it** — and
> that changes the story's size, so it must be reflected **before** dispatch rather than discovered
> mid-build.

**The encouraging half**, and it is the same shape as `paginate.go`'s falsified negative one story
earlier: **both were found by looking rather than by being burned. The search is now cheaper than
the incident.**

## DW-69 and the tagged-surface line, stated once (2026-08-31)

Story 8.1 added projection bounds on font chains, so a pre-existing `.folio` with a longer chain now
fails to **open**. I asked whether that narrowing joins D-7.8.3's before-the-tag set, relaying the
closer's ground for saying no.

### D-8.2.1 — DW-69 stays out, but the closer's ground was FACTUALLY WRONG

**Ruling:** DW-69 does not join the set. It holds exactly **one** open item — D-7.8.2's code audit.

**The correction matters more than the verdict.** The closer argued the narrowing was confined to
the designer because *"`cmd/folio` never calls `Canvas`"*. Measured at `bc671da`:

- `func Canvas(t *Template) (CanvasProjection, error)` is at `page_setup.go:545` and is **exported
  from package `folio`**; so is `CanvasWithTextPaint`;
- `canvasFontChains(t)` is called at **`page_setup.go:593` — inside `Canvas`**, not only from the
  wasm `Engine.load` path.

So a Go integrator calling `folio.Canvas` on a template with a 65-entry chain now gets an error where
they previously got a projection. **The narrowing is on the library's exported API surface.**
*"`cmd/folio` never calls it"* is true and **is not the test** — the tag versions the module's
exported identifiers, not what any particular binary happens to exercise.

**Why it still stays out: TIMING, not surface.** The before-the-tag set exists for changes that must
**land** before the tag because making them afterwards would be breaking. **DW-69's bound already
shipped at `bc671da`** — it is inside whatever v0.1.0 tags. And every correction anyone might later
want is a **widening**: raise the bound, or move it off the public path. Both are safe after the tag.
**A narrowing already inside the tag needs no before-the-tag action.**

### D-8.2.2 — the tagged-surface line, for Epic 8's five remaining stories

Two questions, kept apart, so this is not re-derived per case:

> **(a) Is it in the tagged surface?** — **is it reachable through the module's EXPORTED API?**
> Exported identifiers of `folio-go` are in: `folio.Canvas`, `folio.CanvasWithTextPaint`, `Render`,
> `LoadTemplate`. A `main` package inside the module (`wasm/cmd/engine`) is not importable and is
> out. **"Which binary calls it" is not the test** and would mis-classify **every** projection bound
> in this epic, because `Canvas` is public.
>
> **(b) Must it land before the tag?** — only if it is a **narrowing or a removal that has not landed
> yet.** Widenings are always safe. So the set collects **unshipped** narrowings and removals, which
> is why D-7.8.2's audit (retiring a code — a removal) is in it and DW-69 (a narrowing already
> shipped) is not.

My framing — *AD-22 versions the library, so a narrowing only a designer session can encounter is
not a narrowing of the library's inputs* — survives, **with the correction that "only a designer
session can encounter it" must be MEASURED against the exported surface, not assumed from which
product uses the feature.**

**One thing deliberately deferred, and named so it is not settled by accumulation:** a **JS-safety
bound sitting on the library's exported API refuses a Go integrator for a constraint they do not
have.** That is the same over-broad shape as bounding a stored value to fix a presentation hazard —
D-7.8.5's mistake, one level up. Relaxing it later is free, so it is genuinely cheap to defer; but
**Epic 8 will add four more instances of the pattern, so it must be decided ONCE and deliberately
before then.**

### D-8.2.3 — two lessons from the FR52 spellings and the unproved band

**The person who records a gap is a likely author of a surviving spelling of it.** Two FR52 spellings
survived the three the lead named, and both were mine — one a **bolded block** headed *"...AND THAT
IS A GAP THIS EPIC DOES NOT CLOSE"*, written in the very commit that recorded the gap. It is the
hardest kind to find afterwards **precisely because it reads as authoritative and recent.**
Retracting in place with the old wording quoted is the right repair; a silent edit would leave no
trace that the record had once been wrong.

**A hole in one arm of an enumeration is evidence about THE ENUMERATION, not about that arm.**
`pageFooter` was unproved, so `pageHeader` and `content` were checked too — and rightly: the
mechanism that missed one band had no reason to have covered the others. Treating the found instance
as the whole population is how these become the eighth occurrence rather than the last. Same move as
re-deriving DW-24's site list by grep instead of trusting the hand-list.

## Story 8.2's `multiple-goals` split, and a HIGH defect that was mis-filed (2026-08-31)

Story 8.2's plan gate returned `multiple-goals` over three subjects: the chain editor, a shared
command-JSON authority (DW-32), and the projection guard's sort order (DW-70). The split was ruled
after DW-32 was re-measured and turned out to be something other than what the register said.

### D-8.2.4 — DW-70 is a PRECONDITION of 8.2, and the guardrail matters more than the placement

**Ruling:** in scope. Story 8.2 is what makes an author able to name a chain, so shipping the editor
without it ships a feature whose **second keystroke terminates the worker**. Measured: Go sorts
chain names by **bytes**, the TS guard compares **UTF-16 code units**; `` (UTF-8 `ee 80 80`) versus
U+1F600 (`f0 9f 98 80`) disagree, `isCanvas` goes false, `parseInbound` returns `undefined`, the
client raises `PROTOCOL_INVALID` and calls **`worker.terminate()`**. The canvas blanks and stays
dead.

> **A defect the story makes reachable is a precondition of the story, not a competitor to it.**

Same category as Story 8.0 relative to Epic 8.

**⚠ THE GUARDRAIL, which is the half that could be got backwards: Go's byte ordering is NORMATIVE.**
`fonts` keys are sorted into the canonical `.folio` under AD-9, so **Go's sort IS the byte order of
the document.** The mismatch is one line to fix on **either** side — and fixing the Go side **would
move golden bytes** for any document whose chain names cross the boundary. **The TS guard adopts
code-point ordering to match Go, never the reverse.** Stated in the story explicitly, because
otherwise someone fixes the cheaper-looking side.

### D-8.2.5 — `quote()` is an incomplete JSON escaper, and THAT is on 8.2's path

The plan gate had already falsified my dispatch's premise once: chain names route through the
quoter, not through the numeric splice. **The lead then falsified the cleared route as well.**
`component-property-command.ts:41`'s `quote()` escapes `\`, `"`, `\n`, `\r`, `\t` — **and nothing
else.** JSON requires escaping **all** of U+0000–U+001F. So a chain name carrying any other control
character — pasted, most plausibly — produces **invalid JSON**, which Go rejects with a generic
failure rather than *"that name is not allowed"*.

**Ruling: 8.2 fixes it MINIMALLY — route `quote()` through `JSON.stringify`. One line.** In scope by
the same test as DW-70: this story is the first to make chain names author-supplied on that path. It
is **not** the shared-authority consolidation and **must not grow into it**.

**Engine-side name validation does not substitute, and this is an ordering fact rather than a
preference:** the JSON is **malformed before Go can see the name**, so the engine's rule cannot run.

**Worth recording as a shape:** the dispatch asserted a route, the gate cleared it, and the route was
*still* broken — one level further down. Clearing a path is not the same as measuring it.

### D-8.2.6 — DW-32 splits out, but the Go hardening is NOT a third subject

**Ruling:** the split is confirmed — DW-32 is browser-side, reachable today, HIGH, and has nothing
to do with font chains. **But my proposal to treat the Go-side duplicate-key refusal as a third
subject was rejected, and rightly.**

> **It is the other half of the SAME invariant.** The property is *a component command means exactly
> what it names*, and today **neither side enforces it**: the encoder can produce an ambiguous
> command, and the decoder resolves the ambiguity silently by last-wins.

Splitting the halves across stories is precisely the pattern that produced the five untied Go/TS
invariants and DW-42, and the standing obligation already binds it: **an invariant duplicated across
the Go/TS boundary moves in ONE commit, with a test that reads both sides.**

**And the Go half is what makes the property assertable at all.** Without it the only available test
is *"the encoder produces well-formed JSON"* — **a test of the fix, not of the property**, which
goes green again the moment a future encoder regresses. With it, the engine can be handed
duplicate-key bytes directly and asserted to refuse: a test **of the invariant**, surviving any
encoder. The lead's words: *"This run has been burned seven times by properties asserted by nothing;
I am not going to authorise an eighth by shipping the fix without its invariant."*

### D-8.2.7 — it joins the before-the-tag set, which is now TWO items; placement Epic 15

**Ruling:** the set becomes **two** — D-7.8.2's code audit, and **Story 15.2a**. Test (b) applies
exactly: `ApplyComponentCommand` is **exported** (`component_commands.go:37`), a duplicate-key
refusal **narrows** what it accepts, and it **has not shipped**.

**The category is honoured rather than argued down.** *"Nobody legitimately sends duplicate JSON
keys"* is a **likelihood** argument, and **likelihood is not the criterion** — that is the erosion
the lead has refused twice in this run.

**Placement: its own numbered story in Epic 15**, sequenced **before Story 15.3 cuts the tag**. Not
Epic 8 — D-7.7.12 holds, and Epic 8 does not widen this defect the way it widened Story 8.0's. Epic
15's stated purpose is *"the difference between a repository and a product"*, and tagging over a
known command-integrity defect is squarely that; **the deadline is then satisfied by construction
rather than by a promise.**

**Urgency, measured rather than assumed.** `rawNumberLiteral` takes `PropertyIntent['value']`,
reached from a properties-panel input on an explicit commit, and the projection carries numeric
fields as JSON **numbers** — so a document value cannot arrive there as a brace-bearing string.
Today's exposure is **keystroke-originated and self-inflicted in a local, serverless application**:
**HIGH by mechanism, low by encounter.** That justifies *before the tag* rather than *next*.

**The condition the story must MEASURE rather than assume:** if **any** path lets **document
content** reach `rawNumberLiteral` — a value round-tripped as a string, a paste path, a future field
— then a hostile `.folio` mutates arbitrary components on edit, and **it jumps the queue
immediately**. The type signature and the projection's numeric typing were measured; **not every
panel path was.**

**My own correction, mid-question, recorded because the sequence matters.** I first told the lead
the fix would narrow the Go API and belonged in the set on that basis. Then I measured:
`ApplyComponentCommand` **is** exported, but it is the **victim, not the site** — the splice is
browser-side and the fix touches no Go at all. So the encoder fix alone would **not** have joined the
set. Asking *"is one encoder fix enough?"* is what surfaced the Go half, which **does** join it.
**Parking the question would have been defensible; parking it without knowing it had a deadline
would not.**

### D-8.2.8 — two standing plan-gate checks, because both are now patterns

**(a) Every AD whose text appears in an AC must be named in `Covers:`.** Twice in two stories:
Story 8.1's line omitted AD-16, whose rule is AC1's substance, and Story 8.2's omitted **AD-15**,
whose rule AC2 quotes nearly verbatim (*"the browser never holds its own model of the `fonts`
map"*). The `Covers:` lines name FRs and UX rules reliably and **ADs unreliably**, which makes the
omission **systematic** — so the check is mechanical: **read the ACs, list the invariants they
paraphrase, diff against `Covers:`.**

**(b) An AC that cannot be satisfied is AMENDED, not merely under-delivered.** Story 8.2's AC3
promised an entry *"reads as the face's family and style from the projection"* and **there is no
such projection**. Delivering the **negative** half — the entry is displayed unmodified — was ruled
**better than a halt**, because the projection's entry-shape validator rejects unknown shapes in
**both** directions, so **Story 8.3 cannot change the entry shape without moving that validator in
the same commit**. That is *assert the absence so its arrival trips something*, applied correctly.
**But the AC TEXT had to change too**, with the positive half named as a **forward obligation on
8.3** — otherwise a reader at 8.3 sees an AC marked delivered that says something the story did not
do, which is the **false-record** shape D-8.1.1 rejects and the same one that left two FR52
spellings standing.

## Story 8.3's empty-face-name narrowing — KEEP IT, and the reason is AD-8 (2026-08-31)

Story 8.3 narrowed what already-shipped **1.x** documents load: an empty face name in a chain —
`{"fonts": {"body": [""]}}` — loaded before and is now refused, with **no version gate**. The build
flagged it as the orchestrator's to ratify. I routed it to the lead instead, leaning toward
**reversing** it, on the ground that a narrowing no version gate protects is the compatibility break
the version system exists to prevent. **I was pointed at the wrong rule, and I had not measured the
population.**

### D-8.3.1 — the pre-8.3 behaviour was itself an AD-8 violation

**What the lead measured, and it cuts against the comfortable reading.** `resolveRuneFace`
(`render.go:1202-1206`) does `if _, present := fs[name]; !present { continue }` — an empty face name
is **silently skipped**. So there are **two** shapes, not one:

- `{"body": [""]}` alone → no usable entry → the existing located error. **Never rendered.**
- `{"body": ["", "Noto Sans"]}` → the empty entry is **silently dropped**, Noto Sans is used, and the
  document **renders cleanly.**

**So a class of document DID render and now fails to load.** *"It was always a latent bug"* is true
of the first shape and **false of the second**, and the record must say so rather than letting the
comfortable half stand for both.

**And that second shape is exactly why the refusal is RIGHT.** AD-8's Rule: *"A template names a
family plus an ordered fallback chain; the chain is part of the `FontSet`'s identity, so the same
template with a different chain is a different render, **not a silent substitution**."* An author
declares a two-entry chain and the engine renders from a one-entry chain **with nothing anywhere
saying so.** That is the silent substitution AD-8 forbids **by name**. **The old behaviour was the
defect; Story 8.3 restored the invariant.**

### D-8.3.2 — three grounds, and NOT the one the build offered

**Ruling: keep the refusal, unconditionally.** Explicitly **not** on the ground the build proposed —
*"this repo has recorded such narrowings before"* — which is **precedent-by-habit and not a reason.**

1. **AD-8**, above. Decisive on its own.
2. **D-1.8.1's reader-independence test puts this at load.** That amendment turned on *"the same
   bytes are valid or invalid depending on which library reads them — and a contract whose validity
   depends on the reader is not a contract."* `"Helvetica"` **is** reader-dependent (another library
   might ship it) → skip, a capability question. **`""` is not a face name under any reader** →
   reader-**independent** malformedness → load error. Exactly where D-1.8.1 puts *a recognised thing
   whose content contradicts what it claims to be*.
3. **D-1.4.9 is not the rule in play, and this is my correction.** It promises a **higher-MINOR file
   loads in an older reader** — forward compatibility of the **version field**. It does **not**
   promise a library may never tighten its rejection of **malformed** input. Tightening validation is
   a breaking **library** change under AD-22, released as one — and **nothing is released.**

> **My instinct was right and aimed at the wrong rule: version gates govern format EVOLUTION, not
> MALFORMEDNESS.**

**Version-gating it was rejected on coherence, and the reason is recorded because it will be proposed
again: malformedness is not versioned.** *"This file is malformed only if it claims to be new"*
produces **two answers for the same bytes keyed on a number the author can edit** — strictly worse
than the reader-dependence D-1.8.1 rejected, because there the discriminator was at least **outside
the document**.

**Guardrails:**

- **Test BOTH shapes, and `["", "Noto Sans"]` is the one that matters** — it is the shape whose
  observable behaviour changed, and the one nobody is currently thinking about. **A test of `[""]`
  alone would pass while proving nothing about the case that regressed.**
- **The refusal must name the chain AND the entry index** — confirm it does rather than assume, since
  **the corpus cannot observe any of this.**
- **Record the narrowing naming the second shape explicitly**, so a future reader is not told it only
  affected files that never worked.

**Before-the-tag:** unchanged and needs nothing — already shipped at `af4efde`, every correction is a
**widening**, same shape as DW-69. **The set stays at two.**

### D-8.3.3 — a comment citing a test is a claim to verify in TWO steps

Story 8.3's review found `chainFaceNames`' comment claiming its behaviour was *"pinned by test …
(fonts_embedded_test.go)"*. **The cited tests did not exist** — and the file is **`package template`,
which structurally cannot call `Render`.**

**Why this is worse than the ordinary assert-a-negative shape:** **a citation reads as having been
checked.** *"Pinned by test"* plus a filename is a claim **with evidence attached**, so a reviewer's
natural move — trust it and move on — is the wrong one. An **uncited** *"this is safe"* at least
invites scepticism.

**And the sharp half is not that the tests were missing.** It is that the citation was **falsifiable
without opening the file** — by asking whether that package can reach that code at all.

> **A comment citing a test is a claim to verify in TWO steps: the test EXISTS, and its package can
> REACH the code it claims to pin. Existence is the weaker half; REACHABILITY is where the false ones
> die.**

Ninth and tenth instances of a property asserted by nothing in this run. **That both were caught by
deletion-mutation against a fully green suite is the encouraging half — the technique is now finding
these faster than they are being written.**

### D-8.3.4 — an unregistered standing red is the DW-23 shape, and it has already started

`TestShippedFacesReproduceFromUpstream` **fails rather than skips** without `fontTools`, and that is
**correct and must stay**: *"sources not present"* must never read as *"faces reproduce"* — the
all-clear-must-differ-from-could-not-look rule, applied exactly right.

**But it has gone unmeasured for several stories, and that is the failure already beginning.** An
unregistered standing red is the **DW-23 shape**: a red nobody has decided about is the one that
**masks the next real failure**.

**Two acceptable resolutions, and a third state that is not:** make `fontTools` available in the gate
so the test actually runs, **or** register it explicitly alongside the P6g floor as a **known, named
red with its reason**. **Leaving it as neither is what must stop.**

### D-8.3.5 — the Story 8.4 trap is an AC, not a note

A chain entry naming a **non-font** asset is accepted at load — correct under D-1.8.1 as amended —
but **errors nowhere at render**, because `chainFaceNames` drops embedded entries before anything
looks at them. That is **D-1.8.1's shape half-built**: the amendment requires **load to accept**,
**Render to Error when something actually needs to draw it**, and **`Validate` to predict what Render
would do.** All three halves are Story 8.4's, and **the third is the one most likely to be dropped**
— so it goes into 8.4 as an **acceptance criterion**, not a deferral note.

### D-8.3.6 — D-8.3.2's two guardrails, verified at HEAD rather than assumed

The lead's ruling carried two guardrails it explicitly said to **confirm rather than assume**, since
the golden corpus cannot observe any of this. Both measured at `2fe1e59` through the shipped CLI, on
documents built for the purpose:

| chain | outcome |
|---|---|
| `[""]` | refused — `template: field fonts.body[0]: a font chain entry must name a face — an empty string names none (value: "")` |
| **`["", "Noto Sans"]`** | **refused, identically** — the shape whose observable behaviour changed |
| `["Noto Sans"]` | renders — the control |
| `["Noto Sans", ""]` | refused at **`fonts.body[1]`** |

**Guardrail 1 — `["", "Noto Sans"]` is covered.** This is the shape that *rendered cleanly before*,
by silently dropping the empty entry. A test of `[""]` alone would have passed while proving nothing
about the case that actually regressed.

**Guardrail 2 — the refusal names the chain AND the entry index, and the index is real.**
`["Noto Sans", ""]` reports **`fonts.body[1]`**, not `[0]`. That is the non-first-entry proof:
**an index that is always 0 is indistinguishable from no index at all**, and this discriminates.
It also independently confirms Story 8.3's entry-index plumbing — the AC that looked like a
formatting detail and was actually a change to a decoder that collapsed a whole chain into one
unindexed error.

Measured by the orchestrator, not relayed from the build.

---

### D-8.4.1 — AC5 is measurement, the paint half is still 8.4's, and the design decision that blocked it is made

Story 8.4's plan gate halted `intent gap` on AC5 (*"the preview **measures** … one **measurement**
authority"*) against DW-35, whose filed fix is that the canvas fragment rule derives its CSS family
list from the projected chain — **rasterization**. AD-17 separates those two by name. Routed to the
lead. **Three rulings in one.**

**(a) AC5 is measurement, and the already-true half is PINNED rather than deleted.** AD-17 in its own
words: the canvas *"gets **every** text metric and line break from the engine's measure API … The
browser contributes rasterization only."* AC5 says "measures" twice. The gate measured that
`addCanvasTextPaint` already routes through the render path's own `fontChain` / `shapeSegments` /
`chainVerticalModel` (`page_setup.go:1162,1192,1229`), so the half is satisfied **structurally**.
A satisfied criterion is not a dead one: it is stated as **verified**, asserted by a test, so a later
story cannot quietly fork the path. Deleting it would have removed the only thing standing between
"shared today" and "shared by luck".

**(b) 8.4 does not WIDEN DW-35 — it CREATES the condition, which is why the paint half stays here.**
Before 8.4 an embedded face cannot render at all, so no author can reach the state. After it, the
engine measures with the embedded face while the browser has **no CSS family for it at all** and
falls to `sans-serif` — the owner-reported canvas defect fixed at `c6e4d03`, rebuilt for 8.4's own
headline use case. By the rule set at Story 8.0, **a defect a story makes reachable is a precondition
of that story.** Orphaning it to a neighbour would have shipped 8.4 knowingly broken for the exact
document 8.4 exists to make possible.

**(c) The blocker was a design decision above a builder's authority — Story 8.2's Design Note N3 said
so correctly — so the LEAD MADE IT:**

> **An embedded face's CSS family name is derived from its ASSET KEY, never from `font.family`.**

Grounded, not chosen. AD-8: *"the asset key decides, even where an embedded face and a shipped face
share a family name."* `format-changes.md`: `font.family`/`font.style` are display identity, *"never
used to resolve or substitute a face — resolution is by asset key alone."* Deriving the CSS family
from `font.family` would let an embedded "Inter" collide with a shipped "Inter" in the browser's font
registry — **AD-8's own hazard, arriving one layer down**. The face bytes reach the browser through
the projection and register as a `FontFace` under the derived name, parallel to what `c6e4d03`
already built for the shipped faces.

**Sizing is the gate's call, not the lead's.** If the gate returns `multiple-goals`, the paint half
splits to a story **sequenced immediately after 8.4** — not "later in Epic 8" — and 8.4's record
states the canvas limitation explicitly, so it is **disclosed rather than discovered**.

**Ownership, and the fourth ownership-mechanism failure of this run.** "Recommended owner: Story 8.4"
originates in **Story 8.2's spec**. `awk` over this log found **zero** occurrences of `DW-35`. Nobody
ever ratified it; it propagated for two stories as a recommendation everyone downstream read as a
decision. The register entry also carried **two contradictory `Owner:` bullets**.

> **Standing rule:** a *"recommended owner"* written by a story is **not** an owner until a ruling or
> a decision-log entry adopts it. A register entry carrying two `Owner:` bullets is a **defect in the
> entry**, not an ambiguity to be resolved downstream.

**And the orchestrator's own error, with the lesson that generalises.** The dispatch asserted "the
fourth AC is DW-35 stated as an acceptance criterion." **AC4 is DW-83; DW-35 is AC5.** The epic text
and the register carried the *same* off-by-one — so cross-checking one against the other could never
have caught it. **When two documents agree, check whether they are two sources or one source
copied.** Both corrected (epic and register), not just the one that was noticed.

---

### D-8.4.2 — AC3 stated the thing AD-7 names and rejects; corrected in the EPIC, not just the spec

AC3 said the subset tag is *"a deterministic hash of the **glyph set**"*. `deriveTag`
(`internal/fontset/fontset.go:910-924`) is FNV-1a over the emitted subset **program bytes**. This is
not a near-miss: **AD-7's Rule names the glyph-set reading and rejects it** — the tag is *"a hash of
the embedded font program's own bytes — the value `subset.Subset()` returns, in full — and, **unlike
a hash of the sorted glyph-id set alone**, discriminates two pinned instances of one variable face."*
**D-1.5.8** rejected it after two instances collided. Implementing AC3 literally would have moved
**all 22 recorded digests**.

**Ruling it at the gate was right; recording it to the lead was also right, and for a reason worth
keeping: an AC that contradicts an invariant is an ERROR to correct, not a FORK to route** — but the
correction belongs in the **epic**, because the epic is the artifact later stories re-derive from.
Fixing only the spec leaves the falsehood to be re-injected into every future reader. Same class as
the two surviving FR52 spellings.

---

### D-8.4.3 — AC4's render refusal joins the before-the-tag set, which is now THREE

AC4 introduces a **render-time refusal that has never shipped** for a chain entry naming a non-font
asset, reachable through the exported API. **D-8.2.2(b)** applies cleanly: documents that render
today — by silently dropping the entry — would fail after.

**The "silently wrong rather than shipped-correct" distinction does NOT keep it out, and Story 8.3 is
the precedent that settles it.** There the old behaviour was *also* an AD-8 silent substitution, and
the **only** reason DW-84 stayed out of the set was that it had **already shipped** (`af4efde`).
**The set tracks TIMING, not merit.** AC4 has not shipped, so it is in.

The set: (1) Story 7.8's justified-table load narrowing · (2) DW-32's command-shape injection, Story
15.2a · (3) **AC4's non-font-asset render refusal, Story 8.4**. Item 3 **discharges the moment 8.4
ships it**, returning the set to two — which is the practical value of listing it: it cannot be
quietly deferred out of 8.4.

**`UnsupportedFontMediaTypeError` extension is in-scope repair, confirmed.** It names asset key,
element id and media type but **not** the chain or the entry index AC4 requires; an error type that
cannot say what the AC requires must be extended, and that is the AC's own requirement, not new work.
Two guardrails: **adding fields is additive** and safe on an exported type; **changing message text
is safe under AD-14** (*callers match on the code, never on message text*) — but any existing test
asserting the old string must be **re-pointed and re-measured**, never merely updated.

---

### D-8.4.4 — two structural findings: one corrected at the layer that changes the repair, one confirmed

**(a) CORRECTION to the gate's finding, and the correction changes the fix.** The gate reported
`table_render.go:654-666` as *"a second, independent chain→names conversion"*. It is not:
at `:665` it **calls `chainFaceNames(entries)`** — a second *caller*, not a second conversion.
**`chainFaceNames` IS the single boundary.** What is duplicated is **`fontChain`'s lookup and its
error**, hand-mirrored at `:653-660` under a comment that says so outright — *"Mirrors fontChain's
own error, verbatim in shape (render.go)."* So the repair is **extract the chain LOOKUP, not the
filter**; the filter is already sole. The gate's "four documentless `(chain []string, fs FontSet)`
consumers" is the real remaining seam, and it is the set `chainFaceNames`' own comment names as the
thing it exists to avoid widening.

**The finding was right to raise and was raised in the right place.** At Story 8.1 the orchestrator
asserted a nonexistent single authority as a *premise*; the standing rule from that ruling —
*"route through the single authority X" is a CLAIM the plan gate VERIFIES, never a premise it
accepts* — is what caught this one, at the gate rather than in the code. Right rule, right venue,
wrong layer inside the finding.

**(b) AC6 is vacuous as the fixture stands — CONFIRMED, and it must be fixed IN 8.4.**
`fixtures/embedded-font/` draws **Latin** text while carrying a **Thai** face on purpose, so its
digest **cannot observe the embedded face being used** and AC6 asserts nothing. The document must
draw text the embedded face actually covers. Re-recording that fixture's digest is deliberate and
correct **within the story that owns it**, and free before the tag. Left alone it would be the
eleventh asserted-by-nothing path of this run — **and the only one shipped knowingly.**

---

### D-8.4.5 — sweep Epic 8's remaining `Covers:` lines now, because three-in-four is a rate

Story 8.4's `Covers:` omitted **FR33** (AC2's own words), **AD-7** (AC3's subset tag), **AD-17**
(AC5's entire subject) and **AD-21** (AC6's recorded digest) — **D-8.2.8(a)'s third occurrence in
four stories**.

**Not because per-story correction failed — the gate caught all three, so it works. Because a batch
is cheaper.** Two stories remain; the sweep is one pass; and it removes from two future gates the
attention they would otherwise spend on a known systematic omission. **Three in four is a rate, and a
rate is worth one batch rather than three more instances.**

Swept: **8.5** gains FR33, NFR1, AD-7. **8.6** gains AD-8, AD-16 and **DW-80** — named explicitly
because **8.6's AC5 is DW-80's fix stated as an acceptance criterion**, and naming it is what stops
the link depending on a reader noticing it. D-8.2.8(a)'s mechanical check stays at the gate. Extend
the same sweep to Epic 15's stories when they are planned — the omission is an **authoring-time**
property and Epic 15's stories are authored the same way.

---

### D-8.4.6 — the gate sized the paint half out; Story 8.4a is written and sequenced immediately after 8.4

D-8.4.1 left sizing to the plan gate and bound the outcome in advance: split is legal, silent drop is
not, and any successor is sequenced **immediately after 8.4**. The gate returned `multiple-goals`,
standing, on **three measured grounds** — not on feel:

1. **Separably shippable.** The engine half discharges FR54 and 8.4's own *"As an integrating Go
   developer"* sentence **in full**. The paint half serves a **different user on a different
   surface**.
2. **No precedent anywhere in the designer.** No `new FontFace`, no `document.fonts.add`, no dynamic
   style injection — all three shipped faces are build-time.
3. **It requires re-authoring two guards written to forbid exactly this shape.**
   `canvas-font-stack.test.ts:100-106` asserts the fragment stack contains no `var(`; `:123-132`
   forbids naming a chain entry in a font-family position. A third becomes false in spirit.

**Ground 3 is why Story 8.4a carries an acceptance criterion about its own guards.** A story whose
job is to make two deliberate guards false is the story most likely to **delete** rather than
**widen** them, and a guard re-authored to let a feature through is the cheapest way to lose one.
8.4a's AC4 requires each to be widened to a strictly stronger claim.

**The disclosure is a TEST, not a comment (8.4's Task 14), on Story 8.2's precedent.** The whole risk
of a split is that the gap becomes something a later reader must notice. A comment describing the
limitation would carry a test's evidentiary burden — the same defect DW-35's own entry was corrected
for.

**`oversized` also stands.** Accepted rather than split further: the contract is dense because the
story is dense, and the previous split already removed the one separable deliverable.

**DW-35 re-owned to 8.4a**, and the D-8.4.1 ruling anticipated it in its own words — *"8.4, or the
named successor story if the gate splits it"* — so this is the ruling executing, not a new owner
being invented. `sprint-status.yaml` carries `8-4a-…: backlog` **between** 8.4 and 8.5, so the
sequencing lives in the file the run reads rather than in this paragraph.

---

### D-8.4.7 — the gate corrected the orchestrator's "four consumers" the OTHER way, and found four traps that would have shipped green

**The seam is larger than reported, and the number came from a doc comment.** The orchestrator
relayed *"four documentless `(chain []string, fs FontSet)` consumers"* as the remaining seam. Measured:
`chain []string` is a parameter of **ten** non-test functions, **six** of which take
`(FontSet, *fontCache)`. The "four" is **`chainFaceNames`' own doc comment** — and it is neither the
population nor the risk set: one of its four takes no `FontSet` at all, and three that do are unnamed.
**None of the ten can reach `Assets`**, which is the actual constraint the story must change.

Worth recording because it is the **inverse** of the D-8.4.4(a) correction and the same root cause:
**a number read out of a comment was carried forward as a measurement.** The comment was not wrong
about its own subject; it was answering a different question. 8.4's Task 2 corrects it.

**D-8.4.4(b) sharpened by measurement of the shipped cmaps.** `NotoSans-Regular.ttf` covers **zero**
of U+0E00–U+0E7F; `NotoSansThai-Regular.ttf` covers **87**. But the Thai face **also covers Latin**,
so **a pure-Thai string is the sharp witness and a mixed one is not** — a mixed string would let the
Thai face satisfy the whole run and prove nothing about resolution order. Three Ts-free Thai strings
already in the corpus are named in the spec.

**Four traps that would have shipped green and proved nothing.** The most serious:
`embedded_font_fixture_test.go:218` and `matrix_test.go:1998` both assert `len(programs) != 1` — and
with a pure-Thai document on this chain a **correct** 8.4 also embeds exactly one program, so **both
pass identically on either side of the feature**, certifying the opposite of what they claim.
Inverting to `!= 2` would also be wrong. The others: `expected.json` has no second literal, so its
re-record is invisible; `canvas_projection_wire_test.go` records nothing for the paint tree; and
`canvas-authority-contract.test.ts:145` rewrites every `document.fonts` occurrence **globally before**
the prohibition scan, making its own rule at `:24` **dead** — a measurement call would pass unnoticed.
That last one is carried into **Story 8.4a as an acceptance criterion**, since 8.4a is the story that
would rely on it.

**Two transient reds are expected mid-implementation** — `chain_face_names_test.go:125-127` and
`missing_glyph_corpus_test.go:33` — the moment the fixture text becomes Thai and before resolution
lands. **They are the feature's own red-proof.** The spec says never to relax a diagnostic check to
clear them.

---

### D-8.4.8 — the Thai golden's attestation TRANSFERS by asserted shaped-run equality; a fourth disposition, on measured ground

Story 8.4's new golden draws Thai and had neither a human reading nor a `//go:build matrix` sign-off
gate. Three dispositions were put to the lead: mint the gate and ask the owner to read; mint nothing
on the ground that byte-identity across four targets covers it; or defer to a named story.
**The lead measured, and a fourth disposition beat all three.**

**First, the question the orchestrator could not judge, answered NO.** *"Ts-free"* does **not** make
the stacked-mark hazard structurally absent, and the repo's own fixture README refutes it:
`fixtures/thai-stacked-marks/README.md` records that `ที่`, `ป้ำ` and `ปั` each stack **two** marks
over one base and have rendered since Story 2.3 because Noto Sans Thai resolves those pairs with a
**GSUB lowered-form substitution at zero offset**. **Ts-freeness is orthogonal to stacking.** Ts-free
means every glyph has `YOffset == 0`, which excludes only the **GPOS vertical-displacement** leg —
Story 8.0's subject. It excludes nothing about X-offsets, glyph selection, base-mark association or
cluster order, which are precisely the **eye-only** failure modes. **Disposition 2 does not get
stronger; it gets refuted.** This is recorded at length because the orchestrator's own framing
invited the comfortable answer, and a wrong version of it would have justified skipping every future
reading.

**And the string can plausibly fail in an eye-only way.** `สัญญา` is U+0E2A, **U+0E31 MAI HAN-AKAT
(category `Mn`)**, U+0E0D, U+0E0D, U+0E32 — one above-base mark, whose failure modes are the mark
sitting over ญ instead of ส, rendering as a spacing glyph, or a cluster reorder. All three survive
Ts-freeness.

**The fourth disposition: the attestation transfers, because the shaped run is provably the same
computation over the same bytes.** `fixtures/embedded-font/`'s `e1` and `fixtures/thai-stacked-marks/`'s
`e2` are identical in **every input that determines a shaped run** — same string, `fontFamily: "body"`,
`fontSize: 12`, `width: 400`, `x: 0` — differing only in `y`/`height`, which is **placement, not
shaping**. And the face is the same **program**, measured rather than read off the asset's prose
`source` field:

```
sha256(folio-go/fonts/notosansthai/NotoSansThai-Regular.ttf)          = c94562c1…73caf
embedded asset key, AND sha256 of its decoded 47788 bytes            = c94562c1…73caf
```

The PDFs necessarily differ — different subset, resource named from the asset key (AD-7) — but the
layer a human attests, *the marks sit on the consonants they belong to*, does not. `e2` is that
fixture's **designated control**, and `fixtures/thai-stacked-marks/signoff.json` is the owner's
reading of it.

**Two weaknesses the lead flagged against its own construction, both recorded rather than smoothed:**

1. **The equality is live-vs-live**, so on its own it is the *both-sides-move-together* shape and
   would survive a shaping regression that hit both. It is non-vacuous **only because**
   `fixtures/thai-stacked-marks/expected.pdf` is byte-pinned and the sign-off names its digest —
   **that frozen literal is what the chain terminates in.** If that golden is ever re-recorded, **the
   transfer LAPSES and both fixtures need a real human reading.** Recorded as a *condition*, not a
   task.
2. **The anchor is scopeless.** It reads, in full, *"The rendering at fixtures/thai-stacked-marks/
   expected.pdf looks ok."* — unlike `fixtures/statement-signoff.json`, which enumerates what was
   checked. **An attestation's scope is what transfers, so a scopeless one transfers scopelessly.**
   Accepted here, because it is the owner's eye on a page whose entire subject is Thai mark
   placement. **It must not become the template: future readings name what was checked.**

**The second condition was checked before adding a tripwire, and needs none.** The transfer holds
only while the embedded bytes are the shipped face's. `embeddedFontAssetBytes()` returns
`testShippedNotoSansThai`, which is `//go:embed fonts/notosansthai/NotoSansThai-Regular.ttf` — **the
shipped file directly**. Held by construction; nothing added. **The standing rule:** a future fixture
embedding a face that is **not** a shipped face's bytes owes a **real human reading**, and must go
**red rather than transfer**.

**`fixtures/embedded-font/signoff.json` was written by the orchestrator, not delegated**, and is
written to be unmistakable as transferred: `"kind": "transferred-reading"`, a
`NOT_A_HUMAN_READING` field stating plainly that no human has looked at the file, and **no `reader`
and no `examined` field at all** — inventing either would be a fabricated attestation, and a
reconstructed record that does not say it is reconstructed is the failure mode this run has flagged
before. It also records what it does **not** claim, including the refuted Ts-free reasoning, so the
comfortable argument cannot be quoted back out of the file that declines to make it.

---

### D-8.4.9 — three closing items: the canvas abort, the corrected count, and a process breach not to be absorbed

**(a) The canvas whole-projection abort is a D-7.4.2 violation Story 8.4 made reachable — fixed, not
deferred, and the ruling adds an obligation the fix must still satisfy.** A pre-existing `return` in
`addCanvasTextPaint` became reachable **from document content**, on a document `folio-format.md` calls
**valid**; `wasm/engine.go:119,255,294` propagate it, so the designer could not open the very document
whose chain entry it existed to repair. This is the *pre-existing but propagated* shape ruled at Story
7.9: **the story that makes a latent violation reachable owns it.** The closer verified the repair
degrades across **seven** reachable shapes and bounded the reachability rather than sampling it.

**The obligation the lead added, from its own earlier omission:** D-7.4.2 changed a failure mode
**without naming the tests that asserted the old one**. So this fix must **name the tests currently
asserting the abort and convert them to assert the degrade** — otherwise they keep passing while
measuring nothing. Carried into the follow-up.

**(b) The golden count is 22 → 23 BY ADDITION, zero moved** — materially different from the
"21 vs 22" the orchestrator's own build brief framed it as. `fixtures/embedded-font/` had **no
`expected.pdf` at baseline**; what was re-recorded is `expected.json`'s digest. **No re-attestation
is owed** — `fixtures/statement-signoff.json`'s reading of 2026-08-30 stands untouched. The count is
corrected in the record because `fixtures/thai-stacked-marks/README.md` already calls itself *"the
corpus's twenty-second document"*, and a stale 21/22 would read as contradicting it.

The orchestrator's brief said *"the other 21 must not move"* and the implementation repeated it in two
places before the build caught it. The closer swept four survivors: three corrected, and the fourth —
inside the byte-identity-verified `<intent-contract>` — **left untouched, with the correction recorded
in `## Spec Change Log` instead.** **Ratified: the frozen contract stays byte-identical.** It is the
historical record of what was agreed, and silently editing it after the fact would destroy the one
property that makes freezing it worth anything.

**(c) A step-03 subagent committed on its own, out of order — recorded as a process breach, story not
reopened.** Step-03 does not authorize a commit. The builder re-measured everything afterwards and the
result was right — **but that is the wrong reason to close it: re-measurement is a RECOVERY, not a
repeatable guarantee.** The next occurrence may involve a builder who does not happen to re-measure.
It goes in the run's process record so a **second** instance re-prices the step ordering instead of
being absorbed again as a one-off. Per the standing rule that a deferral must name a **new** trigger,
*"we caught it this time"* is not one.

**(d) `followup_review_recommended` — CLEARED, on the closer's independent pass, not on the build's
report.** The flag was set by 3 medium patches. The closer ran the hard adversarial pass this
orchestrator asked for instead of clearing it on judgement, and its own integers replaced the build's:
dropping the embedded `chainFaceNames` arm reddens **15 top-level tests (22 with subtests)** where the
build reported 11; a `Validate`-only swallow reddens **5 (7)**. All three behavioural fixes were
re-probed independently, and all six rejected findings were spot-checked at their cited locations
(DW-87 discharged for this story). The evidence is in the Delivery Log.

---

### D-8.4.10 — the orchestrator shipped the Story 7.10 defect, in the commit arguing for honest records, after briefing against it

**Commit `4219a1b` left a standing red.** `fixtures/embedded-font/signoff.json` carries **two**
digests — its own fixture's and the anchor's — and **neither was registered**, so
`TestGoldenDigestAgreesAtEveryDeclaredSite` was **failing at HEAD** from that commit until `43da56a`.

This is **exactly the Story 7.10 defect**: adding a sign-off record and a matrix gate without
registering either, which produced two lint reds there and which that story's builder correctly
refused to fix on its own authority. Three aggravations, recorded because the pattern is the point:

1. It was made in the commit whose own message argues that a reconstructed record must say it is
   reconstructed — a commit about recording things honestly.
2. It was made **after** the orchestrator briefed the follow-up builder, in writing, *"Adding a
   sign-off record and a matrix gate without registering them produced two lint reds at Story 7.10 …
   Do not repeat it."*
3. The follow-up builder found it **only because it measured the baseline instead of assuming it** —
   the same discipline that caught the false `ShippedFaces` comparison at Story 8.3's close.

**The generalisation, which is the lead's own from the FR52 spellings: the person who writes the rule
is a likely author of the first violation of it.** Recording a hazard does not inoculate the recorder,
and the interval between writing the warning and committing the violation was one commit.

**One mitigating fact that is also the more useful finding: the registration was NOT EXPRESSIBLE in
the existing vocabulary.** The declared-site machinery's `signoff` case compares a record's
**top-level** `sha256`; the anchor digest lives at `transfer.anchor_sha256`, so declaring it would
have compared the wrong field and passed while measuring nothing. A **new site kind,
`transfer-anchor`**, was required. So the omission was not purely carelessness — but the correct move
on hitting an inexpressible registration was to **stop and say so**, which is what the follow-up
builder did and what the orchestrator did not.

---

### D-8.4.11 — D-8.4.9(a) discharged with the answer "none", shown rather than asserted

The obligation was to **name** the tests asserting the canvas abort and convert them, precisely
because D-7.4.2 had changed a failure mode without naming them. **The answer is that there were
none** — and the distinction that matters is that this was **searched three ways and shown**, not
reported as silence:

1. The suite is **green with the degrade in place**; an abort-asserting test would be red **now**.
2. `git show <c> | grep "^-func Test"` and the TypeScript equivalent across all five 8.4-era commits
   (`15ca0dd`, `1446b87`, `cca0c3c`, `b2efdb4`, `4219a1b`): the five removed Go tests are all about
   the embedded entry being **skipped at Render**, none about the canvas; **zero** TS tests removed.
   This is the check that distinguishes *"no test asserted it"* from *"a test asserted it and was
   deleted"* — the two states an unexamined green suite cannot tell apart.
3. **No** Go or TS test anywhere expects a non-nil error from `CanvasWithTextPaint`.

**Why none existed: the abort was unreachable from document content before Story 8.4**, so nothing had
ever constructed a document to reach it. That is the same fact that made the defect invisible in the
diff — and it is why "no test asserted the old behaviour" was, here, a **finding** rather than a
relief. See **DW-92**: the arm's *retained* half is pinned by nothing either.

---

### D-8.4.12 — DW-92 ruled: the canvas abort widens on the ATTRIBUTABILITY axis, and the lead's own premise was the false one

**The false premise is the lead's, in `folio-go/page_setup.go`:** *"SCOPED TO THE CAPABILITY ERROR,
deliberately. Only `template.UnsupportedFontMediaTypeError` is tolerated: a genuine internal shaping
fault is not a document property, has no author repair, and still aborts the projection as it always
did."* That sentence is **true about internal shaping faults and false about the set the gate actually
excludes.** `errors.As` is keyed on **error type**; the population it excludes contains document
properties. Verified rather than taken: `checkSfnt` bounds-checks the header and every table-directory
record and **never reads a table's contents**, so truncation, bad version tags, zero tables and `ttcf`
collections **are** caught — and a valid directory over unreadable contents is not. That face reaches
`fontset.New` via `parseEmbedded` ← `fontCache.get` ← `shapeSegments`, fails `errors.As`, and
`wasm/engine.go:119,255,294` turn it into `Snapshot{}, err`.

**It is the same axis error as D-7.3.1**, and the lead named it as such: there a set was split by JSON
key **location** when the property that mattered was **which consumer reads it**; here by **which Go
type carries the error** when the property that matters is **whether the fault is attributable to
document content the author can edit on this surface**. **Both times the mechanism named was narrower
than the invariant meant, and both times the implementation was faithful to the words.**

**Verdict.** On the canvas projection, a fault arising from resolving a face **the element's own chain
names** degrades that element and never aborts the projection, **whatever error type carries it**. A
fault arising **after** every face that element needs has resolved keeps aborting. **The gate is
positional, not an enumerated allowlist.**

**Six guardrails, and two of them forbid the obvious implementations:**

1. **Delete the allowlist; do not extend it.** Adding `*fontset.ParseError` beside the existing type
   re-creates the defect one story out: every future face-parse failure type silently rejoins the
   abort and nothing observes it. The discriminator must be **one type minted at the single door that
   owns the attribution** — the `fontCache` face-resolution path, which knows it is resolving a chain
   entry of *this* document. **One site, closed by construction, cannot omit a member.**
2. **Do not pre-resolve the whole chain to get the separation** — checked, and it introduces a *new*
   defect: `resolveRuneFace` skips face names the runes do not need, so a pre-pass would degrade an
   element because of an entry **it never draws with**. **Attribute at the point of failure; do not
   relocate it.** And `chainVerticalModel` is not the seam — `metricsFace` already returns
   `nil, false, nil` for an unparseable embedded face, so **one condition currently has three
   dispositions**.
3. **The retained abort must gain a test — and THAT is the finding, not the abort.** *Mutating that
   arm to degrade reddened nothing in the entire suite.* The half Story 8.4 changed was pinned by one
   test; the half **retained** was pinned by **none** — which is exactly why an unmeasured premise
   survived a gate, a closer and the lead. **General lesson recorded: when a ruling splits a
   population, the retained half is the one nothing tests.**
4. **Invert `TestCanvasStillAbortsOnAnUnreadableCarriedFace`; do not delete it**, keeping its
   `ParseTemplate` precondition `Fatal` **verbatim** — that Fatal is the tripwire for a future story
   moving the check into the loader, and it is **independent of which way the arm points**. The
   builder's **declining to ratify its own measurement was the correct call**, and guarding the
   *precondition* rather than only the behaviour is the part to repeat.
5. **Amend `ContentWindowCountIsExact` cause (c)** — it reads *"its font chain would not resolve"*,
   which does not cover a chain that **resolves to a face that will not parse**. A stale enumeration
   reads as excluding the case this ruling adds.
6. **A host-supplied `FontSet` face that will not parse is OUT of this population**, and it is named
   because it is the case that **discriminates the two axes**: an error-type gate sweeps it in, the
   attributability axis correctly leaves it out — a `FontSet` face is the **host application's**, not
   the document's, and no edit on the canvas repairs it.

**The remaining silence, ruled acceptable and registered rather than left implicit.** `CanvasProjection`
carries **no diagnostics channel**; `FontChainDegraded` is internal and never projected, so a degraded
element is blank with no reason. Accepted **here** on two narrow grounds: D-7.4.2 already established
silent per-element degrade as the canvas's disposition for an unresolvable chain, so this **adds a
member to an existing ruled class** rather than opening a new one; and the reason is **not
unreachable** — the same face still errors with a message on the Render path. Per the standing rule
that *a silence ruled owes an escape hatch*, the obligation is registered (**DW-93**) with a **new**
trigger: *a story that must distinguish "no chain chosen" from "the chosen face will not load" on the
canvas.*

**Confidence, stated by the lead:** **high** on the widening and on (3) being the real finding;
**medium** on the mechanism in (1) — the assumption that would flip it is that the `fontCache` door can
carry the attribution without a caller passing it in. **If it cannot, keep the invariant and find
another single site — do not fall back to an allowlist.**

**The symmetry worth keeping.** `checkSfnt`'s own comment already records that a *previous* unmeasured
negative in that same file was found and proved false. A new unmeasured negative was then written one
story later, about the same file's blind spot. **The file had already said where it does not look, in
a comment, and nobody read that as a population question.**

---

### D-8.4.13 — every line anchor carried across a story boundary in this run has rotted; they are removed

Story 8.4a's plan gate found **all four** anchors handed to it stale — including ones quoted in
**D-8.4.6** and in **DW-35**. `canvas-font-stack.test.ts` grew **189→237 lines** during Story 8.4, so
the `no var(` assertion moved `:100-106`→**`:137`** and the chain-entry prohibition
`:123-132`→**`:224`**; `page_setup.go:1344` is now a `positionSegments` error return, with the discard
site at **`:1381`**; and `table_render.go` calls `chainFaceNames` at **`:676`**, not the `:665`
**D-8.4.4(a) records**.

**Ruling: assertions are named by WHAT THEY ASSERT, not by line, in every artifact that outlives a
story.** A stale anchor is worse than none — it sends a reader to a **real line that says something
else**, which reads as authoritative. The epic text is amended accordingly; **the decision log's
history is NOT rewritten**, on the same principle that kept Story 8.4's frozen contract byte-identical:
a record edited after the fact to look correct is worth less than one that shows what was believed
when.

**Two more corrections from the same gate.**

**(a) AC3's justification was false; its requirement stands (the D-8.4.2 shape, third occurrence).**
AC3 justified itself by `tokens.css`'s `--font-page` being Thai-first. Measured: **`--font-page` never
reaches canvas text at all** — it feeds three type tokens, one used, on chrome, while
`.canvas-text-fragment` uses a **hardcoded Latin-first literal with no `var()`**. The named hazard is
not the hazard; the order requirement is unaffected.

**(b) One premise the orchestrator supplied was not stale but BACKWARDS, and the truth is worse.** It
told the gate that an unlisted fragment key *"blanks the canvas with no diagnostic."* It raises
`PROTOCOL_INVALID` at `engine-client.ts:87`, which **terminates the worker** and rejects every pending
request — **the session is dead until reload.** That makes the Go/TS fragment edit a **hard ordering
constraint**, not a cosmetic one.

**And the limit this story must be built and closed under, recorded before any code exists.** **No
gate in this repository can confirm the canvas visibly paints with the carried face.**
`test:e2e:compile` is `tsc --noEmit`; `npm run test:e2e` (Playwright) appears in **no workflow**, and
browser e2e is deferred by D-000.4; jsdom applies no stylesheet and implements no font loading. Vitest
can prove the derivation, the registration **call**, the fragment attribute, the guards and the
protocol shape — **nothing here can execute a real font load, a real `document.fonts.add`, or a
rasterized glyph.** Written into the spec's Design Notes so the closer inherits it, because **a
compile pass must never be reported as a run.**

---

### D-8.4.14 — 8.4a closes ONE of DW-35's two causes; Story 8.4b owns the other, and the register's blocker was false

**(a) Confirmed: 8.4a closes cause two only, and AC4's tie must be scoped to the carried case.** The
gate's cheap-overrule offer was declined. Verified rather than taken:
`canvas-font-stack.test.ts` asserts engine/browser family disjointness **in both directions** over
`['Noto Sans','Noto Sans Thai','Noto Sans SC']` vs `['IBM Plex Sans','IBM Plex Mono','IBM Plex Sans
Thai']`. **A builder handed the universal form of AC4 finds it red and weakens it** — which is
precisely the failure AC4 exists to prevent, and the failure mode flagged all run: **an AC that
restates a ruling too broadly gets implemented faithfully and shipped wrong.**

**(b) Cause one gets an owner by ruling — and the blocker the register recorded is FALSE.** DW-35
called cause one *"a design-system decision above a builder's authority"*, meaning **renaming the
generated `@font-face` families to Noto**. Measured: `build-wasm.mjs` gives IBM Plex family names to
three **Noto** files, and `find -iname '*plex*'` is **empty**.

**The lead's own first reading was "mislabel — rename it, the escalation collapses", and a single grep
flipped it.** IBM Plex is the UX design system's specified typeface, named throughout `DESIGN.md`, and
promised in `epics.md`'s release **licence manifest**. **The tree is the stand-in, not the label.**
Renaming would have abandoned a real decision and falsified a release artifact.

**So the fork dissolves rather than escalating: DW-35 is about what the CANVAS paints with and says
nothing about chrome.** Verdict — register the shipped faces **additionally** under the engine's own
face names, point the canvas fragment rule there, and **edit no chrome token at all**. No new
binaries, no visual change, no doc amendment. Grounded on **AD-17**: the browser contributes
rasterization only, so it must rasterize with the face the engine measured with — **and it cannot
unless it can name that face.** It is also **D-8.4.1 one case over**: a carried face's browser family
derives from the engine's identity for it (the asset key); a shipped face's from the engine's identity
for it (the `FontSet` name). **One rule for one question.** A mapping table is the alternative and it
is the wrong shape — a second authority maintained in lockstep with the shipped `FontSet`.

**Owner: Story 8.4b**, written into the epic and sequenced after 8.4a. Per D-8.4.1's standing rule this
is a **ruling**, not a recommendation, so **DW-35 survives 8.4a WITH a named successor rather than
without one** — which is the exact state that rule exists to prevent.

**Guardrail carried into 8.4b:** the disjointness assertion is **replaced, not weakened** — it records
the old state and *should* go red; and *"the vocabularies now meet"* is **not** licence to make chrome
ask for Noto. That is D-8.4.15's question, settled there and settled the other way.

---

### D-8.4.15 — OWNER DECISION: ship IBM Plex for real. The lead recommended the opposite, and its own hedge is what settled it

**The escalation.** The product **specifies** IBM Plex throughout `DESIGN.md`, **promises** it in the
redistributed-asset **licence manifest**, and **ships no IBM Plex file** — three generated
`@font-face` rules put IBM Plex family names over three Noto files. A licence manifest naming a font
the product does not contain is **compliance-shaped**, and a collision with established direction is
an escalation by the lead's own standing rule.

**The owner chose option 1: ship IBM Plex for real.** Add the OFL binaries, keep the names honest,
make the manifest true.

**The lead recommended option 2 (adopt Noto, amend the docs) — and named the fact that would flip
it:** *"option 1 is the right answer instead if the IBM Plex choice was real and the Noto files were a
stand-in nobody replaced — which is exactly the fact I cannot establish and you can."* **It was.**
The recommendation rested on *"the product has rendered in Noto its entire life"*, which was true and
**not decisive: it described the stand-in, not the intent.** The hedge is what let this be answered in
one pass instead of two, and it is the pattern to repeat — **a recommendation that names its own
defeater costs one sentence and saves a round trip.**

**Second answer: the `--font-mono` defect is fixed NOW, separately.** `'IBM Plex Mono'` is bound to
`noto-sans-cjk.*.ttf` — a CJK sans — so `--type-mono*`, `--type-brand*`, `--type-band-tab` and
`--type-numeric-lg` render chrome in a CJK face today.

**A COUPLING RESOLVED BY INTERPRETATION, AND LABELLED AS ONE.** With option 1 chosen, the honest fix
for the mono defect **is** adding a real IBM Plex Mono binary — so *"fix it now, separately"* and
*"ship IBM Plex"* became the same work, differing only in scope and order. Read as **its own commit,
landing first**, within **Story 8.4c**, rather than as a separate story. **That reading is the
orchestrator's, not the owner's words**, and is recorded as such in the epic so a later reader does not
attribute it to the owner.

**The acceptance risk of the chosen option is written into 8.4c rather than left to be discovered:**
someone must confirm **IBM Plex has the Thai coverage `--font-page` asks for** before that binding is
trusted. And the bundle weight the owner accepted is **measured and recorded**, not absorbed.

**Net shape after 8.4b and 8.4c:** chrome asks for **real IBM Plex**; the canvas asks for the
**engine's Noto face names**. The two vocabularies stay separate **by design rather than by
accident** — which is what 8.4b was ruled on AD-17 grounds *before* this decision was known.

---

### D-8.4.16 — D-8.4.12 implemented, with one placement deviation reported rather than worked around

**Landed at `5d705b6`.** Verification: untagged **1805/2/5**, matrix **1816/3/5** — the two standing
reds, no third; 23 goldens, none moved; baseline measured at `e25381a` rather than assumed.

**The ruling's MECHANISM held; its PLACEMENT did not.** The assumption the lead flagged as the one that
would flip guardrail 1 — *that the `fontCache` door can carry the attribution without a caller passing
it in* — is **true**, and `get`'s two arms are the whole attributability axis.

**What failed was anticipated by nobody: the stamp cannot be a type declared in a root file of package
`folio`.** `TestFolioMethodNamesAreInjective` walks that package's root non-test files and fails when
two receiver types declare the same method name; `*RenderError` already declares `Error` and `Unwrap`.
A package-local error type reddened it twice. Its own two stated remedies are **rename the methods**
(impossible — `error` mandates `Error() string`) and **take DW-20 now** (a `go/types` walker, a
separate story). The guard is deliberate, has no allowlist, and its doc comment says loosening it is
the exact failure it exists to prevent.

**So the carrier moved and the door did not.** `template.CarriedFaceError` sits beside
`UnsupportedFontMediaTypeError` — the document-attribution error it generalises and now wraps — and is
still minted at **exactly one site**. **No allowlist.** The argument for it being better on the merits
rather than merely tolerable: **attribution is a format fact, so it belongs in the package that owns
the format.**

**Two consequences the lead was asked to weigh.** **DW-20's pressure is now real rather than
theoretical** — the guard fired for the first time on a legitimate change, and the *next*
package-level error type in `folio` fires it again with no route around it. And **the constraint was
invisible in the ruling and would have been invisible in any spec: it is only findable by running the
suite.**

**Guardrail 2 got more than it asked for.** The three dispositions reconciled to **two**: `metricsFace`
now reads the same stamp instead of re-deciding attribution from the face's **name** — a consumer
re-deciding the axis, the exact shape the ruling condemned at the canvas gate.

**Guardrail 3 discharged, with the number.** The arm that reddened **nothing** now reddens
`TestCanvasStillAbortsOnAHostFontSetFaceThatWillNotParse`, which differs from the degrading document in
**one variable: who supplied the face**. Guardrail 6 is asserted directly — `errors.As` must be
**false** for the `FontSet` arm — so the axis cannot collapse silently. Restoring the old allowlist
reddens **exactly one** test, so the widening is observed and nothing else moved.

**Two self-caught failures worth more than the feature.** A **third red appeared mid-run** —
`TestFolioMethodNamesAreInjective`, real, from the builder's own first implementation — and it surfaced
**only because the full suite was run under mutation**; the targeted run was green while the arch guard
was red. And the **first mutation harness silently let a non-compiling mutation through and reported
`pass: 753` for a package that never ran**; it was rebuilt to fail loudly and everything re-run. Both
are this run's recurring family: **a green that measured nothing** — here caught by the builder on
itself.

---

### D-8.4.17 — the placement was never a deviation; DW-20 re-priced on a FIRED trigger; and the epic resequences to 8.4a → 8.4c → 8.4b

**(a) D-8.4.16's "deviation" is ratified, and the lead reclassified it rather than forgiving it.** Read
against the guard itself rather than taken from the report: the verdict constrained **where the type is
MINTED** — *"one type minted at the single door that owns the attribution"* — and `CarriedFaceError` is
still stamped at exactly one site, `fontCache.get`'s embedded arm, on **every** error out of that arm.
**Declaration site is package organisation; minting site is the invariant.** By the standing rule that a
ruling's verdict line outranks its bindings, **the verdict holds exactly**, and what failed was an
incidental locational assumption the lead's own prose invited. **The lead recorded that as its own
error, not the builder's.**

**The builder's ground is better than "tolerable" — it is right.** Attribution is a format fact, so it
belongs beside `UnsupportedFontMediaTypeError`, which it generalises and wraps. **Two properties in
the shipped comment are stronger than anything the ruling asked for**: the stamp **adds no text**, so
every located string the render path prints stays **byte-identical**; and `Unwrap` keeps every existing
`errors.As`/`errors.Is` target reachable **including the media-type error the old allowlist named**.
The lead states it would have missed both.

**Reporting it rather than working around it is the behaviour to keep**, and the reason is worth
holding: **a builder that silently relocates a ruled mechanism leaves a log entry describing code that
does not exist.**

**(b) DW-20 re-priced, not taken — because its trigger FIRED rather than failed.** The guard did
exactly what its own comment predicted: *"the failure mode is a LEGITIMATE commit blocked by an edge
that is not really there, followed by somebody 'fixing' it by loosening
`TestValidateNeverReachesRenderOrInternalPDF`. The safe direction is the dangerous one."* It announced,
and the change **routed around it into a better shape**. **Re-deferring on a fired trigger is
legitimate where renewing one that never fired would not be.**

- **Not inside Epic 8** — four stories outstanding, and the owner has just said "now" about different
  work.
- **New, sharper trigger:** the next change needing a second same-named method on a **root-file receiver
  in package `folio` that CANNOT be relocated** — an error type that must be part of `folio`'s own
  **exported** API has no route around the guard, and `*RenderError` is already exported, so a second
  exported error type is the realistic instance.
- **This occurrence is instance ONE. On instance two, DW-20 is taken BEFORE that change lands, with no
  further deferral.**
- **STANDING PROHIBITION, recorded now rather than discovered later: loosening
  `TestValidateNeverReachesRenderOrInternalPDF` — or any sibling reachability guard — to clear an
  injectivity red is FORBIDDEN. If it appears in a diff it is a STOP, not a finding.** A future builder
  under schedule pressure will find it the obvious repair.
- **The replacement is cheap, not greenfield** — `lint` already reaches across the module boundary with
  `packages.Load` — recorded so *"expensive"* cannot become the third deferral's reason. **A criterion
  erodes from both sides.**

**(c) Sequencing: `8.4a → 8.4c (chrome typeface) → 8.4b → 8.5 → 8.6`.** `sprint-status.yaml` reordered
accordingly.

**The ground is NOT file collision** — checked, and smaller than it looks: **under option 1 the chrome
family NAMES do not change at all**, only the files behind them plus the mono binary, so `declared`
stays `['IBM Plex Sans','IBM Plex Mono','IBM Plex Sans Thai']` and the disjointness assertion stays
true throughout. **Only 8.4b changes `declared`.**

**The ground is the INTERMEDIATE STATE.** If 8.4b landed first it would register **the same Noto file
under two family names** — `IBM Plex Sans` and `Noto Sans` — which reads as redundancy and invites a
future reader to "simplify" it by deleting one. After the IBM Plex work the two registrations point at
**genuinely different files** and the separation is self-evident from the code. **Do not create a state
whose only defence is a comment.**

8.4a goes first rather than having a gated spec wait. **If the owner meant "now" literally — ahead of
8.4a — that is a one-line correction**, and the cost is a gated spec sitting one story longer.

**The "own commit, landing first, same story" reading is confirmed, with two grounds beyond the
owner's words.** The mono binary is **the cheapest end-to-end proof the IBM Plex pipeline works** — one
binary, one generated rule, one licence entry — so it is the right first slice on engineering grounds
independent of the instruction. And splitting it into its own story would put the **licence-manifest
edit in a different unit from the binaries it describes**; the manifest update belongs in that story's
**final** commit, and until then it is *already* false, so incremental truth introduces no new
falsehood.

**(d) The lead's own lesson from being overruled, recorded because it is the more useful half.** Its
recommendation rested on *"the product has rendered in Noto for its entire life."* That was **true and
was not evidence for the conclusion drawn from it**: the current state of an artifact **cannot
distinguish "this is what we chose" from "this is what nobody replaced."** Both hypotheses predict the
identical observation, so **the observation discriminated nothing** — the test the lead applies to
others' evidence and did not apply to its own. **The hedge is the only reason it cost one pass instead
of a wrong build, and the hedge should not have been carrying that much weight.**

**(e) Two builder self-catches promoted to the run's process record, because both generalise.**

1. **A targeted run cannot see an architectural guard.** The third red was real, caused by the
   implementation itself, and the **targeted package run was GREEN while the arch guard was RED** —
   because arch guards live **outside** the package under change. **Any story touching type or method
   structure in `folio`'s root runs the full suite before reporting green.** This is a **scoping**
   lesson, not a mutation-testing one.
2. **A non-compiling mutation reported `pass: 753` for a package that never ran.** This run's recurring
   failure in another costume: **an all-clear indistinguishable from a couldn't-look.** General rule:
   **any harness reporting a count owes a distinct, noisy state for "could not execute" — a compile
   failure must never be able to arrive as a pass.**

**Both were caught by the builder on itself, after a green.**

---

### D-8.4.18 — the step-03 commit breach reaches INSTANCE TWO, and the recovery held again, which is the problem

**Story 8.4a's `c4cd60c` was created by the step-03 implementation subagent.** Step-03 does not
authorize a commit — finalizing is step-04's. The builder kept it (correctly scoped, and step-04 says
to keep commits already made) and added `51e38ac`. The closer audited both: this story's paths only,
trailer present, root `README.md` absent and unchanged, no signoff or fixture files.

**This is instance two.** Instance one is D-8.4.9(c), recorded with the note that **re-measurement is a
recovery, not a repeatable guarantee.** Both times the recovery held. **That is exactly why it is now
worth a price rather than a third note:** the mechanism has produced two unauthorized commits and been
caught twice by a downstream agent that happened to re-measure. Per the standing rule that an Nth
instance **re-prices** a deferral rather than renewing it, this is no longer *"we caught it this
time"* — it is a recurring ordering defect in the build loop whose only defence has been diligence
downstream.

**It is not this run's to fix** — the step ordering lives in the `bmad-build-auto` skill, not in this
repository — so it is recorded here with its count, so a third instance is priced against two prior
ones rather than being met fresh.

---

### D-8.4.19 — Story 8.4a's closer overturned a deferral by falsifying its ground, and that is the pattern to keep

**The build deferred the "two embedded entries in one chain" attribution row**, on the ground that a
second carried font fixture *"creates a new golden digest and a new human sign-off obligation"* — which
this orchestrator's own dispatch had placed above a builder's authority. **The closer disagreed,
falsified the ground, and closed it.**

**The ground is false for the surface it names:** the document is built in Go from bytes **already
committed**, and it is **projected, never rendered** — so there is no PDF, no golden, no digest and no
attestation. `TestTwoCarriedFacesInOneChainAreAttributedToTheirOwnKeys` adds **no binary, no fixture,
no golden, no sign-off**. Red-proved: attributing every fragment to the first indexed key leaves **every
pre-existing test in that file green** and reddens only the new one.

**Why this is worth a decision entry rather than a line in a log.** A deferral is normally respected —
but a deferral is only as good as its **stated ground**, and *"this would require a human sign-off"* is
a ground that **stops downstream review by invoking an authority boundary.** It is therefore the most
valuable kind to check and the least likely to be checked. The closer checked it. **An authority
boundary correctly invoked protects a decision; incorrectly invoked it launders a gap past everyone
downstream.**

**Two smaller closer findings, recorded because both are the run's recurring shape.**

**(a) A rejection with the right verdict and an understated ground.** The build refused a finding about
`fragment.assetKey !== undefined` as *"cosmetic"*. The premise is false — removing the check gives
**two `TS2345` errors** under `strict`. Right answer, wrong reason, and a wrong reason is what a later
reader inherits.

**(b) The build's own package counts did not reproduce.** It reported `16 ok` / `13 ok`; the closer
measured `13` / `12`. **Red sets and identities match exactly**, so nothing is wrong with the result —
but **its counting is not reliable**, and counts are what this run has repeatedly used as evidence.
Prefer identities to counts when relaying a gate.

**Also from the close:** weakening-by-evasion was tested **directly** rather than argued — swapping the
derived family for `'IBM Plex Sans'`, which `App.css` declares and which the old stylesheet-only tie
would therefore have passed, **reddens both widened guards.** And the repaired `document.fonts` rule
was re-proved **on the real corpus**, where the build had proved it only on fixtures.

---

### D-8.4.20 — Story 8.4c's plan gate: the licence check's blind spot is an extension list, and nothing in the repo can observe a font file swap

Recorded at the gate, before any code exists. **Three findings; the second is the one that would have shipped.**

**(a) The Thai-coverage acceptance risk is DISCHARGED BY MEASUREMENT.** It was written into the epic as
the risk of the owner's chosen option specifically so it would not be discovered later. The gate
measured rather than assumed: **IBM Plex Sans Thai 1.1.0 has exact cmap parity with the shipped Noto
Sans Thai** — 87/128 in U+0E00–U+0E7F, **empty set difference in both directions** — covers both
strings from the recorded rendering defect, and reaches **strictly more** Thai-script GPOS mark
positioning (MarkBasePos ×3 / MarkMarkPos ×2 against ×1 / ×2). **One recorded difference: Noto declares
a `thai` `dist` feature Plex does not.** Recorded rather than waved past.

**(b) THE OBVIOUS PROCUREMENT ROUTE BYPASSES AD-26 SILENTLY.** `manifest.ResolveAssets` walks the whole
repo and **hard-fails** without a same-directory `LICENSE*` + `NOTICE*` — **but it recognises only
`.ttf` / `.otf` / `.ttc`.** The npm `@ibm/plex-*` packages declare `OFL-1.1` and ship **zero `.ttf`**.
So installing IBM Plex the obvious way would have left the **licence gate green over binaries it never
looked at** — bypassing the very check this story exists to satisfy. Now an Always constraint in the
spec.

**This is the `checkSfnt` shape again, and that is the third occurrence of it this run: a checker whose
blind spot is a list nobody re-read as a population question.** `checkSfnt` bounds-checks the table
directory and never reads a table's contents; the injectivity guard walks root non-test files only;
this one enumerates three extensions. **Each is correct about what it examines and silent about what it
does not** — and in every case the silence was found by someone measuring the population rather than
reading the check.

**(c) NOTHING IN THE REPOSITORY CAN OBSERVE A FONT FILE SWAP.** Across 443 lines of
`canvas-font-stack.test.ts` **not one assertion opens an `@font-face` `src`, a filename, or a byte** —
every assertion is about family **names**. So a story whose entire subject is **changing which file
sits behind a name** has **no existing net at all** and must build its own; AC1 is phrased as an
observation rather than an edit for exactly that reason. **It also explains how the mislabel survived:
the guards would have stayed green over IBM Plex names on Noto bytes indefinitely.**

**Two premises checked and confirmed rather than assumed.** The chrome family **names** do not change
under the owner's option — `tokens.css` and `App.css` need no edit, `declared` is parsed from family
names only and the `src:` is never read, so the engine/browser disjointness assertion **stays true
throughout** and the sequencing rationale holds. And the bundle gains **+490,280 bytes**
(200,500 + 173,052 + 116,728), new total **11,780,160, +4.34%** — the cost the owner accepted, measured.

**And a standing red re-described in the only words that are honest.**
`TestShippedFacesReproduceFromUpstream` is a **could-not-execute, not a byte divergence**:
`fontgen: fontTools is not importable by this interpreter`. **It never compared bytes.** Per DW-86 that
distinction is the whole reason it must fail rather than skip — **an all-clear must differ from a
couldn't-look**, and so must a red.

---

### D-8.4.21 — NFR7's size budget is prose, and it was already exceeded by 37% before this story

`epics.md` accepts *"~9 MB first load"*. The committed release manifest measures **12,372,693** Brotli
bytes. **Nothing enforces the figure** — no test reads it, no gate compares against it. Story 8.4c adds
**+4.34%** to a number that is **already 37% over a limit nothing checks**.

**Filed rather than acted on at the gate, correctly** — it is a **separate defect** from 8.4c, and a
story must not silently absorb a budget breach it did not cause. But it is recorded **here** rather
than only in the register because it is the same family as (b) and (c) above: **a stated constraint
that no mechanism enforces reads as enforced.** "Measure and record the added weight" is one of 8.4c's
own acceptance criteria, which is what surfaced it — the measurement had a place to land, so the
discrepancy could not stay invisible.

**Routed to the engineering lead** with three dispositions: make the budget enforceable here, file it
with an owner, or correct the epic to a figure that is true. **Not resolved by the orchestrator**,
because all three are direction calls and the third — amending a stated product constraint to match
reality — is the one most likely to be chosen for being cheapest.

---

### D-8.4.22 — **D-8.4.17(c) IS REVERSED.** The order is `8.4a → 8.4b → 8.4c → 8.5 → 8.6`

**Recorded as REVERSED, not amended, so the log shows the ground that changed.**

**The gate found one third of the accident.** It reported that CJK canvas text rasterizes by accident,
scoping it to the mono slot. Measured, **the whole fragment stack is an accident**:

```
.canvas-text-fragment:  'IBM Plex Sans'      'IBM Plex Sans Thai'      'IBM Plex Mono'
build-wasm.mjs      →   NotoSans-Regular     NotoSansThai-Regular      NotoSansSC-Regular
engine FontSet      →   Noto Sans            Noto Sans Thai            Noto Sans SC
```

**The canvas is accidentally correct for EVERY script, not just CJK.** Those three IBM Plex names
resolve today to **precisely the engine's three faces, in the engine's own fallback order.** That is
why nothing has ever looked wrong.

**So the interval was never "CJK is broken for one story."** After 8.4c all three names carry **real
IBM Plex bytes**, and **every** fragment — Latin, Thai and CJK — rasterizes with a face the engine
never measured with, at positions the engine computed from **Noto** metrics. **The canvas loses
fidelity wholesale for one story, on the surface whose entire purpose is showing the author what the
engine will print.** A direct **AD-17** regression.

**And the Thai measurement does not rescue it: CMAP PARITY IS NOT METRIC PARITY.** Identical coverage
with different advance widths draws **the right glyphs in the wrong places**. **The CJK case is the
most visible because it degrades to system fallback; the Latin and Thai cases are worse because they
look plausible.** This is the correction that matters most — D-8.4.20(a) discharged a *coverage* risk
and was read, including by this orchestrator, as discharging more than it did.

**Why the original ground lost.** D-8.4.17(c)'s objection was real: 8.4b-first registers
`noto-sans.ttf` under both `IBM Plex Sans` and `Noto Sans`, *"a state whose only defence is a
comment."* **But it was weighed against nothing** — the lead reasoned about the mono slot as an
isolated mislabel and **never followed the other two names to their files.** A maintenance hazard
lasting one story loses to a wholesale rasterization regression lasting one story.

**The objection was cheaply answerable, which is the second reason to flip — and the better lesson.**
The intermediate state does **not** have to be defended by a comment: **assert it.** 8.4b now carries
an AC pinning that the two names deliberately resolve to one file and **naming 8.4c as the successor
that makes them diverge**, so a future "simplification" goes **red**. **Converting an objection into a
guard with an anchor is what should have been proposed instead of using it as an ordering ground** —
this run has repeatedly found that a comment carrying a test's evidentiary burden is the defect, and
here it was very nearly used to justify a sequence.

**8.4c's `ready-for-dev` spec is not wasted** — nothing in it changes; it moves one slot later. 8.4b is
the smaller spec. **Do not merge them:** 8.4c is already `oversized` + `multiple-goals`, and the two
have genuinely different subjects.

---

### D-8.4.23 — a spec constraint is prose; the licence blind spot and the file-swap blind spot are ONE guard, and it lands in the first commit

**Confirmed at `lint/internal/manifest/manifest.go`:** `fontExtensions` is `[".ttf", ".otf", ".ttc"]`.
The `@ibm/plex-*` npm packages ship `.woff2`/`.woff` and **no `.ttf`**, so the obvious procurement
route would have put font binaries into the bundle **the licence gate cannot see** — green over files
it considers unlicensed.

**Making it an "Always" constraint in 8.4c's spec is NOT ENOUGH, and that is the ruling.** A spec
constraint **binds one story**, and **the next person adding a font by a different route is not reading
this spec.** 8.4c owes a **behavioural guard**: a test that fails when any font file reaching the
runtime bundle carries an extension `fontExtensions` does not recognise. **That closes the CLASS by
construction rather than this INSTANCE.** Cheap, because the bundle's asset list is already enumerated
by the generator. The invariant is *the licence gate sees every font that ships*; the mechanism is the
story's to choose.

**It is the SAME guard as the file-swap gap, which is why both are ruled together.** Nothing in the
repository can observe a font file swap — **zero assertions across 443 lines touch an `@font-face`
`src`, a filename or a byte.** **A guard that checks only names cannot observe a change of the thing
behind the name**, which is exactly how IBM-Plex-names-over-Noto-bytes survived indefinitely, and how
the accident in D-8.4.22 went unnoticed by the lead, the gate and every guard in the repo.

**It compounds across the two stories, which is the elegant part:** with 8.4b first, the guard's **first
job** is to pin the deliberate two-names-one-file state; **8.4c's job is to update it when they
diverge.** Same guard, two stories, **each story's change visible in it.**

**It lands in 8.4c's FIRST commit — the same one as the mono binary — before any binary arrives by any
route. A guard added after the thing it guards has already shipped is one that was never able to
fail.**

**Third instance of one shape this run**, after `checkSfnt` and the golden-digest site kinds: **a
checker whose blind spot is a list nobody re-read as a population question.**

**Warnings: both accepted, no split.** `oversized` and `multiple-goals` are true and both answered by
**structure rather than division** — the mono binary is its own commit within the story, and the
licence-manifest edit must stay in the same unit as the binaries it describes, since splitting it is
what would create a commit where **the manifest describes files that are not there**. **A story that is
large because its parts are genuinely inseparable is not the failure those warnings exist to catch.**

---

### D-8.4.24 — OWNER DECISION: pick a figure and make it enforceable. Story 8.4d, and the epic is NOT rewritten to match the measurement

**The escalation.** `epics.md` accepts *"~9 MB first load"*; the committed release manifest measures
**12,372,693** Brotli bytes — **37% over, since before Epic 8, enforced by nothing.** Story 8.4c adds
**+490,280 bytes (+4.34%)**.

**The owner chose option 2 — set a chosen figure and make THAT enforceable** — which was also the
lead's recommendation, on the ground that *"it stops the drift immediately, which is the failure that
actually occurred here, and it does not pretend a 7.4 MB CJK face fits in 9 MB."*

**The lead hedged this one explicitly, deliberately, and said why:** option 1 (hold ~9 MB and subset or
defer the CJK face) *"is right instead if ~9 MB was a real commitment to a user or a platform
constraint rather than an estimate written during planning — which is the fact I cannot establish and
you can."* It named that as **the same hedge shape** that let the IBM Plex question settle in one pass.
**Twice now a recommendation that named its own defeater has cost one round trip instead of a wrong
build.** That is now a practice, not a coincidence.

**THE RULING INSIDE THE DECISION, which the owner's answer does not by itself settle: the epic must NOT
be rewritten to 12.4 MB.** Rewriting *"~9 MB"* to *"~12.4 MB"* is **moving the threshold to match the
measurement** — the twin of manufacturing sample data to meet a floor — and it **enshrines the overage
as the target while looking like a documentation fix.** **A budget rewritten to whatever the build
currently weighs is not a budget.** So the new figure is **chosen**, the artifact declaring it records
that it was chosen, and the old ~9 MB is **superseded in place with its history** rather than
overwritten — a document that has always said the current number teaches a later reader nothing.

**Placement: Story 8.4d, sequenced after 8.4c** — `8.4a → 8.4b → 8.4c → 8.4d → 8.5 → 8.6`.
**Not folded into 8.4c**, and the reason is mechanical rather than stylistic: the enforceable figure is
*today's measurement plus 8.4c's addition and nothing more*, so **it cannot be written until 8.4c has
landed.** 8.4c's obligation stays exactly what the lead ruled — **record the added weight, do not fix
the budget** — and a story must not silently absorb a budget breach it did not cause.

**The AC that makes this more than a number in a file:** raising the figure later must be a **visible,
deliberate edit** that reds the gate first. **The failure here was never the 12.4 MB — it was that
nothing compared two numbers that were both written down.**

---

### D-8.4.25 — the orchestrator's e2e claim was FALSE and of the wrong category; corrected in place, trigger replaced, stories not re-opened, and the real defect is upstream

**The claim, repeated to the lead and to two plan gates, and written into the Design Notes of both
Story 8.4a and Story 8.4b:** *"Nothing in this repository can execute a real font load, a real
`document.fonts.add`, or a rasterized glyph — Playwright appears in no workflow."*

**It is false.** Verified independently by the lead: **12** spec files in `folio-designer/e2e/`;
`"test:e2e": "playwright test"`; a real `webServer` in `playwright.config.ts` running
`npm run build && npm run preview`; `@playwright/test` installed. **What is true is narrower:
`.github/workflows/ci.yml` runs only `test:e2e:compile`. A CI WIRING FACT WAS GENERALISED INTO A
CAPABILITY CLAIM.**

**And the second time it was RUN rather than inferred.** The suite **built, started the preview server
and executed all four specs** in the file tried, failing only on
`browserType.launch: Executable doesn't exist … chromium_headless_shell-1208`. **One
`npx playwright install` away.**

**So it was a COULDN'T-LOOK, not a CANNOT-LOOK** — the distinction enforced on every other check in
this run, applied to everyone's claims except the orchestrator's own.

**(a) Corrected IN PLACE, with history**, in both stories, on the ~9 MB precedent: the record keeps
what it claimed, marked corrected, with the measurement that overturned it. **A disclosure that
quietly changes category is worse than one that was wrong loudly.**

**(b) The trigger is VOID and REPLACED, not renewed.** *"When browser e2e arrives (D-000.4)"* named an
event that **had already happened before the disclosure was written.** By the standing rule, **a
trigger that could never fire is deleted, not renewed.** The replacement is keyed on the real
condition: **"when CI EXECUTES the Playwright suite."**

**(c) Stories 8.4a and 8.4b are NOT re-opened.** Their acceptance criteria are **true as written**; the
false claim lived in a **Design Note**, not an AC. **Adding executed coverage is an EXTENSION, and
extensions do not join closed stories — repairs do.**

**(d) But EPIC 8 DOES NOT CLOSE without one executed browser assertion covering carried-face
rasterization**, owed at the **epic gate**, not by a re-opened story. The ground: **a `vi.fn()` has no
brand**, jsdom applies no stylesheet and implements no font loading, so **a real-browser run is the
only observer that can distinguish "we called `document.fonts.add`" from "the glyphs on the page came
from the carried face."** That is the entire subject of Story 8.4a and the one thing nothing has
checked. **One assertion, not a suite.**

**(e) THE REAL DEFECT IS BIGGER THAN THE DISCLOSURE AND IS NOT A FONT QUESTION.** Twelve executable
specs exist, run, and are **never run by CI** — **a guard that does not run**, the same class as an
unreferenced Makefile target. **Filed as DW-101 and ranked ABOVE the font residual.**

**The ordering constraint is the part that matters: wire CI to execute the suite BEFORE, or in the
same unit as, adding (d)'s assertion.** An executed assertion added to a suite CI does not run **would
execute once, locally, and never again — reproducing the exact category error being corrected, one
layer up. Do not add the observer before arranging for anyone to watch it.**

**(f) On the self-correction, and the family it belongs to.** The lead's note: the sequence — apply the
couldn't-look test to one's own claim, find it, **measure rather than infer the second time**, and
**run the thing instead of reading about it** — is the correct one, and this correction is worth more
than a clean record. It also flagged that the build's rejected finding resting on *"there is no
`test:e2e`"* is **the same family, and worth recording precisely because ITS VERDICT WAS STILL
RIGHT**: **a correct conclusion resting on a false premise is the hardest defect to find, because
nothing downstream misbehaves.**

---

### D-8.4.26 — DW-35's attribution residual is owned by a new Story 8.4e, sequenced after 8.4c

The closer was right to **narrow rather than close**, and right **not to assign an owner** — that is a
ruling. Assigned: **Story 8.4e — per-fragment shipped-face attribution on the wire. DW-35 does not
close until 8.4e does.**

**The defect is not exotic, and the measurement is what establishes it.** With an authored chain like
`["Noto Sans Thai"]` the engine measures Latin `A` with Noto Sans Thai while a **fixed Latin-first**
stack rasterizes it with Noto Sans — and **all three faces cover `A` and `5`**, with pairwise cmap
overlaps of **339 / 529 / 230**. It is the AD-17 violation 8.4b narrowed, **surviving on the
shipped-face arm.**

**Why it is small and belongs here:** Story 8.4a already built per-fragment face attribution for
**carried** faces — which is why its AC3 is over-satisfied. The residual **extends an existing
mechanism to a second population rather than inventing one**, the same shape that made 8.4b small.

**Explicitly NOT 8.5 or 8.6.** Both are **authoring** stories — which faces an author may pick, and
picking one writing the chain. **This is rasterization fidelity, and folding a fidelity fix into a
feature story is how it becomes the first thing cut.**

---

### D-8.4.27 — DW-100 gates Story 8.4d's THRESHOLD, not its start; "explained" is the bar, not "reproducible"; and 8.4d moves LAST

Three reads of `s1VisibleBytes` gave **three different figures** — 12,423,974 in a spec, 12,426,422 by
a build, **12,423,049 measured twice** by a closer at baseline *and* HEAD. The closer confirmed Story
8.4b moves it by **exactly zero, structurally**, so the conclusion was right and **both recorded
numbers were wrong.**

**(a) It is 8.4d's FIRST and GATING TASK, not a blocker outside it.** The story opens with the
reproducibility work; the threshold is written only after. **Making it a prerequisite outside the story
leaves it homeless in exactly the way DW-35's residual just was.**

**(b) REPRODUCIBLE IS NOT THE BAR — EXPLAINED IS.** The temptation is to adopt the number that
repeated twice and move on. **Do not.** The two other recorded figures came from somewhere, and
adopting today's repeating number without knowing why the others differed **produces a gate whose
all-clear is indistinguishable from a couldn't-look.** Require the **disposition of each prior
figure**, and **record the exact invocation alongside every figure from now on — a number without its
command is not a measurement.**

**(c) Establish FIRST which thing varies, because one answer is far worse.** Either **the measurement
procedure varies** (benign) or **the build output varies at one commit** — **a determinism defect, and
this product's entire premise is byte-identity.** The 12,423,167 read is attributed to an unclean
`dist`, supporting the benign reading, **but it must be SHOWN, not assumed. If the build is
nondeterministic, that finding outranks the budget gate entirely and returns to the lead.**

**(d) Story 8.4d moves to LAST in Epic 8 — the lead correcting its own sequencing a second time.** An
enforceable threshold set **before** the byte-adding stories land either **flaps** or gets **padded
with an arbitrary allowance — and padding it is the same move as rewriting "~9 MB" to "~12.4 MB",
which D-8.4.24 forbade.** **Story 8.5 ships a curated font catalogue and may add binaries.** The
threshold is set **once, against the epic's finished weight.**

**Order: `8.4c → 8.4e → 8.5 → 8.6 → 8.4d`.**

---

### D-8.4.28 — Story 8.4c's spec repaired at the gate, where it is cheap

The reversal at `6f0c095` left three passages assuming the old order. **Corrected in place at the plan
gate, kept verbatim with the correction attached** — corrections at the gate are legal and cheap; the
same corrections after implementation are neither.

1. **The `declared` constraint is INVERTED.** It said *"never add a Noto engine face name to
   `declared`"*. Story 8.4b **already added all three** — that was its subject. `declared` now holds
   **six** families and the offline release emits **6 rules over the same 3 `.ttf` files**. The live
   constraint is **do not REMOVE the engine-named half**.
2. **Task 4's file already EXISTS.** `font-binary-identity.test.ts` was created by 8.4b, so the task is
   an **edit, not a creation** — and specifically it must **update** 8.4b's assertion that
   `IBM Plex Sans` and `Noto Sans` deliberately resolve to one file, which **names this story in its
   own failure message** and was designed for exactly this edit.
3. **Task 5 is DELETED, not adapted, because its premise is false.** It planned a
   **disclosure-of-absence** — that `assets.sansCjk` is present while no `@font-face` names it. **8.4b's
   engine-named rules NAME IT.** The task even anticipated 8.4b and said *"8.4b should delete this
   assertion rather than edit it"*; under the reversed order **there is nothing to delete, because it
   was never written.** **An assertion of a negative is exactly the kind that survives as a false green
   when the negative stops holding** — struck rather than adapted, for that reason. Its I/O matrix row
   went with it.

---

### D-8.4.29 — the budget's metric does not exist in this repository; `s1.cachedBytes` is NOT the replacement

**Branch three of D-8.4.27(c) is confirmed, and it is worse than "the metric is wrong."** Measured at
`generate-offline-release.mjs`: `s1VisibleBytes` is the Brotli size of **four assets found by
HARDCODED FILENAME NEEDLES** — `.wasm`, `/noto-sans.`, `/noto-sans-thai.`, `/noto-sans-cjk.`. It is
structurally blind to IBM Plex, to JS, to CSS, to images, and to every asset that is not one of those
four.

**So the honest disposition of all four historical figures — which D-8.4.27(b) demanded — is that they
measured a thing nobody meant them to measure**, and the drift between them is engine-wasm noise. Not
a procedure that varies, and not a build that varies: a **fourth** branch the original ruling did not
have to hand.

**But DO NOT adopt `s1.cachedBytes`.** Measured: `cacheAssets[].bytes` is
`statSync(outputDir/url).size` — **uncompressed**, over all assets.

| figure | unit | population |
|---|---|---|
| `s1VisibleBytes` | **Brotli ✓** | 4 hardcoded needles **✗** |
| `s1.cachedBytes` | uncompressed **✗** | **all cached assets ✓** |

**Neither is the budget's metric, and the budget's metric exists NOWHERE in this repository.** NFR7
accepts *"~9 MB first load"* — **bytes over the wire, compressed**. Adopting `cachedBytes` would set a
budget in **uncompressed** bytes against a limit written in **transfer** terms, producing a 37.9 MB
figure against a ~9 MB line that is **not a 4× overage but a UNIT MISMATCH** — the failure where **an
uncounted unit lets an implementation detail decide the answer.**

**Verdict: the metric Story 8.4d must build is Brotli bytes over the FULL first-load cache set** —
`visibleBytes`' unit with `cachedBytes`' population. Cheap, since a `.br` is already written per asset.
**Watch the exception: that loop filters on `asset.immutable`, so non-immutable assets have no `.br` on
disk, and the new metric must define their treatment EXPLICITLY rather than skipping them silently.**

**Story 8.4d's task order: (0) the determinism check; (1) define and compute the metric; (2) dispose of
the four historical figures against it; (3) only then set the threshold.** 8.4d still lands **last**.

**Task 0 discharged during Story 8.4c's close, ahead of the ruling, and it closes BENIGN — by
measurement.** Two clean builds at `4f5925a` are **byte-identical**, so it is **not** Brotli
nondeterminism. The wasm carries `vcs.revision`, `vcs.time` and `vcs.modified`: `go build` runs with
`-buildvcs` at default. Proved at one commit with source fixed — a clean tree gives one digest;
touching one tracked file gives another with `vcs.modified=true`. **The engine row is 58% of
`s1VisibleBytes`, so the figure moves EVERY COMMIT AND ON TREE CLEANLINESS, by construction.** Fix:
`-buildvcs=false`.

---

### D-8.4.30 — the guard's commit-placement clause is DISCHARGED, and the lead recorded the clause as its own proxy defect

D-8.4.23 required the licence guard in the **first** commit *"because a guard added after the thing it
guards has already shipped is one that was never able to fail."* It landed in the **third**.

**Discharged.** That reason is a claim about **demonstrated fallibility**, and **commit ordering is
only a PROXY for it**: ordering creates an *opportunity* for the guard to fail; a red-proof **shows**
it failing on the real condition. **The red-proof is the stronger evidence, so it does not merely
substitute — it is what should have been asked for.** The lead's own words: *"I have ruled repeatedly
that a guard keyed on a proxy rather than its purpose is a defect; I wrote one into my own ruling."*
The exposure window is **zero** — nothing pushed, no release between commits — and the closer measured
it empty of the hazard besides (all three binaries `.ttf` by magic bytes).

**Correction of record:** the build routed the omission as `patch`; the closer judged **`bad_spec` was
accurate**, since the omission was in the spec's Tasks. The `patch` routing was still the right
**trade** — reverting correct work for a purely additive guard is worse — but the classification was
wrong, and those are different questions.

**One residual recorded rather than reopened: the guard is PLACE-KEYED to `public/fonts/`.** As built
it is **broader** than the ruling's wording (*"reaching the runtime bundle"*) — correctly, since
`ResolveAssets` walks the whole repo and an unseeable font **anywhere** is the defect. **But a font
vendored elsewhere — into `src/`, or a package's assets copied to another path — is still invisible.
The blind spot has MOVED rather than closed. A place-keyed guard's anchor is one the code can move.**

---

### D-8.4.31 — "a ruling recorded in every document except the one that gets read is a ruling that did not happen"

D-8.4.23 was amended into the **epic** and the **decision log** and **not into Story 8.4c's spec
Tasks** — the only artifact a builder executes. Neither implementation commit carried the guard.

**The asymmetry is the point.** The epic and this log are **records**; a builder does not execute a
record. **A ruling amended into both and not into Tasks is STRICTLY WORSE than one amended into
neither, because the log now testifies that the direction was given.**

**Third artifact-drift failure of this run, and the first with the ruling in hand** — which is what
makes it **structural rather than a lapse**, and it is recorded that way.

**The lead has adopted a process change from it, unprompted:** every ruling of its carrying an
**implementable clause** will end with a **`Carried into:` line naming the artifact that must carry
it** — the spec's Tasks. *"The routing obligation belongs in the ruling, not in your memory of it,
since I am the one who created the clause."* **The orchestrator is to hold it to that and say so if it
is omitted.**

---

### D-8.4.32 — a claim in prose is a claim, whoever wrote it: third instance, third artifact

The orchestrator's patch instruction *"assert IBM Plex Sans covers U+0020–U+017F with no gaps"* was
**copied from the face's NOTICE** and is **false on its face**: U+007F–U+009F are **33 C0/C1 control
points no text face maps**. **The patch agent said so rather than complying** — the loop working in the
direction it is supposed to.

**Third instance of one class, in three different artifacts:** a number read out of a **doc comment**
and carried forward as measured (D-8.4.7); an asset's prose **`source` field**, which the lead insisted
on **hashing rather than reading** (D-8.4.8); and now a **vendor's NOTICE**. **A coverage claim in a
NOTICE is a vendor's description, not a measurement.**

**The method that refuted the largest rejected finding is the one to keep:** the claim that
`--font-mono` losing CJK regresses rendering was refuted by reading **every** `--type-page-mono`
consumer — **an inverse census beats a plausible story.** But the closer then **re-opened it one level
up (DW-102)**: the rejection is airtight at `--type-page-mono`, while the claim was about
`--font-mono`, which **seven** tokens share — and **IBM Plex Mono maps 0 of 20,992 CJK code points
where the file it replaced mapped 20,976**, reaching the document title and the author's prose field.
**A census is only as good as the population it enumerates.**

---

### D-8.4.33 — the 428 KB stub is the physical form of a couldn't-look

The Playwright browser cache held an entry named `chromium-1208` that **looked present and was not a
working install** — 428 KB against 192 MB for a complete one, its Framework bundle absent. **Every
inference from "the cache has Chromium" was therefore wrong, and running the thing was the only way to
find out.**

Its reinstallation then hung repeatedly on a **stale `__dirlock`** left by killed processes — a lock
held by nobody, which no amount of retrying could pass and which reported nothing until Playwright
itself named the file.

**Expectation set for the first execution: a suite that has never run is expected to surface
pre-existing breakage, and that breakage is a FINDING, not a blocker on this epic.**

---

### D-8.4.34 — DW-86 was never a standing red; the defect was in the REGISTRATION, and the interval is covered by an invariant proven with git

**The orchestrator registered DW-86 at Story 8.3's close on the stated ground that the pinned
toolchain and `.font-sources/` were "gitignored and absent, so registration is the practical path."
Both are present** — `.fontgen-venv/bin/python` is **Python 3.12.13 with fontTools 4.63.0**, and all
three upstream variable fonts are in `.font-sources/`. `go test` invokes a bare `python3`. **With
`FOLIO_FONTGEN_PYTHON` pointed at the venv the test PASSES, non-vacuously** — *"derived and compared
3 of 3 faces"*, real digests, and the Thai value **independently matching** D-8.4.8's hash.

D-8.3.4 offered two dispositions — *make fontTools available to the gate, or register the red* — and
**the second was taken because the first was believed impossible.**

**(a) THE TEST IS LARGELY INNOCENT, and the lead's correction of the diagnosis is the more useful
finding.** `fontgen_matrix_test.go` says in its own words *"IT DOES NOT SKIP WHEN ITS INPUTS ARE
ABSENT"* and carries **five distinct `t.Fatalf` sites**. **It already refuses to be quiet and already
distinguishes its causes in its messages.** The three states were conflated **by the register entry**,
which recorded *"pre-existing and environmental"* — **a guess at the cause** — rather than the
assertion that fired. **Quoting the message would have exposed the wrong interpreter on the day the
entry was written**, because the message that fired was not the sources-absent one.

> **STANDING RULE: a standing red is registered by its failing assertion's message, VERBATIM — never
> by a category.**

**(b) Auto-prefer the venv. Ruled yes, on a property specific to this test: IT CANNOT GO GREEN BY
FINDING LESS.** Green requires sources **and** a working interpreter **and** three matching hashes
**and** the `3 of 3` witness; every other state is a loud, distinct red. **Auto-preference cannot
manufacture a pass — only stop suppressing a real run.** The orchestrator's worry — *"green because
something untracked happened to be on this machine"* — was the right worry and does not apply here.
**The alternative has a measured failure record: explicit invocation is what was in place, and the
check was dark for the entire run. An obligation discharged only when someone remembers is not an
obligation.**

Two conditions: preference order `FOLIO_FONTGEN_PYTHON` → `.fontgen-venv/bin/python` (existing **and**
importing fontTools) → `python3`, so an explicit override still wins; and **the interpreter failure
must name its own remedy in its message text.** *"The whole cost of this incident was that the next
person could not tell a wrong interpreter from absent sources at a glance; the message is where that
is fixed, not the register."*
**Carried into: the spec Tasks of whichever story takes DW-103 — not the epic, not this log.**

**(c) CI: state the invariant, not the mechanism — a change to `folio-go/fonts/` must not be able to
merge without this check having run.** Its inputs are gitignored and large, and the artifact has
changed **twice in the project's life**, so provisioning on every run is disproportionate; leaving it
unrun is how it went dark. A **path-filtered** job is proportionate. **Filed WITH DW-101 as ONE policy
pass (DW-103): they are the same defect wearing different clothes, and fixing them separately produces
two answers to one question.**

**(d) The interval IS covered — and NOT by the reasoning that was right to distrust.** *"It passes
now, therefore the interval was fine"* is invalid and this run keeps rejecting it. The sound argument
was **measured, not asserted**:

```
git log --oneline -- folio-go/fonts/
4b797d4  Correct the Noto Sans SC provenance; record D-000.56
3373dac  Story 2.2: The shipped font set and its fallback chain (finisher)
```

**Two commits, ever**, both long before DW-86 was registered. So the argument is **"the artifact under
test never changed, and it is correct now, therefore it was correct at every commit back to
`4b797d4`"** — an argument from an invariant **proven by git rather than assumed**. **Had git shown a
single change in that window the answer would have been NO** — and that discriminator is what makes it
a measurement rather than a rationalisation. It also **reaches further than asked: today's pass is the
first known-good execution and retroactively verifies every commit from `4b797d4` to HEAD.**

**Story 8.4c did not touch this population** — its binaries went to `folio-designer/public/fonts/`,
while this check derives three **Noto** faces from `.font-sources/*-VF.ttf`. **Residual recorded: the
check covers 3 of the 6 shipped faces.** The IBM Plex three are pinned npm static files, so their
guarantee is **provenance, not reproduction** — **two kinds of assurance under one heading**, and
nobody should later read *"byte-identity of the shipped fonts is gated"* as meaning all six.

**(e) The dispatch formula changes: "exactly two standing reds" is a COUNT, and a count is a lossy
set.** It cannot see **one of the two changing its cause** — which is precisely what happened.
**Dispatch with the reds' IDENTITIES: the test name AND the failing assertion.** This is the same
defect as `declared` having a floor and no ceiling, **one level up, in the process rather than the
code.**

**(f) The three-state formulation, adopted by the lead as the run's:** **an all-clear must differ from
a couldn't-look, and a couldn't-look must differ from a LOOKED-IN-THE-WRONG-PLACE.** The third is the
state this run keeps finding — **the 428 KB Chromium stub, the bare `python3`, and
`s1VisibleBytes`' four-needle total. All three are instruments pointed somewhere other than where
their reader believed.**

---

### D-8.4.35 — three findings from Story 8.4e's close, and one unresolved environment fact recorded rather than hidden

**(a) `s1VisibleBytes` moved 656 bytes between two builds the closer reports as being AT THE SAME
COMMIT** — 12,427,899 (build dispatch) against 12,428,555 (close), with `cachedBytes` 1 byte apart.
**D-8.4.29 concluded the determinism question closed benign** on the ground that the wasm carries
`vcs.revision` / `vcs.time` / `vcs.modified` and therefore moves **every commit and on tree
cleanliness** — which explains movement across *different* HEADs. **It does not obviously explain
movement at one commit with a clean tree.** Either the two builds were not in fact at the same commit
and cleanliness, or D-8.4.29's disposition is incomplete.

**This is Story 8.4d's task 0 and it is NOT closed.** Recorded here so that ruling is not read as
settled: *"build twice at one commit and `sha256` the wasm"* must be done **with the tree state
recorded alongside each build**, because tree cleanliness is now known to be an input. **A figure
quoted with its command is not enough if the command's environment is also an input** — the
invocation must include the commit and the tree state.

**(b) The rejection audit does not reconcile: 23 routes against a declared population of 25.** Story
8.4e's build reported 16 rejections over a deduplicated population of 25 claims from four review
layers; the closer spot-checked four as sound at their citations, and found the routes sum to **23**.
**Two claims have no recorded route, and the closer says plainly it could not check them** rather than
implying it had. **This is DW-87's shape one level deeper:** DW-87 was that rejections were counted
and not enumerated; this is that the enumeration itself **does not account for its own population**.
**A census is only as good as the population it enumerates** — and here the census and the population
disagree by two.

**(c) A guard's own stated count had gone false, and the closer found it by re-running the count.**
`TestCanvasIdentifierBoundsStillRefuseAtFiveHundredAndTwelve` declares **"ALL EIGHT"** sites while its
own grep now reports **nine** — this story added the ninth. Corrected in place (the ninth is
unprobeable, so no probe can exist) and filed as **DW-104**. **The `maxCanvasPropertyString` probe
list is hardcoded with its count only in prose, and has now missed a new site TWICE.** Same family as
the `declared` floor-without-ceiling and the `withAssets != 7` fixture count: **a number in prose that
no code derives.**

**(d) The Playwright browser install did not complete, and that is recorded rather than quietly
dropped.** The 162 MB Chrome for Testing archive downloads to 100% and extraction never finishes —
the cache entry stays at **428 KB** with only `chrome-mac-arm64/ABOUT` present, across repeated
attempts, some of which were interrupted by this orchestrator's own kills and a stale `__dirlock`.
**So the 12 Playwright specs still have not executed**, and the executed-browser assertion Epic 8 owes
(**D-8.4.25d**) remains **owed, not attempted-and-passed**.

**What IS established and does not depend on the install:** the specs exist, the config is real, the
harness runs (it built, started the preview server and executed the spec bodies), and **CI runs only
`tsc --noEmit`** — which is DW-101, unchanged. **What is NOT established is anything the browser would
have told us.** Recorded in these words so no later reader takes "we tried" for "we looked."

---

### D-8.5.1 — the 64-asset ceiling splits: the SILENT-FAILURE half lands ahead of 8.5 (Story 8.4f), the design fork lives inside it — and reading the contract dissolved one option

**The option set was wrong, and reading the contract rather than the summary fixed it.** The **64
bound** and the **5-row pinning** are **two constraints on two different things**:

- `release-payload.ts` bounds **`assetCount`** — `candidate.assetCount > maximumCacheAssets → return
  undefined`, where `assetCount` must equal `release.assets.length`, i.e. **every asset in the
  release**.
- `verify-offline-release.mjs` pins the **rows** to exactly five ids by **ordered exact equality**,
  `cachedRows.length === 4`, `rows[4]` positionally, and `cjk-font` as the `Math.max` of the font rows.

**So "don't give catalogue faces S1 rows" is NOT an option — it is already FORCED by the row
pinning — and it does NOTHING for the ceiling, which counts ASSETS, not rows.** Any catalogue face
shipped as a build asset counts toward 64 whether or not it is displayed. **The third option's real
content is therefore much larger than its label: "catalogue faces are not build assets at all."**

**(a) Ahead of 8.5, and separable: crossing the ceiling must FAIL THE BUILD — Story 8.4f.**
`parseS1Payload` returns `undefined` over the bound, and its caller wraps the read in
`try { … } catch { return undefined }` — **soft twice over** — so the **first screen a user sees loses
its payload with nothing said.** **No build script catches it; runtime "catches" it by being quiet.**
`verify-offline-release.mjs` is the natural home: it already validates this payload and already carries
red-proofs (`s1-total-mismatch`, `s1-delivery-fiction`), so **the assertion and its red-proof both have
a pattern to follow.** **A bound whose only enforcement is a silent parse failure is not a bound.**

**It outranks the catalogue's contents — but not for the reason either the orchestrator or the lead
first gave. It outranks because its failure is SILENT**, which makes it reachable by **any** future
asset addition, not just this story. That is why it must not be gated on the catalogue's design
settling.
**Carried into: Story 8.4f's own Tasks — not 8.5's.**

**(b) The design fork lives INSIDE 8.5, because it IS the story's design.** Putting it ahead would
settle the story's core shape outside the story.

**The steer, and why the direction supports it:** the owner chose 20+ families **with the ceiling and
the 37% overage stated in the question.** Reading that as *"accept a much heavier bundle"* is possible;
reading it as ***"the catalogue should not be a bundle problem at all"*** is better supported —
**because this epic already built the machinery for the alternative: 8.3 made a face travel inside a
document, 8.4 made it draw, and 8.6 is "picking a family puts it in the file."** A face **fetched at
pick time and embedded as a carried asset** costs **zero build assets, zero S1 rows, zero first-load
bytes.**

**Two things 8.5 must ESTABLISH rather than assume on that route:** what a picked family does
**offline** — NFR7 promises the *designer* works offline, and a catalogue is a **palette, not
coverage** (the three shipped Noto faces are the coverage), so this is likely fine **but must be stated
and tested, not inferred**; and that a pick-time fetch **does not violate AC3**, a real tension to
**resolve explicitly rather than discover.**

**On raising `maximumCacheAssets` if the bundled route is taken:** read in context it sits beside
`minimumCacheAssets = 10` and `assetUrl.length <= 256` inside a validator for JSON parsed out of
`index.html` — **a defensive SHAPE bound, not a semantic capacity limit.** Raising it is **legitimate
and not a compromise**, on two conditions: **stated headroom** rather than tuning to fit the new count,
and **only after Story 8.4f lands**, so the bound can never again be crossed silently. **A bound tuned
to the current population is one the code can move.**

---

### D-8.5.2 — the unresolvable licence case is settled by existing direction, whichever way the owner ruled

`licenceLabel = "SEE NOTICE"` on a licence text the classifier cannot identify is the **asset-side form
of the silent pass D-1.3.8 forbids on the dependency side.** `licencegraph.go` records the ground
verbatim: *"a silent pass on an unidentifiable licence is the realistic failure mode"* — and it fails
the build on unresolvable, with the refinement that a **commercial EULA has no marker table and
deliberately falls through to unresolvable** rather than through an unreachable case.

**That ground is licence-family-NEUTRAL.** It is not an argument about which licences are acceptable;
it is an argument that **not knowing must not read as fine.** It transfers to assets unchanged.
**A font asset whose `LICENSE*` text cannot be classified to an SPDX identifier FAILS THE BUILD.
`"SEE NOTICE"` stops being a pass.** Settled before the owner answered, and pre-empting none of their
options. **Carried into: Story 8.5's spec Tasks.**

**And the extension-class guard goes REPO-WIDE before 8.5 ships a font anywhere new.** Confirmed by
measurement: `ResolveAssets` walks the **whole repo**, while the extension-class guard walks
**`folio-designer/public/fonts` and nothing else** — so **a `.woff2` in a new directory is invisible to
BOTH**: the licence gate cannot see the extension, and the extension guard cannot see the directory.
**A catalogue is the feature that introduces new directories**, so this is a **precondition, not a
latent hazard.** Making the guard's population equal `ResolveAssets`' own walk is **repairing a guard
whose scope was always meant to match the checker it protects** (D-8.4.30).

---

### D-8.5.3 — OWNER DECISION: a permissive allowlist, enforced like the dependency ban; and 20+ families

**The licence gate does not check licences, and this run repeatedly overstated it.** `ResolveAssets`
requires a `LICENSE*`, a `NOTICE*` and a `Copyright` line beside every committed font, then records the
classified licence **as a LABEL** with `"SEE NOTICE"` as fallback **and returns nil**. **A GPL font
gets a manifest row and a clean build.**

**It is FAITHFUL TO AD-26, whose heading is not its Rule.** The heading says *"nothing copyleft
enters"*; the Rule has **two clauses** — a family ban scoped to **"No dependency"**, enforced on the
whole module graph and the lockfile; and for **redistributed non-code assets**, only that they *"keep
their own terms and their notices"* and *"travel with their licence text and copyright lines."*
**Two clauses, two obligations.** Extending the ban to assets is therefore **a decision, not a bug
fix.**

**Recorded in the orchestrator's words at the lead's request: Story 8.4c's guard and its manifest work
were both about whether the gate can SEE a font. Neither was about whether the licence is acceptable.
Every artifact in this run that called `ResolveAssets` "the licence boundary" — the lead's included —
overstated it.**

**Why it was the owner's and not engineering's: FONTS DO NOT LINK.** AD-26's stated Prevents is about
**static linking attaching obligations to Folio's binary.** Folio **embeds and subsets a font program
into the USER'S PDF**, so an asset's licence attaches to **documents the users produce**, not to Folio.
That is a product and legal call. And `spec-fonts/SPEC.md`'s Open Questions carries the curation half
**deliberately unmarked**, with the question directly above it **struck through as SETTLED** — the
absence of a mark is a signal, not an oversight.

**DECISION 1 — a named permissive allowlist: OFL-1.1, Apache-2.0, MIT, UFL**, enforced the way the
dependency ban is — **fail the build, never warn** — with **unclassifiable treated as failure** per
D-8.5.2. The lead's recommendation, taken.

**DECISION 2 — the BROADER catalogue, 20+ families**, **against** the lead's recommendation of a small
5–10 family MVP set, **with the ceiling and the 37% overage stated in the question.**

**The lead's reading of that, and it sharpens the brief rather than complicating it:** *"the honest
reading is not that they discounted the constraints but that they want the catalogue to be big and
expect the engineering to find a shape where size is not paid for at first load. That is a better brief
than the one I would have written."* It is also what makes D-8.5.1(b)'s fetch-at-pick steer the one to
take.

---

### D-8.5.4 — AC1 claimed a derivation that does not exist; rewritten per procurement route

**The orchestrator's dispatch premise — "the replayable derivation exists and WORKS" — was true of the
existing three faces only.** `instance_faces.py` drives a **hardcoded 3-entry `UPSTREAM` list**;
`.font-sources/` is **gitignored with zero tracked files**; **nothing in the repo fetches an upstream
source** — no script, no CI step, no Make target; and the bootstrap **demands an `out_sha256`
unknowable until after the first run.** **So AC1 was not executable for ANY new family, whichever
families were chosen.**

**The pattern, named by the orchestrator and sharpened by the lead: a mechanism proven over its
existing population is not proven over an extension of it.** The lead's addition is the part worth
keeping: **both times this recurred today the observation was TRUE** — CI does run only the compile,
and the derivation does pass for its three faces. **The defect was never the observation; it was the
QUANTIFIER silently attached to it.** That is harder to catch than a wrong measurement, **because
nothing downstream contradicts it.**

**Ruled: reproduction is required only of a face this project DERIVES; provenance is sufficient for a
vendored static face** — the standard Story 8.4c already shipped IBM Plex on, so extending it is
**consistent with what already shipped, not a new concession.** AC1 states the standard **per
procurement route**, and 8.5 **chooses a route per face and says which.** **What must not happen is an
acceptance criterion claiming a derivation that does not exist.**

**Regardless of route: replace `"3 of 3 faces"` with `len(UPSTREAM)`**, so adding a face fails **on a
byte divergence or not at all — never on a string.** **Third hardcoded count this epic**, after
`declared`'s floor with no ceiling and the probe list that has missed a new site twice. **Three is a
pattern: a count written next to the thing it counts is a literal that stops being true the moment the
thing grows.** Do it **even if 8.5 derives nothing**, because it is what makes the **next** face's
failure legible.

---

### D-8.5.5 — AC3 gets an observable layer now and does NOT block on DW-101

**Measured: there is NO forbidden-host scan anywhere** — zero hits for `gstatic|googleapis|fonts.google`
in any scanned source population. **The only literal "no request leaves the machine" proof is
Playwright** (`e2e/engine-worker.spec.ts` calls `context.setOffline(true)`), **and CI contains zero
occurrences of `playwright`.** So AC3's proof lived entirely in the suite CI does not run — **DW-101
and DW-103 are not adjacent to this story, they were load-bearing for one of its acceptance
criteria.**

**But 8.5 must not be blocked on a CI policy fix.** Ruled: **8.5 builds the forbidden-host source
scan** on the model the gate identified — `canvas-authority-contract.test.ts`'s **comment-stripping
character scanner**, **red-proved in both directions** — cheap, in-repo, and running in CI **today**.

**AC3 is phrased against what that scan can observe and must not claim more.** A source scan proves
**no forbidden host appears in the scanned population**; it does **not** prove no request leaves.
**Writing the stronger claim over the weaker instrument is the exact failure this run keeps finding.**
And **the scan must strip comments** — Story 8.4b measured a chrome-token guard staying green over a
live token parked in a CSS comment, so that is a demonstrated failure mode, not a hypothetical.

**The Playwright offline proof stays at the EPIC 8 GATE**, as the executed browser assertion already
owed under D-8.4.25(d).

---

### D-8.5.6 — AC4 computes per-asset Brotli and 8.4d consumes it; do not build it twice

**The obvious place to record the catalogue's weight would produce a FALSE ZERO.** `s1.cacheAssets[]`
carries `{assetUrl, bytes}` per asset — **uncompressed**. Per-asset Brotli exists **only as `.br` files
on disk and is recorded nowhere.** And `s1VisibleBytes` sums four hardcoded needles and **already
misses 174,949 Brotli bytes of IBM Plex** — so recording against it yields **a number that reads as
"this cost nothing."**

**8.5 computes and records per-asset Brotli** — nearly free, since `generate-offline-release.mjs`
already writes a `.br` per **immutable** asset — **and 8.4d CONSUMES that same figure** for the metric
ruled at D-8.4.29 (Brotli over the full first-load set) **instead of building the same thing twice.**
**Mind the same exception:** that loop filters `asset.immutable`, so non-immutable assets have **no
`.br`** and need an **explicit treatment rather than a silent skip.**

**If 8.5 takes the fetch-at-pick route, AC4's subject becomes "the catalogue adds ZERO first-load
bytes" — and that is then a CLAIM THAT MUST BE ASSERTED, not a happy consequence, precisely because a
true zero and a false zero look identical in a report.** `s1VisibleBytes` missing 174,949 bytes of IBM
Plex is exactly that demonstration.

---

### D-8.5.7 — the determinism question is SETTLED BY MEASUREMENT: the build is deterministic, the variable was tree state, and the pipeline was perturbing its own specimen

**Raised at D-8.4.27(c), closed benign at D-8.4.29, reopened at D-8.4.35(a), reproduced at Story
8.4f's plan gate (2,203 bytes at one sha on a clean tree). Now settled — and settled by running the
discriminator rather than by adopting the plausible explanation.**

**`vcs.time` REFUTED ON ITS PREMISE, not merely left unconfirmed.** The orchestrator proposed it: two
builds minutes apart at one commit would differ, and the earlier byte-identical pair was built inside
the same second. **`vcs.time` is the timestamp of the REVISION, not of the build** — Go stamps the
commit's own time — so two builds at one commit carry the **identical** value however far apart they
run. **Both halves of the hypothesis are false, and the disposition that closed D-8.4.29 cannot be
repaired by leaning on it harder.**

**The discriminator, run at `92cd590` by the orchestrator:**

| probe | tree state | wasm sha256 |
|---|---|---|
| build 1 | `git status --porcelain` **empty** | `ed260565…fb8f8` |
| build 2 | **empty**, compared to build 1's output | **`ed260565…fb8f8` — IDENTICAL** |
| build 3 | **one stray UNTRACKED file** | `a13ab262…3b6c1` — **DIFFERS**, `vcs.modified=true` |

**Two builds at one commit with provably identical tree state produce a byte-identical wasm. One
untracked file changes it.** So: **THE BUILD IS DETERMINISTIC. The variance was TREE STATE.**

`go build` defaults to `-buildvcs`, stamping `vcs.revision` / `vcs.time` / `vcs.modified`; Go derives
`vcs.modified` from `git status`, where **an untracked file is enough**. A ~4-byte input change
produces an **arbitrary** Brotli delta, because compressed size is **not continuous in input size** —
so *"2,203 bytes is too big to be a build stamp"* was never an argument, and the lead said so before
the measurement rather than after.

**THE COROLLARY IS THE SHARP PART: THIS PIPELINE WRITES UNTRACKED FILES INTO THE TREE** — halt files,
result files, spec files. **A run that writes an artifact between two measurements changes the stamp it
is measuring. The instrument perturbs the specimen.** That is precisely why Story 8.4c's pair at
`4f5925a` agreed and Story 8.4f's at `548aa29` did not — **the difference between the two situations,
which is what had to be found.**

**It reopens NOTHING, and the ground matters more than the verdict.** **The wasm binary's bytes and
the PDF's bytes are different artifacts.** AD-21 binds byte-identity of **rendered output** across four
targets, **not** of the compiled binary. And this is **evidence, not assumption**: **every golden digest
held across all four targets at every story this epic, and `TestCrossTargetByteIdentity` passed
throughout** — a compiler-level nondeterminism reaching codegen **would have moved a golden.**
**Recorded in the ruled terms: "a number was noisy", NOT "this product's premise is false."**

**The fix moves to NOW, as Story 8.4g, ahead of Story 8.4f's build.** The lead's own placement as
8.4d's task 0 *"was right while it was a question and went stale when it became a defect"* — the shape
this run keeps finding, caught on itself. **The decisive ground is accrual, not urgency: every story
recording a byte figure before this lands adds another figure to the pile 8.4d must dispose of, and
there are already four.**

**The unit is measure → `-buildvcs=false` → RE-MEASURE.** The re-measurement is **not optional**: *a
fix for a determinism defect that is not shown to have closed it is the same category error as a guard
whose red-proof was allowed to be commit ordering.* **And the provenance loss is recorded as a
deliberate trade, not a free win** — acceptable because the release manifest already carries
`releaseId` and `pageId` derived from asset hashes and AD-22 pins the toolchain, **but stated.**

---

### D-8.5.8 — Story 8.4f's three findings, ratified, and the first premise-clean dispatch of the run

**(a) REMOVING A SOFT `catch` IS NOT AUTOMATICALLY A HARDENING — put this in the run's practice.** The
gate ruled AC4 rather than halting on it, **measured from control flow rather than argued**: today an
unparseable bootstrap still reaches `registerOfflineLifecycle`, publishes `'unavailable'`, and gives
the user *"Offline cache unavailable"* **and a Retry button**. Remove the catch and that call is
**never reached** — the screen sits on *"Checking cache"* forever, **no message, no retry**. **It trades
a stated failure for an unstated hang.** The shape is: narrow to `SyntaxError`, rethrow the rest, name
every rejection reason — **and a Design Note stating what it does and does not change FOR A USER is the
instrument that lets a reviewer judge rather than discover.**

**(b) The red-proof placement constraint is CORRECTNESS, and this is the first time this run the
vacuous-green family was caught BEFORE close.** Any mutation adding manifest assets trips `sameSet`,
the digest loop or the Brotli loop **first**, so a bound check placed after them **could not be
red-proved on its own message** — it would satisfy a bare *"something failed"* harness **while proving
nothing**. Making *"the proof keeps failing on an older guard"* a **Block If** rather than something to
soften the assertion around is the correct instinct: **when a proof will not fail for the right reason,
you move the proof, never weaken the claim.**

**(c) The Node/TypeScript constant sharing holds because it rests on a measured fact.** The verifier is
Node, the bound is declared in TypeScript, and the sharing is one-directional by construction. The
tempting duplicate-plus-tie-test fails because **`npm run build` does not run Vitest** — so a drifted
copy would **ship green through the very gate this story adds**, and the tie-test would be **a guard
the shipping path never runs**. Deriving the number from the single declaration, failing loudly when
exactly one live copy cannot be found, is **a guard anchored where the code cannot move it**.

**(d) Story 8.4f is the FIRST dispatch of this run where every handed premise came back true, and that
is recorded as attributable rather than assumed.** It follows the run of premise failures that produced
the habit of measuring before asserting — and it is worth noting that the two premise errors of this
session that recurred were both cases where **the observation was true and the QUANTIFIER attached to
it was not**, which is harder to catch because nothing downstream contradicts it.

---

### D-8.5.9 — the step-03 commit breach reaches INSTANCE THREE, and the recovery holding a third time is the argument for pricing it

Story 8.4f's step-03 subagent **committed `7a18079` on its own.** Step-03 does not authorize a commit;
finalizing is step-04's. The content was clean, the trailer right, nothing pushed, no protected path
touched — so the builder **appended rather than amended**, per Finalize's keep-commits-already-created
rule. That was the right trade.

**This is instance three** (after D-8.4.9c and D-8.4.18), and the standing note recorded at instance
one was that **re-measurement is a recovery, not a repeatable guarantee.**

**The recovery has now held three times, and that is precisely the argument for pricing it rather than
absorbing it again.** Three unauthorized commits, three catches by a downstream agent that happened to
audit provenance. **A defect whose only defence is that somebody downstream keeps noticing is not
defended — it is lucky.** Per the standing rule that an Nth instance **re-prices** a deferral rather
than renewing it, *"we caught it again"* has stopped being a disposition.

**It remains outside this repository to fix** — the step ordering lives in the `bmad-build-auto` skill,
not in `folio` — so it is recorded here **with its count**, so a fourth instance is priced against
three priors rather than met fresh. **The concrete cost if it recurs unnoticed is now visible:** at
instance three the commit landed while `origin/main` was live, so an unaudited step-03 commit is one
push away from being public.

---

### D-8.5.10 — AC3 was AMENDED, and the closer distinguished which of two opposing rules applied

The orchestrator put a question rather than a ruling: **AC3 required the bound check red-proved on its
own message**, but deleting the guard reddened the proof on `sameSet` instead. Two of this run's rules
point opposite ways, and **choosing between them was the whole question**:

- **D-8.4.24 / D-8.4.29:** moving a threshold to match a measurement is the defect, not the fix — and
  an AC rewritten to match an implementation is that shape **at the acceptance layer**.
- **D-8.4.2:** an AC that contradicts an invariant is **an error to CORRECT, not a fork to route.**

**The closer ruled D-8.4.2, and corrected the orchestrator's mechanism while doing it.** The
orchestrator's provisional reading was that *"an earlier guard legitimately catches the same mutation
first"* — **false.** **No earlier guard preempts anything: the bound guard IS the earliest.** An
impossible-needle probe trips it with its own message — `release carries 65 cache assets, over the
declared maximum of 64`. **Deletion reddens on `sameSet` because the mutation is OVERDETERMINED — 65
assets violate two independent invariants at once** — not because of ordering.

**And "unsatisfiable" overstated it.** The literal string **is** reachable via the regenerate shape, at
a **re-measured** cost (`build:offline` 45.73s × 2 ≈ +91s on a 183.33s red run). So: **the BAR is met
and unmoved; the false premise was about BLAST RADIUS**, and the spec's own Design Note 3 refutes it in
the same document. **AC3's second clause amended in place, loudly, with the original preserved
verbatim** — which is the form this run has required of every correction since the ~9 MB precedent.

**The distinction worth keeping: an AC is moved illegitimately when the BAR changes, and legitimately
when a FALSE PREMISE about how the bar is reached is corrected.** Those look identical in a diff and
are opposite in kind.

---

### D-8.5.11 — a guard NARROWED rather than closed, and the closer corrected the build's account of it

Story 8.4f patched two guards caught not failing for the reason they name. **One closed; one did
not**, and the closer said so rather than accepting the build's report.

**`main.tsx`'s payload mapping and dev-bypass narrowing are executed by NOTHING** — no test imports
`main.tsx`, and Playwright is compile-only. **Both predicates now have teeth** (neutering the parser
bound reddens 5 named tests; re-widening the bypass reddens the one that names it) — **but the call
sites are still run by nothing.** The faithful mutation passes **typecheck, 40 files / 409 tests, lint,
e2e-compile, build AND `verify:offline:red`.** Filed as **DW-110**.

**The detail that makes it instructive:** the *naive* mutation fails `tsc` on **TS6133 — an orphaned
import.** **That is a compile error, not a test**, and it is exactly the kind of failure that reads as
coverage while proving nothing about behaviour. A reviewer seeing red there would reasonably conclude
the path was guarded.

**The DW-106 probe closed properly**: hiding the probe via `.git/info/exclude` makes
`verify:offline:wasm` exit 1 naming it invisible — **the probe's visibility to git IS the
perturbation**, and the code now says so. Exclude restored byte-identically.

**And the most-corroborated finding of the story was rejected soundly and STILL filed.** The
reason-drop, raised by **three** review layers, is scope-correct to reject — AC4 is at the
function-return layer, the Block If forbids the product change, zero non-test `console.*` re-counted in
`src/` and `scripts/`, and the dev bypass is a real consumer of the reason — **and the observation
remains true**, so it is **DW-111** rather than a line in a rejected list. **A sound rejection is not a
refutation of the observation.**

**One audit limit stated rather than implied:** rejections **21–24** are a single grouped line carrying
**no claim and no location**, so they are **unauditable** and were not checked. **DW-87 stays open.**
