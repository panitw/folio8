package folio8

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/panitw/folio8/folio8-go/internal/bind"
	"github.com/panitw/folio8/folio8-go/internal/designer"
	"github.com/panitw/folio8/folio8-go/internal/expr"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

// standInValue is the CLOSED set of values a stand-in path may resolve
// to. DECLARATION ORDER IS PREFERENCE ORDER: where a path's contexts
// admit more than one of these, the first declared wins.
//
// The order is not "emptiest first" and that is deliberate (D-13.4.1's
// per-context table). Emptiness is about what a DISPLAYED value renders
// as; applied to a visibility condition it would hide every conditional
// element, and a layout preview exists to show the layout. So: a
// displayed path resolves to something that renders as nothing ("", or
// null, or the typed operand a typed operator demands), a path that only
// gates visibility resolves to true, and where both apply null is the
// one common member — the element hides, and the screen says so.
type standInValue uint8

const (
	// standInEmptyString renders as nothing in a text binding and is the
	// only string upper()/lower() can be handed that adds no ink.
	standInEmptyString standInValue = iota
	// standInTrue shows a conditional element.
	standInTrue
	// standInInstant is formatDate's fixed operand. It precedes
	// standInZero so a path used ONLY under formatDate takes the instant
	// rather than epoch zero.
	standInInstant
	// standInZero is formatNumber's operand, and is also a legal
	// integral epoch-millisecond operand for formatDate
	// (instantMsFromValue's KindNumber arm), which is what keeps a path
	// used under BOTH from being a false refusal.
	standInZero
	standInOne
	// standInNull is the one member a text binding and a visibility
	// condition share: the text renders empty (AD-14) and the element
	// hides, silently (D-3.2.3).
	standInNull
	// standInEmptyCollection is EVERY collection's stand-in, and it is
	// load-bearing rather than incidental — see collectTable.
	standInEmptyCollection
)

// standInValueCount is the member count, DERIVED from the const block's
// last member rather than counted by hand.
const standInValueCount = int(standInEmptyCollection) + 1

// standInSet is a set of standInValue, as a bit set so a per-path
// intersection is one AND.
type standInSet uint16

// standInSetBits is standInSet's width. TestStandInSetHasOneBitPerValue
// ties this constant to the TYPE (a shift past it truncates to zero);
// the compile-time constant below ties the MEMBER COUNT to it.
const standInSetBits = 16

// COMPILE-TIME: one bit per member. A member added past standInSetBits
// makes this constant negative, and an untyped negative constant
// converted to uint does not compile — which is the point. The
// alternative is a member whose bit shifts out of the set at run time,
// silently, so every path demanding it intersects to the empty set and
// a whole template is refused for a reason nothing states.
const _ = uint(standInSetBits - standInValueCount)

func standInSetOf(values ...standInValue) standInSet {
	var set standInSet
	for _, value := range values {
		set |= 1 << value
	}
	return set
}

// first returns the admitted value earliest in declaration order. It
// walks the const block's own range rather than restating its members,
// so a value added between the two bounds is preferred in the position it
// was declared in and cannot be forgotten here.
func (s standInSet) first() (standInValue, bool) {
	for candidate := standInEmptyString; candidate <= standInEmptyCollection; candidate++ {
		if s&(1<<candidate) != 0 {
			return candidate, true
		}
	}
	return 0, false
}

// jsonValue is the value this stand-in marshals to. Nulls appear ONLY
// here, at a leaf: internal/bind's Lookup returns Null for a leaf that is
// an explicit JSON null and Absent for anything it walks through, so a
// null at an intermediate node would be a located BINDING_PATH_ABSENT
// rather than an empty render.
func (v standInValue) jsonValue() any {
	switch v {
	case standInEmptyString:
		return ""
	case standInTrue:
		return true
	case standInInstant:
		return designer.StandInInstant
	case standInZero:
		return 0
	case standInOne:
		return 1
	case standInNull:
		return nil
	case standInEmptyCollection:
		return []any{}
	}
	return nil
}

