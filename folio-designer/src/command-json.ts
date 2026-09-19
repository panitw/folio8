// THE SINGLE COMMAND-JSON AUTHORITY.
//
// Every byte of every engine command the designer emits is assembled here, and
// nowhere else. This module owns exactly one thing — turning an already-decided
// intent into well-formed JSON transport — and it deliberately owns nothing
// else: it does not know what a command kind is, what fields any kind carries,
// what a valid number is, what a legal id looks like, or whether Go will accept
// what it encodes. Those are engine rules, and the engine answers them with its
// own located sentence.
//
// WHY THIS MODULE EXISTS, stated as the defect it closes. SIX encoders each
// spliced author-supplied and document-supplied text straight into a JSON
// template literal, with three different answers to "escape this". Six, and the
// tally matters more than the number: TWO routed strings through JSON.stringify
// and were right; TWO hand-rolled an escape table that read only the first
// UTF-16 unit of a code point, so an astral character became a lone surrogate
// and bound somewhere nobody typed; ONE escaped nothing at all; and one of the
// two correct ones had a correct helper that two of its own builders did not
// call. That last is the shape that makes auditing hopeless and consolidation
// the only answer — a right helper beside a call site that ignores it reads,
// from the outside, exactly like a right file. A command
// could therefore be made to NAME one component and one property while CHANGING
// a different component's different property — from a typed draft, with the
// author's own selection vanishing from the command entirely. And the non-BMP
// corruption needed no typing at all: a bind segment is a JSON object key taken
// verbatim out of the author's sample-data file, so opening a file and clicking
// a node was enough to address a path nobody picked. Consolidating to one
// encoder is what makes "a command means exactly what it names" a property of
// the designer rather than a claim about whichever encoder was audited last.
//
// THE ESCAPE TABLE IS JSON.stringify AND IS NEVER HAND-ROLLED. It is the only
// correct answer, it handles the whole of U+0000-U+001F, and since ES2019 it
// emits well-formed output for lone surrogates too. A hand-rolled subset is the
// defect above, twice.
//
// NUMBERS TRAVEL BYTE-FOR-BYTE, unquoted, AND ARE NEVER RE-COMPUTED. The
// numeric path applies exactly one test — is this draft already a valid JSON
// number? — and then either passes the author's own literal through untouched
// or replaces it with `null`. There is no Number() round trip anywhere on it.
//
// WHY NOT Number(): IT WIDENS THE ENGINE'S ACCEPT-SET, which is the opposite of
// this story's purpose. Measured on the coerced version this replaces: `1e3`
// reached the wire as `1000`, `0x10` as `16`, `0b101` as `5`, `.5` as `0.5`,
// `007` as `7`, `+5` as `5`, `" 12 "` as `12`, and `9007199254740993` as
// `9007199254740992` — every one of which Go had refused, or received exactly,
// before. The signature of that defect is `1e3` being accepted while `1e21` is
// still refused: one input class splitting two ways is never designed.
//
// THE SHAPE TEST IS NOT A RULE ABOUT WHAT A NUMBER MEANS. It decides only
// whether bytes can REACH Go's rule, never what they signify, and it is the
// JSON number grammar exactly — no narrower. Narrower would be a second
// authority sneaking back in through narrowness instead of coercion: a decimals
// only check would send `1e3` as `null` and cost the located "must be a decimal
// with at most three places" that Go answers today. Go alone rejects exponents,
// rejects more than three decimal places, and bounds against
// MaxCanvasMillipoints.
//
// A BLANK DRAFT BECOMES `null`, and it falls out of the same test rather than
// needing a rule of its own — the empty string is not a JSON number. That
// matters because Number('') is 0 in JavaScript, and silently sending 0 for a
// box the author emptied is exactly the class of invented value this module
// exists to stop. `null` is the convention page setup already shipped, and the
// engine answers it by naming the field and the cause.
//
// THE FIELD LIST IS STILL WRITTEN OUT AT EACH CALL SITE, in order, because the
// engine counts every top-level key and refuses any other arity. Passing the
// fields as an ordered list keeps that count readable at the builder while
// taking the quoting away from it.
const encode = (text: string): ArrayBuffer => new TextEncoder().encode(text).buffer

// A field is a key and an ALREADY-ENCODED JSON value fragment. Keeping the
// value pre-encoded is what lets one builder nest an object or an array inside
// another without either of them writing a brace.
export type JsonField = readonly [key: string, value: string]

export const jsonString = (value: string): string => JSON.stringify(value)

// The JSON number grammar, verbatim from the specification's own railroad: an
// optional minus, an integer part with no leading zeros, an optional fraction
// with at least one digit, and an optional exponent. Nothing else is a JSON
// number, and nothing else may travel unquoted.
const JSON_NUMBER = /^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?$/

// A caller-supplied JS number is spelled by String(), which agrees with
// JSON.stringify on every finite value and yields NaN/Infinity — neither a JSON
// number — for the rest, so those become `null` exactly as they always did.
// An author's DRAFT is already text and is tested as typed, never re-parsed.
export const jsonNumber = (value: number | string): string => {
  const literal = typeof value === 'string' ? value : String(value)
  return JSON_NUMBER.test(literal) ? literal : 'null'
}

export const jsonBoolean = (value: boolean): string => JSON.stringify(value)

export const jsonArray = (values: ReadonlyArray<string>): string => `[${values.join(',')}]`

// THE AUTHORITY MUST BE INCAPABLE OF EMITTING WHAT THE ENGINE NOW REFUSES.
// Go's two exported command doors reject an object that declares the same key
// twice, at any nesting level — so an encoder that could still BUILD one would
// leave the designer able to compose a command whose meaning is decided by
// last-wins somewhere. It throws rather than dropping or renaming: a duplicate
// key here is a programming error at a call site, not a value to repair, and
// silently keeping one of the two is how the ambiguity survives.
export const jsonObject = (fields: ReadonlyArray<JsonField>): string => {
  const seen = new Set<string>()
  return `{${fields.map(([key, value]) => {
    if (seen.has(key)) throw new Error(`command JSON cannot declare ${JSON.stringify(key)} twice in one object`)
    seen.add(key)
    return `${jsonString(key)}:${value}`
  }).join(',')}}`
}

// The envelope every command shares: the kind, the protocol version, then the
// kind's own fields in the order the builder lists them.
export const commandBytes = (kind: string, fields: ReadonlyArray<JsonField>): ArrayBuffer =>
  encode(jsonObject([['kind', jsonString(kind)], ['version', jsonNumber(1)], ...fields]))

