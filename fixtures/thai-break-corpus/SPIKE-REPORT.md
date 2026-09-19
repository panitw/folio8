# Story 2.1 — Thai break-opportunity spike: report

**This report was REWRITTEN after the story was reopened, then further
corrected by the story's finisher after a second QA review.** The prior
version was confirmed by the engineering lead against a results table whose
numbers turned out not to be measurements — that confirmation has been
withdrawn. Everything below was re-measured, fresh, against the artifacts
themselves (D-000.18: confirm against the artifact, never a table summarising
it), after seven corrections the reopening identified and this developer
independently verified before touching any code:

1. **38 corpus items had been manufactured** (obsolete-Thai-consonant
   strings) specifically to make the P6a floor pass — the generator's own
   comment admitted it. Fixed per **D-000.17** (new, binding): a floor that
   is not met is reported as unmet, never filled. Every corpus item now
   carries a `Provenance` (`sourced` / `synthetic`); genuine floors (P5, P6e,
   P6f, P6g) count `sourced` items only; the 38 synthetic tokens are kept,
   clearly labelled, for their real P6a exercise value only.
2. **P6d was wrong**: `isThaiScript` excludes U+0020 (space), so the plain
   space between a given name and a surname counted as "mixing Thai with
   Latin/digits" in every two-token name. Fixed: P6d now requires an actual
   ASCII Latin letter or digit rune.
3. **V11's switch guard didn't guard anything**: the test asserted only that
   the unconstrained mode found ≥1 break, never that it differed from the
   constrained result. Verified by injection: hard-coding
   `unconstrained = false` at the top of `ComputeBreaks` left the test
   PASSING. Fixed, and the fix is itself red-proofed the same way.
4. **P2 was measured self-referentially**: the original test compared the
   engine's own `RunUnknownThai` classification against the engine's own
   breaks list — true by construction, incapable of ever finding the failure
   mode that matters. Replaced with an INDEPENDENT ground truth (a DP-based
   full-segmentability check with no cluster constraint and no matching-order
   dependency) and the honest result is reported below: **P2 fails**.
5. **AC1's "62,107" was an artifact**: 62,107 LINES, but 62,106 DISTINCT
   words (one duplicate, "โรม่า") — which matches the epic's own figure
   exactly. The previously reported "divergence" was fabricated.
6. **AC9 was fail-open in two ways**: `os.ReadDir` never recursed into
   subdirectories, and the guard only ever reported EXTRA files, never
   MISSING ones (deleting both licence files stayed silent). Both fixed,
   both red-proofed, at the real location.
7. **P3 labelled surnames only.** Given names can be split just as wrongly.
   Extended; the finding got larger, per instruction ("the safe direction").

**The P3 finding itself is UNCHANGED and independently reproduced as
correct**: the reviewer's reproduction of all violations, confirmation they
are carried by genuine surnames, and confirmation `break.go` implements
exactly AD-25's two named constraints (no third, undocumented mechanism) all
stand. This report's corrections are about MEASUREMENT INTEGRITY (the
sample and two other guards), not about walking back that finding.

**Finisher's note (this story's second QA review, post-reopening):** a second
adversarial review verified all five blockers and all 19 findings above fixed,
by independent re-derivation, but found the Dev Agent Record itself still
narrating pre-reopening numbers as current fact (Blocker 1), 12 rulings
missing from the dispositions table (Major 2), the 27→26 delta untraced in
any artifact (Major 3), a stale representative-violations table in this very
report (Major 4), an over-broad CC0 licence classifier fallback (Major 1), and
D-000.17's anti-invention property enforced against only one past dodge
(Major 5). All five are FIXED by the finisher; see the corrections folded
into each section below and the Dev Agent Record's superseded/corrected
split. **One consequence worth flagging up front:** fixing Major 5 moved one
item ("ฉั่วสมบูรณ์", self-described only as "a plausible" surname) from
`personal_name`/`sourced` to `synthetic_probe` — it was not independently
attested, so D-000.17 forbids counting it as sourced. This changes several
counts below versus the second review's own figures (P5 personal-name floor,
P6e, P6g, P3), **while leaving P1 and P2 exactly unchanged** (the underlying
text and its break computation did not change, only its label) — every
change is named at its own row.

