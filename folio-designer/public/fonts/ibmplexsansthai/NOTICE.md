# IBMPlexSansThai-Regular.ttf — shipped browser-chrome face (Thai)

**Copyright © 2017 IBM Corp. with Reserved Font Name "Plex"**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt`
in this directory, the unmodified upstream licence text).

> **NO DERIVATION APPLIES.** This file is the upstream file itself, byte for
> byte. IBM publishes **static** TTF instances in its release archives, so
> there is no instancing step to replay and no toolchain to pin: the sha256 of
> the source and the sha256 of the shipped artifact below are **the same
> value**, and that identity *is* the provenance record.

This is one of Story 8.4c's three shipped IBM Plex faces (AC2). It backs the
`@font-face` family **`IBM Plex Sans Thai`** that `scripts/build-wasm.mjs`
emits into `src/generated/runtime-fonts.css`, which `tokens.css`'s
`--font-page` names first.

**It is a browser-chrome face and never enters the engine's `FontSet`.**
`folio-go/fonts` ships the three Noto faces the engine measures with; AD-17
keeps the browser a rasterizer only. Nothing here is embedded in a PDF.

It is a **static** TrueType font — `glyf` outlines, **no** `fvar`/`gvar`/
`avar`, no `CFF`/`CFF2` — meeting NFR7's operative choice of the
glyf/TrueType static build over CFF/OpenType.

## The acceptance risk, discharged by measurement

Replacing the Thai chrome face is the one substitution in Story 8.4c that
could regress something visible, because a Thai rendering defect is already on
this repository's record: Thai fell through to the generic `sans-serif` and
*"letters rendered on top of each other"* (`src/canvas-font-stack.test.ts`
carries the report). All of the following was measured on 2026-09-01 against
the shipped `notosansthai/NotoSansThai-Regular.ttf`.

**cmap parity is exact.** Both faces map **87 of the 128** code points in
`U+0E00`–`U+0E7F`. The set difference is **empty in both directions**; the 41
unmapped are Unicode-unassigned, so both cover 100% of assigned Thai.

**Both strings from the recorded defect are fully covered** — `พระราชบัญญัติ`
and `การทวงถามหนี้`. `src/font-binary-identity.test.ts` asserts this against
the committed bytes, so the guard fails on the failure that actually happened.

**Mark positioning is strictly better, which is the mechanism that defect was
about.** Under script tag `thai`, langsys `dflt`:

| | this face | NotoSansThai-Regular.ttf |
|---|---|---|
| GPOS `MarkBasePos` | **3** | 1 |
| GPOS `MarkMarkPos` | **2** | 2 |
| GPOS `PairPos` | 3 | 1 |
| GPOS `ChainContextPos` | 2 | 2 |
| GPOS features | `kern`, `mark`, `mkmk` | `dist`, `kern`, `mark`, `mkmk` |
| GSUB | `ccmp`, `calt`, + 15 more | `ccmp`, `aalt`, `liga`, `ordn`, `ss01` |
| langsys under `thai` | `dflt`, `PAL`, `SAN`, `THA` | `dflt` |

**The one measured difference, recorded rather than treated as a blocker:**
Noto declares a `dist` feature under `thai` that Plex does not. Plex
compensates with more mark lookups, and `dist` is not the mechanism of the
recorded defect — mark attachment is.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/IBM/plex`, release `@ibm/plex-sans-thai@1.1.0`<br>`https://github.com/IBM/plex/releases/download/%40ibm/plex-sans-thai%401.1.0/ibm-plex-sans-thai.zip` |
| Upstream file | `ibm-plex-sans-thai/fonts/complete/ttf/IBMPlexSansThai-Regular.ttf` |
| Fetched | 2026-09-01 |
| sha256 of the release archive | `d7203f43c20f9abd40487f845c48db4077d2056ea18632c8959591c6815d7fb9` (1,985,960 bytes) |
| **sha256 of the SOURCE (upstream) file** | `83e1db8e8bad06bb760981f1dd528f5f209d20dfadebac12b21bf2f12453c8c6` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `IBMPlexSansThai-Regular.ttf` |
| **sha256 of the SHIPPED file** | `83e1db8e8bad06bb760981f1dd528f5f209d20dfadebac12b21bf2f12453c8c6` |
| Size | 116,728 bytes |
| Instance | Regular — `OS/2.usWeightClass` 400, upstream static build |
| Relation to source | **byte-identical** — the two digests above are equal |

`LICENSE-OFL.txt` is the release archive's own top-level `LICENSE.txt`,
copied unmodified. The face carries the same terms in its `name` table:
nameID 13 reads *"This Font Software is licensed under the SIL Open Font
License, Version 1.1. This license is available with a FAQ at:
http://scripts.sil.org/OFL"* and nameID 14 `http://scripts.sil.org/OFL`.

## Why the GitHub release archive and not `@ibm/plex-sans-thai` on npm

The npm package declares `"license": "OFL-1.1"` and is the obvious route. It
ships **zero** `.ttf`/`.otf`/`.ttc` — only `.woff`/`.woff2`.
`lint/internal/manifest/manifest.go`'s `fontExtensions` recognises only those
three suffixes, so a `.woff2` face would ship with no LICENSE required, no
NOTICE required and **no manifest row** — AD-26's asset half silently not
applying to the very binary it exists to account for. NFR7 independently
calls for the glyf/TrueType static build. Two selectors, one answer.

AD-26, verbatim: "Redistributed non-code assets keep their own terms and
their notices." This directory's `LICENSE-OFL.txt` and this NOTICE together
satisfy that for the shipped file, and are what
`lint/internal/manifest.ResolveAssets` (AC25) reads to attribute the face in
`lint/MANIFEST.md`'s redistributed non-code assets table.
