import { classifyLicenceToken, type LicenceClassification } from './font-licence.ts'
import { faceCopyright, faceIsVariable } from './font-name-table.ts'
import type { FaceCutPermanence } from './font-store.ts'

// THE FONT SOURCE — THE ONE MODULE IN THIS REPOSITORY THAT NAMES A FONT HOST.
//
// Every host this designer is allowed to reach is spelled once, below, on a line
// carrying the `folio8:font-host-declaration` marker IN CODE. That is not a
// formality: `scripts/forbidden-font-hosts.mjs` fails the build on either host
// appearing anywhere else under its scanned roots, and it computes the exemption
// over comment-blanked source, so the marker cannot be written in a comment.
// Anyone reaching for a second fetch site has to either put it here or break the
// build.
//
// WHY THE REPOSITORY HOST AND NEVER THE STYLESHEET ENDPOINT (D-16.3, measured
// 2026-09-02; the two hosts are named in the array below, and deliberately
// nowhere else in this file — the source scan reads RAW text, so a host spelled
// in prose here would be an undeclared occurrence like any other).
// The `css2` endpoint under a modern browser
// user-agent returns `woff2`, which the engine's `decodeRecognisedFont` refuses
// by design — its accepted media types are exactly `font/ttf` and `font/otf`,
// with `font/woff2` deliberately outside the set — and it returns it SPLIT BY
// `unicode-range` INTO PER-SCRIPT SUBSETS, which would embed partial coverage
// into a document naming the whole family. The full TTF that endpoint serves to
// a legacy user-agent is unreachable from a browser, which cannot set
// `User-Agent`. The `css2` host is forbidden outright by the scan's first half
// and that is what keeps this trap shut.
//
// WHAT IS FETCHED, AND IN WHAT ORDER: `METADATA.pb` first, because it decides
// admission; then, only if the terms are admitted, the licence file and the
// bytes. THE ORDER IS NON-NEGOTIABLE — classify, then embed. A face may not
// reach the document before its licence and copyright are in hand.

/**
 * THE HOSTS. Declared here, in code, with the marker, and nowhere else.
 *
 * `fontsRepositoryHost` serves the bytes, the licence text and the per-family
 * metadata, with `access-control-allow-origin: *`, so a browser may read it.
 *
 * `familyIndexHost` serves the family list and CANNOT be read by a browser at
 * all — it sends no `access-control-allow-origin` (D-16.3, measured). It is
 * named here only because `scripts/build-font-index.mjs` snapshots it at build
 * time; nothing at runtime reaches for it, which is exactly why the word "live"
 * is qualified everywhere it appears in this product.
 */
export const fontHostDeclarations = [
  { host: 'raw.githubusercontent.com', declaration: 'folio8:font-host-declaration', role: 'face bytes, licence text and per-family metadata, read at the moment a family is picked' },
  { host: 'fonts.google.com', declaration: 'folio8:font-host-declaration', role: 'the family list, read ONLY by scripts/build-font-index.mjs at build time; unreadable from a browser' },
] as const

export const fontsRepositoryHost = fontHostDeclarations[0].host
export const familyIndexHost = fontHostDeclarations[1].host

/** The branch of `google/fonts` the snapshot and the fetches both read. */
const fontsRepositoryBase = `https://${fontsRepositoryHost}/google/fonts/main`

/**
 * THE PROBE ORDER (D-16.R.6). The index carries no path field, so the directory
 * is derived and then confirmed; these are the four top-level directories
 * `google/fonts` publishes.
 *
 * THE DIRECTORY IS NEVER EVIDENCE OF THE TERMS. Measured: upstream MOVES
 * families between directories — `apache/roboto` now 404s and Roboto lives in
 * `ofl/` — so reading layout as a licence assertion would let a family that
 * moved silently change the terms a document publishes. `METADATA.pb` always
 * wins; a family resolved at `ofl/x` whose token says `APACHE2` is admitted as
 * `Apache-2.0`, and the divergence is RECORDED rather than refused.
 */
export const probeDirectories = ['ofl', 'apache', 'ufl', 'cc-by-sa'] as const

/**
 * THE LICENCE FILE IS NAMED BY THE DECLARED TERMS, NOT BY THE DIRECTORY, for
 * the same reason the directory is not evidence: a family resolved at `ofl/x`
 * that declares `APACHE2` publishes Apache terms, and carrying `OFL.txt` beside
 * them would make the document state terms its own record contradicts. If the
 * file the declared licence names is not there, the pick is REFUSED — a
 * document may not carry a face without its terms.
 *
 * THE MAP HOLDS EXACTLY THE IDS THE TOKEN TABLE CAN EMIT, AND NO OTHERS. It is
 * keyed on `classifyLicenceToken`'s admitted output, not on D-8.5.3's four
 * identifiers, and those are different sets on purpose: `font-licence.ts`
 * deliberately has no `MIT` row and argues at length that this is ABSENCE, NOT
 * NARROWING — `google/fonts` publishes no MIT token, so nothing here can ever
 * be asked for one. A speculative `MIT` row would be dead on arrival and, worse,
 * would be the mapping a future MIT token silently inherited without anybody
 * reviewing which file upstream actually publishes. If the token table ever
 * gains a row, this map gains one in the same change, and until then a missing
 * row is a stated refusal below rather than a URL ending in `undefined`.
 */
const licenceFileFor: Readonly<Record<string, string>> = {
  'OFL-1.1': 'OFL.txt',
  'Apache-2.0': 'LICENSE.txt',
  'Ubuntu-font-1.0': 'UFL.txt',
}

/**
 * THE SLUG RULE, EXACT (D-16.R.6): lowercase the family name, then delete every
 * character outside `[a-z0-9]`.
 *
 * Verified 8 of 8 on deliberately awkward families — `Press Start 2P` →
 * `pressstart2p`, `Baloo Bhai 2` → `baloobhai2`, `Alegreya SC` → `alegreyasc`,
 * `Source Serif 4` → `sourceserif4`, `DM Sans` → `dmsans`, `Ma Shan Zheng` →
 * `mashanzheng`, `Playpen Sans Thai` → `playpensansthai`, `Noto Sans Thai
 * Looped` → `notosansthailooped`.
 *
 * This is a derivation CLOSED BY VERIFICATION, which is why it is not the guess
 * this module forbids one level down for the Regular filename: the directory it
 * proposes is accepted only if that directory's `METADATA.pb` `name` string-
 * equals the family the author picked.
 */