All figures below are computed by the harness
(`folio-go/internal/text/corpus_test.go`, `p2_independent_test.go`,
`s4_test.go`) from the corpus actually read
(`fixtures/thai-break-corpus/corpus.json`, **243 items: 204 sourced, 39
synthetic**), never narrated. Reproduce with:

```
cd folio-go && go test ./internal/text/... -run \
  'TestCorpusMeetsP5Floors|TestCorpusMeetsP6ExerciseFloors|TestP1NeverBreaksInsideCluster|TestP2NeverBreaksInsideUnknownRun|TestP2IndependentDPCrossCheck|TestP3ProperNounsNeverSplit|TestAC10ComputedBreaksMatchS4Basis' -v -count=1
```

## P1–P6, each named with its measured value (AC11)

| # | Condition | Threshold | Measured | Result |
|---|---|---|---|---|
| P1 | No break inside any Thai character cluster | Zero, absolute | **0 violations** (243 items) | **PASS** |
| P2 | No interior break in any dictionary-uncoverable run | Zero, absolute | **26 violations, 17 items** (independent DP ground truth) | **FAIL** |
| P3 | No hand-identified proper noun is split | Zero, absolute | **172 violations, 120/162 proper-noun items** (given names + surnames) | **FAIL** |
| P4 | Trie loads and queries correctly under `js/wasm` | Binary | Loads; probe queries + AC10 fixture match native | **PASS** |
| P5 | Corpus floor (sourced only) | ≥120/≥40/≥40/≥200 | 122/40/42/204 | **PASS** |
| P6 | Exercise floor (a–g) | see below | see below | **FAIL (P6g)** |

**P3's 172 (was 173) and P5's 122/204 (was 123/205) both changed for the same
reason: the finisher's Major 5 fix relabelled one item, `ฉั่วสมบูรณ์`, from
`personal_name`/`sourced` to `synthetic_probe` (see the preamble note above).
P2's 26/17 is unaffected — the item still appears in P2's violation list
under its new id, `synthetic-039` (P2 is computed over every Thai-script span
in the corpus regardless of category or provenance).**

### P5 — corpus floor, sourced items only (D-000.17)

| Category | Floor | Measured (sourced) |
|---|---|---|
| Personal names | ≥120 | **122** |
| Place names | ≥40 | **40** |
| Transaction descriptions | ≥40 | **42** |
| Total | ≥200 | **204** |

39 additional `synthetic_probe` items exist (38 obsolete-consonant tokens,
plus one relabelled name — see the preamble note), excluded from every count
above, kept only for P6a.

### P6 — exercise floor, computed

| # | Floor | Minimum | Measured | Result |
|---|---|---|---|---|
| P6a | Items with ≥1 dictionary-uncoverable run (all 243 items, sourced+synthetic — an uncoverable run is real regardless of provenance) | ≥60 | **64** (39 synthetic + 25 sourced; sourced-only would be **25 < 60**, below floor — the synthetics carry this row, exactly as D-000.17 sanctions) | PASS |
| P6b | Items with a vowel+tone-mark cluster | ≥30 | **63** | PASS |
| P6c | ...of which stacked (above-slot) vowel+tone | ≥10 | **16** | PASS |
| P6d | Items with an ACTUAL Latin letter or digit (fixed; was counting the space in every 2-token name) | ≥20 | **20** (zero margin — all 20 are `transaction_description`; no personal-name or place-name item mixes scripts, and removing/relabelling any one of these 20 would breach this floor) | PASS |
| P6e | Hand-identified proper nouns (given name + surname per personal-name item, plus place names; sourced only) | ≥160 | **284** | PASS |
| P6f | Sourced personal names: unconstrained matcher proposes ≥1 interior break on the surname | ≥90 | **115** (of 122 — see below for what this means measured the other way) | PASS |
| P6g | Sourced personal names: unconstrained matcher proposes none | ≥20 | **7** | **FAIL — reported, not filled (D-000.17)** |

