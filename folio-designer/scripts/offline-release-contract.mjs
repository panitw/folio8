import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { exampleIds } from './build-examples.mjs'

export const RELEASE_RUNTIME = 'v24.16.0'

export const sha256 = (bytes) => createHash('sha256').update(bytes).digest('hex')

// THE ONE SPELLING OF "THIS ASSET IS A STORY 8.5 CATALOGUE FACE".
// `build-wasm.mjs` fingerprints each catalogue face to `<prefix><id>.ttf`, Vite
// re-hashes that into `/assets/<prefix><id>.<20 hex>-<vite hash>.ttf`, and
// `generate-offline-release.mjs` recognises the catalogue's share of the Brotli
// weight by this prefix (AC5). It lives here, in the module both sides already
// import, because a second copy of it is a copy that drifts — the same reason
// the cache-asset bound below is DERIVED rather than re-typed.
export const CATALOGUE_ASSET_PREFIX = 'catalogue-'
export const isCatalogueAssetUrl = (url) => new RegExp(String.raw`^/assets/${CATALOGUE_ASSET_PREFIX}[a-z0-9]+\.[a-f0-9]{20}-[A-Za-z0-9_-]+\.ttf$`).test(url)

// THE ONE SPELLING OF "WHEN IS THIS ASSET FETCHED" (spec-deferred-offline-cache,
// story 1). Every emitted asset belongs to exactly one tier: `core` is the set a
// first-time visitor must have before the designer can be used at all, and
// `deferred` is everything an author pays for only when they reach for it.
//
// IT LIVES HERE, BESIDE `isCatalogueAssetUrl`, FOR THE SAME REASON THAT DOES:
// `generate-offline-release.mjs` stamps the tier into the manifest and
// `verify-offline-release.mjs` refuses a release whose tiers have drifted, and a
// second copy of the rule is a copy that drifts. Story 1 changes NOTHING about
// loading — the worker still precaches and gates on the whole release — so this
// is a declaration and a guard, not yet a behaviour.
export const ASSET_TIERS = ['core', 'deferred']

