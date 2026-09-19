// A MINIMAL sfnt WITH A `name` TABLE, BUILT BY HAND, for the tests that have to
// ask what happens when a face says something unusual about itself.
//
// The 21 committed faces are the corpus for "does the reader agree with real
// bytes"; they cannot be the corpus for "a face with no nameID 0", "a face with
// no name table at all", or "a Macintosh-platform record", because every
// committed face is well-formed. Those cases are synthesised here rather than
// asserted about hypothetically.
//
// Not a font: it has no glyphs, no `head`, no `OS/2`. It is exactly enough sfnt
// for `src/font-name-table.ts` to walk, which is what these tests are about.

export type NameRecord = Readonly<{ platform: number; encoding?: number; language?: number; nameID: number; value: string }>

// UTF-16 CODE UNITS, BIG-ENDIAN — the units, not the code points, because that
// is exactly what the `name` table stores and what the reader decodes back.
const utf16BE = (value: string): number[] => {
  const bytes: number[] = []
  for (let index = 0; index < value.length; index++) {
    const unit = value.charCodeAt(index)
    bytes.push((unit >> 8) & 0xff, unit & 0xff)
  }
  return bytes
}
const latin1 = (value: string): number[] => [...value].map((character) => character.charCodeAt(0) & 0xff)

/** `true` for the Unicode/Windows platforms, which store UTF-16BE. */
const isUnicodePlatform = (platform: number) => platform === 3 || platform === 0

export function sfntWithNames(records: ReadonlyArray<NameRecord>, options: Readonly<{ sfntVersion?: number; omitNameTable?: boolean }> = {}): ArrayBuffer {
  const encoded = records.map((record) => ({ record, bytes: isUnicodePlatform(record.platform) ? utf16BE(record.value) : latin1(record.value) }))
  const storage: number[] = []
  const offsets = encoded.map(({ bytes }) => { const at = storage.length; storage.push(...bytes); return at })

  const nameTable: number[] = []
  const push16 = (target: number[], value: number) => { target.push((value >> 8) & 0xff, value & 0xff) }
  push16(nameTable, 0)
  push16(nameTable, encoded.length)
  push16(nameTable, 6 + encoded.length * 12)
  encoded.forEach(({ record, bytes }, index) => {
    push16(nameTable, record.platform)
    push16(nameTable, record.encoding ?? (isUnicodePlatform(record.platform) ? 1 : 0))
    push16(nameTable, record.language ?? 0)
    push16(nameTable, record.nameID)
    push16(nameTable, bytes.length)
    push16(nameTable, offsets[index])
  })
  nameTable.push(...storage)

  const tables = options.omitNameTable ? [] : [{ tag: 'name', data: nameTable }]
  const directorySize = 12 + tables.length * 16
  const out: number[] = []
  const push32 = (value: number) => out.push((value >>> 24) & 0xff, (value >>> 16) & 0xff, (value >>> 8) & 0xff, value & 0xff)
  push32(options.sfntVersion ?? 0x00010000)
  push16(out, tables.length)
  push16(out, 0)
  push16(out, 0)
  push16(out, 0)
  let offset = directorySize
  for (const table of tables) {
    for (const character of table.tag) out.push(character.charCodeAt(0))
    push32(0)
    push32(offset)
    push32(table.data.length)
    offset += table.data.length
  }
  for (const table of tables) out.push(...table.data)
  return new Uint8Array(out).buffer
}

/** The commonest shape: one Windows-platform copyright record and nothing else. */
export const sfntWithCopyright = (copyright: string): ArrayBuffer => sfntWithNames([{ platform: 3, nameID: 0, value: copyright }])
