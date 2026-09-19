# UbuntuSansMono-Regular.ttf — Story 8.5 catalogue face

**Copyright 2011, 2022, 2023 Canonical Ltd. Licensed under the Ubuntu Font Licence 1.0**

Licensed under the Ubuntu Font Licence, Version 1.0 (see `LICENSE-UFL.txt` in this directory,
the unmodified upstream licence text). Its SPDX identifier is `Ubuntu-font-1.0`, one of
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
the family **`Ubuntu Sans Mono`** into `src/generated/runtime-fonts.css`.

**Shipping the bytes is all this story does.** Nothing makes this family
selectable, nothing embeds it in a PDF, and nothing changes how the producer
subsets — picking behaviour is Story 8.6's.

It is a **single upright static Regular**: `OS/2.usWeightClass` 400,
`fsSelection` 0xc0, `head.macStyle` 0x0,
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
| Upstream project | `github.com/canonical/Ubuntu-Sans-Mono-fonts`, release `v1.100` |
| Download URL | https://github.com/canonical/Ubuntu-Sans-Mono-fonts/releases/download/v1.100/Ubuntu-Sans-Mono-fonts-v1.100.zip |
| Path inside the archive | `Ubuntu-Sans-Mono-fonts-v1.100/fonts/ttf/UbuntuSansMono-Regular.ttf` |
| Fetched | 2026-09-02 |
| sha256 of the release archive | `0e459ed18b552190971561b3e474d0c4583db40483debf9e951321c2a1c2dec8` (8,148,916 bytes) |
| **sha256 of the SOURCE (upstream) file** | `f2cf9d80e5f4dc4fff6b42ba993dfb3ddcb8ebaf358ef516b9c36a6fd407e04b` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `UbuntuSansMono-Regular.ttf` |
| **sha256 of the SHIPPED file** | `f2cf9d80e5f4dc4fff6b42ba993dfb3ddcb8ebaf358ef516b9c36a6fd407e04b` |
| Size | 216,440 bytes |
| Instance | Regular — `OS/2.usWeightClass` 400, upstream static build |
| Declared family (`name` table) | `Ubuntu Sans Mono` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-UFL.txt` is Ubuntu-Sans-Mono-fonts-v1.100/LICENCE.txt (inside the release archive), copied unmodified.
Its sha256 is `2f0015108d68627bd788d313f529c21ff4da2c2c42a5e1f3883acc83480f9002` (4,673 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-UFL.txt` and this NOTICE together satisfy
that for the shipped file.
