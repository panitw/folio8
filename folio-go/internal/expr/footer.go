package expr

import (
	"strconv"
	"strings"
)

// RowBinding is the small, engine-owned view of a table cell binding that an
// editor may expose as a row-field draft.  Everything else remains display
// only: callers must not reverse-engineer expressions in another language.
type RowBinding struct {
	Field    string
	Editable bool
}

// ProjectRowBinding recognizes exactly the editable bare row-path shape.
// formatNumber and all other valid expressions intentionally remain
// non-editable displays; editing them through a field box would lose meaning.
func ProjectRowBinding(bindText, rowAlias string) (RowBinding, error) {
	literal, placeholders, trailing, err := ScanPlaceholders(bindText)
	if err != nil || len(placeholders) != 1 || literal[0] != "" || trailing != "" || placeholders[0].Reserved {
		return RowBinding{}, err
	}
	e, err := Parse(placeholders[0].Inner)
	if err != nil {
		return RowBinding{}, err
	}
	if path, ok := e.(*PathExpr); ok {
		if rest, ok := rowScopedRest(path.Segments, rowAlias); ok {
			return RowBinding{Field: rest, Editable: true}, nil
		}
	}
	return RowBinding{}, nil
}

// RewriteRowBinding migrates the two D-1.4.1 row binding shapes during an
// alias edit.  It reports used=true when the old alias appears anywhere in a
// parsed expression, allowing the command boundary to reject a non-migratable
// expression atomically rather than silently changing its scope.
func RewriteRowBinding(bindText, oldAlias, newAlias string) (rewritten string, migrated, used bool, err error) {
	literal, placeholders, trailing, err := ScanPlaceholders(bindText)
	if err != nil {
		return "", false, false, err
	}
	if len(placeholders) != 1 || literal[0] != "" || trailing != "" || placeholders[0].Reserved {
		for _, part := range placeholders {
			if part.Reserved {
				continue
			}
			node, parseErr := Parse(part.Inner)
			if parseErr != nil {
				return "", false, false, parseErr
			}
			if expressionUsesRoot(node, oldAlias) {
				used = true
			}
		}
		return bindText, false, used, nil
	}
	e, err := Parse(placeholders[0].Inner)
	if err != nil {
		return "", false, false, err
	}
	used = expressionUsesRoot(e, oldAlias)
	if path, ok := e.(*PathExpr); ok {
		if rest, ok := rowScopedRest(path.Segments, oldAlias); ok {
			return "{{" + newAlias + "." + rest + "}}", true, true, nil
		}
	}
	if call, ok := e.(*CallExpr); ok && call.Name == "formatNumber" && len(call.Args) == 2 {
		if path, ok := call.Args[0].(*PathExpr); ok {
			if rest, ok := rowScopedRest(path.Segments, oldAlias); ok {
				if pattern, ok := call.Args[1].(*StringLit); ok {
					return "{{formatNumber(" + newAlias + "." + rest + ", " + strconv.Quote(pattern.Value) + ")}}", true, true, nil
				}
			}
		}
	}
	return bindText, false, used, nil
}

func expressionUsesRoot(e Expr, root string) bool {
	if path, ok := e.(*PathExpr); ok {
		return len(path.Segments) > 0 && path.Segments[0] == root
	}
	for _, child := range Children(e) {
		if expressionUsesRoot(child, root) {
			return true
		}
	}
	return false
}

// DerivedFooter is D-1.4.1's footerOf/footerFormat derivation result
// — resolved ALONGSIDE the document, never written back into it (R2:
// internal/template/serialize.go emits "footerOf" whenever it is SET,
// so writing a derived value back would break D-1.4.3's P3 fixed
// point, Serialize(Parse(b)) == b, for every legitimate document that
// omits footerOf).
type DerivedFooter struct {
	FooterOf string

	// FooterFormat/HasFooterFormat are set only when shape 2 (a
	// formatNumber(...) call) supplied the default pattern (D-1.4.1);
	// shape 1 (a bare row-scoped path) derives no format at all.
	FooterFormat    string
	HasFooterFormat bool
}

