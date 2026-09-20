// This file declares Render (and, at Story 1.7, RenderTo) — the
// world-reading fence (D-1.6.3, AC12): it imports NONE of "os",
// "time", "net", "math/rand". The shell entry points that DO need
// those (LoadTemplate, which reads a path from disk) live in the
// different file folio8.go, which may (D-1.4.6 draws the "os" boundary
// at the package, not the file, but this file narrows it further:
// AC12's property is decidable and exact — "the file declaring Render
// imports none of the four" — and fails the moment someone adds a
// convenience os.Getenv to the render entry point, the same shape
// D-1.1.b used for internal/pdf/numbers.go).
//
// AD-12 locale clarification (AC14, D-1.6.3): the source AC's "read no
// … locale" means the HOST locale — the machine Render happens to run
// on. Discovering the machine's locale is banned. Reading the
// document's OWN declared locale is not merely allowed but REQUIRED
// (AD-12: "the document declares one locale tag and one fixed UTC
// offset … an unlisted tag is a load error, not a fallback") —
// internal/template's parser already enforces the closed locale set at
// load time (Story 1.4); this file's job is never to substitute the
// host's locale for it.
//
// QA Finding 8 (this story's review, Minor): the blank line above is
// deliberate. Without it, Go treats this block as a SECOND package
// doc comment (there is no blank line requirement violated by having
// one — the issue was this file's comment sitting directly above
// `package folio8` with folio8.go's own package comment already present
// elsewhere), and `go doc .` published every ruling id and story
// number here into the library's public documentation, concatenated
// after folio8.go's real package comment. A blank line demotes this to
// an ordinary file comment; AC14's AD-12 locale paragraph stays here,
// where it documents this file specifically.

package folio8

import (
	"errors"
	"fmt"
	"io"

	"github.com/panitw/folio8/folio-go/internal/bind"
)

// Data is the caller's report data as raw JSON bytes (AC22-AC24,
// D-1.6.4). A named, defined type — never []byte and never a type
// alias (AC23): Data and Params become adjacent same-typed arguments
// at Story 1.7, and in a product whose acceptance fixture is a bank
// statement, that swap must be a compile error, not a support ticket
// (D-1.1.c). Bytes, not a decoded value (AC24): AD-23 requires the
// library to own the UseNumber-preserving decode
// (internal/bind.DecodeData) — a caller-decoded `any` or
// `map[string]any` would arrive with its number literals already
// destroyed by encoding/json's default float64 conversion.
type Data []byte

// Params is the caller's RUNTIME values — a statement date, say — as
// raw JSON bytes (AC18-AC20, D-1.7.5). Deliberately a SECOND, separate
// channel from Data (AC12-AC17, D-1.7.4): report data cannot reach
// into "{{params.…}}", and a top-level "params" key inside Data is
// legal, unreachable, ordinary caller JSON (AC15).
//
// A named, DEFINED type, never []byte and never a type alias (AC18,
// D-1.7.5, verbatim): "An alias (type Params = []byte) would make the
// two mutually assignable and destroy the entire point" — Data and
// Params are adjacent, same-underlying-type arguments in Render's
// signature, and in a product whose acceptance fixture is a bank
// statement, swapping them must be a compile error, not a support
// ticket (D-1.1.c). TestParamsDataSwapDoesNotTypeCheck (AC19) proves
// this by construction rather than by comment (AC19a: Story 1.6
// shipped Data's equivalent property as prose only — this is refused
// here for both types).
//
// Decoded through the SAME path as Data (AC20): internal/bind.DecodeData,
// the same UseNumber discipline, the same single literal splitter
// (D-1.6.1), the same Decimal — no second decoder, because params
// carry dates and amounts, where a silent divergence would be least
// visible and most expensive (D-1.7.5). A nil or empty Params means no
// runtime values were supplied — not a decode error — and is treated
// as an empty params document: any "{{params.x}}" placeholder is then
// simply absent (AC16), the same located-error shape as an absent data
// path (AD-14).
type Params []byte

// errNilTemplate is AC14b's located error for Render(nil, d, p, f).
var errNilTemplate = errors.New("folio8: Render: template is nil (a document must be loaded via LoadTemplate/ParseTemplate first)")

// errNilWriter is this story's review, Finding 4 (Major): w is a NEW
// public argument RenderTo introduces, and a public entry point
// documented as ignoring its argument is worse than a located error
// (D-1.5.9) — the same reasoning Story 1.5 applied to Render(nil
// template, ...), applied here to RenderTo(nil writer, ...).
var errNilWriter = errors.New("folio8: RenderTo: writer is nil")

