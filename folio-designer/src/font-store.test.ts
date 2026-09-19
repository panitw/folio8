import fs from 'node:fs'
import path from 'node:path'
import { createHash } from 'node:crypto'
import { fileURLToPath } from 'node:url'
import { describe, expect, it, vi } from 'vitest'
import { IDBFactory as FakeIndexedDBFactory, IDBObjectStore as FakeIndexedDBObjectStore } from 'fake-indexeddb'
import { openFontStore, storedFaceKey, storeWriteRefusal, type FontStore, type StoredFaceRecord } from './font-store'
import { isCarriedFaceAssetKey } from './embedded-face-family'
import { sfntWithCopyright } from './test/sfnt-fixture'
import { assertProvenanceShape } from './test/provenance-shape'

// STORY 16.2 — THE MACHINE FONT STORE.
//
// WHY THERE IS A DEPENDENCY UNDER THESE TESTS, AND WHY IT IS NOT A FAKE THIS
// REPOSITORY WROTE (D-16.R.42, Q2).
//
// jsdom 28.1.0 ships NO IndexedDB at all — measured at the build gate:
// `'indexedDB' in window` is `false` and `globalThis.indexedDB` is `undefined`
// — so without a backing store these tests would have nothing to run against.
// A fake written here would be shaped by its author's reading of the
// request/transaction plumbing, so it would agree with `font-store.ts` BY
// CONSTRUCTION and prove that plumbing not at all: both sides moving together
// is the exact vacuity this run exists to refuse. An independent
// implementation CAN disagree with the code, and that capacity is the whole
// value of the test.
//
// `fake-indexeddb` is a devDependency, Apache-2.0 — precedented in this
// designer by `pdfjs-dist` — with no transitive dependencies of its own, and
// the last describe block below asserts that no shipping module imports it.
//
// AND THE RESIDUAL IS NAMED RATHER THAN HIDDEN. Real browser IndexedDB is
// proven only by a browser. That witness is routed to Story 16.3's browser run
// as a fourth case beside DW-161's three: a stored face survives a reload and
// is offered with the network disabled. Nothing here claims to have run in a
// browser.

const here = path.dirname(fileURLToPath(import.meta.url))

/** A fresh, isolated database per test. Two tests sharing an origin would share a store. */
const freshStore = async (): Promise<FontStore> => {
  const opened = await openFontStore(new FakeIndexedDBFactory())
  if (!opened.ok) throw new Error(`the fixture store could not be opened: ${opened.reason}`)
  return opened.value
}

const face = sfntWithCopyright('Copyright 2026 The Folio Authors')

const record = async (overrides: Partial<StoredFaceRecord> = {}): Promise<StoredFaceRecord> => ({
  key: await storedFaceKey(face),
  family: 'Kanit',
  style: 'Regular',
  licence: 'OFL-1.1',
  licenceText: 'This Font Software is licensed under the SIL Open Font License, Version 1.1.',
  copyright: 'Copyright 2026 The Folio Authors',
  source: 'google/fonts — ofl/kanit/Kanit-Regular.ttf, fetched 2026-09-03',
  mediaType: 'font/ttf',
  scripts: ['latin', 'thai'],
  fetchedAt: '2026-09-03',
  byteLength: face.byteLength,
  bytes: face,
  ...overrides,
})

