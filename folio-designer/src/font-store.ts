// THE MACHINE FONT STORE — A FETCHED FACE STAYS ON THIS MACHINE (Story 16.2).
//
// WHAT IT IS. An origin-scoped IndexedDB store of the faces this designer has
// already fetched, keyed by the SHA-256 of the face bytes — the same content
// address `.folio`'s `assets` map uses, and the same one Go derives at
// `component_commands.go`'s `embedFontFamily`. A read in front of the fetch is
// the whole feature: a hit means no request leaves the machine, which is what
// makes a re-pick work with the network down.
//
// WHAT IT IS NOT, AND BOTH HALVES MATTER:
//
//   IT IS NOT A LIST OF THE FONTS INSTALLED ON THIS COMPUTER. SPEC-fonts'
//   Non-goal *"No host fonts. Faces installed on the authoring or rendering
//   machine are never enumerated or read."* is the ONE clause of that Non-goal
//   D-16.1 left standing, and it is untouched here. The Local Font Access API
//   is not used, not referenced and not feature-detected anywhere in this
//   designer, by any of its spellings — which are enumerated in
//   `scripts/host-font-access.mjs` and deliberately nowhere else, including in
//   this comment, because that scanner reads RAW source and a spelling named in
//   prose would be an occurrence like any other. `src/host-font-access.test.ts`
//   runs it over the whole designer and red-proves it by deleting the guard.
//
//   IT IS NOT A SECOND COPY OF A DOCUMENT'S FONTS. A `.folio` carries its own
//   faces (CAP-2). This store SHORTENS A FETCH; it never stands in for what a
//   file contains, and removing an entry never changes a saved document. It is
//   a cache and a source, never an authority.
//
// AUTHORING ONLY. FR33 is untouched: nothing is fetched or read from the
// machine at render time. Nothing in this module is reachable from the render
// path, which runs in Go over the document's own bytes.
//
// ─────────────────────────────────────────────────────────────────────────────
// WHY IndexedDB, AND WHY `localStorage` IS REFUSED ON ARITHMETIC (D-16.2).
//
// The owner's words were "local storage". The MECHANISM is the engineer's, and
// `localStorage` cannot do this job. Three measurements, written down here so
// the next reader does not "simplify" it back:
//
//   1. `localStorage` is a per-origin quota of roughly 5 MB, and it stores
//      STRINGS. There is no byte type in it at all.
//   2. Putting bytes in a string means base64, which is +33%. A measured
//      `Sarabun-Regular.ttf` is 90,220 bytes, so it costs about 120 KB stored
//      — roughly 40 faces of that size before the origin is full, and the
//      largest face this designer can legitimately offer is `Noto Color Emoji`
//      at 24,271,604 bytes, which is ~32 MB base64 and does not fit AT ALL,
//      alone, in an empty 5 MB origin.
//   3. Its failure mode is the wrong shape: `QuotaExceededError` is thrown
//      SYNCHRONOUSLY from the assignment, with no partial-write path and no way
//      to ask first. A store whose only signal is a throw in the middle of a
//      write cannot degrade; it can only fail.
//
// IndexedDB stores `ArrayBuffer` natively, is asynchronous, and its quota is
// origin-scoped storage rather than a 5 MB string budget. It is also the store
// the service worker's Cache API already sits beside, so this adds no new
// storage REGIME to the origin — only a new database in one that exists.
//
// ─────────────────────────────────────────────────────────────────────────────
// WHY THE CONTENT HASH AND NEVER THE FAMILY NAME.
//
// The store answers "do I already have these bytes", which is the question
// `assetKeyReferenced` and the `assets` map answer. Keying by name would make
// it answer a DIFFERENT question — "do I have something called Sarabun" — and
// the day upstream changes the face, the store would hand over the old bytes
// under the new name and the document would carry a face nobody chose. Under
// AD-8 a family that changes upstream is a DIFFERENT KEY, not a silent
// substitution.
//
// ─────────────────────────────────────────────────────────────────────────────
// THE LIFETIME, WHICH IS THE THIRD ONE IN THIS APPLICATION AND IS ARGUED HERE
// FOR THE SAME REASON THE SECOND ONE WAS (`embedded-face-registry.ts`).
//
//   ImagePaint's lifetime is the COMPONENT's — one object URL per mounted
//   instance, released on unmount.
//   `registerCarriedFaces`' lifetime is the DOCUMENT's — one registration for
//   every carried face, released when the document is replaced, because
//   `document.fonts` is a global name-keyed registry that a per-component
//   lifetime would corrupt.
//   This store's lifetime is the MACHINE's — strictly, the browser profile's
//   origin. It outlives every document and every session, and NOTHING in it is
//   released when a document is replaced. That is the point of it, and it is
//   also why nothing in it may ever be treated as document state: a document's
//   truth is the `.folio`, always.
//
// "ON THIS MACHINE" IS A DELIBERATE UNDERSTATEMENT AND THE UI MUST NOT IMPROVE
// ON IT. Origin-scoped browser storage means: this browser, this profile, this
// origin. Not the operating system, not synced, not shared with another browser
// on the same machine, not shared with another user of the same computer.

