import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { ASSET_TIER_RULES, ASSET_TIERS, CORE_CATALOGUE_FACE_IDS, classifyAssetTier, isCatalogueAssetUrl, declaredCacheAssetBounds, declaredCacheAssetWarning, declaredCoreCacheAssetBounds, declaredCoreCacheByteCeiling, normalizePublicPath, pageIdentity, releaseIdentity } from './offline-release-contract.mjs'

describe('offline release contract', () => {
  it('normalizes the single Windows separator emitted by path.relative', () => {
    expect(normalizePublicPath('assets\\worker-abcdef12.js')).toBe('/assets/worker-abcdef12.js')
  })

  it('makes worker-only changes produce a distinct page and cache identity', () => {
    const assets = [{ url: '/index.html', sha256: 'a'.repeat(64) }, { url: '/assets/app-abcdef12.js', sha256: 'b'.repeat(64) }]
    expect(releaseIdentity(assets, 'c'.repeat(64))).not.toBe(releaseIdentity(assets, 'd'.repeat(64)))
    expect(pageIdentity(assets, 'c'.repeat(64))).not.toBe(pageIdentity(assets, 'd'.repeat(64)))
  })

  // THE TIER IS RECORDED, NOT HASHED (spec-deferred-offline-cache, story 1).
  // `canonicalAssetRows` builds each row from `url` and `sha256` alone, so
  // stamping a tier into the manifest cannot move `release.id` or `pageId` —
  // which is what lets story 1 ship without re-issuing the release to every
  // browser holding it. Asserted rather than assumed, because the day someone
  // widens that row is the day every cached release silently invalidates.
  it('leaves the release and page identity untouched by the tier an asset records', () => {
    const untiered = [{ url: '/index.html', sha256: 'a'.repeat(64) }, { url: '/assets/app-abcdef12.js', sha256: 'b'.repeat(64) }]
    const tiered = untiered.map((asset, index) => ({ ...asset, tier: index === 0 ? 'core' : 'deferred' }))
    const flipped = untiered.map((asset) => ({ ...asset, tier: 'core' }))
    expect(releaseIdentity(tiered, 'c'.repeat(64))).toBe(releaseIdentity(untiered, 'c'.repeat(64)))
    expect(releaseIdentity(flipped, 'c'.repeat(64))).toBe(releaseIdentity(tiered, 'c'.repeat(64)))
    expect(pageIdentity(tiered, 'c'.repeat(64))).toBe(pageIdentity(untiered, 'c'.repeat(64)))
  })
})

// An INDEPENDENT re-read of the real file: line splitting and prefix matching
// rather than the helper's own regex, so this asserts the declared numbers
// without re-typing them. A literal here would be a second authority — the
// exact drift the helper exists to make impossible.
const reReadDeclared = (name) => {
  const line = readFileSync(join(import.meta.dirname, '..', 'src', 'release-payload.ts'), 'utf8').split('\n').filter((candidate) => candidate.startsWith(`const ${name} = `))
  expect(line).toHaveLength(1)
  return Number(line[0].slice(`const ${name} = `.length))
}

