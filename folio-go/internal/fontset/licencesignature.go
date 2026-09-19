package fontset

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/boxesandglue/textshape/ot"
)

// THE TIE BETWEEN A FACE'S DECLARED LICENCE AND WHAT ITS OWN BYTES SAY.
//
// Story 16.1b (D-16.R.5, as replaced in part by D-16.R.7). Until this file
// existed, that tie was a BUILD-TIME test over 21 reviewed faces —
// folio-designer/src/font-catalogue.test.ts:355-366, which holds each
// catalogue face's `name` table record 13 to the SPDX id
// font-catalogue.json declares for it, on the ground that record 13 is "the
// one statement of a face's licence that cannot be edited from outside the
// binary". Epic 16 lets a face arrive from the published library at the
// moment an author picks it, so the population the tie polices stops being
// 21 files somebody read and becomes ~1,946 families nobody has. A build
// gate cannot follow it there. Without this door Epic 16 would replace that
// gate with something STRICTLY WEAKER, on exactly the axis D-8.6.5 already
// cost this project once: 17 of 21 catalogue faces shipped under another
// project's licence, green, until a review caught it.
//
// THE BUILD-TIME TIE IS KEPT, NOT MOVED (D-16.R.7's "both, or halt"). This
// is an addition. font-catalogue.test.ts still checks the 21 committed
// faces at build time, and TestGoLicenceTableSubsumesTheDesignerTable
// enforces that the table below never admits less than the TypeScript one.
//
// THREE OUTCOMES, NEVER TWO (D-16.R.7):
//
//	CONTRADICTION — the statement matches a refuse-signature, or matches a
//	                DIFFERENT admitted licence's signature than the one
//	                declared -> REFUSE, naming both sides.
//	CONFIRMATION  — it matches the declared licence's signature -> admit.
//	NO EVIDENCE   — nothing matches, or there is no readable statement at
//	                all -> ADMIT. See the long note on ReadLicenceStatement.
//
// GO RE-READS THE BYTES, AND MAY NEVER COMPARE THE WIRE `copyright` FIELD
// AGAINST THE WIRE `licence` FIELD. Both of those arrive in the same
// command payload from the same browser, so a check over them proves
// nothing at all: a caller that got the licence wrong got the copyright
// wrong with it, and the two sides would move together. The only statement
// worth tying to is the one inside the bytes about to be written into the
// document.

// licenceSignature is one row of the two tables below: a regular expression
// over a face's own licence statement, and — for an admit-signature — the
// SPDX id that statement confirms.
//
// `id` is the SPDX identifier for an admit-signature and is EMPTY for a
// refuse-signature, which is checked against every face whatever it
// declares and so is keyed by nothing. `label` is what the refusal calls
// the family of licences the row recognises, because "the bytes match
// /(?i)\bA?GPL\b/" is not a sentence anybody can act on.
type licenceSignature struct {
	id      string
	label   string
	pattern *regexp.Regexp
}

// admitLicenceSignatures is the ADMIT half: the sentence each permissive
// licence this project accepts writes into a face's own name table.
//
// AN ORDERED SLICE, NEVER A MAP (AD-1). A map range in Go is deliberately
// randomised, so a face whose statement matched two rows would produce a
// different refusal message run to run — and, worse, the ordering of the
// scan would be unreproducible in exactly the situation a reader is trying
// to reproduce. Matching is order-deterministic here by construction.
//
// SUBSTRING, NOT PREFIX OR EQUALITY, and that is measured rather than
// assumed: at build time `cascadiacode`'s record 13 OPENS "Microsoft
// supplied font..." and carries the OFL sentence further in.
//
// THE APACHE ROW IS THIS STORY'S ADDITION and is owed rather than
// optional: D-16.R.4 admits the upstream `APACHE2` token to `Apache-2.0`,
// and the build-time table has no Apache row (it never needed one — no
// committed catalogue face is Apache-licensed), so every Apache family
// would have passed the licence gate and been refused one step later by
// this tie. Verified against real bytes:
// testdata/fonts/Roboto-Regular.ttf's record 13 reads "Licensed under the
// Apache License, Version 2.0".
//
// AN ADMITTED SPDX ID WITH NO ROW HERE IS NO EVIDENCE, AND ADMITS
// (D-16.R.10). D-8.5.3 admits four identifiers and this table covers
// three; `MIT` is the fourth and gets no row because `google/fonts`
// publishes no MIT token for one to be minted against — "absence, not
// narrowing" (D-16.R.4). Refusing an id with no row would refuse a licence
// the OWNER has explicitly admitted, which a mechanism may not do to a
// policy. The floor that does not depend on this arm is
// refuseLicenceSignatures below, which applies to every face regardless.
var admitLicenceSignatures = []licenceSignature{
	{id: "OFL-1.1", label: "the SIL Open Font License", pattern: regexp.MustCompile(`(?i)SIL Open Font License`)},
	{id: "Ubuntu-font-1.0", label: "the Ubuntu Font Licence", pattern: regexp.MustCompile(`(?i)Ubuntu Font Licence`)},
	{id: "Apache-2.0", label: "the Apache License, Version 2.0", pattern: regexp.MustCompile(`(?i)Apache License,?\s+Version 2\.0`)},
}