// standInDemand is one context's requirement on one path: the values that
// context accepts, and how to name it in a refusal.
type standInDemand struct {
	admits standInSet
	label  string
}

// The document-scope demands, measured against the engine rather than
// asserted (see the story's per-context requirement table):
//
//   - a bare text binding: internal/bind/text.go's write switch accepts
//     KindNull, KindString or KindNumber (printed as its exact decimal,
//     owner decision 2026-09-13) and never a boolean. The empty string
//     stays preferred, so a path used only in text still renders nothing;
//     zero and one are admitted so a path shared with formatNumber() or a
//     direct divisor takes that operand's stand-in rather than refusing.
//   - a visibility condition: expr.ConditionValue accepts KindBool or
//     KindNull and nothing else (no truthiness, AD-14).
//   - a bound collection: render.go's checkTableBindings refuses absent,
//     null and non-array — an EMPTY array is explicitly not an error.
var (
	demandText       = standInDemand{standInSetOf(standInEmptyString, standInInstant, standInZero, standInOne, standInNull), "a text binding"}
	demandCondition  = standInDemand{standInSetOf(standInTrue, standInNull), "a visibility condition"}
	demandCollection = standInDemand{standInSetOf(standInEmptyCollection), "a bound collection"}
)

// demandStringOperand is upper()/lower(): evalUpperLower rejects
// anything but KindString, null included.
func demandStringOperand(name string) standInDemand {
	return standInDemand{standInSetOf(standInEmptyString, standInInstant), name + "()'s operand"}
}

// demandNumberOperand is formatNumber(): evalFormatNumber rejects any
// non-KindNumber operand, null and "" included.
func demandNumberOperand(name string) standInDemand {
	return standInDemand{standInSetOf(standInZero, standInOne), name + "()'s operand"}
}

// demandInstantOperand is formatDate(): instantMsFromValue accepts an
// RFC 3339 string or an integral epoch-millisecond number, and nothing
// else — there is no empty form.
func demandInstantOperand(name string) standInDemand {
	return standInDemand{standInSetOf(standInInstant, standInZero, standInOne), name + "()'s operand"}
}

// standInFunctionRule decides what one call demands of the paths beneath
// it. enclosing is the demand the call itself sits under, which only
// if() passes down: if()'s second and third arguments take their
// requirement from the context ENCLOSING the if, never from if itself.
type standInFunctionRule func(g *standInGenerator, call *expr.CallExpr, enclosing standInDemand, site standInSite) error

// standInFunctionRules is the generator's per-function value-kind rule,
// and its KEY SET is the obligation stand_in_data_test.go gates: set
// equality, both directions, against expr.LegalFunctionNames(), with the
// names restated nowhere. Adding a ninth function to the engine's closed
// table reds that test until a rule appears here.
//
// These rules live in package folio8 because selecting suitable preview
// values is a document concern. The engine owns function names and static
// argument constraints; the generator derives the name set from the engine
// and chooses compatible stand-ins for each operand context.
//
// It is assigned in init rather than at declaration because the rules
// recurse back into the walk that dispatches through this very map, and
// Go refuses that as an initialization cycle at declaration scope.
var standInFunctionRules map[string]standInFunctionRule

func init() {
	standInFunctionRules = map[string]standInFunctionRule{
		"sum":          aggregateProjectionRule("sum"),
		"avg":          aggregateProjectionRule("avg"),
		"count":        aggregateCountRule,
		"upper":        scalarOperandRule(demandStringOperand("upper")),
		"lower":        scalarOperandRule(demandStringOperand("lower")),
		"formatNumber": scalarOperandRule(demandNumberOperand("formatNumber")),
		"formatDate":   scalarOperandRule(demandInstantOperand("formatDate")),
		"if":           conditionalRule,
	}
}