describe('declared cache-asset bounds', () => {
  it('reads the two bounds src/release-payload.ts actually declares', () => {
    expect(declaredCacheAssetBounds()).toEqual({ minimumCacheAssets: reReadDeclared('minimumCacheAssets'), maximumCacheAssets: reReadDeclared('maximumCacheAssets') })
  })

  // One mutation at a time, and each held to ITS OWN message: a reader that
  // failed for a shared reason would prove only that it failed, not that it can
  // tell a rename from a duplicate.
  it('throws when the constant is absent, naming it and reporting none found', () => {
    expect(() => declaredCacheAssetBounds('const minimumCacheAssets = 10\nconst cacheCeiling = 64\n')).toThrow(/`maximumCacheAssets` as a single live constant: found 0 .*reads as none/)
  })

  it('throws when the only occurrence is commented out', () => {
    expect(() => declaredCacheAssetBounds('const minimumCacheAssets = 10\n// const maximumCacheAssets = 64\n')).toThrow(/`maximumCacheAssets` as a single live constant: found 0 /)
  })

  it('throws when the only occurrence is indented rather than line-anchored', () => {
    expect(() => declaredCacheAssetBounds('const minimumCacheAssets = 10\n  const maximumCacheAssets = 64\n')).toThrow(/`maximumCacheAssets` as a single live constant: found 0 /)
  })

  it('throws when the constant is declared twice, reporting both', () => {
    expect(() => declaredCacheAssetBounds('const minimumCacheAssets = 10\nconst maximumCacheAssets = 64\nconst maximumCacheAssets = 4096\n')).toThrow(/`maximumCacheAssets` as a single live constant: found 2 .*authority ambiguous/)
  })

  it('reports the minimum by its own name rather than by the maximum', () => {
    expect(() => declaredCacheAssetBounds('const maximumCacheAssets = 64\n')).toThrow(/`minimumCacheAssets` as a single live constant: found 0 /)
  })

  // Every failure case above injects a fixture string. A message that named
  // src/release-payload.ts anyway would be naming a file it never opened, and
  // would send a reader to edit the one source that is not at fault.
  it('names the source it actually read rather than the file it did not', () => {
    expect(() => declaredCacheAssetBounds('const minimumCacheAssets = 10\n')).toThrow(/^the injected release-payload source does not declare `maximumCacheAssets`/)
    expect(() => declaredCacheAssetBounds('const minimumCacheAssets = 10\n')).not.toThrow(/release-payload\.ts/)
  })

  it('carries a caller-supplied source label into the message', () => {
    expect(() => declaredCacheAssetBounds('const minimumCacheAssets = 10\n', 'a fixture named by its caller')).toThrow(/^a fixture named by its caller does not declare `maximumCacheAssets`/)
  })

  // BOTH NUMBERS READABLE IS NOT THE SAME AS AN ENVELOPE THAT EXISTS. Left
  // unchecked, an inverted declaration makes verifyOfflineRelease fail every
  // release with a message blaming the release for a fault in the declaration.
  it('throws when the declared envelope is inverted, naming the declaration rather than a release', () => {
    expect(() => declaredCacheAssetBounds('const minimumCacheAssets = 64\nconst maximumCacheAssets = 10\n')).toThrow(/inverted cache-asset envelope: `minimumCacheAssets` is 64 and `maximumCacheAssets` is 10, so no release can satisfy both and the fault is in the declaration/)
  })

  // The coherence check is `>` and not `>=`: an envelope of exactly one legal
  // count is coherent, and refusing it would be a second bound nobody declared.
  it('accepts an envelope whose two ends are equal', () => {
    expect(declaredCacheAssetBounds('const minimumCacheAssets = 23\nconst maximumCacheAssets = 23\n')).toEqual({ minimumCacheAssets: 23, maximumCacheAssets: 23 })
  })
})