describe('the content address the store keys by', () => {
  // THE TWO ADDRESSINGS MUST NOT DRIFT. Go derives an embedded face's asset key
  // as `fmt.Sprintf("%x", sha256.Sum256(decoded))` — `component_commands.go`'s
  // `embedFontFamily`. The store derives its own key in the browser with
  // `crypto.subtle`. If those ever disagreed, a store "hit" would be a hit on
  // bytes the document does not hold.
  //
  // ⚠ THIS CONSTANT HAS A SECOND HOME, AND THAT IS WHAT MAKES IT A TIE.
  //
  // The Go half is `folio-go/stored_face_key_tie_test.go` — `TestStoredFaceKeyTie`,
  // whose constant is named `storedFaceKeyTieDigest`. It rebuilds the same
  // 110-byte fixture in Go from the same written-down sfnt layout, derives the
  // digest with `crypto/sha256`, and asserts THE SAME 64 characters written
  // below. That file names this one; this one names that one, so a reader who
  // finds either finds the other.
  //
  // STATED PLAINLY, THE TIE IS: TWO SUITES PINNED TO ONE SHARED CONSTANT. It is
  // not a generated fixture and not a cross-process check — it is one digest and
  // one input, written as literals on both sides of the language boundary, each
  // side deriving the digest by its own means (`crypto.subtle` here,
  // `crypto/sha256` there).
  //
  // IT WAS NOT ALWAYS ONE. Until Story 16.2's review these characters appeared
  // in exactly ONE place in the repository — this file — so a change to Go's
  // derivation (`fmt.Sprintf("%x", sha256.Sum256(decoded))` in
  // `component_commands.go`'s `embedFontFamily`) reddened Go's own tests and
  // left this file's claim ABOUT Go standing and green. That is a transcribed
  // literal, not a tie. Now a change on either side has to face the same
  // constant from both.
  //
  // The value itself was produced by Go, over exactly the bytes
  // `sfntWithCopyright` builds:
  //
  //   $ go run . fixture.bin
  //   71a8f6e9b586701b742afb6a357afc3f01e4a817ec26fe43a83365f6611a847f 110
  //
  // where the program is the two lines that matter — `sha256.Sum256(data)`
  // printed with `%x` — and `fixture.bin` is this fixture's 110 bytes written
  // out unchanged. Recorded as a literal on purpose: a value recomputed here by
  // the same method it is checking would be checking nothing. NEITHER COPY MAY
  // BE ADJUSTED to make the other pass; a disagreement means one of the two
  // addressings moved, and the job is to find out which.
  const goDerivedAssetKey = '71a8f6e9b586701b742afb6a357afc3f01e4a817ec26fe43a83365f6611a847f'

  it('computes the same key Go derives for the same bytes', async () => {
    // THE BYTE-LENGTH GUARD, which `folio-go/stored_face_key_tie_test.go`
    // carries too: a digest over unknown bytes says nothing, so if the fixture
    // moves this reds first and both halves must be re-derived over the new
    // bytes.
    expect(face.byteLength, 'the fixture moved; the Go-derived digest below must be re-derived on BOTH sides (see folio-go/stored_face_key_tie_test.go), not adjusted').toBe(110)
    expect(await storedFaceKey(face)).toBe(goDerivedAssetKey)
  })

  // AND A SECOND, INDEPENDENT SHA-256 AGREES. `crypto.subtle` and Node's
  // `createHash` are different implementations; a mistake in the hex encoding
  // — the one part of this that is ours — shows up here.
  it('agrees with an independent SHA-256 implementation, including its hex encoding', async () => {
    expect(await storedFaceKey(face)).toBe(createHash('sha256').update(Buffer.from(face)).digest('hex'))
  })

  // THE KEY MUST SATISFY THE DERIVATION MODULE'S OWN PREDICATE, because a
  // stored face is registered for preview under the family that module derives
  // from the key, and that derivation is written into an inline `font-family`
  // declaration.
  it('produces a key the carried-face derivation admits', async () => {
    expect(isCarriedFaceAssetKey(await storedFaceKey(face))).toBe(true)
  })
})

