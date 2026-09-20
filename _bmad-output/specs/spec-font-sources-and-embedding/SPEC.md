---
id: SPEC-font-sources-and-embedding
companions:
  - ../spec-fonts/SPEC.md
  - ../spec-install-all-face-cuts/SPEC.md
  - ../spec-loop-section/SPEC.md
  - ../../../docs/folio-format.md
sources: []
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# A third font source: the author's own disk, and a document that may decline to carry it

## Why

**A pain to solve, and it belongs to the author with a brand typeface.** A face reaches a Folio
document by exactly two routes today: it is one of the eleven the engine ships and embeds at build
time ([fonts.go](../../../folio-go/fonts/fonts.go)), or it is fetched from the catalogue tier and
embedded into the `.folio` itself. An author whose company licenses a typeface that is in neither
place — the common case for a brand, and the entire case for a commercial foundry face — has no
route at all. `spec-fonts` closed that door deliberately (D-8.6.1) on the ground that a file off
the author's disk arrives with no licence record and a `2.0` document requires one. **The owner has
now reversed the premise rather than the mechanism:** having the right licence for a font the
author loads is the author's responsibility, not this product's. With that gone, the decline has
nothing left holding it up.

**The second half is weight.** Embedding is the right default for a file that travels alone, and it
is the wrong default for an organisation that renders a thousand statements a day on servers it
controls, where the same face rides inside every document for no reason. So the document gains an
option — on the main template's page setup, beside locale and margins — saying whether it carries
its faces or merely names them. A named face resolves from whatever the rendering host supplies,
and a host may now build that supply from a directory on disk instead of the shipped set alone.

**The third half is what happens when that fails**, and it is the part that costs something. Today a
chain naming a face the renderer was not given refuses the render outright (`TEXT_FACE_ABSENT`) —
`spec-fonts` CAP-4 exists to guarantee no page set is ever produced with a substituted face. Once a
document may legitimately name a face it does not carry, that refusal turns a deployment gap into a
dead pipeline. The owner's call is to **paint it in a shipped face and say so**: a statement that
went out in Noto Sans instead of the brand face is recoverable; a nightly run that produced nothing
is not. The guarantee that survives is not *no substitution* — it is *no silent substitution*.

## Capabilities

- **CAP-1 — A face from the author's own machine**
  - **intent:** The author picks a font file off their own machine and that face becomes usable on a
    chain, exactly as a catalogue face is — reachable from the same control, listed in the same
    browser, applied to elements the same way.
  - **success:** With the catalogue unreachable, the author imports a `.ttf`/`.otf` from disk,
    applies it to a text element, and the designer previews in that face. The face is
    indistinguishable from a catalogue face at every point downstream of the import: same store,
    same family browser entry, same chain entry, same save/open round trip. Picking a family's
    four cut files in one gesture yields **one family of four cuts**, grouped by the binaries' own
    name records, and pressing **B** paints that family's bold.

- **CAP-2 — The document says whether it carries its faces**
  - **intent:** The main template's page setup carries an embed-or-not option, so a document that
    travels alone carries its faces and a document rendered on controlled infrastructure does not.
  - **success:** Toggling the option and saving changes only whether the referenced faces appear in
    `assets` and whether chain entries take the asset or the name shape; the page set the designer
    previews is unchanged either way. The setting travels as a document-settings command alongside
    `setDocumentLocale`/`setDocumentUTCOffset`, survives a save/open round trip, and is undoable.
    Turning the option **off** on a document that carries faces strips them on the next save, and
    the author is warned before the first strip.

- **CAP-3 — A rendering host may supply faces from disk**
  - **intent:** The integrator points the renderer at a directory of font files, and faces found
    there resolve chain entries by name the same way the shipped set does.
  - **success:** A document naming a face it does not carry renders correctly — identically to the
    same document with that face embedded — on a host given a directory containing it, with no
    change to the document and no diagnostic. The same call with an empty or absent directory
    exercises CAP-4 instead.

- **CAP-4 — A substitution that is always on the record**
  - **intent:** A chain entry naming a face that is neither carried nor supplied is painted in
    another face the renderer *does* hold, and reported, instead of refusing the render.
  - **success:** Rendering a document that names an unavailable face, on a renderer holding other
    faces, produces a complete page set plus a diagnostic naming the chain, the entry index, the
    face requested and the face painted. No render path produces a substituted face without that
    diagnostic, and the diagnostic is machine-readable — an integrator can fail their own build on
    it. A Thai document whose brand face is missing, rendered against the shipped eleven, renders
    **in Thai**, not in tofu. A renderer holding **nothing** to substitute from still refuses, with
    `TEXT_FACE_ABSENT` unchanged.

