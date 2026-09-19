// This file implements text-element "{{ }}" binding (D-1.6.5, replaced
// at Story 3.2 by the expression language, D-1.6.5/D-3.2.1: "this
// story's parser REPLACES 1.6's matcher — deleted, not kept
// alongside"). Scope (AC20, unchanged since 1.6): callers apply
// BindText only to text-element `value` interpolation.
package bind

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/panitw/folio8/folio-go/internal/expr"
)

// Substitution records one placeholder's contribution to BindTextSpans'
// output: which data path produced it, and which part of the result it
// occupies.
//
// Start and End are RUNE indices into the returned string, half-open.
// Rune indices, because every position the breaking stage works in is a
// rune index — AD-25's break positions and GlyphInfo.Cluster alike
// (D-2.3.2) — and a byte offset that happened to be equal for ASCII
// would diverge silently on the first Thai character.
//
// A placeholder that resolved to JSON null contributes an EMPTY span
// (Start == End) rather than no span at all. It is reported because the
// caller asked what each path produced, and "nothing, at this position"
// is the honest answer; it can never affect breaking, since an empty
// span has no interior.
//
// THIS TYPE CARRIES NO OPINION ABOUT LINE BREAKING, and internal/bind
// has none. It reports what it substituted and where. Whether a span is
// unbreakable is the DOCUMENT's declaration, matched against Path by
// the caller, and handed to internal/text as a parameter (D-000.16:
// "the signal rides on the value, never through an import").
type Substitution struct {
	// Path is the bare dotted path exactly as authored, including a
	// leading "params" segment when it resolved against the params
	// root — or, for an expression that is not a bare path (a function
	// call, Story 3.2), the expression's own raw source text.
	Path string

	// Start, End are rune indices into BindTextSpans' returned string.
	Start, End int
}

// AD-4's page-number slots (D-1.6.5, AC18) are owned by Story 2.7,
// never resolved from data and never an error at 1.6. A data document
// containing a top-level "page" key must not change what {{page}}
// renders (AC19) — this reservation is checked BEFORE any data lookup,
// or any parse attempt (AC4, Story 3.2), so it can never be shadowed.
//
// Resolve (below) gets this for free from expr.ScanPlaceholders'
// Placeholder.Reserved field: reservation is decided exactly once, by
// internal/expr.IsReserved (AD-13: one definition, not two)
// — this file no longer needs, and no longer keeps, a local alias.
// (QA Finding 5, Major: this file previously hand-rolled a byte-
// identical second "{{"/"}}" search rather than calling
// ScanPlaceholders, which its own doc comment claimed nothing did.)

// rootKind is a resolution root's identity: its literal name (the
// class, e.g. "row" — never the author's own alias spelling) and the
// human-readable description used in error text. It exists as ONE
// defined type, not two string parameters, because of what that buys
// (Story 3.3 finisher pass, correcting D-3.3.1's re-point — see the
// finisher's decision-log correction, D-3.3.7):
//
// FINDING 1 (Blocker), measured: with rootName/rootDesc as bare
// strings, a dispatch bypassing selectRoot entirely and calling
// lookupBound with a literal "page" argument COMPILED and PASSED the
// re-pointed TestBindResolutionRootsAreClosed — the guard had been
// re-pointed at selectRoot's own return statements, but nothing stopped
// a caller from reaching lookupBound WITHOUT going through selectRoot
// at all. That is the exact defeat shape D-3.1.1/Story 2.5's review
// Blocker 2 recorded against the ORIGINAL proxy (lookupBound call
// sites) — the re-point moved the blind spot, it did not close it.
//
// rootKind closes it BY CONSTRUCTION: lookupBound's rootKind parameter
// is a named struct type, not string, so `lookupBound(..., "page", ...)`
// — an untyped string constant — is not assignable to it and does not
// compile. The COMPILER is the guard for "can a root be introduced
// anywhere other than a declaration" — this is the direction the
// re-point lost, and it cannot be relocated again, because there is no
// string-typed parameter left anywhere on the call path for a literal
// to be smuggled through.
//
// The three rootKind VALUES below are the only instances this package
// ever constructs. Whether that set stays exactly {kindData, kindParams,
// kindRow} is the SEPARATE property resolution_roots_arch_test.go's
// TestBindResolutionRootsAreClosed checks — by AST set-equality over
// every rootKind composite literal in the module, which sees a fourth
// root wherever declared, whether or not selectRoot ever returns it and
// whether or not it ever reaches lookupBound. Two different questions,
// one type answering the first, one scan answering the second — never
// blurred (D-3.3.7).
//
// Bundling name and desc into one struct (rather than two parallel
// string parameters) is a free consequence, not an extra feature: it
// makes a mismatched name/desc pair unconstructible, and it is what
// lets the two bare `rootName != "data"` comparisons below become
// `kind != kindData` — two fewer string literals on the root path
// (D-000.47).
type rootKind struct {
	name, desc string
}

