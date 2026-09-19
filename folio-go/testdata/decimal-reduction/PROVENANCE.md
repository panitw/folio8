# Provenance — Story 3.1a Layer-1 oracle golden (`golden.json`)

AD-25's one-time offline reference run, hand-checked, never a runtime dependency — D-2.3.1's precedent,
applied here the way it was applied to shaping via `hb-shape`. This file and `produce_golden.py` are
read by humans; no Go code imports or executes either.

## Producer

- Tool: system `python3`
- Version (`python3 --version`, verbatim): `Python 3.12.13`
- Producing invocation (verbatim, run from this directory):
  ```
  python3 produce_golden.py > golden.json
  ```
- Explicit context used (verbatim from `produce_golden.py`):
  ```python
  CONTEXT = Context(prec=50, rounding=ROUND_HALF_EVEN)
  ```
  `prec=50` is chosen to be far in excess of any value this corpus can produce (corpus A's largest
  result, `avg`, carries 18 significant digits), so the context's precision never itself becomes the
  bottleneck the oracle is trying to measure. `ROUND_HALF_EVEN` is AD-23's own division rounding mode
  ("divides at a defined scale with round-half-to-even"), applied identically to producing the golden
  and (once built) to `AvgDecimals`.

## Corpus provenance (D-000.50 population check — measured before any assertion was written)

Three candidate corpora were measured against an HONEST float64 mutant (accumulate in `float64`, THEN
round to the operation's declared scale — a mutant that skips the final rounding manufactures a
divergence the real implementation would have rounded away):

| corpus | shape | `sum` reddens under the honest float64 mutant? | `avg` reddens? |
|---|---|---|---|
| **A** — `12345678901234.56` + 32 × `0.01` (n=33) | large balance near float64's integer-exactness limit (2⁵³ ≈ 9.007×10¹⁵) absorbing 32 addends smaller than its ulp | **YES** | **YES** |
| **B** — `100.00, 0.0001, 3, 0.000007, 2.5` (n=5) | mixed scale; exercises the alignment path A never touches | no (rounding erases a pre-quantisation divergence) | no |
| **C** — `0.10 0.20 0.30 1.15 2.35 0.05 0.07` (n=7) | the intuitive small-money bank-statement fixture | no | no |

Measured (float64 mutant = accumulate in float64, then round to the declared scale):

```
A: sum exact = 12345678901234.88     float64 mutant = 12345678901234.87    -> REDDENS
   avg exact = 374111481855.602424   float64 mutant = 374111481855.602230  -> REDDENS
B: sum exact = 105.500107            float64 mutant = 105.500107           -> identical
   avg exact = 21.1000214000         float64 mutant = 21.1000214000        -> identical
C: sum exact = 4.22                  float64 mutant = 4.22                 -> identical
   avg exact = 0.602857              float64 mutant = 0.602857             -> identical
```

**Corpus C is the finding that matters**: it is exactly what a reasonable person writes when asked for a
bank-statement fixture, and it is byte-identical under an honest float64 accumulator for both `sum` and
`avg`. An oracle built on it alone would have read as sound in review and caught nothing — D-000.50's
hazard, reproduced on this story's own subject. **Corpus B is a second, subtler trap**: before
quantisation its `avg` appears to diverge (Python reports `21.1000214000` against float64's
`21.1000214`), which looks like real precision loss; rounding the mutant's result to the declared scale
restores the trailing zeros and the two agree exactly. Both B and C ship — labelled **non-discriminating,
as evidence** (D-000.29: settled, never carried) — as the proof that the obvious fixture proves nothing.
B is retained because it is the only member exercising alignment across differing operand scales, which
A (all operands at scale 2) does not touch at all.

## Independent reader (D-000.53)

`/usr/bin/bc` — a tool this project did not write and which did not produce the golden — resolves each
corpus's `sum` independently, forward and in reverse operand order (the reversal doubles as F4's
order-invariance evidence: exact addition is order-invariant, so an independent tool run twice on the
same multiset in different orders must agree with itself and with the golden).

- Reader: `/usr/bin/bc`
- Version (`/usr/bin/bc --version`, first line, verbatim): `bc 7.0.3`

Verbatim invocations and outputs:

CORRECTION (QA review Finding 9, Minor): the two corpus-A invocations below used to read
`12345678901234.56+0.01*32` — a multiplication collapsing the 32 literal `"0.01"` addends into one
term, not a resolution of the recorded 33-operand sequence itself. The conclusion was still sound (both
orders land on the golden's `12345678901234.88`), but for the one member that actually discriminates,
`bc` never performed the accumulation whose exactness is under test. Replaced with the literal 33-term
forward and reversed sums, re-run and re-recorded verbatim:

```
$ echo "scale=2; 12345678901234.56+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01" | /usr/bin/bc
12345678901234.88

$ echo "scale=2; 0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+0.01+12345678901234.56" | /usr/bin/bc
12345678901234.88

$ echo "scale=6; 100.00+0.0001+3+0.000007+2.5" | /usr/bin/bc
105.500107

$ echo "scale=6; 2.5+0.000007+3+0.0001+100.00" | /usr/bin/bc
105.500107

$ echo "scale=2; 0.10+0.20+0.30+1.15+2.35+0.05+0.07" | /usr/bin/bc
4.22

$ echo "scale=2; 0.07+0.05+2.35+1.15+0.30+0.20+0.10" | /usr/bin/bc
4.22
```

Every `bc` total matches the corresponding `golden.json` `sum` field exactly, in both operand orders.
Python's `decimal` module did NOT serve as its own acceptance reader (D-000.53's whole point) — `bc` is
an independent implementation this project did not write, and it did not produce the golden.

`bc` is not used to cross-check `avg`: `bc`'s division truncates rather than rounding half-to-even, so it
is not a faithful independent implementation of AD-23's declared rounding mode. `avg`'s acceptance
instead rests on the order-invariance property (D-000.22 / D-000.61, asserted in Go against the golden
itself, computed from the already-verified `sum`) and on the kernel's oracle comparison test (AC8),
never on `bc`.

## D-000.22 semantic acceptance step (first-recording obligation)

This is a FIRST recording of this golden — under D-000.22, this is the only moment "is this right?" is
answerable at all by re-deriving it, so the recording must be checked against a property read off the
artifact, not restated as a second copy of the same hash. This is asserted in Go (`golden_test.go`,
`TestGoldenIsOrderInvariant`) two ways: the recorded `sum` for every corpus member matches an exact
addition **re-derived from scratch** (not via the kernel this golden validates, not restated from
`produce_golden.py`), and the recorded `sum` scale equals the maximum operand scale of its own corpus
(AC11), read off `golden.json` alone.

CORRECTION (QA review Finding 8, Minor): this section used to name the test's forward/reverse
comparison itself as the acceptance property, per D-000.61's rule that a red-proof must vary operand
order (forced by measuring that a single-order float64 red-proof passed BY LUCK on corpus A's reversed
order). That comparison is real code and stays, but it accumulates in `big.Int`, which is commutative
by construction — the clause cannot fail for any input, confirmed empirically (it passed unchanged
under the float64 mutation that reddened every other assertion in the file). Exact addition genuinely
is order-invariant and float64 addition genuinely is not — that fact is what D-000.61 is about, and it
is what the AC10 red-proof above demonstrates on the kernel's own mutant — but the order-invariance
*clause inside this specific test* is not what discharges D-000.22 here. The independent re-derivation
against the recorded `sum`, and the scale check, are what do.