describe('the store round trip', () => {
  it('reads back everything the embed command requires, byte for byte', async () => {
    const store = await freshStore()
    const written = await record()
    expect(await store.put(written)).toEqual({ ok: true, value: undefined })
    const read = await store.get(written.key)
    expect(read.ok).toBe(true)
    if (!read.ok || read.value === undefined) throw new Error('the round trip lost the record')
    // THE THREE FIELDS THE ENGINE REFUSES A DOCUMENT WITHOUT, plus everything
    // else `embedFontFamilyCommand` names. A store that kept the bytes and
    // dropped the terms would put a document its own parser rejects one step
    // away — which is why they travel together rather than being refetchable.
    expect(read.value.licence).toBe('OFL-1.1')
    expect(read.value.licenceText).toBe(written.licenceText)
    expect(read.value.copyright).toBe(written.copyright)
    expect(read.value.family).toBe('Kanit')
    expect(read.value.style).toBe('Regular')
    expect(read.value.mediaType).toBe('font/ttf')
    expect(read.value.scripts).toEqual(['latin', 'thai'])
    expect(read.value.fetchedAt).toBe('2026-09-03')
    expect(new Uint8Array(read.value.bytes)).toEqual(new Uint8Array(face))
  })

  // THE STORE IS A CARRIER, NEVER AN AUTHORITY (contract Boundary). A carrier
  // that normalises, truncates or re-derives a provenance record has become an
  // authority on a document, and `source` is the field where that would happen
  // first. Asserted at the RETRIEVAL side, through the SHARED tripwire the two
  // writers are held to — which also puts a THIRD call site on that helper,
  // whose two existing callers both feed it known-good values (D-16.R.34 F1).
  it('carries `source` back byte-identically, and the retrieved value still passes the provenance shape', async () => {
    const store = await freshStore()
    const written = await record()
    await store.put(written)
    const read = await store.get(written.key)
    if (!read.ok || read.value === undefined) throw new Error('the round trip lost the record')
    expect(read.value.source).toBe(written.source)
    assertProvenanceShape(expect, 'the machine store, on retrieval', `${read.value.family} ${read.value.style}`, read.value.source)
  })

  // AND THE TRIPWIRE IS NOT DECORATIVE HERE EITHER: a store that DID rewrite
  // `source` into a retrieval path is caught by the same call. This is the
  // negative control the helper's existing call sites do not have.
  it('would fail the provenance shape if a retrieval path were carried instead of a provenance record', () => {
    expect(() => assertProvenanceShape(expect, 'the machine store, on retrieval', 'a rewritten record', 'https://a.host.example/google/fonts/main/ofl/kanit/Kanit-Regular.ttf')).toThrow()
  })

  it('lists exactly the faces that were stored, and nothing else', async () => {
    const store = await freshStore()
    const kanit = await record()
    const other = await record({ key: await storedFaceKey(sfntWithCopyright('Copyright 2026 Another Project')), family: 'Alegreya', bytes: sfntWithCopyright('Copyright 2026 Another Project'), byteLength: sfntWithCopyright('Copyright 2026 Another Project').byteLength })
    await store.put(kanit)
    await store.put(other)
    const listed = await store.list()
    if (!listed.ok) throw new Error(listed.reason)
    expect(listed.value.map((entry) => entry.family)).toEqual(['Alegreya', 'Kanit'])
    // THE LISTING CARRIES NO BYTES, which is what lets a menu be rendered
    // without deserializing every face in the store.
    expect(Object.hasOwn(listed.value[0], 'bytes')).toBe(false)
    expect(listed.value[0].byteLength).toBeGreaterThan(0)
  })

  it('removes both halves of an entry, so a removed face is gone rather than half-gone', async () => {
    const store = await freshStore()
    const written = await record()
    await store.put(written)
    expect(await store.remove(written.key)).toEqual({ ok: true, value: undefined })
    const read = await store.get(written.key)
    expect(read).toEqual({ ok: true, value: undefined })
    const listed = await store.list()
    if (!listed.ok) throw new Error(listed.reason)
    expect(listed.value).toEqual([])
  })

  it('reports an ordinary miss as absence rather than as a failure', async () => {
    const store = await freshStore()
    expect(await store.get('0'.repeat(64))).toEqual({ ok: true, value: undefined })
  })
})

