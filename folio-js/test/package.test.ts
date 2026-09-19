import { execFileSync } from 'node:child_process'
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
// The same list prepack's gate enforces, so the tarball assertion and the
// refusal cannot drift apart.
import { requiredFiles } from '../scripts/package-check.mjs'
import { repoRoot, sha256 } from './helpers.js'

// CAP-6, proved rather than asserted: pack the package, install the tarball
// into an empty project with the network refused and install scripts ignored,
// then run the README's own first-PDF snippet against a corpus fixture and
// compare the PDF's SHA-256 with the fixture's committed expected.json.
const packageRoot = join(repoRoot, 'folio-js')
const fixture = join(repoRoot, 'fixtures', 'alternating-rows')

// The fonts are ~14 MB and the wasm ~12 MB, so packing, installing and the
// first render all take real seconds.
const timeout = 600_000

let workspace: string
let tarball: string
let entries: string[]

/**
 * npm is the one command here that is not a real executable on Windows: it is
 * npm.cmd, which execFileSync cannot spawn without a shell (and modern Node
 * refuses outright, with EINVAL). git, tar and node ARE executables, and
 * CreateProcess appends .exe for them, so only npm needs handling.
 *
 * Running npm-cli.js under this very Node avoids a shell — and so avoids
 * quoting the temp paths these tests pass as arguments. Story 8 runs this
 * suite on the Windows and macOS legs, which is what made it matter.
 */
function npmCli(): string {
  const fromNpm = process.env['npm_execpath']
  if (fromNpm && fromNpm.endsWith('.js') && existsSync(fromNpm)) return fromNpm
  const nodeDir = dirname(process.execPath)
  const candidates = [
    // Windows, and any layout where npm sits beside node.
    join(nodeDir, 'node_modules', 'npm', 'bin', 'npm-cli.js'),
    // POSIX prefix layout: <prefix>/bin/node with <prefix>/lib/node_modules.
    join(nodeDir, '..', 'lib', 'node_modules', 'npm', 'bin', 'npm-cli.js'),
  ]
  const found = candidates.find((path) => existsSync(path))
  if (!found) throw new Error(`cannot locate npm-cli.js beside ${process.execPath}; looked in ${candidates.join(', ')}`)
  return found
}

function run(command: string, args: string[], cwd: string, env: NodeJS.ProcessEnv = {}): string {
  const [file, argv] = command === 'npm' ? [process.execPath, [npmCli(), ...args]] : [command, args]
  return execFileSync(file, argv, { cwd, encoding: 'utf8', env: { ...process.env, ...env }, maxBuffer: 64 * 1024 * 1024 })
}

/** The first ```js block in README.md — the snippet the guide documents. */
function readmeSnippet(): string {
  const readme = readFileSync(join(packageRoot, 'README.md'), 'utf8')
  const match = /```js\n([\s\S]*?)```/.exec(readme)
  if (!match) throw new Error('README.md has no ```js block — the documented first-PDF snippet is gone')
  return match[1]!
}

