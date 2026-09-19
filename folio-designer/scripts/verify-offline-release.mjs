import { brotliCompressSync, brotliDecompressSync, constants } from 'node:zlib'
import { existsSync, readFileSync, readdirSync, rmSync, unlinkSync, writeFileSync } from 'node:fs'
import { basename, dirname, join, relative } from 'node:path'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import { execFileSync } from 'node:child_process'
import { serviceWorkerSource } from './offline-service-worker-template.mjs'
import { assertPinnedRuntime, generateOfflineRelease } from './generate-offline-release.mjs'
import { assertNoVCSStamp, buildEngineWasm } from './wasm-vcs-stamp.mjs'
import { FORBIDDEN_FONT_HOSTS } from './forbidden-font-hosts.mjs'
import { exampleIds } from './build-examples.mjs'
import { RELEASE_RUNTIME, declaredCacheAssetBounds, declaredCacheAssetWarning, isCatalogueAssetUrl, pageIdentity, parseAppVersion, releaseIdentity, sha256 } from './offline-release-contract.mjs'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const dist = join(root, 'dist')
const fail = (message) => { throw new Error(`offline release verification failed: ${message}`) }
// Kept in sync with the `import.meta.env.DEV` offline bypass in src/main.tsx,
// src/App.tsx, and src/offline-lifecycle.ts.
const DEV_BYPASS_MARKERS = ['dev-bypass', 'Offline layer bypassed']
// THE STARTER AND THE BUNDLED EXAMPLES. The starter is required by name — a
// generic `.folio` class would be satisfied by an example template alone — and
// every listed example must ship exactly one immutable template, sample and
// thumbnail, so an example silently dropped from Vite's asset graph fails here.
// Returns the first finding, or null.
export function templateAssetFinding(assets, ids = exampleIds) {
  if (!assets.some((asset) => /\/assets\/starter\.[a-f0-9]{20}-[^/]+\.folio$/.test(asset.url))) return 'missing the starter template runtime asset'
  for (const id of ids) {
    for (const [kind, pattern] of [['template', `${id}\\.[a-f0-9]{20}-[^/]+\\.folio`], ['sample', `${id}\\.sample\\.[a-f0-9]{20}-[^/]+\\.json`], ['thumbnail', `${id}\\.thumbnail\\.[a-f0-9]{20}-[^/]+\\.png`]]) {
      const found = assets.filter((asset) => new RegExp(`^/assets/${pattern}$`).test(asset.url))
      if (found.length !== 1 || !found[0].immutable) return `example '${id}' must ship exactly one immutable ${kind} asset (found ${found.length})`
    }
  }
  return null
}
const sameSet = (left, right) => left.size === right.size && [...left].every((value) => right.has(value))
const brotliOptions = { params: { [constants.BROTLI_PARAM_QUALITY]: 11, [constants.BROTLI_PARAM_MODE]: constants.BROTLI_MODE_GENERIC, [constants.BROTLI_PARAM_LGWIN]: 22 } }
const walk = (directory) => readdirSync(directory, { withFileTypes: true }).flatMap((entry) => entry.isDirectory() ? walk(join(directory, entry.name)) : [join(directory, entry.name)])
const runtimeOutputUrls = (outputDir) => new Set(['/index.html', ...walk(join(outputDir, 'assets')).filter((file) => !file.endsWith('.br')).map((file) => `/${relative(outputDir, file).replaceAll('\\', '/')}`)])

/**
 * THE CACHE-ASSET APPROACH WARNING (Story 11.1, D-11.1.10), AS A FUNCTION A
 * TEST CAN EXECUTE.
 *
 * The bound in `verifyOfflineRelease` REFUSES a release; this only says how
 * much room is left before it would. It ships in this story because THIS is
 * the story that knows the margin is about to be small: seven cuts take the
 * release from 54 slots to 61 of 64, and 11.3 is the story that will be busy
 * spending what is left.
 *
 * THE THRESHOLD IS NEVER A LITERAL. It is read out of `src/release-payload.ts`
 * by the same line-anchored reader the two bounds use, so the number has ONE
 * authority and a second copy cannot drift from it.
 *
 * The message names the MARGIN rather than the count, because the margin is
 * the quantity that goes stale unwatched: DW-162's figure aged 41 -> 20 -> 10
 * while three stories walked past it, precisely because the number nobody
 * printed was the number nobody watched.
 *
 * IT IS A SEPARATE, EXPORTED FUNCTION AND THAT IS THE POINT. Inline in
 * `verifyOfflineRelease` it was realized by code no test could reach — the
 * only way to execute it was a full `npm run build` — while being the sole
 * realization of an acceptance criterion. Extracted, `scripts/verify-offline-release.test.mjs`
 * drives it directly at the threshold and one below it, and `warn` is injected
 * so the emission itself is observable rather than inferred from stderr.
 *
 * Returns the emitted message, or `null` when the count is below the
 * threshold, so a caller and a test can both tell silence from a warning
 * without parsing console output.
 */
export function reportCacheAssetApproach(assetCount, { warn = console.warn } = {}) {
  const { warnCacheAssets, maximumCacheAssets } = declaredCacheAssetWarning()
  if (assetCount < warnCacheAssets) return null
  const message = `offline release approach warning: the release carries ${assetCount} cache assets against a declared maximum of ${maximumCacheAssets} — the margin is ${maximumCacheAssets - assetCount}. The warning threshold is \`warnCacheAssets\` = ${warnCacheAssets} in src/release-payload.ts; nothing fails until the maximum is exceeded.`
  warn(message)
  return message
}

// THE PRECACHED DOCUMENTATION PAGES CARRY NO REMOTE FONT HOST. The source scan
// runs before build-wasm copies the pages and does not walk `docs/`, so this is
// the one check that reads the bytes the release actually ships. It returns the
// first offending `{ url, host }`, or null.
export function documentationFontHostFinding(pages) {
  for (const { url, html } of pages) {
    const found = FORBIDDEN_FONT_HOSTS.find(({ host }) => html.includes(host))
    if (found) return { url, host: found.host }
  }
  return null
}