export function familyDirectorySlug(family: string): string {
  let slug = ''
  for (const character of family.toLowerCase()) if (character >= 'a' && character <= 'z' || character >= '0' && character <= '9') slug += character
  return slug
}

export type FamilyMetadata = Readonly<{
  /** The family name `METADATA.pb` itself declares. The confirmation compares against this. */
  name: string
  /** The upstream licence token — `OFL`, `APACHE2`, `UFL`, … — never an SPDX id. */
  licence: string
  /** Every `fonts { … }` block, in file order. */
  faces: ReadonlyArray<Readonly<{ style: string; weight: number; filename: string }>>
}>

/**
 * A READER FOR `METADATA.pb`'s TEXT PROTO, and only for the four things this
 * story asks it.
 *
 * DEPTH IS TRACKED RATHER THAN IGNORED, because `name:` appears BOTH at the top
 * level (the family) and inside every `fonts { … }` block (the face). A flat
 * scan would read the last face's name as the family's and confirm a directory
 * against the wrong string — the exact failure the confirmation exists to
 * prevent.
 *
 * No protobuf dependency: this reads four fields out of a line-oriented text
 * format, and `package.json`'s three dependencies are a standing decision.
 */
export function parseFamilyMetadata(source: string): FamilyMetadata | undefined {
  let name: string | undefined
  let licence: string | undefined
  const faces: Array<{ style: string; weight: number; filename: string }> = []
  let block: { style: string; weight: number; filename: string } | undefined
  let depth = 0
  for (const rawLine of source.split('\n')) {
    const line = rawLine.trim()
    if (line === '' || line.startsWith('#')) continue
    if (line === '}') {
      // DEPTH HAS A FLOOR, AND THE FLOOR IS WHAT KEEPS A MALFORMED FILE FROM
      // RESOLVING TO THE WRONG STRING. Unfloored, one stray `}` drives the
      // depth negative, the next `{` returns it to 0 without opening a block,
      // and the `name:` inside a `fonts { … }` entry — upstream blocks really
      // do carry one — is then read as the FAMILY name and confirmed against.
      // That is precisely the confusion the name-equality confirmation exists
      // to prevent. Floored, an unbalanced file can only fail to resolve.
      depth = Math.max(0, depth - 1)
      if (depth === 0 && block !== undefined) {
        if (block.filename !== '') faces.push(block)
        block = undefined
      }
      continue
    }
    if (line.endsWith('{')) {
      const opened = line.slice(0, -1).trim()
      if (depth === 0 && opened === 'fonts') block = { style: '', weight: 0, filename: '' }
      depth += 1
      continue
    }
    const colon = line.indexOf(':')
    if (colon === -1) continue
    const key = line.slice(0, colon).trim()
    const raw = line.slice(colon + 1).trim()
    const value = raw.startsWith('"') && raw.endsWith('"') && raw.length >= 2 ? raw.slice(1, -1) : raw
    if (depth === 0) {
      if (key === 'name') name ??= value
      if (key === 'license') licence ??= value
      continue
    }
    if (depth === 1 && block !== undefined) {
      if (key === 'style') block.style = value
      if (key === 'weight') block.weight = Number(value)
      if (key === 'filename') block.filename = value
    }
  }
  if (name === undefined || licence === undefined) return undefined
  return { name, licence, faces }
}

/**
 * THE FOUR CUTS, AND THEY ARE THE FOUR RIBBI SUBFAMILIES — NO FIFTH.
 *
 * `Regular`, `Bold`, `Italic`, `Bold Italic` is the closed set a `.folio` chain
 * entry can name (`shipped-face-cuts.ts` is the same vocabulary for the faces
 * this release ships — read it, it is the reference). Nothing here synthesises
 * a bold or an oblique, nothing instances a variable axis, and no weight
 * outside 400 and 700 is kept: a family's Thin, Light, Medium and Black are
 * real faces upstream and there is nowhere in this format to put them.
 *
 * `style` ON A STORED RECORD IS ONE OF THESE FOUR STRINGS, exactly. That is
 * where `font-store.ts`'s long-standing "non-empty string" requirement starts
 * carrying meaning: a cut is resolved by matching this value, so a record
 * carrying anything else resolves to nothing.
 */
export const faceCuts = ['Regular', 'Bold', 'Italic', 'Bold Italic'] as const
export type FaceCut = typeof faceCuts[number]

/**
 * THE THREE NON-BASE CUTS, FOR A CALLER THAT WANTS THE REGULAR AND NOTHING ELSE.
 *
 * Two callers need exactly one face rather than a family's set — the font
 * browser's specimen, which is SET IN THE REGULAR, and the re-embed refetch,
 * whose document carries the Regular alone. Passing this as `skip` costs one
 * body read instead of up to four, which on a page of twelve browsed rows is
 * the difference between twelve requests and as many as forty-eight.
 */
export const cutsBesideTheRegular: ReadonlyArray<FaceCut> = faceCuts.filter((cut) => cut !== 'Regular')

