# SourceCodePro-Regular.ttf — Story 8.5 catalogue face

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
the family **`Source Code Pro`** into `src/generated/runtime-fonts.css`.

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
| Upstream project | `github.com/adobe-fonts/source-code-pro`, release `2.042R-u` |
| Download URL | https://github.com/adobe-fonts/source-code-pro/releases/download/2.042R-u/1.062R-i/1.026R-vf/TTF-source-code-pro-2.042R-u_1.062R-i.zip |
| Path inside the archive | `TTF/SourceCodePro-Regular.ttf` |
| Fetched | 2026-09-02 |
| sha256 of the release archive | `0c85bac90d15c040b82939aa92bc8404420fccc02e37bbcb9c93a7f21abb52c6` (1,294,648 bytes) |
| **sha256 of the SOURCE (upstream) file** | `74bd80d3e42a08517cd7e1108ba3d86f2da29ac0f3065be95e0357956ab9db37` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `SourceCodePro-Regular.ttf` |
| **sha256 of the SHIPPED file** | `74bd80d3e42a08517cd7e1108ba3d86f2da29ac0f3065be95e0357956ab9db37` |
| Size | 210,312 bytes |
| Instance | Regular — `OS/2.usWeightClass` 400, upstream static build |
| Declared family (`name` table) | `Source Code Pro` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.md` is LICENSE.md at tag 2.042R-u/1.062R-i/1.026R-vf of github.com/adobe-fonts/source-code-pro (the release archive carries no licence file), copied unmodified.
Its sha256 is `7c940e28a5388e9bba866cf0e408edda45fe0899ba98665b8f6ab31dc5e4b8ff` (4,566 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-OFL.md` and this NOTICE together satisfy
that for the shipped file.
