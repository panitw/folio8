# Roboto-Regular.ttf — shipped production face (Latin, Story 16.8)

**Copyright 2011 The Roboto Project Authors (https://github.com/googlefonts/roboto-classic)**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt`
in this directory, the unmodified upstream licence text).

> **THIS FILE IS BYTE-IDENTICAL TO `folio-designer/public/fonts/roboto/Roboto-Regular.ttf`,
> Story 8.5's catalogue face `roboto` (`font-catalogue.json`), ON PURPOSE.** THERE IS EXACTLY
> ONE ROBOTO (Story 16.8's own boundary): the sha256 below and the designer catalogue's
> committed sha256 must always agree, and `fonts_test.go`'s
> `TestShippedRobotoMatchesDesignerCatalogue` makes that a machine-checked property rather than
> a claim two directories happen to agree on today. **`folio-go/testdata/fonts/Roboto-Regular.ttf`
> is a DIFFERENT cut (sha256 `79e851404657dac2106b3d22ad256d47824a9a5765458edb72c9102a45816d9`,
> 167.7 KB) used only to exercise the Apache-2.0 licence-signature path in
> `internal/fontset/licencesignature_test.go` — it is a test fixture, is never embedded, and
> must never be copied here.**

> **NO DERIVATION APPLIES.** Like the designer's own copy, this file is the upstream file
> itself, byte for byte — the upstream project publishes a **static** TTF instance, so there is
> no instancing step to replay and no toolchain to pin. It was **copied from the designer's
> committed catalogue file**, not re-downloaded, so the two sha256 values below being equal is
> the provenance record, not a coincidence to reconcile.

This is Story 16.8's addition to `fonts.Shipped()`'s FontSet — the fourth shipped production
face, joining the three Story 2.2 Noto faces (`"Noto Sans"`, `"Noto Sans Thai"`,
`"Noto Sans SC"`) — keyed as `"Roboto"`. A new document's starter template
(`folio-designer/public/templates/starter.folio`) declares its default `fonts` chain under the
name `"Roboto"`, over the same three families in the same order, so a new document opens in a
typeface with a name rather than in the internal chain name `body` the starter used before this
story — the Noto Sans Thai and Noto Sans SC fallbacks are unchanged and keep Thai and CJK
rendering exactly as they did before.

**Story 11.3 gave that chain its CUTS**, and the entries are objects rather than bare strings now:
Roboto declares `bold`, `italic` and `boldItalic` (this directory's three sibling cuts), Noto Sans
Thai declares `bold` alone, and Noto Sans SC declares none. The exact contents are not restated
here — `folio-go/starter_template_test.go` reads that file and intersects every name it declares
with `fonts.Shipped()`, which is a check rather than a second copy.

It is a **single upright static Regular**: `OS/2.usWeightClass` 400, `glyf` outlines only, no
`fvar`/`gvar`/`avar`, no `CFF`/`CFF2`. NFR7's glyf/TrueType-over-CFF choice is met, as it is for
the three Noto faces beside it.

**An engine-shipped face needs no embedding** — exactly the rule the three Noto faces already
follow (AD-8, D-2.2). A document naming `"Roboto"` in a chain carries no font bytes of its own;
the engine supplies them from this file. Picking Roboto from the designer's own catalogue
(`AVAILABLE LOCALLY`) is unaffected by this story: that pick still runs through Story 8.6's
first-use embed exactly as any other catalogue face's does, because whether the *catalogue's*
copy of Roboto is embedded on pick is a separate question this story leaves as it found it
(Story 16.8's Ask-First list).

## Provenance — the source

| item | value |
|---|---|
| Upstream project | `github.com/googlefonts/roboto-3-classic`, release `v3.016` |
| Download URL | https://github.com/googlefonts/roboto-3-classic/releases/download/v3.016/Roboto_v3.016.zip |
| Path inside the archive | `android/static/Roboto-Regular.ttf` |
| Fetched (by Story 8.5, into the designer's catalogue) | 2026-09-02 |
| **sha256 of the SOURCE (upstream) file** | `e688a215e0841b6e4edb1207c93f88f4c609f82e870884349a7257e449eb9355` |

## Provenance — the shipped artifact

| item | value |
|---|---|
| **Shipped file** | `Roboto-Regular.ttf` |
| **sha256 of the SHIPPED (produced) file** | `e688a215e0841b6e4edb1207c93f88f4c609f82e870884349a7257e449eb9355` |
| Size | 355,956 bytes |
| Instance | Regular — `OS/2.usWeightClass` 400, upstream static build |
| Declared family (`name` table) | `Roboto` |
| Relation to source | **copied unmodified, no derivation** — all three digests recorded in this
project (this file, the designer's catalogue file, and the upstream source file) are the same
value |

## The licence text beside this file

`LICENSE-OFL.txt` is copied unmodified from
`folio-designer/public/fonts/roboto/LICENSE-OFL.txt` (itself OFL.txt at tag v3.016 of
`github.com/googlefonts/roboto-3-classic`; the release archive carries no licence file).
`lint/internal/manifest.ResolveAssets` reads it and this NOTICE to attribute the face in
`lint/MANIFEST.md`; a directory holding a font binary without both files fails the build (AC25,
AD-26).

AD-26, verbatim: "Redistributed non-code assets keep their own terms and their notices." This
directory's `LICENSE-OFL.txt` and this NOTICE together satisfy that for the shipped file.