// kindData, kindParams and kindRow are Story 2.7/3.1/3.3's DECLARED set
// of resolution roots (D-2.5.1's "declared vs observed, both
// directions" shape) — the only rootKind values this package
// constructs, each a package-level composite literal so
// resolution_roots_arch_test.go's AST scan has exactly one place per
// root to find. A "page" NAMESPACE is precisely a FOURTH entry here
// (BindText's own doc comment below carries the fence, verbatim:
// "conflating the two is how 'page' would eventually acquire a
// namespace, which AD-4 forbids forever"). Adding one is a one-line,
// VISIBLE diff to this list, and the guard catches a fourth rootKind
// literal anywhere in the module whether or not this list is edited to
// match it.
var (
	kindData   = rootKind{name: "data", desc: "the report data"}
	kindParams = rootKind{name: "params", desc: "the supplied params"}
	kindRow    = rootKind{name: "row", desc: "the current row"}
)

// declaredResolutionRootNames is the DECLARED set restated as bare
// strings, for the closure test's own set-equality comparison against
// the names it extracts from every rootKind composite literal it finds.
var declaredResolutionRootNames = []string{kindData.name, kindParams.name, kindRow.name}

// BindText resolves every "{{…}}" placeholder in text against data and
// params (Story 3.2's expression language, D-1.6.5, D-1.7.4), scoped to
// one text element (elementID names it in every error). "{{page}}"
// and "{{pages}}" pass through unchanged (AC4).
//
// AD-14's three cases (AC12/AC13, Story 3.2's if() ruling): an absent
// path is an error naming both the data path and elementID; an
// explicit JSON null renders as empty and is not an error; a number
// prints as its exact decimal (expr.Decimal.Text — owner decision
// 2026-09-13, revising AD-14 for numbers in text only); a boolean, array
// or object is an error, never coerced.
//
// "params" is a SECOND resolution root, not a reserved token (Story
// 1.7, D-1.7.4, verbatim): "params is a namespace — a second
// resolution root resolved at bind time, alongside the data root." A
// placeholder whose FIRST path segment is "params" ALWAYS resolves
// against params, never against data — regardless of whether data
// itself happens to carry a top-level "params" key: that key is
// ordinary, unreachable caller JSON. This is deliberately NOT
// implemented by extending expr.IsReserved (page/pages are
// reserved whole TOKENS, resolved from neither root and owned by Story
// 2.7; params is a NAMESPACE, resolved from its own root — conflating
// the two is how "page" would eventually acquire a namespace, which
// AD-4 forbids forever).
func BindText(text string, data, params Value, fc expr.FormatContext, elementID string) (string, error) {
	s, _, _, err := BindTextSpans(text, data, params, fc, elementID)
	return s, err
}

