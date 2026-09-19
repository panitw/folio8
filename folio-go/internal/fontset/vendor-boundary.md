# The vendor boundary: every accessor folio8 calls, and what it says when it does not know

**Measured against `github.com/boxesandglue/textshape v0.0.15`, and against nothing else.**
This document is a claim about one dependency at one version. `folio-go/go.mod` pins that version and
`go list -m all` is asserted to return exactly two modules, so the version above is checkable rather
than asserted. If the pin moves, every row here is unverified until the assertions in
`vendorboundary_test.go` are re-run — which is why those assertions **derive** these claims from the
vendor rather than restating them.

Produced by Story 2.3a (`2-3a-audit-the-vendor-boundary`), under **D-000.25**. Its two ruled fixes are
**D-2.3a.1** (validate table presence at face ingestion) and **D-2.3a.2** (name the `/BaseFont`
substitution).

## Why this document exists

The engine's determinism rests on integer arithmetic and on knowing what its dependency actually told
it. `textshape` has a habit worth taking seriously: **when a piece of a font is missing, it does not
say so — it returns a believable substitute and carries on.** Ask for the font's name and it answers
`"Unknown"`. Ask how tall the capitals are and, if the font never said, it answers with the height of
the tallest letter instead. Nothing in the return value distinguishes a real answer from an invented
one, so a document can come out well-formed while resting on fiction.

Before this audit the project had no written answer to *"what does our one dependency tell us when it
does not know?"* It has one now, and the load-bearing rows are executed rather than narrated.

## Method, precisely enough to replay

Every "with its table absent" observation was produced the same way, and the method is stated because
a differently-shaped mutation measures a different thing:

- **Subject:** `folio-go/testdata/fonts/Roboto-Regular.ttf` — unitsPerEm 2048, 1,294 glyphs,
  `OS/2` version 4 with a real `sCapHeight` of 1456.
- **Mutation:** exactly **one 16-byte record removed from the table directory**, with `numTables`
  decremented. The remaining records keep their original **absolute offsets** — the records after the
  removed one move back 16 bytes *within the directory*, the 16 bytes vacated at the directory's end
  are zeroed, and **the file's length is unchanged**. The stripped table's own bytes stay exactly
  where they were, unreferenced.
- **Why that shape and not a truncation:** it removes the *reference* without disturbing anything
  else, so the observation isolates "this table is absent" from "this font is damaged". An earlier
  attempt shifted the file's tail by 16 bytes instead; every surviving offset then pointed 16 bytes
  past its table and the face reported **14 glyphs** for every mutation — a number that looks like a
  measurement and is an artifact of the mutation. It is recorded here because the wrong version
  produced *plausible* output, which is the failure mode this whole document is about.
- **Read back through:** `ot.ParseFont` → `ot.NewFace`, then the accessor named in the row.
- The helper is `stripTableRecord` in `vendorboundary_test.go`, and every assertion using it
  **asserts the stripped font still parses first** — a strip that produced an unparseable font would
  make every row below pass vacuously, for the wrong reason.

## The three states a row can be in

Two states are not enough, and the third is the one that surprised the audit.

| state | meaning |
|---|---|
| **honest** | it reports absence — an `error`, a `nil`, a `false`. The caller can tell. |
| **substituted** | it returns a **plausible value** in place of data it does not have. |
| **fabricated** | it returns a value for input **outside its domain** — not a default, a real number belonging to something else. |

## Table 1 — vendor behaviour, measured

