import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

export const repoRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..')

// folio8-go/fonts/fonts.go's Shipped(): face name to embedded file.
const shippedFaces: Record<string, string> = {
  'Noto Sans': 'notosans/NotoSans-Regular.ttf',
  'Noto Sans Bold': 'notosans-bold/NotoSans-Bold.ttf',
  'Noto Sans Italic': 'notosans-italic/NotoSans-Italic.ttf',
  'Noto Sans Bold Italic': 'notosans-bolditalic/NotoSans-BoldItalic.ttf',
  'Noto Sans Thai': 'notosansthai/NotoSansThai-Regular.ttf',
  'Noto Sans Thai Bold': 'notosansthai-bold/NotoSansThai-Bold.ttf',
  'Noto Sans SC': 'notosanssc/NotoSansSC-Regular.ttf',
  Roboto: 'roboto/Roboto-Regular.ttf',
  'Roboto Bold': 'roboto-bold/Roboto-Bold.ttf',
  'Roboto Italic': 'roboto-italic/Roboto-Italic.ttf',
  'Roboto Bold Italic': 'roboto-bolditalic/Roboto-BoldItalic.ttf',
}

let fonts: Map<string, Uint8Array> | undefined

/** The same Map fonts.Shipped() would give a Go caller. */
export function shippedFonts(): Map<string, Uint8Array> {
  fonts ??= new Map(Object.entries(shippedFaces).map(([name, file]) => [name, new Uint8Array(readFileSync(join(repoRoot, 'folio8-go', 'fonts', file)))]))
  return fonts
}

export function repoFile(path: string): Uint8Array {
  return new Uint8Array(readFileSync(join(repoRoot, path)))
}

export function sha256(bytes: Uint8Array): string {
  return createHash('sha256').update(bytes).digest('hex')
}