/**
 * ⚠ THREE SPELLINGS OF ONE CUT SET, RECONCILED HERE AND NOWHERE ELSE.
 *
 * MOVED FROM `App.tsx` BY spec-install-all-face-cuts STORY 5, unchanged in
 * value. The panel was the only reader while the only join was chain→store;
 * story 5 gives the font browser a SECOND reader — it has to read a committed
 * catalogue row's cut to say which cuts a local-tier family has before the pick
 * — and a bridge with two readers in one component's private scope is a bridge
 * the second reader copies. This module already declares `faceCuts`, which is
 * one of the three vocabularies, so the reconciliation lives beside it.
 *
 *   · `bold` / `italic` / `boldItalic` — the `.folio` format's own closed
 *     variant key set, which the chain, the projection and the panel speak.
 *   · `Regular` / `Bold` / `Italic` / `Bold Italic` — RIBBI subfamily names,
 *     what `METADATA.pb` publishes and what a `StoredFace.style` is required to
 *     be EXACTLY (`faceCuts` above).
 *   · `Regular` / `Bold` / `Italic` / `BoldItalic` — what `font-catalogue.json`
 *     spells a COMMITTED row, because `scripts/build-wasm.mjs` holds every row
 *     to the format's key set with its capitals.
 *
 * Neither of the last two is wrong; they are two vocabularies for one cut, and
 * anything joining a stored face to a catalogue row has to say which it is
 * using. `'Bold Italic'` with one space is the member that makes a hand-spelled
 * literal a real risk rather than a tidiness argument: it resolves to nothing,
 * silently.
 *
 * THE VARIANT KEY SET NAMES ONLY THE THREE NON-BASE CUTS. A chain entry IS its
 * Regular, so the format has no key for it — which is why `StyleCut` has three
 * members against `FaceCut`'s four, and why the reverse bridge below has to
 * handle the base cut by its absence from this record rather than by a fourth
 * entry in it.
 *
 * ⚠ THE TYPE IS DERIVED FROM THE LIST, exactly as `FaceCut` is derived from
 * `faceCuts` forty lines above. A hand-written union beside a hand-written
 * array is two authorities on one closed set — the defect this very block
 * exists to refuse — and one of them would eventually gain a member the other
 * does not have.
 */
export const styleCuts = ['bold', 'italic', 'boldItalic'] as const
export type StyleCut = typeof styleCuts[number]

/**
 * A `.folio` variant key, as the store and the fetch layer spell the same cut.
 *
 * MODULE-PRIVATE: `ribbiCutOf` is the reader, and a caller holding the record
 * itself could index it with something the accessor would have refused.
 */
const RIBBI_CUT_NAMES: Readonly<Record<StyleCut, FaceCut>> = { bold: 'Bold', italic: 'Italic', boldItalic: 'Bold Italic' }
export const ribbiCutOf = (cut: StyleCut): FaceCut => RIBBI_CUT_NAMES[cut]

/** A `.folio` variant key, as `font-catalogue.json` spells the same cut. */
export const CATALOGUE_CUT_STYLES: Readonly<Record<StyleCut, string>> = { bold: 'Bold', italic: 'Italic', boldItalic: 'BoldItalic' }

/**
 * THE REVERSE BRIDGE — A COMMITTED ROW'S `style` READ BACK AS A RIBBI CUT —
 * AND IT IS **DERIVED FROM THE TWO RECORDS ABOVE**, NEVER RESTATED.
 *
 * A fourth literal spelling of these four strings is the defect this epic has
 * already paid for twice, so this map is BUILT rather than typed: for every cut
 * in the closed set, find the variant key whose RIBBI name is that cut and take
 * ITS catalogue spelling. The one cut with no variant key is the Regular, and
 * the two vocabularies agree on it by construction — the fallback is the cut's
 * own name, not a fourth constant.
 */
const catalogueCutNames: ReadonlyMap<string, FaceCut> = new Map(faceCuts.map((cut) => {
  const key = styleCuts.find((variant) => RIBBI_CUT_NAMES[variant] === cut)
  return [key === undefined ? cut : CATALOGUE_CUT_STYLES[key], cut]
}))

/** `undefined` for a style string neither vocabulary names — a real answer, never a guessed cut. */
export const faceCutOfCatalogueStyle = (style: string): FaceCut | undefined => catalogueCutNames.get(style)

/**
 * WHICH `METADATA.pb` ENTRY IS WHICH CUT — READ, NEVER CONSTRUCTED.
 *
 * This is the generalisation of what used to be `regularFilename`, and it is
 * the same rule four times rather than a new one. A cut is the `fonts { … }`
 * block with that `style` and that `weight`, and the filename kept is THAT
 * BLOCK'S OWN. A filename assembled from the family name —
 * `${family}-Bold.ttf` — is a guess, and it is wrong often enough that the
 * whole pick would fail on families whose files are named for something other
 * than their display name. Nothing below reads the family string at all.
 *
 * ⚠ EXPORTED SINCE spec-install-all-face-cuts STORY 5, FOR THE EMIT STEP. The
 * font browser shows a web family's cuts BEFORE the pick, which means projecting
 * the snapshot's offered-weights list onto this same closed set at build time
 * (`scripts/build-font-index.mjs`). It reads THIS list — the weight and the
 * slope are what a cut IS — rather than minting a fourth table of four strings
 * that would be free to disagree with the fetch path about which weight is a
 * Bold.
 */
export const cutDeclarations: ReadonlyArray<Readonly<{ cut: FaceCut; style: string; weight: number }>> = [
  { cut: 'Regular', style: 'normal', weight: 400 },
  { cut: 'Bold', style: 'normal', weight: 700 },
  { cut: 'Italic', style: 'italic', weight: 400 },
  { cut: 'Bold Italic', style: 'italic', weight: 700 },
]

/** One cut a family publishes: the RIBBI subfamily name, and the file upstream names for it. */
export type PublishedCut = Readonly<{ cut: FaceCut; filename: string }>

/**
 * THE CUT SET A FAMILY PUBLISHES, IN RIBBI ORDER.
 *
 * Absent is a first-class answer: a family publishing only an upright 400 comes
 * back as a one-entry list, which is the common case — measured, 947 of the
 * 1,270 offered web families. Nothing is invented for the three it does not
 * publish and no placeholder is returned for them.
 *
 * ⚠ THE DENOMINATOR MOVED AT spec-install-all-face-cuts STORY 5 AND THE
 * NUMERATOR DID NOT. D2 stopped the dialog offering the three non-variable
 * families that publish no static upright 400 at all — `Buda`, `Molle`,
 * `UnifrakturCook`, which THIS FUNCTION's own caller refuses below, so no pick
 * of them could ever succeed — taking the offered web population from 1,273 to
 * 1,270. None of the three was ever among the 947, because none of them
 * publishes a Regular in the first place.
 */
