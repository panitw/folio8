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

/** The database and its two object stores. Version 1; there is no earlier shape to migrate. */
const databaseName = 'folio8-machine-font-store'
const databaseVersion = 1
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
  const strings = ['key', 'family', 'style', 'licence', 'licenceText', 'copyright', 'source', 'mediaType', 'fetchedAt'] as const
  for (const field of strings) if (typeof candidate[field] !== 'string' || candidate[field] === '') return undefined
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
    mediaType: candidate.mediaType as string,
    scripts: [...(candidate.scripts as string[])],
    fetchedAt: candidate.fetchedAt as string,
    byteLength: candidate.byteLength as number,
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
  let database: IDBDatabase
  try {
    database = await new Promise<IDBDatabase>((resolve, reject) => {
      const opening = factory.open(databaseName, databaseVersion)
      opening.onupgradeneeded = () => {
        const upgrading = opening.result
        if (!upgrading.objectStoreNames.contains(faceStoreName)) upgrading.createObjectStore(faceStoreName, { keyPath: 'key' })
        if (!upgrading.objectStoreNames.contains(byteStoreName)) upgrading.createObjectStore(byteStoreName)
      }
      opening.onsuccess = () => resolve(opening.result)
      opening.onerror = () => reject(opening.error ?? new Error('the database could not be opened'))
      opening.onblocked = () => reject(new Error('another tab of this designer is holding an older version of the store open'))
    })
  } catch (error) {
    return failed(`This browser is not letting the designer keep typefaces on this machine (${detail(error)}), so the fonts you have already downloaded cannot be offered back to you. Everything else works, and picking a family still fetches it.`)
  }
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
        const transaction = database.transaction([faceStoreName, byteStoreName], mode)
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
