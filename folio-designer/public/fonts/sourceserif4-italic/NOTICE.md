# SourceSerif4Display-It.ttf — spec-install-all-face-cuts story 3, the committed tier's cuts

**Copyright © 2014 - 2023 Adobe (http://www.adobe.com/), with Reserved Font Name ‘Source’.**

Licensed under the terms in `LICENSE-OFL.md` in this directory, the unmodified upstream
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

It is the **Italic** cut of the committed family **Source Serif 4 Display**, added by
spec-install-all-face-cuts story 3 so that a family behaves the same whichever
tier it came from (CAP-3). Its Regular is `../sourceserif4/`, and **both come from
the same pinned archive** recorded below — one coherent source, not two.

`scripts/build-wasm.mjs` reads `folio-designer/font-catalogue.json`, fingerprints
this file into `src/generated/runtime/`, and emits one `@font-face` rule naming
the family **`Source Serif 4 Display Italic`** into `src/generated/runtime-fonts.css`.
The rule carries no `font-weight` and no `font-style` descriptor: a cut is its own
CSS family name here, exactly as the thirteen hand-written shipped rules do it.

It is a sloped **Italic** static cut, and every claim below is read from the binary
itself rather than asserted about it: `OS/2.usWeightClass` 400,
`OS/2.fsSelection` 0x0001, `head.macStyle` 0x2,
`post.italicAngle` -12, **no** `fvar`/`gvar` and **no** `CFF`/`CFF2` —
glyf/TrueType outlines only. `src/font-catalogue.test.ts` holds this face's
declared `style` to those same five fields on every run.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/adobe-fonts/source-serif`, release `4.005R` |
| Download URL | https://github.com/adobe-fonts/source-serif/releases/download/4.005R/source-serif-4.005_Desktop.zip |
| Path inside the archive | `source-serif-4.005_Desktop/TTF/SourceSerif4Display-It.ttf` |
| Fetched | 2026-09-20 |
| sha256 of the release archive | `549fdb8f9a682bd06944298621404969f6de77c2e422ff3b8244a1dcd6a0c425` (17,634,695 bytes) |
| **sha256 of the SOURCE (upstream) file** | `7f0a515865cf4caae88931e12264dfa2180ac58be5a1997a5633049e84010e21` |

The archive is the same one this family's Regular was taken from, pinned by the
same digest. **The binding digest is the file digest** — the source file's and the
shipped file's, which are the same value here, and which
`src/font-catalogue.test.ts` checks against the committed bytes on every run.

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `SourceSerif4Display-It.ttf` |
| **sha256 of the SHIPPED file** | `7f0a515865cf4caae88931e12264dfa2180ac58be5a1997a5633049e84010e21` |
| Size | 198,712 bytes |
| Instance | Italic — `OS/2.usWeightClass` 400, upstream static build |
| Declared family (`name` table) | `Source Serif 4 Display` / `Italic` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are equal |

## The licence text beside this file

`LICENSE-OFL.md` is the same unmodified upstream licence text committed beside this
family's Regular, copied from `../sourceserif4/LICENSE-OFL.md`.
Its sha256 is `75784a295293a8992f5a8d99210566e0064a012e6dab6731305e3787f15896c7` (4,492 bytes).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the
face in `lint/MANIFEST.md`; a directory holding a font binary without both files
fails the build (AC25, AD-26).

The binary's own `name` table (nameID 13) declares the same terms, which is what
`src/font-catalogue.test.ts` ties `font-catalogue.json`'s `licence` field to:

> This Font Software is licensed under the SIL Open Font License, Version 1.1. This license is available with a FAQ at: http://scripts.sil.org/OFL. This Font Software is distributed on an ‘AS IS’ BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the SIL Open Font License for the specific language, permissions and limitations governing your use of this Font Software.

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their
notices." This directory's `LICENSE-OFL.md` and this NOTICE together satisfy
that for the shipped file.
