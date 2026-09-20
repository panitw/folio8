import { execFileSync } from 'node:child_process'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

// Go writes its build info into the emitted binary as newline-separated
// `build\t<key>=<value>` records. `go version -m` CANNOT read a wasm binary — it
// answers `unrecognized file format` — so grepping its output for `vcs` prints
// nothing BECAUSE THE TOOL FAILED, not because the stamp is absent. The stamp is
// therefore read from the raw bytes, which is the only reading that is a
// measurement rather than a hope.
//
// This lives apart from build-wasm.mjs because that script is all top-level side
// effects: importing it to test the detector would run a build.
const VCS_SETTINGS = [
  { setting: 'vcs.revision', needle: 'vcs.revision=' },
  { setting: 'vcs.time', needle: 'vcs.time=' },
  { setting: 'vcs.modified', needle: 'vcs.modified=' },
  // The bare `vcs` setting names the version-control system rather than the tree,
  // and `vcs=` is short enough to occur in arbitrary payload bytes, so it counts
  // only where it appears as a build-info record.
  { setting: 'vcs', needle: 'build\tvcs=' },
]

// A stamped value runs to the record's newline; stop at any non-printable byte so
// a needle that landed in binary payload yields a short, readable report instead
// of a screenful of noise.
const readValue = (bytes, start) => {
  let end = start
  while (end < bytes.length && end - start < 200 && bytes[end] >= 0x20 && bytes[end] <= 0x7e) end += 1
  return bytes.toString('latin1', start, end)
}

export function findVCSStampSettings(source) {
  // Every search below is `latin1`, a byte-exact 1:1 mapping, so a needle can
  // never straddle a multi-byte decode. A string source MUST be decoded the same
  // way: Buffer.from's default is UTF-8, which would silently disagree with the
  // searches and shift every offset past the first non-ASCII byte.
  const bytes = Buffer.isBuffer(source) ? source : typeof source === 'string' ? Buffer.from(source, 'latin1') : Buffer.from(source)
  const found = []
  for (const { setting, needle } of VCS_SETTINGS) {
    for (let index = bytes.indexOf(needle, 0, 'latin1'); index !== -1; index = bytes.indexOf(needle, index + needle.length, 'latin1')) {
      found.push({ setting, value: readValue(bytes, index + needle.length) })
    }
  }
  return found
}

export function assertNoVCSStamp(source, label) {
  const found = findVCSStampSettings(source)
  if (found.length === 0) return
  throw new Error(`${label} carries a Go build-info VCS stamp, so its bytes are a function of the working tree rather than of the source: ${found.map(({ setting, value }) => `${setting}=${value}`).join(', ')}. Build it with -buildvcs=false.`)
}

// THE ENGINE'S COMPILE, SPELLED OUT ONCE. build-wasm.mjs calls this so
// `-buildvcs=false` is declared in exactly one place; verify-offline-release.mjs
// calls it with the flag dropped, to prove the detector above still
// discriminates against a real stamped binary (DW-107), and twice with the flag,
// to prove the PROPERTY the flag buys (DW-106). A second copy of this argv would
// be precisely the drift the flag exists to close.
// AND THE TAG THAT KEEPS THE CJK FACE OUT OF IT (spec-deferred-offline-cache,
// CAP-6). `nocjkface` selects folio-go/fonts/notosanssc_absent.go over
// notosanssc.go, so the go:embed directive for the 10,595,932-byte SC face is
// never compiled into THIS binary — 4.72 MiB of the 8.11 MiB Brotli sidecar
// every first-time visitor used to wait for, and bytes the release already
// serves a second time as a deferred canvas asset.
//
// ⚠ IT IS DECLARED HERE AND NOWHERE ELSE, for the same reason `-buildvcs=false`
// is: verify-offline-release.mjs builds the engine twice through
// `buildEngineWasm` to prove the bytes are a function of the source, and a
// second copy of this argv would let the verified build and the shipped build
// differ in exactly the way the two-build comparison exists to refuse.
//
// ⚠ AND IT IS THE DESIGNER'S BUILD ONLY. `go build ./...`, `go test ./...`,
// folio-js, folio-dotnet, the CLI and the golden fixtures all compile without
// it and all keep the eleven-face `fonts.Shipped()` untouched.
export const ENGINE_BUILD_FLAGS = ['-buildvcs=false', '-tags', 'nocjkface']
const goModuleRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'folio-go')

export function buildEngineWasm(outputPath, { flags = ENGINE_BUILD_FLAGS, stdio = 'inherit' } = {}) {
  execFileSync('go', ['build', ...flags, '-o', outputPath, './wasm/cmd/engine'], { cwd: goModuleRoot, env: { ...process.env, GOOS: 'js', GOARCH: 'wasm' }, stdio })
  return outputPath
}
