// STORY 12.2. The document's locale and its UTC offset, as opaque Go-defined
// bytes.
//
// They are COMPONENT commands, not page-setup ones, and that is the whole
// reason this file exists rather than two more keys on pageSetupCommand: the
// engine's page-setup door gates on a seven-key arity that every caller's shape
// depends on, while the component door dispatches on `kind` and counts each
// arm's own fields. band-height-command.ts is the shipped precedent for exactly
// this, one story earlier.
//
// TWO FUNCTIONS, ONE FIELD EACH, because Go has two arms and not one (Story
// 15.2a: a command names exactly what it changes). `locale` and `utcOffset` are
// independent top-level document fields with no shared shape and no shared
// validation, so a single command would have to carry a discriminator that
// could rotate, and could refuse a good locale because of a bad offset.
//
// THIS MODULE HOLDS NO RULE OF ITS OWN. It does not clamp, normalise, or
// validate: AD-12's closed set and ±HH:MM are the ENGINE's rules — one exported
// predicate each, asked by the loader and the command door alike — and the
// existing role="alert" path renders the engine's own located sentence. The
// locale parameter is typed by LocaleTag, which is derived from
// engine-protocol.ts's LOCALE_TAGS and tied to Go by
// engine-bounds-mirror.test.ts; the tags are NOT spelled again here, because a
// copy outside that census is the only kind that can go stale unnoticed.
import { commandBytes, jsonString } from './command-json'
import type { LocaleTag } from './engine-protocol'

export function documentLocaleCommand(locale: LocaleTag): ArrayBuffer {
  return commandBytes('setDocumentLocale', [['locale', jsonString(locale)]])
}

// `utcOffset` travels as the author's DRAFT, passed as typed and quoted by
// jsonString. An emptied box therefore reaches Go as `""`, which the arm
// refuses by naming the field — not as a `null` and not as a value nobody
// typed. That is the same promise band-height-command.ts makes with jsonNumber,
// in the shape a STRING field takes.
export function documentUTCOffsetCommand(utcOffset: string): ArrayBuffer {
  return commandBytes('setDocumentUTCOffset', [['utcOffset', jsonString(utcOffset)]])
}
