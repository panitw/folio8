# UbuntuSans-Italic.ttf — spec-install-all-face-cuts story 3, the committed tier's cuts

**Copyright 2011, 2022, 2023 Canonical Ltd. Licensed under the Ubuntu Font Licence 1.0**

Licensed under the terms in `LICENSE-UFL.txt` in this directory, the unmodified upstream
licence text carried beside this family's Regular. Its SPDX identifier is
`Ubuntu-font-1.0`, one of the four the owner's asset allowlist admits (D-8.5.3).

> **NO DERIVATION APPLIES.** This file is the upstream file itself, byte for
> byte. The upstream project publishes **static** TTF instances, so there is no
> instancing step to replay and no toolchain to pin — the sha256 of the file
> inside the release archive and the sha256 of the shipped artifact are **the
> same value**, and that identity *is* the provenance record.
> `tools/fontgen/instance_faces.py` drives a hardcoded seven-entry `UPSTREAM`
> list of **engine** faces into `folio-go/fonts/`; it has never produced a
> catalogue face, and this story did not extend it.

It is the **Italic** cut of the committed family **Ubuntu Sans**, added by
spec-install-all-face-cuts story 3 so that a family behaves the same whichever
tier it came from (CAP-3). Its Regular is `../ubuntusans/`, and **both come from
the same pinned archive** recorded below — one coherent source, not two.

`scripts/build-wasm.mjs` reads `folio-designer/font-catalogue.json`, fingerprints
this file into `src/generated/runtime/`, and emits one `@font-face` rule naming
the family **`Ubuntu Sans Italic`** into `src/generated/runtime-fonts.css`.
The rule carries no `font-weight` and no `font-style` descriptor: a cut is its own
CSS family name here, exactly as the thirteen hand-written shipped rules do it.

It is a sloped **Italic** static cut, and every claim below is read from the binary
itself rather than asserted about it: `OS/2.usWeightClass` 400,
`OS/2.fsSelection` 0x0081, `head.macStyle` 0x2,
`post.italicAngle` -13.5, **no** `fvar`/`gvar` and **no** `CFF`/`CFF2` —
glyf/TrueType outlines only. `src/font-catalogue.test.ts` holds this face's
declared `style` to those same five fields on every run.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/canonical/Ubuntu-Sans-fonts`, release `v1.006` |
| Download URL | https://github.com/canonical/Ubuntu-Sans-fonts/releases/download/v1.006/UbuntuSans-fonts-1.006.zip |
| Path inside the archive | `UbuntuSans-fonts-1.006/ttf/UbuntuSans-Italic.ttf` |
| Fetched | 2026-09-20 |
| sha256 of the release archive | `20aa7af47beaa6d64327782f7bf60df375edeedbdee2fe407e0bf9721890fb83` (29,602,887 bytes) |
| **sha256 of the SOURCE (upstream) file** | `602ba07e697176cf8e578921524f777b4f0489e4eee46277c08ca15cb1e37fa9` |

The archive is the same one this family's Regular was taken from, pinned by the
same digest. **The binding digest is the file digest** — the source file's and the
shipped file's, which are the same value here, and which
`src/font-catalogue.test.ts` checks against the committed bytes on every run.

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `UbuntuSans-Italic.ttf` |
| **sha256 of the SHIPPED file** | `602ba07e697176cf8e578921524f777b4f0489e4eee46277c08ca15cb1e37fa9` |
| Size | 325,020 bytes |
| Instance | Italic — `OS/2.usWeightClass` 400, upstream static build |
| Declared family (`name` table) | `Ubuntu Sans` / `Italic` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-UFL.txt` is the same unmodified upstream licence text committed beside this
family's Regular, copied from `../ubuntusans/LICENSE-UFL.txt`.
Its sha256 is `2f0015108d68627bd788d313f529c21ff4da2c2c42a5e1f3883acc83480f9002` (4,673 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

The binary's own `name` table (nameID 13) declares the same terms, which is what
`src/font-catalogue.test.ts` ties `font-catalogue.json`'s `licence` field to:

> Licensed under the Ubuntu Font Licence 1.0.

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-UFL.txt` and this NOTICE together satisfy
that for the shipped file.