// THE APPROACH WARNING'S THRESHOLD (Story 11.1). It is read by the same
// line-anchored reader the two bounds use, so the rename/duplicate/comment-out
// cases are already covered above by construction; what is asserted here is
// what is NEW — that the number comes from the real declaration, and that a
// threshold outside the envelope is named as a fault in the declaration rather
// than left as a warning that can never fire.
describe('declared cache-asset approach warning', () => {
  it('reads the threshold src/release-payload.ts actually declares, and the ceiling it is measured against', () => {
    expect(declaredCacheAssetWarning()).toEqual({ warnCacheAssets: reReadDeclared('warnCacheAssets'), maximumCacheAssets: reReadDeclared('maximumCacheAssets') })
  })

  it('throws when the threshold is absent, naming it rather than a bound', () => {
    expect(() => declaredCacheAssetWarning('const minimumCacheAssets = 10\nconst maximumCacheAssets = 64\n')).toThrow(/`warnCacheAssets` as a single live constant: found 0 /)
  })

  // A warning above the ceiling is unreachable — the bound refuses that release
  // first — and one below the floor fires on every release ever emitted. Both
  // are faults in the declaration, and both are silent unless said so here.
  it('refuses a threshold that could never fire, and one that would always fire', () => {
    expect(() => declaredCacheAssetWarning('const minimumCacheAssets = 10\nconst maximumCacheAssets = 64\nconst warnCacheAssets = 65\n')).toThrow(/`warnCacheAssets` 65 above `maximumCacheAssets` 64/)
    expect(() => declaredCacheAssetWarning('const minimumCacheAssets = 10\nconst maximumCacheAssets = 64\nconst warnCacheAssets = 9\n')).toThrow(/`warnCacheAssets` 9 below `minimumCacheAssets` 10/)
  })

  it('accepts a threshold at either end of the envelope', () => {
    expect(declaredCacheAssetWarning('const minimumCacheAssets = 10\nconst maximumCacheAssets = 64\nconst warnCacheAssets = 64\n')).toEqual({ warnCacheAssets: 64, maximumCacheAssets: 64 })
    expect(declaredCacheAssetWarning('const minimumCacheAssets = 10\nconst maximumCacheAssets = 64\nconst warnCacheAssets = 10\n')).toEqual({ warnCacheAssets: 10, maximumCacheAssets: 64 })
  })
})

