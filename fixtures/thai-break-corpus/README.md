# Thai break-opportunity corpus (Story 2.1)

This directory holds Story 2.1's (Thai break-opportunity spike) evaluation
corpus and its measured output. Read `SPIKE-REPORT.md` first for the full
methodology and findings.

## Files

- **`corpus.json`** — 243 hand-constructed items (204 sourced, 39 synthetic
  probe tokens — see each item's `provenance` field), each with
  hand-identified proper-noun spans where applicable. (Finisher correction,
  this story's second QA review, Major 5: one item originally counted as
  sourced — "ฉั่วสมบูรณ์", self-described only as "a plausible" surname —
  was relabelled `synthetic_probe` rather than retroactively asserted as
  attested; see `SPIKE-REPORT.md` and `deferred-work.md` DW-11.)
- **`computed_breaks.json`** — **a cross-target REGRESSION ANCHOR ONLY, never
  a correctness oracle.** It is the CONSTRAINED engine's own raw output
  (`folio-go/internal/text.ComputeBreaks`, `unconstrained=false`), checked in
  so `TestAC10ComputedBreaksMatchS4Basis` can confirm every build target
  (native, `js/wasm`, and — at the Epic 2 gate — `linux/amd64`/`linux/arm64`)
  computes byte-identical break positions. **It is NOT a correctness label
  set.** It encodes exactly the breaks the spike report's P3 and P2 findings
  measured as WRONG (173 and 26 respectively) — that is the whole point:
  AC10 answers "does every target agree with itself", never "is the answer
  right". A future reader must not promote this file to the frozen S4
  conformance fixture (AD-21) without first re-deriving it against whatever
  mechanism Story 2.4 adds per **D-2.1.6** (the owner's ruling: the template
  will declare unbreakable fields) — this file predates that mechanism and
  is guaranteed to change once it lands.
- **`SPIKE-REPORT.md`** — the full measurement report: every P1–P6 condition
  named with its value, the P3 and P2 findings, the P6g sourcing-budget
  finding, and the routed `DECISION NEEDED`.

## Regenerating

From the `folio-go/` module root:

```
go run ./cmd/gencorpus   # rebuilds corpus.json from the curated word lists
go run ./cmd/genbreaks   # recomputes computed_breaks.json from corpus.json
```

Ordering is load-bearing (Trap 1, this story): corpus construction and its
P1/P2/P3 measurement come first; `computed_breaks.json` is generated from
that already-measured engine output, never the reverse.
