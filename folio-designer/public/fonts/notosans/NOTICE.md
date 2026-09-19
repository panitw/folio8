# NotoSans-Regular.ttf — shipped production face (Latin)

**Copyright 2022 The Noto Project Authors (https://github.com/notofonts/latin-greek-cyrillic)**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt`
in this directory, the unmodified upstream licence text).

> **This file is a DERIVATIVE of the upstream release, not the upstream
> file itself.** It is a single static instance produced from the
> upstream *variable* build by the invocation recorded below. Everything
> needed to replay that derivation and reproduce these exact bytes is in
> this document.

This is one of Story 2.2's three shipped production faces (AC1, AC9):
folio-go's bundled `FontSet`, keyed as `"Noto Sans"`, providing Latin (and Greek/Cyrillic) glyph coverage
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
| Upstream project | `github.com/notofonts/latin-greek-cyrillic`, release `NotoSans-v2.015`<br>`https://github.com/notofonts/latin-greek-cyrillic/releases/download/NotoSans-v2.015/NotoSans-v2.015.zip` |
| Upstream file | `NotoSans/googlefonts/variable-ttf/NotoSans[wdth,wght].ttf`<br>(committed during development as `NotoSans-VF.ttf`, byte-identical) |
| Fetched | 2026-08-23 |
| **sha256 of the SOURCE (upstream) file** | `bfb7bb691513f12e734dc346c03a03f784912432d7e3fa8e56efcf906fe86b3d` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `NotoSans-Regular.ttf` |
| **sha256 of the SHIPPED (produced) file** | `a4c811314da2ade3b4d2f44e80cda21dd2624e50be82dcefa298dc45a3c92d6c` |
| Size | 646,160 bytes |
| Instance | Regular — `wght=400`, `wdth=100` (this face has TWO axes) |

## The exact invocation, verbatim

```sh
SOURCE_DATE_EPOCH=1451606400 fonttools varLib.instancer --update-name-table \
    NotoSans-VF.ttf wght=400 wdth=100 -o NotoSans-Regular.ttf
```

`--update-name-table` is required: without it the instanced Regular keeps
naming itself after the axis default, so `name[1]`/`name[2]`/`name[6]`
would still read the wrong weight.

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