// scalarOperandRule walks argument 0 under demand. The pattern argument
// of formatDate/formatNumber is a string literal by grammar and names no
// path.
func scalarOperandRule(demand standInDemand) standInFunctionRule {
	return func(g *standInGenerator, call *expr.CallExpr, _ standInDemand, site standInSite) error {
		return g.walkArg(call, 0, demand, site)
	}
}

// conditionalRule is if(cond, then, else): the condition carries the
// visibility demand, and the two branches inherit the enclosing one.
func conditionalRule(g *standInGenerator, call *expr.CallExpr, enclosing standInDemand, site standInSite) error {
	if err := g.walkArg(call, 0, demandCondition, site); err != nil {
		return err
	}
	if err := g.walkArg(call, 1, enclosing, site); err != nil {
		return err
	}
	return g.walkArg(call, 2, enclosing, site)
}

// aggregateProjectionRule is sum()/avg(): the operand names a collection
// plus a projected field. internal/bind's splitCollectionPath walks the
// path until it meets an array, so the ARRAY must sit at the operand's
// prefix and the trailing segment is the projected field — a field that
// is never read, because the array is empty.
func aggregateProjectionRule(name string) standInFunctionRule {
	return func(g *standInGenerator, call *expr.CallExpr, _ standInDemand, site standInSite) error {
		path, ok := aggregateOperand(call)
		if !ok {
			return nil
		}
		if len(path.Segments) < 2 {
			// A bare collection path handed to sum()/avg() is a located
			// evaluation error with real data too (ProjectCollection's
			// own AC6 arm). The collection is still genuinely there, so
			// declare it and let the engine report the operand as it
			// always has.
			return g.demand(path.Segments, demandCollection, site)
		}
		return g.demand(path.Segments[:len(path.Segments)-1], demandCollection, site)
	}
}

// aggregateCountRule is count(): CollectionLength refuses any segment
// beyond the array, so the WHOLE operand path names the collection.
func aggregateCountRule(g *standInGenerator, call *expr.CallExpr, _ standInDemand, site standInSite) error {
	path, ok := aggregateOperand(call)
	if !ok {
		return nil
	}
	return g.demand(path.Segments, demandCollection, site)
}

// aggregateOperand returns the single bare path an aggregate takes.
// Anything else — a nested call, a literal — is already a load or
// evaluation error the engine reports itself; this projection declares
// nothing for it rather than guessing.
func aggregateOperand(call *expr.CallExpr) (*expr.PathExpr, bool) {
	if len(call.Args) != 1 {
		return nil, false
	}
	path, ok := expr.Ungroup(call.Args[0]).(*expr.PathExpr)
	return path, ok
}

// standInElementRule collects the data paths ONE ELEMENT KIND can reach,
// beyond the visibleIf every kind carries.
type standInElementRule func(g *standInGenerator, tpl *Template, element template.Element) error

// standInElementRules is the SECOND exhaustiveness axis, and it exists
// for the reason the function-registry axis does. A `switch` with a bare
// `return nil` fallthrough drops an unrecognised element kind's bound
// paths SILENTLY, and a dropped path is not a missing stand-in — it is a
// BINDING_PATH_ABSENT hard error at render, in the one mode whose whole
// purpose is never to fail. The key set is gated against
// internal/template's own closedElementTypes by
// TestStandInRulesCoverEveryElementKind, which AST-extracts that set
// rather than restating it.
var standInElementRules map[template.ElementType]standInElementRule

func init() {
	standInElementRules = map[template.ElementType]standInElementRule{
		// Element.Value is text-only, "{{ }}" interpolated, document
		// scope, and is resolved EVEN WHEN THE ELEMENT IS HIDDEN — the
		// bind runs before the visibility skip — so hiding an element
		// does not spare its paths.
		template.ElementText: func(g *standInGenerator, _ *Template, element template.Element) error {
			return g.collectTextValue(element)
		},
		template.ElementTable: (*standInGenerator).collectTable,
		// A barcode's value binds exactly as a text value does, through
		// the same resolver, so its paths are collected the same way.
		template.ElementBarcode: func(g *standInGenerator, _ *Template, element template.Element) error {
			return g.collectTextValue(element)
		},
		template.ElementQRCode: func(g *standInGenerator, _ *Template, element template.Element) error {
			return g.collectTextValue(element)
		},
		// Element.Asset is a literal `assets` map key and is never
		// bound; line and rect carry no kind-specific field at all.
		// Style.*, altRowBackground and headerStyle.* are negative
		// space — a placeholder there is a load error, not a binding.
		template.ElementImage: standInNoBoundFields,
		template.ElementLine:  standInNoBoundFields,
		template.ElementRect:  standInNoBoundFields,
	}
}