export function publishedCuts(metadata: FamilyMetadata): ReadonlyArray<PublishedCut> {
  const cuts: PublishedCut[] = []
  for (const declaration of cutDeclarations) {
    const face = metadata.faces.find((entry) => entry.style === declaration.style && entry.weight === declaration.weight)
    if (face !== undefined && face.filename !== '') cuts.push({ cut: declaration.cut, filename: face.filename })
  }
  return cuts
}

/**
 * THE MEDIA TYPE IS READ FROM THE FILE THE METADATA NAMES, and it is one of the
 * two the engine accepts or the pick is refused here rather than at the
 * boundary. This is also the second place the `woff2` route is shut: a `.woff2`
 * filename has no media type in this table.
 */
const mediaTypes: Readonly<Record<string, string>> = { '.ttf': 'font/ttf', '.otf': 'font/otf' }
const mediaTypeOf = (filename: string): string | undefined => {
  const dot = filename.lastIndexOf('.')
  const extension = dot === -1 ? '' : filename.slice(dot).toLowerCase()
  return Object.hasOwn(mediaTypes, extension) ? mediaTypes[extension] : undefined
}

/**
 * `source` NAMES PROVENANCE, NOT A RETRIEVAL PATH (D-16.R.13, DW-160).
 *
 * This field used to be the fetch host declared above followed by
 * `/google/fonts/main/<path>` —
 * a bare mutable branch URL, and three separate defects in one string.
 *
 *   - **A resolvable-looking URL is a promise of fetchability**, and a promise
 *     that decays is worse than none: a dead link in a year reads as "this
 *     provenance is broken" when the provenance is in fact intact.
 *   - **`main` is a branch**, so the string does not identify the bytes it
 *     claims to describe — upstream may move it tomorrow.
 *   - It disagreed in KIND with the committed tier's own `source`, so a reader
 *     holding a `.folio` could not tell which tier a face came from, which
 *     makes the field uninterpretable rather than merely inconsistent.
 *
 * What it carries instead is exactly three things: the **upstream project**,
 * the **path within it**, and the **fetch date**. It carries no scheme and no
 * host — `src/font-provenance.test.ts` is the tripwire, because the convention
 * alone will not hold — and it carries **no SHA-256**: that is already the
 * asset key the face is stored under, and duplicating it would create two
 * authorities on one fact that can disagree.
 *
 * The cost, stated rather than hidden: a recipient cannot refetch the exact
 * bytes. They do not need to — CAP-2 puts the face inside the file, the asset
 * key pins its identity, and `licenceText` and `copyright` travel with it.
 */
export const webFaceSource = (pathWithinProject: string, today: string): string =>
  `google/fonts — ${pathWithinProject}, fetched ${today}`

/** Everything `embedFontFamilyCommand` requires of a fetched face, and the divergence note. */
export type FetchedFace = Readonly<{
  family: string
  style: string
  licence: string
  licenceText: string
  copyright: string
  source: string
  mediaType: string
  bytes: ArrayBuffer
  /** Set when the resolved directory disagrees with the declared token. An observation, never a refusal. */
  layoutDivergence?: string
}>

/**
 * A cut upstream publishes that this fetch asked for and did not get, with the
 * sentence that refused it and whether the refusal SETTLES it.
 *
 * ⚠ `permanence` IS DERIVED HERE, AT THE POINT THE FAILURE IS KNOWN, AND NEVER
 * GUESSED BY THE CALLER (D-2, amended). Only this module sees the response
 * status, the abort, and the `fvar` table; a caller holding only a sentence
 * would have to pattern-match prose to recover what was already known, and
 * would get it wrong the first time a message was reworded. The caller records
 * what it is told.
 */
export type RefusedCut = Readonly<{ style: FaceCut; reason: string; permanence: FaceCutPermanence }>

/**
 * A PICK RESOLVES A FAMILY'S WHOLE CUT SET, NOT ONE FACE.
 *
 * `faces` carries every cut this call actually fetched and verified, in RIBBI
 * order, each with its own `style`, `source`, licence record and bytes. It can
 * be SHORTER than `published` in two ways and the caller must be able to tell
 * them apart, which is what the other two fields are for:
 *
 *   `published` — every cut upstream declares, whether or not it was fetched.
 *                 This is what the family census records, and it is the only
 *                 authority on the question in this designer.
 *   `refused`   — the published cuts that were asked for and refused, each with
 *                 its own reason. A cut that is in `published`, absent from
 *                 `faces` and absent from `refused` was NEVER ATTEMPTED — the
 *                 caller already held it and said so.
 *
 * THE UPRIGHT REGULAR IS ALWAYS THE BASE AND IS ALWAYS PRESENT IN `published`.
 * A family that declares no upright 400 static face is refused outright, before
 * any byte is kept, exactly as it always was.
 */
export type FetchOutcome =
  | Readonly<{ ok: true; faces: ReadonlyArray<FetchedFace>; published: ReadonlyArray<FaceCut>; refused: ReadonlyArray<RefusedCut> }>
  | Readonly<{ ok: false; reason: string; classification?: LicenceClassification }>

type Fetcher = (url: string) => Promise<Response>

