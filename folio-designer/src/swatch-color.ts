// THE ONE HEX-COLOUR PREDICATE THE DESIGNER OWNS, and the narrow job it is
// allowed to do.
//
// It does NOT decide whether a colour is legal — Go's parseHexColor is that
// gate, asked by the loader and by every command arm that writes a colour, and
// its located sentence is what the author reads. This decides two presentation
// questions the browser genuinely owns: what colour the native picker OPENS on,
// and whether the chip beside it is drawn as SET or as UNSET.
//
// AND THE UNSET HALF IS WHY THIS IS SHARED RATHER THAN COPIED. `swatchColor('')`
// returns black — it has to, because `<input type="color">` accepts nothing
// else — so a control that renders an absent colour without also applying
// `property-swatch-unset` shows a confident black chip for a field the document
// does not declare. That is the same class of defect as a missing "Not set"
// placeholder, wearing a picker, and it is invisible to every command
// assertion. One predicate, one place, so a second surface cannot get the pair
// half right.
export const isHexColour = (text: string): boolean => /^#[0-9a-fA-F]{6}$/.test(text)

export const swatchColor = (text: string): string => isHexColour(text) ? text : '#000000'
