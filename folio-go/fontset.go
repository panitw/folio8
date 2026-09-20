package folio8

// FontSet is the engine's explicit font input (AC1, AC2, AD-8). It maps
// a face name — as referenced by a `.folio` document's `fonts` fallback
// chains — to that face's raw OpenType/TrueType font program bytes.
//
// The engine never goes looking for fonts on the machine it runs on: a
// render either finds the face it needs in the FontSet the caller
// supplied, in the document's own embedded assets, or it fails with a
// located error. internal/fontset performs resolution and subsetting
// entirely against the value handed to it here — it never embeds font
// data itself and never queries the host (AC2, AC3, AC4).
//
// A FontSet MAY BE EMPTY, OR NIL, AND THAT IS NOT A CALLER ERROR. "No
// default font set" is unchanged — there is still no ambient lookup and
// the engine still owns no faces of its own. What changed is that
// SUPPLYING NONE stopped being a caller error when the document needs
// none: a template whose every chain entry resolves to a face the
// document itself carries has nothing for a set to contribute, and no
// argument check may refuse such a call before resolution has been
// attempted. An entry that genuinely cannot be resolved still fails as
// DiagCodeTextFaceAbsent, at the point of use.
//
// It stays map[string][]byte, deliberately. The resolution-mode selector
// that arrived with substitution is an optional argument on Render,
// RenderTo and Validate (FaceFallback), never a field here: making this
// a struct would break every caller in three languages for a property
// that is not about the fonts at all.
//
// Declared in package folio8 at the module root (spine §Source tree:
// "fontset.go — FontSet as a public input"), not under internal/: a
// public input type is not font DATA, so AD-8's "no package under
// internal/ embeds font data" does not apply to declaring its shape
// here — only internal/fontset's own non-test files are forbidden from
// carrying a go:embed directive naming a font file (AC3).
type FontSet map[string][]byte
