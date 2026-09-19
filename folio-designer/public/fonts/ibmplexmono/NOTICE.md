# IBMPlexMono-Regular.ttf — shipped browser-chrome face (monospace)

**Copyright © 2017 IBM Corp. with Reserved Font Name "Plex"**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt`
in this directory, the unmodified upstream licence text).

> **NO DERIVATION APPLIES.** This file is the upstream file itself, byte for
> byte. IBM publishes **static** TTF instances in its release archives, so
> there is no instancing step to replay and no toolchain to pin: the sha256 of
> the source and the sha256 of the shipped artifact below are **the same
> value**, and that identity *is* the provenance record.

This is Story 8.4c's first shipped IBM Plex face (AC1). It backs the
`@font-face` family **`IBM Plex Mono`** that `scripts/build-wasm.mjs` emits
into `src/generated/runtime-fonts.css`, which is what `tokens.css`'s
`--font-mono` — and through it `--type-mono`, `--type-mono-em`,
`--type-numeric-lg`, `--type-brand`, `--type-brand-load`, `--type-band-tab`
and `--type-page-mono` — resolves through.

**It is a browser-chrome face and never enters the engine's `FontSet`.**
`folio-go/fonts` ships the three Noto faces the engine measures with; AD-17
keeps the browser a rasterizer only. Nothing here is embedded in a PDF.

Until this file landed, the family named `IBM Plex Mono` was declared from
`notosanssc/NotoSansSC-Regular.ttf` — a 10.6 MB CJK sans with no monospacing
at all, so every number, tab label and brand mark in the chrome was drawn in
a CJK face. `src/font-binary-identity.test.ts` is what now makes that
observable.

It is a **static** TrueType font — `glyf` outlines, **no** `fvar`/`gvar`/
`avar`, no `CFF`/`CFF2` — meeting NFR7's operative choice of the
glyf/TrueType static build over CFF/OpenType. It is genuinely monospaced:
`post.isFixedPitch` is `1`, and every one of its non-zero advance widths is
600/1000 — one distinct width font-wide.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/IBM/plex`, release `@ibm/plex-mono@2.5.0`<br>`https://github.com/IBM/plex/releases/download/%40ibm/plex-mono%402.5.0/ibm-plex-mono.zip` |
| Upstream file | `ibm-plex-mono/fonts/complete/ttf/IBMPlexMono-Regular.ttf` |
| Fetched | 2026-09-01 |
| sha256 of the release archive | `6d23f01257663d8cc49a0d64c22ced630b79e0e2a0ac08a0da86e9a38bbc481c` (6,940,652 bytes) |
| **sha256 of the SOURCE (upstream) file** | `7c6fbddca4b700be918f5f6183d9bd4464fa427fe435f0b480d77fe2bb8c5a43` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `IBMPlexMono-Regular.ttf` |
| **sha256 of the SHIPPED file** | `7c6fbddca4b700be918f5f6183d9bd4464fa427fe435f0b480d77fe2bb8c5a43` |
| Size | 173,052 bytes |
| Instance | Regular — `OS/2.usWeightClass` 400, upstream static build |
| Relation to source | **byte-identical** — the two digests above are equal |

`LICENSE-OFL.txt` is the release archive's own top-level `LICENSE.txt`,
copied unmodified. The face carries the same terms in its `name` table:
nameID 13 reads *"This Font Software is licensed under the SIL Open Font
License, Version 1.1. This license is available with a FAQ at:
http://scripts.sil.org/OFL"* and nameID 14 `http://scripts.sil.org/OFL`.

## Why the GitHub release archive and not `@ibm/plex-mono` on npm

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