// THE RULES MATCH IDENTITIES, NOT FILENAME SHAPES, and they are mutually
// exclusive: `classifyAssetTier` refuses a URL that reaches two of them at all.
// Shape alone is not enough in either direction. `build-wasm.mjs` fingerprints
// EVERY emitted file as `<stem>.<20 hex>-<vite hash>.<ext>`, so a rule keyed on
// that shape plus an extension would sweep an unrelated `.json`, `.png` or
// `.html` into the deferred tier and stop blocking on it without anyone
// deciding; and a lookahead that stopped one exact stem would let a sibling
// (`noto-sans-cjk-sc`) into the blocking set the same way. The drift guard in
// `verify-offline-release.mjs` cannot backstop either mistake, because it calls
// this same classifier — so an asset no rule RECOGNISES must fall through to
// `undefined` and stop the build.
const CJK_FONT_STEM = 'noto-sans-cjk'
const contentAddressed = String.raw`[a-f0-9]{20}-[A-Za-z0-9_-]+`
// The stem and every face cut of it: `noto-sans-cjk.`, `noto-sans-cjk-bold.`,
// `noto-sans-cjk-sc.`. A boundary of `.` OR `-` is what makes the exclusion in
// the core font rule below cover the whole family rather than one file.
const cjkFamily = String.raw`${CJK_FONT_STEM}(?:-[a-z0-9-]+)?`
const CJK_FONT_ASSET = new RegExp(String.raw`^/assets/${cjkFamily}\.${contentAddressed}\.ttf$`)
// THE BUNDLED TEMPLATES BY NAME. `exampleIds` is the same list
// `build-examples.mjs` builds from and `verify-offline-release.mjs` already
// checks the release against, plus the starter `build-wasm.mjs` emits. Naming
// them is what keeps this rule off `/assets/font-index.<hash>.json`,
// `/assets/logo.<hash>.png` and `/assets/pdf_thumbnail_view-<hash>.js` — the
// last of which is a pdf.js preview chunk whose filename contains `thumbnail`
// and which is core.
const BUNDLED_TEMPLATE_STEMS = ['starter', ...exampleIds]
const BUNDLED_EXAMPLE_ASSET = new RegExp(String.raw`^/assets/(?:${BUNDLED_TEMPLATE_STEMS.join('|')})\.(?:${contentAddressed}\.folio|sample\.${contentAddressed}\.json|thumbnail\.${contentAddressed}\.png)$`)
// THE PRECACHED DOCUMENTATION PAGES BY NAME, and this is the one authority for
// that list: `verify-offline-release.mjs` imports it rather than re-typing it,
// for the same reason the cache-asset bound is derived rather than re-typed.
export const DOCUMENTATION_STEMS = ['rendering-library', 'folio-js', 'folio-dotnet', 'folio-format', 'expression-reference', 'performance']
const BUNDLED_DOCUMENTATION_ASSET = new RegExp(String.raw`^/assets/(?:${DOCUMENTATION_STEMS.join('|')})-[a-f0-9]{20}\.html$`)
const ENGINE_WASM_ASSET = new RegExp(String.raw`^/assets/folio8-engine\.${contentAddressed}\.wasm$`)
// index-*.js, index-*.css, pdf-*.js, pdf.worker-*.mjs, pdf_thumbnail_view-*.js,
// engine.worker-*.js and wasm-exec.<digest>-*.js: everything Vite emits as a
// fingerprinted script or stylesheet.
const APP_BUNDLE_ASSET = /^\/assets\/[A-Za-z0-9_.-]+-[A-Za-z0-9_-]{8,}\.(?:js|mjs|css)$/
// The designer's own UI faces and the document faces an ordinary Latin or Thai
// document declares — every fingerprinted `.ttf` that is neither a catalogue
// face nor part of the CJK family.
const SHELL_OR_DOCUMENT_FONT_ASSET = new RegExp(String.raw`^/assets/(?!${CATALOGUE_ASSET_PREFIX}|${CJK_FONT_STEM}[.-])[a-z0-9-]+\.${contentAddressed}\.ttf$`)
// pdf.js ships its CMaps and standard fonts as whole directories fingerprinted
// once, so the files inside them are not individually content-addressed.
const PDFJS_COLLECTION_ASSET = /^\/assets\/pdfjs-(?:cmaps|standard-fonts)-[a-f0-9]{20}\/[A-Za-z0-9_-]+\.(?:bcmap|ttf)$/
// ONE CATALOGUE FACE IS CORE, BY ID, AND THE TIER IS WRITTEN HERE RATHER THAN
// DERIVED FROM ANYTHING (spec-deferred-offline-cache story 3, owner decision
// 2026-09-19).
//
// `src/generated/runtime-fonts.css` maps the canvas family `Roboto` to
// `catalogue-roboto` while `Roboto Bold`, `Roboto Italic` and `Roboto Bold
// Italic` map to the shipped core files, so story 1 left ONE FAMILY STRADDLING
// THE TWO TIERS — and the starter and all four bundled examples declare that
// chain, which made the DEFAULT DOCUMENT need a deferred asset to paint its
// body text. 0.152 MiB moves the pin 29 → 30 and the blocking load 10.67 →
// 10.82 MiB, which is the price of the first screen being right.
//
// ⚠ THIS IS NOT `isCatalogueAssetUrl` AND MUST NOT BECOME IT. That predicate
// answers "is this one of the 31 faces `font-catalogue.json` declares", which
// `generate-offline-release.mjs` asks for `brotli.catalogue.totalBytes` and for
// the emitted-versus-declared face count; Roboto is still such a face and still
// answers yes. This one answers "which of them blocks the first load", and the
// two questions have different answers for exactly one asset.
// Exported so `offline-release-contract.test.mjs` can hold this list to the
// STARTER'S OWN CHAINS rather than restate it: the whole reason Roboto crossed
// over is that the default document paints in it, and a second catalogue family
// added to `public/templates/starter.folio` would otherwise leave every suite
// and the 30/30 pin green while the first screen every visitor sees regressed
// to substitute-then-correct.
export const CORE_CATALOGUE_FACE_IDS = ['roboto']
// ⚠ EVERY ID IS HELD TO `font-catalogue.json`, AT MODULE LOAD, BECAUSE IT IS
// INTERPOLATED INTO A RegExp. A typo — `robto` — is not a syntax error and not
// a match either: it would compile to a rule that recognises nothing, leave
// every catalogue face deferred, and red only the core-asset PIN in another
// module, with a message naming neither this list nor the typo. Named here
// instead, at the declaration, in the shape of the catalogue guards in
// `build-wasm.mjs`. The ids are also checked to be the shape the regex assumes,
// so a value carrying regex syntax cannot silently widen the rule.
const catalogueIds = new Set(JSON.parse(readFileSync(join(dirname(fileURLToPath(import.meta.url)), '..', 'font-catalogue.json'), 'utf8')).map((face) => face.id))
for (const id of CORE_CATALOGUE_FACE_IDS) {
  if (!/^[a-z0-9]+$/.test(id)) throw new Error(`CORE_CATALOGUE_FACE_IDS carries ${JSON.stringify(id)}, which is not the lower-case alphanumeric shape build-wasm.mjs holds every catalogue id to; it is interpolated into a regular expression, so a value outside that shape could widen the core tier rather than name one face`)
  if (!catalogueIds.has(id)) throw new Error(`CORE_CATALOGUE_FACE_IDS names ${JSON.stringify(id)} and font-catalogue.json declares no such face (it declares ${[...catalogueIds].join(', ')}). The name is interpolated into the core-tier rule, so a typo matches nothing, leaves every catalogue face deferred, and reds only the core cache-asset pin in src/release-payload.ts — a message naming neither this list nor the mistake.`)
}
const CORE_CATALOGUE_FACE_ASSET = new RegExp(String.raw`^/assets/${CATALOGUE_ASSET_PREFIX}(?:${CORE_CATALOGUE_FACE_IDS.join('|')})\.${contentAddressed}\.ttf$`)
// MUTUALLY EXCLUSIVE BY CONSTRUCTION, on the CJK carve-out's precedent:
// `classifyAssetTier` throws on any overlap at all, so the deferred rule is
// written as the catalogue MINUS the core ids rather than as the catalogue with
// a core rule listed after it. The `\.` after the id is what keeps
// `catalogue-robotocondensed`, `catalogue-robotomono` and `catalogue-robotoslab`
// out of the core arm — three siblings whose ids all begin with `roboto`.
const isCoreCatalogueAssetUrl = (url) => CORE_CATALOGUE_FACE_ASSET.test(url)

