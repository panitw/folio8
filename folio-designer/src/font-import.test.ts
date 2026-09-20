import { describe, expect, it, vi } from 'vitest'
import { FileAccessCancelled, type LocalFileHandle } from './file/file-access'
import { FileSystemFontAccess, InputFontAccess, type FontPicker } from './font-file'
import { selectFontFileAccess } from './file/capability'
import { acknowledgedFace, authorSuppliedFaceSource, importFontFiles, importedFaceName, refusedFontFileReport } from './font-import'
import { sfntWithNames } from './test/sfnt-fixture'
import { assertAuthorSuppliedProvenanceShape } from './test/provenance-shape'

// STORY 3 — IMPORTING FONT FILES FROM THE AUTHOR'S OWN MACHINE.
//
// ONE CASE PER ROW OF THE STORY'S I/O & EDGE-CASE MATRIX, driven with real
// `File` objects through the real access tiers where the row is about the
// picker, and through `importFontFiles` over the real bytes where the row is
// about what a binary says about itself. `src/image-file.test.ts` is the shape
// both halves follow.
//
// NOTHING HERE OPENS THE STORE. The import module writes nothing and knows
// nothing about IndexedDB; the store write, the acknowledgement and the census
// are `App.tsx`'s and are driven in `App.font-import.test.tsx`.

const face = (family: string, subfamily?: string, extra: ReadonlyArray<Readonly<{ nameID: number; value: string }>> = [], options?: Readonly<{ withFvar?: boolean }>): ArrayBuffer =>
  sfntWithNames([
    { platform: 3, nameID: 1, value: family },
    ...(subfamily === undefined ? [] : [{ platform: 3, nameID: 2, value: subfamily }]),
    ...extra.map((record) => ({ platform: 3, ...record })),
  ], options)

const picked = (name: string, bytes: ArrayBuffer) => ({ name, mediaType: '', bytes })

