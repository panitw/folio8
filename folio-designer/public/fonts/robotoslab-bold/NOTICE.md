# RobotoSlab-Bold.ttf — spec-install-all-face-cuts story 3, the committed tier's cuts

**Copyright 2018 The Roboto Slab Project Authors (https://github.com/googlefonts/robotoslab)**

Licensed under the terms in `LICENSE-APACHE.txt` in this directory, the unmodified upstream
licence text carried beside this family's Regular. Its SPDX identifier is
`Apache-2.0`, one of the four the owner's asset allowlist admits (D-8.5.3).

> **NO DERIVATION APPLIES.** This file is the upstream file itself, byte for
> byte. The upstream project publishes **static** TTF instances, so there is no
> instancing step to replay and no toolchain to pin — the sha256 of the file
> inside the release archive and the sha256 of the shipped artifact are **the
> same value**, and that identity *is* the provenance record.
> `tools/fontgen/instance_faces.py` drives a hardcoded seven-entry `UPSTREAM`
> list of **engine** faces into `folio-go/fonts/`; it has never produced a
> catalogue face, and this story did not extend it.

It is the **Bold** cut of the committed family **Roboto Slab**, added by
spec-install-all-face-cuts story 3 so that a family behaves the same whichever
tier it came from (CAP-3). Its Regular is `../robotoslab/`, and **both come from
the same pinned archive** recorded below — one coherent source, not two.

`scripts/build-wasm.mjs` reads `folio-designer/font-catalogue.json`, fingerprints
this file into `src/generated/runtime/`, and emits one `@font-face` rule naming
the family **`Roboto Slab Bold`** into `src/generated/runtime-fonts.css`.
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
| Upstream project | `github.com/googlefonts/robotoslab` — the project publishes no tagged release, so the pin is GitHub's source archive for the commit-snapshot release `67af3ce9c4ca574419e1295b6165a2eeee112e6e` |
| Download URL | https://github.com/googlefonts/robotoslab/archive/67af3ce9c4ca574419e1295b6165a2eeee112e6e.tar.gz |
| Path inside the archive | `robotoslab-67af3ce9c4ca574419e1295b6165a2eeee112e6e/fonts/ttf/RobotoSlab-Bold.ttf` |
| Fetched | 2026-09-20 |
| sha256 of the release archive | `c5d85540342b84cfdd5913f37306a1046a36449bb93fe62e4ce3d43beb015d4a` (3,066,432 bytes) |
| **sha256 of the SOURCE (upstream) file** | `09cd9e359296e2f9482e2445ba921041913b55e652645df083079a85a272fce4` |

The archive is the same one this family's Regular was taken from, pinned by the
same digest. **The binding digest is the file digest** — the source file's and the
shipped file's, which are the same value here, and which
`src/font-catalogue.test.ts` checks against the committed bytes on every run.

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `RobotoSlab-Bold.ttf` |
| **sha256 of the SHIPPED file** | `09cd9e359296e2f9482e2445ba921041913b55e652645df083079a85a272fce4` |
| Size | 176,068 bytes |
| Instance | Bold — `OS/2.usWeightClass` 700, upstream static build |
| Declared family (`name` table) | `Roboto Slab` / `Bold` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-APACHE.txt` is the same unmodified upstream licence text committed beside this
family's Regular, copied from `../robotoslab/LICENSE-APACHE.txt`.
Its sha256 is `cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30` (11,358 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

The binary's own `name` table (nameID 13) declares the same terms, which is what
`src/font-catalogue.test.ts` ties `font-catalogue.json`'s `licence` field to:

> Licensed under the Apache License, Version 2.0

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-APACHE.txt` and this NOTICE together satisfy
that for the shipped file.
