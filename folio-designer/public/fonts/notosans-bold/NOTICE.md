# NotoSans-Bold.ttf — shipped production face (Latin, Story 11.1)

**Copyright 2022 The Noto Project Authors (https://github.com/notofonts/latin-greek-cyrillic)**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt`
in this directory, the unmodified upstream licence text).

> **This file is a DERIVATIVE of the upstream release, not the upstream
> file itself.** It is a single static instance produced from the
> upstream *variable* build by the invocation recorded below. Everything
> needed to replay that derivation and reproduce these exact bytes is in
> this document.

Story 11.1's browser-side copy of the engine's shipped face. It is **byte-identical** to
`folio-go/fonts/notosans-bold/NotoSans-Bold.ttf` on purpose: the canvas paints the face the engine resolved, so the
browser must hold the same bytes the engine measured with.

It is a **static** TrueType font — `glyf` outlines, **no** `fvar`/`gvar`/`avar`, no `CFF2`.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/notofonts/latin-greek-cyrillic`, release `NotoSans-v2.015` |
| Download URL | https://github.com/notofonts/latin-greek-cyrillic/releases/download/NotoSans-v2.015/NotoSans-v2.015.zip |
| Path inside the archive | `NotoSans/googlefonts/variable-ttf/NotoSans[wdth,wght].ttf` |
| sha256 of the release archive | `0c34df072a3fa7efbb7cbf34950e1f971a4447cffe365d3a359e2d4089b958f5` (117,491,253 bytes) |
| Local source filename | `NotoSans-VF.ttf` (in the gitignored `.font-sources/`, byte-identical to the archive path above) |
| Fetched | 2026-09-06 |
| **sha256 of the SOURCE (upstream) file** | `bfb7bb691513f12e734dc346c03a03f784912432d7e3fa8e56efcf906fe86b3d` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `NotoSans-Bold.ttf` |
| **sha256 of the SHIPPED (produced) file** | `652b4b154d1c41f01de4c69b6d37d6a73a1c942e43bfcf4f95d4490b2fca6787` |
| Size | 648,284 bytes |
| Instance | Bold — `wght=700`, `wdth=100` (this face has TWO axes) |
| Declared PostScript name (`name[6]`) | `NotoSans-Bold` |

## The exact invocation, verbatim

```sh
SOURCE_DATE_EPOCH=1451606400 fonttools varLib.instancer --update-name-table \
    NotoSans-VF.ttf wght=700 wdth=100 -o NotoSans-Bold.ttf
```

`--update-name-table` is required: without it the instanced face keeps naming itself after the axis
default, so `name[1]`/`name[2]`/`name[6]` would still read the wrong weight.

## Toolchain this derivation is pinned to

| item | value |
|---|---|
| Python | `3.12.13` |
| fontTools | `4.63.0` |
| `SOURCE_DATE_EPOCH` | `1451606400` (2016-01-01T00:00:00Z) |

`SOURCE_DATE_EPOCH` is load-bearing, not hygiene: fontTools writes `head.modified` from the wall
clock unless it is set, and that drags `head.checkSumAdjustment` with it, so without it **every
regeneration produces different bytes**.

`tools/fontgen/instance_faces.py` performs and re-verifies this derivation;
`folio-go/fontgen_matrix_test.go` (`//go:build matrix`) proves the committed file still reproduces from
the upstream source.

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their notices." This
directory's `LICENSE-OFL.txt` and this NOTICE together satisfy that for the shipped file, and are what
`lint/internal/manifest.ResolveAssets` reads to attribute the face in `lint/MANIFEST.md`'s
redistributed non-code assets table.
