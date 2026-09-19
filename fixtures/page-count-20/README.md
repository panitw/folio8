# `page-count-20` — `Page X of Y`, crossing the digit-count boundary

**Story 2.7** (FR31 · AD-4). Golden digest
`b32fa1c5babb8327b09b5c2bc0a11628b8c8885b9c5661c0262ec24920c5150f`.

## Why this fixture exists

`epics.md`'s fourth Story 2.7 acceptance criterion (true range `927-929`, corrected from the
`923-926` this file originally cited — this story's review, Finding 6) requires documents of 1,
5, 20 and 50 pages, `Page X of Y` correct throughout, hashes matching recorded goldens on all
four targets. Measured at this story's
creation: no existing fixture spans **10 or more pages**, so none can express the defect this
construct is actually hard to get right — a page number whose **digit count changes** between
page 9 and page 10 (`Page 9 of 20` is 11 characters; `Page 10 of 20` is 12). 20 pages is the
smallest of the epic's four sizes that crosses that boundary.

## What this document's content can express

- **Twenty pages, deterministically.** Twenty single-line content elements, each placed exactly
  one content-band window (727.89 pt) below the last, so D-2.6.1's sliding window places
  exactly one per page — this is a **construction property**, not tuned line-wrapping, and it
  cannot silently drift to a different page count.
- **The `{{page}}`/`{{pages}}` construct**, in the page footer, resolved on every page: `Page 1
  of 20` through `Page 20 of 20`. D-2.7.2's reservation is `digits(20) == 2` digit-advances,
  right-aligned — pages 1–9 draw one digit inside a two-digit slot, pages 10–20 fill it exactly.
- **A populated page header**, a fixed literal (`PAGE COUNT MATRIX FIXTURE`), so header/footer
  confusion is readable in the text, matching `multi-page`'s precedent.

## What it cannot express

- **All-Latin, single-face, no image.** It says nothing about shaping, coverage or subsetting
  beyond what `multi-page` and `shaped-text` already cover.
- It does not exercise `{{page}}` wrapping to a second line, or a slot resolved inside an
  element narrow enough to force a line break.

## Independent-reader acceptance (D-000.53)

No golden this story records is accepted until a reader this project did not write resolves it
into the objects it claims to contain.

**Reader**: `qpdf` **12.4.0** (`/opt/homebrew/bin/qpdf`).

**Validated at**: `ecd0056` (Story 2.7's baseline commit) and re-run, with identical output,
during this story's finishing pass — matching `fixtures/multi-page/README.md`'s sibling row
(this story's review, Finding 15).

**Invocation and output, verbatim:**

```
$ qpdf --check fixtures/page-count-20/expected.pdf
checking fixtures/page-count-20/expected.pdf
PDF Version: 1.7
File is not encrypted
File is not linearized
No syntax or stream encoding errors found; the file may still contain
errors that qpdf cannot detect

$ qpdf --show-npages fixtures/page-count-20/expected.pdf
20
```

`qpdf --check` resolves the file's cross-reference table and object graph independently of
folio8's own writer and reports no structural defect; `qpdf --show-npages` resolves the page
tree and reports **20**, matching this document's declared page count exactly.

## Matrix registration

Registered in `matrixDocuments` (`folio-go/matrix_test.go`) as the Epic 2 gate's **sixth**
obligation (D-2.7.4, on D-2.6.2's criterion: FR31 had no cross-target artifact before this
entry). Cross-target legs are **deferred to the gate** (D-000.4's override criterion declined —
page-number substitution is integer advance arithmetic on `geom.Length`, no float, vendor call,
compressor or new dependency).

**Native leg run once before this story reached `review` (D-000.54)**:

```
$ FOLIO8_MATRIX_TARGET=darwin/arm64 go test -tags=matrix -count=1 -run TestTargetRenderHash .
--- PASS: TestTargetRenderHash
```

This proves the leg **executes and produces a hash on one target, `darwin/arm64`** — matching
this fixture's recorded digest exactly. It proves **nothing about cross-target agreement**,
which remains the gate's job.
