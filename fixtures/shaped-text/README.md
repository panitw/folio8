# `fixtures/shaped-text/` — the shaping golden

Recorded at **Story 2.3** ("Shape Latin, Thai and CJK text"). This is a **new** fixture, added
beside the four that already existed and replacing none of them (the D-1.8.6 / D-2.2-D5
precedent: add a document, never switch one).

| file | what it is |
|---|---|
| `input.folio` | the source document. Kept **byte-identical** to `folio-go/shapedTextTemplateJSON` (`render_test.go`); `TestShapedTextGoldenFixture` fails if they drift. |
| `expected.json` | the **normative** hash, plus the recorded `folio8GoVersion` and `goToolchain`. |
| `expected.pdf` | the rendered document, for **human diffing only**. Its own SHA-256 is asserted to equal `expected.json`'s, so the two halves cannot drift apart. |
| `harfbuzz-oracle.json` | the independent cross-validation oracle — see below. |

## Why this fixture exists at all

**Every golden fixture that existed before Story 2.3 was blind to shaping.** Measured at story
creation: their text shapes to itself, so a correct shaping implementation and one that never
calls the shaper produce identical bytes. (`font-text` turned out to be a partial exception —
see its own README — but `minimal-rect`, `image-embed` and `multi-script-fallback` genuinely are
blind.)

That means the pre-existing fixtures **cannot confirm this story landed**. If the shaping seam
were written and then never called, every one of their hashes would stay green. The "all clear"
and the "I could not look" are the same value (D-000.9).

So this fixture's text is chosen specifically so that **a wrong answer is a different hash**, and
that property is itself asserted rather than assumed:

- `TestShapedTextFixtureIsShapeObservable` measures every one of this document's face-segments
  against the naive rune→`cmap` answer the renderer used before this story, and **fails loudly**
  if Thai or Latin has zero shape-observable segments. It prints a coverage witness.
- `TestShapedTextFixtureObservabilityRedProof` runs the same measurement against
  `fixtures/multi-script-fallback/` and requires it to report **zero** observable segments — the
  red-proof that gives the green result above its meaning.

## What each element makes observable

| id | text | what it exercises |
|---|---|---|
| `e1` | `ปั ฟั ที่ ป้ำ` | Thai tall consonants with marks. `GSUB` selects a **lowered** mark form; the naive answer collides with the ascender. **This is the defect the story exists to fix.** |
| `e2` | `ณัฐวุฒิ เกิด กรุงเทพ` | a real Thai name (per-syllable clusters, two `GPOS` offsets), a reordered leading vowel, and a plain-Thai negative control. |
| `e3` | `น้ำ า` | a merged `SARA AM` cluster **and** a standalone `SARA AA`, in one document. Both end in the same glyph and need different `/ToUnicode` answers. |
| `e4` | `office fi AV Wo. To,` | Latin `ffi`/`fi` ligatures and three kern pairs. |
| `e5` | `结算单，共３页` | **CJK negative control.** Identical glyphs, every advance exactly 1000. |
| `e6` | `Ada ปั 结` | a mixed run, so shaping is exercised **per face-segment**: the fallback chain decides the face, shaping runs inside what it chose, never across a boundary. |
| `e7` | `AV ก` | a **kerning** Latin segment followed by another face's segment, so the *placement* of the second segment is observable. See below. |

### `e7` is why the golden can see segment PLACEMENT

`e4` and `e6` between them left a live defect green. `e4`'s Latin is shape-observable but is the
**last** segment of its element, so its cursor is never consumed by anything. `e6` *is* the mixed-run
case, but `"Ada "` carries no kern pair, so the unkerned and the shaped cursor happened to agree.
The golden was therefore green over text that was **drawn kerned and placed unkerned** — a fixture
green by accident of segment ordering, which the next ordering change would have surfaced as a
mystery regression months later.

`e7` is the case neither could produce. `"AV "` kerns (−40 units at a 1000-unit em), so the Thai
segment's origin moves by 0.64 pt at 16 pt: a regression to unkerned segment cursors is now a
different hash. `TestFaceSegmentOriginsUseShapedAdvances` states the same property directly, in
hand-derived millipoints, with `"Ada ก"` retained as the negative control.

### `e3` is why CIDs are not glyph ids