// BindTextSpans is BindText plus the report of WHAT it substituted
// WHERE: one Substitution per resolved placeholder, in output order,
// with rune spans into the returned string — and, alongside that,
// every expr.Caveat (Story 3.3, DECISION-5) an aggregate evaluated
// against this text produced: a non-error condition the render
// survives (avg()-on-empty, R9), reported as a SEPARATE return rather
// than folded into Substitution, which is a record about text SPANS
// (every one of its four consumers — shiftSubstitutions,
// runeRangeOverlapsSubstitution, resolvePageTokens, atomicSpansFor,
// render.go/page_number.go/wrap.go — reads spans and nothing else); a
// payload none of them reads but all must preserve is a payload that
// gets silently dropped by the first one that reconstructs the struct
// field by field, which shiftSubstitutions already does.
//
// It is the same traversal, not a second one. BindText delegates here,
// so there is exactly one implementation of the binding grammar and the
// spans cannot drift from the text they describe — the same "only one
// derivation" discipline Story 2.3's Blocker 1 imposed on advances.
//
// {{page}} and {{pages}} produce NO Substitution and NO Caveat. They
// are reserved tokens owned by Story 2.7, resolved from neither root,
// and they name no data path — so there is nothing a document could
// declare about them.
//
// This is a thin wrapper over Resolve, constructing a Scope with NO row
// set — the byte-identical pre-3.1 behaviour when no row is active. A
// future row-scoped caller (Story 4.2) calls Resolve directly with a
// Scope built through NewScope(...).WithRow(...).
func BindTextSpans(text string, data, params Value, fc expr.FormatContext, elementID string) (string, []Substitution, []expr.Caveat, error) {
	return Resolve(text, NewScope(data, params), fc, elementID)
}

// Resolve is the one implementation of the binding grammar's
// resolution-root traversal, taking a Scope rather than bare
// data/params values, so BindText/BindTextSpans and any future
// row-scoped caller share one traversal, never two.
//
// Story 3.2 (D-1.6.5, D-3.2.1): every non-reserved "{{ }}" occurrence
// is parsed and checked by internal/expr (expr.Parse, expr.Check —
// syntax, arity, unknown-function-name, literal-argument-kind), then
// evaluated (expr.Eval) against a Resolver built from scope
// (exprResolver, below) that dispatches each path to the data, params
// or row root exactly as 1.6/3.1 did — the SAME lookupBound helper as
// before, now returning a generic expr.Value rather than a
// string-only result, so it can serve a path resolved from ANYWHERE
// in an expression (a bare top-level path, or nested inside a function
// argument), not only the top-level substitution case.
func Resolve(text string, scope Scope, fc expr.FormatContext, elementID string) (string, []Substitution, []expr.Caveat, error) {
	var out strings.Builder
	// runesWritten tracks the rune length of out, maintained alongside
	// every write rather than recomputed, so a Substitution's bounds are
	// recorded at the moment the text is appended.
	runesWritten := 0
	var subs []Substitution
	var caveats []expr.Caveat
	write := func(s string) {
		out.WriteString(s)
		runesWritten += utf8.RuneCountInString(s)
	}

	// The "{{ }}" search itself is expr.ScanPlaceholders — AD-13's ONE
	// tokenizer for this syntax (QA Finding 5, Major: this loop used to
	// hand-roll its own byte-identical copy of ScanPlaceholders' scan,
	// which scan.go's own doc comment claimed did not exist). literal[i]
	// is the plain text immediately preceding placeholders[i]; trailing
	// is whatever follows the last placeholder (the whole string, if
	// there were none) — exactly the segmentation this loop needs to
	// reassemble its output.
	literal, placeholders, trailing, serr := expr.ScanPlaceholders(text)
	if serr != nil {
		return "", nil, nil, fmt.Errorf("bind: element %s: %s", elementID, serr)
	}
	for i, ph := range placeholders {
		write(literal[i])

		trimmed := strings.TrimSpace(ph.Inner)
		if ph.Reserved {
			// AC4: reserved for Story 2.7 — left byte-for-byte
			// unchanged, never resolved from data, never an error. It
			// names no data path, so it yields no Substitution and no
			// Caveat.
			write("{{")
			write(ph.Inner)
			write("}}")
			continue
		}

		astExpr, perr := expr.Parse(ph.Inner)
		if perr != nil {
			return "", nil, nil, fmt.Errorf(
				"bind: element %s: invalid value expression placeholder %d: %w",
				elementID, i+1, perr,
			)
		}
		if cerr := expr.CheckText(astExpr); cerr != nil {
			return "", nil, nil, fmt.Errorf("bind: element %s: placeholder %d: %w", elementID, i+1, cerr)
		}

		budget := expr.NewBudget()
		resolver := exprResolver{scope: scope, elementID: elementID, budget: budget}
		val, valCaveats, everr := expr.EvalWithBudget(astExpr, resolver, fc, elementID, budget)
		if everr != nil {
			return "", nil, nil, fmt.Errorf("bind: element %s: placeholder %d: %w", elementID, i+1, everr)
		}
		caveats = append(caveats, valCaveats...)

		from := runesWritten
		switch val.Kind {
		case expr.KindNull:
			// AD-14: an explicit JSON null renders as empty, never an
			// error. avg()-on-empty (R9) resolves here too — its own
			// Caveat, collected above, is what tells the caller this
			// particular empty is worth a Warning.
		case expr.KindString:
			write(val.Str)
		case expr.KindNumber:
			// Owner decision 2026-09-13: a number in text prints as its
			// exact decimal — data values and computed numbers alike, no
			// grouping, locale, rounding or float. formatNumber stays the
			// way to get styled output. A positive exponent expands into
			// that many printed zeros, so it is charged to this
			// placeholder's budget before anything is written.
			if val.Num.Exponent > 0 {
				if cerr := budget.Charge(val.Num.Exponent); cerr != nil {
					return "", nil, nil, fmt.Errorf("bind: element %s: placeholder %d: %w", elementID, i+1, expr.WithLocation(astExpr, cerr))
				}
			}
			write(val.Num.Text())
		default:
			return "", nil, nil, fmt.Errorf("bind: element %s: placeholder %d: %w", elementID, i+1,
				expr.WithLocation(astExpr, fmt.Errorf("resolved to a %s, not a string or number — a %s is never coerced to text", val.Kind, val.Kind)))
		}
		subs = append(subs, Substitution{Path: substitutionPathFor(astExpr, trimmed), Start: from, End: runesWritten})
	}
	write(trailing)
	return out.String(), subs, caveats, nil
}

