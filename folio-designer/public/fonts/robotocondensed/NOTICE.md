# RobotoCondensed-Regular.ttf — Story 16.1a local face tier batch

**Copyright 2011 The Roboto Project Authors (https://github.com/googlefonts/roboto-classic)**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt` in this directory,
the unmodified upstream licence text). Its SPDX identifier is `OFL-1.1`, one of
the four the owner's asset allowlist admits (D-8.5.3).

> **NO DERIVATION APPLIES.** This file is the upstream file itself, byte for
> byte. The upstream project publishes **static** TTF instances, so there is no
> instancing step to replay and no toolchain to pin — the sha256 of the file
> inside the release archive and the sha256 of the shipped artifact are **the
> same value**, and that identity *is* the provenance record. This repository
> cannot derive a face at all (`tools/fontgen/instance_faces.py` drives a
> hardcoded three-entry `UPSTREAM` list of engine faces), so a face requiring
> derivation would have been a Block If rather than a task.

It is one of Story 16.1a's ten batch faces, added to the **local face tier** —
the committed, permissively licensed families the designer's font browser offers
with no request leaving the machine. The batch exists because this family is
**variable-only on the `google/fonts` mirror** and therefore unaddable from the
fetched tier, while its **own project** publishes an ordinary static Regular.
The bytes below come from that project, never from the mirror.

`scripts/build-wasm.mjs` reads `folio-designer/font-catalogue.json`, fingerprints
this file into `src/generated/runtime/`, and emits one `@font-face` rule naming
the family **`Roboto Condensed`** into `src/generated/runtime-fonts.css`.

It is a **single upright static Regular**: `OS/2.usWeightClass` 400, the REGULAR
bit set in `fsSelection`, `head.macStyle` 0x0, `post.italicAngle` 0, **no**
`fvar`/`gvar` and **no** `CFF`/`CFF2` — glyf/TrueType outlines only. No bold,
italic, oblique or variable axis ships here; Epic 11 (FR57) owns
realize-vs-retire and the owner ruling has not been made (D-000.7). The
generated rule carries no `font-weight` and no `font-style` descriptor.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/googlefonts/roboto-3-classic`, release `v3.016` |
| Download URL | https://github.com/googlefonts/roboto-3-classic/releases/download/v3.016/Roboto_v3.016.zip |
| Path inside the archive | `android/static/RobotoCondensed-Regular.ttf` |
| Fetched | 2026-09-03 |
| sha256 of the release archive | `1653dbe12f248da8fb0b9920db7b9496cd677ed3981154f6f15285c8bd4e334f` (29,162,959 bytes) |
| **sha256 of the SOURCE (upstream) file** | `8c4c429a54b4af66cd337311569c0eb1d8e6d909c19d393f24a2817a32dc0924` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `RobotoCondensed-Regular.ttf` |
| **sha256 of the SHIPPED file** | `8c4c429a54b4af66cd337311569c0eb1d8e6d909c19d393f24a2817a32dc0924` |
| Size | 355,076 bytes |
| Instance | Regular — `OS/2.usWeightClass` 400, upstream static build |
| Declared family (`name` table) | `Roboto Condensed` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.txt` is OFL.txt at tag v3.016 of github.com/googlefonts/roboto-3-classic (the release archive carries no licence file), copied unmodified.
Its sha256 is `061402327a96aadb0bfb694a960ed289ecd38d383e396243831ab81feb109c41` (4,394 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

The binary's own `name` table (nameID 13) declares the same terms, which is what
`src/font-catalogue.test.ts` ties `font-catalogue.json`'s `licence` field to:

> This Font Software is licensed under the SIL Open Font License, Version 1.1. This license is available with a FAQ at: https://openfontlicense.org

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-OFL.txt` and this NOTICE together satisfy
that for the shipped file.