export const ASSET_TIER_RULES = [
  { name: 'catalogue face', tier: 'deferred', matches: (url) => isCatalogueAssetUrl(url) && !isCoreCatalogueAssetUrl(url) },
  { name: 'core catalogue face', tier: 'core', matches: isCoreCatalogueAssetUrl },
  { name: 'CJK font', tier: 'deferred', matches: (url) => CJK_FONT_ASSET.test(url) },
  { name: 'bundled example', tier: 'deferred', matches: (url) => BUNDLED_EXAMPLE_ASSET.test(url) },
  { name: 'bundled documentation page', tier: 'deferred', matches: (url) => BUNDLED_DOCUMENTATION_ASSET.test(url) },
  { name: 'navigation shell', tier: 'core', matches: (url) => url === '/index.html' },
  { name: 'engine wasm', tier: 'core', matches: (url) => ENGINE_WASM_ASSET.test(url) },
  { name: 'application bundle', tier: 'core', matches: (url) => APP_BUNDLE_ASSET.test(url) },
  { name: 'shell or document font', tier: 'core', matches: (url) => SHELL_OR_DOCUMENT_FONT_ASSET.test(url) },
  { name: 'pdf.js collection', tier: 'core', matches: (url) => PDFJS_COLLECTION_ASSET.test(url) },
]

// `undefined` MEANS UNCLASSIFIED, AND UNCLASSIFIED IS A BUILD FAILURE at the
// caller — never a default. Defaulting an unknown asset to `core` would be safe
// at runtime but silent, and silence in exactly this place is what let the first
// load grow unnoticed; defaulting to `deferred` would be worse, because an asset
// the designer needs would stop blocking without anyone deciding that.
export function classifyAssetTier(url, rules = ASSET_TIER_RULES) {
  const matched = rules.filter((rule) => rule.matches(url))
  // OVERLAP IS A FAULT IN THE RULES, EVEN WHEN THE TWO AGREE. Two rules of the
  // same tier matching one URL is a rule nobody can reason about any more, and
  // the next widening of either one is what turns that into a disagreement. The
  // `rules` parameter is injectable for the same reason the bound readers take
  // a source string: this throw is unreachable from the real table, and a guard
  // no test can execute is a guard nobody has seen work.
  if (matched.length > 1) throw new Error(`asset ${url} reaches ${matched.length} tier rules — ${matched.map((rule) => `${rule.name} (${rule.tier})`).join(', ')} — and the rules are written to be mutually exclusive, so overlap is a fault in the rules rather than in the release`)
  return matched[0]?.tier
}