// decodeParams decodes p through internal/bind.DecodeParams — the SAME
// decode path Data uses (AC20; DecodeData and DecodeParams share one
// json.NewDecoder/UseNumber call, internal/bind/decodeguard_test.go's
// guard), reporting errors as belonging to "params", never "report
// data" (this story's review, Finding 6: DecodeData's messages named
// the wrong root when reused verbatim for params) — with one narrow
// exception: a nil or empty Params is "no runtime values supplied"
// (AC16's premise), not malformed JSON, so it decodes to an empty
// object without invoking the decoder at all. This does not add a
// second decode path: it is a zero-length short-circuit before the one
// decoder, never an alternative to it, so the AC20 guard (exactly one
// json.NewDecoder/UseNumber site under internal/bind) is unaffected.
func decodeParams(p Params) (bind.Value, error) {
	if len(p) == 0 {
		return bind.Value{Kind: bind.KindObject}, nil
	}
	v, err := bind.DecodeParams(p)
	if err != nil {
		return bind.Value{}, fmt.Errorf("folio8: Render: %w", err)
	}
	return v, nil
}

// Render produces a PDF 1.7 document from t, resolving every
// "{{…}}" placeholder in every text element's value against d and p
// (AC1, AC12-AC21, D-1.6.5, D-1.7.4) and embedding every font the
// document's text elements use as a subset (AC1, D-1.5.5): p is
// inserted into its final target position, never appended — target is
// Render(t, d, p, f), so 1.6 added d before f and 1.7 adds p between
// them (D-1.5.5, verbatim: "No call site is ever reordered, only
// extended").
//
// t must be non-nil (AC14b): Story 1.1's fixture document
// (fixtures/minimal-rect/) predates the public API accepting a
// template at all and is pinned via internal/pdf.Serialize() directly
// (AC14a) — a public Render documented as silently ignoring its
// argument is worse than a located error on nil (D-1.5.9).
//
// d must be syntactically valid JSON (AC24); it is decoded once, via
// internal/bind.DecodeData (json.Decoder.UseNumber under the hood), so
// every number literal a bound text placeholder resolves to keeps its
// own author-written precision (AD-23) rather than being narrowed
// through float64. p is decoded through the same path (AC20); a nil
// or empty p means no runtime values were supplied (decodeParams
// above). A top-level "params" key inside d is legal caller JSON and
// simply unreachable by any binding — the caller's JSON shape is their
// business; only the binding namespace is ours (AC15, D-1.7.4).
//
// Text and images are placed by internal/layout (Story 2.5, AD-24):
// each element's page-absolute Y is its band's origin plus its own
// band-relative Y, a translation and never an inversion, and the
// content band's height is derived from page geometry alone by
// internal/layout.ContentHeight. Story 1.6 gave Render its data
// parameter and Story 1.7 its params parameter and its io.Writer form,
// RenderTo (D-1.1.c).
//
// Story 2.8 (D-2.8.3, the owner's decision): Render returns a Result
// rather than a bare []byte. A non-nil error still means exactly what
// it always did — no bytes, nothing rendered — and Result{}.Bytes is
// only ever meaningful on the err == nil path, unchanged in spirit from
// before this story. What is new is Result.Diagnostics: content that
// could not be honoured exactly as declared (FR44: an element's widest
// line exceeding its declared width) is clipped at the box boundary and
// reported as a Diagnostic alongside complete, valid bytes — never
// silently, and never by failing the render (AD-14). See Result's own
// doc comment for the ordering and emptiness guarantees Diagnostics
// carries.
//
// f MAY BE EMPTY, OR NIL, AND THAT IS NOT AN ARGUMENT ERROR. There is
// no default font set and no ambient lookup — the engine still never
// goes looking for fonts on the machine it runs on — but a document
// whose every chain entry resolves to a face the document itself
// CARRIES has nothing for a font set to contribute, and refusing such a
// call before resolution has been attempted refuses a render that would
// have succeeded. An entry that genuinely cannot be resolved still
// fails, as DiagCodeTextFaceAbsent, at the point of use.
//
// fallback is the optional resolution-mode selector, and omitting it —
// which every call written before it existed does — asks for
// FaceFallbackStrict: today's behaviour, byte for byte. Passing
// FaceFallbackSubstitute asks for a rune no present chain member covers
// to be painted in a face this renderer WAS given rather than refused,
// with a DiagCodeTextFaceSubstituted Warning naming what happened. It
// is variadic because that is the only shape Go offers for an optional
// argument on a frozen signature (AD-22); passing more than one, or a
// value outside the closed set, is a named error rather than a clamp.
func Render(t *Template, d Data, p Params, f FontSet, fallback ...FaceFallback) (Result, error) {
	if t == nil {
		return Result{}, errNilTemplate
	}
	data, err := bind.DecodeData(d)
	if err != nil {
		return Result{}, fmt.Errorf("folio8: Render: %w", err)
	}
	params, err := decodeParams(p)
	if err != nil {
		return Result{}, err
	}
	b, diags, rerr := renderDocument(t, data, params, f, fallback...)
	if rerr != nil {
		return Result{}, rerr
	}
	return Result{Bytes: b, Diagnostics: diags}, nil
}

