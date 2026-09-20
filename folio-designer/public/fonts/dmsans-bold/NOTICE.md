# DMSans-Bold.ttf — spec-install-all-face-cuts story 3, the committed tier's cuts

**Copyright 2014 The DM Sans Project Authors (https://github.com/googlefonts/dm-fonts)**

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

It is the **Bold** cut of the committed family **DM Sans**, added by
spec-install-all-face-cuts story 3 so that a family behaves the same whichever
tier it came from (CAP-3). Its Regular is `../dmsans/`, and **both come from
the same pinned archive** recorded below — one coherent source, not two.

`scripts/build-wasm.mjs` reads `folio-designer/font-catalogue.json`, fingerprints
this file into `src/generated/runtime/`, and emits one `@font-face` rule naming
the family **`DM Sans Bold`** into `src/generated/runtime-fonts.css`.
The rule carries no `font-weight` and no `font-style` descriptor: a cut is its own
CSS family name here, exactly as the thirteen hand-written shipped rules do it.

It is an upright **Bold** static cut, and every claim below is read from the binary
itself rather than asserted about it: `OS/2.usWeightClass` 700,
`OS/2.fsSelection` 0x00a0, `head.macStyle` 0x1,
`post.italicAngle` 0, **no** `fvar`/`gvar` and **no** `CFF`/`CFF2` —
glyf/TrueType outlines only. `src/font-catalogue.test.ts` holds this face's
declared `style` to those same five fields on every run.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/googlefonts/dm-fonts` — the project publishes no tagged release, so the pin is GitHub's source archive for the commit-snapshot release `4412393b7d2de9fe7a92064c2dce9b5af5d7fd26` |
| Download URL | https://github.com/googlefonts/dm-fonts/archive/4412393b7d2de9fe7a92064c2dce9b5af5d7fd26.tar.gz |
| Path inside the archive | `dm-fonts-4412393b7d2de9fe7a92064c2dce9b5af5d7fd26/Sans/fonts/ttf/DMSans-Bold.ttf` |
| Fetched | 2026-09-20 |
| sha256 of the release archive | `0d0682bbbdee8c400249a37ce5613edf99f984c51237d8c98ec61f759b02b82c` (12,855,291 bytes) |
| **sha256 of the SOURCE (upstream) file** | `6a6924b35d757e32b21d611733d5b9a7bb6234b07b75334b0e2d2359985b26ee` |

The archive is the same one this family's Regular was taken from, pinned by the
same digest. **The binding digest is the file digest** — the source file's and the
shipped file's, which are the same value here, and which
`src/font-catalogue.test.ts` checks against the committed bytes on every run.

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `DMSans-Bold.ttf` |
| **sha256 of the SHIPPED file** | `6a6924b35d757e32b21d611733d5b9a7bb6234b07b75334b0e2d2359985b26ee` |
| Size | 78,356 bytes |
| Instance | Bold — `OS/2.usWeightClass` 700, upstream static build |
| Declared family (`name` table) | `DM Sans` / `Bold` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.txt` is the same unmodified upstream licence text committed beside this
family's Regular, copied from `../dmsans/LICENSE-OFL.txt`.
Its sha256 is `2af94f4fb533be8fa23282eb33e08ca311ddf47c2f32777e2040b282deeec65c` (4,389 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

The binary's own `name` table (nameID 13) declares the same terms, which is what
`src/font-catalogue.test.ts` ties `font-catalogue.json`'s `licence` field to:

> This Font Software is licensed under the SIL Open Font License, Version 1.1. This license is available with a FAQ at: https://openfontlicense.org

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-OFL.txt` and this NOTICE together satisfy
that for the shipped file.
