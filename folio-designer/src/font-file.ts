import { FileAccessCancelled, FileAccessFailure, type LocalFileHandle } from './file/file-access'

// THE AUTHOR'S OWN FONT FILES, READ THROUGH THE SEAM `image-file.ts` ALREADY
// ESTABLISHED TWICE OVER.
//
// `src/image-file.ts` solves "let the author pick a file off their machine" in
// two tiers — `FileSystemImageAccess` over the File System Access picker and
// `InputImageAccess` over a hidden `<input type="file">` — chosen once in
// `src/file/capability.ts` and injected as an `App` prop. `sample-file.ts` is
// the same shape for JSON. This is that shape a third time, for font files, and
// it is deliberately NOT a new pattern: the receiver-binding defect eef7fbb
// fixed lives in exactly this seam, and the one way past it is to keep every
// tier selecting through the one capability module.
//
// A FILE PICKER IS NOT THE LOCAL FONT ACCESS API, AND THE DISTINCTION IS THE
// WHOLE PERMISSION MODEL. Nothing here enumerates the fonts installed on the
// machine, asks for a font permission, or learns of any file the author did not
// hand over. `spec-fonts`' *"No host fonts"* non-goal survives untouched, and
// `scripts/host-font-access.mjs` is the tripwire over it — this module names
// none of the four spellings it forbids.
//
// THE ONE DIFFERENCE FROM THE IMAGE SEAM IS `multiple`, AND IT IS THE STORY'S
// OWN REQUIREMENT. An author importing a brand typeface is importing its cuts —
// Regular, Bold, Italic, Bold Italic — and the acknowledgement that admits them
// is ONE gesture per import rather than one per file. A single-file picker
// would make a four-cut family four separate admissions, which is the shape the
// decision explicitly refuses.
export type LocalFontFile = Readonly<{ bytes: ArrayBuffer; mediaType: string; name: string }>

/**
 * THE SEAM. Resolves to every file the author picked, in the order the picker
 * gave them; rejects with `FileAccessCancelled` when they picked none.
 *
 * `name` TRAVELS ONLY AS FAR AS A REFUSAL SENTENCE. The import path reads a
 * face's family and cut out of the BINARY and never out of this field — see
 * `font-import.ts` — and no filename, path or machine identity reaches the
 * store or a document. It is carried because a refusal that cannot say WHICH
 * of four files was refused is not a refusal the author can act on.
 */
export interface FontFileAccess { openFonts(): Promise<ReadonlyArray<LocalFontFile>> }

export type FontPicker = Readonly<{ showOpenFilePicker(options: Readonly<{ multiple: true; types: ReadonlyArray<Readonly<{ description: string; accept: Readonly<Record<string, ReadonlyArray<string>>> }>> }>): Promise<ReadonlyArray<LocalFileHandle>> }>

// THE TWO FACE CONTAINERS THIS PRODUCT READS, AND THE SAME TWO `fontdir` READS
// ON THE HOST SIDE (`fontExtensions`). A `.woff`/`.woff2` has no entry here for
// the reason `font-source.ts`'s own media-type table gives: it is a web wrapper,
// not an sfnt face this engine can embed or measure.
//
// THIS IS A PICKER CONVENIENCE AND NEVER THE AUTHORITY. The per-file check in
// `font-import.ts` refuses on the same table, and Go refuses again at the
// command, so widening or narrowing this list can never change what is admitted.
const fontPickerType = { description: 'Font', accept: { 'font/ttf': ['.ttf'], 'font/otf': ['.otf'] } } as const
const fontInputAccept = '.ttf,.otf,font/ttf,font/otf'

export class FileSystemFontAccess implements FontFileAccess {
  private readonly picker: FontPicker
  constructor(picker: FontPicker) { this.picker = picker }
  async openFonts(): Promise<ReadonlyArray<LocalFontFile>> {
    try {
      const handles = await this.picker.showOpenFilePicker({ multiple: true, types: [fontPickerType] })
      if (handles.length === 0) throw new FileAccessCancelled()
      const picked: LocalFontFile[] = []
      for (const handle of handles) {
        const file = await handle.getFile()
        picked.push({ bytes: await file.arrayBuffer(), mediaType: file.type, name: file.name })
      }
      return picked
    } catch (error) {
      if (error instanceof FileAccessCancelled || error instanceof DOMException && error.name === 'AbortError') throw new FileAccessCancelled()
      throw new FileAccessFailure('Could not read the font files you picked')
    }
  }
}

export class InputFontAccess implements FontFileAccess {
  private readonly document: Document
  constructor(document: Document = window.document) { this.document = document }
  openFonts(): Promise<ReadonlyArray<LocalFontFile>> {
    return new Promise((resolve, reject) => {
      const input = this.document.createElement('input')
      input.type = 'file'; input.accept = fontInputAccept; input.multiple = true; input.value = ''; input.style.display = 'none'
      const cleanup = () => input.remove()
      input.addEventListener('change', () => {
        const files = Array.from(input.files ?? []); cleanup()
        if (files.length === 0) { reject(new FileAccessCancelled()); return }
        void Promise.all(files.map(async (file) => ({ bytes: await file.arrayBuffer(), mediaType: file.type, name: file.name })))
          .then(resolve, () => reject(new FileAccessFailure('Could not read the font files you picked')))
      }, { once: true })
      input.addEventListener('cancel', () => { cleanup(); reject(new FileAccessCancelled()) }, { once: true })
      this.document.body.append(input); input.click()
    })
  }
}
