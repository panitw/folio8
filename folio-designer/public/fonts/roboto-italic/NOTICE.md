# Roboto-Italic.ttf — shipped production face (Latin, Story 11.1)

**Copyright 2011 The Roboto Project Authors (https://github.com/googlefonts/roboto-classic)**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt`
in this directory, the unmodified upstream licence text).

> **NO DERIVATION APPLIES.** This file is the upstream file itself, byte for byte — the upstream
> project publishes a **static** TTF instance, so there is no instancing step to replay and no
> toolchain to pin. The SOURCE and SHIPPED digests below are therefore the same value, and that
> identity IS the provenance record.

Story 11.1's browser-side copy of the engine's shipped face. It is **byte-identical** to
`folio-go/fonts/roboto-italic/Roboto-Italic.ttf` on purpose: the canvas paints the face the engine resolved, so the
browser must hold the same bytes the engine measured with.

It is a **static** TrueType font — `glyf` outlines, **no** `fvar`/`gvar`/`avar`, no `CFF2`.

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/googlefonts/roboto-3-classic`, release `v3.016` |
| Download URL | https://github.com/googlefonts/roboto-3-classic/releases/download/v3.016/Roboto_v3.016.zip |
| Path inside the archive | `android/static/Roboto-Italic.ttf` |
| sha256 of the release archive | `1653dbe12f248da8fb0b9920db7b9496cd677ed3981154f6f15285c8bd4e334f` (29,162,959 bytes) |
| Fetched | 2026-09-06 |
| **sha256 of the SOURCE (upstream) file** | `1f4b29f2e9c648707620a0768f4030df7abb15c25383fb73d088fe2624724f46` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `Roboto-Italic.ttf` |
| **sha256 of the SHIPPED (produced) file** | `1f4b29f2e9c648707620a0768f4030df7abb15c25383fb73d088fe2624724f46` |
| Size | 375,320 bytes |
| Instance | Italic — `OS/2.usWeightClass` 400, upstream static build |
| Declared PostScript name (`name[6]`) | `Roboto-Italic` |
| Relation to source | **copied unmodified, no derivation** — the two digests above are the same value |

## The licence text beside this file

`LICENSE-OFL.txt` in this directory is copied unmodified from
`folio-designer/public/fonts/roboto/LICENSE-OFL.txt` — itself `OFL.txt` at tag `v3.016` of
`github.com/googlefonts/roboto-3-classic`, taken from the TAG because **the release archive
this face's binary came out of carries no licence file at all**. That is why the licence text
and the binary have different provenance lines: it is recorded, not overlooked.

**The two upstream URLs in this NOTICE are both correct, and neither is a typo to reconcile.** The
copyright line at the top names `github.com/googlefonts/roboto-classic` because that is the string the
upstream OFL header itself carries, verbatim, and this NOTICE reproduces the licence's own attribution
rather than rewriting it. The provenance table names `github.com/googlefonts/roboto-3-classic` because
that is the repository the `v3.016` release and its archive actually live in, and it is the URL a
reader must use to re-fetch the bytes. Editing either one to match the other would make this NOTICE
disagree with the licence text beside it, or with the download it records.
`folio-go/fonts/roboto/NOTICE.md` has carried this same pair since Story 16.8 and explains it the
same way.

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their notices." This
directory's `LICENSE-OFL.txt` and this NOTICE together satisfy that for the shipped file, and are what
`lint/internal/manifest.ResolveAssets` reads to attribute the face in `lint/MANIFEST.md`'s
redistributed non-code assets table.
