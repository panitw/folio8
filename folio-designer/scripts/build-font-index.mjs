import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { familyIndexHost } from '../src/font-source.ts'

// THE FAMILY INDEX SNAPSHOT (Story 16.1, D-16.3).
//
// THE LIST IS NOT LIVE, AND THIS SCRIPT IS WHY. Google publishes the family
// index at the host `src/font-source.ts` declares as `familyIndexHost` (spelled
// there and, because the source scan reads RAW text, deliberately not here), and
// measured 2026-09-02 and
// re-confirmed 2026-09-03 it returns **no `access-control-allow-origin`**: a
// browser is not allowed to read it. There is no shape in which the designer
// fetches this list at runtime, so it is SNAPSHOTTED HERE, at build time, and it
// ages between releases exactly as the 21-family bundled catalogue did. What is
// genuinely live is the typeface itself — its bytes, its licence text and its
// per-family metadata, all from a host that does send CORS headers. Anyone
// describing the result as a live font browser without that qualification is
// describing something this product does not do, which is why the module emitted
// below carries its own snapshot date and family count and the UI states them.
//
// ─────────────────────────────────────────────────────────────────────────────
// WHAT IS COMMITTED AND WHAT IS GENERATED, STATED RATHER THAN INHERITED.
//
// The precedent does not decide this: `src/generated/font-catalogue.ts` is
// gitignored while `src/generated/pdfjs-assets.ts` is tracked. So:
//
//   `folio-designer/font-index.json` — THE SNAPSHOT ITSELF — IS COMMITTED,
//   beside `font-catalogue.json`, which is the committed source-of-truth for the
//   local face tier. Two reasons, and the first is a shipped gate: `npm run
//   build` MUST NOT REQUIRE NETWORK — an offline release build is verified in
//   CI — so the list a release ships cannot be fetched while building it. The
//   second is that a list fetched at build time would make what a release
//   publishes a function of WHEN somebody built rather than of what somebody
//   reviewed; committed, a change to the family list is a diff in a pull
//   request like every other change.
//
//   `folio-designer/src/generated/font-index.ts` — THE TYPED MODULE — IS
//   GENERATED AND GITIGNORED, exactly as `font-catalogue.ts` is. It is a pure
//   function of the committed JSON, with no network and no second input, so a
//   committed copy would be a second authority that can only ever go stale.
//
// The only networked step is `npm run refresh:font-index`, run deliberately by a
// person, which rewrites the committed JSON. `npm run build`, `npm test` and
// `npm run typecheck` run the EMIT step only.
// ─────────────────────────────────────────────────────────────────────────────

const here = dirname(fileURLToPath(import.meta.url))
const designerRoot = join(here, '..')
export const snapshotPath = join(designerRoot, 'font-index.json')
const generatedPath = join(designerRoot, 'src', 'generated', 'font-index.ts')

// CJK IS OUT BY THE SPEC'S OWN NON-GOAL, and it is excluded HERE rather than
// filtered in the browser so the shipped list never carries rows nothing may
// ever add. A full SC face is 10.6 MB against 646 KB for Latin and 47 KB for
// Thai; CJK stays on the shipped-face path.
const cjkSubsets = ['chinese-simplified', 'chinese-traditional', 'chinese-hongkong', 'japanese', 'korean']

/** The fields the designer renders, and nothing else. The response is 2.7 MB; this is not. */
export function trimFamily(entry) {
  return {
    family: entry.family,
    category: entry.category ?? '',
    // `menu` is a subsetting artefact present on every family and says nothing
    // about coverage, so it is dropped rather than carried 1,946 times.
    subsets: (entry.subsets ?? []).filter((subset) => subset !== 'menu'),
    // AXES IS A PREDICTION, NOT AN INVENTORY, and the module that consumes it
    // says so. Measured: all 558 axes-declaring families still list a `400` key
    // under `fonts`, so that map is an OFFERED-WEIGHTS list rather than a
    // statement about which static files exist upstream. `axes != []` is the
    // only signal available here, it is a good heuristic (verified on Roboto and
    // six others), and the authority stays Go.
    axes: (entry.axes ?? []).map((axis) => axis.tag),
    styles: Object.keys(entry.fonts ?? {}),
    popularity: entry.popularity ?? 0,
  }
}

export function trimSnapshot(published, today) {
  const all = published.familyMetadataList ?? []
  const families = all
    .filter((entry) => !(entry.subsets ?? []).some((subset) => cjkSubsets.includes(subset)))
    .map(trimFamily)
  return {
    snapshotDate: today,
    // THE HOST IS DELIBERATELY NOT WRITTEN INTO THE COMMITTED JSON. `.json` is
    // a scanned extension, and a data file cannot carry the host scanner's
    // declaration marker "in code" — so the endpoint is named where it is
    // declared, in `src/font-source.ts`, and the emit step reads it from there.
    snapshotSource: 'the published family index — see src/font-source.ts for the endpoint, which is declared there',
    publishedFamilies: all.length,
    excludedCjkFamilies: all.length - families.length,
    families,
  }
}