/**
 * The database and its three object stores.
 *
 * VERSION 2 SINCE spec-install-all-face-cuts STORY 1, AND EVERY SCHEMA CHANGE
 * IS ADDITIVE. Version 1 held the two face stores below; version 2 adds the
 * family census beside them and touches neither. A v1 database is opened, given
 * the one store it lacks, and handed back WITH EVERY FACE RECORD IT ALREADY
 * HELD — an author's downloaded typefaces may not be wiped by a schema change,
 * and `src/font-store.test.ts` drives a real v1 database through this path
 * rather than assuming it.
 *
 * ⚠ `databaseVersion` IS A FLOOR, NOT THE AUTHORITY ON THE SCHEMA. The
 * authority is `objectStores` below, and `openFontStore` verifies the stores
 * that are ACTUALLY PRESENT rather than trusting the version number to imply
 * them. A database can sit at the current version and still be missing a store
 * — an intermediate build that bumped the version before a `createObjectStore`
 * line existed is enough — and for such a database `onupgradeneeded` never
 * fires. That state was unrepairable and broke every read and write with a
 * `NotFoundError` the author could do nothing about; it is now repaired on
 * open, which is also why adding a store to `objectStores` does not strictly
 * require moving this number.
 */
const databaseName = 'folio8-machine-font-store'
const databaseVersion = 2
/** Metadata, keyed by the face's content address. Read by `list()` on its own. */
const faceStoreName = 'faces'
/**
 * Bytes, under the SAME key, in a SEPARATE object store — and the split is the
 * one structural decision in this module.
 *
 * `list()` populates a dropdown. If the bytes lived on the metadata record,
 * every listing would deserialize every face — tens of megabytes to render a
 * menu — because IndexedDB has no projection: a `getAll` returns whole records
 * or nothing. Split, a listing reads only the small store, and the bytes are
 * read exactly once, at the moment a pick needs them.
 *
 * Both stores are written and deleted in ONE transaction, so the pair cannot
 * half-exist by way of this module. A pair that is half-present anyway — a
 * browser that dropped one store, a write interrupted by a crash — is exactly
 * what `get()` and `list()` treat as a CORRUPT ENTRY and drop.
 */
const byteStoreName = 'face-bytes'

/**
 * THE FAMILY CENSUS — WHAT UPSTREAM PUBLISHES FOR A FAMILY, AND WHICH OF ITS
 * CUTS THIS MACHINE ASKED FOR AND WAS REFUSED (D-7, spec-install-all-face-cuts).
 *
 * ⚠ IT IS THE ONE FAMILY-KEYED THING IN THIS MODULE, AND THAT IS DELIBERATE.
 * The face store stays content-addressed: a face record is keyed by the SHA-256
 * of its bytes and says nothing about its family's other cuts. The census is a
 * SEPARATE store because the fact it holds — what the family PUBLISHES — is a
 * fact about the family and about nothing this machine happens to hold. Copying
 * it onto each face record would be N copies of one fact that can disagree,
 * which is exactly what `webFaceSource`'s comment above refuses in those words.
 *
 * WHY IT EXISTS AT ALL. Installing a family now fetches every cut it publishes,
 * and a cut can be refused on its own — a variable Bold, a `.woff2` Italic, a
 * stalled body. Without a record of that refusal the completeness predicate
 * would read the family as short FOREVER and offer it for install on every
 * render, so a family whose italic upstream cannot serve would re-offer until
 * the end of time. The refusal is what terminates that loop.
 */
const censusStoreName = 'family-census'

/**
 * ⚠ EVERY OBJECT STORE THIS MODULE NAMES, IN ONE PLACE — THE SINGLE AUTHORITY.
 *
 * This list is read by BOTH halves that have to agree: the upgrade that
 * CREATES the stores, and `transact`, which NAMES them in every transaction.
 * They used to be two hand-maintained lists, and a database that reached the
 * current version without one of the stores made every read and write throw
 * `NotFoundError` — an IndexedDB internal shown to the author — with no path
 * back but clearing site data by hand. Two lists that can disagree is the
 * shape that allowed it; one list is the fix.
 *
 * A store added here is created on the next open of any existing database,
 * whether or not `databaseVersion` moves, because the shape check below repairs
 * by shape rather than trusting the version number.
 */
const objectStores = [faceStoreName, byteStoreName, censusStoreName] as const

/** The key path each store is created with. `undefined` means an out-of-line key, as the byte store uses. */
const storeKeyPaths: Readonly<Record<string, string | undefined>> = {
  [faceStoreName]: 'key',
  [byteStoreName]: undefined,
  [censusStoreName]: 'family',
}

/** Which of this module's stores the open database does NOT have. Empty is a sound schema. */
const missingStores = (database: IDBDatabase): ReadonlyArray<string> =>
  objectStores.filter((name) => !database.objectStoreNames.contains(name))

/**
 * THE ADDITIVE CREATE, AND IT IS THE ONLY SCHEMA WRITE IN THIS MODULE.
 *
 * Every branch is a `contains` guard and a `createObjectStore`, and there is
 * deliberately NO `deleteObjectStore` here or anywhere else: a schema change
 * that wiped an author's downloaded typefaces is the failure mode that ruled
 * the per-face-field option out of D-7, and it may not reappear through the
 * path it was traded for. Repairing a broken shape is never a reason to drop
 * data — the missing store is added beside what is already held.
 */
const createMissingStores = (upgrading: IDBDatabase): void => {
  for (const name of objectStores) {
    if (upgrading.objectStoreNames.contains(name)) continue
    const keyPath = storeKeyPaths[name]
    upgrading.createObjectStore(name, keyPath === undefined ? undefined : { keyPath })
  }
}

