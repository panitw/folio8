// Package template owns the `.folio` document model, its parser and its
// serializer (AD-9: "internal/template owns both the parser and the
// serializer"). It never imports "os" (D-1.4.6: LoadTemplate's path
// argument is handled entirely in package folio8 at the module root) and
// never imports internal/pdf (AC25: the two packages' numeric-spelling
// functions are deliberately unshared).
//
// Every exported type here is Story 1.4's document model. Report data
// (AD-23's exact scaled decimals) is NOT modelled here — that type
// belongs to Story 1.6 (D-1.4.3: "1.4 must not build it").
package template

import (
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// Document is the parsed, canonicalised form of a `.folio` file. Every
// slice and map field is initialised to non-nil empty by the parser and
// by any constructor (D-1.4.3 extended: a nil map/slice with omitempty
// removed serializes to "null", not "{}"/"[]" — the fifth trap, AC23).
type Document struct {
	// Version is carried verbatim (D-1.4.13): never lowered, never
	// gratuitously raised. It is a plain string ("MAJOR.MINOR") and so
	// carries no round-trip hazard of its own (D-1.4.3).
	Version string

	// Locale is one of the closed set en/th/zh-Hans/ja (AD-12).
	Locale string

	// UTCOffset matches ±HH:MM.
	UTCOffset string

	Page  Page
	Fonts Fonts

	// Bands has exactly the three keys content/pageFooter/pageHeader.
	Bands Bands

	// Pages is SPEC-multi-pages' designed pages (D-G.1). Nil for a one-page
	// document, whose content lives in Bands.Content. When set it holds
	// EVERY page, page 1 included, and Bands.Content carries no elements and
	// no section break. Read content through ContentBands, never through
	// Bands.Content directly.
	Pages []ContentPage

	// Assets is keyed by lowercase hex SHA-256 of the raw bytes.
	Assets map[string]Asset

	// UnbreakableValues is the document's declaration of which bound
	// values must never be split across a line break (Story 2.4;
	// D-2.1.6 OWNER, D-2.4.1). Each entry is a BARE ROOT-RELATIVE
	// DOTTED DATA PATH, spelled exactly as `footerOf` is (D-1.4.1: "a
	// bare root-relative dotted value path… No `{{ }}`, no function
	// call, no `[]`") — one path convention in the format, not two.
	// Row-scoped paths are written root-relative under the same
	// convention.
	//
	// DOCUMENT-LEVEL, NOT ELEMENT-LEVEL, AND THE FORMAT'S OWN EXAMPLE
	// IS WHY (D-2.4.1). folio-format.md defines a text element's
	// `value` as a string "which may contain {{ }} bindings", and both
	// canonical examples MIX literal text with bindings — "Statement
	// for {{customer.name}}". An element-level flag would forbid
	// breaking between "Statement" and "for", breaking wrapping for
	// exactly the shape the specification demonstrates. The property
	// belongs to the DATA, not to a box: if customer.name holds a name
	// in the header it holds one in the footer, so it is declared once.
	//
	// The engine NEVER infers membership of this list. That is the
	// whole point of D-2.1.6: Thai surnames are coined by law out of
	// ordinary dictionary words, so no dictionary-coverage rule can
	// tell a proper noun from its parts. See internal/text's package
	// doc for the mechanism and its disclosed limitation.
	//
	// Optional and ADDITIVE: absent from a document that does not use
	// it, and round-tripped as an absent key. A new optional key is a
	// MINOR addition under D-1.4.12 — it is a list, not an extension of
	// a closed set of values — and D-1.4.9's passthrough already
	// guarantees an older library loads a file carrying it.
	//
	// Authored order is preserved (order carries no meaning, and
	// preserving it keeps load/save a fixed point without reordering an
	// author's file). Duplicates are a load error.
	UnbreakableValues []string

	// EmbedFonts is the document's declaration of whether a SAVE carries
	// the faces its chains name, or merely names them
	// (spec-font-sources-and-embedding CAP-2).
	//
	// NOTHING READS IT YET. It has a loader, a serializer, a command and a
	// projection, and no consumer at all: no embedding decision, no
	// stripping, no resolution rule and no renderer branch turns on it.
	// That is deliberate — the setting exists, round-trips and undoes in
	// this story, and the behaviour it governs is a later one.
	//
	// TRUE IS THE DEFAULT AND IT IS THE ABSENT STATE. Every document
	// written before this field existed embeds, so an absent key must keep
	// meaning that: ParseDocument seeds `true` BEFORE it looks for the key,
	// and writeDocument emits nothing when it is true. The consequence is
	// that a Document built as a literal rather than parsed starts at
	// `false` and would serialize the key — every production Document comes
	// from ParseDocument, and a hand-built one in a test that cares must set
	// it.
	//
	// IT MOVES NO FORMAT VERSION. An older reader meets an unknown top-level
	// key, carries it through verbatim (the passthrough rule Extra
	// implements below) and renders the document identically, because the
	// key governs only what a save WRITES. A file declares the lowest
	// version its own content requires, and this content requires nothing.
	EmbedFonts bool

	// NextID is the next element-id counter value, decimal (AC32).
	NextID int64

	// Extra carries unknown top-level keys opaquely (AC8), sorted by
	// byte-order key (AC18).
	Extra []Field
}

// Field is one opaque, unknown key/value pair captured by the
// passthrough store at some nesting level (AC8, AC9).
type Field struct {
	Key   string
	Value RawValue
}

// Page is the document's page setup.
type Page struct {
	// Margin is required in this story's model — folio-format.md's
	// worked example always carries all four edges and no default is
	// documented for an absent margin (unlike style.padding, whose
	// default of 0 IS documented). A future story may relax this.
	Margin Margin

	// Orientation is "portrait" or "landscape".
	Orientation string

	// Size is either a named page size ("A4", "Letter") or a custom
	// {height,width} object. Exactly one of SizeName/SizeCustom is set.
	SizeName   string
	SizeCustom PageSize
	SizeIsName bool

	// Extra carries unknown keys on the page object opaquely (AC8,
	// D-1.4.9 OWNER — this story's finisher review, Finding 2: unlike
	// `bands`, folio-format.md states no closed key set for `page`, so
	// there is no ruling authorising a refusal here).
	Extra []Field
}

// PageSize is a custom page size in points.
type PageSize struct {
	Height geom.Length
	Width  geom.Length
}

// Margin holds the four page-margin edges, in points.
type Margin struct {
	Top, Right, Bottom, Left geom.Length

	// Extra carries unknown keys on page.margin opaquely (AC8, D-1.4.9
	// OWNER; this story's finisher review, Finding 2).
	Extra []Field
}

// Padding holds the four style-padding edges, in points. Each edge is
// individually optional (the worked example's `"padding": {"left": 3,
// "right": 3}` omits top/bottom) — an omitted edge means "use the
// documented default, 0" for layout purposes, but the key itself stays
// absent on serialize (P3: canonical is a fixed point, so an omitted
// edge must round-trip as omitted, not reappear as an authored "0").
type Padding struct {
	Top, Right, Bottom, Left Presence[geom.Length]

	// Extra carries unknown keys on style.padding opaquely (AC8, D-1.4.9
	// OWNER; this story's finisher review, Finding 2).
	Extra []Field
}

// FontChainEntry is ONE entry of a fallback chain, and it has exactly
// THREE shapes since Story 11.2 (Story 8.3, FR53/FR56; FR57): a face
// NAME the renderer is handed at render time, a reference to a face
// carried INSIDE the document as an `assets` entry (`{"asset": "<key>"}`),
// or either of those written as an object carrying optional
// STYLE-VARIANT SIBLINGS from the closed set `bold`, `italic`,
// `boldItalic`.
//
// The two KINDS are discriminated by which of Face/AssetKey is
// non-empty, and exactly one of them ever is — decodeFontChainEntry
// refuses an empty asset key and an empty face alike and enforces
// exactly-one-of at the object form, so `Face != ""` and
// `AssetKey != ""` partition the type rather than merely overlapping
// it. Embedded() is THE predicate; a caller that writes
// `e.AssetKey != ""` itself is writing the same test a second time.
//
// THE MECHANISM CHANGED AT STORY 11.2; THE PROPERTY DID NOT. Until this
// story the object form was EXACTLY one key, and decodeFontChainEntry
// said in its own words what that bought: an unknown key cannot ride
// along disguised as a decoration. That property is preserved here by an
// exactly-one-of discriminant over a CLOSED key set — `face`|`asset`
// plus the three siblings and nothing else — so an entry object carrying
// any other key is still a located load error, still never passthrough,
// and the struct still has no Extra. What is no longer true is only the
// CARDINALITY: an entry of an unknown KIND is still refused, but a known
// entry may now carry a known, enumerated decoration.
//
// THE SET IS CLOSED AND EXTENDING IT LATER IS A MAJOR CHANGE. That price
// is the reason an open sub-object (`{"face":"X","variants":{…}}`) was
// considered and rejected: it would have surrendered the unknown-key
// refusal that is the whole property this shape exists to preserve.
// folio-format.md states the closure and its price.
//
// A SIBLING'S NAMESPACE MATCHES ITS ENTRY'S DISCRIMINANT (AD-8). A
// `face` entry's siblings are FontSet face names; an `asset` entry's are
// `assets` keys. Nothing here is ever PARSED or CONSTRUCTED from a face
// name: `Face + " Bold"` is the naming-convention weight carrier written
// backwards, and it is forbidden on identical grounds.
//
// It is a struct rather than an interface or a `any` because it crosses
// the parse/serialize/project boundary three times and every crossing
// wants the discriminant checkable at compile time.
type FontChainEntry struct {
	// Face is the name of a face the FontSet supplies. Non-empty exactly
	// when this entry is a plain JSON string in the file, or an object
	// whose discriminant is `face`.
	Face string
	// AssetKey is the `assets` key of a face the document carries.
	// Non-empty exactly when this entry is a `{"asset": …}` object.
	AssetKey string

	// Bold, Italic and BoldItalic are the three OPTIONAL style-variant
	// siblings. Each names a face of the SAME KIND as this entry's
	// discriminant — a FontSet face name on a `face` entry, an `assets`
	// key on an `asset` entry.
	//
	// PLAIN STRINGS, MIRRORING Face AND AssetKey: `""` means absent, and
	// an empty string is refused at parse, exactly as those two already
	// work. They are deliberately NOT Presence[string]: Presence exists
	// so an explicit JSON `null` can be told apart from an absent key,
	// and this entry has no such three-valued key anywhere — importing
	// the idiom here would make FontChainEntry the model's only
	// mixed-idiom struct for no gain.
	Bold       string
	Italic     string
	BoldItalic string
}

// FontStyle names the (weight, slope) a text element asks its chain for.
// It is the CLOSED set the variant siblings answer, and it exists so the
// engine never carries two loose booleans past the one place they are
// read off Style.
type FontStyle uint8

const (
	// FontStyleRegular is "no variant requested": the entry's own face.
	FontStyleRegular FontStyle = iota
	FontStyleBold
	FontStyleItalic
	FontStyleBoldItalic
)

// fontChainVariants is THE authority for the closed variant set: the
// file's key spelling, the FontStyle it answers, and the struct field it
// lands in, tied together in ONE table and in the FIXED ORDER every walk
// visits them.
//
// Every consumer derives from this — the parser's refusal messages, the
// serializer's sibling order, the embedded-face index's third
// determinism axis, and the resolver's lookup — so none of them can name
// a set the others no longer enforce.
var fontChainVariants = []struct {
	key   string
	style FontStyle
	field func(*FontChainEntry) *string
}{
	{"bold", FontStyleBold, func(e *FontChainEntry) *string { return &e.Bold }},
	{"italic", FontStyleItalic, func(e *FontChainEntry) *string { return &e.Italic }},
	{"boldItalic", FontStyleBoldItalic, func(e *FontChainEntry) *string { return &e.BoldItalic }},
}

// fontChainVariantKeys returns the closed set's keys in its fixed order.
// It is derived, never a second list.
func fontChainVariantKeys() []string {
	out := make([]string, 0, len(fontChainVariants))
	for _, v := range fontChainVariants {
		out = append(out, v.key)
	}
	return out
}

// Embedded reports whether this entry names a face the document carries
// rather than one the renderer is given. It is the ONE place the
// discriminant is spelled.
func (e FontChainEntry) Embedded() bool { return e.AssetKey != "" }

// SerialisesAsObject reports whether this entry's file shape is an
// OBJECT rather than a bare string.
//
// IT IS ONE PREDICATE WITH TWO CONSUMERS, AND THAT IS THE WHOLE POINT
// (Story 11.2). writeFontChain decides the emitted shape with it, and
// fontsRequireMajor decides the saved version with it. Both used to
// spell `entry.Embedded()`, and the two agreed only because object-form
// and embedded were the same set. The variant siblings separate them:
// `{"face":"Roboto","bold":"Roboto Bold"}` has an empty AssetKey, so the
// old version predicate would have stamped `1.0` on a document no 1.x
// reader can decode — a version that lies. Sharing the predicate makes
// disagreement unrepresentable.
func (e FontChainEntry) SerialisesAsObject() bool {
	if e.Embedded() {
		return true
	}
	for _, v := range fontChainVariants {
		if *v.field(&e) != "" {
			return true
		}
	}
	return false
}

// Variant returns the face name or assets key this entry declares for s,
// or "" when it declares none. FontStyleRegular is the entry's own face,
// which is never a variant and is answered "" here — the caller already
// holds it.
//
// ABSENCE IS A FIRST-CLASS RESULT. "" means the entry declares no face
// for that weight and slope, and the ruled answer to that is the entry's
// OWN base face plus a Warning — never a walk down the chain hunting for
// something bold, and never a name constructed from this one.
func (e FontChainEntry) Variant(s FontStyle) string {
	for _, v := range fontChainVariants {
		if v.style == s {
			return *v.field(&e)
		}
	}
	return ""
}

// EmbeddedAssetKeys returns every `assets` key this entry names — the
// discriminant first, then its declared style-variant siblings in the
// closed set's FIXED ORDER. It is empty for a `face` entry, whose
// siblings are FontSet face names and name no asset at all.
//
// The order is load-bearing: newEmbeddedFaceIndex walks chains in sorted
// name order and entries in authored order, and the siblings are its
// THIRD axis.
func (e FontChainEntry) EmbeddedAssetKeys() []string {
	if !e.Embedded() {
		return nil
	}
	out := make([]string, 0, 1+len(fontChainVariants))
	out = append(out, e.AssetKey)
	for _, v := range fontChainVariants {
		if key := *v.field(&e); key != "" {
			out = append(out, key)
		}
	}
	return out
}

// FaceEntry and AssetEntry build the two shapes. They exist so a caller
// never writes a bare composite literal whose field choice IS the
// discriminant — `FontChainEntry{Face: name}` and
// `FontChainEntry{AssetKey: name}` differ by one word and mean opposite
// things.
func FaceEntry(face string) FontChainEntry { return FontChainEntry{Face: face} }

// AssetEntry builds the embedded shape; see FaceEntry.
func AssetEntry(key string) FontChainEntry { return FontChainEntry{AssetKey: key} }

// Fonts maps a fallback-chain name to its ordered list of entries.
// The chain's own array order is authored and preserved verbatim; only
// the map's keys are sorted at serialize time (AC18).
//
// The element type was []string until Story 8.3. It is []FontChainEntry
// now because a chain may name a face the document itself carries, and
// a string could only ever name a face the renderer already ships — a
// document that wanted any other typeface was an install instruction,
// not a contract.
type Fonts map[string][]FontChainEntry

// Chain is THE authority for "is this a chain style.fontFamily may name":
// it returns the chain only when the key is PRESENT and the chain is
// NON-EMPTY, because a chain with no entries resolves to no face and so is
// not a family anything may name. A caller that needs the weaker question —
// "is this key declared at all", which a chain-editing command needs so an
// empty chain stays deletable — must index the map itself, deliberately.
//
// It replaces the five open-coded copies of that same two-part test measured
// across the module at b2fdaa1, and exists so a sixth is never written:
//   - folio8.knownFontFamily      (component_commands.go) — the fontFamily property command
//   - folio8.defaultFontFamily    (component_commands.go) — the chain a new text element adopts
//   - folio8.canvasFontChains     (page_setup.go)         — the projected chain list
//   - folio8.lookupFontChain      (render.go)             — a chain by name at render
//
// Each caller keeps its own message text; only the predicate is shared.
// All of them were typed on []string until Story 8.3 and are typed on
// []FontChainEntry now.
//
// ⚠ THE COUNT IS FOUR, NOT FIVE, AND HAS BEEN SINCE STORY 8.4. This
// comment claimed five until Story 11.2 re-measured it: the fifth entry
// it listed — "the table header-style resolver (table_render.go)" — stopped
// being a call site when Story 8.4 routed that resolver through
// folio8.lookupFontChain, which is render.go's own site already counted
// above. Re-measured at 3ad4ede: `grep -rn "Fonts.Chain" --include='*.go'`
// outside _test.go names these FOUR call sites and no fifth.
func (f Fonts) Chain(name string) ([]FontChainEntry, bool) {
	chain, ok := f[name]
	if !ok || len(chain) == 0 {
		return nil, false
	}
	return chain, true
}

// Bands holds exactly the three band keys (AC5). Unlike Page, Margin,
// Padding, Border and Asset, Bands deliberately carries NO Extra field:
// AC5 and folio-format.md (:101, "Exactly these three keys (FR6)") make
// the band-name set itself one of the closed sets this story enforces —
// D-1.4.9's "nothing is refused" governs unknown KEYS inside an object,
// not a structural rule the format's own field table states as closed.
// This story's finisher review (Finding 2) confirmed the other five
// object levels had no such backing and fixed those; bands' closure is
// not the same defect and stays as shipped.
type Bands struct {
	Content    Band
	PageFooter Band
	PageHeader Band
}

// ContentPage is one entry of the top-level `pages` array (SPEC-multi-pages):
// one designed page's content column. Its Band carries Elements, SectionBreak
// and SectionBreakAnchor; Height and Extra are never set, because a page
// entry is a closed key set.
type ContentPage struct {
	Band
	// PageBreak is the page's Page Break setting. A missing value loads as
	// true; page 1's value is ignored and always held as true.
	PageBreak bool
}

// ContentBands returns every designed page's content band in page order:
// the one content band of a one-page document, or each entry of Pages.
func (d *Document) ContentBands() []*Band {
	if len(d.Pages) == 0 {
		return []*Band{&d.Bands.Content}
	}
	out := make([]*Band, len(d.Pages))
	for i := range d.Pages {
		out[i] = &d.Pages[i].Band
	}
	return out
}

// ElementBands returns every band that holds elements, in document order:
// the page header, each page's content band, then the page footer.
func (d *Document) ElementBands() []*Band {
	out := []*Band{&d.Bands.PageHeader}
	out = append(out, d.ContentBands()...)
	return append(out, &d.Bands.PageFooter)
}

// PageCount is the number of designed pages, at least 1.
func (d *Document) PageCount() int {
	if len(d.Pages) == 0 {
		return 1
	}
	return len(d.Pages)
}

// PageField is the file location of page i's content: `bands.content` for
// a document written in the one-page shape, `pages[i]` otherwise.
func (d *Document) PageField(i int) string {
	if len(d.Pages) < 2 {
		return contentBandField
	}
	return fmt.Sprintf("pages[%d]", i)
}

// Band is one of the three template bands. Height is a presence flag:
// content bears none (AC5 — "not on content"), pageHeader/pageFooter
// carry one.
type Band struct {
	Elements []Element
	Height   Presence[geom.Length]
	// SectionBreak is the content band's optional `sectionBreak`
	// (spec-section-break): an offset in points from the band's top. The
	// elements declared at or below it form one section that lands after
	// the content above it. Never set on the page header or page footer —
	// parse_bands.go refuses the key there.
	SectionBreak Presence[geom.Length]
	// SectionBreakAnchor is the content band's optional boolean
	// `sectionBreakAnchor` (spec-section-break CAP-7). Absent means anchored,
	// the default; only an explicit `false` is written on save. Valid only
	// beside SectionBreak — parse_bands.go refuses it without one, and on the
	// page header or page footer.
	SectionBreakAnchor Presence[bool]
	Extra              []Field
}

// ElementType is the closed set of element kinds (FR4).
type ElementType string

const (
	ElementText  ElementType = "text"
	ElementImage ElementType = "image"
	ElementTable ElementType = "table"
	ElementLine  ElementType = "line"
	ElementRect  ElementType = "rect"
	// ElementBarcode is a Code 128 symbol whose bindable `value` is its
	// content (spec-barcode-qr-elements). Extending this closed set is a
	// MAJOR change: a document carrying one declares 4.0.
	ElementBarcode ElementType = "barcode"
	// ElementQRCode is a QR Code symbol whose bindable `value` is its
	// content, encoded at the optional `errorCorrection` level
	// (spec-barcode-qr-elements). It joins the barcode's 4.0 rank.
	ElementQRCode ElementType = "qrcode"
)

// ElementID is the canonical spelling of an element/column id: "e" plus
// a lowercase base-36 counter, e.g. "e1", "ea", "e1z" (AD-10).
type ElementID string

// Element is one of the five element kinds, common fields plus the
// kind-specific extension. A table's Width is its authored proportional
// total when set; an absent Width retains legacy point columns. Its Height
// is always absent (AD-13).
type Element struct {
	ID   ElementID
	Type ElementType

	X, Y          geom.Length
	Width, Height Presence[geom.Length]

	VisibleIf Presence[string]
	Style     Presence[Style]

	// KeepTogether is Story 7.7's author-declared keep-together tag
	// (FR51): elements in the CONTENT band sharing one non-empty tag
	// paginate as ONE indivisible unit — the whole set stays in the
	// window it started in, or the whole set moves to the next.
	//
	// It is an ELEMENT-level key rather than a document-level list of
	// id lists (D-7.7 Ruling B): a document-level list would be a
	// second place element ids appear, and something would have to
	// prune it when a component is deleted. A tag is deleted with its
	// own element and can never dangle.
	//
	// Absent (or explicitly null) is "not grouped", and a document
	// declaring no tag renders byte-identically to one written before
	// this key existed. parse_bands.go refuses a tag on a
	// page-header/page-footer element (FR51 scopes the feature to the
	// content band) and on a `table` element (a table's items already
	// carry a row key, and honouring both would be a second grouping
	// model).
	KeepTogether Presence[string]

	// text, barcode, qrcode
	Value Presence[string]

	// ErrorCorrection is a qrcode's error-correction level, one of
	// QRErrorCorrectionTokens. Absent means M. Never null, and never on any
	// other element type: both are refused at load.
	ErrorCorrection Presence[string]

	// image
	Asset Presence[string]

	// table
	Table Presence[TableExt]

	Extra []Field
}

// TableExt is the table-specific extension (AD-13).
type TableExt struct {
	Bind             string
	As               Presence[string]
	Columns          []Column
	HeaderHeight     geom.Length
	AltRowBackground Presence[string]

	// HeaderStyle is an OPTIONAL Style block governing the header row
	// ONLY, never a data row — Story 4.1, the owner's ruling: the
	// author controls how a header looks, reusing the existing Style
	// vocabulary rather than a bespoke header-only schema. A field it
	// leaves absent falls back to the table's own Style (above), then
	// to that field's documented default. It is a deliberate, RULED
	// extension of R5's otherwise-permanent TableExt field set (Story
	// 4.1's Delivery Log records the ruling by name).
	HeaderStyle Presence[Style]

	// Rules is SPEC-table-rules' interior-line block: which BOUNDARIES
	// inside the table carry a line, and at what width and colour. It is
	// deliberately NOT an `edges` vocabulary — an edge belongs to a cell
	// and a boundary belongs to the table, which is why the old
	// cell-chrome model stroked every interior line twice and made the
	// frame's weight a function of the row count. See TableRules.
	Rules Presence[TableRules]

	// MinHeight is SPEC-table-rules' FLOOR under the table's own box —
	// the ruled area a pre-printed form needs when the data has not
	// arrived yet. It NARROWS AD-13 rather than repealing it: a table
	// still declares no `height`, and its drawn extent is still DERIVED,
	// as max(MinHeight, header + Σ rows + footer). An author cannot
	// shorten a table with a small MinHeight and cannot pin a row to a
	// size.
	MinHeight Presence[geom.Length]
}

// TableRules is `table.rules` — the lines INSIDE a table (SPEC-table-rules
// §2).
//
// A rule is drawn ONCE, at a boundary between two things, and NEVER on the
// table's own edge: the perimeter belongs to the element's own
// `style.border` (which since SPEC-table-rules paints the table's box like
// every other element type's does) and the interior belongs here. That
// split is what makes a heavy frame around a fine grid expressible at all.
//
// Between names BOUNDARIES, from the closed set RuleBoundaryTokens:
// "columns" rules every boundary between two adjacent columns, "rows"
// every boundary between two adjacent rows. Both, either, or an explicit
// empty array for none.
//
// Width and Color default exactly as `style.border`'s do — 0.5pt and
// #000000 — and the render resolves them through the SAME two functions a
// border's sub-keys resolve through, never a second copy of the defaults.
type TableRules struct {
	Width   Presence[geom.Length]
	Color   Presence[string]
	Between Presence[[]string]

	// Extra carries unknown keys on `rules` opaquely, exactly as
	// Border.Extra does for `style.border` (D-1.4.9).
	Extra []Field
}

// Column is one table column.
type Column struct {
	ID    ElementID
	Label string
	Width geom.Length
	// Proportion carries exact thousandths of a dimensionless weight. When
	// set, Width is absent on disk and zero in memory.
	Proportion Presence[int64]
	Align      Presence[string]
	// HeaderAlign aligns this column's HEADER cell only. Absent means the
	// header follows Align (and then the header row's own fallback), so a
	// document that never declares it renders exactly as before. Its own
	// closed set, ColumnHeaderAlignTokens; declaring it requires 3.2.
	HeaderAlign Presence[string]
	Bind        string

	Footer       Presence[string]
	FooterOf     Presence[string]
	FooterFormat Presence[string]

	Extra []Field
}

// Style is the optional per-element style block. Every field is
// optional; an absent field means "inherit the documented default"
// (folio-format.md's Style table).
type Style struct {
	Align      Presence[string]
	Background Presence[string]
	// Color is the INK a text-bearing element prints in (Story 10.1).
	// Background is the box behind it; this is the glyphs themselves.
	// Absent means the PDF's own initial fill colour, black, and emits
	// nothing — which is what leaves every document that declares no
	// colour byte-identical.
	Color      Presence[string]
	Bold       Presence[bool]
	Italic     Presence[bool]
	Border     Presence[Border]
	FontFamily Presence[string]
	FontSize   Presence[geom.Length]
	// LineSpacing is Story 7.2's author-set leading ratio, carried as a
	// WHOLE NUMBER OF THOUSANDTHS (the authored 1.5 is 1500, and the
	// absent default is exactly LineSpacingUnit). It is an int64 count
	// and deliberately NOT a geom.Length: it is dimensionless, and
	// spelling it as a length would invite it into the millipoint
	// arithmetic that AD-2 keeps to one unit.
	//
	// It scales the vertical model's Advance and NOTHING else, so a
	// multi-line element's first baseline — hence its top edge — does
	// not move when it changes (D-2.5a/DW-15's two-model split). Absent
	// emits nothing, which is what leaves every document that declares
	// no spacing byte-identical.
	LineSpacing Presence[int64]
	Padding     Presence[Padding]
	Valign      Presence[string]

	Extra []Field
}

// Border is style.border.
type Border struct {
	Color Presence[string]
	Edges Presence[[]string]
	Width Presence[geom.Length]

	// Extra carries unknown keys on style.border opaquely (AC8, D-1.4.9
	// OWNER; this story's finisher review, Finding 2).
	Extra []Field
}

// FontRecord is the optional `font` object a FONT asset may carry
// (Story 8.3): a record ABOUT the face, for the people reading and
// reusing the document, never something the engine derives from the
// bytes and never something resolution consults — a chain entry names a
// face by ASSET KEY, so nothing here can cause a substitution.
//
// Every field is a Presence because absence and an explicit JSON null
// are different things in this format (presence.go), and a refusal
// written only in the non-null branch would let `"family": null` past
// every guard.
// STORY 8.6 ADDED LicenceText AND Copyright, AND THEY ARE THE FIRST KEYS
// HERE THAT ARE NOT ALWAYS OPTIONAL. The struct still models them as
// Presence, because absence and an explicit null must still round-trip
// distinctly and because an UNREFERENCED font asset may legally carry
// neither — but for an asset a chain names by {"asset": key}, parse.go's
// requireEmbeddedFaceLicence refuses the document unless licence,
// licenceText and copyright are all present, non-null and non-empty. A
// font that travels without its terms is not a font that may be passed
// on, so the format does not accept one.
type FontRecord struct {
	Family  Presence[string]
	Style   Presence[string]
	Licence Presence[string]
	// LicenceText is the ACTUAL TEXT of the licence, not its name.
	// D-8.6.1 settled the "inline on each asset, or one document-level
	// notice block?" question as inline-with-the-text: an asset that is
	// passed on alone must carry its own terms, which a document-level
	// block would not survive. The duplication that costs — three OFL
	// families carrying three copies of ~4 KB of near-identical text —
	// is accepted deliberately for that reason.
	LicenceText Presence[string]
	// Copyright is the copyright line the face's own NOTICE publishes.
	// The unsubsetted face carried in the document also states it in its
	// `name` table (nameID 0), which is a MEASUREMENT and not an
	// assumption — but a reader of the JSON should not have to parse a
	// font binary to find out whose bytes these are, and a check should
	// not have to either.
	Copyright Presence[string]
	Source    Presence[string]

	// Extra carries unknown keys on assets[k].font opaquely (AC8,
	// D-1.4.9 OWNER) — the same passthrough every other object level in
	// this model has. NOT the same as FontChainEntry, which deliberately
	// has none: there the object IS the discriminant.
	Extra []Field
}

// Asset is one embedded binary asset.
type Asset struct {
	Data      []string
	MediaType string

	// Font is the optional `font` record a font asset carries (Story
	// 8.3). Presence, not a bare pointer: `"font": null` is a legal,
	// round-trippable spelling that is NOT the same as the key being
	// absent, and an image asset must keep serializing without the key
	// at all — the six shipped fixtures with a non-empty assets map are
	// the population that proves absence costs no bytes.
	Font Presence[FontRecord]

	// Extra carries unknown keys on one assets[entry] object opaquely
	// (AC8, D-1.4.9 OWNER; this story's finisher review, Finding 2).
	Extra []Field
}
