# Asset tiers

Measured from `folio-designer/dist/offline-release-manifest.json` at release
`eeae8910…`, app version 1.0.0. Bytes are Brotli sidecar bytes — what crosses the wire under
the static-host contract — not the 55.9 MiB uncompressed total.

| | assets | wire bytes |
|---|---|---|
| Whole release before the spec (all blocking) | 80 | 18.63 MiB |
| **Core tier** (blocking) | 30 | 6.10 MiB |
| **Deferred tier** (on demand) | 50 | 7.81 MiB |

## Core tier — 30 assets, 6.10 MiB

Everything needed to open, edit, preview and render an ordinary Latin or Thai document.

| asset | MiB |
|---|---|
| `folio8-engine…wasm` | 3.40 |
| `pdf.worker-….mjs` | 0.36 |
| `noto-sans-bold-italic…ttf` | 0.23 |
| `noto-sans-bold…ttf` | 0.22 |
| `noto-sans-italic…ttf` | 0.22 |
| `noto-sans…ttf` | 0.22 |
| `roboto-bold-italic…ttf` | 0.18 |
| `roboto-italic…ttf` | 0.18 |
| `roboto-bold…ttf` | 0.16 |
| `catalogue-roboto…ttf` | 0.15 |
| `index-….js` | 0.14 |
| `pdf-….js` | 0.10 |
| `pdf_thumbnail_view-….js` | <0.01 |
| `pdfjs-standard-fonts-…/LiberationSans-{Regular,Bold,Italic,BoldItalic}.ttf` | 0.29 (4 files) |
| `ibm-plex-sans…ttf` | 0.07 |
| `ibm-plex-mono…ttf` | 0.06 |
| `ibm-plex-sans-thai…ttf` | 0.04 |
| `noto-sans-thai…ttf` | 0.02 |
| `noto-sans-thai-bold…ttf` | 0.02 |
| `index-….css` | 0.01 |
| `/index.html` | 0.01 |
| `wasm-exec…js` | <0.01 |
| `engine.worker-….js` | <0.01 |
| `pdfjs-cmaps-…/Adobe-{CNS1,GB1,Japan1,Korea1}-0.bcmap` | <0.01 (4 files) |

The three IBM Plex faces are the designer's own UI type, not document faces; they are core
because the shell is unreadable without them. The eleven pdf.js assets — chunk, worker,
thumbnail view, four CMaps and four Liberation faces, 0.76 MiB together — are core by decision:
preview is close enough to the primary workflow that it does not justify a refusal path
through it.

## Deferred tier — 50 assets, 7.81 MiB

| group | assets | MiB | fetched when |
|---|---|---|---|
| `noto-sans-cjk…ttf` | 1 | 4.72 | a document declares a CJK face |
| Catalogue faces (`catalogue-*.ttf`), minus `catalogue-roboto` | 30 | 2.85 | that family is picked, or a document declares it |
| Bundled examples (`.folio`, `.sample.json`, thumbnails) + starter | 13 | 0.14 | the examples gallery is opened |
| Bundled documentation HTML | 6 | 0.10 | a documentation page is opened |

The CJK font alone is 60% of the deferred tier and 25% of today's whole first load.

## What the numbers came from

```
python3 - <<'PY'
import os, json
m = json.load(open('folio-designer/dist/offline-release-manifest.json'))
def wire(u):
    p = 'folio-designer/dist' + u
    return os.path.getsize(p + '.br') if os.path.exists(p + '.br') else os.path.getsize(p)
print(sum(wire(a['url']) for a in m['assets']), len(m['assets']))
PY
```

Re-measure after any build that changes the engine or the catalogue: `catalogue.familyCount`
in the manifest has already moved 21 → 31 since the figure `spec-folio` records.

**Corrected 2026-09-19.** The first version of this table read 28 core / 52 deferred, because
the grouping script above swept `/assets/pdf_thumbnail_view-<hash>.js` — a 2.2 KiB pdf.js
preview chunk — into the bundled-examples group on the substring `thumbnail`. pdf.js is core,
so the counts were off by one in each direction. The MiB figures were never wrong: 2.2 KiB does
not show at two decimals, which is exactly why the error survived. Since story 1 the release
carries a `tier` per asset, so these counts are now read from the manifest rather than
re-derived by pattern:

```
python3 -c "import json,collections; m=json.load(open('folio-designer/dist/offline-release-manifest.json')); print(collections.Counter(a['tier'] for a in m['assets']))"
```

**Re-measured 2026-09-19 (story 3).** `catalogue-roboto` — the canvas copy of Roboto, 0.152 MiB
— moved from `deferred` to `core`, taking the tiers from 29 / 10.67 and 51 / 7.96 to 30 / 10.82
and 50 / 7.81. `runtime-fonts.css` maps the family `Roboto` to that catalogue face while
`Roboto Bold`, `Roboto Italic` and `Roboto Bold Italic` map to the shipped core files, so story
1 left one family straddling the two tiers — and the starter and all four bundled examples
declare that chain, which meant the default document needed a deferred fetch to paint its body
text. The figures above are read from a `npm run build` manifest, not re-derived by pattern.

**Re-measured 2026-09-19 (story 5).** The designer's engine wasm is now built `-tags nocjkface`
and no longer embeds `Noto Sans SC`: 8,508,122 Brotli bytes (8.11 MiB) → 3,564,400 (3.40 MiB),
taking the core tier from 30 / 11,335,794 bytes (10.81 MiB) to 30 / 6,392,910 (6.10 MiB) and the
whole release from 18.62 MiB to 13.91 MiB. The COUNTS did not move — the wasm is still one asset,
only smaller — and the deferred tier is untouched at 50 / 7.81 MiB, because the CJK face was
always already there: the release served those exact bytes twice, and now serves them once.

What the tagged engine keeps is the face's THREE hhea integers
(`folio-go/internal/fontset/declared_metrics.go`), so a chain naming the face measures the same
vertical model with or without its glyphs. A Latin or Thai session therefore fetches nothing at
all; the 4.72 MiB deferred asset is fetched only when a CJK rune must actually be drawn.

The core tier's WEIGHT is guarded from this story on. `maximumCoreCacheBytes` in
`folio-designer/src/release-payload.ts` is 6,553,600 (6.25 MiB); `generate-offline-release.mjs`
refuses to emit a release over it and `verify-offline-release.mjs` refuses to pass one, both
reading that one declaration as text through `declaredCoreCacheByteCeiling()`. Before this story
nothing in the repository compared the core tier's size to anything at all, which is how 4.72 MiB
of duplicated CJK glyphs could sit in the blocking download unnoticed.
