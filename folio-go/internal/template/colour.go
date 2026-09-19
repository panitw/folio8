package template

// This file is the format's ONE colour predicate, and it lives in
// internal/template for the reason linespacing.go gives: the module root
// imports this package, so this package may never import the module root
// (AD-1). Only here can the load path (decodeStyle, decodeBorder,
// decodeTableRules and decodeTableExt's altRowBackground) and the
// command doors in the module root (component_commands.go,
// page_setup.go) ask the same question — which is what makes "a colour
// refused in a file is refused in the inspector" true by construction.
//
// COLOUR IS CHECKED AT LOAD (owner ruling, 2026-09-17, with D-7.8.2's
// retirement of the render-time colour code). Until then a malformed
// colour loaded clean and failed the render that first consumed it, so
// a bad colour on an element that never draws — a hidden one, or an
// empty table — was never refused at all. It is now refused at load with
// TEMPLATE_FIELD_INVALID, located at the field, and the render sites that
// decode a colour (the module root's parseHexColor) can no longer see
// one they cannot decode.
//
// The predicate only answers "is this #RRGGBB"; decoding the channels
// stays with parseHexColor in the module root, which is the renderer's
// business and not the format's.

// ColourReason is the load error's and the command doors' reason for a
// value IsHexColour refuses, so a file and an inspector edit word the
// refusal alike.
const ColourReason = "must be a #RRGGBB colour"

// IsHexColour reports whether s is exactly `#` followed by six hex
// digits, in either case — the format's only colour spelling
// (folio-format.md: "Colours are #RRGGBB").
func IsHexColour(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		switch b := s[i]; {
		case b >= '0' && b <= '9', b >= 'a' && b <= 'f', b >= 'A' && b <= 'F':
		default:
			return false
		}
	}
	return true
}

// colourLoadError is the located load refusal for a colour field that is
// not #RRGGBB. A value carrying a `{{ }}` placeholder is worded as the
// colour-by-data refusal it is, so the author learns why a data-driven
// colour cannot work rather than only that the spelling is wrong.
func colourLoadError(field, elementID, value string) error {
	reason := ColourReason
	if containsPlaceholderOpen(value) {
		reason += " — conditional/data-driven styling is not supported: a component's condition turns it on or off (visibleIf), it never changes how the component looks"
	}
	return newLoadError(field, elementID, value, reason)
}

func containsPlaceholderOpen(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '{' && s[i+1] == '{' {
			return true
		}
	}
	return false
}
