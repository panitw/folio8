# Shipped font weight — measured

Measured against `folio8-go/fonts/` with `du -sh`.

| Face | Cuts | Size | Share | Tier |
| --- | --- | --- | --- | --- |
| **Noto Sans SC** | 1 | **10 MB** | **71%** | **opt-in** |
| Noto Sans | 4 (regular, bold, italic, bold italic) | 2.6 MB | 19% | core |
| Roboto | 4 (regular, bold, italic, bold italic) | 1.5 MB | 11% | core |
| Noto Sans Thai | 2 (regular, bold) | 124 KB | 0.9% | core |
| **Total** | 11 | **14 MB** | | core ≈ **4.2 MB** |

## Why the obvious cut is the wrong one

The proposal that opened this work was to drop the Noto family, on the grounds
that Roboto is the default. Measurement does not support it:

**Roboto is the default *face*, not the default *coverage*.** `fonts.go` records
that the starter template names its default chain `"Roboto"` **over the same
three families** — Roboto is first in a fallback chain whose later entries are
Noto Sans Thai and Noto Sans SC. Roboto covers Latin, Greek and Cyrillic. It has
**no Thai and no CJK glyphs at all**. Dropping the Noto faces does not slim the
default chain; it removes Thai and Chinese rendering.

**The Thai face is not where the weight is.** Noto Sans Thai is 124 KB — nine
tenths of one percent. Removing it saves nothing measurable and forfeits a
first-class concern: the Thai bill-payment barcode fixture, the `thai_words.trie`
dictionary embedded in the engine, and Thai line-breaking.

**Noto Sans Latin is the redundant one, and is kept anyway.** At 2.6 MB it
genuinely overlaps Roboto for new documents. `fonts.go` keeps it deliberately:
the three original Noto names remain shipped *"for every document that names
them, `body` chains from before Story 16.8 included."* Removing it is a
back-compat break against templates already authored.

**So the whole opportunity is one face.** Noto Sans SC is 71% of the set and is
useless to any document that does not render Chinese. Making that one face
opt-in captures the entire available saving while leaving every other decision
untouched.

## Resulting tiers

```
core   (~4.2 MB, embedded everywhere)
  Roboto            ×4 cuts
  Noto Sans         ×4 cuts
  Noto Sans Thai    ×2 cuts

opt-in (10 MB, one explicit dependency)
  Noto Sans SC
```

---

# What Noto Sans SC actually covers

Measured against the shipped `folio8-go/fonts/notosanssc/NotoSansSC-Regular.ttf`
— Adobe/Source Han derived, v2.004 — with fontTools: **31,036 glyphs,
30,890 mapped codepoints, 10.1 MB.**

| Block | Covered | | Block | Covered |
| --- | --- | --- | --- | --- |
| CJK Unified Ideographs | **20,976 / 20,992 — 99.9%** | | Basic Latin | 100% |
| CJK Extension A | **6,582 / 6,592 — 99.8%** | | Latin-1 Supplement | 100% |
| Katakana | 100% | | Fullwidth forms | 93.3% |
| Hiragana | 96.9% | | Latin Extended-A | 23.4% |
| CJK punctuation | 100% | | Greek | 34.0% |
| Bopomofo | 89.6% | | Cyrillic | 25.8% |

The partial Latin/Greek/Cyrillic rows do not matter: Roboto and Noto Sans
already cover those in core. CJK Extension B is effectively absent (54 of
42,720), so rare and historic ideographs are out of scope regardless of tier.

## Why the face is 10 MB, and why subsetting is not the answer

The two ideograph blocks are **~27,500 of the 31,036 glyphs**. This is not a
font with a large alphabet; it is 27,500 ideographs. There is no trimming
available that does not amount to choosing which Chinese characters to stop
supporting — which is precisely why the tier boundary is **the whole face**
rather than a subset of it.

## The tier is Chinese *and Japanese*, not "CJK"

Measured, **Japanese depends on this same face**: hiragana, katakana and kanji
are all present. Naming the tier "CJK" understates who the opt-in package is
for and invites a Japanese caller to conclude it is not for them.

**With a caveat that must be stated wherever the tier is named.** Japanese and
Traditional Chinese *render* — every sampled codepoint in `繁體中文`, `臺灣`,
`ひらがなカタカナ` and `日本語漢字` is present — but the glyph shapes are
**Simplified Chinese regional variants**. Characters whose form differs by
region come out with mainland conventions. Coverage is complete; regional
typographic correctness is not.

## Korean is not covered at all

| Block | Covered |
| --- | --- |
| Hangul Jamo | **0 / 256** |
| Hangul Syllables | **0 / 11,172** |

`한국어` renders as tofu. No face in the shipped set covers Hangul, so this is a
**pre-existing gap in folio8**, not one the tiering introduces. It is recorded
here because this is where the coverage evidence now lives.

It also exposes a difference the spec's capabilities do not yet distinguish: a
**missing face** (CAP-3 — the package is not installed) fails loudly, while
**missing coverage** (the face is installed and simply has no glyph) fails
silently into tofu. See the open question.