// THE TIER RULE (spec-deferred-offline-cache, story 1). One asset of every
// deferred group and one of every core group, because the rule's whole job is
// to tell them apart — a table that exercised only the core side would pass
// over a classifier that returned 'core' for everything, which is precisely the
// silent default the rule exists to refuse.
describe('the release asset tier rule', () => {
  const core = [
    ['the navigation shell', '/index.html'],
    ['the engine wasm', '/assets/folio8-engine.955779a7186c6045ccb4-7FiyAaeA.wasm'],
    ['the application bundle', '/assets/index-CCy1fi3-.js'],
    ['the application stylesheet', '/assets/index-mB7_cTCX.css'],
    ['the engine worker', '/assets/engine.worker-DuqtISa3.js'],
    ['the wasm glue', '/assets/wasm-exec.0c949f4996f9a89698e4-Bt7MFzxt.js'],
    ['the pdf.js chunk', '/assets/pdf-DMRcuM1e.js'],
    ['the pdf.js worker', '/assets/pdf.worker-CLesOks4.mjs'],
    // NAMED, because it is the one asset that reads like a bundled example and
    // is not: `asset-tiers.md` counted it into the examples group by its
    // `thumbnail` substring, which is where that companion's 28/52 counts come
    // from. It is pdf.js, and the spec puts pdf.js in the core tier.
    ['the pdf.js thumbnail view, whose name contains `thumbnail`', '/assets/pdf_thumbnail_view-DOWRhSJg.js'],
    ['a pdf.js CMap', '/assets/pdfjs-cmaps-63e94418f6a31380c20a/Adobe-Japan1-0.bcmap'],
    ['a pdf.js standard font', '/assets/pdfjs-standard-fonts-235124a44157171aab33/LiberationSans-Regular.ttf'],
    ['a document face', '/assets/noto-sans.a4c811314da2ade3b4d2-DDPrwTMs.ttf'],
    ['a Thai document face', '/assets/noto-sans-thai.c94562c15cbff8c9af93-CrLOPtlG.ttf'],
    ['a shell UI face', '/assets/ibm-plex-sans.975dcda37d80f038dcd1-Bl2SjS7V.ttf'],
    // THE ONE CATALOGUE FACE THAT BLOCKS (story 3, owner decision 2026-09-19).
    // `runtime-fonts.css` maps the canvas family `Roboto` to this file while
    // `Roboto Bold`, `Roboto Italic` and `Roboto Bold Italic` map to the shipped
    // core faces above, and the starter and all four bundled examples declare
    // that chain — so while it was deferred the DEFAULT DOCUMENT could not paint
    // its body text without a fetch. It is still a `font-catalogue.json` face
    // (`isCatalogueAssetUrl` still says yes, and the generator's byte subtotal
    // and 31-face count still count it); it is merely no longer a deferred one.
    ['the core catalogue face', '/assets/catalogue-roboto.e688a215e0841b6e4edb-DyKMK8wb.ttf'],
  ]
  const deferred = [
    ['the CJK font', '/assets/noto-sans-cjk.5ef5755b1ac650218098-56t-E-tZ.ttf'],
    ['a catalogue face', '/assets/catalogue-arimo.41b22bc8f0b51f932825-CzYM_rPM.ttf'],
    // The catalogue carries Thai faces of its own, whose stems look exactly
    // like the core Thai document faces above: a rule that read the family name
    // rather than the catalogue prefix would put these in the blocking set.
    ['a catalogue Thai face', '/assets/catalogue-notoserifthai.538df2b3033522cd48bf-C0nStReG.ttf'],
    // THE THREE SIBLINGS WHOSE IDS BEGIN `roboto`, and they are here because the
    // core carve-out above is written as an id followed by a DOT: an exclusion
    // anchored to the bare prefix would have pulled 0.4 MiB of catalogue faces
    // nobody decided about into the blocking set, and the pin would still have
    // read 30 while the first load grew.
    ['a catalogue face whose id extends the core one', '/assets/catalogue-robotocondensed.8c4c429a54b4af66cd33-BcQwErTy.ttf'],
    ['a catalogue mono face whose id extends the core one', '/assets/catalogue-robotomono.af0bff7599c3df383175-DxZaQpLm.ttf'],
    ['a catalogue slab face whose id extends the core one', '/assets/catalogue-robotoslab.fd4d98f8403041d58d67-CnMvBxZr.ttf'],
    ['a bundled example template', '/assets/invoice.f4877f7a403af1a19f4d-DYZ03lTH.folio'],
    ['a bundled example sample', '/assets/invoice.sample.78ed96ba6433f0dca7a7-tYlOXWm3.json'],
    ['a bundled example thumbnail', '/assets/invoice.thumbnail.88abffc36556da508be8-BhF2hbER.png'],
    ['the starter template', '/assets/starter.c66729f2a79c99ceae8b-BLD4F45l.folio'],
    ['a bundled documentation page', '/assets/expression-reference-d40a356cf33f8a39d863.html'],
    // THE CJK FAMILY, NOT ONE CJK FILE. An exclusion anchored to the stem plus a
    // dot stops blocking as soon as the next character is a hyphen, and a
    // 4.72 MiB-class face would rejoin the first load unnoticed.
    ['a CJK face cut', '/assets/noto-sans-cjk-bold.0123456789abcdef0123-AbCdEfGh.ttf'],
    ['a CJK script subset', '/assets/noto-sans-cjk-sc.0123456789abcdef0123-AbCdEfGh.ttf'],
  ]

  // THE STRANGERS. `build-wasm.mjs` fingerprints every emitted file as
  // `<stem>.<20 hex>-<vite hash>.<ext>`, so a rule keyed on that shape plus an
  // extension would sweep these into the deferred tier and stop blocking on
  // them without anyone deciding. They are not examples, not documentation
  // pages, and nothing recognises them — so the build must stop.
  const strangers = [
    ['an unrecognised JSON asset', '/assets/font-index.0123456789abcdef0123-AbCdEfGh.json'],
    ['an unrecognised image asset', '/assets/logo.0123456789abcdef0123-AbCdEfGh.png'],
    ['an unrecognised HTML page', '/assets/help-0123456789abcdef0123.html'],
    ['an unrecognised font format', '/assets/mystery-0123456789abcdef0123-AbCdEfGh.woff2'],
    ['an unrecognised media asset', '/assets/some-video-0123456789abcdef0123-AbCdEfGh.webm'],
    ['a root file outside the runtime tree', '/robots.txt'],
  ]

  it.each(core)('blocks the first load on %s', (_name, url) => {
    expect(classifyAssetTier(url)).toBe('core')
  })

  it.each(deferred)('defers %s', (_name, url) => {
    expect(classifyAssetTier(url)).toBe('deferred')
  })

  // UNCLASSIFIED IS `undefined`, AND THE CALLERS TURN THAT INTO A BUILD FAILURE.
  // A default — either way — would be the silence the rule exists to remove.
  it.each(strangers)('leaves %s unclassified rather than defaulting it', (_name, url) => {
    expect(classifyAssetTier(url)).toBeUndefined()
  })

  // THE TWO QUESTIONS ABOUT ROBOTO HAVE DIFFERENT ANSWERS, AND BOTH ARE ASKED.
  // `classifyAssetTier` says `core` (above); `isCatalogueAssetUrl` must still say
  // yes, because `generate-offline-release.mjs` reads it for
  // `brotli.catalogue.totalBytes` and for the check that the emitted catalogue
  // matches `font-catalogue.json`'s 31 faces. Collapsing the two — carving Roboto
  // out of the predicate instead of out of the deferred RULE — would drop a face
  // from that count and a face's bytes from that subtotal, silently.
  it('keeps the core catalogue face a catalogue face for the generator', () => {
    expect(isCatalogueAssetUrl('/assets/catalogue-roboto.e688a215e0841b6e4edb-DyKMK8wb.ttf')).toBe(true)
    expect(classifyAssetTier('/assets/catalogue-roboto.e688a215e0841b6e4edb-DyKMK8wb.ttf')).toBe('core')
  })

  // THE LIST IS HELD TO THE DOCUMENT THAT MADE IT NECESSARY, AND IT READS THAT
  // DOCUMENT RATHER THAN RESTATING IT.
  //
  // `catalogue-roboto` is core for one reason: `public/templates/starter.folio`
  // — the document every visitor lands on, and the base all four bundled
  // examples were drawn from — paints its body text in Roboto, so while that
  // face was deferred the FIRST SCREEN substituted and corrected itself. Nothing
  // else in the tier rules knows that. Adding a second catalogue family to the
  // starter's chains would leave the classification rows above, the 30/30 pin
  // and every other suite green while reintroducing exactly that regression, so
  // the starter's own chains are the input here.
  //
  // It reads the CHAINS rather than the paint because this is a build-time
  // contract with no engine to ask: a chain entry is the strongest statement
  // available here about what the document may need, and erring towards MORE
  // core faces is the safe direction for the first screen.
  it('classifies every catalogue family the starter declares as core', () => {
    const starter = JSON.parse(readFileSync(join(import.meta.dirname, '..', 'public', 'templates', 'starter.folio'), 'utf8'))
    const catalogue = JSON.parse(readFileSync(join(import.meta.dirname, '..', 'font-catalogue.json'), 'utf8'))
    // KEYED BY THE DERIVED CSS FACE NAME, WHICH IS WHAT A CHAIN ENTRY CARRIES
    // (spec-install-all-face-cuts, story 3). It was keyed by `face.family`,
    // which was the same key while a family had exactly one row; the catalogue
    // now declares up to four cuts per family, so a family-keyed Map is
    // LAST-WINS — `Inter` would resolve to whichever cut sorted last — and it
    // could not see a chain naming a CUT at all. A chain entry's `bold` is
    // `Inter Bold`, the CSS family `scripts/build-wasm.mjs` emits from
    // (family, style), so that is the name this lookup has to answer to. The
    // derivation is spelled the way the generator spells it.
    const cssFamilyOf = (face) => face.style === 'Regular' ? face.family : `${face.family} ${face.style === 'BoldItalic' ? 'Bold Italic' : face.style}`
    const idOfFamily = new Map(catalogue.map((face) => [cssFamilyOf(face), face.id]))
    expect(idOfFamily.size, 'two catalogue rows derive the same CSS face name, so this lookup would answer for only one of them').toBe(catalogue.length)
    // Every face name the starter's chains reach, in the format's own two
    // spellings: a bare string entry, and an object with `face` plus cuts.
    const declared = new Set()
    for (const chain of Object.values(starter.fonts ?? {})) {
      for (const entry of chain) {
        if (typeof entry === 'string') { declared.add(entry); continue }
        for (const name of [entry.face, entry.bold, entry.italic, entry.boldItalic]) if (typeof name === 'string' && name !== '') declared.add(name)
      }
    }
    expect(declared.size, 'read no faces out of starter.folio, so this check would be vacuous').toBeGreaterThan(0)
    const catalogueFamilies = [...declared].filter((family) => idOfFamily.has(family))
    expect(catalogueFamilies, 'the starter must declare at least one catalogue family, or this check is asserting over an empty set').not.toEqual([])
    for (const family of catalogueFamilies) {
      const id = idOfFamily.get(family)
      expect(CORE_CATALOGUE_FACE_IDS, `starter.folio declares the catalogue family ${JSON.stringify(family)} (id ${JSON.stringify(id)}) and the core tier does not name it, so the default document would need a deferred fetch to paint and the first screen every visitor sees would substitute and then correct itself`).toContain(id)
      expect(classifyAssetTier(`/assets/catalogue-${id}.0123456789abcdef0123-AbCdEfGh.ttf`)).toBe('core')
    }
  })

  it('classifies into exactly the two declared tiers', () => {
    expect(ASSET_TIERS).toEqual(['core', 'deferred'])
    for (const [, url] of [...core, ...deferred]) expect(ASSET_TIERS).toContain(classifyAssetTier(url))
  })

  // MUTUAL EXCLUSION, MEASURED RATHER THAN ASSERTED IN A COMMENT. The rules are
  // written so that no URL reaches two of them; `classifyAssetTier` refuses
  // overlap, and that refusal is only trustworthy if the real table is actually
  // free of it. A second rule matching a known asset would return the right
  // tier and still leave the next widening of either rule unreasonable about.
  it.each([...core, ...deferred])('is matched by exactly one rule for %s', (_name, url) => {
    expect(ASSET_TIER_RULES.filter((rule) => rule.matches(url)).map((rule) => rule.name)).toHaveLength(1)
  })

  // AND THE REFUSAL ITSELF, EXECUTED. It is unreachable from the real table by
  // construction, so the rules are injected — the same shape the bound readers
  // use to drive their own failure cases with a fixture source.
  it('refuses a URL that reaches two rules, naming both and their tiers', () => {
    const overlapping = [
      { name: 'first rule', tier: 'core', matches: () => true },
      { name: 'second rule', tier: 'deferred', matches: () => true },
    ]
    expect(() => classifyAssetTier('/assets/ambiguous-0123456789abcdef0123-AbCdEfGh.ttf', overlapping)).toThrow(/reaches 2 tier rules — first rule \(core\), second rule \(deferred\) — and the rules are written to be mutually exclusive/)
  })

  // Overlap is a fault EVEN WHEN THE TWO AGREE: two same-tier rules matching one
  // URL is a rule nobody can reason about, and it is the next widening of either
  // that turns it into a disagreement. A guard that only caught disagreement
  // would pass over exactly the state that produces one.
  it('refuses two rules of the same tier matching one URL', () => {
    const agreeing = [
      { name: 'first rule', tier: 'deferred', matches: () => true },
      { name: 'second rule', tier: 'deferred', matches: () => true },
    ]
    expect(() => classifyAssetTier('/assets/ambiguous-0123456789abcdef0123-AbCdEfGh.ttf', agreeing)).toThrow(/reaches 2 tier rules/)
  })
})

