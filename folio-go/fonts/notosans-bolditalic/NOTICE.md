# NotoSans-BoldItalic.ttf — shipped production face (Latin, Story 11.1)

**Copyright 2022 The Noto Project Authors (https://github.com/notofonts/latin-greek-cyrillic)**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt`
in this directory, the unmodified upstream licence text).

> **This file is a DERIVATIVE of the upstream release, not the upstream
> file itself.** It is a single static instance produced from the
> upstream *variable* build by the invocation recorded below. Everything
> needed to replay that derivation and reproduce these exact bytes is in
> this document.

This is one of Story 11.1's seven new shipped production faces: folio-go's bundled `FontSet`, keyed as
`"Noto Sans Bold Italic"`.

**THIS FILE IS BYTE-IDENTICAL TO
`folio-designer/public/fonts/notosans-bolditalic/NotoSans-BoldItalic.ttf` ON PURPOSE, AND THE
BROWSER'S COPY DOES NOT FOLLOW THIS ONE AUTOMATICALLY.** This engine-side copy is the one
`tools/fontgen/instance_faces.py` writes and `make fonts-verify` re-derives and compares; the
browser's copy is a hand-made mirror that no generator writes and no derivation replays. Whoever
moves the bytes here must move the browser's copy in the same commit: the canvas paints the face the
engine resolved, and AD-17 makes the browser a rasterizer only, so a same-named different cut
renders a document differently in the designer and in the engine and fails SILENTLY rather than
loudly. `folio-designer/src/font-binary-identity.test.ts` digest-ties the two copies.

It is a **static** TrueType font — `glyf` outlines, **no** `fvar`/`gvar`/`avar`, no `CFF2`.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/notofonts/latin-greek-cyrillic`, release `NotoSans-v2.015` |
| Download URL | https://github.com/notofonts/latin-greek-cyrillic/releases/download/NotoSans-v2.015/NotoSans-v2.015.zip |
| Path inside the archive | `NotoSans/googlefonts/variable-ttf/NotoSans-Italic[wdth,wght].ttf` |
| sha256 of the release archive | `0c34df072a3fa7efbb7cbf34950e1f971a4447cffe365d3a359e2d4089b958f5` (117,491,253 bytes) |
| Local source filename | `NotoSans-Italic-VF.ttf` (in the gitignored `.font-sources/`, byte-identical to the archive path above) |
| Fetched | 2026-09-06 |
| **sha256 of the SOURCE (upstream) file** | `58e6e0ebd1931b29a365aa2d3e2ee9a9e831a3af7cf3ad1462d4e72154f0b291` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `NotoSans-BoldItalic.ttf` |
| **sha256 of the SHIPPED (produced) file** | `1a1882aa2efca4388498a9db2ced6eca5dd6141d2cd989248e2bf1d189718df1` |
| Size | 665,508 bytes |
| Instance | Bold Italic — `wght=700`, `wdth=100` (this face has TWO axes) |
| Declared PostScript name (`name[6]`) | `NotoSansItalic-BoldItalic` |

## The exact invocation, verbatim

```sh
SOURCE_DATE_EPOCH=1451606400 fonttools varLib.instancer --update-name-table \
    NotoSans-Italic-VF.ttf wght=700 wdth=100 -o NotoSans-BoldItalic.ttf
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

## The PostScript name is upstream-determined, and it is NOT a typo

`name[6]` of this face reads `NotoSansItalic-BoldItalic` — not `NotoSans-BoldItalic`. fontTools composes the instanced
PostScript name from the SOURCE variable font's own variations-PostScript-name prefix, which for
`NotoSans-Italic[wdth,wght].ttf` is `NotoSansItalic`. The upright Bold cut escapes this because the roman
VF's prefix is `NotoSans`.

**Do not "correct" it.** A shipped face is committed OUTPUT of a replayable derivation; hand-editing its
name table would put a manual edit inside the byte-identity regime, which is the one thing this
discipline exists to forbid. `folio-go/shipped_faces_test.go`'s `shippedFaceSpecs` asserts `name[6]`
exactly, so changing this value reds the suite.

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their notices." This
directory's `LICENSE-OFL.txt` and this NOTICE together satisfy that for the shipped file, and are what
`lint/internal/manifest.ResolveAssets` reads to attribute the face in `lint/MANIFEST.md`'s
redistributed non-code assets table.
