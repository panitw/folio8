# NotoSerif-BoldItalic.ttf — spec-install-all-face-cuts story 3, the committed tier's cuts

**Copyright 2022 The Noto Project Authors (https://github.com/notofonts/latin-greek-cyrillic)**

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

It is the **Bold Italic** cut of the committed family **Noto Serif**, added by
spec-install-all-face-cuts story 3 so that a family behaves the same whichever
tier it came from (CAP-3). Its Regular is `../notoserif/`, and **both come from
the same pinned archive** recorded below — one coherent source, not two.

`scripts/build-wasm.mjs` reads `folio-designer/font-catalogue.json`, fingerprints
this file into `src/generated/runtime/`, and emits one `@font-face` rule naming
the family **`Noto Serif Bold Italic`** into `src/generated/runtime-fonts.css`.
The rule carries no `font-weight` and no `font-style` descriptor: a cut is its own
CSS family name here, exactly as the thirteen hand-written shipped rules do it.

It is a sloped **Bold Italic** static cut, and every claim below is read from the binary
itself rather than asserted about it: `OS/2.usWeightClass` 700,
`OS/2.fsSelection` 0x01a1, `head.macStyle` 0x3,
`post.italicAngle` -12, **no** `fvar`/`gvar` and **no** `CFF`/`CFF2` —
glyf/TrueType outlines only. `src/font-catalogue.test.ts` holds this face's
declared `style` to those same five fields on every run.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/notofonts/latin-greek-cyrillic`, release `NotoSerif-v2.015` |
| Download URL | https://github.com/notofonts/latin-greek-cyrillic/releases/download/NotoSerif-v2.015/NotoSerif-v2.015.zip |
| Path inside the archive | `NotoSerif/unhinted/ttf/NotoSerif-BoldItalic.ttf` |
| Fetched | 2026-09-20 |
| sha256 of the release archive | `0e9a43c8a4b94ac76f55069ed1d7385bbcaf6b99527a94deb5619e032b7e76c1` (70,637,713 bytes) |
| **sha256 of the SOURCE (upstream) file** | `1bc4f86502eaa368718f6192bee022ea9a703d5af1c18b7d291212657b63074a` |

The archive is the same one this family's Regular was taken from, pinned by the
same digest. **The binding digest is the file digest** — the source file's and the
shipped file's, which are the same value here, and which
`src/font-catalogue.test.ts` checks against the committed bytes on every run.

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `NotoSerif-BoldItalic.ttf` |
| **sha256 of the SHIPPED file** | `1bc4f86502eaa368718f6192bee022ea9a703d5af1c18b7d291212657b63074a` |
| Size | 513,280 bytes |
| Instance | Bold Italic — `OS/2.usWeightClass` 700, upstream static build |
| Declared family (`name` table) | `Noto Serif` / `Bold Italic` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.txt` is the same unmodified upstream licence text committed beside this
family's Regular, copied from `../notoserif/LICENSE-OFL.txt`.
Its sha256 is `cee9892f9f0cc8fe882c9e9537ee6a89621d86ee7ceaf70b02e2b2b1c25c061a` (4,396 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

The binary's own `name` table (nameID 13) declares the same terms, which is what
`src/font-catalogue.test.ts` ties `font-catalogue.json`'s `licence` field to:

> This Font Software is licensed under the SIL Open Font License, Version 1.1. This license is available with a FAQ at: https://scripts.sil.org/OFL

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-OFL.txt` and this NOTICE together satisfy
that for the shipped file.