// THE CORE TIER'S ENVELOPE. The reader is the same line-anchored
// `readDeclaredConstant` the two bounds above use, so the rename, duplicate and
// comment-out cases are covered by construction; what is asserted here is what
// is NEW — that the numbers come from the real declaration, and that an
// inverted pair is named as a fault in the DECLARATION rather than in a release.
describe('declared core cache-asset bounds', () => {
  it('reads the two core bounds src/release-payload.ts actually declares', () => {
    expect(declaredCoreCacheAssetBounds()).toEqual({ minimumCoreCacheAssets: reReadDeclared('minimumCoreCacheAssets'), maximumCoreCacheAssets: reReadDeclared('maximumCoreCacheAssets') })
  })

  it('throws when the core ceiling is absent, naming it', () => {
    expect(() => declaredCoreCacheAssetBounds('const minimumCoreCacheAssets = 29\n')).toThrow(/`maximumCoreCacheAssets` as a single live constant: found 0 .*reads as none/)
  })

  it('reports the core floor by its own name rather than by the ceiling', () => {
    expect(() => declaredCoreCacheAssetBounds('const maximumCoreCacheAssets = 29\n')).toThrow(/`minimumCoreCacheAssets` as a single live constant: found 0 /)
  })

  // The release-total reader must not answer for the core pair and vice versa:
  // a source declaring only the release bounds has NO core envelope, and reading
  // one out of it would be a bound nobody declared.
  it('does not read the release-total bounds as the core bounds', () => {
    expect(() => declaredCoreCacheAssetBounds('const minimumCacheAssets = 10\nconst maximumCacheAssets = 90\n')).toThrow(/`minimumCoreCacheAssets` as a single live constant: found 0 /)
  })

  it('throws when the declared core envelope is inverted, naming the declaration rather than a release', () => {
    expect(() => declaredCoreCacheAssetBounds('const minimumCoreCacheAssets = 40\nconst maximumCoreCacheAssets = 29\n')).toThrow(/inverted core cache-asset envelope: `minimumCoreCacheAssets` is 40 and `maximumCoreCacheAssets` is 29, so no release can satisfy both and the fault is in the declaration/)
  })

  // THE PIN IS AN ENVELOPE OF EXACTLY ONE LEGAL COUNT, so equal ends must be
  // legal: the coherence check compares `>` and not `>=` for exactly this.
  it('accepts the exact pin the core tier is declared as', () => {
    expect(declaredCoreCacheAssetBounds('const minimumCoreCacheAssets = 29\nconst maximumCoreCacheAssets = 29\n')).toEqual({ minimumCoreCacheAssets: 29, maximumCoreCacheAssets: 29 })
  })

  it('names the source it actually read rather than the file it did not', () => {
    expect(() => declaredCoreCacheAssetBounds('const minimumCoreCacheAssets = 29\n')).toThrow(/^the injected release-payload source does not declare `maximumCoreCacheAssets`/)
  })
})

