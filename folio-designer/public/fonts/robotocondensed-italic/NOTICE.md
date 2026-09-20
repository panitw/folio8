# RobotoCondensed-Italic.ttf — spec-install-all-face-cuts story 3, the committed tier's cuts

**Copyright 2011 The Roboto Project Authors (https://github.com/googlefonts/roboto-classic)**

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

It is the **Italic** cut of the committed family **Roboto Condensed**, added by
spec-install-all-face-cuts story 3 so that a family behaves the same whichever
tier it came from (CAP-3). Its Regular is `../robotocondensed/`, and **both come from
the same pinned archive** recorded below — one coherent source, not two.

`scripts/build-wasm.mjs` reads `folio-designer/font-catalogue.json`, fingerprints
this file into `src/generated/runtime/`, and emits one `@font-face` rule naming
the family **`Roboto Condensed Italic`** into `src/generated/runtime-fonts.css`.
The rule carries no `font-weight` and no `font-style` descriptor: a cut is its own
CSS family name here, exactly as the thirteen hand-written shipped rules do it.

It is a sloped **Italic** static cut, and every claim below is read from the binary
itself rather than asserted about it: `OS/2.usWeightClass` 400,
`OS/2.fsSelection` 0x0201, `head.macStyle` 0x2,
`post.italicAngle` -12, **no** `fvar`/`gvar` and **no** `CFF`/`CFF2` —
glyf/TrueType outlines only. `src/font-catalogue.test.ts` holds this face's
declared `style` to those same five fields on every run.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/googlefonts/roboto-3-classic`, release `v3.016` |
| Download URL | https://github.com/googlefonts/roboto-3-classic/releases/download/v3.016/Roboto_v3.016.zip |
| Path inside the archive | `android/static/RobotoCondensed-Italic.ttf` |
| Fetched | 2026-09-20 |
| sha256 of the release archive | `1653dbe12f248da8fb0b9920db7b9496cd677ed3981154f6f15285c8bd4e334f` (29,162,959 bytes) |
| **sha256 of the SOURCE (upstream) file** | `71dd8475119fd15c38efeb77a328ebdea86d87b666952c81b78f34527ca7e7be` |

The archive is the same one this family's Regular was taken from, pinned by the
same digest. **The binding digest is the file digest** — the source file's and the
shipped file's, which are the same value here, and which
`src/font-catalogue.test.ts` checks against the committed bytes on every run.

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `RobotoCondensed-Italic.ttf` |
| **sha256 of the SHIPPED file** | `71dd8475119fd15c38efeb77a328ebdea86d87b666952c81b78f34527ca7e7be` |
| Size | 374,984 bytes |
| Instance | Italic — `OS/2.usWeightClass` 400, upstream static build |
| Declared family (`name` table) | `Roboto Condensed` / `Italic` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.txt` is the same unmodified upstream licence text committed beside this
family's Regular, copied from `../robotocondensed/LICENSE-OFL.txt`.
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