// substitutionPathFor reports the dotted path a resolved Substitution
// names: the joined segments for a bare path (the common case, and the
// only case before Story 3.2), or the expression's own raw source text
// for anything else (a function call) — there is no single "path" a
// general expression names, and the raw text is the most honest
// available answer.
func substitutionPathFor(e expr.Expr, trimmed string) string {
	if p, ok := e.(*expr.PathExpr); ok {
		return strings.Join(p.Segments, ".")
	}
	return trimmed
}

// exprResolver adapts a Scope to expr.Resolver (ast.go): it is the ONE
// place root dispatch (data/params/row) happens for the WHOLE
// expression tree being evaluated — including a path nested inside a
// function argument, e.g. upper(params.name) — not only a bare
// top-level path, which is why dispatch cannot be decided once before
// calling into expr.Eval and must instead live inside Resolve itself.
type exprResolver struct {
	budget    *expr.Budget
	scope     Scope
	elementID string
}

func (r exprResolver) Resolve(path []string) (expr.Value, error) {
	if err := r.budget.Charge(len(path)); err != nil {
		return expr.Value{}, err
	}
	if len(path) == 0 {
		return expr.Value{}, fmt.Errorf("bind: element %s: empty path", r.elementID)
	}

	// allowRow=true: the scalar dispatch is the ONE caller that ever
	// lets a row scope's own declared alias select the row root
	// (Story 3.1/D-3.1.1) — the two collection methods below never do
	// (R3, AC20).
	kind, root := selectRoot(r.scope, path[0], true)
	if kind != kindData {
		if len(path) == 1 {
			// AC17 (1.7)/D-3.1.1: a bare "{{params}}" or the row
			// alias alone, no dot — the root names a namespace, not a
			// value.
			return expr.Value{}, fmt.Errorf("bind: element %s: %q is a namespace, not a value", r.elementID, path[0])
		}
		return lookupBound(root, path[1:], path, r.elementID, kind, r.budget)
	}
	return lookupBound(root, path, path, r.elementID, kind, r.budget)
}