| accessor | intact | with its table absent | state |
|---|---|---|---|
| `ot.ParseFont` | ok | — | **honest** (returns `error`) |
| `ot.NewFace` | `err=nil` | **`err=nil` in every case measured** | **it has no failure mode** — it ends `return f, nil` on every path and discards every table parse error into `_`. See Table 3. |
| `(*ot.Font).HasTable(tag)` | `true` | `false` | **honest** |
| `(*ot.Font).TableData(tag)` | bytes | `ErrTableNotFound` | **honest** |
| `(*ot.Font).NumGlyphs()` | `1294` | **`0`** — on a missing `maxp` **or one shorter than 6 bytes** | **substituted** — a caller looping to it silently does nothing, and a caller subtracting from it reaches a negative length |
| `(*ot.Face).Upem()` | `2048` | **`1000`** (no `head`) | **substituted** — `"Default for CFF"`, `ot/metrics.go` |
| `(*ot.Face).PostscriptName()` | `"Roboto-Regular"` | **`"Unknown"`** (no `name`) | **substituted** — a well-formed string every downstream assertion accepts |
| `(*ot.Name).FamilyName()` | `"Roboto"` | `"Unknown"` / unguarded `entries[1]` | **substituted** |
| `(*ot.Face).Ascender()` | `1900` | **`800`** (no `hhea`) | **substituted** |
| `(*ot.Face).Descender()` | `−500` | **`−200`** (no `hhea`) | **substituted** |
| `(*ot.Face).CapHeight()` | `1456` | **`1900`, i.e. `Ascender()`** — no `OS/2`, **or `sCapHeight == 0`** | **substituted** |
| `(*ot.Face).BBox()` | `(−1509,−555,2352,2163)` | **`(0,−200,1000,800)`** (no `head`) | **substituted** |
| `(*ot.Face).HorizontalAdvance(g)` | `1839` for gid 36 | **`upem` = `2048`** (no `hhea` **or** no `hmtx`) | **substituted** — and float-typed, see Table 4 |
| `(*ot.Face).HorizontalAdvance(65535)` | **`506`** on a 1,294-glyph face | — | **fabricated** — `(*ot.Hmtx).GetAdvanceWidth` returns `lastAdvanceWidth` for any out-of-range gid |
| `(*ot.Hmtx).GetAdvanceWidth(65535)` | **`506`**, same face | — | **fabricated** — identically; the integer path folio8 now uses inherits this and must bounds-check |
| `ot.ParseHmtxFromFont` | ok | `ErrInvalidTable` when `NumGlyphs()==0`; `TableData`'s error for a missing `hhea`/`hmtx` | **honest** — strictly more honest than the accessor above |
| `(*ot.Face).Cmap()` | non-nil | **`nil`** | **honest** — and folio8 nil-checks it at both call sites |
| `(*ot.Cmap).Lookup(cp)` | `(gid, true)` | `(_, false)` | **honest** |
| `subset.CreatePlan` / `(*Plan).Execute` | ok | `subset: required table missing` | **honest** |
| `(*Plan).MapGlyph` / `(*Plan).OldGlyph` | `(gid, true)` | `(_, false)` | **honest** |
| `(*Plan).NumOutputGlyphs` | count | — | **honest** |
| `ot.NewShaperFromFace` | ok | returns `error` | **honest** |
| `(*ot.Shaper).Shape` → `buf.Info` / `buf.Pos` | parallel slices | — | **unchecked contract** — the vendor does not guarantee the two slices have equal length; folio8 checks it explicitly in `internal/text/shape.go` |

`subset.Subset([]uint16{65535})` on the 4,515-glyph NotoSans face is a further **fabricated** row,
measured by Story 2.3: it returned a 460-byte program and a complete mapping with no error. `Subset`
in this package validates every source id against the face's glyph count before the vendor sees it.

## Table 2 — what folio8 does with each, at the public entry point

`folio8.Render(tmpl, folio8.Data("{}"), folio8.Params("{}"), folio8.FontSet{"Roboto-Regular": <bytes>})`
against `fixtures/font-text/input.folio`. **The "before" column was measured at commit `431a6a5`, the
"after" column at this commit** — every row's disposition ends *confirmed and fixed* or *traced and
closed*, never carried (D-000.29).