// refuseLicenceSignatures is the REFUSE half, and it is checked against
// EVERY face whatever that face declares. This half is new — the
// build-time tie never had it.
//
// Ground: AD-26 Binds *all*, and its stated Prevents is copyleft arriving
// through a plausible-looking package. A declared-id-only check would let
// a GPL face straight in under an `OFL-1.1` token, because the token is
// the very thing under suspicion. So a face whose own bytes name a
// share-alike or GPL-family licence is refused EVEN WHEN the declared
// licence is one this project admits.
//
// Ordered, for the same reason as the table above.
var refuseLicenceSignatures = []licenceSignature{
	// \b-anchored so "GPL" inside a longer word cannot fire it, and with an
	// optional version tail so "GPLv3" and "LGPL-2.1" are caught too —
	// without it the \b after "GPL" fails against the "v" and the face
	// walks through.
	{label: "the GNU GPL family (GPL/LGPL/AGPL)", pattern: regexp.MustCompile(`(?i)\b(?:A|L)?GPL(?:[-\s]?v?\d[.\d]*)?\b`)},
	{label: "the GNU GPL family (GPL/LGPL/AGPL)", pattern: regexp.MustCompile(`(?i)\b(?:GNU\s+)?(?:Affero\s+|Lesser\s+)?General Public License`)},
	{label: "the Server Side Public License", pattern: regexp.MustCompile(`(?i)\bSSPL\b|Server Side Public License`)},
	{label: "a ShareAlike licence (CC BY-SA)", pattern: regexp.MustCompile(`(?i)ShareAlike|Share-Alike|\bCC[-\s]?BY[-\s]?SA\b`)},
}

// maxQuotedStatementBytes bounds the EXCERPT of a face's own statement that a
// refusal below quotes, and it exists to defend a constant in another package.
//
// WHOSE CONSTANT, AND WHY THE TAIL IS THE PART WORTH KEEPING. A component
// failure message is cut at maxComponentFailureMessageBytes = 512
// (component_commands.go:1974, itself hand-copied from the wasm host's own
// literal), and that cut takes the TAIL. `The face says: %q` is the LAST
// clause of both refusals here and the whole reason either is actionable — it
// is the side of the comparison the author cannot look up. So an unbounded
// statement does not merely make a refusal long: it makes the host delete
// exactly the clause the refusal exists for, silently.
//
// This is the common case rather than an exotic one. Measured on the committed
// fixtures, the contradiction message is 378 bytes and the copyleft message
// 420 bytes over a 46-byte statement — most of the budget is already spent
// before the statement arrives — while a face's record 13 is frequently the
// ENTIRE OFL body and record 0 can run to kilobytes.
//
// truncateAtRuneBoundary in package `folio8` does this same job for DataPath,
// but it is unexported there and package `folio8` imports this package rather
// than the other way round, so the equivalent is written here instead of
// shared.
//
// WHAT THIS BOUND DOES NOT COVER, said plainly. It removes the one UNBOUNDED
// term from the message — a statement that can be kilobytes — and leaves the
// rest: `name` and `declared` are interpolated whole, and a caller passing a
// pathologically long font-chain name (legal up to the DataPath cut of 256)
// can still overrun 512 on its own. That is a property every refusal in this
// codebase shares and is not this door's to fix; the statement is the term
// that is long in the ORDINARY case, which is what made it worth bounding.
const maxQuotedStatementBytes = 72

// statementExcerptElision marks a cut so a reader can tell a short statement
// from a truncated one, and so nobody reads the excerpt as the whole of what
// the bytes said.
const statementExcerptElision = " […]"