export function verifyOfflineRelease(outputDir = dist, { wasmWitness = false, reportApproach = false } = {}) {
  assertPinnedRuntime()
  const manifestFile = join(outputDir, 'offline-release-manifest.json')
  if (!existsSync(manifestFile)) fail('missing generated manifest')
  const release = JSON.parse(readFileSync(manifestFile, 'utf8'))
  const contract = JSON.parse(readFileSync(join(root, 'static-host-contract.json'), 'utf8'))
  if (release.version !== 3 || !Array.isArray(release.assets) || release.assets.length < 2 || !/^[a-f0-9]{64}$/.test(release.id) || !/^[a-f0-9]{64}$/.test(release.pageId) || !/^[a-f0-9]{64}$/.test(release.workerRevision)) fail('manifest has no complete release identity')
  const manifestUrls = new Set()
  for (const asset of release.assets) {
    if (!asset || typeof asset.url !== 'string' || !asset.url.startsWith('/') || asset.url.includes('\\') || manifestUrls.has(asset.url)) fail('manifest URL is duplicate or invalid')
    manifestUrls.add(asset.url)
  }
  // THE BOUND THE PARSER DECLARES, ENFORCED WHERE A RELEASE IS STILL REFUSABLE.
  // src/release-payload.ts refuses a payload outside [minimum, maximum] cache
  // assets; until this check existed nothing counted, so crossing it was a
  // build-time silence followed by a runtime one.
  //
  // (a) The count is `release.assets.length`, not `s1.assetCount`: the S1 block
  // below already ties them (`s1.assetCount !== release.assets.length` fails
  // there), so bounding this bounds exactly the quantity parseS1Payload bounds —
  // and it is available here, before anything that could trip first.
  //
  // (b) It sits BEFORE the manifest/output set comparison on the next line
  // ON PURPOSE. A release manufactured over the bound necessarily breaks that
  // exact-set check, and the per-asset digest and Brotli loops further down; a
  // bound check placed after any of them could never be red-proved on its own
  // message. Placement is what makes the proof mean something.
  const { minimumCacheAssets, maximumCacheAssets } = declaredCacheAssetBounds()
  if (release.assets.length > maximumCacheAssets) fail(`release carries ${release.assets.length} cache assets, over the declared maximum of ${maximumCacheAssets}`)
  if (release.assets.length < minimumCacheAssets) fail(`release carries ${release.assets.length} cache assets, under the declared minimum of ${minimumCacheAssets}`)
  // THE APPROACH WARNING, REPORTED ONLY WHEN THE CALLER SAYS THIS IS THE REAL
  // RELEASE. It WARNS and does not fail, and the two are deliberately
  // different outcomes: crossing 64 is a release that the browser's own parser
  // would refuse, while approaching it is information a person needs before
  // planning the next story.
  //
  // `reportApproach` IS AN EXPLICIT OPTION, NOT A FIRST-CALL ASSUMPTION.
  // `runRedProofs` calls `verifyOfflineRelease` two dozen times over a
  // deliberately mutated dist, and an unguarded warning prints two dozen lines
  // about asset counts no release will ever have. The previous shape suppressed
  // those with a module-scope latch set inside this branch — so it latched on
  // the first FIRING rather than the first CALL, and under
  // `npm run verify:offline:red:controls` (`--red-only`) the real release is
  // never verified at all, which meant the one warning line printed was about a
  // deliberately mutated fixture. A warning that describes a fixture while
  // reading as a statement about the release is worse than no warning. Now only
  // the CLI's real-release call asks for it, and a fixture cannot consume it
  // because a fixture never requests it.
  if (reportApproach) reportCacheAssetApproach(release.assets.length)
  const outputUrls = runtimeOutputUrls(outputDir)
  if (!sameSet(manifestUrls, outputUrls)) fail('manifest and production runtime output are not an exact set')
  if (!manifestUrls.has('/index.html')) fail('navigation entry is absent')
  for (const required of ['.wasm', '.css', '.js', '.ttf']) if (!release.assets.some((asset) => asset.url.endsWith(required))) fail(`missing required runtime class ${required}`)
  const templateFinding = templateAssetFinding(release.assets)
  if (templateFinding) fail(templateFinding)
  if (!release.assets.some((asset) => /\/pdf\.worker-[A-Za-z0-9_-]+\.mjs$/.test(asset.url))) fail('missing local PDF.js worker runtime asset')
  if (release.assets.filter((asset) => asset.url.endsWith('.bcmap')).length < 4) fail('missing local PDF.js CMap runtime assets')
  if (!release.assets.some((asset) => /\/pdfjs-standard-fonts-[a-f0-9]{20}\/LiberationSans-Regular\.ttf$/.test(asset.url))) fail('missing local PDF.js standard-font runtime asset')
  // THE BUNDLED DOCUMENTATION: the Go guide, the folio-js and folio-dotnet
  // guides, the format reference and the expression reference are each one
  // precached, content-addressed page, and every link between them names a page
  // this release actually carries — a link left at its canonical `docs/` name
  // would be a dead link offline.
  const documentationStems = ['rendering-library', 'folio-js', 'folio-dotnet', 'folio-format', 'expression-reference']
  for (const stem of documentationStems) {
    const pages = release.assets.filter((asset) => new RegExp(`^/assets/${stem}-[a-f0-9]{20}\\.html$`).test(asset.url))
    if (pages.length !== 1 || !pages[0].immutable) fail(`missing precached documentation page ${stem} (found ${pages.length})`)
    const html = readFileSync(join(outputDir, pages[0].url.slice(1)), 'utf8')
    for (const [, target] of html.matchAll(/\bhref\s*=\s*["']([^"'#]*)/g)) {
      if (!documentationStems.some((candidate) => target.includes(candidate))) continue
      if (!manifestUrls.has(`/assets/${target}`)) fail(`documentation page ${pages[0].url} links to ${target}, which this release does not carry`)
    }
  }
  const precachedPages = release.assets.filter((asset) => /^\/assets\/[^/]+\.html$/.test(asset.url)).map((asset) => ({ url: asset.url, html: readFileSync(join(outputDir, asset.url.slice(1)), 'utf8') }))
  const fontHost = documentationFontHostFinding(precachedPages)
  if (fontHost) fail(`precached page ${fontHost.url} references remote font host ${fontHost.host}`)
  if (release.id !== releaseIdentity(release.assets, release.workerRevision)) fail('release identity does not match canonical assets and worker revision')
  if (release.pageId !== pageIdentity(release.assets, release.workerRevision)) fail('page release identity does not match runtime assets and worker revision')
  if (!readFileSync(join(outputDir, 'index.html'), 'utf8').includes(`name="folio8-page-release" content="${release.pageId}"`)) fail('page does not bind to its release identity')
  const indexHtml = readFileSync(join(outputDir, 'index.html'), 'utf8')
  const bootstrap = indexHtml.match(/<script id="folio8-release-bootstrap" type="application\/json">([^<]+)<\/script>/)?.[1]
  if (!bootstrap) fail('page has no cached S1 release bootstrap')
  let bootS1
  try { bootS1 = JSON.parse(bootstrap).s1 } catch { fail('page S1 bootstrap is not JSON') }
  const sw = readFileSync(join(outputDir, 'sw.js'), 'utf8')
  const embedded = sw.match(/const RELEASE = (.+)\nconst CACHE_NAME/m)?.[1]
  if (!embedded || JSON.stringify(JSON.parse(embedded)) !== JSON.stringify(release)) fail('service worker and manifest release records differ')
  for (const required of ["const CACHE_NAME = 'folio8-release-' + RELEASE.id", "credentials: 'omit'", 'url.origin === origin', 'paths.has(url.pathname)', 'RELEASE.pageId', 'windows.length === 0', 'offline asset integrity mismatch']) if (!sw.includes(required)) fail(`service worker lacks ${required}`)
  if (sw.includes('cache.addAll') || sw.includes('fetch(event.request)')) fail('service worker has a generic network fallback')
  // `skipWaiting` USED TO BE BANNED OUTRIGHT, and the ban is now a LEASH rather
  // than a wall: exactly one occurrence, in exactly the author-gated line below.
  // The property that mattered is unchanged — nothing the BROWSER does on its
  // own can retire a release out from under an open document — because the only
  // caller is a message an author's own tab sends after clearing its document.
  // Counting is the point: a second occurrence, anywhere, is the fault this
  // check exists to catch, and it would almost certainly be in `install`.
  const skipWaitingUses = sw.split('skipWaiting').length - 1
  const gatedActivation = 'if (activateRequest(event.data)) { self.skipWaiting(); return }'
  if (skipWaitingUses !== 1 || !sw.includes(gatedActivation)) fail(`service worker must reach skipWaiting exactly once, through \`${gatedActivation}\`; found ${skipWaitingUses} use(s)`)
  for (const handler of ['install', 'activate']) {
    const start = sw.indexOf(`self.addEventListener('${handler}'`)
    const end = sw.indexOf("self.addEventListener('", start + 1)
    if (start < 0 || sw.slice(start, end < 0 ? undefined : end).includes('skipWaiting')) fail(`service worker ${handler} handler must not reach skipWaiting`)
  }
  // THE VERSION THAT DECIDES WHETHER AN UPGRADE IS MANDATORY, proved to exist and
  // to agree between the record, the worker and the page. A release whose three
  // copies disagree could tell one tab it is optional and another that it is
  // required, so disagreement fails the build rather than shipping.
  parseAppVersion(release.appVersion, 'release appVersion')
  if (!indexHtml.includes(`name="folio8-app-version" content="${release.appVersion}"`)) fail('page does not carry the release app version')
  const markerWrite = sw.indexOf("await cache.put(MARKER")
  const finalVerified = sw.indexOf("await progress('verified', activeAsset)")
  if (markerWrite < 0 || finalVerified < markerWrite) fail('emitted worker can report 100% before its complete marker')
  if (release.thaiDictionary?.delivery !== 'emitted-wasm-digest-witness' || !manifestUrls.has(release.thaiDictionary.wasmUrl) || !/^[a-f0-9]{64}$/.test(release.thaiDictionary.sha256)) fail('Thai dictionary containment is not declared against the emitted wasm')
  const s1 = release.s1
  // THE ORDERED SHAPE OF THE S1 ROWS, extended by Story 11.1's seven cuts.
  // `thai-dictionary` stays LAST and `engine` stays FIRST: the two rows the
  // checks below single out are addressed BY ID from here on, so nothing depends
  // on those positions any more — but the ordered join is still what states the
  // manifest's shape, and an id inserted in the wrong place reds it.
  const s1Ids = ['engine', 'latin-font', 'thai-font', 'cjk-font', 'noto-sans-bold-font', 'noto-sans-italic-font', 'noto-sans-bold-italic-font', 'noto-sans-thai-bold-font', 'roboto-bold-font', 'roboto-italic-font', 'roboto-bold-italic-font', 'thai-dictionary']
  if (!s1 || JSON.stringify(bootS1) !== JSON.stringify(s1) || s1.version !== 1 || s1.releaseId !== release.id || s1.pageId !== release.pageId || s1.unit !== 'MiB' || s1.decimals !== 2 || s1.assetCount !== release.assets.length || !Array.isArray(s1.cacheAssets) || !Array.isArray(s1.rows) || s1.rows.length !== s1Ids.length || s1.rows.map((row) => row.id).join(',') !== s1Ids.join(',')) fail('S1 payload metadata is incomplete or not exactly page/release bound')
  const semanticLabels = ['Engine', 'Latin font', 'Thai font', 'CJK font', 'Noto Sans Bold', 'Noto Sans Italic', 'Noto Sans Bold Italic', 'Noto Sans Thai Bold', 'Roboto Bold', 'Roboto Italic', 'Roboto Bold Italic', 'Thai dictionary']
  if (s1.rows.map((row) => row.label).join(',') !== semanticLabels.join(',') || s1.rows.some((row) => /cloud|download|account|sync/i.test(row.label))) fail('S1 semantic labels contain delivery fiction')
  if (s1.cacheAssets.length !== release.assets.length || new Set(s1.cacheAssets.map((asset) => asset.assetUrl)).size !== release.assets.length) fail('S1 cache assets are incomplete')
  for (const asset of release.assets) {
    const cacheAsset = s1.cacheAssets.find((candidate) => candidate.assetUrl === asset.url)
    if (!cacheAsset || cacheAsset.bytes !== readFileSync(join(outputDir, asset.url.slice(1))).byteLength) fail(`S1 cache denominator is not the emitted release ${asset.url}`)
  }
  if (s1.cachedBytes !== s1.cacheAssets.reduce((total, asset) => total + asset.bytes, 0)) fail('S1 cache denominator is not all release assets')
  const cachedRows = s1.rows.filter((row) => row.delivery === 'cached-asset')
  // DERIVED FROM `s1Ids`, NEVER RE-TYPED (D-11.1.16, extended). Every row but
  // `thai-dictionary` is a cached asset, so the expected cardinality IS
  // `s1Ids.length - 1` — the same expression `src/release-payload.ts` already
  // uses for the same quantity (`cached.length !== ids.length - 1`). It was a
  // literal `11` here, which is a second copy of a number this same function
  // declares eight lines above: an id added to `s1Ids` without touching this
  // line would fail with "S1 cached rows are invalid" about a release that is
  // correct, and the fix would look like relaxing the guard. This is the same
  // positional-literal defect the story spent five bullet points removing from
  // the row reads, one layer up.
  const expectedCachedRows = s1Ids.length - 1
  if (cachedRows.length !== expectedCachedRows || !cachedRows.every((row) => typeof row.assetUrl === 'string' && Number.isSafeInteger(row.bytes) && row.bytes > 0 && /^[a-f0-9]{64}$/.test(row.sha256))) fail(`S1 cached rows are invalid: expected ${expectedCachedRows} cached-asset rows (every id in the declared shape but the one embedded row) and found ${cachedRows.length}`)
  for (const row of cachedRows) {
    const asset = release.assets.find((candidate) => candidate.url === row.assetUrl)
    if (!asset || asset.sha256 !== row.sha256) fail(`S1 row is not bound to an emitted asset ${row.id}`)
    if (readFileSync(`${join(outputDir, row.assetUrl.slice(1))}.br`).byteLength !== row.bytes) fail(`S1 row size is not its emitted Brotli sidecar ${row.id}`)
  }
  // KEYED BY ID, NOT BY POSITION (D-11.1.16). This read was `s1.rows[4]`; the
  // index moved once at Story 11.1 and will move again at 11.3, and an index
  // that has drifted onto another row does not fail — it checks the WRONG ROW
  // and passes. The file's own better convention is two lines below
  // (`rows.find((row) => row.id === 'cjk-font')`) and is adopted here.
  const dictionaryRow = s1.rows.find((row) => row.id === 'thai-dictionary')
  if (!dictionaryRow || dictionaryRow.delivery !== 'embedded-in-engine' || dictionaryRow.assetUrl !== release.thaiDictionary.wasmUrl || dictionaryRow.bytes !== readFileSync(join(root, '..', 'folio-go', 'internal', 'text', 'data', 'thai_words.trie')).byteLength || dictionaryRow.sha256 !== release.thaiDictionary.sha256) fail('S1 Thai dictionary row is not a real embedded witness')
  if (release.s1VisibleBytes !== cachedRows.reduce((total, row) => total + row.bytes, 0)) fail('S1 visible payload total is not row arithmetic')
  const cjk = s1.rows.find((row) => row.id === 'cjk-font')
  if (!cjk || cjk.bytes !== Math.max(...cachedRows.filter((row) => row.id.endsWith('font')).map((row) => row.bytes))) fail('S1 CJK row is not the dominant font payload')
  const thaiSource = readFileSync(join(root, '..', 'folio-go', 'internal', 'text', 'data', 'thai_words.trie'))
  if (sha256(thaiSource) !== release.thaiDictionary.sha256) fail('Thai dictionary audit digest is stale')
  for (const asset of release.assets) {
    const file = join(outputDir, asset.url.slice(1))
    if (!existsSync(file)) fail(`missing manifest asset ${asset.url}`)
    if (sha256(readFileSync(file)) !== asset.sha256) fail(`stale asset digest ${asset.url}`)
  }
  for (const asset of release.assets) {
    const file = join(outputDir, asset.url.slice(1))
    if (asset.immutable) {
      const pdfjsCollection = /\/pdfjs-(?:cmaps|standard-fonts)-[a-f0-9]{20}\//.test(asset.url)
      if (!asset.url.startsWith(contract.immutableRuntime.urlPrefix) || (!/-[A-Za-z0-9_-]{8,}\.[A-Za-z0-9]+$/.test(asset.url) && !pdfjsCollection)) fail(`immutable URL is not content-addressed ${asset.url}`)
      const sidecar = `${file}${contract.immutableRuntime.sidecarSuffix}`
      if (!existsSync(sidecar)) fail(`missing Brotli sidecar ${asset.url}`)
      const original = readFileSync(file)
      const compressed = readFileSync(sidecar)
      if (!brotliDecompressSync(compressed).equals(original)) fail(`stale Brotli sidecar ${asset.url}`)
      if (!brotliCompressSync(original, brotliOptions).equals(compressed)) fail(`non-deterministic Brotli sidecar ${asset.url} under ${RELEASE_RUNTIME}`)
      // AC5, Story 8.5. THE RECORD IS HELD TO THE SIDECAR IT DESCRIBES.
      // `generate-offline-release.mjs` writes each asset's compressed weight
      // into the manifest as it writes the sidecar; an unasserted record drifts
      // silently, and a per-asset weight table nobody checks is decoration
      // rather than evidence. Red-proved as `brotli-record-drift`.
      //
      // It sits LAST inside this loop deliberately: the decompress and
      // re-compress checks above own the "the sidecar is wrong" direction, so
      // reaching here means the sidecar is right and the only thing left that
      // can be wrong is the RECORD OF IT. That is what makes the red proof
      // provable on this guard's own message rather than on a neighbour's.
      if (!Number.isSafeInteger(asset.brotliBytes) || asset.brotliBytes <= 0) fail(`brotli-record-drift: ${asset.url} carries no recorded Brotli byte count`)
      if (asset.brotliBytes !== compressed.byteLength) fail(`brotli-record-drift: ${asset.url} records ${asset.brotliBytes} Brotli bytes and its emitted sidecar is ${compressed.byteLength}`)
    } else if (asset.url !== '/index.html') {
      fail(`unexpected mutable runtime entry ${asset.url}`)
    } else if (asset.brotliBytes !== undefined) {
      fail(`brotli-record-drift: ${asset.url} is not immutable, carries no Brotli sidecar, and must not record a Brotli byte count`)
    }
  }
  // AND THE TOTALS ARE THE ROWS' OWN ARITHMETIC, on the shape
  // `release.s1VisibleBytes` already uses: a headline number that is not the sum
  // of the rows under it is the one figure a reader will quote and nobody will
  // re-derive. The catalogue subtotal is the number Story 8.4d inherits (AC5).
  const immutable = release.assets.filter((asset) => asset.immutable)
  const catalogue = release.assets.filter((asset) => isCatalogueAssetUrl(asset.url))
  const brotli = release.brotli
  if (!brotli || brotli.version !== 1 || !brotli.catalogue) fail('brotli-record-drift: the release carries no per-asset Brotli record')
  if (brotli.immutableAssetCount !== immutable.length) fail(`brotli-record-drift: the Brotli record counts ${brotli.immutableAssetCount} immutable assets and the release carries ${immutable.length}`)
  if (brotli.totalBytes !== immutable.reduce((total, asset) => total + asset.brotliBytes, 0)) fail('brotli-record-drift: the recorded Brotli total is not the per-asset rows\' arithmetic')
  // A CATALOGUE OF NOTHING MUST NOT READ AS A CATALOGUE THAT COSTS NOTHING.
  // Every other check here would pass over a release with zero catalogue faces
  // and a subtotal of zero, which is exactly the vacuous green Story 8.5's own
  // design notes name as the trap.
  if (catalogue.length === 0) fail('brotli-record-drift: the release carries no Story 8.5 catalogue face at all, so its recorded catalogue weight describes nothing')
  if (brotli.catalogue.familyCount !== catalogue.length) fail(`brotli-record-drift: the Brotli record counts ${brotli.catalogue.familyCount} catalogue faces and the release carries ${catalogue.length}`)
  if (brotli.catalogue.totalBytes !== catalogue.reduce((total, asset) => total + asset.brotliBytes, 0)) fail('brotli-record-drift: the recorded catalogue Brotli total is not the catalogue rows\' arithmetic')
  if (wasmWitness) {
    const glue = release.assets.find((asset) => asset.url.includes('/wasm-exec.'))
    if (!glue) fail('wasm runtime glue is absent from the release')
    const wasmDigest = execFileSync(process.execPath, [join(root, 'scripts', 'verify-wasm-dictionary.mjs'), join(outputDir, glue.url.slice(1)), join(outputDir, release.thaiDictionary.wasmUrl.slice(1))], { encoding: 'utf8', timeout: 30000 }).trim()
    if (wasmDigest !== release.thaiDictionary.sha256) fail('emitted wasm dictionary witness does not match the shipped dictionary')
    assertEngineWasmIsTreeIndependent()
  }
  // The dev-server offline bypass is gated on `import.meta.env.DEV` so the
  // branch and its strings are eliminated from production bundles. Prove that
  // elimination against the emitted release rather than trusting the gate.
  for (const asset of release.assets) {
    if (!/\.(?:js|mjs|html|css)$/.test(asset.url)) continue
    const text = readFileSync(join(outputDir, asset.url.slice(1)), 'utf8')
    for (const marker of DEV_BYPASS_MARKERS) if (text.includes(marker)) fail(`development offline bypass shipped in ${asset.url}`)
  }
  if (!contract.immutableRuntime.cacheControl.includes('immutable') || contract.immutableRuntime.contentEncoding !== 'br') fail('immutable host policy is incomplete')
  if (!contract.updateableEntries.some((entry) => entry.url === '/index.html' && !entry.cacheControl.includes('immutable')) || !contract.updateableEntries.some((entry) => entry.url === '/sw.js' && !entry.cacheControl.includes('immutable'))) fail('updateable entries have immutable policy')
  return release
}

// DW-106. build-wasm.mjs guards ONE CAUSE — the absence of four VCS needles in the
// emitted wasm. The PROPERTY that matters is that two builds of one commit agree,
// and a guard keyed on a proxy rather than on its purpose is a defect the
// engineering lead recorded against its own ruling (D-8.4.30). So assert the
// property itself: build the engine twice, with the tree state DELIBERATELY
// DIFFERENT between the two runs, and require the digests to agree — reporting
// both, so a failure names what moved.
//
// ITS LIMIT, STATED RATHER THAN OVERCLAIMED: this would NOT have caught DW-105.
// It holds the checkout PATH fixed by construction, and DW-105 is path dependence
// — one commit built from three different paths gives three different binaries,
// each embedding 87 occurrences of its own absolute root. The value of this check
// is the NEXT tree-dependent input, not the one already measured.
//
// AND A SECOND LIMIT, BECAUSE SILENCE ABOUT IT WOULD REBUILD THIS RUN'S SIGNATURE
// DEFECT INSIDE THE FIX FOR IT: both arms build with -buildvcs=false, which closes
// THE ONLY tree→binary channel ever measured here, so the stray file has no known
// route into the output and this check is expected to be quiet. It watches for a
// FUTURE tree-dependent input, not for the closed one. The probe's capability to
// move a binary was measured at Story 8.4g's close (with the flag absent, a stray
// untracked file changed the digest) and is deliberately NOT re-measured here —
// re-measuring it would mean building without the flag, which is DW-107's job
// below. What IS re-measured every run is that the probe is still VISIBLE to git,
// because a probe git cannot see perturbs nothing and would turn this into an
// all-clear indistinguishable from a couldn't-look.
//
// ⚠ THE PROBE FILENAME MUST NEVER BE ADDED TO .gitignore. Its visibility to git
// IS the perturbation: Go derives `vcs.modified` from `git status`, so an ignored
// probe makes both builds see an identical tree and this check assert nothing.
//
// It runs only under --wasm-witness. `build:wasm` is a dependency of `typecheck`,
// `test` AND `build`, so a second engine build there would tax every designer
// gate; this is the home that runs once.
function assertEngineWasmIsTreeIndependent() {
  const first = join(tmpdir(), `folio8-engine-tree-state-a-${process.pid}.wasm`)
  const second = join(tmpdir(), `folio8-engine-tree-state-b-${process.pid}.wasm`)
  // An untracked file is enough: Go derives `vcs.modified` from `git status`.
  const stray = join(root, '..', 'folio-go', `.folio-tree-state-probe-${process.pid}`)
  try {
    buildEngineWasm(first, { stdio: 'pipe' })
    writeFileSync(stray, 'transient tree-state probe written by verify-offline-release.mjs\n')
    // THE PERTURBATION IS MEASURED BEFORE ANYTHING IS CONCLUDED FROM IT. Go reads
    // the tree through git, so a probe git does not report is not a probe: the two
    // builds would then differ in nothing at all and their agreement would be a
    // couldn't-look wearing an all-clear.
    const strayPath = `folio-go/${basename(stray)}`
    let porcelain
    try { porcelain = execFileSync('git', ['status', '--porcelain'], { cwd: join(root, '..'), encoding: 'utf8', timeout: 30000 }) } catch (error) { fail(`the tree-state probe could not be shown to git: \`git status --porcelain\` in the repository root failed (${error.message}), so whether the second build saw a different tree is unmeasured`) }
    if (!porcelain.includes(strayPath)) fail(`the tree-state probe ${strayPath} is invisible to git, so both builds below see the same tree and their agreement would assert nothing: \`git status --porcelain\` in the repository root does not list it — it must never be added to .gitignore`)
    buildEngineWasm(second, { stdio: 'pipe' })
    const before = sha256(readFileSync(first))
    const after = sha256(readFileSync(second))
    if (before !== after) fail(`the engine wasm is a function of the working tree rather than of the source: two builds of this commit differing only by one stray untracked file digest ${before} and ${after}`)
  } finally {
    rmSync(stray, { force: true })
    rmSync(first, { force: true })
    rmSync(second, { force: true })
  }
}

// DW-107. Story 8.4g red-proved its VCS-stamp guard BY HAND and recorded the
// result in prose, so the evidence could never fail again. This is that proof,
// executed: drop `-buildvcs=false` from the one argv that declares it, build, and
// require the detector to report every setting Go then stamps in.
//
// It deliberately does NOT go through `redProof`: that harness mutates dist/ and
// re-runs verifyOfflineRelease, and the guard under proof lives in build-wasm.mjs.
// The shape is kept — mutate, observe a failure, hold it to the guard's OWN
// message, restore — and the probe builds to a temp file, so src/generated/ is
// never touched and there is nothing to restore by hand.
function proveVCSStampGuardDiscriminates() {
  if (!existsSync(join(root, '..', '.git'))) fail('red proof vcs-stamp-guard could not look: Go stamps VCS build info only inside a version-controlled tree, and this checkout has no .git — an all-clear here would be indistinguishable from a couldn\'t-look')
  const probe = join(tmpdir(), `folio8-engine-vcs-probe-${process.pid}.wasm`)
  try {
    buildEngineWasm(probe, { flags: [], stdio: 'pipe' })
    let message
    try { assertNoVCSStamp(readFileSync(probe), 'engine wasm built without -buildvcs=false') } catch (error) { message = error.message }
    if (!message) fail('red proof vcs-stamp-guard escaped verification: the engine wasm built WITHOUT -buildvcs=false carries no VCS stamp, so the flag build-wasm.mjs depends on is buying nothing')
    for (const setting of ['vcs.revision=', 'vcs.time=', 'vcs.modified=', 'vcs=']) if (!message.includes(setting)) fail(`red proof vcs-stamp-guard failed for the wrong reason: expected the detector to report ${setting}, got: ${message}`)
  } finally { rmSync(probe, { force: true }) }
}

function rewriteRelease(outputDir, release) {
  release.pageId = pageIdentity(release.assets, release.workerRevision)
  release.id = releaseIdentity(release.assets, release.workerRevision)
  writeFileSync(join(outputDir, 'offline-release-manifest.json'), `${JSON.stringify(release, null, 2)}\n`)
  writeFileSync(join(outputDir, 'sw.js'), serviceWorkerSource(release))
}

/**
 * The S1 row a red proof intends to mutate, resolved BY ID.
 *
 * `rows[0]` and `rows[4]` were the two positional reads in this file's
 * falsifiers, and neither said which row it meant: `rows[0]` means "the engine
 * wasm row" and said so nowhere. Story 11.1 inserted seven rows between them.
 * This throws rather than returning undefined, because the failure it is
 * guarding against is a proof that stops proving anything — a mutation applied
 * to `undefined` throws a TypeError the harness would report as the guard
 * having gone red, which is a red proof passing for the wrong reason.
 */
function s1RowById(release, id, proof) {
  const row = release.s1?.rows?.find((candidate) => candidate.id === id)
  if (!row) fail(`red proof ${proof} could not find the S1 row it exists to mutate: no row carries the id '${id}'. A falsifier that cannot locate its subject proves nothing, and must never be allowed to look like a pass.`)
  return row
}

function redProof(name, mutate, expected) {
  let restore
  try {
    restore = mutate(dist)
    let message
    try { verifyOfflineRelease(dist) } catch (error) { message = error.message }
    if (!message) fail(`red proof ${name} escaped verification`)
    // A mutation can trip several guards at once. Where a proof names the guard
    // it is proving, hold it to that guard rather than to any failure at all.
    if (expected && !message.includes(expected)) fail(`red proof ${name} failed for the wrong reason: ${message}`)
  } finally { restore?.() }
}

export function runRedProofs(baseline = verifyOfflineRelease()) {
  const readRelease = (outputDir) => JSON.parse(readFileSync(join(outputDir, 'offline-release-manifest.json'), 'utf8'))
  for (const extension of ['.wasm', '.ttf']) redProof(`stale-${extension.slice(1)}-byte`, (outputDir) => { const witness = baseline.assets.find((candidate) => candidate.immutable && candidate.url.endsWith(extension)); if (!witness) fail(`red proof has no ${extension} witness`); const file = join(outputDir, witness.url.slice(1)); const original = readFileSync(file); writeFileSync(file, Buffer.concat([original, Buffer.from([0])])); return () => writeFileSync(file, original) })
  for (const [name, extension] of [['missing-wasm-row', '.wasm'], ['missing-font-row', '.ttf']]) redProof(name, (outputDir) => { const manifest = join(outputDir, 'offline-release-manifest.json'); const worker = join(outputDir, 'sw.js'); const oldManifest = readFileSync(manifest); const oldWorker = readFileSync(worker); const release = readRelease(outputDir); release.assets = release.assets.filter((asset) => !asset.url.endsWith(extension)); rewriteRelease(outputDir, release); return () => { writeFileSync(manifest, oldManifest); writeFileSync(worker, oldWorker) } })
  for (const [name, matcher] of [['missing-pdfjs-worker', (url) => /\/pdf\.worker-[A-Za-z0-9_-]+\.mjs$/.test(url)], ['missing-pdfjs-cmap', (url) => url.endsWith('.bcmap')], ['missing-pdfjs-standard-font', (url) => /\/pdfjs-standard-fonts-[a-f0-9]{20}\/LiberationSans-Regular\.ttf$/.test(url)]]) redProof(name, (outputDir) => { const manifest = join(outputDir, 'offline-release-manifest.json'); const worker = join(outputDir, 'sw.js'); const oldManifest = readFileSync(manifest); const oldWorker = readFileSync(worker); const release = readRelease(outputDir); release.assets = release.assets.filter((asset) => !matcher(asset.url)); rewriteRelease(outputDir, release); return () => { writeFileSync(manifest, oldManifest); writeFileSync(worker, oldWorker) } })
  redProof('unmanifested-runtime-output', (outputDir) => { const injected = join(outputDir, 'assets', 'injected-12345678.js'); writeFileSync(injected, 'export {}'); return () => unlinkSync(injected) })
  redProof('stale-release-identity', (outputDir) => { const manifest = join(outputDir, 'offline-release-manifest.json'); const original = readFileSync(manifest); const release = JSON.parse(original); release.id = '0'.repeat(64); writeFileSync(manifest, JSON.stringify(release)); return () => writeFileSync(manifest, original) })
  redProof('worker-manifest-drift', (outputDir) => { const worker = join(outputDir, 'sw.js'); const original = readFileSync(worker, 'utf8'); const drifted = original.replace(`"pageId":"${baseline.pageId}"`, `"pageId":"${'0'.repeat(64)}"`); if (drifted === original) fail('red proof could not locate the worker release record'); writeFileSync(worker, drifted); return () => writeFileSync(worker, original) })
  redProof('dictionary-witness-mismatch', (outputDir) => {
    const release = readRelease(outputDir)
    const manifest = join(outputDir, 'offline-release-manifest.json')
    const worker = join(outputDir, 'sw.js')
    const oldManifest = readFileSync(manifest)
    const oldWorker = readFileSync(worker)
    release.thaiDictionary.sha256 = '0'.repeat(64)
    rewriteRelease(outputDir, release)
    return () => { writeFileSync(manifest, oldManifest); writeFileSync(worker, oldWorker) }
  })
  redProof('s1-total-mismatch', (outputDir) => { const manifest = join(outputDir, 'offline-release-manifest.json'); const original = readFileSync(manifest); const release = JSON.parse(original); release.s1.cachedBytes++; writeFileSync(manifest, JSON.stringify(release)); return () => writeFileSync(manifest, original) })
  // A FALSIFIER MUST TARGET ITS SUBJECT BY IDENTITY, NEVER BY POSITION
  // (D-11.1.16). Both of these mutations used to address a row by index —
  // `rows[4]` for the dictionary, `rows[0]` for the engine — and Story 11.1
  // inserted seven rows between them. An index-keyed red proof that lands on
  // the wrong row after an insertion either fails to go red at all, or goes
  // red FOR THE WRONG REASON, and a red proof passing for the wrong reason is
  // the defect class wearing the costume of the thing meant to catch it. So
  // `s1RowById` resolves by id and FAILS LOUDLY when the id is gone: a
  // falsifier that cannot find its subject must never read as a proof.
  redProof('s1-delivery-fiction', (outputDir) => { const manifest = join(outputDir, 'offline-release-manifest.json'); const original = readFileSync(manifest); const release = JSON.parse(original); s1RowById(release, 'thai-dictionary', 's1-delivery-fiction').delivery = 'cached-asset'; writeFileSync(manifest, JSON.stringify(release)); return () => writeFileSync(manifest, original) })
  // THE ENVELOPE, BOTH ENDS, PROVED BY MANUFACTURING A RELEASE OUTSIDE IT.
  // These follow the `missing-*-row` shape rather than the `s1-*` one: the bound
  // is a property of the asset POPULATION, so the only faithful mutation is a
  // release that really carries the wrong number of assets. `rewriteRelease`
  // recomputes `id` and `pageId` so the identity checks cannot trip first, and
  // `expected` holds each proof to the bound guard's own message rather than to
  // any failure at all.
  redProof('asset-count-over-bound', (outputDir) => {
    const manifest = join(outputDir, 'offline-release-manifest.json')
    const worker = join(outputDir, 'sw.js')
    const oldManifest = readFileSync(manifest)
    const oldWorker = readFileSync(worker)
    const release = readRelease(outputDir)
    const { maximumCacheAssets } = declaredCacheAssetBounds()
    while (release.assets.length <= maximumCacheAssets) release.assets.push({ url: `/assets/over-bound-${release.assets.length}-0123456789ab.js`, sha256: '0'.repeat(64), immutable: true })
    rewriteRelease(outputDir, release)
    return () => { writeFileSync(manifest, oldManifest); writeFileSync(worker, oldWorker) }
  }, 'over the declared maximum of')
  redProof('asset-count-under-bound', (outputDir) => {
    const manifest = join(outputDir, 'offline-release-manifest.json')
    const worker = join(outputDir, 'sw.js')
    const oldManifest = readFileSync(manifest)
    const oldWorker = readFileSync(worker)
    const release = readRelease(outputDir)
    const { minimumCacheAssets } = declaredCacheAssetBounds()
    release.assets = release.assets.slice(0, minimumCacheAssets - 1)
    rewriteRelease(outputDir, release)
    return () => { writeFileSync(manifest, oldManifest); writeFileSync(worker, oldWorker) }
  }, 'under the declared minimum of')
  redProof('s1-cloud-label', (outputDir) => { const manifest = join(outputDir, 'offline-release-manifest.json'); const original = readFileSync(manifest); const release = JSON.parse(original); s1RowById(release, 'engine', 's1-cloud-label').label = 'Cloud download'; writeFileSync(manifest, JSON.stringify(release)); return () => writeFileSync(manifest, original) })
  redProof('s1-progress-denominator', (outputDir) => { const manifest = join(outputDir, 'offline-release-manifest.json'); const original = readFileSync(manifest); const release = JSON.parse(original); release.s1.cacheAssets.pop(); writeFileSync(manifest, JSON.stringify(release)); return () => writeFileSync(manifest, original) })
  redProof('s1-bootstrap-drift', (outputDir) => { const index = join(outputDir, 'index.html'); const original = readFileSync(index); writeFileSync(index, original.toString().replace('"releaseId":"', '"releaseId":"0')); return () => writeFileSync(index, original) })
  redProof('dev-bypass-shipped', (outputDir) => {
    const witness = baseline.assets.find((candidate) => candidate.url.startsWith('/assets/') && candidate.url.endsWith('.js'))
    if (!witness) fail('red proof has no script witness')
    const file = join(outputDir, witness.url.slice(1))
    const original = readFileSync(file)
    // Regenerating after the mutation restores every digest, identity, and S1
    // figure, so the bypass guard is the only one left that can trip.
    writeFileSync(file, Buffer.concat([original, Buffer.from(`\n// ${DEV_BYPASS_MARKERS[0]}\n`)]))
    generateOfflineRelease(outputDir)
    return () => { writeFileSync(file, original); generateOfflineRelease(outputDir) }
  }, 'development offline bypass shipped in')
  // THE BROTLI RECORD, PROVED BY DRIFTING IT (AC5, Story 8.5).
  // `rewriteRelease` writes BOTH the manifest and the worker and recomputes the
  // identities, so neither the sw/manifest comparison nor the identity checks
  // can trip first — and `expected` holds the proof to this guard's own
  // message rather than to any failure at all. Deleting the assertion makes
  // this report `escaped verification`.
  redProof('brotli-record-drift', (outputDir) => {
    const manifest = join(outputDir, 'offline-release-manifest.json')
    const worker = join(outputDir, 'sw.js')
    const oldManifest = readFileSync(manifest)
    const oldWorker = readFileSync(worker)
    const release = readRelease(outputDir)
    const witness = release.assets.find((asset) => asset.immutable)
    if (!witness) fail('red proof brotli-record-drift has no immutable witness')
    witness.brotliBytes += 1
    release.brotli.totalBytes += 1
    if (isCatalogueAssetUrl(witness.url)) release.brotli.catalogue.totalBytes += 1
    rewriteRelease(outputDir, release)
    return () => { writeFileSync(manifest, oldManifest); writeFileSync(worker, oldWorker) }
  }, 'brotli-record-drift')
  // AND THE THIRD BRANCH, WHICH THE PROOF ABOVE CANNOT REACH.
  // `brotli-record-drift` mutates the first IMMUTABLE asset, so it exercises the
  // two assertions inside the immutable arm and never the MUTABLE one — delete
  // that arm and every red proof stayed green. /index.html is the only asset
  // that reaches it: it carries no Brotli sidecar, so a recorded Brotli weight
  // for it is a number describing a file nothing compressed, and a number like
  // that sums into the total somebody quotes. `expected` holds this to the
  // branch's own clause rather than to the shared `brotli-record-drift` prefix,
  // so it cannot pass by tripping the immutable arm instead.
  redProof('brotli-record-on-mutable-entry', (outputDir) => {
    const manifest = join(outputDir, 'offline-release-manifest.json')
    const worker = join(outputDir, 'sw.js')
    const oldManifest = readFileSync(manifest)
    const oldWorker = readFileSync(worker)
    const release = readRelease(outputDir)
    const navigation = release.assets.find((asset) => asset.url === '/index.html')
    if (!navigation) fail('red proof brotli-record-on-mutable-entry has no navigation entry')
    if (navigation.immutable) fail('red proof brotli-record-on-mutable-entry: /index.html is marked immutable, so this proof would exercise the wrong branch')
    navigation.brotliBytes = 1
    rewriteRelease(outputDir, release)
    return () => { writeFileSync(manifest, oldManifest); writeFileSync(worker, oldWorker) }
  }, 'is not immutable, carries no Brotli sidecar')
  proveVCSStampGuardDiscriminates()
  redProof('worker-progress-before-marker', (outputDir) => { const worker = join(outputDir, 'sw.js'); const original = readFileSync(worker, 'utf8'); const moved = original.replace("    await cache.put(MARKER, new Response(RELEASE.id, { headers: { 'content-type': 'text/plain' } }))\n    await progress('verified', activeAsset)", "    await progress('verified', activeAsset)\n    await cache.put(MARKER, new Response(RELEASE.id, { headers: { 'content-type': 'text/plain' } }))"); if (moved === original) fail('red proof could not find final marker ordering'); writeFileSync(worker, moved); return () => writeFileSync(worker, original) })
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const redOnly = process.argv.includes('--red-only')
  const wasmWitness = process.argv.includes('--wasm-witness')
  // `reportApproach` ONLY HERE, and only on the branch that verifies the REAL
  // dist. `--red-only` reads the manifest straight off disk and never calls
  // `verifyOfflineRelease` on it, so that run legitimately emits no approach
  // warning rather than emitting one about the first mutated fixture.
  const baseline = redOnly ? JSON.parse(readFileSync(join(dist, 'offline-release-manifest.json'), 'utf8')) : verifyOfflineRelease(dist, { wasmWitness, reportApproach: true })
  if (process.argv.includes('--red-proof') || redOnly) runRedProofs(baseline)
}