| table removed | before (`431a6a5`) | after | disposition |
|---|---|---|---|
| *(none)* | 22,310 bytes, `/CapHeight 711`, `err=nil` | **identical: 22,310 bytes, `/CapHeight 711`** | byte-neutral, asserted |
| `head` | located error, `read head table: table not found` | located error naming the face and `"head"` | **traced and closed** — `readUnitsPerEm` already declined `Upem()` |
| `maxp` | **PANIC**, `makeslice: len out of range` | located error naming the face and `"maxp"` | **confirmed and fixed** — D-2.3a.1 |
| `OS/2` | **22,198 bytes, `err=nil`, `/CapHeight 928`** | located error naming the face and `"OS/2"` | **confirmed and fixed** — D-2.3a.1 |
| `hhea` | located error, from the vendor subsetter | located error naming the face and `"hhea"`, at ingestion | **confirmed and fixed** — now refused by folio8, not by the subsetter |
| `hmtx` | located error, from the vendor subsetter | located error naming the face and `"hmtx"`, at ingestion | **confirmed and fixed** — as above |
| `name` | 22,310 bytes, `err=nil`, `/BaseFont /HXRYNT+Roboto-Regular` — **indistinguishable from intact** | renders, **and the PDF now names the substitution** | **confirmed and fixed** — D-2.3a.2 |
| `cmap` | located error: no face in the chain has a glyph for `U+0048` | **unchanged** | **traced and closed** — see below |

### The `OS/2` row is this audit's answer in one line

At `431a6a5`, a caller-supplied face without `OS/2` rendered a document that **reported success** and
declared **`/CapHeight 928`** — which is not a cap height, it is the ascender scaled to the PDF's
1000-unit em (`1900 × 1000 / 2048 = 928`; the intact face gives `1456 × 1000 / 2048 = 711`). Every
guard in the repository stayed green. That is D-000.25's Finding 2 arriving at an **output byte**.

It never affected anything folio8 ships: all three shipped faces carry `OS/2` version 4 with a real
`sCapHeight` (NotoSans 714, NotoSansSC 733, NotoSansThai 714). The exposure was **caller-supplied
faces** — which is exactly what `folio8.FontSet` is, a public `map[string][]byte`.

### Why `name` and `cmap` are read but **not** required

The guard's table list is not "every table" and not "the tables that broke". It is *the tables folio8
reads*, minus two whose absence folio8 already reports honestly — and each exclusion is a ruled
disposition, not a judgement made here:

- **`name`** — Story 2.2 deliberately tolerates a nameless program. `readPostScriptName` declines
  `PostscriptName()` and returns `""`, which is **observably absent** rather than a plausible string.
  D-2.3a.1 says verbatim: *"Do not require `name`"*, because requiring it would reverse that.
- **`cmap`** — the vendor is **honest** here (`Cmap()` returns `nil`) and folio8 nil-checks it at both
  call sites. Requiring it would break Story 2.2's fallback chain: today a chain whose first face
  carries no `cmap` falls through to the next face and renders. Requiring the table would turn that
  into a load error and make the whole `FontSet` unloadable over one unusable member — a behaviour
  reversal, not a tightening. Measured: with `cmap` stripped, `Render` reports *"no font in chain
  [Roboto-Regular] has a glyph for rune U+0048"*, which is the located failure Story 2.2 ruled for.

`glyf` and `loca` are absent from the list because **folio8 never reads them itself**. The vendor
subsetter does, and reports their absence honestly.

### `Face.Upem()` — traced and closed, and the ruling's premise retired

D-000.25 listed `Upem()` as *"a plain field read — but its population path is unaudited"*, warning
that if construction substituted a default for a missing `head`, **D-1.5.2's 16–16384 validation would
pass on fiction.**

Traced. `ot.NewFace` does exactly the feared thing — `if f.upem == 0 { f.upem = 1000 }` — and
measured, `face.Upem()` returns **1000** on a `head`-stripped Roboto whose real upem is 2048.
**But folio8 never reads it.** `New` calls `readUnitsPerEm`, which parses the `head` table directly and
propagates `ErrTableNotFound`. **D-1.5.2's validation does not pass on fiction.** The row ends
**settled**; do not re-open it.

## Table 3 — `ot.NewFace` has no failure mode

Measured across all seven table-removal cases above: `ot.NewFace` returned `err == nil` every time,
**including the case that panicked**. It ends `return f, nil` on every path and discards every table
parse error into `_`.

