# CascadiaCode-Bold.ttf — spec-install-all-face-cuts story 3, the committed tier's cuts

**Copyright © 2021 Microsoft Corporation. All Rights Reserved.**

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

It is the **Bold** cut of the committed family **Cascadia Code**, added by
spec-install-all-face-cuts story 3 so that a family behaves the same whichever
tier it came from (CAP-3). Its Regular is `../cascadiacode/`, and **both come from
the same pinned archive** recorded below — one coherent source, not two.

`scripts/build-wasm.mjs` reads `folio-designer/font-catalogue.json`, fingerprints
this file into `src/generated/runtime/`, and emits one `@font-face` rule naming
the family **`Cascadia Code Bold`** into `src/generated/runtime-fonts.css`.
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
| Upstream project | `github.com/microsoft/cascadia-code`, release `v2407.24` |
| Download URL | https://github.com/microsoft/cascadia-code/releases/download/v2407.24/CascadiaCode-2407.24.zip |
| Path inside the archive | `ttf/static/CascadiaCode-Bold.ttf` |
| Fetched | 2026-09-20 |
| sha256 of the release archive | `e67a68ee3386db63f48b9054bd196ea752bc6a4ebb4df35adce6733da50c8474` (150,454,761 bytes) |
| **sha256 of the SOURCE (upstream) file** | `58197c5c3ba2967d4780b0ef8aee72d0cbbfa42885ddede9ec96fa7a1df5aead` |

The archive is the same one this family's Regular was taken from, pinned by the
same digest. **The binding digest is the file digest** — the source file's and the
shipped file's, which are the same value here, and which
`src/font-catalogue.test.ts` checks against the committed bytes on every run.

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `CascadiaCode-Bold.ttf` |
| **sha256 of the SHIPPED file** | `58197c5c3ba2967d4780b0ef8aee72d0cbbfa42885ddede9ec96fa7a1df5aead` |
| Size | 604,696 bytes |
| Instance | Bold — `OS/2.usWeightClass` 700, upstream static build |
| Declared family (`name` table) | `Cascadia Code` / `Bold` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.txt` is the same unmodified upstream licence text committed beside this
family's Regular, copied from `../cascadiacode/LICENSE-OFL.txt`.
Its sha256 is `82c05d6c53dfa0c9025985c19e371810020b74ed2c61d51d370f2a8ab2506d52` (4,395 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

The binary's own `name` table (nameID 13) declares the same terms, which is what
`src/font-catalogue.test.ts` ties `font-catalogue.json`'s `licence` field to:

> Microsoft supplied font. You may use this font to create, display, and print content as permitted by the license terms or terms of use, of the Microsoft product, service, or content in which this font was included. You may only (i) embed this font in content as permitted by the embedding restrictions included in this font; and (ii) temporarily download this font to a printer or other output device to help print content. Any other use is prohibited.
 
The following license, based on the SIL Open Font license (https://scripts.sil.org/OFL), applies to this font. Additional license terms may be found on the GitHub repository for this font (https://github.com/microsoft/cascadia-code/blob/main/LICENSE).
 
Permission is hereby granted, free of charge, to any person obtaining a copy of the Font Software, to use, study, copy, merge, embed, modify, redistribute, and sell modified and unmodified copies of the Font Software, subject to the following conditions:
 
1) Neither the Font Software nor any of its individual components, in Original or Modified Versions, may be sold by itself.
 
2) Original or Modified Versions of the Font Software may be bundled, redistributed and/or sold with any software, provided that each copy contains the above copyright notice and this license. These can be included either as stand-alone text files, human-readable headers or in the appropriate machine-readable metadata fields within text or binary files as long as those fields can be easily viewed by the user.
 
3) No Modified Version of the Font Software may use the Reserved Font Name(s) unless explicit written permission is granted by the corresponding Copyright Holder. This restriction only applies to the primary font name as presented to the users.
 
4) The name(s) of the Copyright Holder(s) or the Author(s) of the Font Software shall not be used to promote, endorse or advertise any Modified Version, except to acknowledge the contribution(s) of the Copyright Holder(s) and the Author(s) or with their explicit written permission.
 
5) The Font Software, modified or unmodified, in part or in whole, must be distributed entirely under this license, and must not be distributed under any other license. The requirement for fonts to remain under this license does not apply to any document created using the Font Software.
 
THE FONT SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO ANY WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT OF COPYRIGHT, PATENT, TRADEMARK, OR OTHER RIGHT. IN NO EVENT SHALL THE COPYRIGHT HOLDER BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, INCLUDING ANY GENERAL, SPECIAL, INDIRECT, INCIDENTAL, OR CONSEQUENTIAL DAMAGES, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF THE USE OR INABILITY TO USE THE FONT SOFTWARE OR FROM OTHER DEALINGS IN THE FONT SOFTWARE.

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-OFL.txt` and this NOTICE together satisfy
that for the shipped file.
