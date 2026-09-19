import { brotliCompressSync, constants } from 'node:zlib'
import { existsSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { dirname, join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { serviceWorkerSource } from './offline-service-worker-template.mjs'
import { RELEASE_RUNTIME, isCatalogueAssetUrl, normalizePublicPath, pageIdentity, readAppVersion, releaseIdentity, sha256 } from './offline-release-contract.mjs'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const dist = join(root, 'dist')

const walk = (directory) => readdirSync(directory, { withFileTypes: true }).flatMap((entry) => entry.isDirectory() ? walk(join(directory, entry.name)) : [join(directory, entry.name)])

export function assetsFromDist(outputDir) {
  const assetsDir = join(outputDir, 'assets')
  const assetFiles = walk(assetsDir).filter((file) => !file.endsWith('.br')).sort()
  if (assetFiles.length === 0) throw new Error('production build emitted no runtime assets')
  return [join(outputDir, 'index.html'), ...assetFiles].map((file) => ({ url: normalizePublicPath(relative(outputDir, file)), sha256: sha256(readFileSync(file)), immutable: file !== join(outputDir, 'index.html') }))
}

export function assertPinnedRuntime(version = process.version) {
  if (version !== RELEASE_RUNTIME) throw new Error(`offline release generation requires Node ${RELEASE_RUNTIME}; received ${version}`)
}

export function generateOfflineRelease(outputDir = dist) {
  assertPinnedRuntime()
  if (!existsSync(join(outputDir, 'index.html'))) throw new Error('build output is required before offline release generation')
  const workerRevision = sha256(readFileSync(join(root, 'scripts', 'offline-service-worker-template.mjs')))
  // AUTHORED, AND READ ONCE. `readAppVersion` refuses anything that is not a
  // plain MAJOR.MINOR.PATCH, so a typo in package.json fails the build here
  // rather than shipping a release whose mandatory-ness cannot be evaluated.
  const appVersion = readAppVersion(root)
  const initialAssets = assetsFromDist(outputDir)
  const runtimeAssets = initialAssets.filter((asset) => asset.url !== '/index.html')
  const pageId = pageIdentity(runtimeAssets, workerRevision)
  const thaiDictionary = readFileSync(join(root, '..', 'folio-go', 'internal', 'text', 'data', 'thai_words.trie'))
  const wasm = initialAssets.find((asset) => asset.url.endsWith('.wasm'))
  if (!wasm) throw new Error('production build has no wasm runtime asset')
  const releaseId = releaseIdentity(initialAssets, workerRevision)
  // THE SIDECAR SIZES ARE RECORDED AS THEY ARE WRITTEN (AC5, Story 8.5).
  // Until this story the release said what every asset WEIGHS ON DISK
  // (`s1.cacheAssets`) and what four of them weigh COMPRESSED (the S1 rows),
  // and nothing at all about the compressed weight of the other nineteen — the
  // figure that decides what a first load actually costs over the wire. A
  // catalogue of twenty-one faces makes that gap the majority of the payload.
  //
  // Recorded here rather than re-measured later ON PURPOSE: this is the loop
  // that produces the bytes, so the record cannot describe a sidecar that was
  // never written. `verify-offline-release.mjs` then re-stats every one of them
  // and refuses a release whose record has drifted (`brotli-record-drift`).
  const brotliSidecarBytes = new Map()
  for (const asset of initialAssets.filter((asset) => asset.immutable)) {
    const output = join(outputDir, asset.url.slice(1) + '.br')
    rmSync(output, { force: true })
    writeFileSync(output, brotliCompressSync(readFileSync(join(outputDir, asset.url.slice(1))), { params: { [constants.BROTLI_PARAM_QUALITY]: 11, [constants.BROTLI_PARAM_MODE]: constants.BROTLI_MODE_GENERIC, [constants.BROTLI_PARAM_LGWIN]: 22 } }))
    brotliSidecarBytes.set(asset.url, statSync(output).size)
  }
  // EXACTLY ONE, OR THROW. This was `Array.prototype.find`, which returns the
  // FIRST of several matches and reports nothing about the rest — so a needle
  // matching two emitted assets produced two S1 rows pointing at ONE file, with
  // the second asset in no row at all, and nothing in the build could see it.
  //
  // The trailing dot in every needle below is what keeps `/noto-sans.` off
  // `/noto-sans-bold.`, and the comment there argues that at length — but a
  // convention is not an invariant until something refuses to build when it is
  // broken. A future slot named so that its URL contains an existing needle
  // (`/roboto-italic.` inside a hypothetical `/x/roboto-italic.v2.ttf`, a
  // directory rename, a Vite output layout change) is exactly the silent
  // duplicate this now refuses. The ambiguous case names every match, because
  // "which two" is the whole question a reader will have.
  const find = (needle) => {
    const matches = initialAssets.filter((candidate) => candidate.url.includes(needle))
    if (matches.length === 0) throw new Error(`production build has no ${needle} runtime asset`)
    if (matches.length > 1) throw new Error(`production build has ${matches.length} runtime assets matching ${needle}, and an S1 row must name exactly one: ${matches.map((asset) => asset.url).join(', ')}. The needles are disambiguated by a trailing dot (so /noto-sans. does not reach /noto-sans-bold.); two matches means that no longer holds, and picking the first would emit two rows pointing at one file while the other asset went unrowed.`)
    return matches[0]
  }
  const cachedRow = (id, label, asset) => ({ id, label, delivery: 'cached-asset', assetUrl: asset.url, bytes: statSync(join(outputDir, `${asset.url.slice(1)}.br`)).size, sha256: asset.sha256 })
  const engine = find('.wasm')
  const latin = find('/noto-sans.')
  const thai = find('/noto-sans-thai.')
  const cjk = find('/noto-sans-cjk.')
  // STORY 11.1'S SEVEN CUTS, ONE ROW EACH (D-11.1.15). Itemised rather than
  // aggregated, because `rows` is a MANIFEST SURFACE before it is a screen:
  // `verify-offline-release.mjs` checks the ids and the labels by ordered exact
  // join, and the one question a reader will have after a 26.9% payload growth
  // is WHICH FACE COST WHAT — the single shape an aggregate cannot answer. If
  // twelve rows read badly on the load screen that is a rendering problem with
  // a rendering fix, in the component, never by thinning the record.
  //
  // THE NEEDLES END IN A DOT for the reason `build-wasm.mjs`'s slot labels do:
  // `/noto-sans.` must not reach `/noto-sans-bold.`, and `/noto-sans-thai.`
  // must not reach `/noto-sans-thai-bold.`. `find` throws when a needle matches
  // nothing, so a renamed slot reds here rather than emitting a short manifest.
  //
  // EVERY ID ENDS IN `font` ON PURPOSE. The verifier asserts `cjk-font` is the
  // `Math.max` of the `*font` rows, and an id outside that suffix would quietly
  // shrink that guard's population to the three faces it had before this story
  // — a narrowing dressed as an addition. CJK's Brotli weight (4,948,312) still
  // dominates the largest new cut, and now the guard is measuring that.
  const latinBold = find('/noto-sans-bold.')
  const latinItalic = find('/noto-sans-italic.')
  const latinBoldItalic = find('/noto-sans-bold-italic.')
  const thaiBold = find('/noto-sans-thai-bold.')
  const robotoBold = find('/roboto-bold.')
  const robotoItalic = find('/roboto-italic.')
  const robotoBoldItalic = find('/roboto-bold-italic.')
  const rows = [
    cachedRow('engine', 'Engine', engine),
    cachedRow('latin-font', 'Latin font', latin),
    cachedRow('thai-font', 'Thai font', thai),
    cachedRow('cjk-font', 'CJK font', cjk),
    cachedRow('noto-sans-bold-font', 'Noto Sans Bold', latinBold),
    cachedRow('noto-sans-italic-font', 'Noto Sans Italic', latinItalic),
    cachedRow('noto-sans-bold-italic-font', 'Noto Sans Bold Italic', latinBoldItalic),
    cachedRow('noto-sans-thai-bold-font', 'Noto Sans Thai Bold', thaiBold),
    cachedRow('roboto-bold-font', 'Roboto Bold', robotoBold),
    cachedRow('roboto-italic-font', 'Roboto Italic', robotoItalic),
    cachedRow('roboto-bold-italic-font', 'Roboto Bold Italic', robotoBoldItalic),
  ]
  const visibleBytes = rows.reduce((total, row) => total + row.bytes, 0)
  if (!rows.every((row) => Number.isSafeInteger(row.bytes) && row.bytes > 0)) throw new Error('S1 payload rows are incomplete')
  const s1 = { version: 1, releaseId, pageId, unit: 'MiB', decimals: 2, cachedBytes: 0, assetCount: initialAssets.length, cacheAssets: initialAssets.map((asset) => ({ assetUrl: asset.url, bytes: statSync(join(outputDir, asset.url.slice(1))).size })), rows: [...rows, { id: 'thai-dictionary', label: 'Thai dictionary', delivery: 'embedded-in-engine', assetUrl: engine.url, bytes: thaiDictionary.byteLength, sha256: sha256(thaiDictionary) }] }
  const index = join(outputDir, 'index.html')
  // Regeneration is part of normal local verification. Strip our previous
  // generated bootstrap so a second build:offline run replaces it instead of
  // nesting stale S1 records before the current page identity.
  const originalHtml = readFileSync(index, 'utf8').replace(/<meta name="folio8-page-release"[^>]*>(?:<meta name="folio8-app-version"[^>]*>)?<script id="folio8-release-bootstrap" type="application\/json">[^<]*<\/script>/, '')
  let bootstrappedHtml = originalHtml
  // The bootstrap is cached inside index.html. Its own byte length is the only
  // self-reference, so converge that decimal value before hashing the final page.
  // The S1 payload includes index.html's own emitted size. Keep iterating
  // through the self-reference until both the page bootstrap and manifest
  // describe the same final output (the larger application bundle can require
  // more than the former four passes to settle its decimal widths).
  for (let attempt = 0; attempt < 8; attempt++) {
    const otherBytes = initialAssets.filter((asset) => asset.url !== '/index.html').reduce((total, asset) => total + statSync(join(outputDir, asset.url.slice(1))).size, 0)
    const indexAsset = s1.cacheAssets.find((asset) => asset.assetUrl === '/index.html')
    if (!indexAsset) throw new Error('S1 cache asset list has no navigation entry')
    indexAsset.bytes = s1.cachedBytes - otherBytes
    const bootstrap = `<meta name="folio8-page-release" content="${pageId}"><meta name="folio8-app-version" content="${appVersion}"><script id="folio8-release-bootstrap" type="application/json">${JSON.stringify({ s1 })}</script>`
    bootstrappedHtml = originalHtml.replace('</head>', `${bootstrap}</head>`)
    if (bootstrappedHtml === originalHtml) throw new Error('production index has no head for release bootstrap')
    const nextBytes = otherBytes + Buffer.byteLength(bootstrappedHtml)
    if (nextBytes === s1.cachedBytes) break
    s1.cachedBytes = nextBytes
  }
  writeFileSync(index, bootstrappedHtml)
  const assets = assetsFromDist(outputDir)
  // EVERY IMMUTABLE ASSET CARRIES ITS OWN COMPRESSED WEIGHT (AC5). A missing
  // entry throws rather than defaulting to zero: a record that silently reads
  // "0 bytes" for an asset nobody compressed is worse than no record, because
  // it sums into a total somebody will quote.
  for (const asset of assets) {
    if (!asset.immutable) continue
    const bytes = brotliSidecarBytes.get(asset.url)
    if (!Number.isSafeInteger(bytes) || bytes <= 0) throw new Error(`no Brotli sidecar size was recorded for immutable asset ${asset.url}`)
    asset.brotliBytes = bytes
  }
  // AND THE CATALOGUE'S SHARE OF IT, AS ONE NUMBER (AC5, D-8.4j.8). Story 8.4d
  // owns the threshold and sets it last against the finished weight; this story
  // RECORDS the weight and sets nothing. A subtotal spread over twenty-one rows
  // is a subtotal nobody adds up, which is precisely how 8.4d would inherit a
  // figure it could not use.
  //
  // The catalogue is recognised by the asset-URL prefix `build-wasm.mjs`
  // fingerprints it under, and the count is held to `font-catalogue.json`'s own
  // length — so a prefix change, a dropped face or a Vite naming change reds
  // here rather than quietly reporting the catalogue as weighing nothing.
  const declaredCatalogue = JSON.parse(readFileSync(join(root, 'font-catalogue.json'), 'utf8'))
  const catalogueAssets = assets.filter((asset) => isCatalogueAssetUrl(asset.url))
  if (catalogueAssets.length !== declaredCatalogue.length) throw new Error(`font-catalogue.json declares ${declaredCatalogue.length} catalogue faces and the emitted release carries ${catalogueAssets.length} assets under the catalogue prefix`)
  const immutableAssets = assets.filter((asset) => asset.immutable)
  const brotli = {
    version: 1,
    immutableAssetCount: immutableAssets.length,
    totalBytes: immutableAssets.reduce((total, asset) => total + asset.brotliBytes, 0),
    catalogue: {
      familyCount: catalogueAssets.length,
      // THE ONE NUMBER. Total Brotli bytes the Story 8.5 catalogue adds to the
      // offline release. It is a MEASUREMENT, not a budget, and nothing in this
      // repository compares it to a threshold.
      totalBytes: catalogueAssets.reduce((total, asset) => total + asset.brotliBytes, 0),
    },
  }
  const release = { version: 3, brotli, id: releaseId, pageId, appVersion, workerRevision, thaiDictionary: { delivery: 'emitted-wasm-digest-witness', sha256: sha256(thaiDictionary), wasmUrl: wasm.url, proof: 'the emitted wasm offline-audit operation reports the embedded thai_words.trie digest' }, assets, s1, s1VisibleBytes: visibleBytes }
  writeFileSync(join(outputDir, 'offline-release-manifest.json'), `${JSON.stringify(release, null, 2)}\n`)
  writeFileSync(join(outputDir, 'sw.js'), serviceWorkerSource(release))
  return release
}

if (process.argv[1] === fileURLToPath(import.meta.url)) generateOfflineRelease()
