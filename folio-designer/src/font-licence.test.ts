import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { admittedByTheTokenTable, admittedSpdxIdentifiers, classifiedTokens, classifyLicenceToken, refusalReason } from './font-licence'
import { DECLARED_ONLY_FONT_HOSTS, FORBIDDEN_FONT_HOSTS } from '../scripts/forbidden-font-hosts.mjs'

// STORY 16.1 — THE CLOSED TOKEN TABLE (D-16.R.4).
//
// THE FAILURE THIS FILE EXISTS TO CATCH IS A PARTIAL PASS THAT LOOKS LIKE A
// WORKING PRODUCT. `METADATA.pb` publishes `OFL`, `APACHE2` and `UFL`; D-8.5.3's
// allowlist names `OFL-1.1`, `Apache-2.0`, `MIT` and `Ubuntu-font-1.0`. Compared
// literally, every family is refused — except that `UFL` looks close enough to
// pass a careless eye. So the mapping is asserted in BOTH directions here, and
// the admitted set is held under the owner's four.

const here = path.dirname(fileURLToPath(import.meta.url))

describe('the upstream licence token table', () => {
  // NON-VACUITY FIRST. Every assertion below is over the table, and an empty
  // table satisfies "admits nothing outside the allowlist" perfectly.
  it('classifies the four upstream tokens this designer has actually seen', () => {
    expect([...classifiedTokens].sort()).toEqual(['APACHE2', 'CC-BY-SA', 'OFL', 'UFL'])
  })

  it('maps each admitted upstream token to the SPDX id a document carries, and never to the token itself', () => {
    for (const [token, spdx] of [['OFL', 'OFL-1.1'], ['APACHE2', 'Apache-2.0'], ['UFL', 'Ubuntu-font-1.0']] as const) {
      const classification = classifyLicenceToken(token)
      expect(classification.state, `${token} must be admitted`).toBe('admitted')
      expect(classification.state === 'admitted' && classification.spdx).toBe(spdx)
      // AND THE TOKEN NEVER REACHES THE DOCUMENT. `font-catalogue.json` already
      // writes `Ubuntu-font-1.0` rather than `UFL`, and Go's signature table is
      // keyed on SPDX. Two vocabularies in one field make a document unsortable
      // by its own terms.
      expect(spdx).not.toBe(token)
    }
  })

  // THE THREE STATES, AND THE WHOLE POINT IS THAT REFUSED AND UNRECOGNISED ARE
  // NOT THE SAME ANSWER. `cc-by-sa` is a real top-level directory in
  // `google/fonts`, and it is in the table precisely so that a FIFTH directory
  // appearing upstream reads as "we have not classified this" rather than as a
  // policy decision nobody made.
  it('refuses a mapped-but-unacceptable token by NAME, with a stated reason', () => {
    const classification = classifyLicenceToken('CC-BY-SA')
    expect(classification.state).toBe('refused')
    expect(refusalReason(classification)).toContain('CC-BY-SA')
    expect(refusalReason(classification)).toMatch(/copyleft/i)
    expect(refusalReason(classification)).not.toMatch(/not recognise/i)
  })

  it('refuses an unmapped token as NOT RECOGNISED, never as forbidden', () => {
    const classification = classifyLicenceToken('GPL3')
    expect(classification.state).toBe('unrecognised')
    expect(refusalReason(classification)).toMatch(/does not recognise/)
    expect(refusalReason(classification)).toMatch(/not the same as being forbidden/)
    // It names what it DOES classify, so the reader can tell a fifth upstream
    // directory from a typo.
    for (const token of classifiedTokens) expect(refusalReason(classification)).toContain(token)
  })

  // NO FALL-THROUGH, NO PERMISSIVE DEFAULT, NO WARN-AND-CONTINUE (D-8.5.2). The
  // build gate this replaces FAILED THE BUILD on an unclassifiable licence;
  // moving the check to the author's machine may not weaken it.
  it('admits nothing it was not asked about, including the empty and near-miss cases', () => {
    for (const token of ['', ' ', 'OFL-1.1', 'Apache-2.0', 'OFLX', 'APACHE', 'MIT', 'CC0', 'PROPRIETARY']) {
      expect(classifyLicenceToken(token).state, `${JSON.stringify(token)} must not be admitted by fall-through`).not.toBe('admitted')
    }
    // ⚠ `OFL-1.1` and `Apache-2.0` above are NOT typos. They are the SPDX ids,
    // which are what the table EMITS and never what it reads: an implementation
    // that accepted its own output as input would silently admit a document's
    // own field as if upstream had published it.
  })

  it('normalises case and surrounding space, and nothing else', () => {
    expect(classifyLicenceToken(' ofl ').state).toBe('admitted')
    expect(classifyLicenceToken('Apache2').state).toBe('admitted')
    expect(classifyLicenceToken('OFL 1.1').state).toBe('unrecognised')
    expect(classifyLicenceToken('O F L').state).toBe('unrecognised')
  })

  // THE OWNER'S ALLOWLIST IS THE CEILING. D-8.5.3's four identifiers are a
  // product and legal decision and this module may never outrun them.
  it('admits a SUBSET of D-8.5.3\'s four identifiers', () => {
    expect(admittedSpdxIdentifiers.length).toBe(4)
    expect(admittedByTheTokenTable.length).toBeGreaterThan(0)
    for (const spdx of admittedByTheTokenTable) {
      expect(admittedSpdxIdentifiers as ReadonlyArray<string>, `${spdx} is not one of the four licences the owner admits`).toContain(spdx)
    }
    // AND IT IS A STRICT SUBSET TODAY, WHICH IS WHY THE CHECK IS NOT VACUOUS:
    // MIT is admissible and has no row, because `google/fonts` publishes no MIT
    // token and a token that never arrives needs no mapping. ABSENCE, NOT
    // NARROWING — the module says so itself.
    expect(admittedByTheTokenTable).not.toContain('MIT')
    expect(fs.readFileSync(path.join(here, 'font-licence.ts'), 'utf8')).toMatch(/ABSENCE RATHER THAN\n\/\/ NARROWING/)
  })

  // D-16.4: this module names no host, so the forbidden-host scan's second half
  // keeps ONE small subject. A licence decision that reached for a URL would
  // scatter that subject across two files.
  it('names no host at all', () => {
    const source = fs.readFileSync(path.join(here, 'font-licence.ts'), 'utf8')
    // The hosts are read off the scanner's own lists rather than spelled here,
    // which keeps this file out of the scan's population AND makes the check
    // follow the lists if either ever grows.
    const everyHost = [...FORBIDDEN_FONT_HOSTS, ...DECLARED_ONLY_FONT_HOSTS].map((entry) => entry.host)
    expect(everyHost.length).toBeGreaterThanOrEqual(4)
    for (const host of [...everyHost, 'http://', 'https://']) {
      expect(source, `font-licence.ts must name no host, and it names ${host}`).not.toContain(host)
    }
  })
})