// standInNoBoundFields is the rule for a kind whose own fields can carry
// no data path. It is a NAMED rule rather than a missing map entry, so
// "this kind reaches no data" and "nobody has decided about this kind"
// stay two different states.
func standInNoBoundFields(*standInGenerator, *Template, template.Element) error { return nil }

// standInSite is where a demand came from, for a refusal that can be
// acted on.
type standInSite struct {
	elementID template.ElementID
	field     string
}

// located names this site the way every other located message in this
// module does. It is deliberately NOT called String: render_arch_test.go's
// TestFolio8MethodNamesAreInjective refuses a second receiver in a folio8
// root file declaring a method name another receiver already declares,
// because the call graph it builds merges methods by name.
func (s standInSite) located() string { return fmt.Sprintf("element %s %s", s.elementID, s.field) }

// standInPath is one referenced path and everything asked of it.
type standInPath struct {
	segments []string
	admits   standInSet
	// contexts and sites are parallel, deduplicated, in first-seen
	// order: contexts[i] was demanded at sites[i]. A refusal names both.
	contexts []string
	sites    []string
}

type standInExpression struct {
	node expr.Expr
	site standInSite
}
type standInGenerator struct {
	divisors     []standInExpression
	inequalities []*expr.BinaryExpr
	checks       []standInExpression
	paths        map[string]*standInPath
	// order preserves first-seen order so a refusal reads in document
	// order rather than in map order.
	order []string
}