describe('reading font files the author picked', () => {
  it('imports one picked face, keyed family-only when its subfamily is Regular', () => {
    const outcome = importFontFiles([picked('whatever-they-called-it.ttf', face('Sarabun', 'Regular'))])
    expect(outcome.refused).toEqual([])
    expect(outcome.families).toHaveLength(1)
    const [family] = outcome.families
    expect(family.family).toBe('Sarabun')
    expect(family.faces.map((entry) => entry.style)).toEqual(['Regular'])
    // D2: THE KEY IS `fontdir`'s KEY. A Regular is the bare family, so a
    // document naming this face names what a host's directory would produce.
    expect(importedFaceName(family.faces[0].family, family.faces[0].style)).toBe('Sarabun')
  })

  it('treats an ABSENT subfamily record exactly as `Regular`, which is `fontdir.faceKey`s rule', () => {
    const outcome = importFontFiles([picked('a.ttf', face('Sarabun'))])
    expect(outcome.families[0].faces[0].style).toBe('Regular')
    expect(importedFaceName('Sarabun', outcome.families[0].faces[0].style)).toBe('Sarabun')
  })

  it('groups four cuts picked in one gesture into ONE family of four, keyed as a host would key them', () => {
    const outcome = importFontFiles([
      picked('1.ttf', face('Sarabun', 'Regular')),
      picked('2.ttf', face('Sarabun', 'Bold')),
      picked('3.ttf', face('Sarabun', 'Italic')),
      picked('4.otf', face('Sarabun', 'Bold Italic')),
    ])
    expect(outcome.refused).toEqual([])
    expect(outcome.families).toHaveLength(1)
    expect(outcome.families[0].faces.map((entry) => importedFaceName(entry.family, entry.style)))
      .toEqual(['Sarabun', 'Sarabun Bold', 'Sarabun Italic', 'Sarabun Bold Italic'])
  })

  it('groups files from two families into TWO families, by their name records and never by the gesture', () => {
    const outcome = importFontFiles([
      picked('1.ttf', face('Sarabun', 'Regular')),
      picked('2.ttf', face('Prompt', 'Bold')),
      picked('3.ttf', face('Sarabun', 'Bold')),
    ])
    expect(outcome.families.map((entry) => entry.family)).toEqual(['Sarabun', 'Prompt'])
    expect(outcome.families[0].faces).toHaveLength(2)
    expect(outcome.families[1].faces).toHaveLength(1)
  })

  it('reads the family and the cut from the BINARY, so renaming the file on disk changes neither', () => {
    const bytes = face('Sarabun', 'Bold')
    const named = importFontFiles([picked('Sarabun-Bold.ttf', bytes)]).families[0].faces[0]
    const renamed = importFontFiles([picked('untitled-copy-2.ttf', bytes)]).families[0].faces[0]
    expect(renamed.family).toBe(named.family)
    expect(renamed.style).toBe(named.style)
    expect(renamed.family).toBe('Sarabun')
    expect(renamed.style).toBe('Bold')
  })

  it('refuses ONE variable build and imports the other three, per file', () => {
    const outcome = importFontFiles([
      picked('regular.ttf', face('Sarabun', 'Regular')),
      picked('bold.ttf', face('Sarabun', 'Bold')),
      picked('italic.ttf', face('Sarabun', 'Italic', [], { withFvar: true })),
      picked('bolditalic.ttf', face('Sarabun', 'Bold Italic')),
    ])
    expect(outcome.families[0].faces.map((entry) => entry.style)).toEqual(['Regular', 'Bold', 'Bold Italic'])
    expect(outcome.refused).toHaveLength(1)
    expect(outcome.refused[0].file).toBe('italic.ttf')
    expect(outcome.refused[0].reason).toContain('VARIABLE')
  })

  it('refuses a real .png and a .png somebody renamed .ttf, and the rest still import', () => {
    const png = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 13, 0, 0]).buffer
    const outcome = importFontFiles([
      picked('logo.png', png),
      picked('logo-renamed.ttf', png),
      picked('good.ttf', face('Sarabun', 'Regular')),
    ])
    expect(outcome.families.map((entry) => entry.family)).toEqual(['Sarabun'])
    expect(outcome.refused.map((entry) => entry.file)).toEqual(['logo.png', 'logo-renamed.ttf'])
    // THE TWO ARE REFUSED BY TWO DIFFERENT CHECKS AND BOTH SENTENCES SAY SO:
    // the extension settles the first without reading a byte, and the container
    // guard settles the second over the bytes themselves.
    expect(outcome.refused[0].reason).toContain('does not end in .ttf or .otf')
    expect(outcome.refused[1].reason).toContain('not a static TrueType sfnt')
  })

  it('imports a face whose copyright, licence and licence-text records are ALL absent, with empty strings and no throw', () => {
    const outcome = importFontFiles([picked('a.ttf', face('Sarabun', 'Regular'))])
    const [imported] = outcome.families[0].faces
    expect(imported.copyright).toBe('')
    expect(imported.licence).toBe('')
    expect(imported.licenceText).toBe('')
  })

  it('transcribes nameID 0, 13 and 14 verbatim, with no classification of any kind', () => {
    const outcome = importFontFiles([picked('a.ttf', face('Sarabun', 'Regular', [
      { nameID: 0, value: 'Copyright 2019 The Sarabun Project Authors' },
      { nameID: 13, value: 'This Font Software is licensed under the SIL Open Font License, Version 1.1.' },
      { nameID: 14, value: 'https://scripts.sil.org/OFL' },
    ]))])
    const [imported] = outcome.families[0].faces
    expect(imported.copyright).toBe('Copyright 2019 The Sarabun Project Authors')
    expect(imported.licenceText).toBe('This Font Software is licensed under the SIL Open Font License, Version 1.1.')
    expect(imported.licence).toBe('https://scripts.sil.org/OFL')
  })

  it('imports a face whose binary names a COPYLEFT licence without classifying, warning or refusing it', () => {
    // The product takes no position on the terms of a face the author supplies:
    // no allowlist, no blocklist, no banner. `src/font-licence.ts` governs the
    // catalogue tier and is not on this path.
    const outcome = importFontFiles([picked('a.ttf', face('Brand Grotesk', 'Regular', [
      { nameID: 14, value: 'GPL-3.0-or-later' },
    ]))])
    expect(outcome.refused).toEqual([])
    expect(outcome.families[0].faces[0].licence).toBe('GPL-3.0-or-later')
  })

  it('refuses a face whose family record is absent or blank, because it cannot be keyed', () => {
    const outcome = importFontFiles([
      picked('no-family.ttf', sfntWithNames([{ platform: 3, nameID: 2, value: 'Bold' }])),
      picked('blank-family.ttf', face('   ', 'Bold')),
    ])
    expect(outcome.families).toEqual([])
    expect(outcome.refused.map((entry) => entry.file)).toEqual(['no-family.ttf', 'blank-family.ttf'])
    for (const refusal of outcome.refused) expect(refusal.reason).toContain('record 1')
  })

  // WARNING: THE CONTROL CHARACTER IS WRITTEN AS AN ESCAPE, DELIBERATELY. A raw
  // BEL byte in the literal is invisible on screen, so the case READS as
  // asserting that a perfectly clean `Sarabun` is refused — a test whose
  // meaning depends on a byte nobody can see is a test nobody can review.
  it('refuses a FAMILY record carrying control characters, exactly as `fontdir` does', () => {
    const outcome = importFontFiles([picked('a.ttf', face('Sar\u0007abun', 'Regular'))])
    expect(outcome.families).toEqual([])
    expect(outcome.refused[0].reason).toContain('control characters')
  })

  it('refuses a SUBFAMILY record carrying control characters, which is the other half of the same rule', () => {
    const outcome = importFontFiles([picked('a.ttf', face('Sarabun', 'Bo\u007Fld'))])
    expect(outcome.families).toEqual([])
    expect(outcome.refused[0].reason).toContain('control characters')
  })

  it('trims the name records before keying, because `fontdir` trims before it joins', () => {
    const outcome = importFontFiles([picked('a.ttf', face('  Sarabun  ', '  Bold  '))])
    expect(importedFaceName(outcome.families[0].family, outcome.families[0].faces[0].style)).toBe('Sarabun Bold')
  })

  it('stores the media type the ENGINE recognises rather than whatever the browser declared', () => {
    const outcome = importFontFiles([
      { name: 'a.ttf', mediaType: 'application/octet-stream', bytes: face('Sarabun', 'Regular') },
      { name: 'b.otf', mediaType: '', bytes: face('Sarabun', 'Bold') },
    ])
    expect(outcome.families[0].faces.map((entry) => entry.mediaType)).toEqual(['font/ttf', 'font/otf'])
  })

  it('refuses the SECOND file declaring a face name the first already claimed, by name', () => {
    // Two files, same family and same cut, different bytes: nothing anywhere
    // can tell which one the author meant, so the first picked wins and the
    // other is refused rather than both landing as two conflicting Bolds.
    const outcome = importFontFiles([
      picked('Brand-Bold.ttf', face('Brand Grotesk', 'Bold')),
      picked('Brand-Bold-v2.ttf', face('Brand Grotesk', 'Bold', [{ nameID: 0, value: 'a different build' }])),
      picked('Brand-Italic.ttf', face('Brand Grotesk', 'Italic')),
    ])
    expect(outcome.families[0].faces.map((entry) => entry.style)).toEqual(['Bold', 'Italic'])
    expect(outcome.refused).toHaveLength(1)
    expect(outcome.refused[0].file).toBe('Brand-Bold-v2.ttf')
    expect(outcome.refused[0].reason).toContain('Brand Grotesk Bold')
    expect(outcome.refused[0].reason).toContain('Brand-Bold.ttf')
  })

  it('refuses a family name the designer already offers, because the row would be reachable from nowhere', () => {
    const outcome = importFontFiles([
      picked('kanit.ttf', face('Kanit', 'Regular')),
      picked('brand.ttf', face('Brand Grotesk', 'Regular')),
    ], (family) => family === 'Kanit')
    expect(outcome.families.map((entry) => entry.family)).toEqual(['Brand Grotesk'])
    expect(outcome.refused[0].file).toBe('kanit.ttf')
    expect(outcome.refused[0].reason).toContain('already offers')
  })

  it('refuses a file whose name does not end in .ttf or .otf, and says the one thing that fixes it', () => {
    const outcome = importFontFiles([picked('BrandGrotesk.ttf.bak', face('Brand Grotesk', 'Regular'))])
    expect(outcome.families).toEqual([])
    expect(outcome.refused[0].reason).toContain('rename it')
  })

  it('reports nothing when nothing was refused', () => {
    expect(refusedFontFileReport([])).toBe('')
  })
})

