import { MAX_ENGINE_PAYLOAD_BYTES } from './engine-protocol'
import { FileAccessCancelled, FileAccessFailure, jsonSampleFileFormat, type LocalFileHandle } from './file/file-access'

export type LocalSampleFile = Readonly<{ bytes: ArrayBuffer; name: string }>
export interface SampleFileAccess { openSample(): Promise<LocalSampleFile> }
export type SamplePicker = Readonly<{ showOpenFilePicker(options: Readonly<{ multiple: false; types: ReadonlyArray<Readonly<{ description: string; accept: Readonly<Record<string, ReadonlyArray<string>>> }>> }>): Promise<ReadonlyArray<LocalFileHandle>> }>
// Derived from the shared format rather than re-spelled, so opening a sample and
// saving one name the same type with the same words (story 5's "one wording").
const samplePickerType = { description: jsonSampleFileFormat.description, accept: { [jsonSampleFileFormat.mimeType]: [jsonSampleFileFormat.extension] } } as const
const rejectOversized = (file: File): void => { if (file.size > MAX_ENGINE_PAYLOAD_BYTES) throw new FileAccessFailure(`Selected JSON exceeds the ${MAX_ENGINE_PAYLOAD_BYTES / 1024 / 1024} MiB local preview limit`) }

export class FileSystemSampleAccess implements SampleFileAccess {
  private readonly picker: SamplePicker
  constructor(picker: SamplePicker) { this.picker = picker }
  async openSample(): Promise<LocalSampleFile> {
    try {
      const [handle] = await this.picker.showOpenFilePicker({ multiple: false, types: [samplePickerType] })
      if (!handle) throw new FileAccessCancelled()
      const file = await handle.getFile(); rejectOversized(file)
      return { bytes: await file.arrayBuffer(), name: file.name }
    } catch (error) {
      if (error instanceof FileAccessCancelled || error instanceof DOMException && error.name === 'AbortError') throw new FileAccessCancelled()
      if (error instanceof FileAccessFailure) throw error
      throw new FileAccessFailure('Could not read local sample data')
    }
  }
}

export class InputSampleAccess implements SampleFileAccess {
  private readonly document: Document
  constructor(document: Document = window.document) { this.document = document }
  openSample(): Promise<LocalSampleFile> {
    return new Promise((resolve, reject) => {
      const input = this.document.createElement('input')
      input.type = 'file'; input.accept = '.json,application/json'; input.value = ''; input.style.display = 'none'
      const cleanup = () => input.remove()
      input.addEventListener('change', () => {
        const file = input.files?.item(0); cleanup()
        if (!file) { reject(new FileAccessCancelled()); return }
        try { rejectOversized(file) } catch (error) { reject(error); return }
        void file.arrayBuffer().then((bytes) => resolve({ bytes, name: file.name }), () => reject(new FileAccessFailure('Could not read local sample data')))
      }, { once: true })
      input.addEventListener('cancel', () => { cleanup(); reject(new FileAccessCancelled()) }, { once: true })
      this.document.body.append(input); input.click()
    })
  }
}