// DeriveFooterOf implements D-1.4.1's two derivable `bind` shapes,
// forced to live here (internal/expr) rather than in internal/template
// by F2: template (stage rank 2) can never import expr (rank 3), so it
// cannot parse its own columns' bind strings to check derivability.
// folio8.ParseTemplate (the module root, unranked) is this function's
// only caller (R1).
//
//  1. a bare row-scoped path "{{<alias>.<rest>}}" → footerOf =
//     "<collection>.<rest>";
//  2. a single formatNumber(<bare row-scoped path>, <pattern literal>)
//     call → footerOf = "<collection>.<rest>" from the first argument,
//     AND footerFormat defaults to the pattern literal.
//
// derivable reports whether bindText matched one of the two shapes
// above; when it does not, the caller (folio8.go) is the one that turns
// that into AC21's rejection-arm load error — this function only
// reports the fact, it never itself decides whether non-derivability
// is an error (a column with no `footer` requested at all may
// legitimately have an arbitrary, non-derivable bind).
//
// err is returned only for a genuine failure to even PARSE bindText —
// which should never happen here in practice, since the caller runs
// this only after its own general syntax/arity walk already accepted
// every binding in the document; it is surfaced rather than ignored
// purely so this function is honest about the possibility.
func DeriveFooterOf(bindText, rowAlias, collection string) (result DerivedFooter, derivable bool, err error) {
	literal, placeholders, trailing, serr := ScanPlaceholders(bindText)
	if serr != nil {
		return DerivedFooter{}, false, serr
	}
	// Both derivable shapes are bindText being EXACTLY ONE placeholder
	// — no surrounding literal text, and not one of AD-4's reserved
	// tokens (which are never expressions at all).
	if len(placeholders) != 1 || literal[0] != "" || trailing != "" || placeholders[0].Reserved {
		return DerivedFooter{}, false, nil
	}

	e, perr := Parse(placeholders[0].Inner)
	if perr != nil {
		return DerivedFooter{}, false, perr
	}

	switch n := e.(type) {
	case *PathExpr:
		// Shape 1: a bare row-scoped path.
		rest, ok := rowScopedRest(n.Segments, rowAlias)
		if !ok {
			return DerivedFooter{}, false, nil
		}
		return DerivedFooter{FooterOf: collection + "." + rest}, true, nil

	case *CallExpr:
		// Shape 2: a single formatNumber(<bare row-scoped path>,
		// <pattern literal>) call.
		if n.Name != "formatNumber" || len(n.Args) != 2 {
			return DerivedFooter{}, false, nil
		}
		pathArg, ok := n.Args[0].(*PathExpr)
		if !ok {
			return DerivedFooter{}, false, nil
		}
		rest, ok := rowScopedRest(pathArg.Segments, rowAlias)
		if !ok {
			return DerivedFooter{}, false, nil
		}
		patternArg, ok := n.Args[1].(*StringLit)
		if !ok {
			return DerivedFooter{}, false, nil
		}
		return DerivedFooter{
			FooterOf:        collection + "." + rest,
			FooterFormat:    patternArg.Value,
			HasFooterFormat: true,
		}, true, nil

	default:
		return DerivedFooter{}, false, nil
	}
}

// rowScopedRest reports whether segments is a dotted path whose FIRST
// segment is rowAlias and which names at least one field beyond it
// (a bare "{{<alias>}}" alone is a namespace reference, not a numeric
// source), returning the remaining segments joined by ".".
func rowScopedRest(segments []string, rowAlias string) (string, bool) {
	if len(segments) < 2 || segments[0] != rowAlias {
		return "", false
	}
	return strings.Join(segments[1:], "."), true
}