Shaping `น้ำ` yields four glyphs whose last is the **same glyph** a standalone `า` yields. Under
the ruled `/ToUnicode` mechanism the cluster's text sits on the cluster's **first** glyph and the
rest map to the empty string — so that shared glyph must map to `""` inside `น้ำ` and to `U+0E32`
when it stands alone. One CID cannot carry both.

Mapping it to `""` loses every standalone `า`. Mapping it to `U+0E32` makes `น้ำ` extract as
`NIKHAHIT + SARA AA`, which `U+0E33` has no canonical decomposition to and which NFC never
recomposes — so a reader searching this PDF for `น้ำ` would find nothing.

`/CIDToGIDMap` is ours to define, so CIDs are allocated per **(glyph, extracted text)**. CIDs
`0..NumGlyphs-1` remain the identity block and extra CIDs append above it, which is why every
document needing no extras still emits `/CIDToGIDMap /Identity` and byte-identical output. In
this document only the Thai face needs the extension.

## The semantic acceptance step (D-000.22)

A hash guards against **change**; it has nothing whatever to say about whether what was recorded
was **right**, and it cannot acquire that ability later. Story 2.2 shipped Thin Chinese past three
correct guards because none was asked whether the value *meant* what its name implied — and the
face that broke was the one whose script nobody on this project reads. **Thai is that face this
time.**

So, at this first recording, `TestShapedTextGoldenFixture` asserts these **off the produced PDF**,
separately from and additionally to the hash — each one first proving the object it reads exists:

1. the content stream contains at least one `TJ` array with a **non-zero** adjustment;
2. the Latin face emits the **ffi ligature CID**, and that run emits fewer CIDs than it has runes;
3. the Thai face draws the **lowered** mark form and not the unlowered one
   (`TestShapedTextFixtureThaiDrawsTheLoweredMarkForm`);
4. `/ToUnicode` **round-trips** the document's full source text
   (`TestShapedTextFixtureToUnicodeRoundTrips`) — note that "every CID has an entry" is *not* the
   assertion, since that is satisfied by mapping all four glyphs of `น้ำ` to `น้ำ`, extracting the
   word four times over;
5. `/BaseFont` reads `TAG+<PostScriptName>` per face, `TAG` six uppercase letters — the **shape**,
   never a literal tag, because the tag legitimately changed when the subset input became the
   shaped glyph set;
6. the embedded programs carry **no `fvar` and no `gvar`** and are Regular (carried forward from
   Story 2.2, so a face regressing to variable reddens here too);
7. the page count and declared page size are what the template declares.

### The human step: PENDING

**A human has NOT yet read the rendered Thai. This hash was frozen with that sign-off outstanding.**

Say it that way round, because an earlier version of this section said the opposite — it claimed a
human had looked, and cited the story's completion notes as evidence, while those notes say the
sign-off is outstanding. The claim was false from birth. D-000.24, verbatim: *"A guard credited with
a red-proof it does not have is worse than one openly labelled unproven, because the label tells the
next reader **where to look**. A false credit tells them the opposite."* This README is the artifact a
future re-recorder consults, so it is the worst possible place for one.

What **was** done is a developer-side visual check, and it is genuinely useful: the shaped render was
rasterised and compared line by line against a deliberately naive render (the shaper replaced by the
rune→`cmap` loop it supersedes), and the marks moved off the ascenders exactly as predicted. It is
recorded in the story's completion notes. It is not a substitute, because the thing being checked is
whether Thai *reads correctly to someone who reads Thai*.

**The obligation is tracked by a failing test, not by this paragraph.**
`TestShapedTextThaiSemanticSignOffIsRecorded`
(`folio-go/shaped_signoff_matrix_test.go`) is gated behind the `matrix` build tag, exactly as the
cross-target matrix legs are. Story 2.3 therefore commits green, and **the Epic 2 boundary gate
cannot pass** until this directory contains `thai-signoff.json`:

```json
{
  "reader":   "<the human's name>",
  "date":     "<YYYY-MM-DD>",
  "examined": "<what they looked at and what they saw>",
  "sha256":   "<expected.json's sha256, unchanged>"
}
```

The `sha256` field is not bookkeeping. **It binds the sign-off to the exact bytes it signed off**, so
re-recording this fixture automatically invalidates it and demands a fresh look — otherwise a
sign-off silently carries over to bytes nobody has seen, which is the stale-provenance failure this
project has already been burned by once. It costs one line and it is the whole anti-rot property.