// standInData returns the stand-in JSON data document for tpl: a real
// JSON document whose value at each referenced path is chosen by the
// EXPRESSION WRAPPING THAT PATH, so a page can be laid out before any
// sample data exists.
//
// It is READ-ONLY and it changes NOTHING about Render. Render is called
// with genuinely supplied data, so its semantics, its error contract, its
// diagnostic codes and the golden corpus are untouched structurally
// rather than by discipline — folio8-go rendering the same template with
// ABSENT data still fails with the same located BINDING_PATH_ABSENT
// Error it does today.
//
// The document is byte-deterministic: encoding/json sorts object keys,
// every value is drawn from the closed standInValue set, and nothing on
// this path reads a clock, the environment, a locale or a random source.
//
// A path whose contexts share no legal value is a REFUSAL naming the path
// and its contexts — never a mangled document.
func standInData(tpl *Template) ([]byte, error) {
	if tpl == nil || tpl.doc == nil {
		return nil, fmt.Errorf("folio8: stand-in data requires a template")
	}
	g := &standInGenerator{paths: make(map[string]*standInPath)}
	for _, band := range tpl.doc.ElementBands() {
		for _, element := range band.Elements {
			if err := g.collectElement(tpl, element); err != nil {
				return nil, err
			}
		}
	}
	// Document.UnbreakableValues is the one data-path site outside the
	// bands and it is DELIBERATELY not collected: wrap.go's
	// atomicSpansFor does pure string equality against a
	// Substitution.Path that resolution already produced, so an entry
	// matching nothing is silently inert and an entry matching something
	// names a path some band already contributed.
	if len(g.order) > designer.MaxStandInDataPaths {
		return nil, fmt.Errorf("folio8: stand-in data: template references more than %d data paths", designer.MaxStandInDataPaths)
	}
	// Reconcile only direct scalar candidates. Each pass removes candidates,
	// so the finite path/value set bounds this intersection to a fixed point.
	for changed := true; changed; {
		changed = false
		for _, pair := range g.inequalities {
			left, lok := expr.Ungroup(pair.Left).(*expr.PathExpr)
			right, rok := expr.Ungroup(pair.Right).(*expr.PathExpr)
			if !lok || !rok {
				continue
			}
			a, b := g.paths[strings.Join(left.Segments, ".")], g.paths[strings.Join(right.Segments, ".")]
			if a == nil || b == nil {
				continue
			}
			for _, candidate := range []*standInPath{a, b} {
				if candidate.admits&standInSetOf(standInNull) != 0 && candidate.admits != standInSetOf(standInNull) {
					candidate.admits = standInSetOf(standInNull)
					changed = true
				}
			}
			if (a.admits|b.admits)&standInSetOf(standInNull) != 0 {
				continue
			}
			common := a.admits & b.admits
			if common != 0 && (a.admits != common || b.admits != common) {
				a.admits, b.admits = common, common
				changed = true
			}
		}
	}
	document, err := g.document()
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("folio8: stand-in data: %w", err)
	}
	if len(out) > designer.MaxStandInDataBytes {
		return nil, fmt.Errorf("folio8: stand-in data: document exceeds the %d-byte limit", designer.MaxStandInDataBytes)
	}
	// Check each divisor, including those in unselected branches. Preview
	// never claims to solve formulas or to make visibility conditions true.
	if len(g.divisors) > 0 || len(g.checks) > 0 {
		data, err := bind.DecodeData(out)
		if err != nil {
			return nil, err
		}
		scope := bind.NewScope(data, bind.Value{})
		// Evaluate the original containing expression so discarded inequality
		// branches stay lazy. Demand discovery still visited both branches.
		for _, check := range g.checks {
			if expressionReferencesParams(check.node) {
				continue
			}
			if _, _, err := bind.EvaluateValue(check.node.Text(), scope, expr.NewFormatContext(tpl.doc.Locale, tpl.doc.UTCOffset), string(check.site.elementID)); err != nil {
				return nil, fmt.Errorf("folio8: stand-in data: %s: cannot satisfy formula; supply sample data: %w", check.site.located(), err)
			}
		}
		// Divisor preflight remains deliberately conservative across both branches.
		for _, divisor := range g.divisors {
			if expressionReferencesParams(divisor.node) {
				continue
			}
			value, _, err := bind.EvaluateValue(divisor.node.Text(), scope, expr.NewFormatContext(tpl.doc.Locale, tpl.doc.UTCOffset), string(divisor.site.elementID))
			if err != nil {
				return nil, fmt.Errorf("folio8: stand-in data: %s: cannot provide a nonzero numeric divisor; supply sample data: %w", divisor.site.located(), err)
			}
			if value.Kind != expr.KindNumber || value.Num.Coefficient == 0 {
				return nil, fmt.Errorf("folio8: stand-in data: %s: cannot provide a nonzero numeric divisor for %q; supply sample data", divisor.site.located(), divisor.node.Text())
			}
		}
	}
	return out, nil
}

// collectElement covers every site a data path can reach from one
// element. The completeness surface is closed by internal/template's own
// model: Bands has exactly three keys and carries no Extra, and
// ElementType is the closed five.
func (g *standInGenerator) collectElement(tpl *Template, element template.Element) error {
	// visibleIf belongs to ALL FIVE element kinds, is a BARE expression
	// (no "{{ }}"), and is evaluated in DOCUMENT scope — render_visibility.go's
	// computeVisibility never calls .WithRow.
	if element.VisibleIf.Set && !element.VisibleIf.Null {
		parsed, err := expr.Parse(element.VisibleIf.Value)
		if err != nil {
			return fmt.Errorf("folio8: stand-in data: element %s visibleIf: %w", element.ID, err)
		}
		if err := g.collectFormulaDemands(parsed, demandCondition, standInSite{element.ID, "visibleIf"}); err != nil {
			return err
		}
	}
	rule, ok := standInElementRules[element.Type]
	if !ok {
		return fmt.Errorf(
			"folio8: stand-in data: element %s: element kind %q has no stand-in rule; refusing rather than silently dropping the data paths it may carry",
			element.ID, element.Type,
		)
	}
	return rule(g, tpl, element)
}