**P6f, read the other way (the direct measurement of R2's mitigation — this
story calls P6f/P6g "the closest thing this spike has to a direct measurement
of the risk it exists to retire"):** on the SURNAME alone, the constrained and
unconstrained engines produce identical break sets for **119 of 122** sourced
personal names, differing on **3** (2.5%). On the FULL item text (given name +
surname together), they differ on **17 of 122** (14%). **That the two modes
agree on the overwhelming majority of surnames in isolation, yet diverge on
14% of full names, is the P3 finding restated from the constraint's own point
of view** — most of the time AD-25's override has nothing to do (the
unconstrained dictionary already proposes what the constrained engine
proposes), and P3's violations arise from context the surname-only view does
not see.

## P6g — the floor that could not be met from sourced data (a genuine finding, not a failure of effort)

**7** real Thai personal names satisfy P6g's literal criterion (the
unconstrained matcher proposes **zero** interior breaks on the surname,
either because nothing in the name matches any dictionary entry, or because
the whole surname happens to be listed as a single dictionary entry itself):

| Surname | Note | Whole dictionary entry? | Genuinely uncoverable (the hard path P2 fails on)? |
|---|---|---|---|
| ดอเลาะ | genuine Thai-Malay/Muslim regional surname (Southern provinces) | no | ✅ yes — independently attested |
| แนแซ | genuine Thai-Malay/Muslim regional surname | no | ✅ yes — independently attested |
| ชินวัตร | Shinawatra — whole-word dictionary match | ✅ yes | no — exercises the OPPOSITE path |
| จิราธิวัฒน์ | Chirathivat (Central Group) — whole-word dictionary match | ✅ yes | no — exercises the OPPOSITE path |
| หวั่งหลี | Wanglee — whole-word dictionary match | ✅ yes | no — exercises the OPPOSITE path |
| ประยูรวงศ์ | Prayurawong — whole-word dictionary match | ✅ yes | no — exercises the OPPOSITE path |
| ทวีสิน | Taveesin — whole-word dictionary match | ✅ yes | no — exercises the OPPOSITE path |

**Finisher's correction (second QA review, Minor 1 and Major 5).** An eighth
name, `ฉั่วสมบูรณ์`, previously appeared in this table labelled only as "a
plausible Sino-Thai family name" — plausible is not sourced, and D-000.17
forbids inventing items (including asserting an attestation that does not
exist) to reach a number. Rather than retroactively claim it was attested,
it has been relabelled `synthetic_probe` in `corpus.json` (id `synthetic-039`)
and is excluded from this table and from every genuine floor, the same as the
38 obsolete-consonant probes. **The aggregate that actually matters is not
7 — it is 2.** Five of the seven are whole-dictionary-entry matches that
satisfy P6g's literal wording but exercise the *opposite* polarity from the
one P6g exists to guard (nothing proposed, nothing to override — corroborated
by the fact that none of the five produces a P2 violation). Only `ดอเลาะ` and
`แนแซ` are both genuinely uncoverable AND independently attested real
surnames, and **only these two currently cover the path where P2 fails**
(`name-116` and `name-117` respectively — see `deferred-work.md` DW-11,
corrected in the same pass).

**Why 20 was not reached, stated as a finding about sourcing and the
language, not about effort:** the P6f measurement above (115/122 = 94% of
the sourced personal-name bucket decomposes into recognisable morphemes)
is not incidental — Thai naming convention systematically favours composing
surnames from meaningful, auspicious words. A survey of dozens of additional
candidates (major Thai-Chinese business dynasties: เวชชาชีวะ, อึ๊งภากรณ์,
ล่ำซำ, โสภณพนิช, เตชะอุบล, พรประภา, รัตนรักษ์, มาลีนนท์, and many more; a
broader set of Thai-Malay/regional forms; bare Sino-Thai surnames without the
"แซ่" clan prefix) turned up almost no additional zero-break cases — most
either decompose (joining P6f) or, for the "แซ่X" family, contain "แซ่"
itself as a recognised dictionary word that produces at least one break.
**Genuinely opaque real Thai surnames appear to be a genuine minority even
among names an English-language survey can identify**, and reaching 20 would
plausibly require either a real name database this developer does not have
access to, or accepting names this developer cannot personally verify as
real. Per the reopening's explicit instruction, this is reported as an
unmet floor rather than filled with more obsolete-consonant strings or
unattested names (which would reproduce exactly the defect being corrected).

