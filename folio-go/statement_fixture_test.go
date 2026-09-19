package folio8

// Story 4.7 — the Customer Account Statement, the MVP's flagship
// document, recorded at four page counts (1, 5, 20 and 50).
//
// WHAT THIS FAMILY IS FOR, AND WHAT IT IS NOT.
//
// Measured at this story's baseline (df8cbcc): `grep -l '"table"'
// fixtures/*/input.folio` returned NOTHING. Not one committed golden in
// this repository contained a table, so no recorded byte in the corpus
// could tell a correct table from a broken one, and Story 4.6's
// unconditional-clip mutation reddened ZERO goldens while reddening the
// table behaviour suite. This family is the first table golden the
// programme has. It is therefore a NEW recording compared against
// itself on this first pass, and that limb proves nothing until the
// bytes move. The one non-circular check on the recording is the human
// semantic acceptance step (statement_signoff_matrix_test.go).
//
// FOUR DOCUMENTS, ONE TEMPLATE, ONE PARAMS DOCUMENT, FOUR DATA
// DOCUMENTS THAT DIFFER ONLY IN THE LENGTH OF THE BOUND COLLECTION.
// That last clause is AC1(ii) and it is the whole reason this family is
// not built the way fixtures/page-count-* is. page-count-N places N
// single-line elements one content-window apart, so its page count is a
// GEOMETRIC CONSTRUCTION: it is accidentally true at any N and says
// nothing whatever about a table. Here the page count is a CONSEQUENCE
// of how many rows the data carries, which is the property a statement
// actually has.
//
// THE ROW ARITHMETIC, MEASURED (not asserted from theory):
//   - A4 portrait, margins 36pt, pageHeader band 54pt, pageFooter band
//     34pt  =>  content band 681.89pt.
//   - The table starts 54pt down the content band and its header row is
//     28pt, so page 1 has 611.89pt of row space and every continuation
//     page has 653.89pt.
//   - A single-line row is 28.88pt tall (8pt Noto Sans line box plus
//     8pt padding top and bottom).
//
// TWO FACTS, AND THEY ARE NOT THE SAME FACT. This file recorded only the
// second of them and called it the first, and the story file recorded a
// band table that was off by one at both ends (this story's review,
// Finding 5). Both are re-measured here:
//
//   - HOW MANY ROWS ARE PLACED ON PAGE 1: NINETEEN. Read off the
//     artifact — TestStatementDocumentsRenderTheirDeclaredPageCount logs
//     the partition, and for statement-5 it is [19 22 22 22 10]. Every
//     continuation page places 22. This is the number
//     statementRowsOnFirstPage in statement_semantics_test.go states,
//     and the two are now consistent rather than contradictory.
//   - WHERE THE PAGE COUNT STEPS: one row LOWER at both ends, because
//     the sum footer aggregate consumes a row slot on the FINAL page.
//     So the largest collection that still fits in P pages is
//     18 + 22*(P-1), not 19 + 22*(P-1).
//
// Measured directly, by rendering at each band edge:
//
//	n =   18 -> 1 page      n =   19 -> 2 pages
//	n =   84 -> 4 pages     n =   85 -> 5 pages
//	n =  106 -> 5 pages     n =  107 -> 6 pages
//	n =  414 -> 19 pages    n =  415 -> 20 pages
//	n =  436 -> 20 pages    n =  437 -> 21 pages
//	n = 1074 -> 49 pages    n = 1075 -> 50 pages
//	n = 1096 -> 50 pages    n = 1097 -> 51 pages
//
// So the true bands are 1..18 / 85..106 / 415..436 / 1075..1096, and the
// four collection lengths below sit in the MIDDLE of their bands rather
// than at an edge — 10 above the lower edge and 11 below the upper — so
// the footer aggregate's own orphan-avoidance (Story 4.5) cannot tip a
// document into an extra page.
//
// WHAT EACH DOCUMENT DISCRIMINATES (D-4.5.5 — a golden that would look
// identical under a broken implementation is a hash of nothing):
//   - statement-1  : the discriminating rows themselves, and the ONLY
//                    document where "header on every page" is
//                    accidentally true (it has one page). It is in the
//                    family so that AC2(i)'s part (a) has a subject to
//                    exclude.
//   - statement-5  : the smallest document with continuation pages, so
//                    the first that can express a header that does not
//                    repeat, or a footer aggregate emitted on page 1.
//   - statement-20 : the LARGEST document a person can realistically
//                    read end to end at a re-attestation, and the one
//                    the sign-off's own `examined` instructions give
//                    that role. It crosses the page-9-to-page-10 digit
//                    boundary in "Page X of Y" (D-2.7.2) with a table
//                    present — but so does statement-50, recorded in
//                    the same commit, so that alone is NOT this
//                    fixture's reason to exist (this story's review,
//                    Finding 15) and a future reader deciding whether
//                    it may be retired should not be told it is.
//   - statement-50 : the only document in the repository where CJK
//                    subsetting happens at volume (41 distinct CJK
//                    glyphs), and the subject DW-14's /ToUnicode
//                    prediction is measured against.
//
// NOT SET, DELIBERATELY: `altRowBackground`. It is declared in the
// schema and applied by nobody; Story 4.8 owns it. Setting it here
// would move these bytes the moment 4.8 lands.

