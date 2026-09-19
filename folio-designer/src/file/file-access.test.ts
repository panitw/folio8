import { describe, expect, it, vi } from 'vitest'
import { selectFileAccess, selectSampleFileAccess } from './capability'
import { FileAccessCancelled, folioFileFormat, jsonSampleFileFormat, localFileName, pdfFileFormat, type LocalFileFormat, type LocalFileHandle } from './file-access'
import { FileSystemAccess } from './file-system-access'
import { InputDownloadAccess } from './input-download'

const bytes = new Uint8Array([0, 255, 7]).buffer
// STORY 13.1 — THE PDF FIXTURE, AND WHY IT LOOKS LIKE THIS.
//
// Thirty-three bytes, non-uniform, carrying a high byte (255), a NUL, a CR/LF
// pair and a run that is not a prefix of itself. A truncation, a text decode
// and re-encode, or a UTF-8 round trip all CHANGE it — which a three-byte
// ascending fixture would not reliably show.
const pdfBytes = new Uint8Array([37, 80, 68, 70, 45, 49, 46, 55, 10, 37, 226, 227, 207, 211, 10, 0, 255, 128, 1, 254, 200, 17, 42, 7, 240, 13, 10, 37, 37, 69, 79, 70, 10]).buffer
const pdfByteList = Array.from(new Uint8Array(pdfBytes)).join(',')
const file = () => new File([bytes], 'report.folio', { type: 'application/json' })

function handle(name = 'report.folio', events: string[] = []): LocalFileHandle {
  return {
    name,
    getFile: async () => file(),
    createWritable: async () => ({
      write: async (written) => { events.push(`write:${Array.from(new Uint8Array(written)).join(',')}`) },
      close: async () => { events.push('close') },
    }),
  }
}

