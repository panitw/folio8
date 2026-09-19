package folio8

// FontSet is the engine's explicit font input (AC1, AC2, AD-8). It maps
// a face name — as referenced by a `.folio` document's `fonts` fallback
// chains — to that face's raw OpenType/TrueType font program bytes.
//
// The engine never goes looking for fonts on the machine it runs on: a
// render either finds the face it needs in the FontSet the caller
// supplied, or it fails with a located error. internal/fontset performs
// resolution and subsetting entirely against the value handed to it here
// — it never embeds font data itself and never queries the host (AC2,
// AC3, AC4).
//
// Declared in package folio8 at the module root (spine §Source tree:
// "fontset.go — FontSet as a public input"), not under internal/: a
// public input type is not font DATA, so AD-8's "no package under
// internal/ embeds font data" does not apply to declaring its shape
// here — only internal/fontset's own non-test files are forbidden from
// carrying a go:embed directive naming a font file (AC3).
type FontSet map[string][]byte