import (
	"fmt"
	"strings"
)

// statementLogoAssetKey is the SHA-256 of the 128-byte PNG below — a
// 48x32 RGB mark, drawn as a white "F" on a dark blue field. It is
// deliberately a recognisable SHAPE rather than a solid block: the
// human sign-off (AC8) is asked to confirm the logo "is present and not
// a black box", and a solid rectangle makes that question unanswerable.
const statementLogoAssetKey = "f4a37bba5652865abc8e24be5e1aad4d5ad42ce5727715f6d19b93861d23f6a4"

// statementTemplateJSON is fixtures/statement-*/input.folio, kept
// BYTE-IDENTICAL to all four copies by hand — multi_page_template.go's
// and page_count_matrix_templates.go's established discipline. All four
// fixture directories carry the SAME template; only their data document
// differs (AC1(ii)).
const statementTemplateJSON = `{
  "assets": {
    "` + statementLogoAssetKey + `": {
      "data": [
        "iVBORw0KGgoAAAANSUhEUgAAADAAAAAgCAIAAADbtmxLAAAAR0lEQVR42u3XQQ0AIAwEsLnACW7w",
        "LwMkwAPCIF1OQB+3JYtSW6oE0GegfmCAgIBWQOnWHug50K4KAwEBuUNAQEC3QT5XoGkGGMiUzlsM",
        "R5wAAAAASUVORK5CYII="
      ],
      "mediaType": "image/png"
    }
  },
  "bands": {
    "content": {
      "elements": [
        {"id": "e5", "type": "text", "x": 0, "y": 0, "width": 300, "height": 14, "value": "Customer: {{customer.name}}", "style": {"fontFamily": "body", "fontSize": 10}},
        {"id": "e6", "type": "text", "x": 0, "y": 16, "width": 300, "height": 14, "value": "Account: {{account.number}}", "style": {"fontFamily": "body", "fontSize": 10}},
        {"id": "e7", "type": "text", "x": 0, "y": 32, "width": 420, "height": 14, "value": "Statement period: {{period.from}} to {{period.to}}", "style": {"fontFamily": "body", "fontSize": 10}},
        {"id": "e8", "type": "table", "x": 0, "y": 54, "bind": "transactions[]", "as": "txn", "headerHeight": 28,
          "style": {"fontFamily": "body", "fontSize": 8, "padding": {"bottom": 8, "left": 3, "right": 3, "top": 8}},
          "columns": [
            {"id": "e9", "label": "Date", "width": 60, "align": "left", "bind": "{{txn.date}}"},
            {"id": "ea", "label": "Reference", "width": 66, "align": "left", "bind": "{{txn.ref}}"},
            {"id": "eb", "label": "Description", "width": 170, "align": "left", "bind": "{{txn.description}}"},
            {"id": "ec", "label": "Note", "width": 34, "align": "left", "bind": "{{txn.note}}"},
            {"id": "ed", "label": "Amount", "width": 84, "align": "right", "bind": "{{formatNumber(txn.amount, \"#,##0.00\")}}", "footer": "sum", "footerFormat": "#,##0.00"}
          ]}
      ]
    },
    "pageFooter": {
      "elements": [
        {"id": "e3", "type": "text", "x": 0, "y": 8, "width": 380, "height": 12, "value": "Confidential - for the named account holder only. Generated {{params.generatedDate}}.", "style": {"fontFamily": "body", "fontSize": 7}},
        {"id": "e4", "type": "text", "x": 400, "y": 8, "width": 123, "height": 12, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "body", "fontSize": 7, "align": "right"}}
      ],
      "height": 34
    },
    "pageHeader": {
      "elements": [
        {"id": "e1", "type": "image", "asset": "` + statementLogoAssetKey + `", "x": 0, "y": 6, "width": 36, "height": 24},
        {"id": "e2", "type": "text", "x": 48, "y": 12, "width": 320, "height": 16, "value": "CUSTOMER ACCOUNT STATEMENT", "style": {"fontFamily": "body", "fontSize": 11}}
      ],
      "height": 54
    }
  },
  "fonts": {"body": ["Noto Sans", "Noto Sans Thai", "Noto Sans SC"]},
  "locale": "en",
  "nextId": 14,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// statementParamsJSON is fixtures/statement-*/params.json — the SUPPLIED
// generated date (AC4). It is the only place in this fixture family
// where the string "2026-08-27" occurs: every transaction date is in
// 2026-07, and the statement period is 2026-07-01..2026-07-31, so a
// date that resolved from the DATA document instead of params could not
// accidentally produce this text. That is AC4's D-000.86 part (a),
// built into the fixture rather than asserted about it.
const statementParamsJSON = `{
  "generatedDate": "2026-08-27"
}
`

// statementGeneratedDate is the value above, restated as a Go literal so
// the assertions do not read it back out of the same string they are
// checking (D-000.68: two independent statements of one number).
const statementGeneratedDate = "2026-08-27"

// --- the discriminating rows (D-4.5.5) ---------------------------------
//
// These five rows are the reason this family can express a defect at
// all, and they are a PREFIX of every one of the four data documents —
// which is what "the four data documents differ ONLY in the length of
// the bound collection" means in practice.
//
// Row 1  three faces in ONE cell under ONE fallback stack. Measured at
//        HEAD: no test and no fixture in the repository put all three
//        shipped scripts inside a table; the table suites use a single
//        -face fontFamily per subtest. This is genuinely new coverage.
// Row 2  a Description that WRAPS, and whose wrap point falls BETWEEN
//        TWO WORDS ("...for the" / "regional office"). A cell one
//        physical line longer than its column would not discriminate a
//        wrong break; this one does, and the wrap point is named in the
//        assertion.
// Row 3  Thai whose break is asserted against the FROZEN S4 corpus:
//        thai-001, "ประเทศไทย", labelled words ["ประเทศ","ไทย"],
//        expectedBreaks [6]. The Note column's 34pt width (28pt usable
//        after padding) was chosen so this string CANNOT fit on one
//        line while its first labelled word CAN — the width is the
//        knob, not the string set.
// Row 4  the lowered/stacked Thai mark case, "ปั ฟั ที่ ป้ำ", verbatim
//        from fixtures/shaped-text/input.folio element e1 — already
//        human-signed at that fixture's digest. GPOS inside a cell.
// Row 5  thai-002, "เก็บเงิน". Its labelled seam is NOT asserted, and
//        the reason is now a MEASUREMENT rather than an inference.
//
//        As first recorded, the reason given was that thai-002 "presents
//        six advancing glyphs — the same six as thai-001's first word
//        ประเทศ". The glyph COUNT is right (8 runes less two zero-advance
//        marks) but the argument is not: the six glyphs are different
//        glyphs (เ ก บ เ ง น against ป ร ะ เ ท ศ), and line breaking is
//        decided by ADVANCE WIDTH, not by glyph count. This story's
//        review, Finding 13, and it was right.
//
//        Measured instead, by sweeping the Note column's declared width
//        and reading the breaks and the TEXT_CLIPPED_WIDTH diagnostics
//        off the render:
//
//	          thai-001's first labelled word "ประเทศ"  23,704 mp
//	          thai-002 "เก็บเงิน", whole string        23,200 mp < A <= 23,400 mp
//	                                                   (fits at a content width of
//	                                                    23,400; breaks at 23,200)
//
//        Breaking thai-002 at its labelled seam therefore requires a
//        column content width below ~23,400 mp, and "ประเทศ" does not fit
//        in anything under 23,704 mp — so every width that breaks
//        thai-002 CLIPS thai-001's first word and raises
//        TEXT_CLIPPED_WIDTH, which AD-14's no-unasserted-diagnostic
//        precondition turns into a fatal. The two windows are disjoint
//        by at least 304 mp. The conclusion the first recording reached
//        is correct; the reason it gave did not establish it.
//
//        The row is kept because it is a second, independent Thai cell
//        the human reader examines; it is not counted as a second
//        break observable.
//
// Row 3 is the ONLY break this family asserts against the frozen
// corpus, and one is what AC5 requires ("at least one"). thai-007,
// thai-008 and thai-009 are recorded DIVERGENCES in break-signoff.json
// — the human label and the engine deliberately disagree — and are
// therefore not used at all.

// statementDiscriminatingRows is that prefix, in order.
var statementDiscriminatingRows = []statementRow{
	{"2026-07-02", "TXN-0001", "Opening balance Ada ก 汉", "", "1250.10"},
	{"2026-07-03", "TXN-0002", "Quarterly maintenance retainer for the regional office", "", "480.33"},
	{"2026-07-05", "TXN-0003", "Domestic transfer", "ประเทศไทย", "75.07"},
	{"2026-07-07", "TXN-0004", "Card issue fee", "ปั ฟั ที่ ป้ำ", "12.51"},
	{"2026-07-09", "TXN-0005", "Interest credit", "เก็บเงิน", "33.99"},
}

// statementFillerCents is the cent part of every filler row's amount,
// cycled, and it is the reason this family's money column can express a
// decimal-arithmetic defect at all.
//
// THE DEFECT THIS EXISTS TO CATCH, and the history that put it here.
// As first recorded, every amount in all four documents was an exact
// multiple of 0.25 — cent parts drawn from {0, 25, 50, 75}. Every such
// value is EXACTLY REPRESENTABLE in binary floating point, so a renderer
// whose money path went through float64 would have produced BYTE-
// IDENTICAL goldens and AC2(iii) would still have passed. The check was
// exact; the FIXTURE was not (D-4.5.5: a behaviour is only tested by an
// input that would render differently under a wrong implementation).
// That is Story 4.5's money-path failure one notch up — 4.5's column
// reached the default only with integral values; quarter-integral is the
// same thing to a binary float.
//
// NONE of these cent parts is a multiple of 25, so NONE of the amounts
// is exactly representable in float64. The set is measured, not chosen
// for looks: with these values the canonical naive money path — parse
// each amount with strconv.ParseFloat, accumulate in a float64, scale to
// minor units with int64(total*100) — renders a DIFFERENT total from the
// exact-decimal path in ALL FOUR documents, and a different Amount cell
// for dozens of individual rows. Under the old all-quarter values it
// rendered every cell and every total identically.
// TestStatementMoneyColumnDiscriminatesFloatArithmetic is that property,
// asserted permanently, so a future edit to this list cannot quietly
// take the teeth back out.
var statementFillerCents = []int{90, 60, 84, 67, 36, 3, 51, 20, 61, 99}

// statementCJKDescriptions are ordinary Chinese banking descriptions
// cycled through the filler rows. They carry FORTY distinct CJK glyphs
// between them (forty-one with row 1's 汉), which is what makes
// statement-50 the first document in the repository where CJK
// subsetting happens at any volume at all — AC7's "name and measure
// what is new".
//
// They are DELIBERATELY NOT taken from fixtures/expected-breaks/. That
// file is S4, the frozen authority Thai and CJK break correctness is
// judged against for the life of the project, and putting one of its
// labelled items into an unasserted cell of a second signed artifact
// would create a second apparent authority over the same rules. Thai
// reuses the frozen set BECAUSE AC5 asserts against it; CJK asserts
// nothing here, so it uses ordinary text.
var statementCJKDescriptions = []string{
	"转账手续费",
	"存款利息",
	"自动扣款",
	"网上银行支付",
	"月度服务费",
	"现金提取",
	"工资入账",
	"退款处理",
	"外币兑换",
	"定期存单到期",
}

// statementRow is one transaction, spelled exactly as the data document
// spells it. Amount is a STRING holding an exact decimal literal, never
// a Go float: AD-23 exists because this is the money column, and a
// float would put the bug this fixture exists to catch into the fixture
// itself.
type statementRow struct {
	Date, Ref, Description, Note, Amount string
}

// statementFixture is one of the four recorded documents.
type statementFixture struct {
	slug  string
	rows  int
	pages int
}

// statementFixtures is the declarative list every statement test drives.
//
// The row counts are MEASURED, not derived: each sits in the middle of
// the band of collection lengths that produces its page count, so
// neither the footer aggregate's orphan-avoidance nor a one-row miscount
// can silently move a document to a different page count.
//
// The bands are measured at their edges (see this file's header). Page 1
// PLACES 19 rows and every continuation page places 22; the band edges
// sit one lower than that arithmetic alone would give, because the sum
// footer aggregate reserves a row slot on the final page. Both facts are
// needed, and recording only one of them is what made this comment and
// statementRowsOnFirstPage read as a contradiction.
//
//	1 page  : 1..18      -> 5    (the discriminating rows, and nothing else)
//	5 pages : 85..106    -> 95
//	20 pages: 415..436   -> 425
//	50 pages: 1075..1096 -> 1085
var statementFixtures = []statementFixture{
	{"statement-1", 5, 1},
	{"statement-5", 95, 5},
	{"statement-20", 425, 20},
	{"statement-50", 1085, 50},
}

// statementRows returns the n transactions of the document whose
// collection length is n: the five discriminating rows above, then
// deterministic filler.
//
// The filler is a CONSTRUCTION, spelled out here rather than pasted as
// a 160KB literal, and the construction is what the on-disk data.json
// is checked against (TestStatementFixtureDataMatchesTheInRepoBuilder)
// — the same byte-identity discipline page-count-*'s template constants
// carry, with a pure function in place of a constant.
func statementRows(n int) []statementRow {
	out := make([]statementRow, 0, n)
	for _, r := range statementDiscriminatingRows {
		if len(out) == n {
			return out
		}
		out = append(out, r)
	}
	for i := len(statementDiscriminatingRows) + 1; len(out) < n; i++ {
		desc := fmt.Sprintf("Settlement line item %d", i)
		if i%3 == 0 {
			desc = statementCJKDescriptions[(i/3-1)%len(statementCJKDescriptions)]
		}
		out = append(out, statementRow{
			Date:        fmt.Sprintf("2026-07-%02d", ((i-1)%28)+1),
			Ref:         fmt.Sprintf("TXN-%04d", i),
			Description: desc,
			Note:        "",
			Amount:      fmt.Sprintf("%d.%02d", 10+i, statementFillerCents[(i-1)%len(statementFillerCents)]),
		})
	}
	return out
}

// statementDataJSON builds the data document for a collection of n
// transactions, in the exact byte spelling committed as
// fixtures/statement-*/data.json.
//
// It is hand-assembled rather than produced by encoding/json because
// the committed file must be byte-stable across Go versions and because
// the amounts must reach the parser as EXACT DECIMAL LITERALS — Go's
// encoding/json would route a numeric field through float64, which is
// precisely the failure AD-23 exists to prevent, in the money column,
// in the fixture the money column's correctness is recorded from.
//
// It carries NO date field of its own beyond the transaction dates and
// the statement period, all of which are in 2026-07 (AC4 part (a)).
func statementDataJSON(n int) string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "customer": {"name": "Ada Lovelace"},` + "\n")
	b.WriteString(`  "account": {"number": "TH-4471-0092-8836"},` + "\n")
	b.WriteString(`  "period": {"from": "2026-07-01", "to": "2026-07-31"},` + "\n")
	b.WriteString("  \"transactions\": [\n")
	rows := statementRows(n)
	for i, r := range rows {
		b.WriteString(fmt.Sprintf(
			`    {"date": %s, "ref": %s, "description": %s, "note": %s, "amount": %s}`,
			jsonStringForStatement(r.Date), jsonStringForStatement(r.Ref),
			jsonStringForStatement(r.Description), jsonStringForStatement(r.Note),
			r.Amount,
		))
		if i != len(rows)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("  ]\n}\n")
	return b.String()
}

