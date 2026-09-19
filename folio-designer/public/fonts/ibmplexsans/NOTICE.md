# IBMPlexSans-Regular.ttf — shipped browser-chrome face (Latin)

**Copyright © 2017 IBM Corp. with Reserved Font Name "Plex"**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt`
in this directory, the unmodified upstream licence text).

> **NO DERIVATION APPLIES.** This file is the upstream file itself, byte for
> byte. IBM publishes **static** TTF instances in its release archives, so
> there is no instancing step to replay and no toolchain to pin: the sha256 of
> the source and the sha256 of the shipped artifact below are **the same
> value**, and that identity *is* the provenance record.

This is one of Story 8.4c's three shipped IBM Plex faces (AC2). It backs the
`@font-face` family **`IBM Plex Sans`** that `scripts/build-wasm.mjs` emits
into `src/generated/runtime-fonts.css`, which `tokens.css`'s `--font-sans`
resolves through — and which `--font-page` names as its second choice.

**It is a browser-chrome face and never enters the engine's `FontSet`.**
`folio-go/fonts` ships the three Noto faces the engine measures with; AD-17
keeps the browser a rasterizer only. Nothing here is embedded in a PDF.

Until this file landed, the family named `IBM Plex Sans` was declared from
`notosans/NotoSans-Regular.ttf` — IBM Plex in name only.
`src/font-binary-identity.test.ts` is what now makes that observable: it opens
this file and checks that its own `name` table says `IBM Plex Sans`.

It is a **static** TrueType font — `glyf` outlines, **no** `fvar`/`gvar`/
`avar`, no `CFF`/`CFF2` — meeting NFR7's operative choice of the
glyf/TrueType static build over CFF/OpenType.

## Chrome coverage, measured 2026-09-01 against the shipped Noto Sans

| range | this face | NotoSans-Regular.ttf |
|---|---|---|
| ASCII `U+0020`–`U+007E` | **95 / 95** | 95 |
| Latin-1 Supplement `U+00A0`–`U+00FF` | **96 / 96** | 96 |
| Latin Extended-A `U+0100`–`U+017F` | **128 / 128** | 128 |

Zero gaps across `U+0020`–`U+017F`, which is the whole of what the chrome
draws. It is thinner beyond: 895 mapped code points against Noto Sans's 3,094
(Latin Extended-B, Greek and Cyrillic are all narrower). The chrome renders
none of those, and the CANVAS does not use this face at all — canvas text asks
for the engine's own `Noto Sans` — so the difference is recorded rather than
treated as a regression.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/IBM/plex`, release `@ibm/plex-sans@1.1.0`<br>`https://github.com/IBM/plex/releases/download/%40ibm/plex-sans%401.1.0/ibm-plex-sans.zip` |
| Upstream file | `ibm-plex-sans/fonts/complete/ttf/IBMPlexSans-Regular.ttf` |
| Fetched | 2026-09-01 |
| sha256 of the release archive | `fb365d910566e6d199cc2c15579a7dd9a267128e18431a394ed81f1970c69200` (9,921,777 bytes) |
| **sha256 of the SOURCE (upstream) file** | `975dcda37d80f038dcd143c22e33ca2d97a0cc5a929aace1c749153b0fe1afa5` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `IBMPlexSans-Regular.ttf` |
| **sha256 of the SHIPPED file** | `975dcda37d80f038dcd143c22e33ca2d97a0cc5a929aace1c749153b0fe1afa5` |
| Size | 200,500 bytes |
| Instance | Regular — `OS/2.usWeightClass` 400, upstream static build |
| Relation to source | **byte-identical** — the two digests above are equal |

`LICENSE-OFL.txt` is the release archive's own top-level `LICENSE.txt`,
copied unmodified. The face carries the same terms in its `name` table:
nameID 13 reads *"This Font Software is licensed under the SIL Open Font
License, Version 1.1. This license is available with a FAQ at:
http://scripts.sil.org/OFL"* and nameID 14 `http://scripts.sil.org/OFL`.

## Why the GitHub release archive and not `@ibm/plex-sans` on npm

The npm package declares `"license": "OFL-1.1"` and is the obvious route. It
ships **zero** `.ttf`/`.otf`/`.ttc` — only `.woff`/`.woff2`.
`lint/internal/manifest/manifest.go`'s `fontExtensions` recognises only those
three suffixes, so a `.woff2` face would ship with no LICENSE required, no
NOTICE required and **no manifest row** — AD-26's asset half silently not
applying to the very binary it exists to account for. NFR7 independently
calls for the glyf/TrueType static build. Two selectors, one answer.

## One Regular, no weight matrix

`DESIGN.md` specifies `fontWeight` 400, 500 and 600 across its tokens. The
generator emits one rule per family with **no `font-weight` and no
`font-style` descriptor**, so every weight and italic in the chrome is
browser-synthesised from a single Regular — as it already was from the Noto
faces. Shipping one static Regular preserves that behaviour exactly and
changes only which face is synthesised from.

AD-26, verbatim: "Redistributed non-code assets keep their own terms and
their notices." This directory's `LICENSE-OFL.txt` and this NOTICE together
satisfy that for the shipped file, and are what
`lint/internal/manifest.ResolveAssets` (AC25) reads to attribute the face in
`lint/MANIFEST.md`'s redistributed non-code assets table.