// statementExcerpt cuts a statement to maxQuotedStatementBytes AT A RUNE
// BOUNDARY. Cutting by bytes alone would split a multi-byte rune — a real
// risk here and not a theoretical one, since the matrix's non-Latin row
// (`ofl/wdxllubrifonttc` states OFL 1.1 in Traditional Chinese) is exactly a
// statement with no single-byte runes in it at all.
func statementExcerpt(statement string) string {
	if len(statement) <= maxQuotedStatementBytes {
		return statement
	}
	cut := maxQuotedStatementBytes
	for cut > 0 && !utf8.RuneStart(statement[cut]) {
		cut--
	}
	return statement[:cut] + statementExcerptElision
}

// ReadLicenceStatement returns the licence statement a face makes about
// ITSELF, and reports whether it made one at all.
//
// WHAT "PRESENT" MEANS, DEFINED HERE BECAUSE THE WHOLE nameID 0 DOOR HANGS
// ON IT: a record is PRESENT when the parsed name table yields a NON-EMPTY
// string for it. Empty and absent are the same condition to this function,
// deliberately — see the fidelity note below, which is why they have to be.
//
// nameID 13 is the licence DESCRIPTION and decides alone whenever it is
// present. nameID 0 is the COPYRIGHT notice and is consulted ONLY WHEN 13
// IS ABSENT.
//
// WHY THAT IS A WIDENING AND NOT A FALLBACK, because the difference is the
// whole safety of it. Measured at Story 16.1's plan gate over 100 upstream
// faces: all three static `ufl/` families carry NO record 13 at all and
// state their terms in record 0, so a tie that looked only at 13 would
// have refused every genuine Ubuntu-licensed family. The property worth
// anything is "the one statement of a face's licence that cannot be edited
// from outside the binary", and that belongs to the NAME TABLE, not to
// record 13 specifically. But if 0 were consulted whenever 13 failed to
// MATCH, a face whose 13 says GPL could be laundered by a
// permissive-sounding 0 — defeating the contradiction check with the very
// thing it exists to catch. Absence is a different condition from
// disagreement, and only absence opens the second door.
//
// KNOWN FIDELITY LIMIT OF THE VENDOR READER, recorded here rather than
// discovered in review. ot.ParseName keeps a record only when it decodes
// to a non-empty string, and it decodes only platform 0/3 (UTF-16BE) and
// platform 1 encoding 0 (Mac Roman); anything else is skipped, and
// (*ot.Name).Get returns "" for both "no such record" and "record
// skipped". So a face stating its terms ONLY under an exotic
// platform/encoding reads as ABSENT here, which opens the record-0 door
// one step early. That can only make this check QUIETER, never louder — it
// cannot manufacture a false refusal — and D-16.R.7's own "how we'd know
// it was wrong" accepts exactly that cost: "a check quieter than intended,
// never a document publishing false terms".
//
// AND THE CASE THAT ARGUMENT DOES NOT COVER, named here so it is a KNOWN
// LIMIT rather than a later discovery. (*ot.Name) holds ONE entry per
// nameID — Name.entries is keyed by nameID alone, with no platform or
// language in the key — so a face carrying record 13 under SEVERAL
// platform/language combinations is reduced to a single string: each
// decodable record overwrites the last, so the FINAL one in table order
// wins, which is a property of the file's record layout and of nothing
// this project chose. A face whose English record 13 is permissive and whose
// record 13 under another platform or language names a copyleft licence is
// therefore decided by whichever record the vendor parser kept, and this
// door may read the permissive one and admit. That is louder than merely
// quiet: it is a contradiction this check can miss outright.
//
// It is recorded and NOT fixed. Do not "fix" it by hand-parsing the name
// table; a second parser here would be a second authority on what a face
// says about itself, and the spec forbids one. The floor underneath is the
// same as everywhere else in this file: refuseLicenceSignatures still
// applies to whichever record IS read, and the build-time tie still covers
// every face this repository itself redistributes.
func ReadLicenceStatement(data []byte) (string, bool) {
	parsed, err := ot.ParseFont(data, 0)
	if err != nil {
		return "", false
	}
	// The guard ORDER is readPostScriptName's, copied deliberately, so that
	// each failure is refused by the narrowest condition that explains it
	// rather than by a later one that would have caught it anyway.
	//
	// THE THREE FAILURES ARE DELIBERATELY COLLAPSED, and this signature
	// cannot tell them apart: no name table, a name table that will not
	// parse, and a name table that parses to nothing in 13 or 0 all return
	// ("", false), and every caller and every test in this package sees the
	// same NO EVIDENCE. That is not an oversight to be repaired by widening
	// the return: all three admit, for the one reason recorded on
	// RefuseContradictedLicence — a face that says nothing has made no
	// statement to be false — so a caller that could distinguish them would
	// have nothing different to do with the distinction.
	if !parsed.HasTable(ot.TagName) {
		return "", false
	}
	table, err := parsed.TableData(ot.TagName)
	if err != nil {
		return "", false
	}
	names, err := ot.ParseName(table)
	if err != nil {
		return "", false
	}
	if description := names.Get(13); description != "" {
		return description, true
	}
	if copyright := names.Get(0); copyright != "" {
		return copyright, true
	}
	return "", false
}