/**
 * THE FETCH TIMEOUT — 30 SECONDS, AND THE CONSTANT CARRIES ITS OWN ARITHMETIC
 * (D-16.R.14, D-16.R.42; discharges DW-165).
 *
 * WHAT IT IS FOR. A fetch that REJECTS degrades with a stated message. A fetch
 * that STALLS never settles, so the pick's `finally` never runs and the font
 * family control stays disabled FOR THE REST OF THE SESSION with no message and
 * no way back. That is the worst member of the fetch-failure class and the one
 * the story's own matrix did not cover. A stall is reachable today: a captive
 * portal, a hung proxy, a half-open connection.
 *
 * THE MEASUREMENT THE NUMBER COMES FROM. Timed fetches against the real
 * repository host, five repetitions each, over both shapes in this chain:
 *
 *   `METADATA.pb`                        4,957 B        max   359 ms
 *   `OFL.txt`                            4,383 B        max   275 ms
 *   `Kanit-Regular.ttf`                175,148 B        max    58 ms
 *   `NotoColorEmoji-Regular.ttf`    24,271,604 B        max   805 ms   (this build)
 *   `NotoColorEmoji-Regular.ttf`    24,271,604 B        max 2,097 ms   (the build gate)
 *
 * THE MAXIMUM, NEVER THE MEAN OR THE MEDIAN: this is a ceiling on patience, not
 * an estimate of typical latency. The sizing figure is the build gate's
 * 2,097 ms — the larger of the two independent samples of the same face, kept
 * because a ceiling sized on the faster sample would be the tighter and
 * therefore the wronger one.
 *
 * WHY THAT FACE IS THE TARGET. The budget serves the FETCHABLE population, not
 * the committed one. Of 1,811 index rows, 1,270 are addable after removing
 * variable-only rows, the three that publish no upright Regular, and the 31
 * local-tier families; 1,218 of those publish a `<slug>-Regular.ttf` upstream,
 * and their sizes are median 107,440 B, p90 420,092 B, p99 1,715,888 B, max
 * 24,271,604 B — `Noto Color Emoji`, which sits in the snapshot as
 * `variable: false` and is therefore offerable TODAY. Sizing against the 646 KB
 * the committed faces reach would be a denominator error: that is the wrong
 * population by ~37x at the tail.
 *
 * THE FACTOR IS x10, AND THE REASON IS THE SAMPLE'S OWN LIMIT. One connection,
 * one day, five repetitions. That sample cannot speak for a connection ten
 * times slower, and x10 is the margin bought instead of pretending it can. The
 * arithmetic, in full:
 *
 *   2,097 x 10 = 20,970 <= 30,000
 *
 * WHY 30,000 AND NOT 20,000. `2,097 x 10 = 20,970`, so a 20 s budget puts the
 * single largest offerable face OUTSIDE the budget the x10 factor was chosen to
 * cover — by 5%, on the one case the factor exists for — and the constant would
 * ship carrying arithmetic that contradicts its own stated reason. Cost
 * asymmetry settles the direction: too short WRONGLY REFUSES A LEGITIMATE PICK
 * OF A REAL FONT, loudly and repeatably; too long only lengthens an
 * already-bounded hold on a genuine stall.
 */
export const fetchTimeoutMs = 30_000

/**
 * `AbortSignal.timeout`, NOT A HAND-ARMED `setTimeout` + `clearTimeout`, AND
 * THE SIGNAL GOES INTO `fetch()` ITSELF.
 *
 * Both halves are deliberate and both are about the SAME failure:
 *
 *   THE SIGNAL REACHES THE BODY STREAM. The bytes are read by
 *   `response.arrayBuffer()` AFTER this fetcher has returned. A timeout that
 *   only covered the headers would leave the worst real stall — headers arrive,
 *   then a 24 MB body trickles or stops — completely uncovered. Passing the
 *   signal into `fetch()` makes the abort reach the body.
 *
 *   THERE IS NO DISARM PATH. `AbortSignal.timeout` cannot be cleared, which is
 *   exactly why it is chosen: a hand-armed timer invites a `clearTimeout` when
 *   the headers arrive, and that clear is precisely the line that would stop it
 *   covering `arrayBuffer()`.
 *
 * THE TIMEOUT IS SITED HERE, IN THE DEFAULT FETCHER, and not at the six call
 * sites — one place covers every round-trip in the chain, and a seventh
 * round-trip added later is covered without anyone remembering to cover it.
 *
 * IT TAKES ITS BUDGET AS AN ARGUMENT SO THE TIMEOUT ITSELF CAN BE PROVEN TO
 * FIRE. `AbortSignal.timeout` runs on the platform's own timer, which a fake
 * clock does not reach, so a test of the real mechanism has to be a test at a
 * real, short budget. That is a production factory used with its default
 * everywhere in the product — not a test-only hook — and `fetchTimeoutMs` is
 * asserted separately so a shortened default could never pass unnoticed.
 */
export const timedFetcher = (timeoutMs: number = fetchTimeoutMs): Fetcher => (url) => fetch(url, { signal: AbortSignal.timeout(timeoutMs) })

/**
 * AN ABORT IS ITS OWN DEGRADATION AND MUST NOT BORROW THE OFFLINE ONE.
 *
 * `AbortSignal.timeout` rejects with a `TimeoutError`; an explicitly aborted
 * controller rejects with an `AbortError`. Both are recognised, because both
 * mean the same thing to the author: the request was stopped from this side,
 * having never answered.
 */
const isAbort = (error: unknown): boolean => {
  const name = typeof error === 'object' && error !== null && 'name' in error ? (error as { name: unknown }).name : undefined
  return name === 'TimeoutError' || name === 'AbortError'
}

/**
 * THE STALL'S OWN SENTENCE, AND IT DELIBERATELY DOES NOT REUSE THE OFFLINE
 * WORDING.
 *
 * "You cannot install a family without a network connection" is FALSE when the
 * network is up and the host is hanging — it sends the author to check their
 * wifi over a problem that is not theirs. Located at the control the author
 * acted on, exactly as the offline refusal is.
 *
 * IT ALSO STATES THAT NOTHING WAS RETRIED. A silent retry over a deterministic
 * stall hides it (Story 16.0's `Never:` clause), so this designer does not
 * retry — and says so, because "it took 30 seconds and gave up" reads as a
 * failure to try hard enough unless the decision is visible.
 *
 * ⚠ AND IT SAYS ONLY WHAT A TIMEOUT KNOWS, WHICH IS LESS THAN IT IS TEMPTING TO
 * SAY. An earlier wording claimed "your network is reachable — the font host is
 * not answering". A timeout cannot know either half. The same abort fires when
 * the network is DOWN and the packets are being blackholed rather than refused
 * — the captive-portal case this story cites as its own trigger — when DNS
 * hangs, and on a link simply too slow to move a 24 MB face in 30 s. Asserting
 * a healthy network is the SAME class of false statement this refusal exists to
 * avoid, failing in the other direction: the offline wording sends the author
 * to check wifi that is fine, and this one told them not to check wifi that is
 * not.
 *
 * What is known is exactly this: the request was started and did not finish
 * inside the budget. So that is what it says, and it stays clearly distinct
 * from the offline refusal — which claims, as it may, that a request could not
 * be MADE at all.
 */
