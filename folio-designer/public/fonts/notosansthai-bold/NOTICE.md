# NotoSansThai-Bold.ttf — shipped production face (Thai, Story 11.1)

**Copyright 2022 The Noto Project Authors (https://github.com/notofonts/thai)**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt`
in this directory, the unmodified upstream licence text).

> **This file is a DERIVATIVE of the upstream release, not the upstream
> file itself.** It is a single static instance produced from the
> upstream *variable* build by the invocation recorded below. Everything
> needed to replay that derivation and reproduce these exact bytes is in
> this document.

Story 11.1's browser-side copy of the engine's shipped face. It is **byte-identical** to
`folio-go/fonts/notosansthai-bold/NotoSansThai-Bold.ttf` on purpose: the canvas paints the face the engine resolved, so the
browser must hold the same bytes the engine measured with.

It is a **static** TrueType font — `glyf` outlines, **no** `fvar`/`gvar`/`avar`, no `CFF2`.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/notofonts/thai`, release `NotoSansThai-v2.002` |
| Download URL | https://github.com/notofonts/thai/releases/download/NotoSansThai-v2.002/NotoSansThai-v2.002.zip |
| Path inside the archive | `NotoSansThai/googlefonts/variable/NotoSansThai[wdth,wght].ttf` |
| sha256 of the release archive | `af889cc673fc714060ce5e4e088fbad32aa4c0571a19958efeaff128a22da485` (4,720,990 bytes) |
| How the archive was verified | **transitively**: the extracted source file below hashes to the `src_sha256` already pinned in `tools/fontgen/instance_faces.py`, so this is the release the repo already shipped from. This is the same standard as Noto Sans and a weaker one than Roboto, whose archive digest was recorded before this story and matched directly. |
| Local source filename | `NotoSansThai-VF.ttf` (in the gitignored `.font-sources/`, byte-identical to the archive path above) |
| Fetched | 2026-09-06 |
| **sha256 of the SOURCE (upstream) file** | `5a1c559bb539583c8a1fd99d1c5b9491e5e14478c9cd2bd0970d5c3096cc9ef8` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `NotoSansThai-Bold.ttf` |
| **sha256 of the SHIPPED (produced) file** | `fe60f91611714dc6a57d5facb1818292b08c22cf88a55c60084173ea92e2ddbd` |
| Size | 47,800 bytes |
| Instance | Bold — `wght=700`, `wdth=100` (this face has TWO axes) |
| Declared PostScript name (`name[6]`) | `NotoSansThai-Bold` |

## The exact invocation, verbatim

```sh
SOURCE_DATE_EPOCH=1451606400 fonttools varLib.instancer --update-name-table \
    NotoSansThai-VF.ttf wght=700 wdth=100 -o NotoSansThai-Bold.ttf
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