// selectRoot is D-3.1.1's ONE place a resolution root is chosen (Story
// 3.3, R2): the shared helper both the scalar dispatch above and the
// two collection methods below call, implementing the SAME precedence
// — params, then row alias, then data — unchanged (D-3.1.1).
//
// selectRoot returns one of exactly three rootKind VALUES (kindData,
// kindParams, kindRow, above) — never a computed rootKind, never a
// string. That every rootKind this function can return is one of the
// three package-level composite literals is what
// resolution_roots_arch_test.go's TestBindResolutionRootsAreClosed
// checks (by AST set-equality over every rootKind{...} composite
// literal in the module — see the type's own doc comment for why the
// SEPARATE property, "can a root only be introduced by declaration", no
// longer depends on scanning this function at all: it is now the
// compiler's job, because lookupBound's rootKind parameter is a named
// struct type a string literal cannot satisfy).
//
// allowRow gates whether the row root is eligible to be selected at
// all: the scalar dispatch (exprResolver.Resolve) always passes true.
// The two collection methods (CollectionLength, ProjectCollection)
// always pass false, because AD-11 requires an aggregate's collection
// path to resolve ROOT-RELATIVE — bypassing the row root even when the
// row scope's own declared alias equals the collection's first segment
// (R3, AC20: a row scope aliased "transactions" must not narrow
// sum(transactions.amount) to that one row's own nested field). params
// stays eligible for a collection path either way — R3 bypasses only
// the row root, never params.
func selectRoot(scope Scope, first string, allowRow bool) (kind rootKind, root Value) {
	// AC12/AC13/D-1.7.4: the FIRST segment "params" always selects the
	// params root, decided at the path-segment level (not on a
	// trimmed literal string) so whitespace-tolerant spellings are
	// caught identically.
	if first == "params" {
		return kindParams, scope.params
	}
	// Story 3.1/D-3.1.1: when a row scope is active and the FIRST
	// segment equals the region's declared alias, the row root is
	// selected — evaluated AFTER params (params "can be shadowed by
	// nothing") and BEFORE the data root (a row never shadows the
	// document root). kindRow's name field is the LITERAL "row" — the
	// root CLASS, never scope.rowAlias, which is document data and
	// could never be assigned into a rootKind's name field here anyway
	// (there is no code path that constructs one from it).
	if allowRow && scope.rowSet && first == scope.rowAlias {
		return kindRow, scope.row
	}
	return kindData, scope.data
}

// lookupBound resolves subPath against root — data, params or row —
// and converts a Present value to a general expr.Value (Story 3.2: no
// longer string-only, so a path resolved from inside a function
// argument — e.g. if(row.active, …) — can carry a bool, not only
// text). fullPath is the ORIGINAL placeholder path (the full dotted
// path, including a leading "params" segment when root is the params
// tree) used only for error text. kind is selectRoot's own return
// value, threaded through unchanged — never re-decided here.
//
// kind is a rootKind, not a pair of strings (Story 3.3 finisher pass,
// Finding 1): a caller cannot pass a literal root name here at all —
// `lookupBound(..., "page", ...)` does not compile, because an untyped
// string constant is not assignable to rootKind. Every caller MUST
// obtain kind from selectRoot (the only function that constructs a
// rootKind other than the three package-level declarations), which is
// what makes "a root can only be introduced by declaration" a property
// the compiler enforces rather than one an AST scan tries to observe.
func lookupBound(root Value, subPath, fullPath []string, elementID string, kind rootKind, budgets ...*expr.Budget) (expr.Value, error) {
	var budget *expr.Budget
	if len(budgets) > 0 {
		budget = budgets[0]
	}
	val, presence := root.Lookup(subPath)
	switch presence {
	case Absent:
		return expr.Value{}, &PathAbsentError{Path: strings.Join(fullPath, "."), Err: fmt.Errorf("bind: element %s: %s path %q is absent from %s", elementID, kind.name, strings.Join(fullPath, "."), kind.desc)}
	case Null:
		return expr.Value{Kind: expr.KindNull}, nil
	case Present:
		switch val.Kind {
		case KindString:
			return expr.Value{Kind: expr.KindString, Str: val.Str}, nil
		case KindBool:
			return expr.Value{Kind: expr.KindBool, Bool: val.Bool}, nil
		case KindNumber:
			if err := budget.Charge(len(val.Num)); err != nil {
				return expr.Value{}, err
			}
			d, derr := val.AsDecimal()
			if derr != nil {
				return expr.Value{}, fmt.Errorf(
					"bind: element %s: %s path %q: %w",
					elementID, kind.name, strings.Join(fullPath, "."), derr,
				)
			}
			return expr.Value{Kind: expr.KindNumber, Num: d}, nil
		default:
			return expr.Value{}, fmt.Errorf(
				"bind: element %s: %s path %q is a %s, not a scalar value usable in an expression",
				elementID, kind.name, strings.Join(fullPath, "."), val.Kind,
			)
		}
	}
	// Unreachable given Presence's own closed three-value enum
	// (Absent/Null/Present, all three handled above). QA Finding (Nit):
	// this used to fall through to "return expr.Value{}, nil" — a
	// silent KindNull with no error at all. evalIf reads a null
	// condition as SILENTLY false (owner ruling), so if a fourth
	// Presence state were ever added, this used to be the quietest
	// possible failure mode: a whole section could disappear from a
	// rendered document with no diagnostic anywhere. Kept as a located
	// error instead, naming the unhandled value, so a future enum
	// widening fails loudly here rather than silently downstream.
	return expr.Value{}, fmt.Errorf(
		"bind: element %s: internal: %s path %q resolved to an unhandled presence value %v",
		elementID, kind.name, strings.Join(fullPath, "."), presence,
	)
}