describe('storage that cannot be opened or written', () => {
  // A PRIVATE WINDOW, CLEARED SITE DATA, STORAGE BLOCKED. The designer keeps
  // working and says what is degraded; nothing throws into the pick path.
  //
  // ⚠ AND THE PRECONDITION IS ASSERTED RATHER THAN ASSUMED, because passing
  // `undefined` does NOT bypass the default — it TRIGGERS it. The signature is
  // `factory: IDBFactory | undefined = globalThis.indexedDB`, so the argument
  // below selects `globalThis.indexedDB`, and this test exercises "the
  // environment has no IndexedDB" only for as long as that is true of this
  // file's environment. It is true today (measured at the build gate: jsdom
  // 28.1.0 provides none), and it is a property of the ENVIRONMENT rather than
  // of anything this test controls.
  //
  // So it is stated. The day a global fake is installed in this file — a
  // `setupFiles` entry, an import with a side effect — this test would
  // otherwise stop testing what its name says while staying green. Now it reds,
  // and it reds with the reason written next to it.
  it('degrades with a stated reason when the environment has no IndexedDB at all', async () => {
    expect(globalThis.indexedDB, 'this test asserts the no-IndexedDB path by letting the default parameter resolve; if a global IndexedDB has been installed in this file, the default no longer selects nothing and this test must be given an explicit absent factory instead').toBeUndefined()
    const opened = await openFontStore(undefined)
    expect(opened.ok).toBe(false)
    if (opened.ok) return
    expect(opened.reason).toMatch(/not letting the designer keep typefaces on this machine/)
    // AND IT SAYS WHAT STILL WORKS. A degradation the author cannot act on is
    // an error message wearing a friendlier coat.
    expect(opened.reason).toMatch(/picking a family still fetches it/i)
  })

  it('degrades rather than throwing when opening the database fails', async () => {
    const factory = { open: () => { throw new Error('the origin is not allowed to use storage') } } as unknown as IDBFactory
    const opened = await openFontStore(factory)
    expect(opened.ok).toBe(false)
    if (!opened.ok) expect(opened.reason).toContain('the origin is not allowed to use storage')
  })

  // A WRITE THAT FAILS — the quota row of the matrix. The store returns an
  // outcome; it is the CALLER that decides the fetch and the embed still
  // succeeded, and the sentence it says about it names the family and states
  // that the caching is what failed.
  it('returns a failed outcome from a write the origin refuses, and never rejects', async () => {
    const store = await freshStore()
    // THE REFUSAL IS INJECTED INTO THE REAL PLUMBING RATHER THAN SIMULATED BY A
    // STAND-IN STORE, and at the exact place a browser raises it: the `put`
    // itself, as a `QuotaExceededError`. Patching the backing implementation
    // leaves every other layer — the transaction, the request, this module's
    // own error path — genuinely exercised, which a hand-written "store that
    // always fails" would not.
    const original = FakeIndexedDBObjectStore.prototype.put
    FakeIndexedDBObjectStore.prototype.put = function refuse(): never { throw new DOMException('the origin has no room left for this face', 'QuotaExceededError') }
    try {
      const written = await store.put(await record())
      expect(written.ok, 'a write the origin refuses must come back as an outcome, not as a rejection').toBe(false)
      if (!written.ok) expect(written.reason).toContain('no room left')
    } finally {
      FakeIndexedDBObjectStore.prototype.put = original
    }
  })

  // BEHAVIOUR-CHANGED (Story 16.5). This asserted the sentence claimed the family
  // "was added to this document", which was TRUE under Story 16.2 — the store
  // write came after the embed, so a quota refusal was a caching failure with a
  // succeeded pick behind it. Under embed-on-use the write IS the install and
  // nothing follows it, so the same failure now means the author got nothing.
  // The old claim is asserted ABSENT rather than merely replaced, because the
  // one way this can regress is a sentence that keeps saying a document moved.
  //
  // RETIRED, STORY 16.6: the cross-check against `App.tsx`'s panel heading and
  // the "remove control on every face" assertion. The panel and its remove
  // control are gone by owner decision (D-16.R.82); `storeWriteRefusal` no
  // longer names a region to visit or a control to press, so there is nothing
  // left for those two assertions to check. The remaining assertions — that the
  // sentence names the family and the reason, and says a refusal rather than a
  // caching hiccup — still hold and stay.
  it('states a refused install as a refused install, naming the family and what did not happen', () => {
    const stated = storeWriteRefusal('Kanit', 'the origin is out of space')
    expect(stated).toContain('Kanit')
    expect(stated).toContain('was not installed on this machine')
    expect(stated).toContain('the origin is out of space')
    expect(stated, 'nothing may claim a document changed: no command is sent at install').not.toContain('added to this document')
    expect(stated).toMatch(/no document was changed/)
    // AND IT OFFERS NO REMEDY (Story 16.6). With no removal control left
    // anywhere in the designer, a sentence that still pointed at one would be
    // pointing at nothing — this is the deletion's own guard on the sentence.
    expect(stated, 'no remedy remains to offer: the removal control this once pointed at is deleted').not.toMatch(/remove/i)
  })
})

