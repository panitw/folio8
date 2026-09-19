// THE CLOSED LICENCE TOKEN TABLE (D-16.R.4).
//
// WHAT THIS MODULE DECIDES, AND THE ONE IT DOES NOT. It decides whether the
// terms a family publishes are terms this product accepts at all, from what the
// library says about that family, BEFORE a single byte is embedded. It does not
// decide whether a face's bytes agree with the terms written beside them — that
// is a different question with a different authority, `fontset.
// RefuseContradictedLicence` in Go, and this module's answer is literally that
// one's input (`component_commands.go`'s `record.Licence.Value`).
//
// THE UPSTREAM TOKEN IS NOT AN SPDX ID, and getting that wrong is how this epic
// would have shipped broken. Measured 2026-09-02: `METADATA.pb` carries
// `license: "OFL"`, `"UFL"`, `"APACHE2"` — never `OFL-1.1`, `Ubuntu-font-1.0` or
// `Apache-2.0`. Compared literally against D-8.5.3's four identifiers, every
// family would be refused except by the accident that `UFL` matches nothing and
// looks like it does. So there are two vocabularies, and this table is the one
// place they meet.
//
// `font.licence` IN THE `.folio` CARRIES THE SPDX ID, NEVER THE UPSTREAM TOKEN.
// Precedent, not preference: `font-catalogue.json` already writes
// `Ubuntu-font-1.0` rather than `UFL`, and Go's signature table is keyed on
// SPDX. Two vocabularies in one field make a document unsortable by its own
// terms.
//
// THIS MODULE NAMES NO HOST, deliberately (D-16.4). Every allowed host is
// spelled in exactly one module, `src/font-source.ts`, so the forbidden-host
// scan's second half has one small subject rather than a scattering of them.
//
// MIT IS ADMISSIBLE AND HAS NO ROW HERE, AND THAT IS ABSENCE RATHER THAN
// NARROWING. D-8.5.3's four identifiers — OFL-1.1, Apache-2.0, MIT,
// Ubuntu-font-1.0 — are owner policy about acceptable licences and are not
// amended by this module. `google/fonts` publishes no MIT token, so MIT has
// nothing to map: a token that never arrives needs no row, and adding one would
// invent a mapping from a string upstream does not emit.

/** D-8.5.3's allowlist, as the owner set it. This module may never outrun it. */
export const admittedSpdxIdentifiers = ['OFL-1.1', 'Apache-2.0', 'MIT', 'Ubuntu-font-1.0'] as const

/**
 * THREE STATES, NEVER TWO.
 *
 * `admitted` — the token is in the table and maps to an SPDX id a document may
 * carry.
 *
 * `refused` — the token is in the table and this product does not accept those
 * terms. The token is NAMED and the reason is STATED.
 *
 * `unrecognised` — the token is not in the table. Also a refusal, and it says
 * THE TOKEN WAS NOT RECOGNISED rather than that it is forbidden.
 *
 * The third state exists because absent and refused must stop looking the same.
 * `cc-by-sa` is a real top-level directory in `google/fonts`, and it is in the
 * table precisely so that a FIFTH directory appearing upstream reads as "we have
 * not classified this" rather than as a policy decision nobody made.
 */
export type LicenceClassification =
  | Readonly<{ state: 'admitted'; token: string; spdx: string }>
  | Readonly<{ state: 'refused'; token: string; reason: string }>
  | Readonly<{ state: 'unrecognised'; token: string; reason: string }>

type TableRow = Readonly<{ spdx: string } | { refusal: string }>

/**
 * THE TABLE. Closed, and every row is a decision somebody made here.
 *
 * NO FALL-THROUGH, NO PERMISSIVE DEFAULT, NO WARN-AND-CONTINUE (D-8.5.2). A
 * token with no row is refused. The build gate this replaces failed the build on
 * an unclassifiable licence rather than warning about it, and moving the check
 * to the author's machine may not weaken it — D-8.6.5, where 17 of 21 faces
 * carried another project's licence undetected, is the precedent for what an
 * unwatched licence gate costs.
 */
const licenceTokens: Readonly<Record<string, TableRow>> = {
  OFL: { spdx: 'OFL-1.1' },
  APACHE2: { spdx: 'Apache-2.0' },
  UFL: { spdx: 'Ubuntu-font-1.0' },
  // PRESENT AND REFUSED. ShareAlike is copyleft: a document embedding such a
  // face would carry a term about what the person receiving the file may do
  // with the whole of it, which is outside the allowlist D-8.5.3 admits. It is
  // listed rather than omitted so this refusal reads as a decision.
  'CC-BY-SA': { refusal: 'ShareAlike is a copyleft term, and the licences this product admits for an embedded face are OFL-1.1, Apache-2.0, MIT and Ubuntu-font-1.0' },
}

/** The SPDX ids this table can put into a document. Asserted a subset of D-8.5.3's four. */
export const admittedByTheTokenTable: ReadonlyArray<string> =
  Object.values(licenceTokens).flatMap((row) => ('spdx' in row ? [row.spdx] : []))

/** The upstream tokens this table classifies at all, admitted or refused. */
export const classifiedTokens: ReadonlyArray<string> = Object.keys(licenceTokens)

/**
 * Classify one `METADATA.pb` `license:` token.
 *
 * The token is compared AFTER trimming and upper-casing, because the upstream
 * file is hand-maintained text and `ofl` and `OFL` are the same statement; it is
 * never compared loosely in any other way — no prefix match, no substring, no
 * stripped punctuation. A token this table does not hold is `unrecognised`, not
 * a near miss for the row it most resembles.
 */
export function classifyLicenceToken(token: string): LicenceClassification {
  const normalised = token.trim().toUpperCase()
  const row = Object.hasOwn(licenceTokens, normalised) ? licenceTokens[normalised] : undefined
  if (row === undefined) {
    return {
      state: 'unrecognised',
      token: token.trim(),
      reason: `its licence is published as "${token.trim()}", which this designer does not recognise — it has not been classified, which is not the same as being forbidden. The tokens it classifies are ${classifiedTokens.join(', ')}`,
    }
  }
  if ('spdx' in row) return { state: 'admitted', token: normalised, spdx: row.spdx }
  return { state: 'refused', token: normalised, reason: `its licence is published as "${normalised}", and ${row.refusal}` }
}

/** A refusal's sentence, for the two states that carry one. */
export function refusalReason(classification: LicenceClassification): string | undefined {
  return classification.state === 'admitted' ? undefined : classification.reason
}
