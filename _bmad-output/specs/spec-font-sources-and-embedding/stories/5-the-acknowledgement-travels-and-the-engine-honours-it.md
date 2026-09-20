---
title: 'The acknowledgement travels, and the engine honours it'
type: 'feature'
created: '2026-09-21'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: 'f746c16a0b7126264214bcb2942f5f83a413ef15'
context:
  - '{project-root}/_bmad-output/specs/spec-font-sources-and-embedding/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 3 lets an author import a face on their acknowledgement, but that acknowledgement
is a designer-time gesture the engine never sees. Two doors then refuse the face anyway: a binary
whose own name table names a copyleft licence is refused whatever is declared beside it, and a
record whose `licence`, `licenceText` or `copyright` is blank is refused outright — which is exactly
the face an author-supplied binary often is.

**Approach:** The font record carries the acknowledgement, so it travels with the document and a
developer opening someone else's `.folio` can see the assertion was made. Every door that asks the
licence question honours it. One field does both jobs: what the document records is what the engine
reads.

## Decisions

- **D1 — This is the `5.0` the owner ruled on, and it is derived.** Apply D-7.3.1's test: would a
  pre-`5.0` reader **refuse** the file or **render it wrong**? It **refuses** — twice over.
  `requireEmbeddedFaceLicence` (`parse.go:782-805`) rejects a record with blank terms, and
  `RefuseContradictedLicence` rejects a copyleft binary whatever is declared. An older reader
  carries the unknown key through (the `font` record's key set is **open**, `parse.go:998`) and then
  refuses the document on the very terms the key exists to excuse. `SupportedMajor` goes 4 → 5 and
  `SupportedVersion` becomes `"5.0"`. Contrast story 4, where the key changed only what a *save*
  writes and no reader was harmed.
- **D2 — The trigger is the presence of the acknowledgement on a font record a chain names.** Not
  "blank terms", not "copyleft" — the flag itself. A document carrying it needs a reader that
  honours it, and that is what a version declares. A document with no acknowledged record keeps its
  version and its bytes.
- **D3 — One field, both jobs.** What the document records is what the engine reads. No separate
  origin discriminant beside it — two sources of truth about one face can disagree.
- **D4 — The acknowledgement excuses the three TERMS fields, and nothing else.** Blank
  `licence`, `licenceText` and `copyright` become legal on an acknowledged record, at both the
  command gate and the load gate. `family`, `style` and `source` stay required and non-blank at the
  command door: they are identity, not terms, and the acknowledgement says nothing about them.
- **D5 — `SupportedMajor` moves exactly once.** `spec-loop-section` opens the same `5.0`. Whichever
  lands first makes the edit; the other finds it done. Add a rank rather than renumbering
  (D-7.7.2), and **add** a trigger to the `5.0` ladder row rather than rewriting it.

## Boundaries & Constraints

**Always:**
- Honoured at **every** door, or the feature is a trap: `embedFontFamily`
  (`component_commands.go:4689`), `embedFontCut` (`:4858`), and the load path's two arms —
  `decodeFontChainEntry`'s `{"asset": …}` arm (`parse.go:609`) and each style-variant sibling
  (`:684`). Honour it at the embed doors only and a document saves and will not reopen; at the load
  doors only and the author cannot create one.
- `refuseLicenceSignatures` and its GPL/SSPL/ShareAlike patterns, the admit table, and the
  silence-admits rule on `RefuseContradictedLicence` are **unchanged**. This adds a condition on
  whether the question is asked about a given face — it does not edit the answer.
- **The catalogue tier never writes one.** A catalogue face carrying an acknowledgement is a defect,
  and worth a test that says so: it is the one thing that would quietly collapse the distinction
  between a face Folio distributes and a face the author supplied.
- The limit is written **where the field is defined**: this flag asserts and does not prove. The
  file cannot say who made the acknowledgement or whether it is true, and anyone hand-writing a
  `.folio` can set it. That is accepted deliberately, and a reader of the format must meet it at the
  field rather than deduce it.

**Never:**
- No change to `refuseLicenceSignatures`, the licence allowlist, or `src/font-licence.ts`.
- Do not relax `family`, `style` or `source` (D4).
- Do not renumber existing version ranks, and do not rewrite `spec-loop-section`'s `5.0` row.
- No designer UI work: `embedFontFamilyCommand` has no non-test caller yet, and wiring embedding is
  not this story.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Acknowledged, blank terms | Record acknowledged, three terms fields blank | Embeds and loads | N/A |
| Acknowledged, copyleft binary | Name table names GPL, record acknowledged | Embeds and loads | N/A |
| Unacknowledged, blank terms | No acknowledgement, blank terms | Refused, message unchanged | Load/command error |
| Unacknowledged, copyleft | No acknowledgement, GPL binary | Refused, message unchanged | Load/command error |
| Variant cut | An acknowledged record reached via `bold`/`italic`/`boldItalic` | Same admission at the variant arm | N/A |
| Identity still required | Acknowledged record with blank `family`, `style` or `source` | Still refused at the command door | Command error |
| Version trigger | Document carries an acknowledged record a chain names | Declares `5.0` | N/A |
| No trigger | Document with no acknowledged record | Version and bytes unchanged | N/A |
| Older reader | A `4.x` reader meets the document | Refuses it — which is why this is MAJOR | Load error |
| Catalogue face | A catalogue-tier record | Never carries an acknowledgement | Defect if it does |

</frozen-after-approval>

## Code Map

- **The four doors.** `folio-go/component_commands.go:4689` (`embedFontFamily`) and `:4858`
  (`embedFontCut`) both call `fontset.RefuseContradictedLicence(name, record.Licence.Value, decoded)`
  and fail with `componentFailure("", fontChainPath(name), …)` before anything reaches
  `t.doc.Assets`. `folio-go/internal/template/parse.go:609` (`decodeFontChainEntry`'s `{"asset"}`
  arm) and `:684` (each style-variant sibling) call `requireEmbeddedFaceLicence`.
- `folio-go/internal/fontset/licencesignature.go:305` `RefuseContradictedLicence`, with the
  reasoning at `:255-304` — read it before changing anything; the silence-admits rule and the
  refuse-half's "consults every face regardless" both survive this change.
- **The two blank-field gates, which differ.** Command: `component_commands.go:4972-4974`,
  `strings.TrimSpace(value) == ""` over the **six** fields listed at `:4955-4960` (family, style,
  licence, licenceText, copyright, source). Load: `parse.go:782-805` `requireEmbeddedFaceLicence`,
  over **licence/licenceText/copyright only**, error field `assets.<key>.font.<name>`. D4 relaxes
  the three terms fields on both, and nothing else.
- **The record.** Struct `internal/template/model.go:726-752` (six `Presence[string]` fields plus
  `Extra`); parse `parse.go:958-1003` `decodeAssetFont` with its alphabetical key table at
  `:971-983`; serialize `internal/template/serialize.go:347-374` `writeFontRecord`, called at
  `:736`. **Key set is OPEN** — `parse.go:998` `extraFields(obj, inner)` — deliberately, and
  contrasted with `FontChainEntry` at `model.go:747-751`. The **command** door is closed by arity:
  `componentFields(raw, 12)`, mirrored in `folio-designer/src/font-chain-command.ts:127-150`.
- **Version.** `internal/template/version.go:116-117` the two consts; version-string consts at
  `:155-184`; rank enum at `:429-439` (**append**, D-7.7.2); `versionForRank` at `:450-460`; the
  probe belongs beside the document-level `fontsRequireMajor(d.Fonts)` at `:352`, since `assets`
  and `fonts` hang off no element. Ladder table `docs/folio-format.md:113-127`, with the
  `embedFonts` no-row note at `:136` left alone.
- **Designer.** `folio-designer/src/font-chain-command.ts:127` `embedFontFamilyCommand` (fields at
  `:139-145`) and `:199` `embedFontCutCommand` — arity 12 today, mirrored from Go. Values come from
  `src/font-import.ts:193-196` (name IDs 0/13/14, may be `''`) for imported faces and
  `font-source.ts:711,810` for catalogue ones. The author-supplied marker is
  `font-import.ts:115-116` `authorSuppliedFaceSource(today)`, applied only by `acknowledgedFace` at
  `:129-130`.

**Tests that must change** (they pin the refusals this story conditions):
`internal/fontset/licencesignature_test.go:493,:248,:282,:361,:557,:662`;
`component_commands_test.go:2267,:2420-2448,:2761,:4578-4592,:4594-4600`;
`internal/template/fonts_embedded_test.go:1243,:88`; `font_chain_variants_test.go:515`.
**Version pins:** `internal/template/version_test.go:19,30,59,124,146,181,202`;
`folio-go/template_test.go:106-111`; `pages_command_test.go:88`; `section_break_test.go:519-530`;
`line_spacing_test.go:702`; `matrix_test.go:2100`; and the `"version": "4.1"` fixture templates in
`multi_page_flow_template.go`, `multi_page_statement_template.go`,
`section_break_statement_template.go`, `section_break_unanchored_template.go`.

## Tasks & Acceptance

**Execution:**
- [x] `internal/template/model.go`, `parse.go`, `serialize.go` -- the field on the font record, its
      decode and its emission; omitted when unset so no existing document's bytes move.
- [x] `internal/template/parse.go` -- `requireEmbeddedFaceLicence` admits blank terms on an
      acknowledged record (D4), at both `:609` and `:684`.
- [x] `internal/template/parse.go` (or its caller) -- skip the contradiction question for an
      acknowledged record on the load path.
- [x] `folio-go/component_commands.go` -- both embed doors honour it; the blank-field gate relaxes
      only the three terms fields; command arity 12 → 13.
- [x] `internal/template/version.go` -- `SupportedMajor` 4 → 5, `SupportedVersion` `"5.0"`, a new
      appended rank, its `versionForRank` row, and the probe beside `fontsRequireMajor`.
- [x] `folio-designer/src/font-chain-command.ts` -- mirror the new field and the new arity.
- [x] `docs/folio-format.md` + `.html` twin (and the `_bmad-output/specs/spec-folio` copy) -- the
      record's new key, the `5.0` ladder row, and the field's own "asserts but does not prove" note.
- [x] tests -- one per I/O Matrix row, plus a test that a catalogue-tier record never carries one.

**Acceptance Criteria:**
- Given a font record carrying the acknowledgement with blank `licence`, `licenceText` and
  `copyright`, when it is embedded by command and the document reloaded, then both succeed.
- Given an acknowledged record whose binary's name table names GPL, then it embeds and loads; given
  the same record without the acknowledgement, then both doors refuse it with today's message
  unchanged.
- Given an acknowledged record with a blank `family`, `style` or `source`, then the command still
  refuses it.
- Given a document carrying an acknowledged record a chain names, then it declares `5.0`; given a
  document with none, then its version and bytes are unchanged.
- Given `go test -count=1 ./...` in `folio-go`, then it passes; every golden digest for a document
  with no acknowledged record is unmoved. (`internal/text`'s `P6g` is pre-existing and red at
  baseline.)
- Given `npm test` in `folio-designer`, then it passes with the mirrored arity.

## Implementation Notes

**The key is `authorAcknowledged`, a boolean on `assets[k].font`.** Three-valued like every other
key on that record: only a present, non-null `true` acknowledges, and `FontRecord.Acknowledged()` /
`Asset.FaceAcknowledged()` are the one spelling every door asks through — the command door, the load
door and the version probe — so the three cannot drift.

**Task 3 was a no-op, and the story's own Code Map is why.** There IS no contradiction question on
the load path: `RefuseContradictedLicence` has exactly one call site pair, both in
`component_commands.go`, and `parse.go`'s own long comment (the "ONE-DOOR ASYMMETRY IS DELIBERATE,
D-16.R.11" block) records the second door as deliberately deferred past `folio-go/v1.0.0`. So
"skip the contradiction question on load" had nothing to skip. Recorded rather than silently
dropped, on A-25's rule.

**`false` is on the wire and off the record.** `componentFields` is an exact count, so the arity
moved (`embedFontFamily` 12 → 13, `embedFontCut` 13 → 14) and a catalogue pick sends
`"authorAcknowledged": false`. Go records the field only when it is `true`, which is what keeps every
existing document's bytes unmoved and what makes "a catalogue face never carries one" structural
rather than a convention. A key the designer sometimes omitted would have been an arity refusal on
whichever path was not under test.

**The blank-field relaxation needed a second reader, not a widened one.** `commandString` refuses
`""` and thirty other commands share it, so `commandFontRecordString(raw, key, blankLegal)` sits
beside it: the key is still required and still must be a string, and only the emptiness check is
conditional.

**The version probe reaches variant siblings through `EmbeddedAssetKeys()`.** A chain whose regular
is a catalogue face and whose bold is the author's own acknowledged cut requires `5.0` — the load
door is asked about that sibling too, so a probe that looked only at the base would stamp a version
that lies. Asserted directly (`TestAnAcknowledgedVariantCutAlsoDeclaresFiveZero`).

**No version pin outside `version_test.go` moved.** The blast-radius list in the Code Map named
seven files of version pins; `SupportedMajor` is a ceiling and every fixture still declares the
lowest version its own content requires, so only `TestHigherMajorIsLoadError` needed editing (its
"one major above the ceiling" fixture went `5.0` → `6.0`). A-25's lesson, a second time.

**The copyleft fixture is built, not committed.** `faceDeclaringGPL` rewrites the committed
Roboto's `name` table in memory — a minimal restatement of
`internal/fontset/licencesignature_test.go`'s technique, kept to the one Windows en-US record this
needs, because those helpers are in a test package that cannot be imported. A precondition test
(`TestTheCopyleftFixtureReallySaysSo`) asserts the rewrite really lands, so the "acknowledged
copyleft admits" row cannot pass by admitting NO EVIDENCE instead.

**Designer: mirror only.** `font-chain-command.ts` gained the field and both arities; the three
existing call sites in `App.tsx` pass `false`, which is correct for the catalogue tier they serve.
⚠ **The import→embed wiring is NOT done, and is a real gap for story 6:** story 3 writes imported
faces into the same `font-store` as catalogue ones, and `StoredFace` carries no acknowledgement
field — so an author-supplied face embedded through `dispatchEmbed` or the cut path today would send
`false` and be refused exactly as before. This story's Boundaries forbid that wiring ("no designer UI
work"; "wiring embedding is not this story"), so it is left, named, for the story that owns it.

**Review round 1 added two rules the plan had not named.**

*An embedded face's RECORD is as frozen as its bytes.* An assets key is the content hash and both
embed doors write "only if absent", so byte-identical faces arriving with and without an
acknowledgement would have silently kept whichever record landed first — putting an acknowledgement
nobody made on a catalogue face, or losing the author's own and with it the `5.0` the document needs
to reopen. `refuseAcknowledgementMismatch` refuses the disagreement at both doors, on the same freeze
rule the cut door already applies to different bytes. Agreement is still a no-op.

*Null is neither `false` nor the empty string.* `json.Unmarshal` of `null` into a Go string or bool
is a no-op that returns no error, so `commandBool` read `"authorAcknowledged": null` as `false` and
the blank-legal string reader recorded `"licence": null` as a present empty string — collapsing two
of the three states the loader keeps distinct. Both now refuse `null` explicitly. The `commandBool`
fix is repo-wide and strictly a tightening: `snap`, `pageBreak`, `anchor` and `embedFonts` never
admitted `null` as a value either.

## Spec Change Log

## Review Triage Log

Pass 1 — three layers.

| # | Finding | Verdict | Evidence |
|---|---|---|---|
| 1 | The acknowledgement cannot be produced by any designer path: every call site hardcodes `authorAcknowledged: false` and `StoredFace` carries no such field, so an imported face with blank terms is still refused. The engine honours an assertion the product cannot make. | **high — deferred to story 6** | Pre-verified by the verification-gap layer, which traced the whole path. This story's Boundaries explicitly fence designer wiring out, so it is not a defect here — but story 6 must carry it, and must land the pinning test. Recorded in `DECISIONS.md` A-30. |
| 2 | The deferred second contradiction door — `parse.go:750-781`'s "ONE-DOOR ASYMMETRY IS DELIBERATE … close this second door before any refuse-signature is added after v1.0.0" — was not amended. When that door is opened it must skip acknowledged faces, or every acknowledged copyleft face saved today becomes unopenable. | **medium** | Confirmed. The story's own Implementation Notes record it; the file a future implementer will actually read does not. |
| 3 | An asset key already in `t.doc.Assets` under a differently-acknowledged record is not reconciled, so a catalogue pick can silently inherit an acknowledgement, or an acknowledgement can be silently dropped. | **medium** | Confirmed at both embed doors. Two picks of byte-identical faces, one acknowledged, is the reachable case. |
| 4 | Nothing tests that the acknowledgement excuses **only** the three terms fields at the engine. An acknowledged record that is a variable font, whose bytes contradict its `mediaType`, or which exceeds the size bound should still be refused — those are the refusals a maintainer might later move inside the acknowledged branch. | **medium** | Confirmed: the only negative coverage is blank `family`/`style`/`source`. |
| 5 | `commandFontRecordString` duplicates `commandString`'s body rather than composing with it, so a future tightening of `commandString` will silently not apply to an acknowledged record's terms fields. | **medium** | Confirmed. Exactly the drift this file's comments warn about elsewhere. |
| 6 | The two doors disagree about `null` in a terms field: `json.Unmarshal` flattens `null` into `""` with no error at the command door, while the record is deliberately three-valued everywhere else and the load path keeps `null` distinct. `commandBool` has the same shape for the flag itself. | **medium** | Confirmed. Low impact — the resulting record is legal — but it is the one place a wire `null` is quietly flattened. |
| 7 | The `true` path of the new wire field is never exercised in the designer: every call site and every test case passes `false`, so a builder encoding it as `"true"` would be caught only by Go. | **medium** | Confirmed across `font-chain-command.test.ts`. |
| 8 | `authorAcknowledged: false` is a bare literal at three `App.tsx` call sites with no code-level marker of the gap, beside a `font-import.ts` that already knows which faces are acknowledged — the "two sources of truth about one face" D3 forbids. | **medium** | Confirmed. The gap is written only in the story file. |
| 9 | The copyleft fixture erases the face's identity records — `nameTableWithLicenceStatement` builds a one-record table, dropping name IDs 0/1/2/14 — so "an acknowledged copyleft face embeds" is proven with a binary that names nothing but a GPL statement. And `TestTheCopyleftFixtureReallySaysSo` greps for `"GPL"` in a message whose asserted wording elsewhere is `"copyleft or share-alike"`, so the precondition rests on an incidental substring. | **medium** | Both confirmed. The evidence does not match the claim it supports. |
| 10 | Three stale arity comments: `App.tsx:2606` now reads "THE COMMAND IS UNTOUCHED, AT THIRTEEN FIELDS" (asserting it was not changed while recording this story's change to it); `font-chain-command.test.ts:16` still says "the twelve fields"; the module header still says "Both arities are exactly what they were". | **low** | Confirmed. Arity is explicitly "part of the contract" in that file. |
| 11 | `TestAnAcknowledgedAssetNoChainNamesRaisesNothing` asserts only `!= "5.0"`, where its sibling asserts the exact version — a probe bug returning a different wrong rank would pass. | **low** | Confirmed. |
| 12 | `fontsRequireAcknowledgedFace` sorts chain names to compute a boolean any-match that cannot depend on order. | **low** | Confirmed. `fontsRequireMajor`'s shape copied without asking whether the sort earned its place. |
| 13 | Doc gaps: a non-boolean `authorAcknowledged` is a load error but the new section never says so; the ladder row reads as if only a base entry triggers `5.0` when a style-variant sibling does too; the `_bmad-output` copy drops the "a reader must not treat this key as evidence" sentence; and nothing says how the acknowledgement interacts with story 4's `embedFonts`. | **medium** | All four confirmed across both copies. |
| 14 | An acknowledged record declaring an SPDX id its own binary contradicts is refused by nothing, because skipping `RefuseContradictedLicence` skips its ADMIT half as well as its REFUSE half. | **low — accepted** | Confirmed. Accepted deliberately: the owner's ruling is that Folio takes no position on the terms of an author-supplied face, and "the declared id does not match the binary" is a position. Recorded in `DECISIONS.md` A-29. |

**Routing.** No `bad_spec` and no `intent_gap`. #2–#13 route to **patch**; #1 is deferred to story 6
with its pinning test; #14 is accepted with its reasoning recorded.

## Design Notes

D1 is worth stating beside story 4's opposite answer, because the two look contradictory and are
not. Story 4's key changed what a **save** writes; an older reader ignored it, rendered the same
page set, and preserved it — so it required no version at all. This key changes what a reader must
**accept**: an older reader carries it through (the record's key set is open) and then refuses the
document on exactly the terms the key exists to excuse. Same format, same passthrough machinery,
opposite answer — because the ladder asks what the content *requires of a reader*, not how new the
key is.

## Verification

**Commands:**
- `cd folio-go && go vet ./... && go test -count=1 ./...` -- expected: pass except the pre-existing
  `internal/text` `P6g` corpus floor.
- `cd folio-go && go test -count=1 -run 'TestTargetRenderHash' -tags=matrix .` -- expected: pass.
- `cd folio-designer && npm test` -- expected: pass.
- `cd folio-designer && ./node_modules/.bin/oxlint && ./node_modules/.bin/tsc -p tsconfig.app.json
  --noEmit` -- expected: both exit 0. Run the binaries directly.
