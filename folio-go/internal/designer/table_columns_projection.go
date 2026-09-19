package designer

// TableColumnsProjection is a bounded paint/editor projection. It deliberately
// carries only the bounded table configuration the focused editor can paint.
// It never contains canonical bytes, sample data, parsed sample shape, or
// aggregate values.
type TableColumnsProjection struct {
	Sizing     string `json:"sizing"`
	TotalWidth int64  `json:"totalWidth"`
	TableID    string `json:"tableId"`
	Collection string `json:"collection"`
	Alias      string `json:"alias"`

	// STORY 12.3 — the table-level header and row properties, twenty-six
	// members since Story 14.8, and the arithmetic is 12 x 2 + 1 + 1.
	//
	// TWO MEMBERS PER HEADER-STYLE FIELD. The committed one is what the
	// document actually declares — empty/zero when the key is absent —
	// so the control can tell SET from UNSET and "clear it back to
	// absent" stays a thing the author can express. The resolved one is
	// what the document will USE, which for an absent field is the
	// table's own `style.<field>` and then that field's documented
	// default. One member cannot serve both: a single resolved value
	// makes absence unrepresentable, and a single committed value
	// forces the browser to run the cascade (D-12.3.1).
	//
	// THE RESOLVED HALF IS THE ENGINE'S OWN ANSWER, not a second
	// implementation of it. resolveHeaderStyle (table_render.go) is the
	// ONE cascade in this program and it is called here, unchanged and
	// unexported, because this file is package folio8 too. Nothing in
	// TypeScript may re-derive, mirror or approximate it (AC2, AC3,
	// AD-15, AD-17).
	//
	// HeaderHeight AND AltRowBackground CARRY ONE MEMBER EACH, and that
	// asymmetry is deliberate — do not "fix" it. A resolved member
	// answers "what will be used when this is absent". HeaderHeight is
	// REQUIRED (parse_bands.go hard-errors on its absence), so it is
	// never absent; AltRowBackground is a flat override on odd
	// zero-based collection indexes with no fallback level of its own,
	// so absence resolves to nothing rather than to an inherited value.
	// For both, the question has no content and committed IS resolved.
	//
	// ABSENCE IS SPELLED AS THE ZERO VALUE, the same convention
	// TableColumnProjection already uses for Binding, Footer, FooterOf
	// and FooterFormat: "" for a string, 0 for a length or a ratio. It
	// is the only spelling available to a wire record whose key set is
	// pinned exactly in both directions (canvas_projection_wire_test.go),
	// where an `omitempty` would make a key appear only sometimes and
	// the browser's hasExactKeys reject exactly those documents. The one
	// place it is lossy is a hand-edited `headerStyle.fontSize: 0`,
	// which no command can write (the arm refuses a non-positive size)
	// and which DW-26 already records as unbounded at the loader.
	//
	// AND, SINCE STORY 11.3, THE TWO BOOLEANS — a SECOND disclosed
	// limit, of the same kind and a wider one. `false` is the zero
	// value, so HeaderBold/HeaderItalic collapse committed-ABSENT with
	// committed-`false`, and unlike the fontSize case a command CAN
	// write the losing value: `tableHeaderStyleFields` carries `bold`
	// and `italic`, and `{"bold": null}` decodes to present(false).
	// The collapse is accepted rather than answered with a tri-state
	// for one field, on D-11.3.3's ground: CanvasProjection is
	// engine<->browser, both in this repo, moving in one commit — it is
	// NOT tag-bound, so a tri-state can be added the day something needs
	// one, and consistency inside one struct beats a second idiom until
	// then. Nothing today needs one: the panel's third state is
	// declared-true-but-no-face, which survives the collapse. AN
	// UNDISCLOSED LIMIT AGES INTO FALSE REASSURANCE, so it is disclosed.
	HeaderHeight     int64  `json:"headerHeight"`
	AltRowBackground string `json:"altRowBackground"`

	HeaderFontFamily          string `json:"headerFontFamily"`
	HeaderFontFamilyResolved  string `json:"headerFontFamilyResolved"`
	HeaderFontSize            int64  `json:"headerFontSize"`
	HeaderFontSizeResolved    int64  `json:"headerFontSizeResolved"`
	HeaderLineSpacing         int64  `json:"headerLineSpacing"`
	HeaderLineSpacingResolved int64  `json:"headerLineSpacingResolved"`
	HeaderBackground          string `json:"headerBackground"`
	HeaderBackgroundResolved  string `json:"headerBackgroundResolved"`
	HeaderColor               string `json:"headerColor"`
	HeaderColorResolved       string `json:"headerColorResolved"`
	HeaderValign              string `json:"headerValign"`
	HeaderValignResolved      string `json:"headerValignResolved"`
	HeaderAlign               string `json:"headerAlign"`
	HeaderAlignResolved       string `json:"headerAlignResolved"`
	HeaderBold                bool   `json:"headerBold"`
	HeaderBoldResolved        bool   `json:"headerBoldResolved"`
	HeaderItalic              bool   `json:"headerItalic"`
	HeaderItalicResolved      bool   `json:"headerItalicResolved"`

	// STORY 14.8's THREE PAIRS, AND THE DOTTED KEY NAMES ARE FORCED RATHER
	// THAN CHOSEN. `tableHeaderStyleFields` spells the authorable attributes
	// `border.width`, `border.color` and `border.edges` — dotted, so that
	// `table.headerStyle.` + field is a path the document actually has —
	// and table_header_style_test.go derives THESE key names from THOSE
	// field names by string transformation (strip `header`, lowercase the
	// first letter) and requires the two sets to be equal. So the dot
	// propagates out of the command spelling, into the json tag, into the
	// browser's hasExactKeys list, and into quoted TypeScript property
	// access. There is no independent naming decision here to make.
	//
	// ⚠ THE RESOLVED TRIO IS NOT A FIELD-BY-FIELD CASCADE, and that is the
	// one thing about these six members a reader has to know.
	// resolveHeaderStyle takes the header's border WHOLE: the moment
	// `headerStyle.border` exists at all, the table's own `style.border`
	// stops contributing, and every sub-key the header does not declare
	// falls to the FORMAT's default (0.5pt, #000000, all four edges) rather
	// than to the table's value for it. These members report that, computed
	// by the same three functions the RENDERER calls (table_render.go's
	// resolvedBorderWidth/Color/Edges) so the panel cannot show a number the
	// PDF does not draw. The panel discloses the block-granularity in words;
	// nothing here softens it.
	//
	// HeaderBorderEdgesResolved IS EMPTY EXACTLY WHEN NOTHING IS PAINTED —
	// both when no border resolves at all and when a declared `edges: []`
	// leaves no side to stroke — and it is the ONLY member that can say
	// so about the PAINT. The colour members spell "no border resolves"
	// as this projection's ordinary absence, "".
	//
	// ⚠ AND THE WIDTH PAIR IS THE ONE PLACE ON THIS PROJECTION WHERE A
	// LENGTH IS SPELLED AS A STRING RATHER THAN AS THOUSANDTHS OF A POINT.
	// "" is absent, "0" is a declared zero, "500" is a declared half point.
	// THE UNITS ARE UNCHANGED — still integer thousandths, exactly as
	// HeaderHeight and HeaderFontSize — only the SPELLING OF ABSENCE moves.
	//
	// The reason is the loader's own, and it is why this pair differs from
	// HeaderHeight rather than being inconsistent with it.
	// internal/template/parse_bands.go says of a border width: "ZERO IS
	// VALID and stays accepted: it is the thinnest device line PDF can
	// draw, not an absent border. Only a NEGATIVE width is refused." So a
	// header border authored as nothing but `{"width": 0}` is a real,
	// declared, painted border — and a numeric member whose absence is
	// spelled 0 cannot tell it apart from a header that declares no border
	// at all. It did not, and the panel therefore told an author "nothing
	// here is set, so this header row takes the table's own border" about a
	// header that had taken the border over. HeaderHeight never needed this
	// spelling because a zero header height is not a meaningful declaration
	// of anything; a zero border width is.
	//
	// BOTH HALVES OF THE PAIR ARE STRINGS, and the resolved half is
	// formatted in Go. Every other pair on this struct shares one type
	// across the pair (HeaderFontSize/HeaderFontSizeResolved are both
	// int64); a committed string beside a resolved number would be the
	// first breach of that, and it would breach it on the one pair a reader
	// is most likely to mis-read. Both-strings also makes the border trio
	// uniform — six string members, "" meaning absent throughout, the same
	// as the colour and edge pairs beside them.
	HeaderBorderWidth         string `json:"headerBorder.width"`
	HeaderBorderWidthResolved string `json:"headerBorder.widthResolved"`
	HeaderBorderColor         string `json:"headerBorder.color"`
	HeaderBorderColorResolved string `json:"headerBorder.colorResolved"`
	HeaderBorderEdges         string `json:"headerBorder.edges"`
	HeaderBorderEdgesResolved string `json:"headerBorder.edgesResolved"`

	// SPEC-table-rules' FOUR MEMBERS: the interior lines and the ruled
	// area's floor, which the owner ruled belong to the TABLE EDITOR and
	// not to the inspector's BOX section (that section already authors
	// the table's own `style.border`/`style.background`, which is now
	// exactly what it says it is).
	//
	// MinHeight is THOUSANDTHS, like HeaderHeight, and 0 is absent — a
	// non-positive floor is refused at the loader, so 0 is unambiguous
	// here in a way a zero border width is not.
	//
	// The RULES trio follows the header-border trio's spellings exactly,
	// and for the same reasons stated at length above: the width pair is
	// STRINGS because a declared `0` is a real, painted, thinnest-device
	// line and a numeric member whose absence is spelled 0 cannot tell it
	// from an absent block; the resolved halves are the format's own
	// defaults, computed by the SAME resolvedBorderWidth/Color the
	// renderer calls; and RulesBetween is the boundary set in the
	// format's own order, comma-joined, "" for none — the same shape
	// HeaderBorderEdges uses, because the browser's guard admits a
	// canonical list and a re-ordered one would be refused.
	//
	// "" ON THE WHOLE TRIO MEANS "THE TABLE DECLARES NO `rules` BLOCK",
	// and that is a disclosed collapse: a hand-edited `rules: {}` or
	// `rules: {"between": []}` reads the same way here. Neither paints
	// anything, and the editor's own way of saying "no rules" is to
	// author none — so nothing an author can express through the panel
	// is lost, and a document that says it in the other spelling still
	// round-trips byte-identically, because the panel writes nothing
	// unless the author changes something.
	MinHeight          int64  `json:"minHeight"`
	RulesWidth         string `json:"rules.width"`
	RulesWidthResolved string `json:"rules.widthResolved"`
	RulesColor         string `json:"rules.color"`
	RulesColorResolved string `json:"rules.colorResolved"`
	RulesBetween       string `json:"rules.between"`

	// THE TABLE'S CELL PADDING, left and right, AS THE DOCUMENT DECLARES IT
	// (`style.padding.left/right`) — committed only, because nothing cascades
	// into a table's own style. Thousandths of a point spelled as STRINGS, on
	// the header-border width's grounds: "" is absent and "0" is a declared
	// zero, which a numeric member could not tell apart. The loader admits a
	// negative length here, so a leading '-' is a legal spelling.
	//
	// PaddingHeaderOverride reports that `headerStyle.padding` exists, which
	// takes the header row's padding WHOLE (resolveHeaderStyle), so these two
	// no longer reach the header. It is deliberately NOT spelled `header…`:
	// every `header*` key on this struct must be a committed/resolved pair
	// (table_header_style_test.go), and this is a flag, not a pair.
	PaddingLeft           string `json:"paddingLeft"`
	PaddingRight          string `json:"paddingRight"`
	PaddingHeaderOverride bool   `json:"paddingHeaderOverride"`

	Columns []TableColumnProjection `json:"columns"`
}

type TableColumnProjection struct {
	Proportion string `json:"proportion"`
	ID         string `json:"id"`
	Header     string `json:"header"`
	Width      int64  `json:"width"`
	Align      string `json:"align"`
	// HeaderAlign is the COMMITTED `columns[].headerAlign`, "" when absent.
	HeaderAlign string `json:"headerAlign"`
	// HeaderAlignResolved is the alignment this column's header cell PRINTS
	// with: columnHeaderAlign over resolveHeaderStyle's fallback, the renderer's
	// own helper, so the cascade `headerAlign` → `align` → `headerStyle.align` →
	// `style.align` → `left` is asked in Go and never re-derived in the browser.
	// Never "". The editor presses it while HeaderAlign is unset (owner,
	// 2026-09-13: press what actually prints).
	HeaderAlignResolved string `json:"headerAlignResolved"`
	Binding             string `json:"binding"`
	RowField            string `json:"rowField"`
	RowFieldEditable    bool   `json:"rowFieldEditable"`
	Footer              string `json:"footer"`
	FooterOf            string `json:"footerOf"`
	FooterFormat        string `json:"footerFormat"`
}