// jsonStringForStatement quotes s as a JSON string WITHOUT escaping
// non-ASCII: the Thai and CJK cells must be readable in the committed
// data document, because a human reads that document when deciding
// whether the sign-off in AC8 is honest.
func jsonStringForStatement(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// statementExpectedSumMinor returns the exact sum of the n rows'
// amounts, in MINOR UNITS (hundredths), computed by integer arithmetic
// on the decimal literals — never through a float.
//
// AD-23, stated because this is the check that catches the bug AD-23
// exists for: "the sum footer's independently-computed check value must
// be computed as an exact decimal too, or the check inherits the bug it
// exists to catch."
func statementExpectedSumMinor(n int) int64 {
	var total int64
	for _, r := range statementRows(n) {
		total += statementAmountMinor(r.Amount)
	}
	return total
}

// statementFormatMinor renders a minor-unit total in the "#,##0.00"
// shape the Amount column's footerFormat declares — grouped by
// thousands, two decimal places — by integer arithmetic, so the
// expectation is independent of the renderer's own formatter.
func statementFormatMinor(minor int64) string {
	neg := minor < 0
	if neg {
		minor = -minor
	}
	cents := minor % 100
	whole := minor / 100
	digits := fmt.Sprintf("%d", whole)
	var grouped strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(c)
	}
	out := fmt.Sprintf("%s.%02d", grouped.String(), cents)
	if neg {
		out = "-" + out
	}
	return out
}
