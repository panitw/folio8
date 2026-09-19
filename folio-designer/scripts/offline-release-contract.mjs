import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

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