/**
 * WHETHER A REFUSED CUT IS SETTLED OR MERELY NOT HERE YET (D-2/D-5, amended
 * 2026-09-20 after review).
 *
 * THE FIRST CUT OF THIS STORY RECORDED ONE KIND OF REFUSAL AND THAT WAS WRONG.
 * A stalled body and an offline connection were written down exactly like a
 * variable `fvar` or a 404, and because D-5 settled a cut on ANY recorded
 * refusal, one bad minute of network permanently stranded a Bold the family
 * really publishes, with no path back: the family read complete, so it was
 * never offered for install again and the cut was never retried.
 *
 *   `permanent` — the refusal is a property of what upstream publishes and
 *                 will not change by trying again: a variable `fvar`, a
 *                 filename this engine cannot read, a 404. It SETTLES the cut,
 *                 which is what keeps D-5's predicate from becoming a silent
 *                 retry loop.
 *   `transient` — the refusal is a property of this attempt: a stalled body,
 *                 no network, a 5xx. It settles NOTHING. The family reads
 *                 incomplete and the cut is fetched again on a later pick.
 *
 * ⚠ THE DEFAULT IS `transient`, AND THE ASYMMETRY IS THE REASON. Retrying
 * something permanent costs one wasted request on a pick the author made
 * anyway; stranding something transient costs a Bold that is gone for good. So
 * only a failure shape that is CONFIDENTLY permanent is written down as one,
 * and everything else — including a body that would not parse, which is exactly
 * what a captive portal's 200 HTML login page looks like — is transient.
 */
export type FaceCutPermanence = 'permanent' | 'transient'

/** One cut upstream publishes that this machine asked for and did not get, with the reason and whether it is settled. */
export type FamilyCutRefusal = Readonly<{ style: string; reason: string; permanence: FaceCutPermanence }>

/**
 * THE CENSUS RECORD, AND IT DISTINGUISHES THREE STATES RATHER THAN TWO — which
 * is the obligation D-7's approval came with, because the completeness
 * predicate needs all three and story 2's panel sentence will too:
 *
 *   `published` holds the cut AND `refused` names it  →  upstream publishes it
 *                                                        and the fetch failed.
 *   `published` does not hold the cut                 →  upstream publishes no
 *                                                        such cut.
 *   `published` holds it and `refused` does not, and  →  never attempted.
 *   no face record of that style is on this machine
 *
 * ⚠ THERE IS NO `held` FIELD, AND ITS ABSENCE IS THE DECISION'S OWN REASONING
 * APPLIED TO ITSELF. D-7 chose a separate store over a per-face field because
 * that is ONE AUTHORITY ON ONE FACT; a `held` list here would be a SECOND
 * authority on a fact the face records already carry — every stored record has
 * a `style`, so which cuts this machine holds is read off the face set. Two
 * lists that can disagree is the failure this store's own shape exists to
 * avoid, and it would also fail in a way the reader can see: the face store
 * SELF-HEALS by dropping a record it cannot verify, and a `held` list would go
 * on claiming a cut whose bytes had just been dropped.
 *
 * `published` IS THE AUTHORITY ON WHAT THE FAMILY PUBLISHES.
 * `generated/font-index.ts`'s `styles` must not become a second one — that
 * carry-through is a later story's and is deliberately not consulted here.
 */
export type FamilyCensus = Readonly<{
  family: string
  /** The RIBBI cuts upstream publishes, as subfamily names: `Regular`, `Bold`, `Italic`, `Bold Italic`. */
  published: ReadonlyArray<string>
  /** The published cuts this machine asked for and did not get, each with the sentence that refused it. */
  refused: ReadonlyArray<FamilyCutRefusal>
  /** The day the census was written, `YYYY-MM-DD`. */
  recordedAt: string
}>

/**
 * D-5's COMPLETENESS PREDICATE, OVER THE CENSUS AND THE CUTS ACTUALLY HELD.
 *
 * A family is complete when it holds every cut it publishes OR carries a
 * recorded PERMANENT refusal for each cut it lacks. That second clause is what
 * D-2's record exists for and what keeps a family whose Italic upstream cannot
 * serve from being offered for install on every render for ever.
 *
 * ⚠ A TRANSIENT REFUSAL SETTLES NOTHING, and that is the amendment. A stalled
 * Bold leaves the family INCOMPLETE, so it is offered for install again and the
 * Bold is fetched on the next pick — which is what makes `font-source.ts`'s own
 * stall sentence, *"Try the pick again if you like"*, true for a cut that is not
 * the base.
 *
 * MEASURED: 947 of the 1,270 offered web families publish a Regular and nothing
 * else — `font-index.json`, `axes == []`, `styles` carrying `400` and none of
 * `700`, `400i`, `700i` — so for 74.5% of the population this is complete the
 * moment the Regular lands and the census never has a refusal to carry at all.
 *
 * ⚠ THE PREDICATE IS THE PROJECTION, NOT `styles == ["400"]`. Only 935 rows
 * carry that literal one-entry list; the other 12 also publish weights outside
 * the RIBBI four — a 500, a 300 — which the emit step DROPS rather than maps,
 * so they yield a Regular alone exactly like the 935 and belong in the same
 * count. Citing the narrower predicate beside the wider number is how 935 and
 * 947 come to be read as one fact.
 */