func (g *standInGenerator) collectTextValue(element template.Element) error {
	if !element.Value.Set || element.Value.Null {
		return nil
	}
	_, placeholders, _, err := expr.ScanPlaceholders(element.Value.Value)
	if err != nil {
		return fmt.Errorf("folio8: stand-in data: element %s value: %w", element.ID, err)
	}
	for _, placeholder := range placeholders {
		if placeholder.Reserved {
			// AD-4's page/pages are late-bound by the template layer and
			// are never resolved from data.
			continue
		}
		parsed, perr := expr.Parse(placeholder.Inner)
		if perr != nil {
			return fmt.Errorf("folio8: stand-in data: element %s value: %w", element.ID, perr)
		}
		if err := g.collectFormulaDemands(parsed, demandText, standInSite{element.ID, "value"}); err != nil {
			return err
		}
	}
	return nil
}

// collectTable covers the table's THREE data-path sources, two of which
// appear nowhere in the placeholder stream.
func (g *standInGenerator) collectTable(tpl *Template, element template.Element) error {
	if !element.Table.Set {
		return nil
	}
	table := element.Table.Value
	collection := tableCollectionSegments(table.Bind)
	if len(collection) == 0 {
		return nil
	}
	// Source 1: TableExt.Bind. A document-root collection path requiring
	// an array; render.go's checkTableBindings rules an EMPTY array legal.
	if err := g.demand(collection, demandCollection, standInSite{element.ID, "bind"}); err != nil {
		return err
	}
	for _, column := range table.Columns {
		// Column.Bind IS DELIBERATELY NOT COLLECTED, and the reason is
		// the [] stand-in itself. table_render.go guards the row block
		// with `if len(items) > 0 || hasFooter` and its row loop
		// iterates once per item; over an empty collection it iterates
		// ZERO times, so no row-scope binding — and therefore no
		// Column.Bind, and no table's own row alias — is ever
		// evaluated. That is why every collection stand-in is [], and
		// why row-scope paths need no stand-in at all.
		if !column.Footer.Set {
			continue
		}
		switch column.Footer.Value {
		case "count":
			// Source 2: table_render.go's footerCellExprText SYNTHESIZES
			// count(TrimSuffix(bind, "[]")) at render time. It appears
			// nowhere in the placeholder stream, and it is evaluated at
			// DOCUMENT scope. Declared here as a first-class source so
			// this walk does not silently depend on Source 1 above.
			if err := g.demand(collection, demandCollection, standInSite{column.ID, "footer count"}); err != nil {
				return err
			}
		case "sum", "avg":
			// Source 3: the same synthesis produces sum|avg(footerOf),
			// where footerOf is the explicit Column.FooterOf or the one
			// derived from Column.Bind by expr.DeriveFooterOf. Both are
			// REQUIRED BY THE LOADER to be prefixed by this table's own
			// collection path (parse_bands.go AC43 check 3, and
			// DeriveFooterOf builds collection + "." + rest), so the
			// array sits at that collection and the remainder is a
			// projected field inside it — projected over zero elements,
			// hence no stand-in of its own. avg over the empty
			// collection resolves to the engine's real CodeEmptyAverage
			// Warning, which preview shows rather than suppresses.
			if _, ok := effectiveFooterOf(tpl, column); !ok {
				continue
			}
			if err := g.demand(collection, demandCollection, standInSite{column.ID, "footer " + column.Footer.Value}); err != nil {
				return err
			}
		}
	}
	return nil
}

