import { describe, expect, it, vi } from 'vitest'
import { MAX_ENGINE_PAYLOAD_BYTES } from './engine-protocol'
import type { LocalFileHandle } from './file/file-access'
import { FileSystemSampleAccess } from './sample-file'

const handleFor = (file: File): LocalFileHandle => ({ name: file.name, getFile: async () => file, createWritable: async () => ({ write: async () => undefined, close: async () => undefined }) })

describe('local sample-file boundary', () => {
  it('checks File.size before reading a selected file', async () => {
    const arrayBuffer = vi.fn()
    const file = { name: 'oversized.json', size: MAX_ENGINE_PAYLOAD_BYTES + 1, arrayBuffer } as unknown as File
    const access = new FileSystemSampleAccess({ showOpenFilePicker: vi.fn(async () => [handleFor(file)]) })
    await expect(access.openSample()).rejects.toThrow('local preview limit')
    expect(arrayBuffer).not.toHaveBeenCalled()
  })

  it('passes opaque accepted bytes and only local metadata through the picker seam', async () => {
    const bytes = new Uint8Array([123, 125]).buffer
    const file = { name: 'sample.json', size: bytes.byteLength, arrayBuffer: vi.fn(async () => bytes) } as unknown as File
    const showOpenFilePicker = vi.fn(async () => [handleFor(file)])
    const access = new FileSystemSampleAccess({ showOpenFilePicker })
    await expect(access.openSample()).resolves.toEqual({ name: 'sample.json', bytes })
    // ⚠ STORY 5 — THE PICKER'S ARGUMENT, PINNED, because it stopped being a
    // literal. `samplePickerType` is now DERIVED from `jsonSampleFileFormat`
    // (one wording for opening a sample and for saving one), and a derivation
    // that transposed the MIME and the extension — `{ '.json':
    // ['application/json'] }` — would type-check and ship green. The value below
    // is what the author's file dialog actually filters on.
    expect(showOpenFilePicker).toHaveBeenCalledWith({ multiple: false, types: [{ description: 'JSON sample data', accept: { 'application/json': ['.json'] } }] })
  })
})