export const censusIsComplete = (census: FamilyCensus, heldCuts: ReadonlySet<string>): boolean =>
  census.published.every((cut) => heldCuts.has(cut) || census.refused.some((entry) => entry.style === cut && entry.permanence === 'permanent'))

/**
 * Everything the store keeps about a face EXCEPT its bytes: what `list()`
 * returns, and what the family control shows.
 *
 * EVERYTHING `embedFontFamily` REQUIRES IS HERE, and that is a constraint
 * rather than a convenience: a face offered from the store must be embeddable
 * WITHOUT A NETWORK, and the command refuses without `licence`, `licenceText`
 * and `copyright` — Story 8.6 made all three required of an asset a chain names
 * (`parse.go`'s `requireEmbeddedFaceLicence`). A store that kept the bytes and
 * dropped the terms would put a document its own parser refuses one step away.
 */
export type StoredFace = Readonly<{
  /** The lowercase hex SHA-256 of the face bytes. The asset key, byte for byte. */
  key: string
  family: string
  style: string
  licence: string
  licenceText: string
  copyright: string
  /** Provenance, carried BYTE-IDENTICALLY. See `put`. */
  source: string
  /**
   * WHETHER THE AUTHOR ACKNOWLEDGED THEIR RIGHT TO THIS FACE AT IMPORT
   * (spec-font-sources-and-embedding story 6, D4).
   *
   * ⚠ IT IS A FIELD AND NOT AN INFERENCE FROM `source`. `source` is TEXT — the
   * sentence `authorSuppliedFaceSource` writes — and parsing it back to decide
   * whether a face may be embedded would be a second authority over one fact,
   * which story 5's D3 forbids. The store records the answer; nothing reads the
   * prose.
   *
   * A CATALOGUE FACE IS ALWAYS `false`, and that is permanent rather than
   * pending: the acknowledgement is an assertion the AUTHOR makes about a file
   * they supplied, and a face this product distributes carries none.
   */
  authorAcknowledged: boolean
  mediaType: string
  scripts: ReadonlyArray<string>
  /** The day the bytes were fetched, `YYYY-MM-DD`. */
  fetchedAt: string
  /** The length of the bytes held under `key`, so a listing can state a size without reading them. */
  byteLength: number
}>

/** A `StoredFace` plus the bytes themselves: what `put()` writes and `get()` reads back. */
export type StoredFaceRecord = StoredFace & Readonly<{ bytes: ArrayBuffer }>

/**
 * EVERY OPERATION RETURNS AN OUTCOME AND NOTHING THROWS INTO A CALLER.
 *
 * A private window, cleared site data, a quota refusal and a browser with the
 * database disabled are all ordinary conditions of a designer that must keep
 * working. A rejected promise reaching the pick path would turn a caching
 * failure into a failed pick, which is precisely backwards: the caching is what
 * failed, and the fetch and the embed are unaffected.
 */
export type StoreOutcome<T> =
  | Readonly<{ ok: true; value: T }>
  | Readonly<{ ok: false; reason: string }>

export type FontStore = Readonly<{
  get(key: string): Promise<StoreOutcome<StoredFaceRecord | undefined>>
  put(record: StoredFaceRecord): Promise<StoreOutcome<void>>
  list(): Promise<StoreOutcome<ReadonlyArray<StoredFace>>>
  remove(key: string): Promise<StoreOutcome<void>>
  /** Every family census this machine holds. Read once beside `list()`, for the same reason. */
  listCensus(): Promise<StoreOutcome<ReadonlyArray<FamilyCensus>>>
  /** Records what a family publishes and which of its cuts were refused. One row per family. */
  putCensus(record: FamilyCensus): Promise<StoreOutcome<void>>
}>

/**
 * THE KEY, COMPUTED IN THE BROWSER, FOR THE STORE'S OWN ADDRESSING.
 *
 * The browser does NOT hash anything the document carries — Go alone hashes,
 * bounds and admits the bytes that reach `assets` (D-5.13.1/D-5.13.3), and this
 * changes none of that. What this hashes is the store's own address for its own
 * shelf, and it must agree with Go's exactly or the two addressings drift and
 * a hit would be a hit on the wrong bytes.
 *
 * THE AGREEMENT IS PINNED FROM BOTH SIDES, not asserted from this one.
 * `src/font-store.test.ts` and `folio-go/stored_face_key_tie_test.go`
 * (`TestStoredFaceKeyTie`) write out the SAME digest over the SAME 110-byte
 * fixture and each derive it by their own means — two suites pinned to one
 * shared constant. Each file's comment names the other.
 *
 * `crypto.subtle` is available in every browser this designer supports and in
 * the test environment (measured at the build gate: jsdom 28.1.0 provides
 * `crypto.subtle` even though it provides no IndexedDB at all).
 */
export async function storedFaceKey(bytes: ArrayBuffer): Promise<string> {
  // A VIEW, NOT THE BUFFER ITSELF, so a buffer that came back from storage in
  // another realm still hashes. See `asArrayBuffer` for why that is a real
  // condition and not a hypothetical one.
  const digest = await crypto.subtle.digest('SHA-256', new Uint8Array(bytes))
  let hex = ''
  for (const byte of new Uint8Array(digest)) hex += byte.toString(16).padStart(2, '0')
  return hex
}

