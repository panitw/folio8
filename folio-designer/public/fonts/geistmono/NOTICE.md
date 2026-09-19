# GeistMono-Regular.ttf — Story 8.5 catalogue face

**Copyright 2024 The Geist Project Authors (https://github.com/vercel/geist-font.git)**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt` in this directory,
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
the family **`Geist Mono`** into `src/generated/runtime-fonts.css`.

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
| Upstream project | `github.com/vercel/geist-font`, release `v1.7.2` |
| Download URL | https://github.com/vercel/geist-font/releases/download/v1.7.2/geist-font-v1.7.2.zip |
| Path inside the archive | `geist-font/GeistMono/ttf/GeistMono-Regular.ttf` |
| Fetched | 2026-09-02 |
| sha256 of the release archive | `7fc800d2ac6b92844895196e5041aca55d814c15db70c44f79b3b83ab82b04e2` (8,207,303 bytes) |
| **sha256 of the SOURCE (upstream) file** | `42d8ad2e610238e64e8abfcde3037c63f7850a73928742b7ab7229d897bcb155` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `GeistMono-Regular.ttf` |
| **sha256 of the SHIPPED file** | `42d8ad2e610238e64e8abfcde3037c63f7850a73928742b7ab7229d897bcb155` |
| Size | 148,516 bytes |
| Instance | Regular — `OS/2.usWeightClass` 400, upstream static build |
| Declared family (`name` table) | `Geist Mono` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.txt` is geist-font/OFL.txt (inside the release archive), copied unmodified.
Its sha256 is `c683bfbcc7e087f5d37a54ef628f10387c451a83ddc459b151403a164ac46c90` (4,383 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-OFL.txt` and this NOTICE together satisfy
that for the shipped file.