// --- Story 3.3: the collection seam (R1) ---
//
// CollectionLength and ProjectCollection are expr.Resolver's two
// collection-facing methods (ast.go), implemented here because
// internal/expr may not import internal/bind (D-1.6.1's rank) — expr
// DECLARES the seam, bind IMPLEMENTS it, exactly as it already does for
// Resolve.
//
// Both discriminate the FOUR states R8 requires before any kernel is
// reached (AC17/AC18): present-and-empty, absent, explicit null, and
// present-but-not-an-array. CORRECTED (Story 3.3 finisher pass, Finding
// 9): this used to claim "only the first is a legitimate zero", which
// the very next paragraph already contradicted (a null collection path
// is ALSO a zero observation) and which the story's own plain-terms
// opener repeated and the reviewer found falsified against the shipped
// code. TWO of the four are zeros (present-and-empty; explicit null);
// TWO are located Errors that stop the render (absent;
// present-but-not-an-array). Collapsing either ERROR arm into a zero is
// F6's silent-zero hazard one layer up from where Story 3.2 killed it
// for a bare path.
//
// An explicit JSON null AT THE COLLECTION'S OWN PATH (as opposed to a
// null ELEMENT inside a real array, R7/AC19) is treated as ONE zero
// observation — R7's ruling applied one level up: a collection that is
// present but blank is not a plain zero-length collection (that is
// what a *literally* empty array means, and is a different subject,
// AC12's "zero-length" row versus its "all-null" row and its "null
// collection path" row); it is one null value where a collection was
// expected, and R7 already answers what a null value contributes: a
// zero observation, counted once in avg's divisor. CollectionLength
// therefore reports 1 for a null collection path, and ProjectCollection
// reports one KindNull element — never 0 and never an error — so
// count/sum/avg over a null "transactions" field behave exactly as they
// would over a literal [null] collection, which is what the JSON
// actually cannot distinguish it from.

// splitCollectionPath walks root down subPath's segments, exactly as
// Value.Lookup does (nested objects only), until either subPath is
// exhausted or the current value is Kind == KindArray — Story 3.3's
// own "the path's array prefix names the collection" (AC6). consumed
// reports how many of subPath's segments were used to reach that
// point, so a caller can split subPath into the collection-naming
// prefix (subPath[:consumed]) and, for ProjectCollection, the
// projected field path that remains (subPath[consumed:]).
//
// presence mirrors Value.Lookup's own three-value Presence: Absent for
// a missing key (or a path that runs through a non-object, non-array
// value before it is exhausted — the same "the report does not contain
// this" reading Lookup already gives that shape, AC8 there), Null for
// an explicit JSON null found before an array ever appears, and
// Present otherwise — whether the final value is actually KindArray is
// the caller's own check (AC17's "present, not an array" arm).
func splitCollectionPath(root Value, subPath []string) (val Value, consumed int, presence Presence) {
	cur := root
	for _, seg := range subPath {
		if cur.Kind == KindArray {
			return cur, consumed, Present
		}
		if cur.Kind != KindObject {
			if cur.Kind == KindNull {
				return cur, consumed, Null
			}
			// A non-object, non-array, non-null value with a segment
			// still to resolve through it: nothing sensible to look up
			// inside a string/number/bool, exactly Lookup's own "runs
			// through a non-object" case (value.go) — Absent.
			return Value{}, consumed, Absent
		}
		child, ok := cur.Obj[seg]
		if !ok {
			return Value{}, consumed + 1, Absent
		}
		cur = child
		consumed++
	}
	if cur.Kind == KindArray {
		return cur, consumed, Present
	}
	if cur.Kind == KindNull {
		return cur, consumed, Null
	}
	return cur, consumed, Present
}