describe('`source` for the author-supplied tier', () => {
  it('carries no path, no filename, no scheme and no digest, and names the acknowledgement day', () => {
    assertAuthorSuppliedProvenanceShape(expect, 'authorSuppliedFaceSource, called directly', authorSuppliedFaceSource('2026-09-21'))
  })

  it('is the source every acknowledged face actually carries', () => {
    const outcome = importFontFiles([picked('Sarabun-Bold.ttf', face('Sarabun', 'Bold'))])
    const stamped = acknowledgedFace(outcome.families[0].faces[0], '2026-09-21')
    assertAuthorSuppliedProvenanceShape(expect, 'an imported face', stamped.source)
    // AND THE FILENAME IS NOWHERE IN THE RECORD, which is the prohibition the
    // shape assertion states generically, said once against the concrete file.
    expect(JSON.stringify({ ...stamped, bytes: undefined })).not.toContain('Sarabun-Bold.ttf')
  })

  // spec-font-sources-and-embedding STORY 6, D4 — THE ASSERTION IS A FIELD AND
  // NOT A SENTENCE TO BE PARSED BACK.
  //
  // `source` is prose about the acknowledgement; `authorAcknowledged` is the
  // fact the engine reads to admit a face whose binary declares terms it would
  // otherwise refuse. They are stamped together by one writer, and the boolean
  // exists so nothing downstream ever has to read the prose — which would be a
  // second authority over one fact.
  it('stamps the acknowledgement itself beside the sentence about it', () => {
    const [read] = importFontFiles([picked('a.ttf', face('Sarabun', 'Regular'))]).families[0].faces
    expect('authorAcknowledged' in read, 'an unacknowledged face carries no acknowledgement at all').toBe(false)
    expect(acknowledgedFace(read, '2026-09-21').authorAcknowledged).toBe(true)
  })

  it('is stamped at the ACKNOWLEDGEMENT and not at the pick, so no day can be recorded that nobody acknowledged on', () => {
    // An unacknowledged face carries no `source` at all — there is no shape in
    // which the pick's day could be the one recorded.
    const [read] = importFontFiles([picked('a.ttf', face('Sarabun', 'Regular'))]).families[0].faces
    expect('source' in read).toBe(false)
    expect(acknowledgedFace(read, '2026-09-22').source).toContain('2026-09-22')
  })
})