/**
 * `instanceof ArrayBuffer` IS REALM-SCOPED, AND THE BYTES COMING OUT OF THIS
 * STORE HAVE CROSSED A REALM.
 *
 * MEASURED, and it is the exact defect an independent IndexedDB implementation
 * was brought in to be able to find: a buffer read back out of the store
 * reports `constructor.name === 'ArrayBuffer'` and `byteLength` correctly while
 * `value instanceof ArrayBuffer` is **false**, because the structured clone
 * produced it in a different realm from the one this module's `ArrayBuffer`
 * binding names. An `instanceof` gate therefore threw away every sound entry
 * and reported it as a lost-its-bytes corruption — a store that silently
 * remembers nothing, which is the one failure this feature cannot have.
 *
 * The brand check below is realm-independent. A view is accepted too and
 * narrowed to exactly its own window, because a `Uint8Array` over a larger
 * buffer is a different set of bytes from the buffer it sits in.
 */
function asArrayBuffer(value: unknown): ArrayBuffer | undefined {
  if (value instanceof ArrayBuffer) return value
  if (ArrayBuffer.isView(value)) return value.buffer.slice(value.byteOffset, value.byteOffset + value.byteLength) as ArrayBuffer
  if (typeof value === 'object' && value !== null && Object.prototype.toString.call(value) === '[object ArrayBuffer]') return value as ArrayBuffer
  return undefined
}

/** The key shape both sides agree on: 64 lowercase hex characters, exactly `isCarriedFaceAssetKey`'s. */
const storedKeyShape = /^[a-f0-9]{64}$/

const failed = (reason: string): StoreOutcome<never> => ({ ok: false, reason })
const succeeded = <T>(value: T): StoreOutcome<T> => ({ ok: true, value })
const detail = (error: unknown): string => error instanceof Error ? error.message : String(error)

/**
 * A RECORD READ BACK IS NOT TRUSTED, IT IS CHECKED.
 *
 * What comes out of IndexedDB is whatever was in IndexedDB — an older shape, a
 * partial write, something another tab wrote. A record that does not have this
 * shape is a CORRUPT ENTRY: treated as absent, dropped, and refetched on the
 * next pick. Self-healing, and said out loud rather than swallowed.
 */
function soundFace(value: unknown): StoredFace | undefined {
  if (!value || typeof value !== 'object') return undefined
  const candidate = value as Record<string, unknown>
  // ⚠ TWO CLASSES OF STRING, AND THE SPLIT IS THE DISK-IMPORT STORY'S ONE
  // CHANGE TO THIS MODULE.
  //
  // THE IDENTITY FIELDS MUST BE PRESENT AND NON-EMPTY. A record with no `key`
  // cannot be addressed, one with no `family`/`style` cannot be resolved to a
  // cut, one with no `mediaType` cannot be embedded, and one with no `source`
  // or `fetchedAt` has lost the provenance the store exists to carry beside the
  // bytes. An empty one of those is a corrupt entry and goes.
  //
  // THE LICENCE FIELDS MAY BE EMPTY, AND EMPTY IS A REAL VALUE RATHER THAN A
  // MISSING ONE. They used to be held to the same non-empty rule, on the
  // ground that `embedFontFamily` refuses without all three — true of a face
  // this product DISTRIBUTES, and false of one the AUTHOR supplies. A brand
  // typeface off an author's own disk can legally declare no nameID 0, 13 or
  // 14 at all; its terms are the author's responsibility and the designer
  // transcribes what the binary says, which is sometimes nothing. Under the old
  // rule such a face was written to the store and then read back as CORRUPT and
  // dropped — a face that vanished between one listing and the next, with no
  // error anybody could see. The field must still be a STRING: absent is still
  // an unreadable record, because absence means a record shape this build does
  // not understand, while `''` means the binary said nothing.
  const identity = ['key', 'family', 'style', 'source', 'mediaType', 'fetchedAt'] as const
  for (const field of identity) if (typeof candidate[field] !== 'string' || candidate[field] === '') return undefined
  const transcribed = ['licence', 'licenceText', 'copyright'] as const
  for (const field of transcribed) if (typeof candidate[field] !== 'string') return undefined
  // ⚠ AND A THIRD CLASS: A FIELD THAT MAY BE ABSENT ALTOGETHER (story 6, D4).
  //
  // `authorAcknowledged` joins the record in this story, and EVERY RECORD
  // WRITTEN BEFORE IT CARRIES NO SUCH KEY — the store is probed by shape rather
  // than by version and this schema change is additive, so there is no upgrade
  // in which to stamp one. Holding an old record to a strict `typeof … ===
  // 'boolean'` would read every face an author already downloaded as CORRUPT
  // and drop it, which is precisely the defect story 3 found in the licence
  // trio and fixed. Absent means the author asserted nothing, which is the
  // truth about a record written before the assertion existed and the safe
  // direction besides: the engine's licence guard stays in force for it.
  //
  // ANYTHING THAT IS NOT `true` IS `false`. A record carrying a string
  // `"true"`, a number or a null is not one this build wrote, and reading a
  // non-boolean as an acknowledgement would let a value nobody asserted
  // override a guard.
  const acknowledged = candidate.authorAcknowledged === true
  if (!storedKeyShape.test(candidate.key as string)) return undefined
  if (!Array.isArray(candidate.scripts) || !candidate.scripts.every((script) => typeof script === 'string')) return undefined
  if (typeof candidate.byteLength !== 'number' || !Number.isSafeInteger(candidate.byteLength) || candidate.byteLength <= 0) return undefined
  return {
    key: candidate.key as string,
    family: candidate.family as string,
    style: candidate.style as string,
    licence: candidate.licence as string,
    licenceText: candidate.licenceText as string,
    copyright: candidate.copyright as string,
    source: candidate.source as string,
    authorAcknowledged: acknowledged,
    mediaType: candidate.mediaType as string,
    scripts: [...(candidate.scripts as string[])],
    fetchedAt: candidate.fetchedAt as string,
    byteLength: candidate.byteLength as number,
  }
}

