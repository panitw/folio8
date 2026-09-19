# SourceSans3-Regular.ttf — Story 8.5 catalogue face

**Copyright © 2023 Adobe (http://www.adobe.com/), with Reserved Font Name ‘Source’**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.md` in this directory,
the unmodified upstream licence text). Its SPDX identifier is `OFL-1.1`, one of
the four the owner's asset allowlist admits (D-8.5.3).

> **NO DERIVATION APPLIES.** This file is the upstream file itself, byte for
> byte. The upstream project publishes **static** TTF instances, so there is no
> instancing step to replay and no toolchain to pin — the sha256 of the file
> inside the release archive and the sha256 of the shipped artifact are **the
> same value**, and that identity *is* the provenance record. This repository
> cannot derive a face at all (`tools/fontgen/instance_faces.py` drives a
> hardcoded three-entry `UPSTREAM` list), so a face requiring derivation would
> have been a Block If rather than a task.

It is one of Story 8.5's 21 catalogue faces: a curated, permissively
licensed typeface **bundled and precached** with the designer's offline release
(D-8.5.12 — no live font service, no arbitrary URL, no download on first use).
`scripts/build-wasm.mjs` reads `folio-designer/font-catalogue.json`, fingerprints
this file into `src/generated/runtime/`, and emits one `@font-face` rule naming
the family **`Source Sans 3`** into `src/generated/runtime-fonts.css`.

**Shipping the bytes is all this story does.** Nothing makes this family
selectable, nothing embeds it in a PDF, and nothing changes how the producer
subsets — picking behaviour is Story 8.6's.

It is a **single upright static Regular**: `OS/2.usWeightClass` 400,
`fsSelection` 0x40, `head.macStyle` 0x0,
`post.italicAngle` 0, **no** `fvar`/`gvar` and **no** `CFF`/`CFF2`
— glyf/TrueType outlines only. No bold, italic, oblique or variable axis ships
here; Epic 11 (FR57) owns realize-vs-retire and the owner ruling has not been
made (D-000.7). The generated rule carries no `font-weight` and no
`font-style` descriptor, so every other weight the chrome asks for stays
browser-synthesised from this one face.

**It is a browser-chrome face and never enters the engine's `FontSet`.**
`folio-go/fonts` ships the three Noto faces the engine measures with; AD-17
keeps the browser a rasterizer only. Nothing here is embedded in a PDF.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/adobe-fonts/source-sans`, release `3.052R` |
| Download URL | https://github.com/adobe-fonts/source-sans/releases/download/3.052R/TTF-source-sans-3.052R.zip |
| Path inside the archive | `TTF/SourceSans3-Regular.ttf` |
| Fetched | 2026-09-02 |
| sha256 of the release archive | `1b0dd1ec44b39f1dd98bbd153a1a3815f083639874ddee02c842bd601bad3d21` (2,478,353 bytes) |
| **sha256 of the SOURCE (upstream) file** | `4644c81b86ec9caaa76b634889968ed3c4f4f52f054855933acc7c2b21e53b0f` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `SourceSans3-Regular.ttf` |
| **sha256 of the SHIPPED file** | `4644c81b86ec9caaa76b634889968ed3c4f4f52f054855933acc7c2b21e53b0f` |
| Size | 431,196 bytes |
| Instance | Regular — `OS/2.usWeightClass` 400, upstream static build |
| Declared family (`name` table) | `Source Sans 3` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.md` is LICENSE.md at tag 3.052R of github.com/adobe-fonts/source-sans (the release archive carries no licence file), copied unmodified.
Its sha256 is `89ad2c4f66dd29127527493e729c31e731f111cf10faf5774c3db9275ed0c22c` (4,579 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-OFL.md` and this NOTICE together satisfy
that for the shipped file.