describe('an entry that has gone bad', () => {
  /** Reaches past the store to write whatever a browser, another tab or an older build might have left. */
  const writeRaw = async (factory: IDBFactory, key: string, faceRecord: unknown, bytes: unknown): Promise<void> => {
    const database = await new Promise<IDBDatabase>((resolve, reject) => {
      const opening = factory.open('folio8-machine-font-store', 1)
      opening.onsuccess = () => resolve(opening.result)
      opening.onerror = () => reject(opening.error)
    })
    await new Promise<void>((resolve, reject) => {
      const transaction = database.transaction(['faces', 'face-bytes'], 'readwrite')
      if (faceRecord !== undefined) transaction.objectStore('faces').put(faceRecord)
      if (bytes !== undefined) transaction.objectStore('face-bytes').put(bytes, key)
      transaction.oncomplete = () => resolve()
      transaction.onerror = () => reject(transaction.error)
    })
    database.close()
  }

  it('treats a record this build cannot read as absent, drops it, and says so', async () => {
    const factory = new FakeIndexedDBFactory()
    const opened = await openFontStore(factory)
    if (!opened.ok) throw new Error(opened.reason)
    const key = await storedFaceKey(face)
    const info = vi.spyOn(console, 'info').mockImplementation(() => undefined)
    try {
      // A record from a shape that no longer exists: no licence text at all,
      // which is precisely the field a document may not carry a face without.
      await writeRaw(factory, key, { key, family: 'Kanit', style: 'Regular', licence: 'OFL-1.1', copyright: 'c', source: 's', mediaType: 'font/ttf', scripts: [], fetchedAt: '2026-09-03', byteLength: face.byteLength }, face)
      expect(await opened.value.get(key)).toEqual({ ok: true, value: undefined })
      expect(info.mock.calls.map((call) => String(call[0])).join('\n')).toContain('has been dropped from this machine')
      // SELF-HEALING, NOT MERELY IGNORED: it is gone, so the next pick fetches
      // rather than finding the same bad entry again forever.
      await vi.waitFor(async () => {
        const listed = await opened.value.list()
        if (!listed.ok) throw new Error(listed.reason)
        expect(listed.value).toEqual([])
      })
    } finally {
      info.mockRestore()
    }
  })

  // THE CONTENT ADDRESS IS THE CLAIM, AND IT IS CHECKED ON THE READ THAT FEEDS
  // AN EMBED. Bytes that no longer hash to their own key are not a slightly
  // wrong entry; they are a face nobody chose, one step from a document.
  it('drops an entry whose bytes no longer hash to the key they are stored under', async () => {
    const factory = new FakeIndexedDBFactory()
    const opened = await openFontStore(factory)
    if (!opened.ok) throw new Error(opened.reason)
    const written = await record()
    await opened.value.put(written)
    const info = vi.spyOn(console, 'info').mockImplementation(() => undefined)
    try {
      const different = sfntWithCopyright('Copyright 2026 Somebody Else Entirely')
      // Same length, different bytes — so only the hash can tell, which is the
      // point of checking the hash rather than the length.
      const padded = new Uint8Array(written.byteLength)
      padded.set(new Uint8Array(different).subarray(0, written.byteLength))
      await writeRaw(factory, written.key, undefined, padded.buffer)
      expect(await opened.value.get(written.key)).toEqual({ ok: true, value: undefined })
      expect(info.mock.calls.map((call) => String(call[0])).join('\n')).toContain('no longer matches the content address')
    } finally {
      info.mockRestore()
    }
  })

  it('treats an entry whose bytes are gone as absent rather than embedding a record with nothing behind it', async () => {
    const factory = new FakeIndexedDBFactory()
    const opened = await openFontStore(factory)
    if (!opened.ok) throw new Error(opened.reason)
    const written = await record()
    const info = vi.spyOn(console, 'info').mockImplementation(() => undefined)
    try {
      await writeRaw(factory, written.key, { ...written, bytes: undefined, scripts: [...written.scripts] }, undefined)
      expect(await opened.value.get(written.key)).toEqual({ ok: true, value: undefined })
      expect(info.mock.calls.map((call) => String(call[0])).join('\n')).toContain('has lost its bytes')
    } finally {
      info.mockRestore()
    }
  })
})

// THE STORE IS RUNTIME DATA, NOT A RELEASE ASSET, AND THE OFFLINE RELEASE
// CONTRACT DOES NOT MOVE (contract Block If).
//
// A face this store keeps lives in the browser's own database, written at the
// moment of a pick. It is not precached, not content-addressed into the release
// manifest, and it consumes NO cache slot — so the bound that governs how many
// assets a release may carry is untouched. Asserted rather than assumed,
// because DW-162 halved that bound's margin to 10 of 64 and nothing watches it:
// a store implementation that accidentally landed a face in the release
// manifest would otherwise be caught by a release failing, months later.
describe('the offline release contract is untouched', () => {
  const releasePayload = fs.readFileSync(path.join(here, 'release-payload.ts'), 'utf8')

  it('leaves the cache-asset envelope exactly where it was', () => {
    // The two lines the release contract reader is anchored to, byte for byte.
    // `scripts/offline-release-contract.mjs` requires each as a single live
    // `const <name> = <digits>`, so this asserts the value AND the shape that
    // reader depends on.
    expect(releasePayload).toContain('const minimumCacheAssets = 10\n')
    expect(releasePayload).toContain('const maximumCacheAssets = 90\n')
  })

  it('adds no release asset of its own, because the store is a database and not a bundle', () => {
    const store = fs.readFileSync(path.join(here, 'font-store.ts'), 'utf8')
    for (const releaseShaped of [/runtimeAssetUrls/, /cacheAssets/, /assetCount/, /precache/i, /\bServiceWorker\b/]) {
      expect(store, `the machine store is runtime data; ${String(releaseShaped)} would put it in the release manifest`).not.toMatch(releaseShaped)
    }
  })
})

