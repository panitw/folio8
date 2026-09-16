package designer

// StandInInstant is formatDate's stand-in operand: ONE FIXED INSTANT,
// never time.Now(). Byte-determinism is the whole point — the same
// template must produce the same stand-in bytes on every run and every
// target, because those bytes are hashed into the preview identity.
//
// WHY MID-RANGE AND NOT AN EDGE INSTANT. validateCivilRanges and
// civilFromInstantMs (internal/expr) re-bound the SHIFTED civil year to
// [1, 9999], so an instant near either bound can flip validity under the
// document's own utcOffset. A midday instant in the middle of the range
// cannot.
const StandInInstant = "2024-01-15T12:00:00Z"

// MaxStandInDataPaths bounds how many distinct data paths one template
// may contribute to a stand-in document, and MaxStandInDataBytes bounds
// the document those paths produce.
//
// THE GENERATOR MUST BOUND ITS OWN OUTPUT. engine.worker.ts decodes a
// byte response with `base64ToBytesBounded(..., operation === 'render' ?
// MAX_ENGINE_RENDER_PDF_BYTES : undefined)` — `undefined` for every
// operation that is not a render, this one included — so there is no
// transport bound behind this one. ParameterReferences bounds at 128 for
// the same reason.
const (
	MaxStandInDataPaths = 512
	MaxStandInDataBytes = 256 << 10
)