// walk records what one expression demands of the paths inside it.
func (g *standInGenerator) walk(node expr.Expr, demand standInDemand, site standInSite) error {
	switch value := node.(type) {
	case *expr.PathExpr:
		return g.demand(value.Segments, demand, site)
	case *expr.CallExpr:
		rule, ok := standInFunctionRules[value.Name]
		if !ok {
			// Unreachable for a template that loaded: expr.Check
			// refuses any name outside the closed table. Declaring
			// nothing is the only safe answer to a name whose value-kind
			// rule this generator does not have.
			return nil
		}
		return rule(g, value, demand, site)
	case *expr.GroupExpr:
		return g.walk(value.Inner, demand, site)
	case *expr.UnaryExpr:
		return g.walk(value.Operand, demandNumberOperand(value.Op), site)
	case *expr.ConditionalExpr:
		if err := g.walk(value.Condition, demandCondition, site); err != nil {
			return err
		}
		if err := g.walk(value.Then, demand, site); err != nil {
			return err
		}
		return g.walk(value.Else, demand, site)
	case *expr.BinaryExpr:
		leftDemand, rightDemand := demandNumberOperand(value.Op), demandNumberOperand(value.Op)
		if value.Op == "!=" {
			g.inequalities = append(g.inequalities, value)
			leftDemand = inequalityDemand(value.Right)
			rightDemand = inequalityDemand(value.Left)
		}
		if value.Op == "/" || value.Op == "%" {
			g.divisors = append(g.divisors, standInExpression{value.Right, site})
			if _, ok := expr.Ungroup(value.Right).(*expr.PathExpr); ok {
				rightDemand = standInDemand{standInSetOf(standInOne), "a nonzero divisor"}
			}
		}
		if err := g.walk(value.Left, leftDemand, site); err != nil {
			return err
		}
		return g.walk(value.Right, rightDemand, site)
	case *expr.StringLit, *expr.NumberLit, *expr.BoolLit, *expr.NullLit:
		// A literal references no path.
		return nil
	}
	// expr.Expr's exprNode() is unexported, so only internal/expr can add
	// a node kind — and if one ever is, dropping it here loses whatever
	// paths it holds. Refuse, located, rather than emit a document that
	// is quietly missing them.
	return fmt.Errorf(
		"folio8: stand-in data: %s: expression node %T is unknown to the stand-in generator; refusing rather than silently dropping the data paths inside it",
		site.located(), node,
	)
}

func (g *standInGenerator) walkArg(call *expr.CallExpr, index int, demand standInDemand, site standInSite) error {
	if index >= len(call.Args) {
		// Arity is enforced by expr.Check at load; a short call cannot
		// reach a render.
		return nil
	}
	return g.walk(call.Args[index], demand, site)
}

// demand records one context's requirement on one path, or reports that
// the path's contexts share no legal value.
func (g *standInGenerator) demand(segments []string, demand standInDemand, site standInSite) error {
	if len(segments) == 0 {
		return nil
	}
	// params.* is out of scope BY RULING, not by omission: selectRoot
	// always sends a leading "params" segment to the params root, which
	// this projection does not supply, and an absent param stays a
	// located BINDING_PATH_ABSENT.
	if segments[0] == "params" {
		return nil
	}
	key := strings.Join(segments, ".")
	// AD-4's reserved whole tokens never acquire a data namespace.
	if expr.IsReserved(key) {
		return nil
	}
	existing, seen := g.paths[key]
	if !seen {
		g.paths[key] = &standInPath{segments: slices.Clone(segments), admits: demand.admits, contexts: []string{demand.label}, sites: []string{site.located()}}
		g.order = append(g.order, key)
		return nil
	}
	if !slices.Contains(existing.contexts, demand.label) {
		existing.contexts = append(existing.contexts, demand.label)
		existing.sites = append(existing.sites, site.located())
	}
	existing.admits &= demand.admits
	if existing.admits == 0 {
		return fmt.Errorf(
			"folio8: stand-in data: path %q is used as %s; those contexts share no legal value, so no stand-in can be supplied for this template; supply sample data",
			key, describeContexts(existing),
		)
	}
	return nil
}