// THE CORE TIER'S BYTE CEILING (spec-deferred-offline-cache, story 5). Same
// line-anchored reader again, so the rename/duplicate/comment-out cases are
// covered by construction; what is asserted here is the one NEW coherence rule
// and that the number comes from the real declaration.
describe('declared core cache byte ceiling', () => {
  it('reads the ceiling src/release-payload.ts actually declares', () => {
    expect(declaredCoreCacheByteCeiling()).toEqual({ maximumCoreCacheBytes: reReadDeclared('maximumCoreCacheBytes') })
  })

  it('throws when the ceiling is absent, naming it', () => {
    expect(() => declaredCoreCacheByteCeiling('const minimumCoreCacheAssets = 30\nconst maximumCoreCacheAssets = 30\n')).toThrow(/`maximumCoreCacheBytes` as a single live constant: found 0 .*reads as none/)
  })

  // A CEILING OF FEWER BYTES THAN THERE ARE ASSETS is a declaration no release
  // could ever satisfy, so it is named as a fault in the declaration rather
  // than left to fail every build with a message blaming the release.
  it('refuses a ceiling no release could meet, naming the declaration', () => {
    expect(() => declaredCoreCacheByteCeiling('const minimumCoreCacheAssets = 30\nconst maximumCoreCacheAssets = 30\nconst maximumCoreCacheBytes = 4\n')).toThrow(/declares `maximumCoreCacheBytes` 4, fewer bytes than the 30 assets the core tier is pinned to carry/)
  })

  it('does not read the asset count as the byte ceiling', () => {
    expect(() => declaredCoreCacheByteCeiling('const minimumCoreCacheAssets = 30\nconst maximumCoreCacheAssets = 30\nconst maximumCacheAssets = 90\n')).toThrow(/`maximumCoreCacheBytes` as a single live constant: found 0 /)
  })
})
