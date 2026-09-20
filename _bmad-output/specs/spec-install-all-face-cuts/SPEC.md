---
id: SPEC-install-all-face-cuts
companions:
  - ../spec-fonts/SPEC.md
  - ../../../docs/folio-format.md
sources: []
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# Install a family's four cuts, not only its Regular

## Why

**A pain to solve, and it lands on the author mid-sentence.** An author installs Sarabun from the
Add font dialog, types a heading, presses **B**, and is told *"No bold face in this family — the
engine paints the regular face and warns."* Nothing failed. The install did exactly what it was
built to do: `regularFilename()` narrows a family to its weight-400 upright
([font-source.ts:196-198](../../../folio-designer/src/font-source.ts#L196-L198)), the family is
refused outright if it has none ([:449-453](../../../folio-designer/src/font-source.ts#L449-L453)),
and the one fetched face is stamped `style: 'Regular'`
([:532](../../../folio-designer/src/font-source.ts#L532)). The code says so in its own comments —
*"a pick embeds ONE face"*
([App.tsx:2530-2534](../../../folio-designer/src/App.tsx#L2530-L2534),
[component_commands.go:4412-4418](../../../folio-go/component_commands.go#L4412-L4418)).

Three things make this worth closing now rather than living with. The **engine already honours
cuts**: `chainFaceNames` splits a chain into base and styled faces and paints the variant when it
covers the rune ([render.go:1360-1382](../../../folio-go/render.go#L1360-L1382),
[:2091-2098](../../../folio-go/render.go#L2091-L2098)) — this is live, not stored-and-unconsumed.
The **data is already in hand**: the `METADATA.pb` parse carries every static face with its weight
and style ([font-source.ts:116-121](../../../folio-designer/src/font-source.ts#L116-L121),
[:138-187](../../../folio-designer/src/font-source.ts#L138-L187)), and only the narrowing throws
the other three away. And a **working multi-cut model already ships** — Roboto and the Noto
families reach a four-cut chain entry through
[shipped-face-cuts.ts:66-141](../../../folio-designer/src/shipped-face-cuts.ts#L66-L141). So the
gap is not capability. It is that the install path stops one step short of the shape everything
downstream is already built to read. This answers SPEC-fonts' standing open question, *"Do bold and
italic get realized in this scope?"* — that question is still open and un-amended in
`spec-fonts/SPEC.md`, and writing the answer back to it is an action item for when this work lands.

**The shape this takes is two-sided, and the split is what keeps documents lean.** Installing a
family fills the **designer's local face store** with every cut that family has. The **document**
gains a face only when the author actually uses it. So a document that never bolds still carries
one face, and the four-times-the-bytes cost falls only on documents that earn it.

## Capabilities

- **CAP-1 — A pick installs the family's cuts, and use embeds them**
  - **intent:** Picking a family from the Add font dialog puts every cut that family publishes into
    the designer's local face store, and the author's first use of a cut embeds that face in the
    document and declares it on the chain entry.
  - **success:** Install Sarabun on a blank document, apply it to a text element, press **B** — the
    text paints in Sarabun Bold with no diagnostic. Save without ever pressing **B** and the
    document carries one font asset; press **B** first and it carries two, the entry naming both.
    Either reopens to the same result.

- **CAP-2 — Truthful absence**
  - **intent:** The cut-absence sentence appears only when the cut is genuinely unavailable, and a
    family publishing fewer than four cuts installs the ones it has.
  - **success:** A family with all four cuts never produces any of the three sentences — including
    before the bold is embedded, since the store holds it. A family with no upstream italic still
    declares `bold`, and pressing **I** still produces *"No italic face in this family"* — the
    sentence now reports the family upstream rather than the installer.

- **CAP-3 — Both tiers behave the same**
  - **intent:** The committed catalogue families carry their cuts too — committed from upstream,
    since they cannot be fetched — so a family behaves identically whichever tier it came from.
  - **success:** For every family the dialog offers, whether it resolves from the committed tier or
    is fetched from upstream, the cuts available are the cuts that family publishes. No offline
    author loses bold that an online author would get.

- **CAP-4 — Existing documents complete themselves**
  - **intent:** Opening a document or template whose families are missing cuts offers to fetch
    them, and on acceptance fills the local store in the background while the author keeps working.
  - **success:** Open the existing Sarabun document: it opens immediately and is editable
    throughout; the author is asked whether to complete the missing cuts; on acceptance the status
    bar reports progress and then the outcome, and afterwards **B** paints. The document itself is
    untouched throughout — not modified, not marked changed, nothing added to undo. Offline or on
    decline, the document is identical and the existing warning stands.

- **CAP-5 — An installed family stays one entry in the dialog**
  - **intent:** A family the author has installed appears once in the Add font dialog however many
    of its faces are held, and each cut the author asks for is painted by that family's matching
    face rather than an arbitrary one.
  - **success:** With four faces of one family in the store, the browser lists that family once;
    each cut resolves to the face whose recorded `style` matches, never to whichever face sorted
    first; and the result is stable across reloads and across faces fetched on the same day.

- **CAP-6 — Cuts are visible before the pick**
  - **intent:** The author can see which cuts a family has before choosing it, so a Regular-only
    family is recognisable at pick time rather than at the moment they press **B**.
  - **success:** Every family the dialog lists shows its available cuts alongside it, and the cuts
    shown match what installing that family actually yields.

## Constraints

- **The `.folio` format does not change.** Four assets per entry, declared through the existing
  closed `bold`/`italic`/`boldItalic` keys
  ([docs/folio-format.md:240-275](../../../docs/folio-format.md#L240-L275)). No new key, no version
  bump beyond the `2.0` an asset entry already declares. The closed set is what bounds this work to
  four cuts: a fifth weight has nowhere to be written.
- **The store and the document are separate, and only the document is a contract.** Installing and
  completing write to the designer's local face store. Embedding into the document happens on use.
  A chain entry declares a cut only when that face is embedded, so the format's existing rule —
  a named asset must be present or the load is a located error — holds unchanged.
- **Completion never mutates the document.** No edit, no dirty flag, no undo entry, no save. An
  author who declines, or who is offline, has a document byte-for-byte as it was.
- **An embedded face is never replaced.** The local store refreshes from upstream freely; a face
  already in a document is frozen at the vintage it was embedded. A cut embedded later is taken at
  the embedded Regular's vintage where that is available. This is what keeps a no-op open/save
  round trip byte-identical. SPEC-fonts states four separate byte-identity rules; the freeze rule is
  derived from them and is first stated here. An asset key is the
  SHA-256 of the face bytes, so a swapped vintage is a changed document.
- **The upright Regular stays the base and stays required.** A family with no weight-400 normal is
  still refused before any byte is embedded. The other three cuts are additions to that base, never
  substitutes for it.
- **Every embedded face carries its own licence record.** A variant asset key already clears the
  same bar as the entry's own asset
  ([docs/folio-format.md:960-961](../../../docs/folio-format.md#L960-L961)). Lazy embedding means a
  document carries as many records as it carries faces, not four per family.
- **No synthetic bold or oblique, and no variable axes.** A cut is a real static face or it is
  absent; absence keeps its existing `TEXT_STYLE_FACE_UNDECLARED` warning and base-face paint
  ([diag.go:145](../../../folio-go/internal/diag/diag.go#L145)).
- **A committed family's cuts are committed from upstream, never fetched by the designer.** 30 of
  the 31 committed families are unreachable from the web tier — 28 carry variable axes on the
  `google/fonts` mirror and `addableFromTheWeb` filters variable families out
  ([font-index.ts:149](../../../folio-designer/src/font-index.ts#L149)), and Inter Display and
  Source Serif 4 Display are absent from the index entirely. The committed tier exists because a
  family variable-only **on that mirror** often publishes ordinary statics from **its own project**,
  and those are what this repository commits. Every one of the 31 Regulars is an upstream file
  copied byte for byte — each `NOTICE.md` records *"copied unmodified, no derivation"*, and the
  arimo one states *"This repository cannot derive a face at all"*. `tools/fontgen/instance_faces.py`
  drives a hardcoded list of seven **engine** faces, all Noto, writing to `folio-go/fonts/`; it has
  never produced a catalogue face. The cuts follow the same route the Regulars took, and licence
  admission stays on the build gate rather than moving into the browser.

- **Each fetched cut is verified on its own terms.** The variable-font refusal
  ([font-source.ts:513-515](../../../folio-designer/src/font-source.ts#L513-L515)) and the
  `.ttf`/`.otf` media-type check
  ([:206-211](../../../folio-designer/src/font-source.ts#L206-L211)) apply per face, so one bad cut
  is refused without poisoning the family.
- **The offline release asset budget rises deliberately, and stays pinned.** `maximumCacheAssets`
  moves from 90 to a measured number covering 31 families at up to four cuts, with the rationale
  written beside the `const` line in the shape
  [release-payload.ts:65-91](../../../folio-designer/src/release-payload.ts#L65-L91) requires —
  the derivation reader is line-anchored and a reformatted line fails the build.
- **The blocking core tier does not grow.** It is pinned at exactly 30 assets on both ends
  ([release-payload.ts:150-151](../../../folio-designer/src/release-payload.ts#L150-L151)), and
  catalogue faces are classified `deferred`
  ([offline-release-contract.mjs:126](../../../folio-designer/scripts/offline-release-contract.mjs#L126)),
  so these cuts cost nothing at app startup and must keep costing nothing.
- **Opening stays non-blocking.** Cuts are fetched eagerly as a document opens, so a document
  naming three catalogue families fetches more than it does today. The open must not wait on any of
  it, and the status bar must make the work visible rather than leaving the author guessing.

## Non-goals

- **No weight beyond the four cuts.** Light, Medium, SemiBold, Black and their italics are not
  installed, not offered, and not declarable. Declined against the format's closed variant set.
- **No variable fonts.** Variable-only families stay filtered out of the dialog exactly as today.
- **No synthetic styling of any kind**, and no walking down the chain to find a bold elsewhere.
- **No change to the chain mechanism, face naming, or the `fonts` map syntax.**
- **No host fonts and no font upload.** Unchanged from SPEC-fonts; a face arrives from the
  committed tier or from upstream, never from the author's disk or the machine's font book.
- **Retiring the committed tier.** Considered and refuted by measurement: it would delete 30 of 31
  families from the product: they are variable-only on the mirror the fetch path reads, and reach
  this repository only as hand-committed statics from their own projects.
- **Refreshing a document's embedded faces.** A document is never silently re-vintaged.
- **No CJK catalogue change.** Noto Sans SC stays on the shipped-face path.

## Success signal

The author installs Sarabun, presses **B**, and gets bold — with nothing else to do and nothing to
read. The Sarabun document they saved last week offers to repair itself the next time they open it,
does so without touching the document, and the only evidence is a line in the status bar that
appears and then goes away.

## Assumptions

- *"All the font faces"* means the four cuts the typography panel can ask for, because that is what
  the **B**/**I** controls request and what the format can declare. Confirmed by the owner.
- A family's cuts are identified from `METADATA.pb` by weight 700 for bold and style `italic` for
  slope, matching how the four shipped families are cut in `shipped-face-cuts.ts`.
- The completion prompt appears on each open of a document with incomplete families until they are
  complete; a decline is not remembered across opens. Read from *"ask user at the document opening
  time"*, which settles when the author is asked but not whether a refusal persists.

## Open Questions

- What is the new `maximumCacheAssets` number? Measured at story-3 planning: the 31 families publish
  **112 cuts**, so the release goes from 80 assets to 158–161 and the repo grows ~21.8 MiB. The
  proposal is 90 → 168 with the warning at 160, to be re-derived from a real emitted manifest and
  pinned with its own rationale comment.
- When the author first uses a cut, the embed is an authored action and belongs in undo. Does
  undoing it remove the asset from the document, or leave an unreferenced face behind?
- The freeze rule says a later cut is taken at the embedded Regular's vintage *where available*.
  When it is not — the store holds only the newer bold — is the mismatched bold embedded, or is the
  cut treated as absent and the warning left standing?
- Does a document opened on a machine whose store is empty, and declined, differ in any way from
  one whose family genuinely has no bold? Both show the same sentence today, and only one is fixable.
- CAP-2 has a latent falsehood once a cut can be refused AT INSTALL — a variable face, a bad media
  type, a 404, a stall. A family that genuinely publishes a bold can then end up with none stored,
  and the absence sentence would blame the family for the installer's refusal. Does the install
  report a skipped cut, and does the panel distinguish *"upstream has none"* from *"this machine
  failed to fetch it"*?