// describeContexts names every context a path was demanded in, with the
// site each came from, so a refusal can be acted on rather than merely
// received.
func describeContexts(path *standInPath) string {
	parts := make([]string, 0, len(path.contexts))
	for index, context := range path.contexts {
		parts = append(parts, fmt.Sprintf("%s (%s)", context, path.sites[index]))
	}
	return strings.Join(parts, " and as ")
}

// document materialises the collected paths into one JSON object tree.
// Paths are visited in sorted order, which is what makes a leaf/branch
// collision reportable: "a" sorts before "a.b", so the leaf is always
// placed first and the branch that needs it to be an object always finds
// it there.
func (g *standInGenerator) document() (map[string]any, error) {
	root := map[string]any{}
	for _, key := range slices.Sorted(maps.Keys(g.paths)) {
		path := g.paths[key]
		value, ok := path.admits.first()
		if !ok {
			// Unreachable: demand refuses an empty intersection at the
			// moment it becomes empty. Kept as a located error rather
			// than a panic (AD-14).
			return nil, fmt.Errorf("folio8: stand-in data: path %q has no admitted stand-in value", key)
		}
		node := root
		for index, segment := range path.segments[:len(path.segments)-1] {
			prefix := strings.Join(path.segments[:index+1], ".")
			child, exists := node[segment]
			if !exists {
				next := map[string]any{}
				node[segment] = next
				node = next
				continue
			}
			next, isObject := child.(map[string]any)
			if !isObject {
				return nil, fmt.Errorf(
					"folio8: stand-in data: path %q needs %q to be an object, but %q is itself a bound value; no stand-in document satisfies both",
					key, prefix, prefix,
				)
			}
			node = next
		}
		leaf := path.segments[len(path.segments)-1]
		if _, exists := node[leaf]; exists {
			// Unreachable while paths are visited in sorted order: a
			// shorter path always sorts before every path that extends
			// it, so a leaf/branch collision is always caught by the
			// branch arm above. Kept as a located error so a future
			// change of traversal order cannot silently overwrite a
			// value instead.
			return nil, fmt.Errorf("folio8: stand-in data: path %q is already occupied; no stand-in document satisfies both", key)
		}
		node[leaf] = value.jsonValue()
	}
	return root, nil
}

func inequalityDemand(other expr.Expr) standInDemand {
	all := standInSetOf(standInEmptyString, standInInstant, standInZero, standInOne, standInTrue, standInNull)
	kinds := expr.KnownKinds(other)
	if len(kinds) == 0 {
		return standInDemand{all, "an inequality scalar"}
	}
	admitted := all
	for _, kind := range kinds {
		compatible := standInSetOf(standInNull)
		switch kind {
		case expr.KindNull:
			continue
		case expr.KindString:
			compatible |= standInSetOf(standInEmptyString, standInInstant)
		case expr.KindNumber:
			compatible |= standInSetOf(standInZero, standInOne)
		case expr.KindBool:
			compatible |= standInSetOf(standInTrue)
		}
		admitted &= compatible
	}
	return standInDemand{admitted, "an inequality scalar"}
}

func expressionReferencesParams(e expr.Expr) bool {
	if path, ok := e.(*expr.PathExpr); ok && len(path.Segments) > 0 && path.Segments[0] == "params" {
		return true
	}
	for _, child := range expr.Children(e) {
		if expressionReferencesParams(child) {
			return true
		}
	}
	return false
}

func (g *standInGenerator) collectFormulaDemands(node expr.Expr, demand standInDemand, site standInSite) error {
	if expressionHasInequality(node) {
		g.checks = append(g.checks, standInExpression{node, site})
	}
	return g.walk(node, demand, site)
}
func expressionHasInequality(node expr.Expr) bool {
	if binary, ok := node.(*expr.BinaryExpr); ok && binary.Op == "!=" {
		return true
	}
	for _, child := range expr.Children(node) {
		if expressionHasInequality(child) {
			return true
		}
	}
	return false
}
