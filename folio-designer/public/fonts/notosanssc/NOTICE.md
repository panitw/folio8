# NotoSansSC-Regular.ttf — shipped production face (Simplified Chinese)

**Copyright 2014-2021 Adobe (http://www.adobe.com/), with Reserved Font Name 'Source'**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt`
in this directory, the unmodified upstream licence text).

> **This file is a DERIVATIVE of the upstream release, not the upstream
> file itself.** It is a single static instance produced from the
> upstream *variable* build by the invocation recorded below. Everything
> needed to replay that derivation and reproduce these exact bytes is in
> this document.

This is one of Story 2.2's three shipped production faces (AC1, AC9):
folio-go's bundled `FontSet`, keyed as `"Noto Sans SC"`, providing Simplified Chinese (CJK) glyph coverage
for the `fonts` fallback chains a `.folio` document declares (see
`folio-format.md`'s `fonts` example,
`["Noto Sans", "Noto Sans Thai", "Noto Sans SC"]`).

It is a **static** TrueType font — `glyf` outlines, **no** `fvar`/`gvar`/
`avar`, no `CFF2` — at `OS/2.usWeightClass` **400 (Regular)**. NFR7's
operative choice, glyf/TrueType over CFF/OpenType, is met and reinforced;
NFR7's *"variable"* adjective was amended under D-000.6 (see D-2.2.5),
because the variable build cannot meet NFR7's own font budget.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/google/fonts`, commit `2894aab31764f10f29c421bdfd2340d3b382d384` (2022-12-09, *"Noto Sans SC hotfix2 (#5533)"*) |
| Upstream file | `ofl/notosanssc/NotoSansSC[wght].ttf`<br>(committed during development as `NotoSansSC-VF.ttf`, byte-identical) |
| Fetched | 2026-08-23; **provenance corrected 2026-08-25** — see the correction note below |
| **sha256 of the SOURCE (upstream) file** | `a3041811a78c361b1de50f953c805e0244951c21c5bd412f7232ef0d899af0da` |

### Provenance correction, 2026-08-25

**This row previously named `github.com/notofonts/noto-cjk` at commit
`523d033d6cb47f4a80c58a35753646f5c3608a78`. That was wrong**, and it was found the first time anyone
tried to replay the derivation for the Epic 2 boundary gate.

Measured, not inferred:

- That repository **at that commit contains no `NotoSansSC[wght].ttf` at all.** Its whole tree carries
  exactly one SC TrueType variable font — `Sans/Variable/TTF/Subset/NotoSansSC-VF.ttf` — and that file
  hashes to `d68bafcb…`, **not** the recorded source hash.
- The file that **does** hash to the recorded `a3041811…` is `ofl/notosanssc/NotoSansSC[wght].ttf` in
  **`github.com/google/fonts`**, confirmed at the pinned commit above and not merely on `main`.
- `NotoSansSC[wght].ttf` is the **Google Fonts** naming convention, which is the tell the original row
  contradicted itself on: it named a noto-cjk commit while quoting a google/fonts filename.

**The recorded sha256 was CORRECT throughout, so the shipped face is the intended file** — the defect
was the pointer to where it came from, not the artifact. `TestShippedFacesReproduceFromUpstream` now
passes on all three faces with this corrected source.

**The other two faces' provenance was verified in the same pass and is correct as written** — both
release zips resolve, and in each the file matching the recorded hash is the `googlefonts` variable
build (`NotoSans[wdth,wght].ttf`, `NotoSansThai[wdth,wght].ttf`).

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `NotoSansSC-Regular.ttf` |
| **sha256 of the SHIPPED (produced) file** | `5ef5755b1ac6502180985a2aba6ef0b42d6663829f6f653898ced8411a060158` |
| Size | 10,595,932 bytes |
| Instance | Regular — `wght=400` — this face has ONE axis, and its **default is 100 (Thin)**, which is why the pin is explicit and why D-000.21 exists |

## The exact invocation, verbatim

```sh
SOURCE_DATE_EPOCH=1451606400 fonttools varLib.instancer --update-name-table \
    NotoSansSC-VF.ttf wght=400 -o NotoSansSC-Regular.ttf
```

`--update-name-table` is required: without it the instanced Regular keeps
naming itself after the axis default, so `name[1]`/`name[2]`/`name[6]`
would still read the wrong weight.

## Reserved Font Name — checked, and it clears

This face's licence declares the Reserved Font Name **'Source'** (it
descends from Adobe's Source Han Sans). OFL 1.1 §3 forbids a **derivative**
from being distributed under a Reserved Font Name, and this file *is* a
derivative — so the question is live rather than theoretical.

Our face is named **`Noto Sans SC`** (`name[1]`), with PostScript name
`NotoSansSC-Regular` (`name[6]`). Neither contains 'Source', so the
derivative clears the RFN restriction.

The Latin and Thai faces declare **no** Reserved Font Name, so the
question does not arise for them.

## Toolchain this derivation is pinned to

| item | value |
|---|---|
| Python | `3.12.13` |
| fontTools | `4.63.0` |
| `SOURCE_DATE_EPOCH` | `1451606400` (2016-01-01T00:00:00Z) |

`SOURCE_DATE_EPOCH` is load-bearing, not hygiene: fontTools writes
`head.modified` from the wall clock unless it is set, and that drags
`head.checkSumAdjustment` with it, so without it **every regeneration
produces different bytes**.

`tools/fontgen/instance_faces.py` performs and re-verifies this
derivation; `folio-go/fontgen_matrix_test.go` (`//go:build matrix`)
proves the committed file still reproduces from the upstream source.

AD-26, verbatim: "Redistributed non-code assets keep their own terms and
their notices." This directory's `LICENSE-OFL.txt` and this NOTICE
together satisfy that for the shipped file, and are what
`lint/internal/manifest.ResolveAssets` (AC25) reads to attribute the
face in `lint/MANIFEST.md`'s redistributed non-code assets table.
