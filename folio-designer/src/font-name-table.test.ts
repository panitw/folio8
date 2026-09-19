import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { faceCopyright, fontView, nameTableString, requireStaticTrueTypeTables, sfntTableDirectory } from './font-name-table'
import { sfntWithCopyright, sfntWithNames } from './test/sfnt-fixture'
import { blankComments } from '../scripts/forbidden-font-hosts.mjs'

// STORY 16.1 — THE ONE `name`-TABLE READER, HELD TO BOTH CORPORA.
//
// It is checked against REAL COMMITTED FACES, which is what makes it a reader of
// fonts rather than of its own fixtures, AND against synthesised faces, because
// the cases that matter most — no nameID 0, no `name` table at all, a
// Macintosh-platform record — do not occur among 21 well-formed binaries and
// cannot be asserted about hypothetically.

const here = path.dirname(fileURLToPath(import.meta.url))
const fontsRoot = path.join(here, '..', 'public', 'fonts')
const manifest: ReadonlyArray<{ directory: string; file: string; family: string }> = JSON.parse(fs.readFileSync(path.join(here, '..', 'font-catalogue.json'), 'utf8'))

describe('the shared sfnt name-table reader', () => {
  // NON-VACUITY: the loop below is over the manifest, and an empty one passes
  // every assertion in it.
  it('reads a copyright out of every committed catalogue face', () => {
    // THE POPULATION FLOOR — ONE OF FOUR, AND ALL FOUR MOVE TOGETHER.
    // The other three are `src/font-catalogue.test.ts` ("declares at least twenty NEW families"),
    // `src/font-index.test.ts` ("is the whole bundled catalogue, unchanged") and
    // `src/font-provenance.test.ts` ("is asserted over the whole committed tier").
    // Raised 20 -> 31 by Story 16.1a, which added ten families to the local
    // face tier. D-16.R.12: "a floor left at 21 while the tier grows to 30 is
    // a floor that stops measuring the thing it was built to measure" — and
    // D-16.R.18's correction to it: a floor that exists in N files is N
    // floors, so a batch that raises one and leaves the rest behind is
    // silently unmeasured at the ones it left.

    expect(manifest.length, 'the local face tier population floor; Story 16.1a raised it 20 -> 31').toBeGreaterThanOrEqual(31)
    for (const face of manifest) {
      const bytes = fs.readFileSync(path.join(fontsRoot, face.directory, face.file))
      const copyright = faceCopyright(bytes)
      expect(copyright.trim(), `${face.family} must state whose it is`).toBe(copyright)
      expect(copyright.length, `${face.family}'s copyright is implausibly short`).toBeGreaterThan(4)
    }
  })

  it('reads the family and licence-description records the same walk reaches', () => {
    const face = manifest[0]
    const view = fontView(fs.readFileSync(path.join(fontsRoot, face.directory, face.file)))
    const tables = requireStaticTrueTypeTables(view)
    expect(Object.keys(tables)).toContain('name')
    expect(nameTableString(view, tables, 1) ?? nameTableString(view, tables, 16)).toBeTruthy()
  })

  it('refuses a container whose sfnt version is not a static TrueType outline font', () => {
    // `OTTO`, and a WOFF wrapper. Both have a table directory at the same
    // offsets and mean something different, so reading one as the other would
    // produce plausible garbage rather than an error.
    for (const version of [0x4f54544f, 0x774f4646, 0x774f4632]) {
      expect(() => requireStaticTrueTypeTables(fontView(sfntWithNames([{ platform: 3, nameID: 0, value: 'x' }], { sfntVersion: version })))).toThrow(/not a static TrueType sfnt/)
    }
  })

  it('decodes a Unicode-platform record as UTF-16BE and a Macintosh one as single bytes', () => {
    const bytes = sfntWithNames([
      { platform: 1, nameID: 0, value: 'Mac Roman copyright' },
      { platform: 3, nameID: 13, value: 'Licensed under the SIL Open Font License, Version 1.1' },
    ])
    const view = fontView(bytes)
    const tables = sfntTableDirectory(view)
    expect(nameTableString(view, tables, 13)).toBe('Licensed under the SIL Open Font License, Version 1.1')
    expect(nameTableString(view, tables, 0)).toBe('Mac Roman copyright')
  })

  it('prefers a Unicode-platform record when a face carries both', () => {
    const bytes = sfntWithNames([
      { platform: 1, nameID: 0, value: 'the single-byte copy' },
      { platform: 3, nameID: 0, value: 'the Unicode copy' },
    ])
    const view = fontView(bytes)
    expect(nameTableString(view, sfntTableDirectory(view), 0)).toBe('the Unicode copy')
  })

  // SILENCE AND ABSENCE ARE DIFFERENT ANSWERS, and this reader does not collapse
  // them: it returns `undefined` and lets the caller decide, which is what lets
  // Go's licence tie ADMIT a silent face (D-16.R.7) while `faceCopyright`
  // REFUSES one.
  it('returns nothing rather than throwing when the record or the whole table is absent', () => {
    const noRecord = fontView(sfntWithNames([{ platform: 3, nameID: 1, value: 'Family Only' }]))
    expect(nameTableString(noRecord, sfntTableDirectory(noRecord), 0)).toBeUndefined()
    const noTable = fontView(sfntWithNames([], { omitNameTable: true }))
    expect(nameTableString(noTable, sfntTableDirectory(noTable), 0)).toBeUndefined()
  })

  // BUT `copyright` IS REQUIRED OF AN EMBEDDED FACE. The engine refuses to load
  // a document that embeds a face without one (`requireEmbeddedFaceLicence`), so
  // admitting a blank here would write a file this product cannot open. AND
  // BLANK IS EMPTY: `" "` states exactly as much as `""`.
  it('refuses an absent, empty or blank copyright, in the words the engine would use', () => {
    for (const bytes of [sfntWithNames([], { omitNameTable: true }), sfntWithNames([{ platform: 3, nameID: 1, value: 'Family Only' }]), sfntWithCopyright(''), sfntWithCopyright('   \n ')]) {
      expect(() => faceCopyright(bytes)).toThrow(/declares no copyright in its own `name` table \(nameID 0\)/)
    }
    expect(faceCopyright(sfntWithCopyright('  Copyright 2026 Someone  '))).toBe('Copyright 2026 Someone')
  })

  // THE UNTRUSTED CALLER GETS THE VERSION GUARD TOO. `faceCopyright`'s hottest
  // caller is `font-source.ts`, over bytes fetched from a third party seconds
  // earlier, so it reads the directory through `requireStaticTrueTypeTables`
  // rather than through the unchecked walk: an `OTTO`/CFF or WOFF container has
  // a directory at the same offsets and means something else, and a 200 that is
  // not a font at all has no directory to walk. Both are REFUSED, in the version
  // message, rather than producing a plausible string.
  it('refuses a container that is not a static TrueType sfnt rather than walking it', () => {
    for (const version of [0x4f54544f, 0x774f4646, 0x774f4632]) {
      const wrapped = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright someone else' }], { sfntVersion: version })
      expect(() => faceCopyright(wrapped)).toThrow(/not a static TrueType sfnt/)
    }
    // A plain 200 that is not a font — an error page, a redirect notice — is the
    // same answer in the same words, and never a `RangeError` from the walk.
    expect(() => faceCopyright(new TextEncoder().encode('<!doctype html><title>404: Not Found</title>'))).toThrow(/not a static TrueType sfnt/)
    expect(() => faceCopyright(new Uint8Array([0x00, 0x01]))).toThrow(/not a static TrueType sfnt/)
  })

  // A TRUNCATED OR HOSTILE `name` TABLE YIELDS ABSENCE, NOT GARBAGE. The record
  // offsets are third-party numbers; a record pointing past the table's own
  // declared length is skipped, so the caller that requires a value states its
  // refusal instead of the walk throwing or slicing whatever follows.
  it('reads no record that points past the name table the file declares', () => {
    const bytes = new Uint8Array(sfntWithCopyright('Copyright 2026 Someone'))
    const view = fontView(bytes)
    const name = requireStaticTrueTypeTables(view)['name']
    // The record's string offset, moved far beyond the end of the table.
    const record = name.offset + 6
    new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength).setUint16(record + 10, 0xfff0)
    expect(nameTableString(fontView(bytes), requireStaticTrueTypeTables(fontView(bytes)), 0)).toBeUndefined()
    expect(() => faceCopyright(bytes)).toThrow(/declares no copyright in its own `name` table \(nameID 0\)/)
  })

  it('reads an ArrayBuffer and a view over one identically', () => {
    const bytes = sfntWithCopyright('Copyright 2026 Someone')
    expect(faceCopyright(bytes)).toBe(faceCopyright(new Uint8Array(bytes)))
    // AND AT A NON-ZERO byteOffset, which is what a Node `Buffer` almost always
    // is: a view into a shared pool. A reader that ignored byteOffset would
    // read another face's bytes.
    const padded = new Uint8Array(bytes.byteLength + 8)
    padded.set(new Uint8Array(bytes), 8)
    expect(faceCopyright(padded.subarray(8))).toBe('Copyright 2026 Someone')
  })

  // NO PARSER DEPENDENCY, AND NO `Buffer`. The first is a standing decision the
  // designer's payload budget rests on; the second is what lets one module serve
  // the browser, vitest and the plain-`node` build script.
  it('adds no dependency and uses no Node-only API', () => {
    // Comment-blanked: the module EXPLAINS that it uses no `Buffer` and why,
    // and a check that could not tell the explanation from a use would be
    // satisfied by deleting the explanation.
    const code = blankComments(fs.readFileSync(path.join(here, 'font-name-table.ts'), 'utf8'), '.ts')
    expect(code, 'the blanker must not have eaten the file').toContain('faceCopyright')
    expect(code).not.toMatch(/\bBuffer\b/)
    expect(code).not.toMatch(/^import /m)
    const packageJson = JSON.parse(fs.readFileSync(path.join(here, '..', 'package.json'), 'utf8')) as { dependencies: Record<string, string> }
    expect(Object.keys(packageJson.dependencies).sort()).toEqual(['pdfjs-dist', 'react', 'react-dom'])
  })

  // THE EXTRACTION IS REAL: neither remaining caller carries its own walk any
  // more. A "shared" reader beside two surviving hand-copies would be a third
  // copy, which is exactly what this story set out not to write.
  it('is the only sfnt name-table walk left in the designer', () => {
    for (const file of [path.join(here, 'font-catalogue.test.ts'), path.join(here, '..', 'scripts', 'build-wasm.mjs')]) {
      const source = fs.readFileSync(file, 'utf8')
      expect(source, `${path.basename(file)} must not declare its own name-table walk`).not.toMatch(/function nameTableString|const nameTableString/)
    }
  })
})
