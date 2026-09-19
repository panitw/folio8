// Builds folio-js's engine: folio-go/wasm/cmd/render compiled to js/wasm with
// the go.mod toolchain, plus that toolchain's wasm_exec.js glue, into wasm/.
// The command and the glue lookup follow folio-designer/scripts/build-wasm.mjs.
import { copyFileSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const packageRoot = join(dirname(fileURLToPath(import.meta.url)), '..')
const goModuleRoot = join(packageRoot, '..', 'folio-go')
const outputDir = join(packageRoot, 'wasm')

// Run `go env` inside the module so GOTOOLCHAIN=auto resolves the go.mod
// toolchain, whose GOROOT carries the matching wasm_exec.js.
const goRoot = execFileSync('go', ['env', 'GOROOT'], { cwd: goModuleRoot, encoding: 'utf8' }).trim()
const wasmExec = [join(goRoot, 'lib', 'wasm', 'wasm_exec.js'), join(goRoot, 'misc', 'wasm', 'wasm_exec.js')].find(existsSync)
if (!wasmExec) throw new Error(`wasm_exec.js not found below ${goRoot}`)

rmSync(outputDir, { recursive: true, force: true })
mkdirSync(outputDir, { recursive: true })
// -trimpath MATTERS HERE, unlike in the designer's build. The engine folio-js
// publishes is ONE artifact that every Node platform loads, so it must not
// depend on the machine that built it: without -trimpath the binary embeds the
// build host's module and GOROOT paths, and CI proved the consequence — the six
// matrix legs produced three different engines, one per runner OS, from
// identical source. The rendered PDFs agreed; the artifact did not.
execFileSync('go', ['build', '-trimpath', '-buildvcs=false', '-o', join(outputDir, 'folio8-render.wasm'), './wasm/cmd/render'], {
  cwd: goModuleRoot,
  env: { ...process.env, GOOS: 'js', GOARCH: 'wasm' },
  stdio: 'inherit',
})
copyFileSync(wasmExec, join(outputDir, 'wasm_exec.js'))