/**
 * AND A CENSUS READ BACK IS CHECKED THE SAME WAY A FACE IS, FOR THE SAME REASON.
 *
 * What comes out of IndexedDB is whatever was in IndexedDB. A census that does
 * not have this shape is treated as ABSENT, which reads the family as
 * incomplete and offers it for install again — the self-healing direction. The
 * other direction would be worse in kind: a malformed census admitted as sound
 * could report a family complete that holds nothing, and the family would never
 * be offered again.
 */
function soundCensus(value: unknown): FamilyCensus | undefined {
  if (!value || typeof value !== 'object') return undefined
  const candidate = value as Record<string, unknown>
  for (const field of ['family', 'recordedAt'] as const) if (typeof candidate[field] !== 'string' || candidate[field] === '') return undefined
  if (!Array.isArray(candidate.published) || !candidate.published.every((cut) => typeof cut === 'string' && cut !== '')) return undefined
  if (!Array.isArray(candidate.refused)) return undefined
  const refused: FamilyCutRefusal[] = []
  for (const entry of candidate.refused as unknown[]) {
    if (!entry || typeof entry !== 'object') return undefined
    const cut = entry as Record<string, unknown>
    if (typeof cut.style !== 'string' || cut.style === '') return undefined
    if (typeof cut.reason !== 'string' || cut.reason === '') return undefined
    // A REFUSAL WITH NO RECOGNISED PERMANENCE MAKES THE WHOLE CENSUS UNSOUND,
    // which reads the family as incomplete and offers it for install again —
    // the self-healing direction. Admitting one and assuming `permanent` would
    // settle a cut on a record this build cannot actually read.
    if (cut.permanence !== 'permanent' && cut.permanence !== 'transient') return undefined
    refused.push({ style: cut.style, reason: cut.reason, permanence: cut.permanence })
  }
  return {
    family: candidate.family as string,
    published: [...(candidate.published as string[])],
    refused,
    recordedAt: candidate.recordedAt as string,
  }
}

/** One IndexedDB request, as a promise that rejects with the request's own error rather than an `Event`. */
function request<T>(source: IDBRequest<T>): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    source.onsuccess = () => resolve(source.result)
    source.onerror = () => reject(source.error ?? new Error('the request failed with no stated reason'))
  })
}

/** One transaction, as a promise that settles on `complete`, `error` or `abort` — never on none of them. */
function settled(transaction: IDBTransaction): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(transaction.error ?? new Error('the transaction failed with no stated reason'))
    transaction.onabort = () => reject(transaction.error ?? new Error('the transaction was aborted'))
  })
}

/**
 * THE ONE OPEN PATH, AND IT DEGRADES RATHER THAN THROWING INTO CALLERS.
 *
 * A private window, cleared site data, a browser with storage blocked and a
 * test environment with no `indexedDB` at all are all the same shape to the
 * caller: no store, a stated reason, and a designer that still works with an
 * empty group. THE MESSAGE IS SHOWN ONCE, not per pick — a caller that reports
 * this on every pick has turned a standing condition into a stream of noise the
 * author cannot act on.
 *
 * `onblocked` is answered rather than left to hang: another tab holding an
 * older version open is a real, ordinary condition, and an unanswered `open`
 * would leave the caller waiting forever with no message — the exact shape this
 * story's timeout work exists to refuse one layer up.
 */
export async function openFontStore(factory: IDBFactory | undefined = globalThis.indexedDB): Promise<StoreOutcome<FontStore>> {
  if (!factory || typeof factory.open !== 'function') {
    return failed('This browser is not letting the designer keep typefaces on this machine, so the fonts you have already downloaded cannot be offered back to you. Everything else works, and picking a family still fetches it.')
  }
  /**
   * Opened at an explicit version, or at whatever version already exists when
   * `version` is omitted. The upgrade handler is the same either way, because
   * there is only one way this module creates a store.
   */
  const openAt = (version?: number): Promise<IDBDatabase> => new Promise<IDBDatabase>((resolve, reject) => {
    const opening = version === undefined ? factory.open(databaseName) : factory.open(databaseName, version)
    opening.onupgradeneeded = () => createMissingStores(opening.result)
    opening.onsuccess = () => resolve(opening.result)
    opening.onerror = () => reject(opening.error ?? new Error('the database could not be opened'))
    opening.onblocked = () => reject(new Error('another tab of this designer is holding an older version of the store open'))
  })

  let database: IDBDatabase
  try {
    // ⚠ THE SHAPE IS VERIFIED, NOT INFERRED FROM THE VERSION NUMBER.
    //
    // Opening at a fixed `databaseVersion` treats version equality as proof
    // that the schema matches, and it is not: a database can reach the current
    // version WITHOUT one of its stores — an intermediate build that bumped the
    // version before a `createObjectStore` line existed is enough, and a long
    // dev session with hot reload is exactly how one is reached. For such a
    // database `onupgradeneeded` never fires, so nothing ever repaired it and
    // every read and write threw `NotFoundError` for ever.
    //
    // So: open at whatever exists, ASK WHICH STORES ARE ACTUALLY THERE, and if
    // any are missing reopen one version higher so the additive create runs.
    // A fresh database arrives here at version 1 holding nothing and takes the
    // same repair path, which is why there is no separate create branch.
    database = await openAt()
    if (missingStores(database).length > 0) {
      const repairedVersion = Math.max(databaseVersion, database.version + 1)
      database.close()
      database = await openAt(repairedVersion)
    }
  } catch (error) {
    return failed(`This browser is not letting the designer keep typefaces on this machine (${detail(error)}), so the fonts you have already downloaded cannot be offered back to you. Everything else works, and picking a family still fetches it.`)
  }
  // AND IF THE REPAIR DID NOT TAKE, SAY SO IN WORDS THE AUTHOR CAN ACT ON.
  // `NotFoundError: One of the specified object stores was not found` is an
  // internal; it tells the author nothing and suggests nothing. What matters to
  // them is that this store holds only re-fetchable COPIES, so resetting it
  // costs them nothing they cannot get back.
  const unrepaired = missingStores(database)
  if (unrepaired.length > 0) {
    database.close()
    return failed(`The designer's store of downloaded typefaces is in a shape this build could not repair (it is missing ${unrepaired.join(' and ')}). Clear this site's data in your browser to reset it. Your documents and the faces already embedded in them are untouched — this store holds only copies of faces the designer can fetch again.`)
  }
  database.onversionchange = () => database.close()
  return succeeded(fontStoreOver(database))
}