// RenderTo produces the same document as Render, but writes it to w
// instead of returning it (AC1, D-1.7.1). It takes the same five
// arguments in the same order — w is the sole addition, always first
// — and no options struct wraps them: an options struct would make f
// omittable at compile time (folio8.Request{Template: t, Data: d} would
// compile with a zero FontSet), turning an AD-8 violation from a
// compile error into a runtime one (D-1.7.1, verbatim).
//
// RenderTo CANNOT stream, and this is not an oversight (AC3, D-1.7.2):
// AD-7 requires /ID to be the first 16 bytes of a SHA-256 hash taken
// over the serialized body up to the point /ID itself is written, and
// the classic cross-reference table records each object's BYTE OFFSET
// within the finished file — both facts require the complete document
// to exist before the trailer, and therefore the first byte written to
// w, can be produced. "'No buffering of the whole document' is a
// property we cannot have, and asserting it would assert something
// false" (D-1.7.2). Do not "optimise" this into an incremental writer:
// doing so would silently break AD-7's content-derived /ID and the
// xref table's offsets. RenderTo therefore builds the document once,
// via the single shared core Render also uses (renderDocument, AC4),
// then issues exactly one logical write of the result — never a
// second call carrying a tail (AC8) — and turns every writer failure
// into a located, non-nil error (AC5-AC8), INCLUDING w itself being
// nil (this story's review, Finding 4: w is a new public argument this
// story introduces, and D-1.5.9's "a public entry point documented as
// ignoring its argument is worse than a located error" applies to it
// exactly as it applies to a nil template):
//
//   - w == nil returns errNilWriter, never a panic;
//   - a Write that returns a non-nil error is surfaced, never
//     swallowed or replaced by an unrelated error (AC5);
//   - a short write (n < len(b) with err == nil, which io.Writer
//     permits) is itself reported as an error — never silently
//     treated as success (AC6, the same failure mode Story 1.1's
//     TestMain caught, Nit 25);
//   - the number of bytes actually accepted by w is verified against
//     len(b) via a counting write (AC7).
//
// This file (AC9, D-1.6.3/D-1.7.3) is located by lint's
// findRenderDeclaringFiles — BY AST, never by filename — as the file
// declaring Render and RenderTo, and is scanned for forbidden imports
// (time/os/net/math/rand). AC11's residual gap, MEASURED not merely
// asserted (M-5): that guard is a file-scoped import rule, so it
// cannot stop a DELIBERATE cross-file route — RenderTo staying in this
// clean file while calling an os.WriteFile helper declared in
// folio8.go (which legitimately imports "os" for LoadTemplate) still
// builds and the guard still passes. Source AC1's "no temporary file"
// is therefore a CAPABILITY claim ("you can serve a PDF without
// touching disk"), not a security boundary the guard proves — AC7/AC8's
// counting-writer assertions above are the behavioural half that
// actually pins what this implementation does. A filesystem-snapshot
// check to close this gap is deliberately NOT built (disproportionate,
// and it would test the OS rather than this library).
//
// It takes the same optional trailing FaceFallback Render does, which
// is what keeps the two argument lists a prefix of one another.
//
// D-2.8.6: RenderTo returns ([]Diagnostic, error), NOT Result. Result
// names what a render PRODUCED, and RenderTo does not produce bytes to
// its caller — it writes them to w as a side effect. A Result whose
// Bytes were always nil on this path would be a claim that is false and
// unobservable: nil, empty, and "the bytes went to the writer" are
// indistinguishable at the call site. This asymmetry with Render's
// signature is deliberate and costs this doc comment, not a trap.
//
// D-2.8.2/D-2.8.6: a Warning is not an error (AD-14), so a document that
// renders with warnings still writes its COMPLETE bytes to w and still
// returns those warnings — RenderTo never again produces "no output at
// all" for a document that rendered successfully with something to
// report.
//
// fallback is Render's optional FaceFallback, forwarded verbatim and
// documented there. Omitting it is FaceFallbackStrict. Forwarding it is
// load-bearing rather than tidy: a lenient caller who reached for the
// streaming form and silently got a refusal would have no way to tell
// the binding had dropped their selector.
func RenderTo(w io.Writer, t *Template, d Data, p Params, f FontSet, fallback ...FaceFallback) ([]Diagnostic, error) {
	if w == nil {
		return nil, errNilWriter
	}
	res, err := Render(t, d, p, f, fallback...)
	if err != nil {
		return nil, err
	}
	n, err := w.Write(res.Bytes)
	if err != nil {
		return nil, fmt.Errorf("folio8: RenderTo: write failed after %d of %d bytes: %w", n, len(res.Bytes), err)
	}
	if n != len(res.Bytes) {
		return nil, fmt.Errorf("folio8: RenderTo: short write: wrote %d of %d bytes, writer reported no error", n, len(res.Bytes))
	}
	return res.Diagnostics, nil
}
