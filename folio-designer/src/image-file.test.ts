import { describe, expect, it, vi } from 'vitest'
import { selectImageFileAccess, type FileAccessBrowser } from './file/capability'
import { FileAccessCancelled, type LocalFileHandle } from './file/file-access'
import { FileSystemImageAccess, InputImageAccess, type ImagePicker } from './image-file'

const handleFor = (file: File): LocalFileHandle => ({ name: file.name, getFile: async () => file, createWritable: async () => ({ write: async () => undefined, close: async () => undefined }) })

describe('local image-file boundary', () => {
  it('passes opaque bytes and the browser-declared media type through the File System Access picker seam, with no size pre-rejection', async () => {
    const bytes = new Uint8Array([0x89, 0x50, 0x4e, 0x47]).buffer
    const file = { name: 'logo.png', type: 'image/png', size: 200 * 1024 * 1024, arrayBuffer: vi.fn(async () => bytes) } as unknown as File
    const access = new FileSystemImageAccess({ showOpenFilePicker: vi.fn(async () => [handleFor(file)]) })
    await expect(access.openImage()).resolves.toEqual({ name: 'logo.png', mediaType: 'image/png', bytes })
  })

  it('rejects cancellation from the File System Access tier as FileAccessCancelled', async () => {
    const access = new FileSystemImageAccess({ showOpenFilePicker: vi.fn(async () => []) })
    await expect(access.openImage()).rejects.toBeInstanceOf(FileAccessCancelled)
  })

  it('restricts the File System Access picker to png/jpeg — the media types image.go recognises today', async () => {
    const showOpenFilePicker = vi.fn<ImagePicker['showOpenFilePicker']>(async () => [])
    const access = new FileSystemImageAccess({ showOpenFilePicker })
    await access.openImage().catch(() => undefined)
    const options = showOpenFilePicker.mock.calls[0]![0]
    expect(options.multiple).toBe(false)
    expect(options.types).toEqual([{ description: 'Image', accept: { 'image/png': ['.png'], 'image/jpeg': ['.jpg', '.jpeg'] } }])
  })

  it('passes opaque bytes and media type through the <input type=file> fallback tier', async () => {
    const bytes = new Uint8Array([0xff, 0xd8]).buffer
    const file = { name: 'logo.jpg', type: 'image/jpeg', arrayBuffer: vi.fn(async () => bytes) } as unknown as File
    const access = new InputImageAccess(document)
    const pending = access.openImage()
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    expect(input.accept).toBe('.png,.jpg,.jpeg,image/png,image/jpeg')
    Object.defineProperty(input, 'files', { value: { item: () => file, length: 1 }, configurable: true })
    input.dispatchEvent(new Event('change'))
    await expect(pending).resolves.toEqual({ name: 'logo.jpg', mediaType: 'image/jpeg', bytes })
  })

  it('rejects cancellation from the <input type=file> fallback tier', async () => {
    const access = new InputImageAccess(document)
    const pending = access.openImage()
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    input.dispatchEvent(new Event('cancel'))
    await expect(pending).rejects.toBeInstanceOf(FileAccessCancelled)
  })

  it('selects exactly one capability tier from complete picker capabilities and falls back for incomplete APIs', () => {
    const complete = { document, url: { createObjectURL: vi.fn(), revokeObjectURL: vi.fn() }, showOpenFilePicker: vi.fn() }
    expect(selectImageFileAccess(complete)).toBeInstanceOf(FileSystemImageAccess)
    expect(selectImageFileAccess({ document, url: { createObjectURL: vi.fn(), revokeObjectURL: vi.fn() } })).toBeInstanceOf(InputImageAccess)
  })

  it('invokes the real window picker with the window as its receiver (eef7fbb precedent)', async () => {
    // Story 5.13's own instruction: add a unit test asserting the new
    // picker's receiver, following eef7fbb's precedent — that commit's
    // defect was invisible to vi.fn() doubles because they carry no brand
    // check, so only a real receiver-recording function catches it.
    const pickerWindow = window as typeof window & { showOpenFilePicker?: unknown }
    const prior = pickerWindow.showOpenFilePicker
    const receivers: unknown[] = []
    pickerWindow.showOpenFilePicker = function (this: unknown) { receivers.push(this); return Promise.resolve([]) }
    try {
      await expect(selectImageFileAccess().openImage()).rejects.toBeInstanceOf(FileAccessCancelled)
      expect(receivers).toEqual([window])
    } finally { pickerWindow.showOpenFilePicker = prior }
  })

  it("binds the picker to selectImageFileAccess's OWN browser argument, not just currentBrowser()'s default (Finding 8)", async () => {
    // The test above calls selectImageFileAccess() with its currentBrowser()
    // default, which already binds showOpenFilePicker to window
    // (capability.ts:15) — re-binding an already-bound function is a no-op,
    // so that test cannot see capability.ts:42's OWN `.bind(browser)` at
    // all. Pass an EXPLICIT browser here, bypassing currentBrowser()
    // entirely, with an unbound receiver-recording function, so the only
    // thing that can make the receiver come out as `explicitBrowser` is
    // line 42's own bind. Deleting `.bind(browser)` from line 42 reddens
    // this test while leaving the test above green.
    const receivers: unknown[] = []
    const explicitBrowser: FileAccessBrowser = {
      document,
      url: { createObjectURL: vi.fn(), revokeObjectURL: vi.fn() },
      showOpenFilePicker: function (this: unknown) { receivers.push(this); return Promise.resolve([]) },
    }
    await expect(selectImageFileAccess(explicitBrowser).openImage()).rejects.toBeInstanceOf(FileAccessCancelled)
    expect(receivers).toEqual([explicitBrowser])
  })
})