beforeAll(() => {
  workspace = mkdtempSync(join(tmpdir(), 'folio-js-pack-'))
  // --ignore-scripts skips prepack, which would rebuild the wasm with the Go
  // toolchain: `npm test` runs after `npm run build`, so the tree on disk is
  // already what prepack would produce. prepack's own refusal to pack an
  // incomplete tree is exercised below, against a broken tree, in seconds.
  const packed = JSON.parse(run('npm', ['pack', '--ignore-scripts', '--json', '--pack-destination', workspace], packageRoot)) as { filename: string }[]
  tarball = join(workspace, packed[0]!.filename)
  entries = run('tar', ['-tzf', tarball], workspace)
    // SPLIT ON EITHER LINE ENDING. Windows' bundled bsdtar writes CRLF, so
    // splitting on \n alone leaves a trailing \r on every entry and every
    // name comparison below fails — which is exactly how both Windows legs
    // of the folio-js matrix first went red.
    .split(/\r?\n/)
    .filter(Boolean)
    .map((line) => line.replace(/^package\//, '').replace(/\/$/, ''))
}, timeout)

afterAll(() => {
  if (workspace) rmSync(workspace, { recursive: true, force: true })
})

describe('the packed tarball', () => {
  it('carries the prebuilt engine and the built JavaScript', () => {
    for (const path of requiredFiles) expect(entries).toContain(path)
  })

  it('carries all eleven faces with their licence and notice', () => {
    const manifest = JSON.parse(readFileSync(join(packageRoot, 'fonts', 'manifest.json'), 'utf8')) as { faces: { name: string; file: string }[] }
    expect(manifest.faces).toHaveLength(11)
    for (const face of manifest.faces) {
      expect(entries).toContain(`fonts/${face.file}`)
      expect(entries).toContain(`fonts/${dirname(face.file)}/LICENSE-OFL.txt`)
      expect(entries).toContain(`fonts/${dirname(face.file)}/NOTICE.md`)
    }
    expect(entries.filter((path) => path.endsWith('.ttf'))).toHaveLength(11)
  })

  it('carries no source, no test and no lockfile', () => {
    for (const path of entries) {
      expect(path.startsWith('src/'), `${path} is source`).toBe(false)
      expect(path.startsWith('test/'), `${path} is a test`).toBe(false)
      expect(path.startsWith('scripts/'), `${path} is a build script`).toBe(false)
      // dist/*.d.ts are the published types; TypeScript sources are not.
      expect(path.endsWith('.ts') && !path.endsWith('.d.ts'), `${path} is TypeScript source`).toBe(false)
    }
    expect(entries).not.toContain('package-lock.json')
    expect(entries).not.toContain('tsconfig.json')
    expect(entries).not.toContain('vitest.config.ts')
  })

  it('is built from a tree that tracks no font or wasm bytes', () => {
    const tracked = run('git', ['ls-files', 'folio-js'], repoRoot).split(/\r?\n/).filter(Boolean)
    expect(tracked.filter((path) => path.endsWith('.ttf') || path.endsWith('.wasm'))).toEqual([])
  })
})

describe('install hygiene', () => {
  it('the packed package.json has no dependencies and no install scripts', () => {
    const manifest = JSON.parse(run('tar', ['-xOzf', tarball, 'package/package.json'], workspace)) as {
      private?: boolean
      version: string
      dependencies?: object
      peerDependencies?: object
      optionalDependencies?: object
      scripts: Record<string, string>
      folio8EngineVersion: string
    }
    expect(manifest.private).toBeUndefined()
    // THE SHAPE AND THE SOURCE, not a literal: pinning the number here would
    // redden a routine version bump in a test that has no opinion about which
    // version is right. What matters is that what packs is what the repository
    // declares.
    expect(manifest.version).toMatch(/^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/)
    expect(manifest.version).toBe((JSON.parse(readFileSync(join(packageRoot, 'package.json'), 'utf8')) as { version: string }).version)
    expect(manifest.dependencies).toBeUndefined()
    expect(manifest.peerDependencies).toBeUndefined()
    expect(manifest.optionalDependencies).toBeUndefined()
    for (const script of ['preinstall', 'install', 'postinstall', 'prepare']) {
      expect(manifest.scripts[script], `${script} runs on install`).toBeUndefined()
    }
  })

  it('wires prepack to the build and the completeness gate', () => {
    // Without this, deleting or misspelling prepack keeps every other test
    // green while the publish gate quietly stops running.
    const { scripts } = JSON.parse(readFileSync(join(packageRoot, 'package.json'), 'utf8')) as { scripts: Record<string, string> }
    expect(scripts.prepack, 'prepack is gone: npm pack would ship whatever happens to be on disk').toBeDefined()
    expect(scripts.prepack).toContain('npm run build')
    expect(scripts.prepack).toContain('scripts/package-check.mjs')
    expect(scripts.build).toContain('build:wasm')
    expect(scripts.build).toContain('build:fonts')
  })

  it('records the engine version its wasm was built from', async () => {
    const { version } = await import('../src/version.js')
    const manifest = JSON.parse(readFileSync(join(packageRoot, 'package.json'), 'utf8')) as { folio8EngineVersion: string }
    expect(manifest.folio8EngineVersion).toBe(version)
  })
})

describe('an offline install renders the README snippet', () => {
  it(
    'writes a PDF matching the fixture hash',
    () => {
      const project = join(workspace, 'consumer')
      mkdirSync(project, { recursive: true })
      writeFileSync(join(project, 'package.json'), `${JSON.stringify({ name: 'folio-js-offline-consumer', version: '0.0.0', private: true, type: 'module' }, null, 2)}\n`)

      // A cache of its own plus --offline means nothing can be fetched and
      // nothing can be served from this machine's warm cache: if the package
      // needed a dependency or a network round trip, this fails.
      run('npm', ['install', tarball, '--offline', '--ignore-scripts', '--no-audit', '--no-fund', '--cache', join(workspace, 'npm-cache')], project, { npm_config_offline: 'true' })

      cpSync(join(fixture, 'input.folio'), join(project, 'invoice.folio'))
      cpSync(join(fixture, 'data.json'), join(project, 'invoice.json'))
      writeFileSync(join(project, 'first-pdf.mjs'), readmeSnippet())

      run('node', ['first-pdf.mjs'], project)

      const expected = JSON.parse(readFileSync(join(fixture, 'expected.json'), 'utf8')) as { sha256: string }
      expect(sha256(new Uint8Array(readFileSync(join(project, 'invoice.pdf'))))).toBe(expected.sha256)
    },
    timeout,
  )
})

describe('prepack refuses an incomplete package', () => {
  const check = (root: string, extra: string[] = []) => {
    try {
      run('node', [join(packageRoot, 'scripts', 'package-check.mjs'), '--root', root, '--go-fonts', join(repoRoot, 'folio-go', 'fonts'), '--parity', join(packageRoot, 'test', 'data', 'go-parity.json'), ...extra], workspace)
      return ''
    } catch (error) {
      return String((error as { stderr?: Buffer | string }).stderr ?? '')
    }
  }

  it('names the wasm, the built JavaScript and every missing face', () => {
    const empty = mkdtempSync(join(tmpdir(), 'folio-js-empty-'))
    const stderr = check(empty)
    expect(stderr).toContain('refusing to pack an incomplete package')
    expect(stderr).toContain('wasm/folio8-render.wasm')
    expect(stderr).toContain('dist/index.js')
    expect(stderr).toContain('Noto Sans SC')
    rmSync(empty, { recursive: true, force: true })
  })

  it('names a face whose bytes drifted from folio-go/fonts/', () => {
    const drifted = mkdtempSync(join(tmpdir(), 'folio-js-drift-'))
    const manifest = JSON.parse(readFileSync(join(packageRoot, 'fonts', 'manifest.json'), 'utf8')) as { faces: { name: string; file: string }[] }
    for (const path of requiredFiles.filter((file) => file !== 'fonts/manifest.json')) {
      mkdirSync(join(drifted, dirname(path)), { recursive: true })
      writeFileSync(join(drifted, path), '')
    }
    mkdirSync(join(drifted, 'fonts'), { recursive: true })
    cpSync(join(packageRoot, 'fonts', 'manifest.json'), join(drifted, 'fonts', 'manifest.json'))
    for (const face of manifest.faces) {
      mkdirSync(join(drifted, 'fonts', dirname(face.file)), { recursive: true })
      // Everything is in place; only "Roboto Bold" differs from the Go source.
      const source = join(packageRoot, 'fonts', face.file)
      if (face.name === 'Roboto Bold') writeFileSync(join(drifted, 'fonts', face.file), 'not the face folio-go embeds')
      else cpSync(source, join(drifted, 'fonts', face.file))
      for (const licence of ['LICENSE-OFL.txt', 'NOTICE.md']) cpSync(join(packageRoot, 'fonts', dirname(face.file), licence), join(drifted, 'fonts', dirname(face.file), licence))
    }

    const stderr = check(drifted)
    expect(stderr).toContain('the packaged face "Roboto Bold"')
    expect(stderr).toContain('differs from folio-go/fonts/')
    expect(stderr).not.toContain('"Noto Sans SC"')
    rmSync(drifted, { recursive: true, force: true })
  }, timeout)
})

// The advertised support range and the tested one must be the same range.
// package.json's `engines.node` is what an installer's npm enforces; the
// folio-js CI matrix's lowest Node is what the corpus is actually rendered on.
// Nothing else ties the two together, so bumping one without the other would
// quietly ship a floor no leg has ever exercised.
describe('the declared Node floor', () => {
  it('is the lowest Node the CI matrix renders the corpus on', () => {
    const pkg = JSON.parse(readFileSync(join(packageRoot, 'package.json'), 'utf8')) as { engines: { node: string } }
    const declared = /^>=\s*(\d+\.\d+)/.exec(pkg.engines.node)
    expect(declared, `engines.node is "${pkg.engines.node}"; this check expects a ">=major.minor" floor`).not.toBeNull()

    const workflow = readFileSync(join(repoRoot, '.github', 'workflows', 'ci.yml'), 'utf8')
    const list = /^\s*node:\s*\[([^\]]*)\]/m.exec(workflow)
    expect(list, 'no `node: [...]` matrix axis found in .github/workflows/ci.yml').not.toBeNull()
    const versions = [...list![1]!.matchAll(/(\d+)\.(\d+)\.(\d+)/g)].map((m) => [Number(m[1]), Number(m[2]), Number(m[3])] as const)
    expect(versions.length).toBeGreaterThan(1)
    const floor = versions.sort((a, b) => a[0] - b[0] || a[1] - b[1] || a[2] - b[2])[0]!

    expect(`${floor[0]}.${floor[1]}`, 'engines.node and the CI matrix floor have drifted apart').toBe(declared![1])
  })
})
