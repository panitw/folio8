import { classifyLicenceToken, type LicenceClassification } from './font-licence.ts'
import { faceCopyright, faceIsVariable } from './font-name-table.ts'

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
 * WHICH FILE IS REGULAR IS READ, NOT CONSTRUCTED.
 *
 * It is the `fonts { style: "normal", weight: 400 }` entry's own `filename`. A
 * filename assembled from the family name — `${family}-Regular.ttf` — is a guess,
 * and it is wrong often enough that the whole pick would fail on families whose
 * files are named for something other than their display name.
 */
export function regularFilename(metadata: FamilyMetadata): string | undefined {
  return metadata.faces.find((face) => face.style === 'normal' && face.weight === 400)?.filename
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

export type FetchOutcome =
  | Readonly<{ ok: true; face: FetchedFace }>
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
 * the committed one. Of 1,811 index rows, 1,273 are addable after removing
 * variable-only rows and the 31 local-tier families; 1,218 of those publish a
 * `<slug>-Regular.ttf` upstream, and their sizes are median 107,440 B, p90
 * 420,092 B, p99 1,715,888 B, max 24,271,604 B — `Noto Color Emoji`, which sits
 * in the snapshot as `variable: false` and is therefore offerable TODAY. Sizing
 * against the 646 KB the committed faces reach would be a denominator error:
 * that is the wrong population by ~37x at the tail.
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
 * ONE PICK, ONE RESOLUTION. Probing is once per pick — never at index render and
 * never on a keystroke: four probes across 1,946 families must not become a
 * browsing cost.
 */
export async function fetchWebFamily(family: string, fetcher: Fetcher = timedFetcher(), today: string = new Date().toISOString().slice(0, 10)): Promise<FetchOutcome> {
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

  const filename = regularFilename(metadata)
  if (filename === undefined) {
    return refuse(`${family} publishes no upright Regular (a static face at weight 400) upstream, so there is no single face to embed`)
  }
  const mediaType = mediaTypeOf(filename)
  if (mediaType === undefined) return refuse(`${family}'s Regular is published as ${filename}, which is not a font file this engine reads`)

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

  let bytes: ArrayBuffer
  try {
    const response = await fetcher(`${fontsRepositoryBase}/${directory}/${slug}/${filename}`)
    if (!response.ok) return refuse(`${family}'s face ${filename} responded ${response.status}`)
    bytes = await response.arrayBuffer()
  } catch (error) {
    // THIS CATCH SPANS `arrayBuffer()` AS WELL AS THE REQUEST, and that is the
    // whole reason the signal is passed into `fetch()` rather than armed around
    // it: the largest offerable face is 24 MB, so the body is where a real
    // stall lives.
    if (isAbort(error)) return refuse(stalledRefusal(family, `sending the ${filename} face itself`))
    return refuse(`${family} could not be fetched right now (${error instanceof Error ? error.message : String(error)}). You cannot install a family without a network connection; the faces this machine already holds are still offered, and using one of those needs no network at all.`)
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
    if (faceIsVariable(bytes)) {
      return refuse(`${family} cannot be installed: the face published upstream as ${filename} is a VARIABLE font (it carries an \`fvar\` table), and a document may only carry a single static face — PDF 1.7 cannot express a variable font, and this designer will not pick an instance on your behalf. Nothing was kept on this machine.`)
    }
    copyright = faceCopyright(bytes)
  } catch (error) {
    return refuse(`${family} cannot be installed: ${error instanceof Error ? error.message : String(error)}`)
  }

  // LAYOUT DISAGREEMENT IS AN OBSERVATION, NOT A REFUSAL. Recorded because
  // systematically it means the probe order is costing round-trips.
  const expected = { 'OFL-1.1': 'ofl', 'Apache-2.0': 'apache', 'Ubuntu-font-1.0': 'ufl' }[classification.spdx]
  const layoutDivergence = expected !== undefined && expected !== directory
    ? `${family} is published under ${directory}/ while its own metadata declares ${classification.token} (${classification.spdx}); the metadata is the authority on the terms and the directory is only where the files sit`
    : undefined

  return {
    ok: true,
    face: {
      family,
      style: 'Regular',
      licence: classification.spdx,
      licenceText,
      copyright,
      source: webFaceSource(`${directory}/${slug}/${filename}`, today),
      mediaType,
      bytes,
      layoutDivergence,
    },
  }
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
