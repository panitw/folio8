# `fixtures/declared-variants/` — a document that declares its cuts (Story 11.5)

The golden for **a chain entry that declares `bold`, `italic` and `boldItalic`, and elements that ask
for all four styles**. It is the corpus's **twenty-fourth** document and the **first that declares
bold or italic at all**.

> **The `expected.pdf` in this directory is ATTESTED.** Panit Wechasil read the page on
> **2026-09-06** and the record is in [`signoff.json`](signoff.json). See
> [What a human was asked to judge](#what-a-human-was-asked-to-judge) below.
>
> It shipped as a **candidate**: `folio-go/declared_variants_signoff_matrix_test.go` was written as a
> *failing* test (D-11.5.1, arm [A]), the story **halted** on it, and the red was discharged inside
> the same story rather than filed as a deferral. That gate reds again, by construction, the moment
> `expected.pdf` is re-recorded — a reading is a reading of specific bytes.

## What it red-proves (DW-237)

Story 11.2 made a text element ask its chain for a (weight, slope) and made the chain answer with a
face it **declares** — `chainFaceNames`' own comment states the rule: *"A VARIANT IS READ, NEVER
CONSTRUCTED."* Story 11.2's DW-233 tripwire covers that **mechanism** behaviourally. Nothing covered
the **outcome** in recorded bytes.

Measured at this story's baseline, twice and by two independent mechanisms: `grep -a` for `"bold"`
and for `"italic"` over every file under `fixtures/` returned **0**, and an independent `python3`
byte-walk over every file in all 29 fixture directories returned **0** as well — with the positive
control `"fontFamily"` returning 23 files, so the instrument was live. So the engine could have
started drawing every bold run in the regular face and **every gate in this repository would have
stayed green**: no diagnostic, no red test, no moved golden.

The red proof that this fixture is a witness rather than a decoration is the measurement in Story
11.5's own record: with `variant := entry.Variant(want)` deleted from `folio-go/render.go`'s
resolver, **this** golden moves and this fixture's test fails, and **no pre-existing golden test
fails at all** — which is DW-237 demonstrated rather than restated.

## What each element is for

| Element | Value | What it proves |
|---|---|---|
| `e1` | `Handgloves — Roboto Regular`, 24 pt | the base face, and the comparison line every other one is read against |
| `e2` | `Handgloves — Roboto Bold`, 24 pt, `bold` | the `bold` sibling was READ: the run selects `/RobotoBold` and the page embeds a `Roboto-Bold` program |
| `e3` | `Handgloves — Roboto Italic`, 24 pt, `italic` | the `italic` sibling, likewise |
| `e4` | `Handgloves — Roboto Bold Italic`, 24 pt, `bold` + `italic` | the `boldItalic` sibling — the one a naive "append Bold" convention gets wrong |
| `e5` | `Handgloves quickly`, 24 pt, `align: center` | THE CONTROL half of the metrics witness |
| `e6` | `Handgloves quickly`, 24 pt, `align: center`, `bold` | the same string in the same 400 pt box, in the bold cut |

**Each of `e1`–`e4` makes its claim in the face it claims.** A line that *says* "Roboto Bold" while
*looking* regular is a swap a person sees instantly, so the page witnesses itself. "Handgloves" is
the type-tester's word because it carries ascender, descender, round bowl and tight counters in ten
letters.

**`e5`/`e6` are the machine's test, and they exist because the obvious version was vacuous.** The
first design was a wrap demonstration — the same paragraph in the same box, regular against bold,
expecting bold to take an extra line. **It never did**: swept across 14 box widths from 150 to
340 pt, the line counts were equal every single time. Measured rather than tuned: centred in a 400 pt
box, the same string starts at **x = 132.548** regular and **x = 130.724** bold. Centring places a
line at `left + (box − measured) / 2`, so the origin shows **half** the width difference: that
1.824 pt offset is a line **3.648 pt wider**, on a regular line measuring 206.904 pt. Bold metrics
*do* reach layout — Roboto's bold is simply only **~1.76 %** wider than its regular, so a break
almost never moves. (The doubling is the step it is easy to drop; halving 3.648 into 1.4 % is the
arithmetic slip this sentence exists to stop.)

**Someone will propose the wrap test again**, because it is the obvious way to show bold metrics
reaching layout. The measurement is recorded here so the next reader inherits it instead of repeating
the sweep: a wrap assertion on this family would be a test that passes for the wrong reason.

## No Thai and no CJK, deliberately

A bolded CJK run earns a Warning — no cut exists for Noto Sans SC (D-A) — and Thai would drag in the
mark-placement sign-off precedent, muddying what the owner is being asked to judge about *this* page.
The document renders with **zero diagnostics**, and that is part of its definition: every cut it asks
for is declared, so a correctly-authored render never reaches
`DiagCodeTextStyleFaceUndeclared`.

## What is recorded

- One A4 portrait page, 36 pt margins, six single-line text elements.
- Four **distinct** font resources selected by four `Tf` operators: `/Roboto`, `/RobotoBold`,
  `/RobotoItalic`, `/RobotoBoldItalic`.
- Four **distinct** embedded `FontFile2` programs, naming `Roboto-Regular`, `Roboto-Bold`,
  `Roboto-Italic` and `Roboto-BoldItalic` in `/BaseFont`.
- 60,594 bytes, `sha256` `2405d005bbb1297556e41770cfa9353e1171b2d85af809dc1d21b0504f75ef4d`.

The document declares `"version": "2.0"`: an object-form chain entry raises the saved version through
`fontsRequireMajor`, which is keyed on the same `FontChainEntry.SerialisesAsObject` predicate
`writeFontChain` uses to choose the emitted shape.

## How `expected.pdf` was recorded

`SOURCE_DATE_EPOCH` must be **unset** — exported, the CLI injects an `/Info` dict the in-process
render can never reproduce. From `folio-go/`:

```sh
CGO_ENABLED=0 GOWORK=off go run ./cmd/folio8 render \
  -o ../fixtures/declared-variants/expected.pdf \
  ../fixtures/declared-variants/input.folio
```

`input.folio` is the serializer's **own canonical output** — generated by
`ParseTemplate` → `SerializeTemplate` rather than hand-formatted — so the document is a fixed point
the engine will not rewrite on save.

## What a human was asked to judge

This is the standing procedure. It was carried out on 2026-09-06, and it is what the next reader
repeats when a re-record stales the record.

Open `expected.pdf`. Four lines, each naming the cut it is set in. The question is only:
**is line 2 a real bold face, or a regular face thickened?** The tells, in order of reliability:

1. **Counters** — the enclosed white inside `a`, `e`, `o`, `g`. A real bold keeps them open and
   shaped; a smeared regular chokes them toward slits. Line 2's "Handgloves" against line 1's is the
   direct comparison.
2. **Stem-to-round contrast** — a real bold thickens vertical stems more than the thin parts of
   curves. A synthetic bold thickens everything uniformly and looks inflated rather than drawn.
3. **Width** — a real Roboto Bold is slightly wider; line 2 should end marginally right of line 1's
   comparable point, not sit exactly on top of it.
4. **Lines 3 and 4** — a real italic is a *drawn* italic, not a slanted regular: check `a`, `f`, `e`
   for different letterform construction rather than the same shapes leaning over.

If the faces read wrong, the fixture is **re-recorded** rather than attested — that is the outcome
the gate exists to make possible, and it is far cheaper than the reverse.

To attest, write `signoff.json` in this directory:

```json
{
  "reader":   "<your name>",
  "date":     "<YYYY-MM-DD>",
  "examined": "<what you looked at and what you saw>",
  "sha256":   "<the sha256 of the expected.pdf you actually opened>"
}
```

**The digest must be the hash of the PDF you opened**, taken directly from it:

```sh
shasum -a 256 fixtures/declared-variants/expected.pdf
```

Not "the value in `expected.json`". Today the two are the same number, so copying the sidecar works
and nothing notices. After a re-record they are the same number again — both updated in the same
stroke — while the *page you read* was the old one, and copying the sidecar would then manufacture
exactly the stale attestation this gate exists to prevent. The gate hashes the artifact for that
reason; write down what you hashed.

**And keep the record declared as a digest site**, or an untagged test reds. `goldenDigestRecord`
(`folio-go/byte_neutrality_test.go`) asserts that the set of files carrying this golden's digest is
exactly the set it declares. This fixture declares **four** sites, and the fourth is the record:

```go
{kind: "expected.json", relPath: "fixtures/declared-variants/expected.json"},
{kind: "second-literal"},
{kind: "readme",   relPath: "fixtures/declared-variants/README.md"},
{kind: "signoff",  relPath: "fixtures/declared-variants/signoff.json"},
```

A record arriving without that entry is an undeclared site and reds
`TestGoldenDigestAgreesAtEveryDeclaredSite` — which is exactly what happened the first time. On a
re-record, all four move **together**, in one commit, alongside a fresh reading.

**No agent may invent that file.** `reader`, `date` and `examined` are claims about a person having
looked, and one written on their behalf is a fabricated attestation — the single failure the whole
mechanism exists to prevent. The record that exists states on its face how it was produced: a
verbatim transcription of the reader's own words and judgment, made at the reader's explicit
instruction. Any future record should be as explicit about its own provenance.
