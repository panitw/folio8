# Arimo-Regular.ttf — Story 16.1a local face tier batch

**Copyright 2020 The Arimo Project Authors (https://github.com/googlefonts/arimo)**

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
the family **`Arimo`** into `src/generated/runtime-fonts.css`.

It is a **single upright static Regular**: `OS/2.usWeightClass` 400, the REGULAR
bit set in `fsSelection`, `head.macStyle` 0x0, `post.italicAngle` 0, **no**
`fvar`/`gvar` and **no** `CFF`/`CFF2` — glyf/TrueType outlines only. No bold,
italic, oblique or variable axis ships here; Epic 11 (FR57) owns
realize-vs-retire and the owner ruling has not been made (D-000.7). The
generated rule carries no `font-weight` and no `font-style` descriptor.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/googlefonts/Arimo` — the project publishes no tagged release, so the pin is GitHub's source archive for the commit-snapshot release `4a6255f269916ae7ad3fc2706b0935e7621396b8` |
| Download URL | https://github.com/googlefonts/Arimo/archive/4a6255f269916ae7ad3fc2706b0935e7621396b8.tar.gz |
| Path inside the archive | `Arimo-4a6255f269916ae7ad3fc2706b0935e7621396b8/fonts/ttf/Arimo-Regular.ttf` |
| Fetched | 2026-09-03 |
| sha256 of the release archive | `3d855617e6d0f8c103cc20e527239db721e9168847490b35688f53650ea23566` (8,733,218 bytes) |
| **sha256 of the SOURCE (upstream) file** | `41b22bc8f0b51f932825d37bc55b5eb6ba67dfe599a626e4aff2b43b624f9f8c` |

**The archive digest is a fetch-time measurement, not the binding one.** This project tags no
release, so the archive above is the source snapshot GitHub generates for the pinned commit; the
commit is immutable but the generated tarball's compression is GitHub's, not upstream's. **The two
digests that bind are the file digests** — the source file's and the shipped file's, which are the
same value here, and which `src/font-catalogue.test.ts` checks against the committed bytes on every
run.

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `Arimo-Regular.ttf` |
| **sha256 of the SHIPPED file** | `41b22bc8f0b51f932825d37bc55b5eb6ba67dfe599a626e4aff2b43b624f9f8c` |
| Size | 478,712 bytes |
| Instance | Regular — `OS/2.usWeightClass` 400, upstream static build |
| Declared family (`name` table) | `Arimo` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.txt` is OFL.txt inside the same commit-pinned archive, copied unmodified.
Its sha256 is `c46fda6859efb80c0a3c1faa868908a3d2fcb26eb7fd9602d73bebeec21bca3a` (4,383 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

The binary's own `name` table (nameID 13) declares the same terms, which is what
`src/font-catalogue.test.ts` ties `font-catalogue.json`'s `licence` field to:

> This Font Software is licensed under the SIL Open Font License, Version 1.1. This license is available with a FAQ at: https://openfontlicense.org

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-OFL.txt` and this NOTICE together satisfy
that for the shipped file.
