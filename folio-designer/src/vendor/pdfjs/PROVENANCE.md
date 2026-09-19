# Vendored PDF.js source: what was taken, from where, and everything changed in it

**Measured against `mozilla/pdf.js` tag `v6.2.108`, build SHA `0365cbde0`, and against nothing else.**
This document is a claim about two files at one version. `vendor-pin.test.ts` beside it **derives** the
rows below from the files on disk rather than restating them: it re-hashes each `.js`, reads the
expected hash out of *this* manifest, checks the tag here against `pdfjs-dist`'s installed version in
`package.json`, checks that exactly the expected files are present, checks the Apache-2.0 header is
still there, and re-runs `canvas-authority-contract.test.ts`'s own prohibition list over each `.js`.
If any row here stops being true, that test reds naming which one.

Produced by Story 13.6 (`13-6-the-preview-navigates-by-page-thumbnails`), under owner decision
**D-13.6.4**, which narrowed "the thumbnail modules and their dependencies" to exactly these two files.

## Why folio8 vendors source instead of importing it

`pdfjs-dist` ships the thumbnail code only inside `web/pdf_viewer.mjs`, whose stylesheet
(`web/pdf_viewer.css`) references 32 unique images. `vite.config.ts` sets `assetsInlineLimit: 0`, and
folio8's offline release budget (`src/release-payload.ts`) caps the payload at `maximumCacheAssets = 90` (64 when this measurement was taken).
Importing the viewer bundle would add its stylesheet and 32 image rows and fails that budget outright.

**What vendoring these two files actually costs, measured — not predicted.** `npx vite build` followed by
`npm run build:offline`, run outside this story's own gate cadence:

| | |
|---|---|
| `dist/assets/pdf_thumbnail_view-<hash>.js` | 7.56 kB raw / 2.62 kB gzip — **its own chunk** |
| `s1.assetCount` | **62** of `maximumCacheAssets = 64` — **2 free slots** |
| `brotli.immutableAssetCount` | 61 |
| `brotli.totalBytes` | 19,048,890 |

⚠ **One asset row, not zero.** `page-rail.tsx` reaches this module through a DYNAMIC `import()`, so Rollup
emits it as a separate chunk and it appears in the offline manifest as its own row. The dynamic import is
not negotiable: a static one puts `pdfjs-dist/build/pdf.mjs` into `App.tsx`'s module graph, and pdf.js
touches `DOMMatrix` while evaluating, which jsdom does not define — measured, three unit files fail to
load. So the cost is one asset row and ~7.6 kB, against a viewer-bundle route that would have cost 33
rows and roughly 800 kB.

## Upstream

| | |
|---|---|
| repository | `https://github.com/mozilla/pdf.js` |
| tag | `v6.2.108` |
| build SHA | `0365cbde0` (present in both `build/pdf.mjs` and `web/pdf_viewer.mjs` of `pdfjs-dist@6.2.108`) |
| fetched from | `https://raw.githubusercontent.com/mozilla/pdf.js/v6.2.108/web/` |
| licence | Apache License 2.0, unchanged. Full text at `folio-designer/third-party-notices/pdfjs-dist/LICENSE-APACHE-2.0`. |

The tag and the `pdfjs-dist` version in `folio-designer/package.json` are **the same number on purpose**:
the vendored source calls into `pdfjs-dist/build/pdf.mjs` at runtime, so a package bump that leaves this
manifest behind is a fork drifting away from the library it calls. `vendor-pin.test.ts` reds on that.

## The files

Line and byte counts are given for BOTH sides, because the upstream figures alone describe bytes that are
no longer on disk. Only the "as vendored" SHA-256 column is executable — `vendor-pin.test.ts` reads it out
of this table and re-hashes the files; every other column here is narration and is marked as such.
Line counts are `wc -l`.

| file | lines (upstream → vendored) | bytes (upstream → vendored) | SHA-256 (upstream, as fetched) | SHA-256 (as vendored here) |
|---|---|---|---|---|
| `pdf_thumbnail_view.js` | 556 → 569 | 17,105 → 17,991 | `8bb39945f9199f8c35fc1cb7999dc0542ab5859691365dab15e43be241093526` | `acff60ada54732f76f27b8dc52d71a129f2be2d0e947e8b19fa01294b3e0cdda` |
| `renderable_view.js` | 71 → 71 | 1,617 → 1,617 | `b1f630e45648c765a02c8733412118f9a245d1e9dd6b9d034d4e46eadb1cfd79` | `b1f630e45648c765a02c8733412118f9a245d1e9dd6b9d034d4e46eadb1cfd79` |

The story's Code Map records these two files as 557 and 72 lines; both are one high — a `split('\n')`
count includes the empty element after the trailing newline. The figures above are `wc -l` and are what
the files measure on disk.

`renderable_view.js` is **byte-for-byte upstream** — the two hashes are equal because it has zero import
statements and needed no modification at all. `pdf_thumbnail_view.js` carries exactly the three
modifications recorded below and nothing else.

Beside each `.js` sits a hand-written `.d.ts` sibling. Those are folio8's own files, not vendored bytes,
and they declare only the surface folio8 drives. They exist because `allowJs` is false and `strict` is
on, so an undeclared `.js` import is a hard `TS7016`.

## Every modification, and why