// ─────────────────────────────────────────────────────────────────────────────
// THE PICKER SEAM, MIRRORING `src/image-file.test.ts` CASE FOR CASE.

const handleFor = (file: File): LocalFileHandle => ({ name: file.name, getFile: async () => file, createWritable: async () => ({ write: async () => undefined, close: async () => undefined }) })
const fileFor = (name: string, bytes: ArrayBuffer): File => ({ name, type: 'font/ttf', arrayBuffer: vi.fn(async () => bytes) } as unknown as File)

describe('local font-file boundary', () => {
  it('passes every picked file through the File System Access tier, in order', async () => {
    const one = face('Sarabun', 'Regular')
    const two = face('Sarabun', 'Bold')
    const access = new FileSystemFontAccess({ showOpenFilePicker: vi.fn(async () => [handleFor(fileFor('a.ttf', one)), handleFor(fileFor('b.ttf', two))]) })
    await expect(access.openFonts()).resolves.toEqual([
      { name: 'a.ttf', mediaType: 'font/ttf', bytes: one },
      { name: 'b.ttf', mediaType: 'font/ttf', bytes: two },
    ])
  })

  it('asks the File System Access picker for MULTIPLE files, restricted to .ttf and .otf', async () => {
    const showOpenFilePicker = vi.fn<FontPicker['showOpenFilePicker']>(async () => [])
    await new FileSystemFontAccess({ showOpenFilePicker }).openFonts().catch(() => undefined)
    const options = showOpenFilePicker.mock.calls[0]![0]
    // ONE ACKNOWLEDGEMENT PER IMPORT IS WHAT `multiple` BUYS. A single-file
    // picker would make a four-cut family four separate admissions.
    expect(options.multiple).toBe(true)
    expect(options.types).toEqual([{ description: 'Font', accept: { 'font/ttf': ['.ttf'], 'font/otf': ['.otf'] } }])
  })

  it('rejects a dismissed picker as FileAccessCancelled, so nothing happens and no dialog is raised', async () => {
    await expect(new FileSystemFontAccess({ showOpenFilePicker: vi.fn(async () => []) }).openFonts()).rejects.toBeInstanceOf(FileAccessCancelled)
  })

  it('passes every picked file through the <input type=file> fallback tier', async () => {
    const one = face('Sarabun', 'Regular')
    const access = new InputFontAccess(document)
    const pending = access.openFonts()
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    expect(input.accept).toBe('.ttf,.otf,font/ttf,font/otf')
    expect(input.multiple).toBe(true)
    Object.defineProperty(input, 'files', { value: [fileFor('a.ttf', one)], configurable: true })
    input.dispatchEvent(new Event('change'))
    await expect(pending).resolves.toEqual([{ name: 'a.ttf', mediaType: 'font/ttf', bytes: one }])
  })

  it('rejects cancellation from the <input type=file> fallback tier', async () => {
    const access = new InputFontAccess(document)
    const pending = access.openFonts()
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    input.dispatchEvent(new Event('cancel'))
    await expect(pending).rejects.toBeInstanceOf(FileAccessCancelled)
  })

  it('selects exactly one capability tier, through the one composition seam', () => {
    const url = { createObjectURL: vi.fn(), revokeObjectURL: vi.fn() }
    expect(selectFontFileAccess({ document, url, showOpenFilePicker: vi.fn() })).toBeInstanceOf(FileSystemFontAccess)
    expect(selectFontFileAccess({ document, url })).toBeInstanceOf(InputFontAccess)
  })

  it("binds the picker to selectFontFileAccess's OWN browser argument (eef7fbb precedent)", async () => {
    const receivers: unknown[] = []
    const explicitBrowser = {
      document,
      url: { createObjectURL: vi.fn(), revokeObjectURL: vi.fn() },
      showOpenFilePicker: function (this: unknown) { receivers.push(this); return Promise.resolve([]) },
    }
    await expect(selectFontFileAccess(explicitBrowser).openFonts()).rejects.toBeInstanceOf(FileAccessCancelled)
    expect(receivers).toEqual([explicitBrowser])
  })
})