## THE P3 FINDING (independently reproduced and confirmed correct)

**P1 holds with zero violations. P3 does not: 172 individual break
positions, across 120 of 162 proper-noun-bearing items (both given names and
surnames), are proposed by the CONSTRAINED engine strictly inside a
hand-identified proper-noun span.** (Was 173/121 of 163 before the
finisher's Major 5 correction relabelled one item out of the personal-name
population — see the preamble note; the underlying architectural finding is
identical.)

### Why this happens, mechanically (unchanged from the original finding)

AD-25's rule, verbatim: *"two constraints sit under whatever the dictionary
proposes, and both override it: unknown runs are atomic; no break inside a
Thai character cluster."* Neither constraint is about **proper-noun
identity**. A coined Thai name built from morphemes that are themselves
ordinary dictionary words (`ศรี` + `สุข`, `ทอง` + `บุญ`, and 113 more sourced
examples — 94% of the sourced personal-name bucket, P6f = 115/122) is, from
the dictionary's point of view, not an uncoverable run at all — the break at
the morpheme boundary is proposed exactly as it would be for any two
unrelated words placed next to each other.

**This finding was independently reproduced by the reviewer**: all 104
violations against the PRIOR (now-superseded) corpus were reproduced exactly,
confirmed to be carried entirely by genuine surnames, and `break.go` was
confirmed to implement exactly AD-25's two named absolutes with no
proper-noun concept anywhere in it. This developer's refusal to add a third
mechanism unilaterally was confirmed genuine. **The owner has since ruled
(D-2.1.6): the template will declare unbreakable fields, landing in Story
2.4.** This spike's finding is what produced that architecture change.

## THE P2 FINDING (new — a second, independent gap, of the same root cause)

**Ground truth (isFullySegmentable — a DP over the raw dictionary, with no
cluster constraint and no matching-order dependency: a token is
"uncoverable" iff no split of it into complete dictionary words exists at
all) disagrees with the CONSTRAINED engine's classification in 26 positions
across 17 items.**

Representative violations (full list: 26, in `TestP2IndependentDPCrossCheck`'s
log; the four below are re-generated directly from the current test output —
**Finisher's correction, second QA review, Major 4**: this table previously
carried four rows surviving from the pre-rebuild, superseded corpus, three of
which named the wrong text for their id):

| Item | Text | Span | Ground truth says | Engine did |
|---|---|---|---|---|
| name-021 | ชัยวัฒน์ วงศ์ไพร | "ชัยวัฒน์" (a GIVEN name) | not fully segmentable | proposed 3 interior breaks |
| name-026 | อรุณี ทองตระกูล | "อรุณี" (a GIVEN name) | not fully segmentable | proposed 1 interior break |
| name-116 | สุขสันต์ ดอเลาะ | "ดอเลาะ" (a P6g genuinely-uncoverable surname) | not fully segmentable | proposed 1 interior break |
| txn-006 | ค่าน้ำประปา กปน. | "กปน" (an abbreviation) | not fully segmentable | proposed 1 interior break |