// collectionSubPath applies selectRoot with allowRow=false (R3: an
// aggregate operand always resolves root-relative, bypassing the row
// root even when the row's own declared alias would otherwise match,
// AC20) and strips a leading "params" segment exactly as the scalar
// dispatch does, returning the remaining path to walk against root,
// plus kind for error text. ok is false for a bare
// "{{params}}"-shaped collection path (a namespace, not a collection),
// which the caller reports as a located error the same way
// exprResolver.Resolve does for a bare scalar namespace reference.
func collectionSubPath(scope Scope, path []string, elementID string) (subPath []string, kind rootKind, root Value, err error) {
	kind, root = selectRoot(scope, path[0], false)
	if kind != kindData {
		if len(path) == 1 {
			return nil, rootKind{}, Value{}, fmt.Errorf("bind: element %s: %q is a namespace, not a value", elementID, path[0])
		}
		return path[1:], kind, root, nil
	}
	return path, kind, root, nil
}

// CollectionLength is count()'s only operation (AC1, AC3, R5):
// STRUCTURALLY unable to reach a projected value — it never calls
// ProjectCollection, and never inspects an element's own fields, so an
// element missing the field a sum()/avg() over the same path would
// reject still counts (R5's "count is a property of the collection;
// sum and avg are properties of a projection over it").
func (r exprResolver) CollectionLength(path []string) (int, error) {
	if err := r.budget.Charge(len(path)); err != nil {
		return 0, err
	}
	if len(path) == 0 {
		return 0, fmt.Errorf("bind: element %s: empty collection path", r.elementID)
	}
	subPath, kind, root, err := collectionSubPath(r.scope, path, r.elementID)
	if err != nil {
		return 0, err
	}

	val, consumed, presence := splitCollectionPath(root, subPath)
	switch presence {
	case Absent:
		return 0, &PathAbsentError{Path: strings.Join(path, "."), Err: fmt.Errorf("bind: element %s: %s collection path %q is absent from %s", r.elementID, kind.name, strings.Join(path, "."), kind.desc)}
	case Null:
		// See the file-level comment above: a null collection path is
		// one zero observation, not zero elements.
		return 1, nil
	case Present:
		if val.Kind != KindArray {
			return 0, fmt.Errorf("bind: element %s: %s collection path %q is a %s, not a collection", r.elementID, kind.name, strings.Join(path, "."), val.Kind)
		}
		if consumed != len(subPath) {
			return 0, fmt.Errorf(
				"bind: element %s: %s collection path %q does not name a collection: %d segment(s) beyond the array are unused — count() takes a bare collection path, never a projected field",
				r.elementID, kind.name, strings.Join(path, "."), len(subPath)-consumed,
			)
		}
		return len(val.Arr), nil
	}
	return 0, fmt.Errorf("bind: element %s: internal: %s collection path %q resolved to an unhandled presence value %v", r.elementID, kind.name, strings.Join(path, "."), presence)
}