describe('local file access boundary', () => {
  it('selects exactly one capability tier from complete picker capabilities and falls back for incomplete APIs', () => {
    const url = { createObjectURL: vi.fn(() => 'blob:local'), revokeObjectURL: vi.fn() }
    const complete = { document, url, showOpenFilePicker: vi.fn(), showSaveFilePicker: vi.fn() }
    expect(selectFileAccess(complete)).toBeInstanceOf(FileSystemAccess)
    expect(complete.showOpenFilePicker).toHaveBeenCalledTimes(0)
    expect(selectFileAccess({ document, url, showOpenFilePicker: vi.fn() })).toBeInstanceOf(InputDownloadAccess)
  })

  it('probes the real window picker pair when no explicit browser seam is supplied', () => {
    const pickerWindow = window as typeof window & { showOpenFilePicker?: ReturnType<typeof vi.fn>; showSaveFilePicker?: ReturnType<typeof vi.fn> }
    const priorOpen = pickerWindow.showOpenFilePicker
    const priorSave = pickerWindow.showSaveFilePicker
    pickerWindow.showOpenFilePicker = vi.fn()
    pickerWindow.showSaveFilePicker = vi.fn()
    try { expect(selectFileAccess()).toBeInstanceOf(FileSystemAccess) }
    finally { pickerWindow.showOpenFilePicker = priorOpen; pickerWindow.showSaveFilePicker = priorSave }
  })

  it('invokes the real window pickers with the window as their receiver', async () => {
    // The pickers are Window methods and the browser rejects any other
    // receiver with an illegal invocation, which the boundary would report as
    // an unreadable local file.
    const pickerWindow = window as typeof window & { showOpenFilePicker?: unknown; showSaveFilePicker?: unknown }
    const priorOpen = pickerWindow.showOpenFilePicker
    const priorSave = pickerWindow.showSaveFilePicker
    const receivers: unknown[] = []
    const record = function (this: unknown) { receivers.push(this); return Promise.resolve([]) }
    pickerWindow.showOpenFilePicker = record
    pickerWindow.showSaveFilePicker = record
    try {
      await expect(selectSampleFileAccess().openSample()).rejects.toBeInstanceOf(FileAccessCancelled)
      await expect(selectFileAccess().open()).rejects.toBeInstanceOf(FileAccessCancelled)
      expect(receivers).toEqual([window, window])
    } finally { pickerWindow.showOpenFilePicker = priorOpen; pickerWindow.showSaveFilePicker = priorSave }
  })

  it('returns opaque selected bytes and an in-memory target from the File System Access tier', async () => {
    const selected = handle()
    const access = new FileSystemAccess({ showOpenFilePicker: vi.fn(async () => [selected]), showSaveFilePicker: vi.fn() })
    const opened = await access.open()
    expect(opened.name).toBe('report.folio')
    expect(opened.target?.handle).toBe(selected)
    expect(new Uint8Array(opened.bytes)).toEqual(new Uint8Array(bytes))
  })

  it('silently classifies picker aborts and writes then closes before reporting an in-place save', async () => {
    const events: string[] = []
    const selected = handle('saved.folio', events)
    const picker = { showOpenFilePicker: vi.fn(async () => { throw new DOMException('cancel', 'AbortError') }), showSaveFilePicker: vi.fn(async () => selected) }
    const access = new FileSystemAccess(picker)
    await expect(access.open()).rejects.toBeInstanceOf(FileAccessCancelled)
    const target = await access.acquireSaveTarget({ suggestedName: 'ignored.folio', currentTarget: { kind: 'in-place', name: selected.name, handle: selected }, saveAs: false, format: folioFileFormat })
    await expect(access.writeSave(target, { bytes })).resolves.toMatchObject({ name: 'saved.folio', target: { handle: selected } })
    expect(events).toEqual(['write:0,255,7', 'close'])
    expect(picker.showSaveFilePicker).not.toHaveBeenCalled()
  })

  it('uses a fresh File System Access target for Save As and does not report success when close fails', async () => {
    const broken: LocalFileHandle = { name: 'new.folio', getFile: async () => file(), createWritable: async () => ({ write: async () => undefined, close: async () => { throw new Error('media removed') } }) }
    const picker = { showOpenFilePicker: vi.fn(), showSaveFilePicker: vi.fn(async () => broken) }
    const access = new FileSystemAccess(picker)
    const target = await access.acquireSaveTarget({ suggestedName: 'old.folio', saveAs: true, format: folioFileFormat })
    await expect(access.writeSave(target, { bytes })).rejects.toThrow('Could not save local file')
    expect(picker.showSaveFilePicker).toHaveBeenCalledWith(expect.objectContaining({ suggestedName: 'old.folio' }))
  })

  // THE BROWSER ALREADY NAMED THE PROBLEM; THE BOUNDARY MUST NOT UNNAME IT.
  //
  // Every throw here used to collapse to the bare sentence "Could not save
  // local file", so a save that failed on alternate attempts looked identical
  // to one that failed because permission had lapsed. `NoModificationAllowed`
  // (the file is locked, usually by the previous writable or a sync client)
  // and `NotAllowed` (the grant is gone) are different problems with different
  // fixes, and an author photographing the bar could distinguish neither.
  it('carries the browser\'s own name for a failed write into the reported sentence', async () => {
    const locked: LocalFileHandle = { name: 'locked.folio', getFile: async () => file(), createWritable: async () => { throw new DOMException('The file is locked', 'NoModificationAllowedError') } }
    const access = new FileSystemAccess({ showOpenFilePicker: vi.fn(), showSaveFilePicker: vi.fn() })
    const target = await access.acquireSaveTarget({ suggestedName: 'report.folio', currentTarget: { kind: 'in-place', name: locked.name, handle: locked }, saveAs: false, format: folioFileFormat })
    await expect(access.writeSave(target, { bytes })).rejects.toThrow('Could not save local file: NoModificationAllowedError: The file is locked')
  })

  // An abort is this boundary's own vocabulary, not an unanticipated throw, so
  // it stays a cancellation and grows no cause.
  it('still reports a native abort as a cancellation rather than a described failure', async () => {
    const aborted: LocalFileHandle = { name: 'cancelled.folio', getFile: async () => file(), createWritable: async () => { throw new DOMException('cancel', 'AbortError') } }
    const access = new FileSystemAccess({ showOpenFilePicker: vi.fn(), showSaveFilePicker: vi.fn() })
    const target = await access.acquireSaveTarget({ suggestedName: 'report.folio', currentTarget: { kind: 'in-place', name: aborted.name, handle: aborted }, saveAs: false, format: folioFileFormat })
    await expect(access.writeSave(target, { bytes })).rejects.toThrow('Local file selection was cancelled')
  })

  it('keeps each denied, stale, write, and close failure as a failed local save', async () => {
    const failedHandles: LocalFileHandle[] = [
      { name: 'denied.folio', getFile: async () => file(), createWritable: async () => { throw new Error('denied') } },
      { name: 'stale.folio', getFile: async () => file(), createWritable: async () => ({ write: async () => { throw new Error('stale') }, close: async () => undefined }) },
      { name: 'close.folio', getFile: async () => file(), createWritable: async () => ({ write: async () => undefined, close: async () => { throw new Error('close') } }) },
    ]
    for (const failed of failedHandles) {
      const access = new FileSystemAccess({ showOpenFilePicker: vi.fn(), showSaveFilePicker: vi.fn() })
      const target = await access.acquireSaveTarget({ suggestedName: 'report.folio', currentTarget: { kind: 'in-place', name: failed.name, handle: failed }, saveAs: false, format: folioFileFormat })
      await expect(access.writeSave(target, { bytes })).rejects.toThrow('Could not save local file')
    }
  })

  it('downloads exact opaque bytes without inventing an overwrite target', async () => {
    const anchor = { href: '', download: '', style: { display: '' }, click: vi.fn(), remove: vi.fn() }
    const fakeDocument = { body: { append: vi.fn() }, createElement: vi.fn(() => anchor) } as unknown as Document
    let captured: Blob | undefined
    const url = { createObjectURL: vi.fn((blob: Blob) => { captured = blob; return 'blob:local' }), revokeObjectURL: vi.fn() }
    const access = new InputDownloadAccess(fakeDocument, url)
    const target = await access.acquireSaveTarget({ suggestedName: 'report', saveAs: false, format: folioFileFormat })
    await expect(access.writeSave(target, { bytes })).resolves.toEqual({ name: 'report.folio' })
    expect(anchor.download).toBe('report.folio')
    expect(anchor.click).toHaveBeenCalledOnce()
    expect(new Uint8Array(await captured!.arrayBuffer())).toEqual(new Uint8Array(bytes))
  })

  it('uses an accept-filtered input and accepts a same-file selection as a fresh opaque open', async () => {
    const access = new InputDownloadAccess()
    const opening = access.open()
    const input = document.body.querySelector<HTMLInputElement>('input[type="file"]')!
    expect(input.accept).toContain('.folio')
    Object.defineProperty(input, 'files', { configurable: true, value: { item: () => file() } })
    input.dispatchEvent(new Event('change'))
    await expect(opening).resolves.toMatchObject({ name: 'report.folio' })
    expect(document.body.querySelector('input[type="file"]')).toBeNull()
  })

  it('acquires a native save picker before any serialization/write work', async () => {
    const events: string[] = []
    const picker = { showOpenFilePicker: vi.fn(), showSaveFilePicker: vi.fn(async () => { events.push('picker'); return handle('picked.folio', events) }) }
    const access = new FileSystemAccess(picker)
    const target = await access.acquireSaveTarget({ suggestedName: 'picked', saveAs: true, format: folioFileFormat })
    await access.writeSave(target, { bytes })
    expect(events).toEqual(['picker', 'write:0,255,7', 'close'])
  })

  // STORY 13.1 — THE SAME BOUNDARY, CARRYING A SECOND FORMAT.
  //
  // Three `.folio` hardcodings became one parameter: the suggested name, the
  // picker's accept entry, and the download blob's MIME. Each is asserted here
  // over BOTH formats, because a story that carried only two of the three
  // ships a PDF named `.folio` or a `.pdf` written as `application/json`.

  it('names a save for the format it is being written as, stripping at most one suffix belonging to either known format', () => {
    expect(localFileName('report', folioFileFormat)).toBe('report.folio')
    expect(localFileName('report', pdfFileFormat)).toBe('report.pdf')
    // A template's own name is what Save PDF suggests from, so the `.folio`
    // suffix must be REPLACED rather than appended to — and the same in the
    // other direction, which is the symmetry the module's comment defends.
    expect(localFileName('report.folio', pdfFileFormat)).toBe('report.pdf')
    expect(localFileName('report.pdf', folioFileFormat)).toBe('report.folio')
    // ⚠ THE IDENTITY CASE, PINNED. Strip-then-append must be a NO-OP here, and
    // this is the case a future refactor breaks silently: `report`, or
    // `report.folio.folio`, would both look plausible in a diff.
    expect(localFileName('report.folio', folioFileFormat)).toBe('report.folio')
    expect(localFileName('report.pdf', pdfFileFormat)).toBe('report.pdf')
    // ⚠ AND THE SHIPPED CASING SURVIVES. This is the input that separates the
    // case-preserving early return from its absence — nothing else here does —
    // and it is not cosmetic: `REPORT.folio` overwrites `REPORT.FOLIO` on a
    // case-insensitive filesystem and forks the author's template into two
    // one-keystroke-apart files on a case-sensitive one.
    expect(localFileName('REPORT.FOLIO', folioFileFormat)).toBe('REPORT.FOLIO')
    // ⚠ THE STRIPS DO NOT CHAIN. Folding over the format list took `.pdf` off
    // and then `.folio` off what was left, so this returned
    // `report.folio.folio`. One trailing extension, one slice.
    expect(localFileName('report.folio.pdf', folioFileFormat)).toBe('report.folio')
    expect(localFileName('report.folio.pdf', pdfFileFormat)).toBe('report.folio.pdf')
    expect(localFileName('report.pdf.folio', pdfFileFormat)).toBe('report.pdf')
    // A name that is ALREADY a `.folio` file whose stem happens to end in `.pdf`
    // keeps both: one extension is removed, never two, so the stem survives.
    expect(localFileName('report.pdf.folio', folioFileFormat)).toBe('report.pdf.folio')
    // ⚠ THE UNTITLED FALLBACK RUNS AFTER THE STRIP. It used to run before, so a
    // name that is nothing BUT an extension became the stem-less dotfile `.pdf`.
    expect(localFileName('.folio', pdfFileFormat)).toBe('untitled.pdf')
    expect(localFileName('.pdf', folioFileFormat)).toBe('untitled.folio')
    // ...on the CROSS-format path only. A `.folio` saved as a `.folio` returns
    // unchanged, which is the behaviour `folio8Name` shipped; changing it would
    // be a third unrequested change to `.folio` naming.
    expect(localFileName('.folio', folioFileFormat)).toBe('.folio')
    expect(localFileName('   ', pdfFileFormat)).toBe('untitled.pdf')
    // A dot that is not a known extension is part of the name.
    expect(localFileName('quarterly.summary', pdfFileFormat)).toBe('quarterly.summary.pdf')
    expect(localFileName('  a/b\\c.folio  ', pdfFileFormat)).toBe('abc.pdf')
    // ⚠ CASE-FOLDED ON BOTH SIDES. Both shipped extensions are lowercase, so a
    // bare comparison passes today by coincidence rather than by rule. A format
    // that declares its extension in upper case must still recognise its own
    // suffix instead of double-suffixing forever.
    const shouting: LocalFileFormat = { description: 'PDF document', mimeType: 'application/pdf', extension: '.PDF' }
    expect(localFileName('report.pdf.pdf', shouting)).toBe('report.pdf.pdf')
    expect(localFileName('report.PDF', pdfFileFormat)).toBe('report.PDF')
    // Recognising a suffix across cases is not the same as rewriting one: a
    // cross-format save still restyles the extension it appends.
    expect(localFileName('REPORT.FOLIO', pdfFileFormat)).toBe('REPORT.pdf')
    // ⚠ STORY 5 — `.json` IS NOT IN THE STRIP SET, AND THESE THREE ROWS ARE WHAT
    // SAYS SO. Save sample data introduced a third format; putting it in
    // `knownFileFormats` would have quietly RENAMED the other two save paths,
    // because a template can genuinely be titled `data.json` (the download
    // tier's open input accepts `application/json`, and the title comes from the
    // opened file's name). The first two rows are the shipped answers that were
    // invisible until now; the third is the sample save, which needs no strip
    // because its own suffix takes the case-preserving early return.
    expect(localFileName('data.json', folioFileFormat)).toBe('data.json.folio')
    expect(localFileName('data.json', pdfFileFormat)).toBe('data.json.pdf')
    expect(localFileName('ledger.json', jsonSampleFileFormat)).toBe('ledger.json')
    expect(localFileName('ledger', jsonSampleFileFormat)).toBe('ledger.json')
    // The three shipped formats, pinned: the picker entry and the blob MIME
    // below are both derived from these, so a wrong value here is a wrong file
    // there — and `sample-file.ts` builds its OPEN picker type from the third.
    expect(folioFileFormat).toEqual({ description: 'folio8 template', mimeType: 'application/json', extension: '.folio' })
    expect(pdfFileFormat).toEqual({ description: 'PDF document', mimeType: 'application/pdf', extension: '.pdf' })
    expect(jsonSampleFileFormat).toEqual({ description: 'JSON sample data', mimeType: 'application/json', extension: '.json' })
  })

  it('offers a PDF-only native picker entry for a PDF save and writes then closes the exact bytes it was handed', async () => {
    const events: string[] = []
    const picked = handle('statement.pdf', events)
    const picker = { showOpenFilePicker: vi.fn(), showSaveFilePicker: vi.fn(async () => { events.push('picker'); return picked }) }
    const access = new FileSystemAccess(picker)
    const target = await access.acquireSaveTarget({ suggestedName: 'statement.folio', saveAs: true, format: pdfFileFormat })
    expect(picker.showSaveFilePicker).toHaveBeenCalledWith({ suggestedName: 'statement.pdf', types: [{ description: 'PDF document', accept: { 'application/pdf': ['.pdf'] } }] })
    // The acquired target CARRIES the format, so the write cannot disagree with
    // the picker that named it.
    expect(target.format).toBe(pdfFileFormat)
    await expect(access.writeSave(target, { bytes: pdfBytes })).resolves.toMatchObject({ name: 'statement.pdf', target: { handle: picked } })
    expect(events).toEqual(['picker', `write:${pdfByteList}`, 'close'])
  })

  it('keeps the native open and template-save entries on the .folio format', async () => {
    const picker = { showOpenFilePicker: vi.fn(async () => [handle()]), showSaveFilePicker: vi.fn(async () => handle('kept.folio')) }
    const access = new FileSystemAccess(picker)
    await access.open()
    expect(picker.showOpenFilePicker).toHaveBeenCalledWith({ multiple: false, types: [{ description: 'folio8 template', accept: { 'application/json': ['.folio'] } }] })
    await access.acquireSaveTarget({ suggestedName: 'kept', saveAs: true, format: folioFileFormat })
    expect(picker.showSaveFilePicker).toHaveBeenCalledWith({ suggestedName: 'kept.folio', types: [{ description: 'folio8 template', accept: { 'application/json': ['.folio'] } }] })
  })

  it('downloads a PDF under its own suffix and MIME while the template download keeps its own', async () => {
    const download = async (suggestedName: string, format: typeof pdfFileFormat) => {
      const anchor = { href: '', download: '', style: { display: '' }, click: vi.fn(), remove: vi.fn() }
      const fakeDocument = { body: { append: vi.fn() }, createElement: vi.fn(() => anchor) } as unknown as Document
      let captured: Blob | undefined
      const url = { createObjectURL: vi.fn((blob: Blob) => { captured = blob; return 'blob:local' }), revokeObjectURL: vi.fn() }
      const access = new InputDownloadAccess(fakeDocument, url)
      const target = await access.acquireSaveTarget({ suggestedName, saveAs: true, format })
      const saved = await access.writeSave(target, { bytes: pdfBytes })
      return { anchor, captured: captured!, saved, target }
    }
    const pdf = await download('statement.folio', pdfFileFormat)
    expect(pdf.target).toEqual({ name: 'statement.pdf', format: pdfFileFormat })
    expect(pdf.saved).toEqual({ name: 'statement.pdf' })
    expect(pdf.anchor.download).toBe('statement.pdf')
    expect(pdf.captured.type).toBe('application/pdf')
    expect(Array.from(new Uint8Array(await pdf.captured.arrayBuffer())).join(',')).toBe(pdfByteList)
    // THE CONTROL. Same tier, same bytes, the other format — so the two claims
    // above are about the format parameter rather than about this tier.
    const folio8 = await download('statement.folio', folioFileFormat)
    expect(folio8.anchor.download).toBe('statement.folio')
    expect(folio8.captured.type).toBe('application/json')
  })

  it('revokes the object URL and removes the anchor after every post-creation failure', async () => {
    const anchor = { href: '', download: '', style: { display: '' }, click: vi.fn(() => { throw new Error('blocked') }), remove: vi.fn() }
    const fakeDocument = { body: { append: vi.fn() }, createElement: vi.fn(() => anchor) } as unknown as Document
    const url = { createObjectURL: vi.fn(() => 'blob:local'), revokeObjectURL: vi.fn() }
    const access = new InputDownloadAccess(fakeDocument, url)
    const target = await access.acquireSaveTarget({ suggestedName: 'report', saveAs: false, format: folioFileFormat })
    await expect(access.writeSave(target, { bytes })).rejects.toThrow('Could not download local file')
    await Promise.resolve()
    expect(anchor.remove).toHaveBeenCalledOnce()
    expect(url.revokeObjectURL).toHaveBeenCalledWith('blob:local')
  })
})