A fifth item worth naming explicitly: `synthetic-039` (text `ฉั่วสมบูรณ์`,
relabelled from `personal_name` per Major 5 above) also violates — span
`[0,11]`, break at rune 4 — confirming its relabelling did not remove it from
this measurement, only from the genuine-name floors. **Story 2.4's retained
fixture** (D-2.1.9's table) is **re-pointed** from the pre-rebuild
`name-101`/`name-102` ids to the current, stable genuinely-uncoverable
sourced-surname ids: **`name-116` (`ดอเลาะ`) and `name-117` (`แนแซ`)** — two
ids, not three, now that the third candidate (`ฉั่วสมบูรณ์`) has been
relabelled synthetic and is no longer a sourced personal name at all. See
also `folio-go/internal/text/break_test.go`'s V11 comment, corrected the same
way (Nit 1).

**Several of these are given names, a different population from the 2
opaque-surname cases already known from P3.** The mechanism: the
constrained engine's atomic-run "resumption scan" — which looks forward past
an unmatched stretch for the next position at which SOME dictionary word
begins, so the unmatched middle can stay atomic — can land on a short,
spurious legal match embedded partway through a span that, as a WHOLE, has no
valid segmentation at all. That spurious partial match is enough to produce
an interior break, even though ground truth says the entire span should be
atomic.

**This is not calibrated to any particular number.** 26/17 is reported
because that is what the harness measures against the committed corpus
today; a future fix landing at a smaller number is still a fail unless it is
exactly zero, and "tuning" toward a specific non-zero figure would recreate
the self-referential failure this correction exists to close, at a new
value. **AC6 is therefore not fully met either** — the same architectural
gap P3 already surfaced (no proper-noun/atomicity-hint concept beyond
dictionary coverage) also produces occasional violations against a WORD, not
just a NAME, when a short spurious dictionary match happens to fall inside
an otherwise fully uncoverable span.

### The 27 → 26 delta, traced (D-000.19; finisher's correction, Major 3)

D-000.19 is binding and permits only two resolutions for an unexplained delta
between two independent computations: traced to a **named change**, or to an
**identified defect**. This story's first code review independently computed
**27** violations; the harness (and this story's second, independent
re-derivation) measures **26**. **Trace: the two figures were never measured
over the same population.** The first review's 27 was computed over the
**pre-reopening, 220-item corpus** (140 personal-name items, no
`Provenance`/synthetic split, no given-name proper-noun spans). D-000.17
mandated a full corpus rebuild; the current corpus has 243 items (123
sourced personal names at the time, now 122 after Major 5's relabelling),
built from different word lists with given-name spans added. Measured
independently on the current corpus (this story's second QA review's own
from-scratch Python DP, using its own tokeniser, not the engine's
`scriptSpans`): **26**, matching the harness exactly. **Resolution: traced to
the D-000.17 corpus rebuild — different populations, not drift or
calibration.** No further action follows from this trace; it exists so a
later reader does not see an unexplained 27-vs-26 and suspect a nudged
computation.

## What I did NOT do (unchanged from the original report)

I did not add a third mechanism to make P3 or P2 pass. I did not calibrate
the P2 independent check to a specific target number. I did not fill the
P6g floor with more synthetic strings. Both findings (P6g and P2) are
reported as real measurements — a corpus-provenance/sourcing finding (P6g)
and an engine finding (P2) — for the owner/engineering lead, not silently
resolved.

## Delivery Log (D-000.4)

**Measured, run in this story (`rtk proxy` first, then redirect):**

- `go build ./...`, `go vet ./...`, `gofmt -l .` — folio-go, lint (green
  after two `gofmt -w` fixes on newly-added test files); hashmatrix has a
  pre-existing, unrelated `go build ./...` quirk (`probe` package name
  collision with root output name), not introduced by this story.
- `folio-go` full suite, `-count=1`: **314 `--- PASS`, exactly 2 `--- FAIL`,
  0 `--- SKIP`** (re-measured by the finisher after Major 5's fix added one
  new passing test, `TestCorpusRegeneratedMatchesCommitted`; the second QA
  review measured 313/2/0 immediately before that test existed). **`internal/text`
  shows two — and only two — genuine, expected `--- FAIL` lines**:
  `TestCorpusMeetsP6ExerciseFloors` (P6g, reported unmet) and
  `TestP2IndependentDPCrossCheck` (P2, reported failing) — both intentional
  per this reopening's explicit instruction to let them go red. Every other
  package (`folio-go`, `cmd/gencorpus`, `internal`, `internal/bind`,
  `internal/fontset`, `internal/geom`, `internal/pdf`, `internal/template`) is
  `ok`.
- `lint` module, full suite, `-count=1`, including `GOPROXY=off`: green
  (`cmd/genmanifest`, `internal/licence`, `internal/manifest`,
  `internal/rules` — now including `wordlistassets_test.go`'s expanded
  subdirectory/missing-file red-proofs).
- `go build -tags=matrix ./...` / `go vet -tags=matrix ./...`: green.
- Licence manifest (`lint/MANIFEST.md`): regenerated; now correctly carries
  the CC0 wordlist row (`folio-go/internal/text/wordlist/words_th.txt |
  CC0-1.0 | ...`), fixed by adding a CC0-full-text fallback marker to
  `ClassifyLicenceText` (the committed `LICENSE-CC0-1.0.txt` is the full CC0
  1.0 Universal legal code, which needed the same kind of text-marker match
  the Apache/BSD/MIT fallbacks already use, not just the SPDX-line path).
  **Finisher's correction (second QA review, Major 1):** the fallback marker
  originally added also matched `"CREATIVE COMMONS CORPORATION IS NOT A LAW
  FIRM"`, boilerplate opening EVERY Creative Commons legal code family
  (CC BY, CC BY-SA, CC BY-NC, CC BY-ND, ...), not just CC0 — a CC BY-NC-SA
  (NonCommercial) text classified as permissive `CC0-1.0`. Narrowed to match
  only `"CC0 1.0 UNIVERSAL"`, which is sufficient for the committed file
  (verified: it appears on line 3). Three new test cases in
  `lint/internal/licence/classify_test.go` assert CC BY-NC-SA, CC BY-SA and
  CC BY-ND legal-code preambles all classify `FamilyUnknown` (fail the build,
  per D-1.3.8), not permissive — all three pass after the fix.
- **`js/wasm` leg (AC3/AC4), re-verified after the corpus rebuild**:
  `TestProbeQueries` and `TestAC10ComputedBreaksMatchS4Basis` both
  `--- PASS` under Node via `go_js_wasm_exec`, target confirmed
  `GOOS=js GOARCH=wasm`.

**Explicitly named as UNRUN, deferred to the Epic 2 boundary gate
(D-000.4, D-2.1.1):** `linux/amd64`, `linux/arm64` matrix legs.

## Additional review-finding disclosures (Findings 10, 11, 15, 16, 17, 19; second-review Majors 1–5, Minors 1–5)

**Finding 10 — AC5's "hand review" is mechanical labelling, not an independent
annotation pass, and the pre-stated disagreement rate is structurally zero.**
Disclosed rather than silently accepted: `corpus.json`'s proper-noun spans
were assigned by this developer while constructing the corpus (curated name
lists, morpheme compounds, and a small number of researched real family
names), not by a separate reviewer checking the ENGINE's proposed breaks
against independent judgement. `computed_breaks.json` is the engine's own
output. Because both are produced by (and checked against) the same
developer's labels, the "dictionary-disagreement rate" the story pre-stated
as illustrative (§"Recorded, not gated") has no independent signal to
measure — it is not computed here, and reporting a number would misstate
what it means. P3's 172 and P2's 26 are not derived from this rate; they are
direct comparisons of engine output against proper-noun/coverage
GROUND-TRUTH definitions (hand-identified spans; the independent DP
segmentability check respectively), not against a separately-reviewed
"correct" break set. This is named explicitly so the next reader does not
assume a human-annotated ground truth exists beyond what is described here.

**Finding 11 — S4-basis fixture provenance.** `fixtures/thai-break-corpus/README.md`
now states explicitly that `computed_breaks.json` is a cross-target
regression anchor only, never a correctness oracle, and that it is expected
to become stale once Story 2.4 implements D-2.1.6's declared-unbreakable-field
mechanism.

**Finding 15 — the `runtime.Caller` guard's actual reach.** `RuleRuntimeCaller`
(`lint/internal/rules/forbiddenimports.go`) is wired into `ScanForbiddenImports`,
whose production caller (`TestForbiddenImportsProductionScan`) scans
`folio-go/internal/` ONLY. V1's original framing ("`runtime.Caller` occurs
zero times **in the repository** today") is a repo-wide measurement; the
GUARD's enforcement is narrower — `folio-go/internal/**` only. Verified
（this story's dev record): injecting `runtime.Caller(0)` into
`folio-go/internal/text/data.go` fails the production scan (as designed);
injecting the identical call into `folio-go/cmd/gentrie/main.go` does
**not** — the whole `lint` suite stays green, `go vet ./cmd/...` stays
clean. This is a real gap in the guard's reach relative to V1's stated
scope, disclosed rather than silently left implied as closed. Widening it
(to the whole repo, or at least to `folio-go/` including `cmd/`) is a
separate decision, out of this story's scope to make unilaterally.

**Finding 16 — the place-040 label correction, quantified.** The
`placeNameSpanOverride` in `cmd/gencorpus/main.go` narrows `place-040`'s
("สาขาเซ็นทรัลเวิลด์") proper-noun span from `[0,18]` to `[4,18]` — "สาขา"
(branch) is not part of the place name. Measured effect on the CURRENT
corpus: the engine proposes exactly one break in this item, at rune 4.
Under the unlabelled span `[0,18]` that break is strictly inside and counts
as a P3 violation; under the corrected span `[4,18]` it lands exactly on the
span's own boundary and does not (still true after Major 5's relabelling,
which did not touch this item). The correction is disclosed with its exact
effect, per the review's request, rather than only asserted.

**Finding 17 — P6e's break-bearing subset.** Measured from
`computed_breaks.json`, break totals by category: `personal_name` **416**,
`transaction_description` **140**, `place_name` **1**, `synthetic_probe`
**1** (the relabelled `ฉั่วสมบูรณ์`/`synthetic-039` item, Major 5 — its own
break is unaffected by the relabelling, only its category changed). Of P6e's
284 hand-identified proper nouns (D-000.17: sourced only), the 40 place
names contribute at most one possible P3 violation between them (and, after
the Finding 16 correction, zero). **P3's evidence rests almost entirely on
the personal-name bucket** — stated explicitly here so the owner does not
read P6e's 284 as 284 items of equally-weighted evidence.

**Finding 19 — noted, no action taken (Nit, explicitly not requiring a fix
this story).** 611 of the wordlist's 62,106 distinct entries contain an
embedded space (measured). `isThaiScript` excludes U+0020, so `scriptSpans`
always splits on it, and no dictionary lookup this engine performs ever
queries a substring spanning a space — these 611 entries are unreachable
dead weight in the embedded 2,481,373-byte trie. Not acted on: correct
today, and worth reconsidering only if binary size becomes a real
constraint (e.g. once Story 2.2 adds font data to the same binary).

**Finding 20 (Major 5) — D-000.17's "may not invent items" property, closed
against the class rather than one past dodge.** The second QA review found
the obsolete-consonant bar (`containsObsoleteConsonant`) applied to only two
of the five sourced buckets (`sourcedDecomposableSurnames`,
`sourcedOpaqueSurnames`), that nothing gated `corpus.json` against
`cmd/gencorpus`'s own output (so a hand-edit adding an invented item bypassed
every check), and that `ฉั่วสมบูรณ์` was a live, self-declared-"plausible"
instance of exactly the dodge the ruling forbids. Three fixes: (a) the bar
(renamed `checkNoObsoleteConsonant`) now covers all five sourced buckets
(given names, both surname lists, place names, transaction descriptions);
(b) a new test, `folio-go/cmd/gencorpus/main_test.go`'s
`TestCorpusRegeneratedMatchesCommitted`, regenerates the corpus from
`buildItems()` and compares it structurally against the committed
`corpus.json` — mirroring `internal/text`'s
`TestTrieRegeneratedMatchesCommitted` precedent — so a hand-edited corpus now
reddens a gate rather than passing silently; (c) `ฉั่วสมบูรณ์` is relabelled
`synthetic_probe` rather than retroactively asserted as attested (see Finding
21 / P6g above and `deferred-work.md` DW-11).