If the Thai reads **wrong**, the fixture is re-recorded rather than the sign-off written. That
outcome is what this gate exists to make possible, and it is cheaper than the reverse.

### The boundary this deferral does not cross

Stated here so it is never re-derived, and so "pending" cannot become the default (D-2.3.5):

> **D-000.22's machine-checkable half is never deferrable. Only its irreducibly-human half is, and
> only against a mechanical blocker.**

Weight class, name records, `TJ` adjustment values, axis pins, byte-identity across targets — all
asserted **at recording**, with no deferral available. Every one of the seven properties listed above
is asserted today, in this story, off the produced PDF. Only *"does this Thai read correctly to
someone who reads Thai"* may be pending, because it is the one thing no assertion substitutes for.

**The evidence for that split is this project's own: the Thin-Chinese defect was caught by a
machine-checkable property, not by a human look.** The machine half is where nearly all the value has
come from, so it stays mandatory — and confining "pending" to the human half means the rule loses
almost nothing while remaining livable.

## `harfbuzz-oracle.json` — the independent cross-validation

The frozen expectation table in `folio-go/shaping_expectations_test.go` is cross-validated against
**HarfBuzz itself** — the reference implementation, not a sibling port.

This is a **one-time offline reference run, hand-checked, frozen here. HarfBuzz is never a build,
test or runtime dependency of `folio-go`**: the module graph is unchanged and
`TestModuleGraphAllowlist` is untouched (AD-25 permits exactly this; Story 1.1 set the precedent
with `qpdf --check`).

Two alternatives were rejected:

- **`go-text/typesetting`**, which `epics.md:753` named, fails `gomod_test.go`'s `wantModuleGraph`
  — a guard whose entire purpose is to make a new module a conscious act — and it is a *sibling*:
  `textshape`'s README credits `benoitkugler/textlayout`, and `go-text/typesetting` descends from
  the same code. Two ports with a shared ancestor agreeing is weaker evidence than it looks.
- **`textshape`'s own `harfbuzz-tests/` corpus** is curated by the vendor and selected to pass. It
  says what the vendor already knew it got right. This oracle runs HarfBuzz against **folio8's**
  corpus instead.

Each row records the exact `argv` **verbatim** and the SHA-256 of the font file it was run
against, because a *described* command drifts from the *run* command, and because an agreement
measured against an artifact folio8 no longer ships is not an agreement. `assertOracleSubject`
checks that digest against the committed face on every run.

**To replay:** run each row's own `argv` verbatim from the repository root. Recorded with
`hb-shape (HarfBuzz) 14.2.0`.

## If this hash goes red

Under **AD-21/AD-22 this is a defect until proven to be an intended, versioned change.** Do not
regenerate the fixture to make a test pass. Establish first *what* changed and *why*; a golden
that is re-recorded whenever it disagrees is not a golden.

If you re-record it deliberately, the semantic assertions above and the human check are **not**
optional extras to be skipped because "only the hash moved" — the moment this fixture is committed
it becomes an **input**, and every downstream guard agrees with it forever.

## Provenance — external structural validation (D-000.53)

No golden artifact is accepted — first recording **or re-recording** (D-000.44) — until a reader this
project did not write parses it and resolves it into the semantic objects it claims to contain.

| | |
|---|---|
| reader | `qpdf` **12.4.0** |
| invocation | `qpdf --check fixtures/shaped-text/expected.pdf` |
| result | exit **0** — `No syntax or stream encoding errors found` |
| invocation | `qpdf --show-npages fixtures/shaped-text/expected.pdf` |
| result | **1** page(s), matching the declared `/Count` |
| validated at | `50ad6c8` (Story 2.6) |

**Settled**, validated unchanged at `50ad6c8`. This artifact predates D-000.53 and was validated retroactively; it is recorded here so the row ends settled rather than carried (D-000.29).

The external reader is the **acceptance instrument, at recording time only** — run off-leg on the
recording machine, hand-checked, output pasted here. It is never a runtime or CI dependency (AD-25,
`TestModuleGraphAllowlist`), and it is deliberately **not** gated to "the legs that have qpdf": a check
that runs on some legs and not others reproduces D-000.9's failure — an "all clear" indistinguishable
from "I could not look" — one level up, at the leg. The standing every-leg regression guard is the
in-repo checker `folio-go/golden_structural_validity_test.go`, which is hermetic and covers all four
targets including `js-wasm`.