const stalledRefusal = (family: string, stage: string): string =>
  `${family} stopped responding while ${stage}: the designer waited ${fetchTimeoutMs / 1000} seconds for a reply that never finished arriving, and then stopped. The request was made and did not complete in time — from here this designer cannot tell whether the font host is hanging, whether something between here and it is, or whether the connection is simply too slow for this face — and nothing was retried automatically, because retrying over a stall that repeats only hides it. Try the pick again if you like; the faces this machine already holds are still offered.`

// STORY 16.5 MIGRATED THE VERB IN EVERY SENTENCE THIS MODULE PRODUCES, from
// "added" to "installed". This resolver runs on exactly one path now — the
// INSTALL — and a refusal that said a family "cannot be added" while nothing was
// ever going to be added to a document was the same class of false UI string
// this epic has ruled against four times. The word "add" survives nowhere here.
const refuse = (reason: string, classification?: LicenceClassification): FetchOutcome => ({ ok: false, reason, classification })

/**
 * ONE PICK, ONE RESOLUTION, AND SINCE spec-install-all-face-cuts STORY 1 THAT
 * RESOLUTION IS THE FAMILY'S WHOLE CUT SET.
 *
 * Probing is once per pick — never at index render and never on a keystroke:
 * four probes across 1,946 families must not become a browsing cost. Adding the
 * other three cuts adds at most three more body reads to a chain that already
 * makes three requests, and `timedFetcher` arms a FRESH timeout per request, so
 * the extra cuts do not share the base chain's deadline.
 *
 * THE ABORT-TERMINATES-THE-CHAIN CONTRACT NOW COVERS THE BASE CHAIN AND NOTHING
 * ELSE — directory probe, licence file, upright Regular. Those three are what
 * the family cannot be installed without, so an abort in any of them refuses
 * the family and stops. EVERY OTHER CUT IS INDEPENDENTLY REFUSABLE: a stalled
 * Bold, a variable Italic and a `.woff2` Bold Italic each come back as one
 * entry in `refused` while the family installs around them. A family whose
 * italic upstream cannot serve is not a family the author may not have.
 *
 * `skip` NAMES THE CUTS THIS CALL IS NOT TO FETCH, AND IT HAS TWO USES.
 *
 * An INSTALL passes the cuts this machine already holds, so re-picking a family
 * the store holds a short set for fetches only what is missing (D-3). A caller
 * that needs ONE FACE — the browser's specimen, which is set in the Regular,
 * and the re-embed refetch, whose document carries the Regular alone — passes
 * `cutsBesideTheRegular`, which is what keeps browsing at one body read per
 * family rather than up to four.
 *
 * A cut named here is absent from `faces` AND from `refused`, because it was
 * never attempted on this call — which is the third state the census has to be
 * able to express.
 */