**Finding 21 (Major 2) — the ruling dispositions table completed.** The
Dev Agent Record's dispositions table stopped at `D-2.1.4`, omitting twelve
rulings issued during the reopening, including `D-000.17` and `D-000.18` —
the two standing rules that caused the reopening — and `D-2.1.13`/`D-2.1.14`,
which specify most of the work this pass delivered. The finisher extended
the table to all 36 rulings; see the Dev Agent Record's corrected
dispositions table in the story file.

**Finding 22 (Minor 3) — P6d clears its floor with zero margin.** P6d = 20
against a floor of ≥20, and all 20 qualifying items are `transaction_description`
— no personal-name or place-name item mixes scripts (`place-040` is the one
branch-style name and contains no Latin letter or digit). The floor is
honestly met, but removing or relabelling any single transaction description
would breach it. Recorded here (and in the P6 table above) so a later corpus
edit does not silently breach a pre-stated floor; no code change, per the
review's own suggested resolution.

**Finding 23 (Minor 4) — Task 14's disagreement-rate clause: not computed,
with a structural reason, and a `DECISION NEEDED` this developer/finisher
does not resolve.** Task 14 asked for "the disagreement rate against its
pre-stated escalation thresholds." As Finding 10 already discloses, that rate
is structurally zero — `corpus.json`'s proper-noun spans and
`computed_breaks.json`'s engine output are not two independently-produced
annotations to compare, so a computed "rate" would misstate what it measures.
The pre-stated thresholds (>15% of items; any item with >3 disagreements) are
never restated because there is no rate to hold them against. **This is a
partial delivery of Task 14, named as such rather than left implied as
complete.** Separately: whether "AC5's hand review is mechanical labelling
rather than an independent annotation pass" changes what evidentiary weight
P3's 172 carries is a genuine, unresolved question about what this spike's
finding actually establishes — routed here as an open `DECISION NEEDED` for
the engineering lead/owner, per D-2.0.2's meta-rule (reclassifying a
pre-stated deliverable is not this developer's or finisher's judgement call
to make silently). It does not change P1–P6's measured values above.

