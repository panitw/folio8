# Fixture: minimal-rect

This fixture is the golden record for Story 1.1 — the first PDF `folio-go` ever produced: a
single A4 page containing one filled rectangle, rendered with no compression, no `/Info`
dictionary, and no `/CreationDate` or `/ModDate` (AD-7).

## Contents

- `expected.json` — the normative record. It carries the SHA-256 of the rendered bytes, the
  `folio-go` version (`folio8GoVersion`) and the exact Go toolchain version (`goToolchain`) that
  produced the hash. The hash is what tests compare against; the toolchain field exists so a test
  can detect and refuse a toolchain drift (see below) rather than silently re-measuring against it.
- `expected.pdf` — the recorded bytes, kept for human diffing only. **The hash in `expected.json`
  is normative, not this file** — do not "fix" a mismatch by eyeballing the PDF.

## If this fixture's hash test goes red

Under AD-21 and AD-22, a hash change here is **a defect until proven to be an intended, versioned
change**. Do not regenerate this fixture to make a failing test pass (vacuity guard 4 in Story
1.1). Investigate what changed in the rendering pipeline first. Only re-record `expected.json` /
`expected.pdf` deliberately, as part of a change that is explicitly documented as a byte-identity
breaking event (e.g. a Go toolchain bump under AD-22, or a deliberate format change), never as a
routine "make CI green" step.

If `runtime.Version()` at test time differs from the recorded `goToolchain`, the test that reads
this fixture fails on that mismatch specifically (before ever comparing hashes), with a message
explaining that a toolchain bump is a versioned breaking change under AD-22 and must be
re-measured deliberately — this is counter-metric C6.

## Provenance — external structural validation (D-000.53)

No golden artifact is accepted — first recording **or re-recording** (D-000.44) — until a reader this
project did not write parses it and resolves it into the semantic objects it claims to contain.

| | |
|---|---|
| reader | `qpdf` **12.4.0** |
| invocation | `qpdf --check fixtures/minimal-rect/expected.pdf` |
| result | exit **0** — `No syntax or stream encoding errors found` |
| invocation | `qpdf --show-npages fixtures/minimal-rect/expected.pdf` |
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
