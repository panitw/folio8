import { type FileAccess } from './file-access'
import { FileSystemAccess, type FileSystemPicker } from './file-system-access'
import { InputDownloadAccess } from './input-download'
import type { DownloadUrl } from './input-download'
import { FileSystemSampleAccess, InputSampleAccess, type SampleFileAccess, type SamplePicker } from '../sample-file'
import { FileSystemImageAccess, InputImageAccess, type ImageFileAccess, type ImagePicker } from '../image-file'
import { FileSystemFontAccess, InputFontAccess, type FontFileAccess, type FontPicker } from '../font-file'

export type FileAccessBrowser = Partial<FileSystemPicker> & Readonly<{ document: Document; url: DownloadUrl }>

function currentBrowser(): FileAccessBrowser {
  const pickerWindow = window as typeof window & Partial<FileSystemPicker>
  // The pickers are Window methods and the browser brand-checks their receiver.
  // Bind them to the window here, where it is still in hand; carrying the bare
  // references out on this record would strand them on a plain object.
  return { document: window.document, url: URL, showOpenFilePicker: pickerWindow.showOpenFilePicker?.bind(pickerWindow), showSaveFilePicker: pickerWindow.showSaveFilePicker?.bind(pickerWindow) }
}

// Called once from the composition root. The application receives only FileAccess.
export function selectFileAccess(browser: FileAccessBrowser = currentBrowser()): FileAccess {
  if (typeof browser.showOpenFilePicker === 'function' && typeof browser.showSaveFilePicker === 'function') {
    return new FileSystemAccess({ showOpenFilePicker: browser.showOpenFilePicker.bind(browser), showSaveFilePicker: browser.showSaveFilePicker.bind(browser) })
  }
  return new InputDownloadAccess(browser.document, browser.url)
}

// Sample selection shares the same single capability decision as template
// selection, but exposes no template target, save, or document semantics.
export function selectSampleFileAccess(browser: FileAccessBrowser = currentBrowser()): SampleFileAccess {
  if (typeof browser.showOpenFilePicker === 'function') return new FileSystemSampleAccess({ showOpenFilePicker: browser.showOpenFilePicker.bind(browser) } as SamplePicker)
  return new InputSampleAccess(browser.document)
}

// Image selection shares the same single capability decision as template and
// sample selection, exposing no save, handle retention or document
// semantics (AD-20). currentBrowser() already binds the real picker methods
// to window before they leave it — the receiver-binding defect eef7fbb fixed
// (this file previously bound `this` to a plain object literal, and Chrome's
// receiver brand-check turned every call into "Illegal invocation") — so
// reusing it here, rather than re-copying the picker onto a literal, is what
// keeps this tier from repeating that defect.
export function selectImageFileAccess(browser: FileAccessBrowser = currentBrowser()): ImageFileAccess {
  if (typeof browser.showOpenFilePicker === 'function') return new FileSystemImageAccess({ showOpenFilePicker: browser.showOpenFilePicker.bind(browser) } as ImagePicker)
  return new InputImageAccess(browser.document)
}

// Font-file selection is the same single capability decision a third time, and
// it is here rather than in `font-file.ts` for the reason
// `file-access-contract.test.ts` enforces: capability selection lives in ONE
// composition seam, so no module may test for a picker of its own. The picker
// is bound through `currentBrowser()` exactly as the image tier is — that is
// what keeps this tier from repeating eef7fbb's receiver-binding defect.
//
// ⚠ A PICKER IS NOT THE LOCAL FONT ACCESS API. This selects between two ways
// of letting the author hand over a FILE THEY CHOSE; nothing here enumerates
// the machine's installed fonts. See `font-file.ts`.
// `as unknown as` RATHER THAN A DIRECT CAST, AND THE REASON IS `multiple`.
// `FileSystemPicker`'s declaration types the picker for the single-file tiers
// (`multiple: false`); the font tier asks for `multiple: true`, which the real
// browser method accepts and the narrowed local type does not overlap with. The
// widening is stated here, at the one composition seam, rather than by loosening
// `FontPicker` to `multiple: boolean` — which would have let a single-file
// picker satisfy the font tier's type and silently import one cut of a four-cut
// family.
export function selectFontFileAccess(browser: FileAccessBrowser = currentBrowser()): FontFileAccess {
  if (typeof browser.showOpenFilePicker === 'function') return new FileSystemFontAccess({ showOpenFilePicker: browser.showOpenFilePicker.bind(browser) } as unknown as FontPicker)
  return new InputFontAccess(browser.document)
}