`fontset.go`'s `ot.NewFace` error branch is therefore **unreachable**. Under D-000.24 the honest
treatment is to **keep it and label it** — it is the assertion that would catch the vendor gaining a
failure mode — and **not to count it as coverage**. It is labelled in the source and there is no test
claiming to exercise it.

## Table 4 — the float boundary, and why the old guard could not see it

`(*ot.Face).HorizontalAdvance` returns a fractional type. Its two branches both convert a `uint16`,
and a `uint16` is exactly representable in a 24-bit mantissa, so `int64(accessor(x))` was **lossless
for every input the vendor can produce**. There is **no red-proof that the old sites were lossy, and
there cannot be one** — do not manufacture one (D-000.24). The disposition was never "prove they are
wrong"; it was to stop asking a fractional question that has an integer answer.

**The integer answer is the same answer, and that was measured, not assumed:**

| face | glyphs | `int64(HorizontalAdvance(g))` vs `int64(GetAdvanceWidth(g))` |
|---|---|---|
| `folio-go/testdata/fonts/Roboto-Regular.ttf` | 1,294 | **0 mismatches** |
| `folio-go/fonts/notosans/NotoSans-Regular.ttf` | 4,515 | **0 mismatches** |
| `folio-go/fonts/notosansthai/NotoSansThai-Regular.ttf` | 467 | **0 mismatches** |
| `folio-go/fonts/notosanssc/NotoSansSC-Regular.ttf` | 31,036 | **0 mismatches** |
| **total** | **37,312** | **0** |

So the swap changed no width, no `/W` array, no content stream and no golden.

**The guard that could not see it.** `folio-go/internal/arch_test.go` matches the *spelling* of a type
identifier and the *kind* of a literal. A vendor function returning a fractional value is invisible to
it: the type is inferred, so neither identifier is ever written. Measured at `431a6a5` — the
syntactic guard reported **zero** under `folio-go` while **four** float-typed value expressions stood
in `internal/fontset/fontset.go` (lines 328, 329, 565, 566).

The same blind spot has a second, independent face: `PinAxisLocation(tag, 700)` passes an untyped
integer constant to a fractional parameter. No identifier is written and the literal's kind is `INT`,
so the syntactic guard cannot see it either. Comments across the repository claimed AD-23 made
`PinAxisLocation` *unreachable* on exactly that basis; **that claim was false** and is corrected at
its source (D-2.2.4 correction, and D-2.2.4 (correction, amended)).

**The claim was swept for, not sampled.** D-2.2.4 (correction) enumerated **four** sites; Story
2.3a's review found **two more**; Story 2.3a's finisher swept every form of the claim
(`PinAxisLocation`, "unreachable", "AD-23 bans", "identifier `float32`", "arch guard bans") across
**every file type**, not just `.go` and `.md`, and found **eight** carrier sites in total — the
eighth class of file being a Python build tool, which no earlier pass had looked at. Six are
corrected in place (four in `internal/fontset/fontset.go` and `internal/text/shape.go`, one in
`internal/fontset/fontset_test.go`, one in `tools/fontgen/instance_faces.py`); the remaining two sit
in a closed story record and are corrected by an appended section rather than an edit, under
D-1.6.6. Instances inside `folio-mvp-decision-log.md` are left exactly as written: that document is
append-only by its own header, and its correction is itself an appended entry.

Three further sites match the search terms and are **not** carriers — they state the claim in order
to record that it is false (this document, and `lint/internal/rules/floattyped_test.go`), or
describe the identifier guard accurately and conditionally (Story 1.5's record, which says only that
"a call site that **names the type** would go red", which is true).

Both are now covered by `lint`'s **`no-float-typed-value`** rule, which matches on the type go/types
**resolves**, never on what the source spells. The syntactic guard is unchanged and still runs: two
guards, two mechanisms, one invariant.

## Where the executable form lives

The load-bearing rows above are **derived** from the vendor by
`folio-go/internal/fontset/vendorboundary_test.go`, so this document cannot drift away from the
artifact it describes (D-000.14). Rows whose hazard is not reachable through the public path carry
that label in the assertion's own text and are **not counted as coverage** (D-000.24).
