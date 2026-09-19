# `fixtures/justified-thai/` — a Thai paragraph justified to both margins (Story 7.3)

The golden for **`style.align: "justify"` over text that has no spaces in it**.

`fixtures/justified-text/` is pure Latin, so every gap it distributes slack into is a run of
whitespace the author typed. Thai writes its sentences **without spaces**: a Thai line's break
opportunities come from the shipped dictionary (AD-25), and until this fixture no test in the tree
named Thai and `justify` together. The behaviour was already correct when this fixture was added —
measured, not assumed, before a byte was recorded — so what this document adds is not a fix but the
thing that makes the behaviour **falsifiable**. That is the same absence that let `valign` ship
uncovered and cost DW-24 three stories.

It is the corpus's second document declaring **`"version": "2.0"`**, for the same reason
`fixtures/justified-text/` is the first: `align: "justify"` extends a closed set, which D-1.4.12
makes a MAJOR change, and under D-1.4.13 a document declares the lowest version its own content
requires.

## What each element is for

| Element | `align` | Box | What it proves |
|---|---|---|---|
| `e1` | `justify` | 220 pt | the subject: six lines, four of them justified across **dictionary-derived** gaps |
| `e2` | **absent** | 220 pt | THE CONTROL — the same string, chain, size and box with no alignment at all. It packs into the same six lines at the same six baselines, one run each, at the element's own left edge. A build that ignored `justify`, *or that saw Thai and quietly fell back to ragged left*, would render `e1` as exactly this. |
| `e3` | `justify` | 50 pt | AD-25's **atomic unknown run**: the segmenter proposes no interior break opportunity inside its first line, so that line has nowhere to put slack |

## `e1`, line by line

`e1`'s value carries **not one space character** — only the single line feed between its two
paragraphs. Every interior opportunity below is therefore a seam the shipped wordlist proposed, and
that is asserted rather than asserted-about: the test fails outright if a space appears in the
value.

| Line | Interior gaps | Slack | Result |
|---|---|---|---|
| 0 | 7 | 13,893 mp | justified — `7 × 1,984` with **5** left over |
| 1 | 6 | 12,771 mp | justified — `6 × 2,128` with **3** left over |
| 2 | — | — | **ragged**: the author ended it by typing a break, and it is *not* the last line. Only the break-kind field can answer this. |
| 3 | 5 | 7,942 mp | justified — `5 × 1,588` with **2** left over |
| 4 | 9 | 31,757 mp | justified — `9 × 3,528` with **5** left over |
| 5 | — | — | **ragged**: the last line. Only the line *index* can answer this. |

Four different gap counts, so a build that hard-coded one cannot agree with all of them. All four
remainders are non-zero, and three of the four are **larger than half the gap count** — which an
implementation spreading the remainder from the *end* would place at the wrong gaps.

## `e3`, and the third ragged condition

Three independent conditions leave a justified line ragged, and they stay three because each answers
a different question (D-7.1.5). `fixtures/justified-text/` covers two of them. The third — **a line
with no interior break opportunity at all** — was the one the acceptance criteria were originally
silent on, and only Thai makes it reachable: an unknown run is atomic, so it offers no seam.

`e3` is `"ณัฐกานต์ ปฐพี"` in a 50 pt box. Its first line is `"ณัฐกานต์"`, and **the segmenter proposes
zero break opportunities strictly inside that run**. That line is justified, is **not** the last
line, was **not** ended by a mandatory break, and leaves **9,179 mp of positive slack** — so none of
the other conditions can account for the result — and it is still set at the element's natural start
edge, because there is nowhere to put the slack.

That zero is **measured, not assumed**: the test asks the production `text.Opportunities` against the
shipped `text.Dictionary()` and counts what comes back, with a `t.Fatalf` if it is ever non-zero. So
if the segmenter's answer for this run changes, the case stops being AD-25's atomic run and the test
says so instead of quietly passing.

> **Do not check this by grepping the wordlist.** `folio-go/internal/text/wordlist/words_th.txt`
> **does** contain `กานต์`, a suffix of the run — the property here is not "the wordlist lacks these
> letters". The greedy matcher does match `กานต์` and does propose a break in front of it; D-2.1.9's
> both-sides-coverable filter (`internal/text/tileable.go`) then withdraws that proposal, because the
> stretch before it cannot be tiled by dictionary entries at all. The net answer — and the only thing
> the justification rule reads, and the only thing this fixture asserts — is **no interior
> opportunity**, whatever individual entries the wordlist happens to hold.

## The rules the numbers pin

- **The gaps are dictionary-derived.** Thai is not falling back to ragged left: `e1` draws 8, 7, 1,
  6, 10 and 1 runs across its six lines against the control's one per line, and every one of those
  piece boundaries is a wordlist seam.
- **The pieces still spell the line.** Each justified line's pieces, concatenated in ascending x, are
  asserted equal to the same line as the *control* sets it — a production string, never a hand
  transcription. Splitting Thai into per-word runs is exactly where a combining mark could be dropped
  or reordered across a piece boundary, and the digest alone would only say "hash mismatch".
- **The two ragged lines are ragged for the reasons stated.** Line 2's break kind is asserted to be
  `text.BreakMandatory` and line 5's index to be the final one — and *both* are asserted to have
  interior break opportunities, so neither is quietly the zero-gap case that `e3` covers.
- **The remainder is ordered**, in the line's own reading order, which for Thai is ascending x
  exactly as it is for Latin.
- **The right edge lands on the declared width exactly**, in integer millipoints, at the same
  standard as the Latin case.
- **`justifiedLinePieces` is script-agnostic.** It distributes over the opportunity list the caller
  already holds; nothing in it knows what script produced those opportunities, which is why this
  fixture needed no change to the justification rule.

## Files

| File | What it is |
|---|---|
| `input.folio` | the template, kept byte-identical to `folio-go/justifiedThaiTemplateJSON` by hand |
| `expected.json` | the recorded digest and the toolchain it was recorded under |
| `expected.pdf` | the recorded artifact |

The data is the empty object: the document binds nothing, because justification is a property of the
declared box and the packed line.

Recorded digest: `58ca47772e144c4b123c45e1eec3c893cd1e8c2a0e26d3f3af1ba504e6ff94fb`.