function fontStoreOver(database: IDBDatabase): FontStore {
  /**
   * ⚠ `work` MUST ISSUE EVERY REQUEST SYNCHRONOUSLY, BEFORE IT AWAITS ANYTHING.
   *
   * This is the one non-obvious rule of IndexedDB and it is silent when broken.
   * A transaction commits as soon as its last outstanding request settles and
   * control returns to the event loop, so a second request placed AFTER an
   * `await` lands on a transaction that has already gone inactive — which, on
   * the two-store writes below, means the metadata is written and the bytes are
   * not. That does not throw here; it produces exactly the half-written pair
   * `get()` later treats as a corrupt entry, so the store would appear to work
   * and forget everything.
   *
   * Every caller therefore places its requests first and combines their
   * promises afterwards.
   */
  const transact = <T>(mode: IDBTransactionMode, work: (transaction: IDBTransaction) => Promise<T>): Promise<StoreOutcome<T>> =>
    (async () => {
      try {
        // ⚠ THE STORE LIST IS WHERE A TRANSACTION'S REACH IS DECIDED, and the
        // census is named here rather than in a second `transact` of its own: a
        // transaction that does not name a store cannot touch it, and a census
        // write placed in a transaction opened over the two face stores would
        // throw `NotFoundError` rather than fail quietly.
        const transaction = database.transaction([...objectStores], mode)
        const done = settled(transaction)
        // ⚠ `done` IS OBSERVED THE MOMENT IT EXISTS, NOT ONLY ON THE PATH THAT
        // AWAITS IT.
        //
        // `work` can throw — a request rejecting is the ordinary quota case —
        // and when it does, control leaves for the `catch` below and `done` is
        // NEVER AWAITED. The transaction then aborts, `done` rejects, and a
        // rejected promise with no handler is an unhandled rejection: noise in
        // a console the author reads, and a process-level failure in some test
        // runners, over a condition this module has already handled correctly
        // and reported as an outcome.
        //
        // A no-op handler attached HERE, at creation, settles that for every
        // path at once. It does not swallow anything: `await done` below still
        // sees the same rejection and still turns it into a failed outcome.
        void done.catch(() => undefined)
        const value = await work(transaction)
        await done
        return succeeded(value)
      } catch (error) {
        return failed(detail(error))
      }
    })()

  /**
   * DROPPING A CORRUPT ENTRY IS ITSELF ALLOWED TO FAIL, AND SILENTLY.
   *
   * The read that discovered the corruption has already decided the right
   * answer — the entry is absent — and a failure to tidy up must not turn that
   * answer into an error. The next read finds it again and tries again.
   */
  const drop = (key: string, why: string): void => {
    console.info(`The typeface this designer had stored under ${key} ${why}, so it has been dropped from this machine's store. Picking that family again will fetch it.`)
    void transact('readwrite', (transaction) => {
      const face = request(transaction.objectStore(faceStoreName).delete(key))
      const bytes = request(transaction.objectStore(byteStoreName).delete(key))
      return Promise.all([face, bytes]).then(() => undefined)
    })
  }

  return {
    async get(key) {
      const read = await transact('readonly', (transaction) => {
        const face = request<unknown>(transaction.objectStore(faceStoreName).get(key))
        const bytes = request<unknown>(transaction.objectStore(byteStoreName).get(key))
        return Promise.all([face, bytes]).then(([held, stored]) => ({ face: held, bytes: stored }))
      })
      if (!read.ok) return read
      const face = soundFace(read.value.face)
      if (face === undefined) {
        // An entry the metadata half has no sound record for. If NOTHING is
        // there at all it is an ordinary miss and nothing is dropped; if
        // something is there but is not a record this build understands, it is
        // corrupt and goes.
        if (read.value.face !== undefined || read.value.bytes !== undefined) drop(key, 'is not a record this version of the designer can read')
        return succeeded(undefined)
      }
      const bytes = asArrayBuffer(read.value.bytes)
      if (bytes === undefined || bytes.byteLength !== face.byteLength) {
        drop(key, 'has lost its bytes, or holds a different number of them than its record claims')
        return succeeded(undefined)
      }
      // THE CONTENT ADDRESS IS VERIFIED ON THE READ THAT FEEDS AN EMBED, not
      // merely trusted. The key IS the claim "these are those bytes", and this
      // is the only place it can be checked. Bytes that no longer hash to their
      // own key are not a slightly-wrong entry; they are a face nobody chose,
      // one step from a document.
      if (await storedFaceKey(bytes) !== face.key) {
        drop(key, 'no longer matches the content address it was stored under')
        return succeeded(undefined)
      }
      return succeeded({ ...face, bytes })
    },

    /**
     * `source` IS WRITTEN AND READ BACK BYTE-IDENTICALLY. The store is a
     * CARRIER, and a carrier that normalises, truncates or re-derives a
     * provenance record has become an authority on a document — the one thing
     * this store is never allowed to be. `src/font-store.test.ts` asserts the
     * retrieved `source` through the shared `assertProvenanceShape` tripwire,
     * at the RETRIEVAL side, so the store is held to the same contract the two
     * writers are.
     */
    put(record) {
      return transact('readwrite', (transaction) => {
        const { bytes, ...face } = record
        const written = request(transaction.objectStore(faceStoreName).put({ ...face, scripts: [...face.scripts] }))
        const kept = request(transaction.objectStore(byteStoreName).put(bytes, record.key))
        return Promise.all([written, kept]).then(() => undefined)
      })
    },

    async list() {
      const read = await transact('readonly', (transaction) => request<unknown[]>(transaction.objectStore(faceStoreName).getAll()))
      if (!read.ok) return read
      const sound: StoredFace[] = []
      for (const candidate of read.value) {
        const face = soundFace(candidate)
        if (face === undefined) {
          const key = candidate && typeof candidate === 'object' && typeof (candidate as Record<string, unknown>).key === 'string' ? (candidate as Record<string, unknown>).key as string : undefined
          if (key !== undefined && storedKeyShape.test(key)) drop(key, 'is not a record this version of the designer can read')
          continue
        }
        sound.push(face)
      }
      // ONE STABLE ORDER, BY FAMILY THEN KEY. `getAll` returns key order, which
      // is the hash order — that is, arbitrary — and a menu whose rows move
      // between reads for no reason the author can see is a menu nobody trusts.
      // The key breaks the tie so two different faces of one family have an
      // order at all.
      return succeeded([...sound].sort((left, right) => left.family.localeCompare(right.family) || left.key.localeCompare(right.key)))
    },

    remove(key) {
      return transact('readwrite', (transaction) => {
        const face = request(transaction.objectStore(faceStoreName).delete(key))
        const bytes = request(transaction.objectStore(byteStoreName).delete(key))
        return Promise.all([face, bytes]).then(() => undefined)
      })
    },

    /**
     * A CENSUS THIS BUILD CANNOT READ IS SKIPPED, NOT DROPPED.
     *
     * The face store drops a corrupt entry because the bytes behind it are
     * unusable and the next pick should refetch them. A census carries no bytes
     * and costs nothing to leave in place; skipping it reads the family as
     * incomplete, which offers it for install again, and the next install
     * overwrites the row with a sound one. Deleting would be the same outcome
     * with an extra write.
     */
    async listCensus() {
      const read = await transact('readonly', (transaction) => request<unknown[]>(transaction.objectStore(censusStoreName).getAll()))
      if (!read.ok) return read
      const sound: FamilyCensus[] = []
      for (const candidate of read.value) {
        const census = soundCensus(candidate)
        if (census !== undefined) sound.push(census)
      }
      return succeeded(sound)
    },

    putCensus(record) {
      return transact('readwrite', (transaction) =>
        request(transaction.objectStore(censusStoreName).put({
          family: record.family,
          published: [...record.published],
          refused: record.refused.map((entry) => ({ style: entry.style, reason: entry.reason, permanence: entry.permanence })),
          recordedAt: record.recordedAt,
        })).then(() => undefined))
    },
  }
}

