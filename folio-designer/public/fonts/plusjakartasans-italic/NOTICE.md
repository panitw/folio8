# PlusJakartaSans-Italic.ttf — spec-install-all-face-cuts story 3, the committed tier's cuts

**Copyright 2020 The Plus Jakarta Sans Project Authors (https://github.com/tokotype/PlusJakartaSans)**

Licensed under the terms in `LICENSE-OFL.txt` in this directory, the unmodified upstream
licence text carried beside this family's Regular. Its SPDX identifier is
`OFL-1.1`, one of the four the owner's asset allowlist admits (D-8.5.3).

> **NO DERIVATION APPLIES.** This file is the upstream file itself, byte for
> byte. The upstream project publishes **static** TTF instances, so there is no
> instancing step to replay and no toolchain to pin — the sha256 of the file
> inside the release archive and the sha256 of the shipped artifact are **the
> same value**, and that identity *is* the provenance record.
> `tools/fontgen/instance_faces.py` drives a hardcoded seven-entry `UPSTREAM`
> list of **engine** faces into `folio-go/fonts/`; it has never produced a
> catalogue face, and this story did not extend it.

It is the **Italic** cut of the committed family **Plus Jakarta Sans**, added by
spec-install-all-face-cuts story 3 so that a family behaves the same whichever
tier it came from (CAP-3). Its Regular is `../plusjakartasans/`, and **both come from
the same pinned archive** recorded below — one coherent source, not two.

`scripts/build-wasm.mjs` reads `folio-designer/font-catalogue.json`, fingerprints
this file into `src/generated/runtime/`, and emits one `@font-face` rule naming
the family **`Plus Jakarta Sans Italic`** into `src/generated/runtime-fonts.css`.
The rule carries no `font-weight` and no `font-style` descriptor: a cut is its own
CSS family name here, exactly as the thirteen hand-written shipped rules do it.

It is a sloped **Italic** static cut, and every claim below is read from the binary
itself rather than asserted about it: `OS/2.usWeightClass` 400,
`OS/2.fsSelection` 0x0081, `head.macStyle` 0x2,
`post.italicAngle` -8, **no** `fvar`/`gvar` and **no** `CFF`/`CFF2` —
glyf/TrueType outlines only. `src/font-catalogue.test.ts` holds this face's
declared `style` to those same five fields on every run.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/tokotype/PlusJakartaSans`, release `2.7.1` |
| Download URL | https://github.com/tokotype/PlusJakartaSans/releases/download/2.7.1/PlusJakartaSans-2.7.1.zip |
| Path inside the archive | `PlusJakartaSans-2.7.1/ttf/PlusJakartaSans-Italic.ttf` |
| Fetched | 2026-09-20 |
| sha256 of the release archive | `4bfc5cdf97d750423bb3d1d40ed8e529bc92288924d9c65e18ff486acefac66c` (1,018,646 bytes) |
| **sha256 of the SOURCE (upstream) file** | `3b959c96e558a6716653c4efcbe2582f8cc6df907473d0bca0b8888c5d5a6c4b` |

The archive is the same one this family's Regular was taken from, pinned by the
same digest. **The binding digest is the file digest** — the source file's and the
shipped file's, which are the same value here, and which
`src/font-catalogue.test.ts` checks against the committed bytes on every run.

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `PlusJakartaSans-Italic.ttf` |
| **sha256 of the SHIPPED file** | `3b959c96e558a6716653c4efcbe2582f8cc6df907473d0bca0b8888c5d5a6c4b` |
| Size | 135,332 bytes |
| Instance | Italic — `OS/2.usWeightClass` 400, upstream static build |
| Declared family (`name` table) | `Plus Jakarta Sans` / `Italic` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.txt` is the same unmodified upstream licence text committed beside this
family's Regular, copied from `../plusjakartasans/LICENSE-OFL.txt`.
Its sha256 is `995c7199cab65954f545996326755daee7b63cc6b42b06c13da1f9502ab08a99` (4,402 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

The binary's own `name` table (nameID 13) declares the same terms, which is what
`src/font-catalogue.test.ts` ties `font-catalogue.json`'s `licence` field to:

> This Font Software is licensed under the SIL Open Font License, Version 1.1. This license is available with a FAQ at: https://scripts.sil.org/OFL

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-OFL.txt` and this NOTICE together satisfy
that for the shipped file.