export async function fetchWebFamily(family: string, fetcher: Fetcher = timedFetcher(), today: string = new Date().toISOString().slice(0, 10), skip: ReadonlyArray<string> = []): Promise<FetchOutcome> {
  const slug = familyDirectorySlug(family)
  if (slug === '') return refuse(`${family} has no directory this designer can derive from its name`)

  let directory: string | undefined
  let metadata: FamilyMetadata | undefined
  let sawSomething = false
  for (const candidate of probeDirectories) {
    let response: Response
    try {
      response = await fetcher(`${fontsRepositoryBase}/${candidate}/${slug}/METADATA.pb`)
    } catch (error) {
      // A STALL AND AN OFFLINE REFUSAL ARE TWO DIFFERENT FAILURES WITH TWO
      // DIFFERENT RIGHT ANSWERS, so they get two sentences.
      if (isAbort(error)) return refuse(stalledRefusal(family, 'looking for its upstream metadata'))
      return refuse(`${family} could not be reached right now (${error instanceof Error ? error.message : String(error)}). You cannot install a family without a network connection; the faces this machine already holds are still offered, and using one of those needs no network at all.`)
    }
    if (response.status === 404) continue
    if (!response.ok) return refuse(`${family}'s upstream metadata responded ${response.status}`)
    sawSomething = true
    const parsed = parseFamilyMetadata(await response.text())
    if (parsed === undefined) return refuse(`${family}'s upstream METADATA.pb could not be read`)
    // THE CONFIRMATION. A mismatch is a REFUSAL, never a fallback to the next
    // directory: the slug is a derivation and this is the check that closes it,
    // so continuing past a disagreement would turn "derived then confirmed"
    // back into the guess it exists to replace.
    if (parsed.name !== family) {
      return refuse(`${family} does not match the upstream directory ${candidate}/${slug}, which publishes "${parsed.name}". The directory is derived from the family name and confirmed by the family's own metadata, and a disagreement is refused rather than guessed past.`)
    }
    directory = candidate
    metadata = parsed
    break
  }
  if (directory === undefined || metadata === undefined) {
    return refuse(sawSomething
      ? `${family} could not be resolved upstream`
      : `${family} is in this designer's snapshot of the family list but is no longer published upstream. The list ships with the designer and ages between releases, so it can name a family that has since been renamed or withdrawn.`)
  }

  // CLASSIFY, THEN EMBED. Nothing below this line is fetched until the terms are
  // admitted, and no byte reaches the document before its licence and copyright
  // are in hand.
  const classification = classifyLicenceToken(metadata.licence)
  if (classification.state !== 'admitted') return refuse(`${family} cannot be installed: ${classification.reason}`, classification)

  // THE CUT SET, AND THE UPRIGHT REGULAR IS STILL REQUIRED OF EVERY FAMILY.
  // It is the base every other cut hangs off: the face a chain entry names, the
  // one a document paints with when nothing asks for bold or italic, and the
  // only cut this designer will not install a family without. A family
  // publishing no static upright 400 is refused here, before any byte is kept,
  // exactly as it was when a family meant one face.
  const cuts = publishedCuts(metadata)
  const regular = cuts.find((entry) => entry.cut === 'Regular')
  if (regular === undefined) {
    return refuse(`${family} publishes no upright Regular (a static face at weight 400) upstream, so there is no face for the rest of its cuts to hang off`)
  }
  // THE REGULAR'S MEDIA TYPE IS CHECKED HERE AND AGAIN PER CUT, AND THE
  // DUPLICATION IS DELIBERATE: this one runs BEFORE the licence file is
  // fetched, so a family whose base face is a `.woff2` is refused without a
  // round-trip for terms that will never travel with anything.
  if (mediaTypeOf(regular.filename) === undefined) return refuse(`${family}'s Regular is published as ${regular.filename}, which is not a font file this engine reads`)

  // NOTHING LEFT TO FETCH IS NOT A REASON TO FETCH THE TERMS (D-3).
  //
  // A re-pick of a family this machine already holds in full happens on the
  // MIGRATION PATH and it is the common case, not a corner: 947 of the 1,270
  // offered families publish a Regular and nothing else, so every one of them
  // installed before this story is a `stored` row holding everything it
  // publishes and carrying no census. Picking it writes the census and fetches
  // no bytes at all — and the licence text is what travels with a NEW RECORD,
  // so reading it when no record will be written is a round-trip that buys
  // nothing. The terms already on the held records are untouched and remain
  // whatever the fetch that wrote them admitted.
  const skipped = new Set(skip)
  if (cuts.every((entry) => skipped.has(entry.cut))) {
    return { ok: true, faces: [], published: cuts.map((entry) => entry.cut), refused: [] }
  }

  // A MISSING ROW IS A STATED REFUSAL, NEVER A MALFORMED FETCH. Unguarded, this
  // lookup would build a URL ending in `undefined`, fetch it, and refuse the
  // family by naming a licence file that does not exist anywhere — a message
  // that sends the reader upstream to look for a file nobody ever published.
  // The admitted set and this map are meant to be the same set; if they ever
  // diverge, this says so in those words.
  if (!Object.hasOwn(licenceFileFor, classification.spdx)) {
    return refuse(`${family} declares ${classification.spdx}, which this designer admits but has no licence file name for, so its terms cannot be fetched to travel with it`, classification)
  }
  const licenceFile = licenceFileFor[classification.spdx]
  const read = await readText(fetcher, `${fontsRepositoryBase}/${directory}/${slug}/${licenceFile}`)
  // A STALL READING THE LICENCE IS A STALL, NOT A MISSING LICENCE FILE. Before
  // the timeout existed this catch could only mean "not there"; now it can also
  // mean "never answered", and reporting a stall as "publishes no OFL.txt"
  // would send the author upstream to look for a file that is sitting there.
  if (!read.ok && read.stalled) return refuse(stalledRefusal(family, 'sending the text of its licence'))
  if (!read.ok || read.text.trim() === '') {
    return refuse(`${family} declares ${classification.spdx} but publishes no ${licenceFile} beside its face, so its terms cannot travel with it. A document may not carry a face without the text of its licence.`)
  }
  const licenceText = read.text

  // LAYOUT DISAGREEMENT IS AN OBSERVATION, NOT A REFUSAL. Recorded because
  // systematically it means the probe order is costing round-trips. It is a
  // property of the FAMILY's directory, so it is computed once and carried by
  // every cut the family yields rather than re-derived per face.
  const expected = { 'OFL-1.1': 'ofl', 'Apache-2.0': 'apache', 'Ubuntu-font-1.0': 'ufl' }[classification.spdx]
  const layoutDivergence = expected !== undefined && expected !== directory
    ? `${family} is published under ${directory}/ while its own metadata declares ${classification.token} (${classification.spdx}); the metadata is the authority on the terms and the directory is only where the files sit`
    : undefined

  /**
   * ONE CUT, FETCHED AND VERIFIED ON ITS OWN. Returns the face, or the sentence
   * that refuses it — never both, and never a partial face.
   *
   * EVERY PER-FACE CHECK RUNS OVER THIS CUT'S OWN BYTES: its media type, its
   * `fvar` table, its nameID 0. One bad cut is refused on its own evidence, so
   * a family whose italic is a variable font still installs its Regular and its
   * Bold. The caller decides what a refusal MEANS — the Regular's refuses the
   * family, every other cut's is recorded and stepped over.
   */
  const fetchCut = async (cut: FaceCut, filename: string): Promise<Readonly<{ ok: true; face: FetchedFace }> | Readonly<{ ok: false; reason: string; permanence: FaceCutPermanence }>> => {
    // AN UNREADABLE MEDIA TYPE IS PERMANENT AND COSTS NO REQUEST. The filename
    // upstream publishes is not going to become a `.ttf` by asking again.
    const cutMediaType = mediaTypeOf(filename)
    if (cutMediaType === undefined) return { ok: false, permanence: 'permanent', reason: `${family}'s ${cut} is published as ${filename}, which is not a font file this engine reads` }
    let bytes: ArrayBuffer
    try {
      const response = await fetcher(`${fontsRepositoryBase}/${directory}/${slug}/${filename}`)
      if (!response.ok) {
        // ⚠ ONLY "THERE IS NO SUCH FILE" IS PERMANENT. 404 and 410 are upstream
        // stating what it publishes; every other status is a fact about this
        // attempt — 5xx is the host having a bad minute, 429 is rate limiting,
        // and a 403 from a proxy is a captive portal. Settling a cut on any of
        // those would strand a face upstream really has.
        const permanence: FaceCutPermanence = response.status === 404 || response.status === 410 ? 'permanent' : 'transient'
        return { ok: false, permanence, reason: `${family}'s face ${filename} responded ${response.status}` }
      }
      bytes = await response.arrayBuffer()
    } catch (error) {
      // THIS CATCH SPANS `arrayBuffer()` AS WELL AS THE REQUEST, and that is the
      // whole reason the signal is passed into `fetch()` rather than armed around
      // it: the largest offerable face is 24 MB, so the body is where a real
      // stall lives.
      //
      // BOTH ARMS ARE TRANSIENT, AND THEY ARE THE CASES THE AMENDMENT EXISTS
      // FOR. A stall and an offline connection say nothing whatever about what
      // upstream publishes, and recording them as settled is what stranded a
      // Bold with no path back.
      if (isAbort(error)) return { ok: false, permanence: 'transient', reason: stalledRefusal(family, `sending the ${filename} face itself`) }
      return { ok: false, permanence: 'transient', reason: `${family} could not be fetched right now (${error instanceof Error ? error.message : String(error)}). You cannot install a family without a network connection; the faces this machine already holds are still offered, and using one of those needs no network at all.` }
    }

    // TWO READS OF THE SAME BYTES, IN THIS ORDER, INSIDE ONE GUARD.
    //
    // THE `fvar` FILTER IS FIRST AND IT MAY ONLY REFUSE (Story 16.5). Under
    // install/embed separation the engine's own variable-face refusal no longer
    // reaches the author at the moment they acted — it reaches them at first USE,
    // which is a worse moment — so the refusal is ALSO made here, over the bytes
    // in hand, at the moment the author asked for the family. It is a filter and
    // not an authority: `fontset.RefuseVariableFace` still decides what enters a
    // document, and nothing here admits anything. See `faceIsVariable`'s own doc
    // for why a refuse-only copy is permitted at install and forbidden at the
    // command.
    //
    // AND AN UNPARSABLE FACE IS REFUSED BEFORE EITHER LOOKUP, by the container
    // guard both readers share: a 200 carrying an error page throws out of
    // `requireStaticTrueTypeTables` and is stated in those words. Go answers
    // `nil` for the same bytes. That divergence is deliberate and is asserted on
    // purpose in `src/font-variable-face-tie.test.ts`.
    //
    // nameID 0 IS SECOND. Absent is a refusal, because the engine refuses to load
    // a document that embeds a face with no copyright, so admitting one here would
    // put a file this product cannot open one step away.
    let copyright: string
    try {
      // A VARIABLE FACE IS PERMANENT: the `fvar` table is in the bytes upstream
      // publishes, and no number of retries will remove it.
      if (faceIsVariable(bytes)) {
        return { ok: false, permanence: 'permanent', reason: `${family} cannot be installed: the face published upstream as ${filename} is a VARIABLE font (it carries an \`fvar\` table), and a document may only carry a single static face — PDF 1.7 cannot express a variable font, and this designer will not pick an instance on your behalf. Nothing was kept on this machine.` }
      }
      copyright = faceCopyright(bytes)
    } catch (error) {
      // ⚠ A BODY THAT WILL NOT PARSE IS TRANSIENT, WHICH LOOKS WRONG AND IS NOT.
      // `requireStaticTrueTypeTables` throws for "this is not a font", and the
      // commonest real producer of those bytes is not a broken upstream face —
      // it is a 200 carrying a captive portal's HTML login page, which is the
      // most transient failure in the whole set. The default rule decides it:
      // a shape that is not confidently permanent is transient, because
      // retrying costs one request and stranding costs a face for good.
      return { ok: false, permanence: 'transient', reason: `${family} cannot be installed: ${error instanceof Error ? error.message : String(error)}` }
    }

    return {
      ok: true,
      face: {
        family,
        style: cut,
        licence: classification.spdx,
        licenceText,
        copyright,
        source: webFaceSource(`${directory}/${slug}/${filename}`, today),
        mediaType: cutMediaType,
        bytes,
        layoutDivergence,
      },
    }
  }

  // THE BASE CHAIN ENDS HERE, AT THE REGULAR. Its refusal is the family's
  // refusal and nothing below it runs — which is what keeps a stall on the base
  // face from becoming four stalls, and what makes "nothing was kept" true.
  const faces: FetchedFace[] = []
  const refused: RefusedCut[] = []
  if (!skipped.has('Regular')) {
    const base = await fetchCut('Regular', regular.filename)
    if (!base.ok) return refuse(base.reason)
    faces.push(base.face)
  }

  // AND EVERY OTHER CUT IS ITS OWN, SEQUENTIALLY AND WITHOUT `Promise.all`.
  //
  // THREE EXTRA CUTS IS THREE EXTRA REQUESTS AT MOST, and they are made one at
  // a time on purpose: this designer's whole host discipline is one chain of
  // stated round-trips, and firing four concurrent cross-origin body reads at
  // the font host — up to 24 MB each — would replace a bounded, cancellable
  // sequence with a burst nothing here can reason about. The cost is latency on
  // a path the author already waits on; the gain is that the abort budget, the
  // refusal order and the request count all stay things a test can write down.
  for (const entry of cuts) {
    if (entry.cut === 'Regular' || skipped.has(entry.cut)) continue
    const fetched = await fetchCut(entry.cut, entry.filename)
    if (fetched.ok) faces.push(fetched.face)
    else refused.push({ style: entry.cut, reason: fetched.reason, permanence: fetched.permanence })
  }

  return { ok: true, faces, published: cuts.map((entry) => entry.cut), refused }
}

/**
 * THE OUTCOME IS A UNION AND NOT `string | undefined`, because "there is no
 * such file" and "the host never answered" are two different failures and only
 * one of them is the author's problem to act on.
 */
type TextOutcome = Readonly<{ ok: true; text: string }> | Readonly<{ ok: false; stalled: boolean }>

async function readText(fetcher: Fetcher, url: string): Promise<TextOutcome> {
  try {
    const response = await fetcher(url)
    if (!response.ok) return { ok: false, stalled: false }
    return { ok: true, text: await response.text() }
  } catch (error) {
    // AND THIS CATCH `return`s A FAILURE RATHER THAN CONTINUING, which is one
    // of the three points at which the chain terminates on the first abort.
    // `src/font-source.test.ts` asserts that termination table-driven, with the
    // fetcher call count written as a literal.
    return { ok: false, stalled: isAbort(error) }
  }
}