Each is marked in the source with a `FOLIO8 MODIFICATION n of 3` comment naming its reason, so the file
itself carries the record and a diff against upstream lands on exactly these three places.

1. **`import … from "pdfjs-lib"` → `from 'pdfjs-dist/build/pdf.mjs'`** (upstream line 24).
   `pdfjs-lib` is a build alias that exists only inside pdf.js's own bundler configuration; it does not
   resolve here. `OutputScale` and `RenderingCancelledException` — the only two names imported — are
   both confirmed exported from `pdfjs-dist/build/pdf.mjs`, the module folio8's viewer already loads, so
   the vendored code shares the already-shipped library rather than bringing a second copy.

2. **The `import { AppOptions } from "./app_options.js"` line is deleted** (upstream line 26).
   `AppOptions` is read at exactly two sites, both supplying a default for a value that is already a
   constructor option. Keeping the import would pull `app_options.js` — 1,082 lines of viewer
   application configuration, none of which folio8 has — into the closure.

3. **The two `AppOptions.get(...)` fallbacks become declared constants** (upstream lines 95-96).
   `DEFAULT_MAX_CANVAS_PIXELS = 2 ** 25` and `DEFAULT_MAX_CANVAS_DIM = 32767` are `app_options.js`'s own
   declared values for `maxCanvasPixels` and `maxCanvasDim` at this tag, copied verbatim.

   ⚠ **One upstream behaviour is genuinely not reproduced, and it is deliberate.** `app_options.js` does
   not only declare those defaults — it also applies a **compatibility override** that lowers
   `maxCanvasPixels` to `5242880` (5 MP) when it detects Android or iOS by user-agent and touch-point
   sniffing. Dropping the module drops that clamp, so a vendored thumbnail on a mobile browser would
   rasterise against the 2**25 (33.5 MP) ceiling rather than the 5 MP one.

   This is immaterial to folio8 and must not be "fixed" by re-adding UA sniffing: `App.css`'s `.app-shell`
   sets `min-width: 1024px`, so the designer is a desktop surface and the Android/iOS branch could never
   fire. It is recorded here because this document's subject is *everything* that changed, and
   "`app_options.js` supplies exactly two numeric defaults" is true of its declarations but not of its
   whole behaviour. A future re-vendor that widens folio8 to a touch viewport must revisit this line.

Nothing else was touched: no reformatting, no comment removal, no dead-code stripping, and the
Apache-2.0 header of each file is byte-for-byte intact.

## What was deliberately NOT vendored, and why

**`pdf_thumbnail_viewer.js` is not here, and taking it later is a decision, not a refresh.** Measured at
this tag it is 1,964 lines of page-editing UI (`#deletePages`, `#cutPages`, `#pastePages`, `#undo`,
`#reportTelemetry`), it produces 36 hits against `canvas-authority-contract.test.ts`'s prohibited
identifiers, and it holds an internal `_currentPageNumber`. That field is the reason more than the size:
`App.tsx`'s `previewViewState` is folio8's single page-state authority, and a vendored file with its own
current-page value would be a second one. `PDFThumbnailView` itself holds no such value —
`toggleCurrent(isCurrent)` is driven entirely by the caller — which is why the rail satisfies the
one-authority rule by construction rather than by discipline.

Also deliberately absent: `pdf_viewer.mjs`, `PDFViewer` and `pdf_viewer.css` (the asset-budget reason at
the top), and `app_options.js`, `ui_utils.js`, `event_utils.js` and `pdf_rendering_queue.js` — folio8
supplies its own three-method adapters for the collaborators the class actually touches.

`vendor-pin.test.ts` asserts this directory holds **exactly six entries**, by name: the two vendored `.js`
files listed above, their two hand-written `.d.ts` siblings (`pdf_thumbnail_view.d.ts`,
`renderable_view.d.ts`), this manifest, and the test itself (`vendor-pin.test.ts`). A seventh file
appearing here reds it, which is what stops the fork growing quietly.

## Re-vendoring procedure

Adding a file to this directory needs owner approval — D-13.6.4 authorised these two and no more. To
move to a new tag:

1. Pick the tag. It must equal the `pdfjs-dist` version in `folio-designer/package.json`; bump the
   package and the source together or not at all.
2. Fetch each file at that tag and record its hash as fetched:
   ```
   for f in pdf_thumbnail_view.js renderable_view.js; do
     curl -fsSL "https://raw.githubusercontent.com/mozilla/pdf.js/<TAG>/web/$f" -o "$f"
     shasum -a 256 "$f"
   done
   ```
3. Update the "SHA-256 (upstream, as fetched)" column, the tag, the build SHA and the line/byte counts.
4. Re-apply the three modifications above, each with its `FOLIO8 MODIFICATION` comment. If upstream has
   changed such that a modification no longer applies, record what replaced it here — do not drop the
   row.
5. Re-hash the modified files and update the "SHA-256 (as vendored here)" column.
6. Re-check the `.d.ts` siblings against the new source: the constructor options, the three collaborator
   call sites and the public methods folio8 drives.
7. Run `npx vitest run src/vendor/pdfjs/vendor-pin.test.ts` — it re-derives every row above — then the
   designer suite, then `e2e/preview-page-rail.spec.ts`, which is the only witness that the vendored
   code actually rasterises.
8. Update `folio-designer/third-party-notices/pdfjs-dist/NOTICE` if the file list changed.