/**
 * THE SENTENCE THE DESIGNER SAYS WHEN THE STORE WILL NOT TAKE A FACE — install's
 * own refusal, when the store write IS the whole act.
 *
 * STORY 16.5 INVERTED STORY 16.2's RULING. 16.2 put the store write AFTER the
 * embed and said, correctly, that a quota refusal was *"not a failed pick: the
 * face was fetched, the terms were admitted and the document has it"* — a
 * degradation with something left to degrade FROM.
 *
 * UNDER EMBED-ON-USE THERE IS NOTHING LEFT TO DEGRADE FROM. Installing IS the
 * store write; no command follows it and no document has the face. So a refusal
 * here now decides whether the author gets the font at all, and calling that a
 * caching failure would report a failed install as a success with a footnote.
 * It is stated as the refusal it is, and it says what did NOT happen so the
 * author is not left wondering which of their two files moved.
 *
 * IT OFFERS NO REMEDY (Story 16.6). It used to point at the machine store
 * panel's per-face remove control as a way to free space; that panel is gone
 * and there is no other way to free space on this machine, so the sentence
 * says only what failed and what did not happen.
 */
export const storeWriteRefusal = (family: string, reason: string): string =>
  `${family} was not installed on this machine (${reason}). Nothing was kept and no document was changed.`
