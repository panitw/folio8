# NotoSansThai-Regular.ttf — shipped production face (Thai)

**Copyright 2022 The Noto Project Authors (https://github.com/notofonts/thai)**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt`
in this directory, the unmodified upstream licence text).

> **This file is a DERIVATIVE of the upstream release, not the upstream
> file itself.** It is a single static instance produced from the
> upstream *variable* build by the invocation recorded below. Everything
> needed to replay that derivation and reproduce these exact bytes is in
> this document.

This is one of Story 2.2's three shipped production faces (AC1, AC9):
folio-go's bundled `FontSet`, keyed as `"Noto Sans Thai"`, providing Thai glyph coverage
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
| Upstream project | `github.com/notofonts/thai`, release `NotoSansThai-v2.002`<br>`https://github.com/notofonts/thai/releases/download/NotoSansThai-v2.002/NotoSansThai-v2.002.zip` |
| Upstream file | `NotoSansThai/googlefonts/variable/NotoSansThai[wdth,wght].ttf`<br>(committed during development as `NotoSansThai-VF.ttf`, byte-identical) |
| Fetched | 2026-08-23 |
| **sha256 of the SOURCE (upstream) file** | `5a1c559bb539583c8a1fd99d1c5b9491e5e14478c9cd2bd0970d5c3096cc9ef8` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `NotoSansThai-Regular.ttf` |
| **sha256 of the SHIPPED (produced) file** | `c94562c15cbff8c9af93042adb1c63981b5deeeba40693ea8d98cd3b33b73caf` |
| Size | 47,788 bytes |
| Instance | Regular — `wght=400`, `wdth=100` (this face has TWO axes) |

## The exact invocation, verbatim

```sh
SOURCE_DATE_EPOCH=1451606400 fonttools varLib.instancer --update-name-table \
    NotoSansThai-VF.ttf wght=400 wdth=100 -o NotoSansThai-Regular.ttf
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
