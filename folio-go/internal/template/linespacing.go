package template

import (
	"encoding/json"
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// This file is `style.lineSpacing`'s ONE validation function (Story 7.2,
// D-7.2.3), and it lives in internal/template for a structural reason
// rather than a tidy one: the module root imports this package, so this
// package may never import the module root (AD-1). A load-time-validated
// key therefore needs its predicate here, not in the module root — the
// precedent the colour predicate (colour.go) has since followed.
// internal/template is the only place BOTH the load path
// (parse_bands.go's decodeStyle) and the property-command path
// (folio-go/component_commands.go) can reach the same function — which
// is what makes "a value refused in a file is refused in the inspector,
// for the same reason" true by construction instead of by discipline.

// LineSpacingUnit is the denominator `lineSpacing` is carried in: a
// whole number of THOUSANDTHS, so the authored `1.5` is 1500 and the
// absent default is exactly LineSpacingUnit. It is dimensionless — a
// ratio applied to the vertical model's Advance — which is why the
// model carries an int64 count and never a geom.Length (AD-2/AD-23: the
// unit system is millipoints, and a ratio is not a length).
const LineSpacingUnit = 1000

// MinLineSpacingThousandths and MaxLineSpacingThousandths are D-7.2.3's
// load-time range: [1, 1000000], i.e. 0.001 to 1000.0 inclusive.
//
// THE RANGE CARRIES NO TYPOGRAPHIC OPINION, and both bounds need that
// said out loud, because both look like they do.
//
// There is deliberately NO 1.0 minimum. Tight leading — one line's
// letters reaching into the line below — is what a filed contract is
// set in, and a load-time check cannot see the font size, so a bound
// pretending to be typographic while blind to the input that determines
// it is wrong at every size but one. (Measured on the shipped chain at
// 12pt, the canvas's old overlap cliff was 784 thousandths rejects /
// 785 passes; it MOVES with face and size.)
//
// MaxLineSpacingThousandths is a STATED SANITY CEILING, NOT A DERIVED
// SAFETY BOUND. Deriving a ceiling honestly from int64 overflow at the
// scaling site gives roughly 1023 THOUSANDTHS — that is, an honest
// overflow-derived ceiling would forbid `lineSpacing` above 1.0
// altogether. That reductio is the whole reason this number is a sanity
// bound and nothing more; the real overflow precondition is discharged
// where both operands exist, in the leading model (D-7.2.4), never here.
// Do not re-propose either bound as typography.
const (
	MinLineSpacingThousandths = 1
	MaxLineSpacingThousandths = 1000000
)

// ValidateLineSpacingThousandths is the single predicate over an
// already-decoded thousandths count. It returns the REASON a value is
// refused, phrased so it can be handed unchanged to a load error's
// Reason and to a property command's error alike.
func ValidateLineSpacingThousandths(thousandths int64) error {
	if thousandths < MinLineSpacingThousandths || thousandths > MaxLineSpacingThousandths {
		return fmt.Errorf(
			"lineSpacing must be between %d and %d thousandths (%s to %s); %d is outside that range",
			MinLineSpacingThousandths, MaxLineSpacingThousandths,
			FormatLineSpacing(MinLineSpacingThousandths), FormatLineSpacing(MaxLineSpacingThousandths),
			thousandths,
		)
	}
	return nil
}

// DecodeLineSpacing is THE function both entry points call: it converts
// a JSON number literal to a whole number of thousandths through the
// module's existing exact-decimal path (decodePoints — the same
// big.Int arithmetic `fontSize` uses, which rejects more than three
// decimal places and int64 overflow and never rounds), then applies
// ValidateLineSpacingThousandths.
//
// The ×1000 decodePoints performs to reach millipoints is exactly the
// ×1000 a thousandths ratio needs, so the ≤3-decimal discipline is
// INHERITED rather than restated — there is no second decimal parser
// for this key to drift from.
func DecodeLineSpacing(literal string) (int64, error) {
	v, err := decodePoints(literal)
	if err != nil {
		return 0, err
	}
	if err := ValidateLineSpacingThousandths(int64(v)); err != nil {
		return 0, err
	}
	return int64(v), nil
}

// DecodeLineSpacingRaw is DecodeLineSpacing over a raw JSON token,
// rejecting a non-number (and a quoted number, which decodeNumberRaw
// never coerces) before the decimal path sees it. The property-command
// path in the module root calls this; decodeStyle calls it too, so the
// two cannot diverge on what "a lineSpacing value" even is.
func DecodeLineSpacingRaw(raw json.RawMessage) (int64, error) {
	// NULL IS REFUSED HERE, EXPLICITLY, AND THAT IS NOT BELT AND BRACES.
	// encoding/json silently NO-OPS a JSON null into a json.Number, so
	// decodeNumberRaw returns "" without error and the empty literal
	// reaches decodePoints, which reports `invalid numeric literal ""` —
	// a message about the engine's own parser rather than about anything
	// the author wrote. `lineSpacing` has no meaning for an explicit
	// null (unlike `background` and `color`, where null is "no colour"),
	// so it is refused in the author's terms, worded as the
	// property-command path already words it.
	if rawIsNull(raw) {
		return 0, fmt.Errorf("lineSpacing does not accept null; omit the key to inherit the leading the declared font chain itself rules")
	}
	n, err := decodeNumberRaw(raw)
	if err != nil {
		return 0, err
	}
	return DecodeLineSpacing(string(n))
}

// FormatLineSpacing spells a thousandths count the way the format spells
// it on disk (appendPoints — the canonical exact-decimal spelling), so
// every message about a ratio quotes it in the AUTHOR'S units rather than
// in the engine's: an author who typed `0.6` is told about `0.6`, never
// about `600/1000`.
//
// Exported because the render-time leading guards live in the module
// root (folio-go/wrap.go) and must word a ratio the same way this
// package's load-time refusal does.
func FormatLineSpacing(v int64) string {
	return string(appendPoints(nil, geom.Length(v)))
}