// RefuseContradictedLicence is the byte-taking door, and it is the sibling
// of RefuseVariableFace in every respect that matters: it takes raw bytes
// rather than a parsed face, it re-parses them itself, it returns an
// untyped fmt.Errorf, and it returns nil for a face it cannot read.
//
// `declared` is the SPDX identifier the caller is claiming on this face's
// behalf — the `licence` field of the `font` record about to be written
// beside these bytes.
//
// WHY SILENCE ADMITS, WRITTEN HERE BECAUSE IT IS THE PART THAT WILL LOOK
// WRONG TO THE NEXT READER. The threat this guard exists for is a face
// travelling under ANOTHER PROJECT'S terms — D-8.6.5, where 17 of 21 faces
// carried a licence that was not theirs. That is a FALSE statement. A face
// that says nothing readable has made no statement to be false, so
// refusing it catches none of that threat; and it is not cheap noise
// either, because it was MEASURED — 50 of 100 sampled upstream faces were
// refused under the original "absent or unparseable => REFUSE" contract,
// the great majority of them silences, about a sixth of the library. A
// guard that loud is one somebody eventually turns off.
//
// Nor is silence a hole in the structural sense: genuinely unreadable
// bytes are refused one step later by template.DecodeFontForRender /
// checkSfnt, which is what makes NO EVIDENCE safe to admit here rather
// than merely tolerable. This function answers exactly one question —
// "do these bytes contradict the licence being claimed for them?" — and
// widening it to speak for structural validity too would make it a
// second, partial copy of that gate's rules.
func RefuseContradictedLicence(name, declared string, data []byte) error {
	statement, present := ReadLicenceStatement(data)
	if !present {
		return nil // NO EVIDENCE.
	}

	// THE REFUSE HALF FIRST, AND AGAINST EVERY FACE. It does not consult
	// `declared` at all, which is the point: the declaration is what is
	// under suspicion.
	for _, signature := range refuseLicenceSignatures {
		if signature.pattern.MatchString(statement) {
			return fmt.Errorf(
				"fontset: font %q: this face's own `name` table names %s, and folio8 does not embed a copyleft or "+
					"share-alike face whatever licence is declared for it (this one is declared %q): a font program is "+
					"embedded and subset into every PDF the author produces, so its terms attach to the author's own "+
					"documents. The face says: %q",
				name, signature.label, declared, statementExcerpt(statement),
			)
		}
	}

	// THE ADMIT HALF. Every row is scanned before anything is decided,
	// rather than returning on the first hit: a statement that matches the
	// declared licence's row AND some other row is a CONFIRMATION, and
	// deciding on whichever row happened to come first would turn table
	// order into policy.
	var matched []licenceSignature
	for _, signature := range admitLicenceSignatures {
		if signature.pattern.MatchString(statement) {
			if signature.id == declared {
				return nil // CONFIRMATION.
			}
			matched = append(matched, signature)
		}
	}
	if len(matched) == 0 {
		return nil // NO EVIDENCE: the bytes made no claim this table recognises.
	}

	// EVERY MATCHED LABEL IS NAMED, not just the first. The scan above
	// deliberately visits every row so that table order does not become
	// policy; naming matched[0] alone would hand the order that policy back
	// one line later, and it would do it in the message — the one artefact a
	// reader diagnoses from. A statement that names two licences neither of
	// which is the declared one is a face with two things wrong with it, and
	// a refusal that mentioned one of them would send the author to fix half.
	labels := make([]string, 0, len(matched))
	for _, signature := range matched {
		labels = append(labels, signature.label)
	}

	// CONTRADICTION. The message names BOTH sides, because either one alone
	// is unactionable: the author cannot tell whether the catalogue row is
	// wrong or the binary is the wrong binary without seeing the two
	// statements side by side.
	return fmt.Errorf(
		"fontset: font %q: this face is declared %q and its own `name` table names %s instead — the binary's own "+
			"statement of its terms is the one that cannot be edited from outside it, so the declaration is what is "+
			"wrong here, or the bytes are not the face they are labelled as. The face says: %q",
		name, declared, strings.Join(labels, " and "), statementExcerpt(statement),
	)
}