export function readSnapshot() {
  return JSON.parse(readFileSync(snapshotPath, 'utf8'))
}

// The closed script vocabulary the fallback-tail proposal is written against —
// `latin`, `thai`, `cjk`, exactly as `font-catalogue.json` declares it. CJK is
// never produced because CJK families are excluded above; the arm stays so the
// two vocabularies are the same vocabulary.
const scriptsOf = (family) => {
  const scripts = []
  if (family.subsets.includes('latin')) scripts.push('latin')
  if (family.subsets.includes('thai')) scripts.push('thai')
  return scripts
}

/**
 * THE EMIT STEP. Pure, offline, and a function of the committed JSON alone.
 *
 * The row shape ECHOES `CatalogueFace` — `family`, `scripts` — so `App.tsx`
 * reads one kind of thing from two tiers rather than two shapes from two
 * sources. What it deliberately does NOT carry is a licence field: the index
 * publishes none, and inventing one here would be a second licence authority
 * ageing on its own schedule (D-16.R.6). No licence is knowable before a pick.
 */
export function emitFontIndexModule() {
  const snapshot = readSnapshot()
  const rows = snapshot.families.map((family) =>
    `  { family: ${JSON.stringify(family.family)}, category: ${JSON.stringify(family.category)}, scripts: [${scriptsOf(family).map((script) => JSON.stringify(script)).join(', ')}], variable: ${family.axes.length > 0}, popularity: ${family.popularity} },`)
  writeFileSync(generatedPath,
    `// GENERATED by scripts/build-font-index.mjs from font-index.json. Do not edit.\n`
    + `//\n`
    + `// A BUILD-TIME SNAPSHOT, NOT A LIVE LIST. ${familyIndexHost}/metadata/fonts sends no\n`
    + `// access-control-allow-origin header, so a browser cannot read it. This list ages\n`
    + `// between releases; the FACES it names are fetched live at the moment of a pick.\n`
    + `import type { CatalogueScript } from './font-catalogue'\n\n`
    + `export type IndexFamily = Readonly<{ family: string; category: string; scripts: ReadonlyArray<CatalogueScript>; variable: boolean; popularity: number }>\n\n`
    + `/** The day this snapshot was taken. The UI states it, because a stale list is the cost of the list not being fetchable. */\n`
    + `export const familyIndexSnapshotDate = ${JSON.stringify(snapshot.snapshotDate)}\n`
    + `/** How many families the source published on that day, BEFORE this designer excluded any. */\n`
    + `export const familyIndexPublishedFamilies = ${snapshot.publishedFamilies}\n`
    + `/** CJK families, excluded from the snapshot by SPEC-fonts' own non-goal. */\n`
    + `export const familyIndexExcludedCjkFamilies = ${snapshot.excludedCjkFamilies}\n\n`
    + `export const familyIndex: ReadonlyArray<IndexFamily> = [\n${rows.join('\n')}\n]\n`)
  return { families: snapshot.families.length, snapshotDate: snapshot.snapshotDate }
}

/** THE ONLY NETWORKED STEP, and it is never on the build path. */
export async function refreshSnapshot(today = new Date().toLocaleDateString('en-CA')) {
  const url = `https://${familyIndexHost}/metadata/fonts`
  const response = await fetch(url)
  if (!response.ok) throw new Error(`the family index responded ${response.status}; the snapshot was NOT rewritten, because a partial list is worse than a list that is one release old`)
  const snapshot = trimSnapshot(JSON.parse(await response.text()), today)
  if (snapshot.families.length < 1000) throw new Error(`the family index returned only ${snapshot.families.length} usable families, which is not the list this designer ships; the snapshot was NOT rewritten`)
  writeFileSync(snapshotPath, `${JSON.stringify(snapshot, undefined, 1)}\n`)
  return snapshot
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  if (process.argv.includes('--refresh')) {
    const snapshot = await refreshSnapshot()
    console.log(`font index snapshot refreshed: ${snapshot.families.length} families kept of ${snapshot.publishedFamilies} published (${snapshot.excludedCjkFamilies} CJK excluded), dated ${snapshot.snapshotDate}`)
  }
  const emitted = emitFontIndexModule()
  console.log(`font index module emitted: ${emitted.families} families from the snapshot of ${emitted.snapshotDate} (a BUILD-TIME snapshot; the list is not fetched at runtime)`)
}
