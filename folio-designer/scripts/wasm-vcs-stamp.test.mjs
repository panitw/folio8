import { describe, expect, it } from 'vitest'
import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { assertNoVCSStamp, findVCSStampSettings } from './wasm-vcs-stamp.mjs'

const runtimeDirectory = join(import.meta.dirname, '..', 'src', 'generated', 'runtime')
// The record layout Go actually emits, transcribed from the bytes of a stamped
// `GOOS=js GOARCH=wasm` build rather than from memory.
const stamped = Buffer.concat([
  Buffer.from([0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00]),
  Buffer.from('build\tCGO_ENABLED=0\nbuild\tGOARCH=wasm\nbuild\tGOOS=js\nbuild\tvcs=git\nbuild\tvcs.revision=873757f038f66491aa431dc2b1f015ed42132e42\nbuild\tvcs.time=2026-09-01T09:30:20Z\nbuild\tvcs.modified=true\n'),
  Buffer.from([0xf9, 0x32, 0x43, 0x31]),
])
const unstamped = Buffer.concat([
  Buffer.from([0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00]),
  // Keeps the surrounding build records, and a loose mention of `vcs`, so a
  // detector matching `build\t` or the bare word would redden here.
  Buffer.from('build\tCGO_ENABLED=0\nbuild\tGOARCH=wasm\nbuild\tGOOS=js\nreproducible without vcs provenance\n'),
  Buffer.from([0xf9, 0x32, 0x43, 0x31]),
])

describe('wasm VCS stamp detector', () => {
  it('reports every VCS setting a stamped binary carries, with its value', () => {
    expect(findVCSStampSettings(stamped)).toEqual([
      { setting: 'vcs.revision', value: '873757f038f66491aa431dc2b1f015ed42132e42' },
      { setting: 'vcs.time', value: '2026-09-01T09:30:20Z' },
      { setting: 'vcs.modified', value: 'true' },
      { setting: 'vcs', value: 'git' },
    ])
  })

  it('throws for a stamped binary, naming the settings it found', () => {
    expect(() => assertNoVCSStamp(stamped, 'probe.wasm')).toThrow(/probe\.wasm.*vcs\.revision=873757f0.*vcs\.modified=true/s)
  })

  it('reports a binary with no build-info VCS settings clean', () => {
    expect(findVCSStampSettings(unstamped)).toEqual([])
    expect(() => assertNoVCSStamp(unstamped, 'probe.wasm')).not.toThrow()
  })

  // A literal-driven unit test proves the predicate discriminates; it never proves
  // the predicate admits what actually ships. Tie it to the real population.
  it('finds no VCS stamp in the engine wasm this build emitted', () => {
    // An all-clear must be distinguishable from a couldn't-look: without this, an
    // absent generated tree throws a bare ENOENT and reads as an unrelated fault.
    if (!existsSync(runtimeDirectory)) throw new Error(`${runtimeDirectory} does not exist, so this test could not look at the emitted wasm at all. Run \`npm run build:wasm\` first — \`npm test\` does it for you.`)
    const emitted = readdirSync(runtimeDirectory).filter((file) => /^folio8-engine\.[a-f0-9]{20}\.wasm$/.test(file))
    expect(emitted).toHaveLength(1)
    expect(findVCSStampSettings(readFileSync(join(runtimeDirectory, emitted[0])))).toEqual([])
  })
})
