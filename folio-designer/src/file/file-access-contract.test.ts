import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { blankComments } from '../../scripts/forbidden-font-hosts.mjs'

const sourceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const production = fs.readdirSync(sourceRoot, { recursive: true }).filter((entry): entry is string => typeof entry === 'string' && /\.(?:ts|tsx)$/.test(entry) && !entry.includes('.test.')).map((entry) => ({ name: String(entry), source: fs.readFileSync(path.join(sourceRoot, String(entry)), 'utf8') }))

// STORY 16.2 — ONE NAMED MODULE MAY HOLD DURABLE STATE, AND IT IS NOT ALLOWED
// TO HOLD A DOCUMENT.
//
// This prohibition is Story 5.5's, and its subject is stated in the test name
// below: no durable/browser-cloud FICTION. A designer that quietly kept the
// author's template in browser storage — or implied a cloud, a sync, an account
// or a recent-files list it does not have — would be lying about where the
// author's work lives, which is the whole reason `.folio` is a file the author
// holds.
//
// Story 16.2 adds durable state on purpose, ruled and scoped: an origin-scoped
// store of FETCHED FONT BYTES, so a typeface downloaded once is offered again
// without a network. It is exempted BY NAME, and only while it still spells its
// own open path — the same shape `canvas-authority-contract.test.ts` uses to
// carve out `embedded-face-registry.ts`. The exemption is deliberately not a
// pattern (`*-store.ts`, say): a second module reaching for browser storage
// must come back here and argue for itself.
//
// AND THE EXEMPTION IS BOUNDED BY WHAT THE MODULE MAY CONTAIN. The store is a
// cache and a source, never an authority on a document: it keeps face bytes and
// their licence record, and a `.folio` still carries its own faces. The
// assertion below holds it to that — a document-shaped word appearing in it
// would mean the exemption had been used for something it was not granted for.
//
// ⚠ AND THE EXEMPTION IS NARROWED TO ONE SPELLING, NOT TO ONE FILE.
//
// An earlier shape of this returned early on the filename, which dropped the
// WHOLE prohibition for `font-store.ts` — not just `indexedDB` but
// `localStorage`, `sessionStorage`, `caches.open`, `showDirectoryPicker`,
// `navigator.storage.getDirectory` and the entire cloud/sync/account fiction
// half. That is exactly backwards for this module: the intent contract's
// `Never:` clause names `localStorage` and names it AT THE STORE ("Never: …
// store in `localStorage`"), so a whole-file skip left the one module the
// contract most explicitly forbids it in as the one module where it was
// unguarded.
//
// The exempt module is therefore SCANNED like every other file, over a source
// with exactly two things taken out of it:
//
//   COMMENTS, because D-16.2 MANDATES that this module write the `localStorage`
//   refusal arithmetic into its own prose ("Write that reasoning into the
//   module, so the next person does not 'simplify' it back") and a raw-text
//   scan cannot tell an explanation of why a thing is refused from the thing.
//   This is the idiom `canvas-authority-contract.test.ts` already uses. It is
//   the comments that are blanked, never the code: `localStorage.setItem(…)`
//   written in this module is still reported, and the mutation proof below is
//   that assertion.
//
//   THE `indexedDB` SPELLING, which is the single prohibition this store was
//   granted an exemption from and the only one it needs — it is a store over
//   IndexedDB, and `globalThis.indexedDB` is its one line of code that names it.
//
// Everything else in the prohibition still applies to `font-store.ts`.
const durableStateExemption = 'font-store.ts'

/** The exempt module's source as the prohibition sees it: comments blanked, and the one granted spelling removed. */
const asScanned = (name: string, source: string): string =>
  name === durableStateExemption ? blankComments(source, '.ts').replace(/\bindexedDB\b/g, '') : source

