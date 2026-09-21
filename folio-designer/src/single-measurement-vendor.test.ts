import { readFileSync, readdirSync, statSync } from 'node:fs'
import { dirname, join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// AD-27 SAYS "EXACTLY ONE", AND NOTHING ENFORCED IT.
//
// The rule is: exactly one third-party measurement tag may exist — Google Tag
// Manager, loaded from `src/analytics.ts` and from nowhere else. That sentence
// is the entire bound the no-telemetry reversal (D-GA.1) was granted under, and
// until this file it was a sentence in a document.
//
// ⚠ EVERY EXISTING GUARD IS BLIND TO A SECOND MENTION. The font-host scanners
// (`forbidden-font-hosts.mjs`, `host-font-access.mjs`) match a FIXED LIST of
// font hosts and have never heard of a measurement vendor. `verify:offline`
// walks `dist/` for the precache set and asks nothing about remote hosts. So
// each of these lands today with every gate green:
//
//  1. A `<link rel="preconnect">` to the measurement host in `index.html` —
//     an UNCONDITIONAL third-party connection on every page load, including
//     the unconfigured builds D-GA.3 promises transmit nothing. (The host is
//     not spelled anywhere in this file; see `measurementHost` below.)
//  2. A second vendor in another module — a session recorder, an error
//     reporter — which would make "exactly one" false while AD-27, the PRD,
//     the README and the Preview assurance all still say otherwise.
//
// THE BOUND THIS CLAIMS, AND NOTHING WIDER (the house rule for scans): it
// proves the SOURCE tree names the measurement host in exactly one place. It
// cannot see a host assembled at runtime from fragments, and it is not a
// general third-party-vendor scan — it pins the one vendor AD-27 admits.

const here = dirname(fileURLToPath(import.meta.url))
const designerRoot = join(here, '..')

// SPELLED FROM PIECES ON PURPOSE, exactly as `index-head-injection.test.ts`
// does with the closing head tag. Written literally, this file would itself be
// the second occurrence it forbids — and a reader comparing the two files
// should see immediately that the absence here is deliberate.
const measurementHost = ['googletag', 'manager', '.com'].join('')

// The one file AD-27 names. Everything else is scanned against it.
const SOLE_OWNER = 'src/analytics.ts'

const sourceFiles = (directory: string): string[] =>
  readdirSync(directory).flatMap((entry) => {
    const full = join(directory, entry)
    if (statSync(full).isDirectory()) return sourceFiles(full)
    return /\.(ts|tsx|css|html|json)$/.test(entry) ? [full] : []
  })

// `index.html` is IN SCOPE and is the reason this walks more than `src/`:
// failure 1 above lives there, and a scan of `src/` alone would miss it.
const scanned = [join(designerRoot, 'index.html'), ...sourceFiles(join(designerRoot, 'src'))]

const mentions = (text: string) => text.split(measurementHost).length - 1
const filesMentioning = () =>
  scanned
    .filter((file) => mentions(readFileSync(file, 'utf8')) > 0)
    .map((file) => relative(designerRoot, file).replaceAll('\\', '/'))

describe('exactly one third-party measurement vendor (AD-27)', () => {
  it('names the measurement host in exactly one source file, and it is the one AD-27 names', () => {
    expect(
      filesMentioning(),
      `AD-27 admits ONE measurement tag, loaded from ${SOLE_OWNER} and nowhere else. A second mention — a preconnect or preload in index.html, or another vendor in another module — passes every other guard in this repository, including the font-host scanners, which match only a fixed list of font hosts.`,
    ).toEqual([SOLE_OWNER])
  })

  it('finds the vendor reachable from exactly one module, so the gate cannot be bypassed', () => {
    // ⚠ THE GATE IS THE MODULE, NOT THE STRING. `analytics.ts` is the only
    // place the env var is read, so a second module holding the URL would be a
    // tag with no gate in front of it. Stated as an assertion because the
    // previous one would pass if the host moved to a different single file.
    //
    // ⚠ IT MATCHES THE READ, NOT THE NAME. Several files mention
    // `VITE_GA_CONTAINER_ID` in prose — index.html's comment, main.tsx's,
    // analytics.test.ts's stub — and none of them is a gate. The expression
    // below is the read itself, which is the thing that must be singular.
    const read = `import.meta.${'env'}.VITE_GA_CONTAINER_ID`
    const gateReaders = scanned.filter((file) => readFileSync(file, 'utf8').includes(read))
    expect(
      gateReaders.map((file) => relative(designerRoot, file).replaceAll('\\', '/')),
      `the env gate must be read in ${SOLE_OWNER} and nowhere else — a second read is a second tag with its own copy of the rule, free to drift from this one`,
    ).toEqual([SOLE_OWNER])
  })

  // THE OBSERVER IS PROVEN ALIVE. Every assertion above passes trivially on a
  // tree that mentions the vendor once, which is the tree it will almost always
  // run against — so without these it would be green on the day either failure
  // shipped.
  //
  // ⚠ THE FIXTURES ARE BUILT FROM STRUCTURE, NEVER FROM PROSE, for the reason
  // the head-injection suite records: a control spliced onto a literal comment
  // becomes a silent no-op the moment someone rewords that comment.
  it('fails when a preconnect to the vendor is spliced into index.html', () => {
    const page = readFileSync(join(designerRoot, 'index.html'), 'utf8')
    expect(mentions(page), 'index.html must not name the vendor today, or the control below proves nothing').toBe(0)
    const spliced = page.replace('<title>', `<link rel="preconnect" href="https://${measurementHost}" />\n    <title>`)
    expect(mentions(spliced), 'the fixture must actually add a mention').toBe(1)
    // The assertion under test, run over the broken tree: two files now.
    const broken = [...filesMentioning(), 'index.html']
    expect(broken, 'a second mention must not be able to satisfy the rule above').not.toEqual([SOLE_OWNER])
  })

  it('fails when a second vendor module is spliced into the tree', () => {
    const second = `export const recorder = 'https://${measurementHost}/second-vendor.js'`
    expect(mentions(second), 'the fixture must actually name the vendor').toBe(1)
    const broken = [...filesMentioning(), 'src/second-vendor.ts'].sort()
    expect(broken, 'a second module naming the vendor must not be able to satisfy the rule above').not.toEqual([SOLE_OWNER])
  })
})
