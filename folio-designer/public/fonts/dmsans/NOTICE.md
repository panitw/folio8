# DMSans-Regular.ttf — Story 16.1a local face tier batch

**Copyright 2014 The DM Sans Project Authors (https://github.com/googlefonts/dm-fonts)**

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
the family **`DM Sans`** into `src/generated/runtime-fonts.css`.

It is a **single upright static Regular**: `OS/2.usWeightClass` 400, the REGULAR
bit set in `fsSelection`, `head.macStyle` 0x0, `post.italicAngle` 0, **no**
`fvar`/`gvar` and **no** `CFF`/`CFF2` — glyf/TrueType outlines only. No bold,
italic, oblique or variable axis ships here; Epic 11 (FR57) owns
realize-vs-retire and the owner ruling has not been made (D-000.7). The
generated rule carries no `font-weight` and no `font-style` descriptor.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/googlefonts/dm-fonts` — the project publishes no tagged release, so the pin is GitHub's source archive for the commit-snapshot release `4412393b7d2de9fe7a92064c2dce9b5af5d7fd26` |
| Download URL | https://github.com/googlefonts/dm-fonts/archive/4412393b7d2de9fe7a92064c2dce9b5af5d7fd26.tar.gz |
| Path inside the archive | `dm-fonts-4412393b7d2de9fe7a92064c2dce9b5af5d7fd26/Sans/fonts/ttf/DMSans-Regular.ttf` |
| Fetched | 2026-09-03 |
| sha256 of the release archive | `0d0682bbbdee8c400249a37ce5613edf99f984c51237d8c98ec61f759b02b82c` (12,855,291 bytes) |
| **sha256 of the SOURCE (upstream) file** | `a95a338942aa357145427be7c963ff691f9fe859c3512d9c0483e1914d57a208` |

**The archive digest is a fetch-time measurement, not the binding one.** This project tags no
release, so the archive above is the source snapshot GitHub generates for the pinned commit; the
commit is immutable but the generated tarball's compression is GitHub's, not upstream's. **The two
digests that bind are the file digests** — the source file's and the shipped file's, which are the
same value here, and which `src/font-catalogue.test.ts` checks against the committed bytes on every
run.

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `DMSans-Regular.ttf` |
| **sha256 of the SHIPPED file** | `a95a338942aa357145427be7c963ff691f9fe859c3512d9c0483e1914d57a208` |
| Size | 78,256 bytes |
| Instance | Regular — `OS/2.usWeightClass` 400, upstream static build |
| Declared family (`name` table) | `DM Sans` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.txt` is Sans/OFL.txt inside the same commit-pinned archive, copied unmodified.
Its sha256 is `9af36190332437f5ecd09974de43c1f7c77a310a996cdd8ceb25628b458840e1` (4,482 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

The binary's own `name` table (nameID 13) declares the same terms, which is what
`src/font-catalogue.test.ts` ties `font-catalogue.json`'s `licence` field to:

> This Font Software is licensed under the SIL Open Font License, Version 1.1. This license is available with a FAQ at: https://openfontlicense.org

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-OFL.txt` and this NOTICE together satisfy
that for the shipped file.
