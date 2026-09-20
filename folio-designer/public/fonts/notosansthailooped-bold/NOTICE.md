# NotoSansThaiLooped-Bold.ttf — spec-install-all-face-cuts story 3, the committed tier's cuts

**Copyright 2022 The Noto Project Authors (https://github.com/notofonts/thai)**

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

It is the **Bold** cut of the committed family **Noto Sans Thai Looped**, added by
spec-install-all-face-cuts story 3 so that a family behaves the same whichever
tier it came from (CAP-3). Its Regular is `../notosansthailooped/`, and **both come from
the same pinned archive** recorded below — one coherent source, not two.

`scripts/build-wasm.mjs` reads `folio-designer/font-catalogue.json`, fingerprints
this file into `src/generated/runtime/`, and emits one `@font-face` rule naming
the family **`Noto Sans Thai Looped Bold`** into `src/generated/runtime-fonts.css`.
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
| Upstream project | `github.com/notofonts/thai`, release `NotoSansThaiLooped-v2.000` |
| Download URL | https://github.com/notofonts/thai/releases/download/NotoSansThaiLooped-v2.000/NotoSansThaiLooped-v2.000.zip |
| Path inside the archive | `NotoSansThaiLooped/unhinted/ttf/NotoSansThaiLooped-Bold.ttf` |
| Fetched | 2026-09-20 |
| sha256 of the release archive | `d82d9a0d4f7b453d6eda8b3e10be680b0265138f610e91e4f5e887222f8bdb79` (5,414,813 bytes) |
| **sha256 of the SOURCE (upstream) file** | `de8d86343a81f6a7fe504ced219c9c81c11260ec9ed6c573c2258f07e006fd16` |

The archive is the same one this family's Regular was taken from, pinned by the
same digest. **The binding digest is the file digest** — the source file's and the
shipped file's, which are the same value here, and which
`src/font-catalogue.test.ts` checks against the committed bytes on every run.

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `NotoSansThaiLooped-Bold.ttf` |
| **sha256 of the SHIPPED file** | `de8d86343a81f6a7fe504ced219c9c81c11260ec9ed6c573c2258f07e006fd16` |
| Size | 30,492 bytes |
| Instance | Bold — `OS/2.usWeightClass` 700, upstream static build |
| Declared family (`name` table) | `Noto Sans Thai Looped` / `Bold` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.txt` is the same unmodified upstream licence text committed beside this
family's Regular, copied from `../notosansthailooped/LICENSE-OFL.txt`.
Its sha256 is `dad6e6abc2bf3fc37cc698af7607c3f4d4235039695713b222e5a034fb5b9b1c` (4,380 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

The binary's own `name` table (nameID 13) declares the same terms, which is what
`src/font-catalogue.test.ts` ties `font-catalogue.json`'s `licence` field to:

> This Font Software is licensed under the SIL Open Font License, Version 1.1. This Font Software is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the SIL Open Font License for the specific language, permissions and limitations governing your use of this Font Software.

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-OFL.txt` and this NOTICE together satisfy
that for the shipped file.