// CONDITION 1 OF THE FOUR THE DEPENDENCY WAS ADMITTED UNDER (D-16.R.42):
// `devDependencies` ONLY, asserted by a scan rather than by a convention. A
// test-only package drifting into the shipped bundle is the class
// `scripts/forbidden-font-hosts.mjs` exists to catch in another medium, and the
// mechanism is the same: read the real source, name the file and the line.
describe('the test-only backing store never reaches the product', () => {
  const testOnlyPackage = 'fake-indexeddb'

  it('is declared in devDependencies and not in dependencies', () => {
    const manifest = JSON.parse(fs.readFileSync(path.join(here, '..', 'package.json'), 'utf8')) as { dependencies: Record<string, string>; devDependencies: Record<string, string> }
    expect(Object.hasOwn(manifest.devDependencies, testOnlyPackage)).toBe(true)
    expect(Object.hasOwn(manifest.dependencies, testOnlyPackage)).toBe(false)
    // AND THE SHIPPED SET IS STILL THE THREE IT WAS. A dependency added to the
    // wrong half would otherwise only be caught by the negation above, which
    // says nothing about the fourth package somebody adds next.
    expect(Object.keys(manifest.dependencies).sort()).toEqual(['pdfjs-dist', 'react', 'react-dom'])
  })

  // CONDITION 2, THE OTHER HALF: no SHIPPING module imports it. A `.test.ts`
  // file may; nothing else may. The population is asserted first, because a
  // walk that found nothing is indistinguishable from a walk that found no
  // violation.
  it('is imported by no shipping module under src/', () => {
    const shipping: string[] = []
    const walk = (directory: string) => {
      for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
        const full = path.join(directory, entry.name)
        if (entry.isDirectory()) { walk(full); continue }
        if (!/\.(ts|tsx|mts|cts|js|jsx|mjs|cjs)$/.test(entry.name)) continue
        // The test tree is where the package belongs; `src/test/` is test
        // support and is excluded on the same ground.
        if (/\.test\.(ts|tsx|mts|cts|js|jsx|mjs|cjs)$/.test(entry.name)) continue
        if (path.relative(path.join(here), full).split(path.sep)[0] === 'test') continue
        shipping.push(full)
      }
    }
    walk(here)
    expect(shipping.length, 'the walk read almost nothing, which cannot support a claim about the shipping tree').toBeGreaterThan(30)
    const offenders = shipping.filter((file) => fs.readFileSync(file, 'utf8').includes(testOnlyPackage))
    expect(offenders.map((file) => path.relative(here, file)), `${testOnlyPackage} is a devDependency and must never be reachable from a shipping module`).toEqual([])
  })

  // CONDITION 4's INPUT, RECORDED SO IT IS CHECKABLE RATHER THAN REMEMBERED.
  // AD-26 forbids GPL, LGPL, AGPL, SSPL and commercial EULAs at ANY DEPTH. The
  // package adds exactly itself, under Apache-2.0 — the licence `pdfjs-dist`
  // already ships under, so it is precedented rather than novel — and its
  // installed LICENSE file is read here rather than its `package.json` field,
  // because the field is a claim and the file is the grant.
  it('adds exactly one package, Apache-2.0, with no transitive dependencies', () => {
    const installed = path.join(here, '..', 'node_modules', testOnlyPackage)
    const manifest = JSON.parse(fs.readFileSync(path.join(installed, 'package.json'), 'utf8')) as { version: string; license: string; dependencies?: Record<string, string> }
    expect(manifest.version).toBe('6.2.5')
    expect(manifest.license).toBe('Apache-2.0')
    expect(Object.keys(manifest.dependencies ?? {})).toEqual([])
    const licence = fs.readFileSync(path.join(installed, 'LICENSE'), 'utf8')
    expect(licence).toContain('Apache License')
    expect(licence).toContain('Version 2.0, January 2004')
    for (const forbidden of ['GNU GENERAL PUBLIC LICENSE', 'GNU LESSER GENERAL PUBLIC LICENSE', 'GNU AFFERO', 'Server Side Public License']) {
      expect(licence, `AD-26 forbids ${forbidden} at any depth`).not.toContain(forbidden)
    }
  })
})