function forbiddenFileState(files: ReadonlyArray<Readonly<{ name: string; source: string }>>): string[] {
  return files.filter(({ name, source }) => /\b(?:localStorage|sessionStorage|indexedDB|caches\.open|showDirectoryPicker)\b|navigator\.storage\.getDirectory\s*\(|\b(?:cloud|sync|recent files|collaborator|account)\b/i.test(asScanned(name, source))).map(({ name }) => name)
}

function extraCapabilityChecks(files: ReadonlyArray<Readonly<{ name: string; source: string }>>): string[] {
  return files.filter(({ name, source }) => name !== 'file/capability.ts' && /(?:if|\?)\s*\([^\n]*show(?:Open|Save)FilePicker|window\.show(?:Open|Save)FilePicker/.test(source)).map(({ name }) => name)
}

function opaqueByteRewrites(files: ReadonlyArray<Readonly<{ name: string; source: string }>>): string[] {
  return files.filter(({ name, source }) => name !== 'sample-data.ts' && /\bTextDecoder\b|replace\([^)]*\\n[^)]*\\r\\n|new\s+Blob\s*\(\s*\[\s*(?:['"`]|(?:text|decoded|content)\b)/.test(source)).map(({ name }) => name)
}

function fileNetworkCalls(files: ReadonlyArray<Readonly<{ name: string; source: string }>>): string[] {
  return files.filter(({ name, source }) => name.startsWith('file/') && /\bfetch\s*\(/.test(source)).map(({ name }) => name)
}

describe('local file policy structure', () => {
  it('has production witnesses while keeping capability selection in one composition seam and no durable/browser-cloud fiction', () => {
    expect(production.some(({ name }) => name === 'file/capability.ts')).toBe(true)
    expect(production.some(({ name }) => name === 'file/file-system-access.ts')).toBe(true)
    expect(production.some(({ name }) => name === 'file/input-download.ts')).toBe(true)
    expect(extraCapabilityChecks(production)).toEqual([])
    expect(forbiddenFileState(production)).toEqual([])
    expect(opaqueByteRewrites(production)).toEqual([])
    expect(fileNetworkCalls(production)).toEqual([])
  })

  // THE ONE EXEMPTION, HELD TO ITS TERMS. Three claims, and each of them is
  // what makes granting it safe rather than a hole:
  //
  //   it exists (so the exemption is not silently naming a deleted file),
  //   it is the ONLY module reaching for durable browser storage, and
  //   it stores FONT BYTES, never a document.
  it('exempts exactly one module from the durable-state prohibition, and holds it to storing font bytes rather than documents', () => {
    const exempt = production.find(({ name }) => name === durableStateExemption)
    expect(exempt, 'the exemption must name a module that exists; an exemption for a deleted file is a hole nobody can see').toBeDefined()
    // IT REALLY IS THE STORE, not some other file that inherited the name.
    expect(exempt!.source).toContain('export async function openFontStore')
    // AND NOTHING ELSE IS EXEMPT. Removing the name from the filter above puts
    // it straight back into the prohibition, which is the red-proof: this is a
    // carve-out for one module, not a weakening of the rule.
    expect(forbiddenFileState([exempt!])).toEqual([])
    expect(forbiddenFileState([{ ...exempt!, name: 'somewhere-else.ts' }])).toEqual(['somewhere-else.ts'])
    // AND THE REAL MODULE STILL SPELLS THE ONE THING IT WAS EXEMPTED FOR, in
    // CODE and not merely in prose — so this exemption is not quietly covering
    // a module that stopped needing it.
    expect(exempt!.source).toContain('globalThis.indexedDB')
    // THE STORE IS A CACHE AND A SOURCE, NEVER AN AUTHORITY ON A DOCUMENT. It
    // may not keep the author's template, or anything shaped like one.
    for (const documentShaped of [/\bsaveTemplate\b/, /\bdocumentBytes\b/, /\bserialize\b/, /\brevision\b/, /\bsnapshot\b/]) {
      expect(exempt!.source, `the durable-state exemption covers fetched font bytes only; ${String(documentShaped)} is document state`).not.toMatch(documentShaped)
    }
  })

  // THE MUTATION PROOF FOR THE NARROWING, and it is the assertion that makes
  // the paragraph above a mechanism rather than a claim.
  //
  // The exemption is for ONE SPELLING. Every other prohibition still applies
  // INSIDE the exempt file, so a synthetic `font-store.ts` that reached for any
  // of them in code is reported under its real name. Without this, "narrowed"
  // and "skipped entirely" look identical from outside: both leave the real
  // module green.
  it('still reports a non-exempt prohibition written inside the exempt module itself', () => {
    const inTheStore = (source: string) => forbiddenFileState([{ name: durableStateExemption, source }])
    // `localStorage` IS THE ONE THAT MATTERS. The intent contract's `Never:`
    // clause names it and names it at this module — "Never: … store in
    // `localStorage`" — so this is the prohibition a whole-file exemption
    // dropped at precisely the place it was written for.
    expect(inTheStore('localStorage.setItem("faces", base64)\n')).toEqual([durableStateExemption])
    expect(inTheStore('sessionStorage.setItem("faces", base64)\n')).toEqual([durableStateExemption])
    expect(inTheStore('await caches.open("folio8-faces")\n')).toEqual([durableStateExemption])
    expect(inTheStore('const directory = await showDirectoryPicker()\n')).toEqual([durableStateExemption])
    expect(inTheStore('const root = await navigator.storage.getDirectory()\n')).toEqual([durableStateExemption])
    // AND THE FICTION HALF, which the whole-file exemption also dropped.
    expect(inTheStore('const label = "synced from your cloud account"\n')).toEqual([durableStateExemption])
    // THE COMMENT DIRECTION, stated so it cannot be read the wrong way round.
    // Blanking comments is what lets D-16.2's MANDATED `localStorage`
    // arithmetic live in this module's prose. It exempts prose only; the line
    // above proves code is still caught.
    expect(inTheStore('// `localStorage` is a ~5 MB string quota, so it is refused\n')).toEqual([])
    // AND THE GRANTED SPELLING, which is the only thing that passes in code.
    expect(inTheStore('const factory = globalThis.indexedDB\n')).toEqual([])
    // ...but only for THIS module. Anywhere else it is a violation like any other.
    expect(forbiddenFileState([{ name: 'file/cache.ts', source: 'const factory = globalThis.indexedDB\n' }])).toEqual(['file/cache.ts'])
  })

  it('red-proves an accidental second capability branch and prohibited durable file state', () => {
    expect(extraCapabilityChecks([{ name: 'App.tsx', source: 'if (window.showSaveFilePicker) save()' }])).toEqual(['App.tsx'])
    expect(forbiddenFileState([{ name: 'file/cache.ts', source: 'localStorage.setItem("template", "bytes")' }])).toEqual(['file/cache.ts'])
    expect(forbiddenFileState([{ name: 'file/opfs.ts', source: 'navigator.storage.getDirectory()' }])).toEqual(['file/opfs.ts'])
    expect(fileNetworkCalls([{ name: 'file/upload.ts', source: 'fetch("/template", { method: "POST" })' }])).toEqual(['file/upload.ts'])
    expect(opaqueByteRewrites([{ name: 'file/rewrite.ts', source: 'const text = new TextDecoder().decode(bytes); new Blob([text.replace(/\\n/g, "\\r\\n")])' }])).toEqual(['file/rewrite.ts'])
  })
})
