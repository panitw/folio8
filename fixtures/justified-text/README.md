# `fixtures/justified-text/` — a paragraph justified to both margins (Story 7.3)

The golden for **`style.align: "justify"`** (FR47). Measured at this story's baseline,
`grep -oh '"align"[^,}]*' fixtures/*/input.folio` across the whole corpus returned **16 `left` and
8 `right`, and nothing else** — so no recorded byte in this repository could tell a build that
distributes a justified line's slack across its gaps from one that quietly draws it ragged. The
story's byte-neutrality guard ("every existing golden hashes identically") is only falsifiable
alongside a document that *is* justified, and this is it.

**It is also the first committed document declaring `"version": "2.0"`.** That is not decoration and
it is not the library's ceiling leaking into a document. Under D-1.4.13 a document declares the
lowest version its own *content* requires; this one requires 2.0 because `align: "justify"`
**extends a closed set**, which D-1.4.12 makes a MAJOR change. An older reader refuses an alignment
word it does not recognise — deliberately, so that a file can never be drawn wrongly by a reader
that misunderstands it — so this file is *unreadable* to a 1.x reader rather than quietly ragged,
and its version says so honestly.

## What each element is for

Both elements carry the **same string, the same face, the same size and the same 200 pt box**. They
differ in exactly one thing.

| Element | `align` | What it proves |
|---|---|---|
| `e1` | `justify` | the subject: five lines, three of them justified to the declared right edge exactly |
| `e2` | **absent** | THE CONTROL — it packs into the same five lines at the same five baselines, one run each, at the element's own left edge. A build that ignored `justify` would render `e1` as exactly this. |

## `e1`, line by line

Justification is a property of the line, and three independent conditions leave one ragged. They
stay three because each answers a different question (D-7.1.5), and a single flag collapsing any two
would make the third re-derivable wrongly.

| Line | Interior gaps | Slack | Result |
|---|---|---|---|
| 0 | 7 | 3,034 mp | justified — `7 × 433` with **3** left over |
| 1 | — | — | **ragged**: the author ended it by typing a break, and it is *not* the last line. Only the break-kind field can answer this. |
| 2 | 5 | 1,494 mp | justified — `5 × 298` with **4** left over |
| 3 | 7 | 16,839 mp | justified — `7 × 2,405` with **4** left over |
| 4 | — | — | **ragged**: the last line. Only the line *index* can answer this — a line no break ended carries the zero value `BreakOptional`. |

A third condition, unreachable in this document but stated in the rule: a line with **no interior
break opportunity** has nowhere to place slack and is set at the natural start edge too. AD-25 makes
it reachable, because an unknown Thai run is atomic.

## The rules the numbers pin

- **The remainder is ordered.** With `g` gaps and slack `s`, every gap receives `s / g` and the
  **first `s mod g` gaps in ascending position along the line** each receive one additional
  millipoint. Ascending order is the line's own reading order — never map iteration, never locale,
  never target. All three justified lines here carry a non-zero remainder, and two carry one larger
  than half the gap count, which an implementation spreading the remainder from the *end* would
  place at the wrong gaps.
- **The distributed amounts sum to the slack exactly.** No float, no second rounding site, no
  division whose remainder is discarded.
- **The right edge lands on the declared width exactly.** The slack is `boxWidth − Σ pieceWidths`,
  not `boxWidth − ln.width`: each piece's advance is rounded on its own and a sum of roundings is
  not the rounding of a sum, so a line positioned from the packer's single measurement could miss
  the edge by a millipoint or two — small enough to look like nothing, large enough to differ
  between targets. `measureRuneRange` *is* `positionSegments`' cursor arithmetic, so measuring the
  pieces with it closes the loop with one derivation.
- **Slack only.** Nothing is stretched and nothing is re-shaped. Interior inter-word spaces are real
  shaped glyphs with their own advance; justification adds space *between* pieces.
- **One rule, both producers.** The canvas shows the word positions the PDF prints (AD-17, the Story
  5.9 invariant) by consuming the same engine-computed offsets. The browser never justifies —
  `text-align: justify` is contractually banned across every production, unit and e2e source.

## Files

| File | What it is |
|---|---|
| `input.folio` | the template, kept byte-identical to `folio-go/justifiedTemplateJSON` by hand |
| `expected.json` | the recorded digest and the toolchain it was recorded under |
| `expected.pdf` | the recorded artifact |

The data is the empty object: the document binds nothing, because justification is a property of the
declared box and the packed line, and bound data would only add a way for the fixture to move for
reasons that are not its subject.

Recorded digest: `6da3b12e694fdd7d7f865631ca190346898f45f85633facdb91d2b69590777d6`.