## Vacuity register — dispositioned (V1–V11), reopening corrections noted

| # | Disposition |
|---|---|
| V1 | Unchanged: `RuleRuntimeCaller` guard, red-proofed by injection at the real location. |
| V2 | Unchanged: anchored on AD-1's existing `os` import ban. |
| V3 | **Corrected**: floors now assert against SOURCED distinct-item counts (D-000.17); a floor short of its minimum is reported failing (`TestCorpusMeetsP6ExerciseFloors` now correctly fails on P6g), never silently met by non-genuine items. |
| V4 | P6a (64 = 25 sourced + 39 synthetic; sourced-only would be 25 < 60, below floor) reported independently of the P2 result — moot as a vacuity hazard either way, since P2 reports 26, not 0 (**Minor 2, finisher's correction**). |
| V5 | P6b (63) / P6c (16) likewise. |
| V6 | Unchanged: `TestInternalTextJSWasmLeg` asserts on the logged `GOOS=js GOARCH=wasm` line. |
| V7 | Unchanged: two-direction red-proof for `CC0-1.0`; narrowed by Major 1's fix so it no longer over-matches the whole CC family (three new red-proof cases, `classify_test.go`). |
| V8 | This report names all six of P1–P6 with measured values, honestly (two now FAIL). |
| V9 | **Corrected**: `ScanWordlistAssets` now also (a) recurses into subdirectories and (b) fails on a MISSING required file, not just an extra unaccounted one — both red-proofed live, at the real location, in both directions. |
| V10 | Unchanged: non-zero total break count, every item has a fixture entry (243/243). |
| V11 | **Corrected**: the switch guard now asserts the two modes' results actually DIFFER for its example input (not merely that one is non-empty) — verified, by injection, to catch a hard-coded `unconstrained = false`, which the ORIGINAL version of this guard did not. **Corpus-wide figure added (Minor 5, finisher's correction)**: see the P6f row above — the two modes agree on 119 of 122 sourced surnames (97.5%) and disagree on 17 of 122 full item texts (14%). |

## Decision routing (per the story's own "Who decides" table)

**Outcome: DEVIATION, unchanged in kind, corrected in measurement
integrity.** P3 (confirmed, unchanged) and now P2 (a second, independent
finding of the same root cause) both fail as pre-stated absolutes. P6g (a
corpus floor) is honestly reported unmet from sourced data — a scope/
sourcing question, not an engine defect, but still requires the owner's
attention since S4's personal-name population would otherwise under-
represent the "genuinely opaque" polarity P6g exists to guarantee.

This developer does not decide the resolution to any of these. Routed as an
updated `DECISION NEEDED` to the engineering lead / owner.
