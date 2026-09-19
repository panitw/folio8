import { FileAccessCancelled, FileAccessFailure, type LocalFileHandle } from './file/file-access'

// Story 5.13's local-image read: a narrow, read-bytes-only interface with no
// save, no handle retention and no document semantics — the same shape
// sample-file.ts already established for sample JSON (AD-20). The browser
// does not decide whether a picked file is a legal asset: mediaType here is
// only the browser/OS's OWN declared value (File.type), carried opaquely to
// the engine command, which is the sole authority on recognising it (AC1).
export type LocalImageFile = Readonly<{ bytes: ArrayBuffer; mediaType: string; name: string }>
export interface ImageFileAccess { openImage(): Promise<LocalImageFile> }
export type ImagePicker = Readonly<{ showOpenFilePicker(options: Readonly<{ multiple: false; types: ReadonlyArray<Readonly<{ description: string; accept: Readonly<Record<string, ReadonlyArray<string>>> }>> }>): Promise<ReadonlyArray<LocalFileHandle>> }>

// Restricted to the media types internal/template/image.go recognises TODAY
// (image/png, image/jpeg) — Task 3's own instruction. This is a picker UX
// convenience only, never the authority: Go alone decides recognition and
// decodability once the command reaches it (AC1), so widening or narrowing
// this list can never change what the engine accepts.
const imagePickerType = { description: 'Image', accept: { 'image/png': ['.png'], 'image/jpeg': ['.jpg', '.jpeg'] } } as const
const imageInputAccept = '.png,.jpg,.jpeg,image/png,image/jpeg'

// D-5.13.4: the browser must not pre-reject on size. Go enforces the host-
// memory bound (maxComponentAssetBytes, component_commands.go); no size
// check exists here, deliberately, unlike sample-file.ts's rejectOversized.

export class FileSystemImageAccess implements ImageFileAccess {
  private readonly picker: ImagePicker
  constructor(picker: ImagePicker) { this.picker = picker }
  async openImage(): Promise<LocalImageFile> {
    try {
      const [handle] = await this.picker.showOpenFilePicker({ multiple: false, types: [imagePickerType] })
      if (!handle) throw new FileAccessCancelled()
      const file = await handle.getFile()
      return { bytes: await file.arrayBuffer(), mediaType: file.type, name: file.name }
    } catch (error) {
      if (error instanceof FileAccessCancelled || error instanceof DOMException && error.name === 'AbortError') throw new FileAccessCancelled()
      throw new FileAccessFailure('Could not read the local image')
    }
  }
}

export class InputImageAccess implements ImageFileAccess {
  private readonly document: Document
  constructor(document: Document = window.document) { this.document = document }
  openImage(): Promise<LocalImageFile> {
    return new Promise((resolve, reject) => {
      const input = this.document.createElement('input')
      input.type = 'file'; input.accept = imageInputAccept; input.value = ''; input.style.display = 'none'
      const cleanup = () => input.remove()
      input.addEventListener('change', () => {
        const file = input.files?.item(0); cleanup()
        if (!file) { reject(new FileAccessCancelled()); return }
        void file.arrayBuffer().then((bytes) => resolve({ bytes, mediaType: file.type, name: file.name }), () => reject(new FileAccessFailure('Could not read the local image')))
      }, { once: true })
      input.addEventListener('cancel', () => { cleanup(); reject(new FileAccessCancelled()) }, { once: true })
      this.document.body.append(input); input.click()
    })
  }
}