export function normalizePublicPath(value) {
  return `/${value.replaceAll('\\', '/').replace(/^\/+/, '')}`
}

export function canonicalAssetRows(assets) {
  return [...assets]
    .map((asset) => `${asset.url}:${asset.sha256}`)
    .sort()
}

export function releaseIdentity(assets, workerRevision) {
  // index.html is the release bootstrap: it carries the identity it binds to.
  // Excluding that self-describing mutable shell avoids an impossible hash cycle;
  // the verifier still hashes and exact-set checks the emitted HTML itself.
  return sha256(Buffer.from([...canonicalAssetRows(assets.filter((asset) => asset.url !== '/index.html')), `worker:${workerRevision}`].join('\n')))
}

export function pageIdentity(assets, workerRevision) {
  return releaseIdentity(assets, workerRevision)
}

// THE CACHE-ASSET BOUND HAS ONE AUTHORITY: the `const` lines in
// src/release-payload.ts. The verifier must not re-type the number. `npm run
// build` is `build:wasm && tsc -b && vite build && build:offline &&
// verify:offline` — it never runs Vitest — so a second copy of the constant plus
// a tie test would leave a drifted build GREEN. The TypeScript module cannot be
// imported here either (this module needs node:crypto; tsconfig.app.json includes
// only src/; src/ imports nothing from scripts/), so the number is DERIVED from
// the declaration, on the idiom verify-offline-release.mjs already uses for the
// index.html bootstrap and for sw.js's `const RELEASE = …`.
const releasePayloadSource = join(dirname(fileURLToPath(import.meta.url)), '..', 'src', 'release-payload.ts')

// Line-anchored and multiline ON PURPOSE, and it must match EXACTLY ONCE.
// A false failure here stops a build loudly; a false success silently re-creates
// the very defect the bound exists to prevent, so every ambiguous case fails:
//   - `// const maximumCacheAssets = 64` is a commented-out copy, not a bound;
//     `^const` refuses it rather than counting dead code as live.
//   - two live declarations means first-match-wins would pick one and hide the
//     other, so two is a failure rather than a preference.
//   - zero means the constant was renamed, deleted or reformatted, and a reader
//     that answered "no bound" there would disable the guard in silence.
//
// The message names the source it actually read. It used to hardcode
// `src/release-payload.ts` even when a caller injected a fixture string — which
// is how every failure case is driven — so a failure report could name a file
// that was never opened.
function readDeclaredConstant(source, name, label) {
  const matches = [...source.matchAll(new RegExp(String.raw`^const ${name} = (\d+)$`, 'gm'))]
  if (matches.length !== 1) throw new Error(`${label} does not declare \`${name}\` as a single live constant: found ${matches.length} line-anchored \`const ${name} = <digits>\` declarations, expected exactly 1${matches.length === 0 ? ' (a renamed, deleted, reformatted or commented-out constant reads as none)' : ' (a second live declaration makes the authority ambiguous)'}`)
  return Number(matches[0][1])
}

