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
import { commandBytes, jsonBoolean, jsonString } from './command-json'
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

// spec-font-sources-and-embedding CAP-2: whether the document carries its
// faces. A THIRD function rather than a key on either of the two above, by the
// same rule that made those two separate — Go has three arms, one field each,
// and a command names exactly what it changes.
//
// THE VALUE IS A BOOLEAN AND IT TRAVELS AS ONE. jsonBoolean emits `true` or
// `false`, never `"true"` and never `null`: the panel's checkbox has exactly
// two states, so there is no unset value to represent and nothing for the
// engine to guess at. Nothing is validated here either — the engine's arm owns
// the refusal, as it does for the other two.
export function documentEmbedFontsCommand(embedFonts: boolean): ArrayBuffer {
  return commandBytes('setDocumentEmbedFonts', [['embedFonts', jsonBoolean(embedFonts)]])
}

// ─────────────────────────────────────────────────────────────────────────────
// THE STRIP WARNING (story 6, D5).
//
// STRIPPING IS THE ONE GENUINELY DESTRUCTIVE ACT IN THIS EPIC: the faces the
// document carries are deleted, and the only way back is document undo. So it
// is announced before the first one — and only the first, because an author who
// has been told once and toggles the setting again knows what they are asking
// for, and a modal on every toggle is a modal nobody reads.
//
// THE COUNT IS THE FACT THE AUTHOR NEEDS and the entries are not listed: the
// answer is all-or-nothing either way, which is `CompleteFontsDialog`'s own
// reasoning applied to the opposite direction.
export const STRIP_FACES_TITLE = 'Remove the faces this document carries?'

export const stripFacesQuestion = (count: number, unresolvable = 0): string => [
  `${count === 1 ? 'One chain entry carries a face' : `${count} chain entries carry faces`} inside this document.`,
  `Turning embedding off replaces ${count === 1 ? 'it' : 'them'} with the face name and deletes the face itself, so the document will need those faces supplied wherever it is rendered.`,
  // ⚠ SAID BECAUSE IT IS A LOSS THE AUTHOR CANNOT SEE COMING. An entry that
  // carries a face may also declare the ASSET keys of its bold and italic, and
  // the rewrite cannot carry those over — there is no command that writes a
  // name entry's variants onto an entry that already exists. Pressing B again
  // redeclares them by name.
  `Any bold or italic a carried entry declared is discarded with it, until you ask for that weight again.`,
  // ⚠ AND THE ONE THAT WOULD OTHERWISE BE DISCOVERED FROM A WRONG-LOOKING PAGE.
  unresolvable === 0 ? '' : `${unresolvable === 1 ? 'One of those faces is' : `${unresolvable} of those faces are`} not on this machine and not one this release ships, so the preview here will stop drawing ${unresolvable === 1 ? 'it' : 'them'} too.`,
  // ⚠ NOT "one undo puts everything back": the strip and the setting are TWO
  // commands and therefore two history entries. Promising one step in the one
  // dialog whose job is informed consent would be a false statement the author
  // acts on.
  `Undo puts the faces back; the setting itself is a separate step in the document's history.`,
].filter((line) => line !== '').join(' ')

export const STRIP_FACES_CONFIRM_LABEL = 'Remove the faces'
export const STRIP_FACES_DECLINE_LABEL = 'Keep embedding'