- **CAP-7 — A document that carries its faces needs no font set**
  - **intent:** A template whose every chain entry resolves to an embedded asset renders without
    the caller supplying any faces at all, because there is nothing for a font set to contribute.
  - **success:** Rendering such a document with an empty or absent font set succeeds and produces
    the same bytes as rendering it with any font set whatsoever. No argument check refuses the call
    before resolution has been attempted, and an entry that genuinely cannot be resolved still
    fails as `TEXT_FACE_ABSENT`. Fixes what
    [issue #1](https://github.com/panitw/folio8/issues/1) reports — though **the issue stays open
    until `folio-dotnet` publishes**; see Constraints.

- **CAP-5 — An author-supplied face states what its binary states**
  - **intent:** An embedded author-supplied face carries the licence identity the font program
    itself declares, copied verbatim, so a travelling document still reports terms without Folio
    asserting any.
  - **success:** Importing a face and embedding it writes `copyright` from the binary's name ID 0,
    `licenceText` from name ID 13 and `licence` from name ID 14, byte-for-byte as the binary holds
    them, with an absent name record producing an empty value and a loadable document. No import is
    ever refused, delayed, or gated on what those records say.

- **CAP-6 — An acknowledgement the author makes, and the document carries**
  - **intent:** Importing a font file asks the author to accept that they hold the right to use
    and redistribute it; accepting is what admits the face, and the document records that the
    assertion was made so it travels with the file.
  - **success:** No face reaches the store without the author accepting the dialog, and declining
    imports nothing. A document carrying an acknowledged face **loads and embeds without the
    licence-contradiction refusal**, including a face whose own binary names a copyleft licence;
    the same face with no acknowledgement on its record is still refused, at both doors, with the
    message unchanged. A catalogue face never carries one.

## Constraints

- **The engine never reads the filesystem.** `folio8.FontSet` stays the engine's only font input
  ([fontset.go](../../../folio-go/fontset.go)). The disk source is a **host-side** helper that
  builds a `FontSet` from a directory the integrator names; the engine's own change is the
  resolution mode, not an I/O path. This is what keeps `spec-fonts`' render-time purity intact
  where it can be: the engine still never goes looking for fonts on the machine it runs on.
- **No filesystem path is ever written into a `.folio`.** A non-embedded face is recorded as the
  chain entry's existing **string** shape — a face name — which already means *"a face the renderer
  is given at render time"* ([folio-format.md](../../../docs/folio-format.md)). A path would make
  the document machine-specific and would end FR12/S9: a template a person or an agent can edit
  without the designer, and a hand-written template that renders.
- **A substitution is never silent.** Every fallback carries a diagnostic naming the chain, the
  entry, the face requested and the face painted. A render that substitutes and says nothing is a
  defect, not a configuration.
- **The substitute is coverage-resolved, per rune, among the faces the renderer WAS GIVEN** — the
  supplied font set plus the document's own embedded assets. Not a fixed face and not the chain's
  own first entry: a fallback that cannot draw the script it replaced has substituted nothing, it
  has produced tofu. **It is not "the shipped set", and the distinction is load-bearing:** package
  `folio8` never imports `folio-go/fonts` — `fonts.go` calls that one-directional import
  load-bearing, since reversing it would drag ~14.8 MB of faces into every consumer — so the
  **engine owns no faces** and can only paint with what the caller handed it. A host passing
  `fonts.Shipped()` gets the eleven; a host passing nothing gets nothing to substitute from.
- **The three states are one rule, not three checks.** An entry that resolves renders; an entry
  that cannot resolve **with candidates available** substitutes and warns; an entry that cannot
  resolve **with no candidate at all** is `TEXT_FACE_ABSENT`. No eager argument check may pre-empt
  that rule before resolution has been attempted — which is what
  [issue #1](https://github.com/panitw/folio8/issues/1) reports and
  [Folio8.cs:183](../../../folio-dotnet/src/Folio8/Folio8.cs#L183) currently does.
- **Issue #1 is not closed by merging the fix.** It stays open until `folio-dotnet`'s next
  release is published, because the reporter is on the 1.0.1 nuget package and is living with a
  zero-byte-face workaround until there is a build they can consume. No commit message, PR body or
  changelog entry for this work may use a GitHub closing keyword against it; it is closed by hand
  once `2.0.0` ships.
- **"No default font set" survives, narrowed.** The engine still performs no ambient lookup and
  still has no faces of its own; what changes is that *supplying none* stops being a caller error
  when the document needs none. The principle is stated in all three bindings and in
  `spec-client-libraries/api-surface.md`, so the wording there is part of the change, not a
  follow-up.
- **A disk family is not second-class.** An import groups its files into a family by the binaries'
  own name records (family, weight, slope), so an author-supplied family reaches the same
  four-cut chain entry a catalogue family does (`spec-install-all-face-cuts` CAP-1).
- **The Local Font Access API stays forbidden.** `host-font-access.test.ts`'s source scan is
  untouched — a file the author picks is not the font book enumerated.
- **The embedded mode is unchanged in every respect.** Byte identity across the four targets, dedupe
  by lowercase-hex SHA-256, faces stored whole, no save-time subsetting, PDF subsetting untouched.
  This work adds a second mode; it does not alter the first.
- **The catalogue tier keeps its licence gate.** The build-time allowlist (OFL-1.1, Apache-2.0, MIT,
  UFL) and its fail-the-build admission check (`spec-fonts` D-8.5.2/D-8.5.3) are untouched. The
  no-licence-gate ruling is scoped to faces the **author** supplies and does not travel to faces
  **Folio** distributes. **The catalogue tier never writes an acknowledgement**, and a catalogue
  face that arrives carrying one is a defect.
- **The acknowledgement is one field, doing both jobs.** What the document records is what the
  engine reads — there is no separate origin discriminant beside it. It is honoured at **both**
  doors that ask the licence question, `embedFontFamily`
  ([component_commands.go:4598](../../../folio-go/component_commands.go#L4598),
  [:4767](../../../folio-go/component_commands.go#L4767)) and document load
  ([parse.go:711](../../../folio-go/internal/template/parse.go#L711)); a guard honoured at one of
  them is a document that saves and will not reopen.
- **`refuseLicenceSignatures` is unchanged for every unacknowledged face.** The override is scoped
  to the acknowledgement, not to embedded faces in general. Nothing about the GPL/SSPL/ShareAlike
  patterns, the admit table, or the silence-admits rule moves.
- **The format additions ride `5.0`, shared with `spec-loop-section`.** The ceiling is
  `SupportedMajor = 4` ([version.go:116](../../../folio-go/internal/template/version.go#L116)) and
  the ladder tops at `4.1`; a new key on a font record is a MAJOR change by the format's own
  compatibility rules, and it **joins** the `5.0` that `spec-loop-section` opens rather than
  opening a `6.0`. Two consequences bind implementers. **`SupportedMajor` moves 4 → 5 exactly
  once** — whichever spec's story lands first makes that edit and the other finds it done; the
  `5.0` ladder row and `version.go` are a **shared edit surface** between the two specs. And
  **only a document that carries the construct declares `5.0`** — the same rule `spec-loop-section`
  states for the loop element and the `$.` path, and the same D-1.4.13 principle that makes
  version a property of the document. A document with no acknowledged font record keeps its
  version and its bytes.
- **Two version lines, and they are not the same number.** The **`.folio` format** goes `4.1` →
  `5.0` — what a *document* declares. The **client libraries** go `1.x` → **`2.0.0`**, cut for
  Go, Node and .NET **together** once this work and `spec-loop-section` land — what an *integrator*
  upgrades. A document never declares `2.0.0` and a library never declares `5.0`.
- **All three libraries ship the same behaviour at `2.0.0`.** A `.folio` that substitutes a face,
  or that carries an acknowledged font record, behaves identically through `folio-go`, `folio-js`
  and `folio-dotnet` — the diagnostic and the load outcome included. A capability that reaches only
  the Go surface is not done.
- **The acknowledgement asserts; it does not prove.** The file cannot say who made it or whether
  it is true, and anyone hand-writing a `.folio` can set it. That is accepted deliberately — it is
  the same position the product already takes on the author's responsibility — and it must be
  stated where it is written, not discovered by a reader of the format.
- **Licence fields are transcribed, never composed.** The designer copies the binary's own name
  records and performs no classification, no inference from a family name, and no defaulting to an
  identifier nobody read. An empty value is legal and means the binary said nothing.
- **Chain semantics are unchanged.** An ordered list resolved per rune for coverage; entries may mix
  shipped names, supplied names and embedded assets in any order.

## Non-goals

- **No enumeration of the machine's installed fonts.** The disk source is a directory an integrator
  names and an import is a file an author picks. Neither side reads the operating system's font
  book, and `spec-fonts`' *"No host fonts"* non-goal survives in exactly that narrowed form.
- **No licence verification, classification or admission for an author-supplied face.** No prompt,
  no checkbox, no blocklist, no warning banner. The responsibility is the author's and the product
  does not take a position on it.
- **No network involvement.** This spec adds no fetch, no font service and no URL. The catalogue
  tier's existing fetch behaviour is out of scope and unchanged.
- **No path, URL or machine identity in the document.** Including as an optional hint, a comment or
  a diagnostic aid.
- **No change to how the PDF producer subsets**, and no save-time subsetting in either mode.
- **No variable-font axes and no synthetic bold or oblique.** A disk import is a face, on the same
  terms as every other face.
- **No per-chain or per-face embed control.** The option is the document's, whole.

## Success signal

An author licenses their company's typeface, imports the file from their own disk, lays out the
statement, turns embedding **off** because these run on the company's own servers, and saves. The
developer drops the same font file into the service's font directory and the nightly run produces
the brand face, hash-identical to the designer's preview. Six months later a new server is built
without that directory: the run still completes, every statement legible in Noto Sans, and the log
names the chain, the entry and the face that was missing — so someone fixes the directory instead
of finding out from a customer.

## Assumptions

- **The engine is not the thing that reads disk.** The owner said *"not loaded from the disk on
  rendering"* without naming a layer; this spec places the read in a host-side helper and leaves
  only the lenient/strict resolution mode inside the engine. Derived from `FontSet` being the sole
  font input and from the standing rule that the engine never queries the host.
- **A non-embedded face is named, not pathed** — the chain entry's existing string shape carries it,
  so this half of the work needs no format change at all.
- **The disk source is an explicit directory**, not the OS font book.
- **Strict refusal remains reachable.** `TEXT_FACE_ABSENT` is not deleted; an integrator who wants
  today's behaviour must still be able to get it, since a substituted face in a legal document is a
  real failure for some consumers. How that is selected is the implementing story's design.

## Open Questions

- ~~**BLOCKER for the embed path:** `RefuseContradictedLicence` refuses any face whose own name
  table matches a copyleft or restricted signature, which would refuse an author-supplied face and
  contradict the ruling that Folio takes no position on the author's terms.~~
  **SETTLED 2026-09-20 by OWNER DECISION: the author's acknowledgement overrides the guard, and the
  acknowledgement is recorded on the font record.** Two things narrowed the question before it was
  answered, and both belong on the record. **First, the guard was smaller than first reported:**
  `refuseLicenceSignatures`
  ([licencesignature.go:114-121](../../../folio-go/internal/fontset/licencesignature.go#L114-L121))
  matches only the GPL family, SSPL and ShareAlike — a proprietary foundry face, which is the case
  this spec exists for, trips none of them, and a face that says nothing admits already. **Second,
  a dialog cannot reach the guard:** it is asked at two doors inside the engine, so an
  acknowledgement that overrides it has to travel **in the document**, which is why the record and
  the marker are the same field. What the owner chose, of the three ways out, is **(a)** — the
  override — rather than narrowing the ruling or permitting name-only. See CAP-6.
- ~~Does a new key on a font record move the `.folio` version?~~ **SETTLED 2026-09-20 by OWNER
  DECISION: yes — MAJOR, and it rides the `5.0` that `spec-loop-section` opens.** See the
  constraint above. What remains open is narrower: **which of this spec's constructs trigger the
  `5.0` row.** The font-record acknowledgement plainly does. The **embed-or-not document setting
  cuts the other way** and needs reading rather than assuming: it governs what a **save** writes,
  not what a render does, so an older reader that ignored it would render the document
  *identically* and would diverge only on re-save. That is not obviously D-7.3.1's
  refuse-rather-than-render-wrong case, and it turns on how the loader treats an unknown top-level
  document key. Story 4's question.
- Does the designer warn at **authoring** time that a non-embedded document will substitute on a
  host lacking the face, or is the substitution only discoverable at render?
- Does permitting an **empty** `licenceText` on a chain-named embedded face move the `.folio`
  version, given `2.0` currently requires the three fields present? (Present-but-empty may satisfy
  the existing rule as written; this needs reading against the loader, not assuming.)
- Is an author-supplied **CJK** face refused, warned about, or embedded at full weight? Inherited
  unresolved from `spec-fonts`, and this spec makes it reachable by a new route.
- `spec-fonts` CAP-3's success criterion and `spec-fonts` CAP-4 both need amending for the reversals
  recorded here; the amendments are an action item, not a silent consequence.