// THE APPROACH WARNING'S THRESHOLD, READ THROUGH THE SAME LINE-ANCHORED READER
// (Story 11.1). It is a SEPARATE export rather than a third field on
// `declaredCacheAssetBounds` deliberately: that function's return shape is
// asserted by exact equality in `offline-release-contract.test.mjs`, and its
// failure cases are driven with two-constant fixture strings — widening it
// would have made every one of those fixtures throw for a reason that has
// nothing to do with the case under test.
//
// IT IS NOT A BOUND AND NOTHING FAILS ON IT. `maximumCacheAssets` is still the
// only number that refuses a release; this one only decides when the build says
// out loud how much margin is left. That distinction is why the warning names
// the MARGIN rather than the count: DW-162's figure aged 41 -> 20 -> 10 while
// three stories walked past it, precisely because the number nobody printed was
// the number nobody watched.
export function declaredCacheAssetWarning(source, label = source === undefined ? 'src/release-payload.ts' : 'the injected release-payload source') {
  const text = source === undefined ? readFileSync(releasePayloadSource, 'utf8') : source
  const warnCacheAssets = readDeclaredConstant(text, 'warnCacheAssets', label)
  const { minimumCacheAssets, maximumCacheAssets } = declaredCacheAssetBounds(text, label)
  // A THRESHOLD OUTSIDE THE ENVELOPE IS A WARNING THAT CANNOT FIRE, or one that
  // fires on every release ever emitted. Either way the fault is in the
  // declaration and not in a release, so it is named here rather than left to
  // read as an ordinary quiet build.
  if (warnCacheAssets > maximumCacheAssets) throw new Error(`${label} declares \`warnCacheAssets\` ${warnCacheAssets} above \`maximumCacheAssets\` ${maximumCacheAssets}: a release over the warning threshold is already refused by the bound, so this warning could never fire and the fault is in the declaration rather than in any release`)
  if (warnCacheAssets < minimumCacheAssets) throw new Error(`${label} declares \`warnCacheAssets\` ${warnCacheAssets} below \`minimumCacheAssets\` ${minimumCacheAssets}: every emittable release would warn, which is the same as no warning at all`)
  return { warnCacheAssets, maximumCacheAssets }
}

export function declaredCacheAssetBounds(source, label = source === undefined ? 'src/release-payload.ts' : 'the injected release-payload source') {
  const text = source === undefined ? readFileSync(releasePayloadSource, 'utf8') : source
  const minimumCacheAssets = readDeclaredConstant(text, 'minimumCacheAssets', label)
  const maximumCacheAssets = readDeclaredConstant(text, 'maximumCacheAssets', label)
  // BOTH NUMBERS CAN BE READABLE AND THE ENVELOPE STILL IMPOSSIBLE. An edit that
  // left the floor above the ceiling would make every release fail verification
  // with a message blaming the RELEASE for carrying the wrong number of assets,
  // when the fault is in this declaration. Say which it is, here, once.
  if (minimumCacheAssets > maximumCacheAssets) throw new Error(`${label} declares an inverted cache-asset envelope: \`minimumCacheAssets\` is ${minimumCacheAssets} and \`maximumCacheAssets\` is ${maximumCacheAssets}, so no release can satisfy both and the fault is in the declaration rather than in any release`)
  return { minimumCacheAssets, maximumCacheAssets }
}

// THE CORE TIER'S OWN ENVELOPE, READ THROUGH THE SAME LINE-ANCHORED READER, AND
// A SEPARATE EXPORT FOR THE REASON `declaredCacheAssetWarning` IS ONE: the shape
// `declaredCacheAssetBounds` returns is asserted by exact equality in
// `offline-release-contract.test.mjs`, and its failure cases are driven with
// two-constant fixture strings, so widening it would make every one of those
// fixtures throw for a reason that has nothing to do with the case under test.
//
// `minimumCacheAssets`/`maximumCacheAssets` keep bounding the release TOTAL,
// unchanged. These two bound the BLOCKING SET — the assets a first-time visitor
// must have before the designer is usable — which is the number that decides
// what a first load costs.
export function declaredCoreCacheAssetBounds(source, label = source === undefined ? 'src/release-payload.ts' : 'the injected release-payload source') {
  const text = source === undefined ? readFileSync(releasePayloadSource, 'utf8') : source
  const minimumCoreCacheAssets = readDeclaredConstant(text, 'minimumCoreCacheAssets', label)
  const maximumCoreCacheAssets = readDeclaredConstant(text, 'maximumCoreCacheAssets', label)
  // Same fault, same voice as the release-total envelope above: an inverted
  // declaration would fail every release with a message blaming the RELEASE for
  // the wrong number of core assets, when the fault is in the declaration.
  if (minimumCoreCacheAssets > maximumCoreCacheAssets) throw new Error(`${label} declares an inverted core cache-asset envelope: \`minimumCoreCacheAssets\` is ${minimumCoreCacheAssets} and \`maximumCoreCacheAssets\` is ${maximumCoreCacheAssets}, so no release can satisfy both and the fault is in the declaration rather than in any release`)
  return { minimumCoreCacheAssets, maximumCoreCacheAssets }
}