// ProjectCollection is sum()'s and avg()'s only operation (AC1, AC2):
// exactly one Value per element, in DATA ORDER — never sorted,
// deduplicated or filtered — projecting path's trailing segments
// (everything after the array prefix) against each element in turn.
//
// Per-element defects (AC19) are located Errors, FIRST-FAILURE rather
// than collected (D-000.60's guardrail permits either, and states it
// explicitly here — corrected, Story 3.3 finisher pass, Finding 17:
// this comment used to claim the declaration lived "in the delivery
// log", which it did not until this pass; see the story's Delivery Log
// for the same statement and its reason): a single-error return is the
// shape every other located error in this package already uses, and
// folio-go has no multi-error type to spend one on introducing here.
// A projected field absent from an element,
// or present but not a number, stops the projection and names the
// collection path, the zero-based element index, the projected field
// path, and the element id (r.elementID — the SAME "element" AD-14
// already means everywhere else in this codebase: the template's own
// binding site, not a per-row identifier). An explicit null projected
// field is never an error (R7): it is returned as Value{Kind: KindNull},
// faithfully — decimalsFromProjection (internal/expr/aggregate.go) is
// what turns that into the additive identity; this function only
// reports what the data actually held.
func (r exprResolver) ProjectCollection(path []string) ([]expr.Value, error) {
	if err := r.budget.Charge(len(path)); err != nil {
		return nil, err
	}
	if len(path) == 0 {
		return nil, fmt.Errorf("bind: element %s: empty collection path", r.elementID)
	}
	subPath, kind, root, err := collectionSubPath(r.scope, path, r.elementID)
	if err != nil {
		return nil, err
	}

	val, consumed, presence := splitCollectionPath(root, subPath)
	switch presence {
	case Absent:
		return nil, &PathAbsentError{Path: strings.Join(path, "."), Err: fmt.Errorf("bind: element %s: %s collection path %q is absent from %s", r.elementID, kind.name, strings.Join(path, "."), kind.desc)}
	case Null:
		// See the file-level comment above: a null collection path is
		// one zero observation — represented here as a single KindNull
		// element, the same value a real null ELEMENT resolves to
		// below.
		return []expr.Value{{Kind: expr.KindNull}}, nil
	case Present:
		if val.Kind != KindArray {
			return nil, fmt.Errorf("bind: element %s: %s collection path %q is a %s, not a collection", r.elementID, kind.name, strings.Join(path, "."), val.Kind)
		}
		if consumed == len(subPath) {
			// AC6: a BARE collection path passed to sum()/avg() — no
			// projected field remains. Whether a path names a
			// collection is data-dependent, so this is a located
			// Error at EVALUATION, never at load (Check runs without
			// data).
			return nil, fmt.Errorf(
				"bind: element %s: %s collection path %q names a collection, not a projected field — sum()/avg() require a projected field (e.g. %s.<field>)",
				r.elementID, kind.name, strings.Join(path, "."), strings.Join(path, "."),
			)
		}
	default:
		return nil, fmt.Errorf("bind: element %s: internal: %s collection path %q resolved to an unhandled presence value %v", r.elementID, kind.name, strings.Join(path, "."), presence)
	}

	projSegs := subPath[consumed:]
	// Charge before allocation or per-row traversal; divide before multiplying
	// so a malicious collection size cannot wrap the work estimate.
	if r.budget != nil && len(val.Arr) > 1_000_000/max(len(path), 1) {
		return nil, fmt.Errorf("expression projection exceeds 1000000 work units")
	}
	if err := r.budget.Charge(len(val.Arr) * len(path)); err != nil {
		return nil, err
	}
	out := make([]expr.Value, len(val.Arr))
	for i, elem := range val.Arr {
		fieldVal, fp := elem.Lookup(projSegs)
		switch fp {
		case Absent:
			return nil, &PathAbsentError{Path: strings.Join(path, "."), Err: fmt.Errorf(
				"bind: element %s: %s collection %q element %d: projected field %q is absent from the element",
				r.elementID, kind.name, strings.Join(path[:len(path)-len(projSegs)], "."), i, strings.Join(projSegs, "."),
			)}
		case Null:
			out[i] = expr.Value{Kind: expr.KindNull} // R7: a zero observation, resolved downstream.
		case Present:
			if fieldVal.Kind != KindNumber {
				return nil, fmt.Errorf(
					"bind: element %s: %s collection %q element %d: projected field %q is a %s, not a number (never coerced)",
					r.elementID, kind.name, strings.Join(path[:len(path)-len(projSegs)], "."), i, strings.Join(projSegs, "."), fieldVal.Kind,
				)
			}
			if err := r.budget.Charge(len(fieldVal.Num)); err != nil {
				return nil, err
			}
			d, derr := fieldVal.AsDecimal()
			if derr != nil {
				return nil, fmt.Errorf(
					"bind: element %s: %s collection %q element %d: projected field %q: %w",
					r.elementID, kind.name, strings.Join(path[:len(path)-len(projSegs)], "."), i, strings.Join(projSegs, "."), derr,
				)
			}
			out[i] = expr.Value{Kind: expr.KindNumber, Num: d}
		}
	}
	return out, nil
}