// THE BUILD-TIME RASTERIZER, ADMITTED UNDER THE SAME POLICY (startup templates,
// story 1). `@napi-rs/canvas` turns page 1 of each bundled example's engine PDF
// into its dialog thumbnail inside `scripts/build-examples.mjs`. It was already
// in the lockfile as `pdfjs-dist`'s optional canvas backend; declaring it pins
// the exact version the build depends on. It runs at build time only and must
// never reach the shipped bundle.
describe('the build-time rasterizer never reaches the product', () => {
  const buildOnlyPackage = '@napi-rs/canvas'

  it('is declared in devDependencies, pinned exactly, and not in dependencies', () => {
    const manifest = JSON.parse(fs.readFileSync(path.join(here, '..', 'package.json'), 'utf8')) as { dependencies: Record<string, string>; devDependencies: Record<string, string> }
    expect(manifest.devDependencies[buildOnlyPackage]).toBe('1.0.8')
    expect(Object.hasOwn(manifest.dependencies, buildOnlyPackage)).toBe(false)
    expect(Object.keys(manifest.dependencies).sort()).toEqual(['pdfjs-dist', 'react', 'react-dom'])
  })

  it('is imported by no shipping module under src/', () => {
    const shipping: string[] = []
    const walk = (directory: string) => {
      for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
        const full = path.join(directory, entry.name)
        if (entry.isDirectory()) { walk(full); continue }
        if (!/\.(ts|tsx|mts|cts|js|jsx|mjs|cjs)$/.test(entry.name)) continue
        if (/\.test\.(ts|tsx|mts|cts|js|jsx|mjs|cjs)$/.test(entry.name)) continue
        if (path.relative(path.join(here), full).split(path.sep)[0] === 'test') continue
        shipping.push(full)
      }
    }
    walk(here)
    expect(shipping.length, 'the walk read almost nothing, which cannot support a claim about the shipping tree').toBeGreaterThan(30)
    const offenders = shipping.filter((file) => fs.readFileSync(file, 'utf8').includes(buildOnlyPackage))
    expect(offenders.map((file) => path.relative(here, file)), `${buildOnlyPackage} is a build-time devDependency and must never be reachable from a shipping module`).toEqual([])
  })

  // AD-26 forbids GPL, LGPL, AGPL, SSPL and commercial EULAs at ANY DEPTH. The
  // package has no runtime dependencies; its optional dependencies are its own
  // per-platform prebuilt binaries at the same version. The installed LICENSE
  // file is read, because the field is a claim and the file is the grant.
  it('is MIT, with only its own same-version platform binaries beneath it', () => {
    const installed = path.join(here, '..', 'node_modules', buildOnlyPackage)
    const manifest = JSON.parse(fs.readFileSync(path.join(installed, 'package.json'), 'utf8')) as { version: string; license: string; dependencies?: Record<string, string>; optionalDependencies?: Record<string, string> }
    expect(manifest.version).toBe('1.0.8')
    expect(manifest.license).toBe('MIT')
    expect(Object.keys(manifest.dependencies ?? {})).toEqual([])
    for (const [name, version] of Object.entries(manifest.optionalDependencies ?? {})) {
      expect(name, 'an optional dependency outside the package family would be a new, unreviewed package').toMatch(/^@napi-rs\/canvas-[a-z0-9-]+$/)
      expect(version).toBe('1.0.8')
    }
    const licence = fs.readFileSync(path.join(installed, 'LICENSE'), 'utf8')
    expect(licence).toContain('MIT License')
    expect(licence).toContain('Permission is hereby granted, free of charge')
    for (const forbidden of ['GNU GENERAL PUBLIC LICENSE', 'GNU LESSER GENERAL PUBLIC LICENSE', 'GNU AFFERO', 'Server Side Public License']) {
      expect(licence, `AD-26 forbids ${forbidden} at any depth`).not.toContain(forbidden)
    }
  })
})