// THE CORE TIER'S BYTE CEILING, READ THROUGH THE SAME LINE-ANCHORED READER
// (spec-deferred-offline-cache, story 5), and a THIRD separate export for the
// reason the two above are separate: `declaredCoreCacheAssetBounds`'s return
// shape is asserted by exact equality, and widening it would break every
// fixture-driven failure case that has nothing to do with bytes.
//
// COUNT AND WEIGHT ARE DIFFERENT QUESTIONS AND HAVE DIFFERENT ANSWERS. The
// count pin is an equality — an asset entering or leaving the blocking set is
// always somebody's decision. This is a one-sided bound, because the weight
// moves with every bundle change and a pin that red every commit would be
// deleted rather than respected.
//
// THE FLOOR IS THE `maximumCoreCacheAssets` COUNT, not a byte figure, and the
// sanity check is deliberately crude: a ceiling of fewer bytes than the core
// tier has ASSETS could never be met by any release, so the fault would be in
// this declaration rather than in any build.
export function declaredCoreCacheByteCeiling(source, label = source === undefined ? 'src/release-payload.ts' : 'the injected release-payload source') {
  const text = source === undefined ? readFileSync(releasePayloadSource, 'utf8') : source
  const maximumCoreCacheBytes = readDeclaredConstant(text, 'maximumCoreCacheBytes', label)
  const { maximumCoreCacheAssets } = declaredCoreCacheAssetBounds(text, label)
  if (maximumCoreCacheBytes < maximumCoreCacheAssets) throw new Error(`${label} declares \`maximumCoreCacheBytes\` ${maximumCoreCacheBytes}, fewer bytes than the ${maximumCoreCacheAssets} assets the core tier is pinned to carry: no release could ever satisfy it and the fault is in the declaration rather than in any release`)
  return { maximumCoreCacheBytes }
}

// CORE-TIER BROTLI WEIGHT, DERIVED IN ONE PLACE so the build that RECORDS it
// and the verifier that CHECKS it cannot compute it two ways. `/index.html` is
// the one mutable asset and carries no Brotli sidecar, so `immutable` is the
// same filter every other Brotli total in this repository applies.
//
// ⚠ THE SET IS SPELLED ONCE, HERE, and the count and the weight are both taken
// from it. Two independent `tier === 'core' && immutable` filters in a file
// whose own comment says the derivation lives in one place is the drift this
// export exists to prevent, written into the export itself.
function coreTierBrotliAssets(assets) {
  return assets.filter((asset) => asset.tier === 'core' && asset.immutable)
}

export function coreTierBrotliBytes(assets) {
  return coreTierBrotliAssets(assets).reduce((total, asset) => total + asset.brotliBytes, 0)
}

export function coreTierBrotliAssetCount(assets) {
  return coreTierBrotliAssets(assets).length
}

// THE APP VERSION, AND THE ONE RULE THAT MAKES AN UPGRADE MANDATORY.
//
// The release `id` is a content hash: it answers "is this the same bytes", which
// is exactly the wrong question for "must this user stop and take the update".
// Every deploy changes the hash, and most deploys are not worth interrupting an
// author mid-document for. So mandatory-ness is carried by a SEPARATE, AUTHORED
// number — the MAJOR of `package.json`'s version — and the rule is the whole
// policy in one line: a greater major is mandatory, anything else is optional.
//
// Authored, not derived, is the point. The owner decides a release is mandatory
// by bumping the major, which is a deliberate act with a diff; nothing about the
// build can promote a release to mandatory on its own.
const semverPattern = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/

export function parseAppVersion(value, label = 'app version') {
  const match = semverPattern.exec(String(value ?? ''))
  if (!match) throw new Error(`${label} is not a plain MAJOR.MINOR.PATCH version: received ${JSON.stringify(value)}`)
  return { major: Number(match[1]), minor: Number(match[2]), patch: Number(match[3]) }
}

export function readAppVersion(root) {
  const declared = JSON.parse(readFileSync(join(root, 'package.json'), 'utf8')).version
  parseAppVersion(declared, 'package.json version')
  return declared
}

// `undefined` for either side means "cannot tell", and cannot-tell is NEVER
// mandatory: an old page that predates versioning, or a worker that declines to
// answer, must degrade to the optional prompt rather than lock an author out of
// their document on missing evidence.
export function upgradeIsMandatory(fromVersion, toVersion) {
  if (typeof fromVersion !== 'string' || typeof toVersion !== 'string') return false
  try { return parseAppVersion(toVersion).major > parseAppVersion(fromVersion).major } catch { return false }
}
