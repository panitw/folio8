# The `.folio` format

This is the canonical reference for the `.folio` template format: what every field is called, what
it means, which values are legal, and what the library does when a file breaks a rule. Format
version **4.2** is the highest version this library supports.

The format is a public contract, not an implementation detail. A person or a program can write or
edit a template by hand, without the designer, and a hand-written template renders exactly as a
designer-written one does. The designer never parses `.folio` itself; the engine owns the document.

- To render a template from Go, see the [rendering library guide](rendering-library.md).
- To render one from Node, see [folio-js](folio-js.md); from .NET, see [folio-dotnet](folio-dotnet.md).
- For expression syntax — paths, functions, formulas — see the
  [expression reference](expression-reference.md).

## Units

**Coordinates, sizes, margins and font sizes are in PDF points** (1 pt = 1/72 inch), written as
JSON numbers with **at most three decimal places**.

Three decimals is exactly one millipoint, the engine's internal unit, so the conversion is an exact
×1000 with no rounding. Values are parsed through the same exact-decimal path as report data —
never binary floating point. A coordinate with more than three decimal places is a load error,
because it cannot be represented exactly.

Points rather than raw millipoints because a hand-editor writes `"x": 36`, not `"x": 36000`.

## Document

```json
{
  "assets": {},
  "bands": {},
  "fonts": {},
  "locale": "th",
  "nextId": 14,
  "page": {},
  "unbreakableValues": ["customer.name"],
  "utcOffset": "+07:00",
  "version": "1.0"
}
```

| Field | Meaning |
|---|---|
| `version` | *Required.* `"MAJOR.MINOR"`. It describes the **document**, not the program that wrote it: a file declares the lowest version its own content requires. See [*Versions*](#versions) for the ladder and the load and save rules. |
| `locale` | *Required.* One tag from the closed set `en`, `th`, `zh-Hans`, `ja`. An unlisted tag is a load error. |
| `utcOffset` | *Required.* A fixed offset, `±HH:MM`, with hours `00`–`23` and minutes `00`–`59`. Anything else is a load error. The engine reads no host time zone. |
| `page` | *Required.* Page setup (below). |
| `fonts` | Named font stacks (below). |
| `bands` | Exactly three bands (below). |
| `pages` | *Optional.* The designed pages of a document with two or more pages (see [*Designed pages*](#designed-pages)). Absent from a one-page document. |
| `assets` | Embedded binary assets, keyed by content hash (below). |
| `nextId` | *Required.* The next element-id counter value, a plain decimal integer greater than the highest id present. Persisted so ids survive a save without renumbering; it is never repaired or inferred. |
| `unbreakableValues` | *Optional.* A list of **bare root-relative dotted value paths** (e.g. `"customer.name"`) whose bound values must never be split across a line break — the same path convention `columns[].footerOf` uses: no `{{ }}`, no function call, no `[]`. Row-scoped paths are written root-relative under that same convention. The engine **never infers** membership; see [*Line breaking*](#line-breaking). Declared once for the document because the property belongs to the data, not to a box. Absent means no value is protected. |
| `embedFonts` | *Optional.* A **boolean** — `true` or `false` and nothing else; a quoted `"true"`, a number or `null` is a load error, and `null` is not a synonym for the default. It says whether a **save** carries the faces the document's chains name, or merely names them. Absent means `true` — every document written before this key existed carries its faces. An authored `"embedFonts": true` is **accepted** and canonicalises back to the **absent key** on save (the same normalisation that turns `{"face": "X"}` with no variants into `"X"`), so `true` is legal to write and only `false` ever survives in a file. It governs what a save WRITES and nothing else: two copies of one document that differ only in this setting render identically, and a library that does not know the key carries it through verbatim and renders the same page set. It therefore raises no version (see [*Versions*](#versions)). **What an EDITOR does with it, stated because a reader will ask.** It is honoured at the moment a face is chosen rather than by a transform at write time: with the setting `true` a face an author applies is carried into [`assets`](#assets) and the chain entry takes the `asset` shape; with it `false` the same gesture writes the face **name** — the store's family-plus-style key, which is what a renderer's own supplied faces are keyed by — and nothing is added to `assets`. Turning it `false` on a document that already carries faces rewrites those entries as their names and drops every asset no entry then references; that is the editor's act and one the author is asked about, not something a loader or a save performs. **The rewrite is not cut-preserving, and an author deciding whether to turn the setting off needs to know that:** a carried entry's `bold`/`italic`/`boldItalic` are `assets` keys, and the entry it is replaced by is a bare face name, so those declarations are discarded along with the faces. Asking for that weight again redeclares it by name. **No filesystem path, URL or machine identity is ever written for a non-embedded face.** A face the document does not carry is a face NAME and nothing else. |

> `locale: "ja"` renders completely — the shipped font set has every Japanese glyph — but in
> Simplified-Chinese kanji SHAPES, because a font holds one drawing per codepoint and Chinese and
> Japanese share codepoints they draw slightly differently. This is legible and correct-content,
> never a missing-glyph error; it is a typography limitation, not a rendering failure. Supply your
> own face for Japanese typography — the font set passed to the renderer is a plain map from face
> name to font bytes, and the font chain is always declared by the document, never inferred from
> `locale`.

Top-level keys appear sorted, as does every object in the file — that is the serializer's job (see
[*Saving*](#saving)), not something an author maintains by hand.

### Alignment sets

`align` is not one closed set, and it is not two. It is **four**, and they are keyed on **the code
that consumes the value**, which in this format means **the element type that owns the style block**:

| Set | Where it applies | Legal values |
|---|---|---|
| Style align | a **non-table** element's own `style.align` | `left` · `center` · `right` · `justify` |
| Table style align | a **table**'s `style.align` and its `headerStyle.align` | `left` · `center` · `right` |
| Column align | `columns[].align` | `left` · `center` · `right` |
| Column header align | `columns[].headerAlign` | `left` · `center` · `right` |

A value outside its own set is a **located load error** naming the element and the field, and the
message lists exactly the members of the set that rejected it — so a table's rejection never names
`justify` as legal, and a text element's always does.

`columns[].headerAlign` is a fourth, separate declaration of the column triple. It feeds the same
cell consumer as the other table sets, so it carries the same three values.

**Why the partition is by consumer and not by key path.** A table's `style.align`, its
`headerStyle.align` and its `columns[].align` are all read into one fallback and drawn by one set of
cell switches: they are **one consumer wearing three key paths**. Splitting the set along the JSON
key instead would let a table declare `justify`, load, and render identically to `left` with no
diagnostic. So justified table cells are not supported, and the format *refuses* the declaration
instead of accepting it and ignoring it.

> **When splitting a closed set, partition it by the code that consumes the value, not by where the
> value is written in the document.**

## Versions

`version` is `"MAJOR.MINOR"`. This library supports every version up to and including **4.2**.

**Loading.**

- A document whose `MAJOR` is higher than the library's (`5`) is a load error
  (`TEMPLATE_MALFORMED`), never a best-effort render. So is a `version` that is not of the form
  `MAJOR.MINOR`.
- A document with a supported `MAJOR` and a higher `MINOR` (say `5.7`) loads. Keys the library does
  not know are carried through verbatim and written back on save.

**The ladder.** A file declares the lowest version its own content requires. The first matching row,
from the top, decides:

| Version | Required when the document… |
|---|---|
| `4.2` | names, from any chain in `fonts`, an asset whose `font` record carries `authorAcknowledged: true` — as the entry's own `asset` value **or** as one of its `bold`/`italic`/`boldItalic` style variants (see [*A font asset*](#a-font-asset)) |
| `4.1` | lists `pages` (see [*Designed pages*](#designed-pages)), or a content band or page declares `sectionBreak` or `sectionBreakAnchor` (see [*Pagination*](#pagination)) |
| `4.0` | has any element whose `type` is `barcode` or `qrcode` |
| `3.3` | has a text expression (a text element's `value` or a column's `bind`) statically known to be able to return a number (see [*Expressions*](#expressions)) |
| `3.2` | has a table column that declares `headerAlign` |
| `3.1` | has a table that declares `rules` or `minHeight` |
| `3.0` | has a table that declares a total `width` with proportional columns |
| `2.0` | has a style that sets `align: "justify"` (which only a **non-table** element's `style` can — see [*Alignment sets*](#alignment-sets)); **or** any `visibleIf`, text `value` or column `bind` uses formula syntax or boolean/null literals (see [*Expressions*](#expressions)); **or** any chain in `fonts` declares an entry that serialises as an OBJECT — an embedded face, or a face carrying style variants (see [*`fonts`*](#fonts)) |
| `1.2` | has an element that sets `keepTogether` |
| `1.1` | has a style (including a table's `headerStyle`) that sets `lineSpacing` or `color` |
| `1.0` | none of the above |

**Saving.** The rule is applied **on save**, in terms of what the document SERIALISES to. Saving
raises the version to the **highest** requirement the document's own written form actually carries;
it **never lowers** the version the document was loaded with, and it never stamps the library's own
ceiling on a document that does not need it. (So `{"face": "X"}` with no variants, which canonicalises
back to the bare string `"X"`, raises nothing.) A document using none of those keys still declares
`1.0` however new the library that wrote it.

A font asset that no chain references raises nothing: it rides through an older reader as ordinary
passthrough and renders correctly. The trigger is the chain **entry**, not the asset.

`embedFonts` has **no ladder row**, deliberately. A file declares the lowest version its own
content requires, and this key requires nothing of a reader: a library that does not know it
carries it through verbatim and renders exactly the same page set, because the setting governs
what a **save writes** rather than what a render reads. Giving it a rank would make a document
declare a version it does not need.

### Compatibility rules

- A MINOR increment may add **new optional keys** only. It may **not** change the meaning of an
  existing key, and it may **not** extend a closed set of legal values.
- Extending a closed set is a **MAJOR** change, because every existing library validates those sets
  as load errors. The same holds for changing the legal *shape* of an existing value: when a `fonts`
  chain entry became allowed to be an object, older readers — which decode an entry as a string —
  refuse such a file outright, so it requires `2.0`.
- A pattern-constrained string (such as `utcOffset`) is not a closed set. Making a reader stricter
  about documents it already reads is not a version trigger either, because version describes the
  document, never the reader.

### Closed sets

The format has **thirteen** closed sets of legal values. A value outside any of them is a load error,
and adding a member to any of them is a MAJOR change:

| # | Where | Legal values |
|---|---|---|
| 1 | element `type` | `text` · `image` · `table` · `line` · `rect` · `barcode` · `qrcode` |
| 2 | `locale` | `en` · `th` · `zh-Hans` · `ja` |
| 3 | `page.size` (as a name) | `A4` · `Letter` |
| 4 | `page.orientation` | `portrait` · `landscape` |
| 5 | a non-table element's `style.align` | `left` · `center` · `right` · `justify` |
| 6 | a table's `style.align` and `headerStyle.align` | `left` · `center` · `right` |
| 7 | `columns[].align` | `left` · `center` · `right` |
| 8 | `columns[].headerAlign` | `left` · `center` · `right` |
| 9 | `style.valign` | `top` · `middle` · `bottom` |
| 10 | `style.border.edges` members | `top` · `right` · `bottom` · `left` |
| 11 | `columns[].footer` | `sum` · `count` · `avg` |
| 12 | table `rules.between` members | `columns` · `rows` |
| 13 | qrcode `errorCorrection` | `L` · `M` · `Q` · `H` |

Two **key** sets are closed in the same way: the keys a `fonts` chain entry object may carry
(`face` or `asset`, plus `bold`, `italic`, `boldItalic`), and the keys a `pages` entry may carry
(`elements`, `pageBreak`, `sectionBreak`, `sectionBreakAnchor`). An unknown key there is a load
error rather than passthrough.

`mediaType` on an asset is deliberately **open** (see [*A font asset*](#a-font-asset)).

## Load errors

Loading a template (`ParseTemplate` or `LoadTemplate` in the Go library) either returns a template
or refuses the file with one error carrying a stable code. A refused file never renders. Match on the
code, not on the message text.

| Code | When |
|---|---|
| `TEMPLATE_MALFORMED` | The bytes are not a JSON object, a value under an unknown key cannot be read, the `version` is not `MAJOR.MINOR`, or its `MAJOR` is higher than the library supports. |
| `TEMPLATE_FIELD_INVALID` | The general code for a well-formed file carrying an unacceptable field: a missing required field, a value of the wrong JSON kind, a closed-set value outside its set, a misspelled or duplicate id, a coordinate with more than three decimals, a bad `fonts` entry, a missing font licence record, a table width or proportion that cannot be allocated, a barcode or QR value that is refused at load, a colour that is not `#RRGGBB`, a `lineSpacing` outside its domain, and every other field rule in this document without a more specific code below. The error names the field and, where there is one, the element. |
| `EXPRESSION_INVALID` | An expression that does not parse or does not check: syntax, arity, an unknown function, a provably wrong kind, a condition whose statically known outcome is a number or string. |
| `TABLE_FOOTER_SOURCE_UNRESOLVED` | A `sum`/`avg` footer with no `footerOf` whose `bind` is not one of the two derivable shapes, or a `footerOf` not under the table's collection path. |
| `TABLE_FOOTER_SOURCE_FORBIDDEN` | `footerOf` beside `footer: "count"`, or `footerOf`/`footerFormat` with no `footer`. |
| `TABLE_MIN_HEIGHT_UNPLACEABLE` | A table `minHeight` taller than the content window. Names the element. |
| `SECTION_BREAK_INVALID` | A `sectionBreak` or `sectionBreakAnchor` that breaks its rules (see [*`bands`*](#bands)). Located by field path — the band or page — not by element. |
| `SECTION_BREAK_STRADDLED` | An element whose declared box lies on both sides of a section break. Names the element; on a multi-page document the page's break path too. |
| `PAGES_INVALID` | A `pages` array or entry that breaks its rules (see [*Designed pages*](#designed-pages)). Located by field path. |

Render-time codes (warnings such as `TABLE_ROW_CLIPPED_HEIGHT` or `BARCODE_DOES_NOT_FIT`, and render
errors such as `CONTENT_UNLAYOUTABLE` or `BINDING_PATH_ABSENT`) are described where they arise below
and listed in full in the [rendering library guide](rendering-library.md).

## `page`

```json
"page": {
  "margin": { "bottom": 36, "left": 36, "right": 36, "top": 36 },
  "orientation": "portrait",
  "size": "A4"
}
```

`size` is `"A4"`, `"Letter"`, or an object `{"height": 841.89, "width": 595.28}` for a custom
page. `orientation` is `"portrait"` or `"landscape"`.

## `fonts`

```json
"fonts": {
  "body": ["Noto Sans", "Noto Sans Thai", "Noto Sans SC"]
}
```

Each key names an **ordered fallback chain**, tried left to right per glyph. `style.fontFamily`
references a key of this object, never a face name directly — so a chain is declared once and
reused, and the chain is part of the render's identity. A glyph covered by no face in the chain
produces a diagnostic (`TEXT_MISSING_GLYPH`) naming the element and the rune; it is never silently
blank. That warning is the outcome only when every face the chain declares was supplied to the
renderer — if a declared face is missing from the supplied font set, the same glyph refuses the
render instead, with the error `TEXT_FACE_ABSENT` naming the absent faces.

**A chain entry has exactly three legal shapes**, and they may be mixed in one chain, in any order:

```json
"fonts": {
  "body": [
    "Noto Sans",
    { "asset": "9ab1e6c2f0d34b7a5c8e1f20d4b6a839c7e5024f1b8d63a09e4c7512fb3d8a6e" }
  ]
}
```

1. **A string** — the name of a face the renderer is given at render time (the shipped set, or one
   the integrator supplies).
2. **A one-key object `{"asset": "<key>"}`** — a face carried *inside the document*, whose value is
   a key of the top-level `assets` object. A document declaring one requires `2.0`.
3. **An object carrying STYLE VARIANTS** — either of the two above written as an object, with
   optional siblings naming the faces this entry is drawn in when an element declares
   `style.bold`, `style.italic`, or both. The object's shape is exactly one discriminant —
   `face` **or** `asset`, never both and never neither — beside any of the three variant keys:

```json
"fonts": {
  "body": [
    "Noto Sans Thai",
    { "face": "Roboto", "bold": "Roboto Bold", "italic": "Roboto Italic" },
    { "asset": "9f86d0…", "bold": "1b4f0e…" }
  ]
}
```

| Key | Meaning |
|---|---|
| `face` | The discriminant of a shipped/supplied face, written as an object so it can carry variants. `{"face": "Roboto"}` and the bare string `"Roboto"` are the same entry; the bare string is what a variant-free entry serialises back to. |
| `asset` | The discriminant of a face the document carries — a key of the top-level `assets` object, exactly as shape 2. |
| `bold`, `italic`, `boldItalic` | *Optional.* The face this entry is drawn in for that weight and slope. **This is a CLOSED set of exactly three keys**, and extending it later is a **MAJOR** change, not an additive one — the closure is what keeps an unknown key inside an entry a load error rather than a decoration that rides along. ⚠ **The type differs from `style`'s keys of the same name**: on a `style` block `bold` and `italic` are **booleans** saying *what the author asked for*; on a chain entry they are **face names** (or `assets` keys) saying *what to draw it with*. A variant may **not name its own entry's base** — see below. |

A variant's namespace **matches its entry's discriminant**: a `face` entry's variants are face
names, an `asset` entry's variants are `assets` keys. A cross-namespace sibling —
`{"asset": "<key>", "bold": "Roboto Bold"}`, or `{"face": "Roboto", "bold": "<assets key>"}` — is a
**load error naming the sibling**, e.g. `fonts.body[1].bold`. Nothing is ever *inferred* from a
face name: an entry that declares no variant for the requested style has none, however the
renderer's own faces happen to be named.

**A variant naming its entry's OWN base face is a load error naming the sibling**, on either arm:
`{"face": "Roboto", "bold": "Roboto"}` and `{"asset": "<key>", "bold": "<the same key>"}` are both
refused at `fonts.body[1].bold`. Such an entry declares nothing — the face drawn at that weight is
the face that would have been drawn anyway — and it does so *silently*, because an entry that
declares a variant is not an entry that declares none, so the warning the three absence conditions
below all earn never fires. Write **no key at all** to say this entry has no such cut. **The base is
the only privileged name:** two *different* variants of one entry may name the same face —
`{"face": "Roboto", "bold": "X", "italic": "X"}` loads and renders — and that is deliberate, not an
oversight.

⚠ **The limit of that refusal.** It is **string equality** against the entry's own discriminant, so
it cannot see `{"face": "Roboto", "bold": "Roboto Copy"}` where two distinct face names hold
**identical bytes**. That renders bold-as-regular just as silently and the entry looks entirely
well-formed. Detecting it would mean comparing what is *inside* the two faces, and nothing in this
format is ever resolved or compared by anything read out of a font binary — the same rule that makes
an embedded face's `family` display identity rather than a resolver. The refusal closes the mistake
an author reaches by writing the same name twice; it does not close the case reached by a stranger
route.

**Three conditions all end the same way**, and an author needs all three to predict what a page
will look like. In each, the rune is drawn in **that entry's own base face** — never in a later
entry's, because losing the weight is a smaller lie than changing the typeface — and a **warning**
(`TEXT_STYLE_FACE_UNDECLARED`) names the element, the rune and the face actually drawn:

1. **The entry declares no variant** for the requested weight and slope.
2. **The declared variant names a face the renderer was not given.** The same tolerance a bare
   chain entry has (above) applies to a variant: it is skipped in silence, never a load or render
   error, because the same document is correct wherever that face IS supplied.
3. **The declared variant does not cover the rune.** Coverage chose the entry on its BASE face, so
   the variant is not guaranteed to carry the glyph.

A partial match is an absence too: an element declaring both `bold` and `italic`, against an entry
declaring only `bold`, draws the base face and warns — choosing the bold cut would be a nearest-fit
search, and nothing here searches. No weight or slope is ever synthesized.

Anything else — a number, an array, an object with neither `face` nor `asset`, an object with both,
or an object carrying any key outside that closed set — is a **load error naming the chain and the
entry's index**, e.g. `fonts.body[1]`. An `{"asset": …}` entry whose key is **not present in
`assets`** is likewise a load error, and it names the chain, the index and the key:
`fonts.body[1].asset`. A chain entry is never silently dropped and never coerced.

**An entry naming an asset that is not a font is ACCEPTED AT LOAD and errors at RENDER.** The load
path checks that the key exists in `assets` and nothing else — it never inspects the asset's
`mediaType` or its bytes to decide whether the entry is legal. That is the same rule the open
`mediaType` set rests on (see [*A font asset*](#a-font-asset)), and it applies to a *wrong-kind*
asset exactly as it does to an unrecognised font container: a chain entry naming an `image/png`
asset is a valid `.folio`. The failure arrives at render, and **only when something must actually
draw with that entry** — when a rune reaches it because no earlier entry in the chain covers that
rune. It is a located error naming **the chain, the entry's index and the asset key**. A document
whose text is covered entirely by the entries ahead of it renders clean and says nothing, because
nothing ever asked what those bytes were.

**Validation returns the identical error.** Validating that document — without rendering it —
returns the *same* error, with the same text and the same coordinates, and no diagnostics alongside
it. A validator that accepted a document the renderer refuses would be a second rule system, and the
one an author would trust is the one that says yes.

**Refusing is NOT what happens when a chain names a face nobody supplied, and the difference is
deliberate.** Given `["Noto Sans", <a non-font asset>, "Noto Sans Thai"]`, the render is refused at
the middle entry even though the entry after it covers every rune. Given
`["Noto Sans", "No Such Face", "Noto Sans Thai"]`, the middle entry is **skipped in silence** and the
third entry draws. The two conditions are not the same kind of thing:

- A chain entry naming a **non-font asset** is a defect *inside the document*. It travels with the
  file, it is wrong on every machine that will ever open it, and no deployment can make it right —
  so the moment something must draw with it, it is refused and located.
- A chain entry naming a **face the renderer was not given** is a property of *this* render, not of
  the document. The same file is correct wherever that face is supplied, and a fallback chain exists
  precisely so a document survives a host that is missing one of its faces.

**A face is resolved by ASSET KEY, never by name.** An embedded entry's `font.family` is display
identity — what a chain editor shows a person — and is never used to resolve or substitute a face.
Where a document carries a face whose `font.family` is `"Inter"` and the renderer is also given a
face named `"Inter"`, the two are **different faces** and neither ever stands in for the other; the
chain entry's shape decides which one is meant.

The map's keys have **no authored order**: they are sorted on write, like every other object in the
file. Only the array *inside* a chain is ordered, and that order is the author's and is preserved
verbatim.

## `bands`

```json
"bands": {
  "content":    { "elements": [] },
  "pageFooter": { "elements": [], "height": 40 },
  "pageHeader": { "elements": [], "height": 80 }
}
```

Exactly these three keys. `pageHeader` and `pageFooter` declare a `height`; **`content` does
not** — its height is derived as page height minus margins minus header minus footer, by one
function. Storing it would be a second source of truth.

| Band key | Meaning |
|---|---|
| `sectionBreak` | *Optional, `content` only.* One offset in points from the content band's top. Every element whose `y` is at or below it forms the **below-line section**, whatever its `x`; see [*Pagination*](#pagination) for where that section lands. It must be greater than 0 and less than the content height, it may appear once, and it is refused on `pageHeader` and `pageFooter` — each a load error (`SECTION_BREAK_INVALID`) naming the band. No element's declared box may lie on both sides of it (for a table the box is `y` to `y + headerHeight`; its rows and its `minHeight` floor may run past the line): that is a load error (`SECTION_BREAK_STRADDLED`) naming the element. The break is never drawn in the PDF. A document carrying it requires `4.1`. |
| `sectionBreakAnchor` | *Optional, `content` only.* A boolean, the section break's **Anchor** setting; absent means `true` (anchored). `false` makes the break **unanchored**: see [*Pagination*](#pagination) for how an unanchored section follows the content above it. It is valid only beside `sectionBreak`, it may appear once, and it must be `true` or `false` — declaring it without `sectionBreak`, on `pageHeader` or `pageFooter`, twice, as `null` or as any non-boolean is a load error (`SECTION_BREAK_INVALID`) naming the band. It is written only when `false`: an explicit `true` is dropped on save. It is part of the same `4.1` as `sectionBreak`. |

Every element's `x` and `y` are relative to **its band's** top-left corner, never to the page.

### Designed pages

A document may have several **designed pages**, each its own content column. The page header, the
page footer and the page setup are shared by every page. The file has **two shapes**, chosen by page
count:

- **One page.** The content is `bands.content`, and there is no `pages` key.
- **Two or more pages.** Every page, page 1 included, is an entry of the top-level `pages` array,
  in page order, and `bands.content` is `{ "elements": [] }` with no section break.

```json
"pages": [
  { "elements": [], "sectionBreak": 300 },
  { "elements": [], "pageBreak": true }
]
```

| Page key | Meaning |
|---|---|
| `pages[].elements` | *Required.* The page's content elements, exactly as `content`'s. Their `x` and `y` are relative to the page's content band. |
| `pages[].pageBreak` | A boolean, the page's **Page Break** setting. It is written as `true` or `false` on every page after the first; a later page with no value loads as `true`. On the first page it does not apply: a value there loads without error and is dropped on save. |
| `pages[].sectionBreak`, `pages[].sectionBreakAnchor` | *Optional.* The page's own section break, as `content`'s, on any page. Each page has at most one. Its range, its Anchor and its straddle rule are checked against that page's own elements only, and a problem is a load error located at that page's key (`pages[i].sectionBreak` or `pages[i].sectionBreakAnchor`; `SECTION_BREAK_INVALID`, or `SECTION_BREAK_STRADDLED` naming the element). It affects only that page's own content and overflow. |

Element ids are unique across the whole document, and a `keepTogether` group cannot span pages. Each
of the following is a load error (`PAGES_INVALID`) naming where it is: a `pages` array with fewer
than two entries or that is not an array (`pages`); elements in `bands.content` beside `pages`
(`bands.content`); a section break in `bands.content` beside `pages` (the key's path,
`bands.content.sectionBreak` or `bands.content.sectionBreakAnchor`); an entry that is not an object,
lacks `elements`, carries a key other than those above, or has a non-boolean or repeated `pageBreak`
(`pages[i]`, with the key); a `keepTogether` tag used on two pages (naming both). A duplicate id on a
later page is the ordinary duplicate-id load error (`TEMPLATE_FIELD_INVALID`), located at that page.
A document with `pages` requires `4.1`.

### Pagination

A document whose content is taller than one content band becomes **several pages**. The page header
and the page footer are drawn on **every** one of them, identically, at the same band origins: page
thirty-four is as complete as page one.

**The content band is a window onto one tall column.** The elements of `content` form a single
column of unbounded height; each page shows one page-height window onto it. A longer report is
**more windows**, never rearranged furniture. Four rules decide what a reader sees.

| | rule |
|---|---|
| 1 | **Where a page begins.** The first page's window begins at the top of the content band. Each later window begins at the top of the **first item that did not fit** in the window before it. |
| 2 | **What the unit is.** The unit that lands on a page is the **line**, not the element. A paragraph continues from the foot of one page to the head of the next. |
| 3 | **No line is ever split.** A line is drawn on the first page whose window holds it **entirely**, from the top of its tallest possible ascender to the bottom of its deepest possible descender. |
| 4 | **An image is atomic**, and the same rule applies to its **declared box**. |

**Whitespace at the foot of a page is correct.** Rule 3 means a line that would fall half on one
sheet and half on the next is drawn whole on the next sheet, and the space it vacated stays empty. A
statement cannot ship a half-line, and there is no setting that trades this away.

**Nothing moves sideways and nothing reflows to close a gap.** Every element keeps exactly the
position its author gave it within the column, so no element is ever displaced because a neighbour
grew — **with two exceptions, the below-line section of a `sectionBreak` and a designed page
with Page Break off, both described below; each moves as one rigid block.**
One consequence follows directly and an author should know it before designing a report rather
than finding it in a diff:

> **Across a window boundary, declared vertical gaps collapse.** An element that begins a window is
> drawn at the top of its page, whatever gap was declared above it. That is the price of never
> splitting a line and never reflowing a sibling. The one exception is the first page of a
> below-line section, which begins at the section's declared offset rather than at the top of the
> page — or, unanchored, directly below where the content above it ends.

**A section break moves the content below it as one rigid block.** When the content band declares
`sectionBreak`, the elements declared at or below it (the *section*) are paginated after the elements
above it (the *above-line content*). An **anchored** break (the default) moves the section by whole
pages only; an **unanchored** one (`"sectionBreakAnchor": false`) may also move it down within a page:

| | rule |
|---|---|
| 1 | **Where the above-line content ends.** It is the lowest bottom, in page space, of any above-line item on the last page that content reaches — counting a table's row displacement and its `minHeight` floor. |
| 2 | **Where the section lands.** Anchored: if that bottom is at or above the break, the section is drawn on that same page; otherwise on a new page added after it. Unanchored: if the content above ends on the first page, at or above the break, the section is drawn on that page; every other case is rule 5. |
| 3 | **Where on the page.** Anchored, each section element sits at its declared `y` in the content band, and nothing above the break is drawn on an added page. Unanchored, the same holds when the content above ends on the first page, at or above the break: an unanchored section is never pulled up there, so a break nothing crosses changes nothing. When that content reaches a later page, see rule 5. |
| 4 | **What the section does next.** A section taller than the room left on its landing page continues onto later pages under the four window rules at the start of *Pagination* — anchored, starting from its declared offset; unanchored and moved to a new page, starting from the top of that page's content window. |
| 5 | **Unanchored, when the content above ends below the break, or reaches a page after the first.** On such a later page the section follows the content wherever it ends, above or below the break, so it sits next to the content that pushed it. Let *E* be that bottom and *extent* the section's lowest bottom in page space when it is laid out at its declared offset, measured as in rule 1 — its rows as they grow with data, a table's row displacement, its `minHeight` floor and any floor push. If the section laid out that way needs more than one content window, or a group in it is clipped, there is no room. Otherwise, if *E* + (*extent* − the break) is at or above the bottom of the content window, the whole section is moved by *E* − the break on that same page (down, or up when a later page's content ends above the break), each element keeping its offset from the break. Otherwise the whole section moves to a new page added after it, with the break at the top of the content window. If the content above was clipped on its last page, there is no room, and the section moves to a new page. |

The line reserves space only on the page where the section lands: rows of above-line content use the
full page on every other page. An added page is a full page of the document — it carries the page
header and page footer, and `{{pages}}` and `{{page}}` count it. A document whose above-line
content fits on the first page and ends at or above the break renders exactly as it would without
the key — unless a `keepTogether` group has members on both sides of the break, since that group is
split on every render and so can paginate differently. Such a group is split at the line; each
side is kept together on its own, and every render returns the Warning
`SECTION_BREAK_SPLITS_KEEP_TOGETHER` naming the group. There is at most one break per designed page,
and it is never drawn. In a document with several designed pages, each page's break applies to that
page's own content column and overflow exactly as above, and to no other page; a group split by a
page's break is reported once per render, naming that page's group.

**Designed pages follow one another in page order.** Each designed page is paginated as its own
column under every rule above, including its own section break. How a page follows the page before it
is its Page Break setting (page 1 always starts the document):

| | rule |
|---|---|
| **On** | The page starts a new output page after the previous designed page's last output page, overflow included, with its content at its declared positions. |
| **Off** | Let *E* be where the previous page's content ends on its last output page, measured as in section-break rule 1, its below-line section included. The page's content is paginated alone, with its own break, as one block. If the previous page occupies **more than one** output page, nothing was clipped on its last output page, and the block paginates to exactly one output page with nothing clipped whose extent below the content window's top fits between *E* and the content window's bottom, the block is moved down as one rigid block so that its window top sits at *E*, every item keeping its offset, and it is drawn on that same output page. Otherwise the page starts a new output page exactly as with Page Break on. The block never moves up onto an earlier output page: when the previous page fits on a single output page, a Page Break off page still starts a new output page. A page with Page Break off and no elements, after a page that occupies more than one output page, adds no output page. |

The page header and page footer are drawn on every output page, and `{{pages}}` and `{{page}}` count
the output pages of all designed pages together. The design canvas draws every page at its declared
positions only; Page Break off changes the rendered output, never the canvas.

**No page is ever blank, except an empty designed page.** Because a window begins at the first item
that did not fit rather than at a fixed multiple of the content height, an element declared far
below the preceding content starts the next page instead of generating empty pages in front of it.
A designed page with no elements is still one output page, carrying only the page header and page
footer.

**An item that fits in no window is an error, not a surprise.** If a single line is taller than the
content band — a font size larger than the space available — or an image's **declared box** is, then
no window of any position can hold it, and rendering the document **fails with an error
(`CONTENT_UNLAYOUTABLE`) naming the element**. It is never drawn partly, never split, and never
spilled past the page edge.

Both cases are decidable from the **template and the font set alone**, with no report data: line
height is a function of the declared font stack and font size (see [*Vertical placement*](#vertical-placement)),
and the window is page height minus the declared margins and band heights. So this is a fault in the
document, reported the same way to everyone who renders it, and not something one report's data can
trigger and another's cannot.

**A table row taller than the page is clipped, not refused — and the reason is authorship.** A row's
height is not something the author typed; it is whatever the record made it. One customer's address
runs to nine lines while every other customer's runs to two, and a statement run of a hundred
thousand documents can contain exactly one record that no page could hold. Refusing that document
would take down the run for a fault the author could not have seen when designing the template. So
an over-tall **table row** — a header row, a data row, or the footer row — is placed alone on a
fresh page, drawn as far down as the page has room for, and cut off there. The render **succeeds**
and returns the finished document, with a warning (`TABLE_ROW_CLIPPED_HEIGHT`) naming the table,
which row, the row's height and the height it was measured against. Whole lines are dropped, never
half of one, and the row's own rectangle stops at the page's content bottom.

The line between the two answers is therefore **who is responsible for the height**:

| the thing that is too tall | where its height came from | what happens |
|---|---|---|
| a table row (header, data or footer) | the **data** — the author may never have seen the record | clipped to a page of its own, **warning**, document produced |
| a declared keep-together group whose members each fit, but whose **union** does not | the **author's** own declaration that these elements travel as one | clipped to a page of its own, **warning**, document produced |
| a declared keep-together group holding an element that is by itself too tall | the **author's** own declaration — and removing the tag is the fix | **error**, no document |
| a line of a text element | the **author's** declared font size | **error**, no document |
| an image | the **author's** declared box | **error**, no document |

folio8 absorbs what the data made too tall, and refuses what the author typed too tall — with one
deliberate exception, which is a *declared group* rather than a typed box. A set of elements the
author declared inseparable can add up to more than a page even though every member fits, and the
author has already said what should happen to it: keep it whole. Refusing the whole document at that
point would throw away a signature block for the sake of a rule about typed heights, so a group that
is too tall **only in aggregate** takes the row's answer instead — a page of its own, cut off at that
page's content bottom, and the same `TABLE_ROW_CLIPPED_HEIGHT` warning, worded for a group rather
than a row and naming the group the author declared. Whole members' lines and images are dropped,
never half of one: an image inside such a group is **removed, not moved**.

That exception reaches the *aggregate* and nothing else. If a single element of the group is by
itself taller than a content window, the document is **refused** and the error names that element.
What decides is **what** is too tall, never whether it happens to be tagged: a group of one adds
nothing, so tagging an element can never turn a refusal into a warning. A long text element the
author declares inseparable is refused for the same reason a too-tall image is — no page can hold
what was declared atomic. The difference from the untagged case is deliberate: untagged, that same
text element's lines simply split across pages and print in full, and the tag is what makes it
unsatisfiable, so removing the tag is the fix. A table row is the one thing that is never refused,
because its height comes from the data and its author has nothing to remove.

A typo in a template should still be found by the person who can fix it, at the moment they can fix
it; a pathological record should still not be able to stop a print run. Nothing is silent in any of
these directions — the clip always carries its warning, and the refusal always names its element.

There is no page-break key inside a content column and no widow or orphan control. `keepTogether`
(below) is the one thing an author writes on an element that pagination reads: it says which elements
must not be separated, never where a page ends. Within a column, where the pages fall is derived from
the four window rules and the section break above, and from nothing else; `pages[].pageBreak` only
decides how one designed page follows the previous one.

## Elements

Common to all seven types:

```json
{
  "height": 14,
  "id": "e7",
  "type": "text",
  "width": 200,
  "x": 20,
  "y": 12
}
```

| Field | Meaning |
|---|---|
| `id` | `e` + the counter in lowercase base 36 — `e1`, `ea`, `e1z`. Opaque: never derived from position or content, never reused, never renumbered on save. Unique across the whole document. Every diagnostic that concerns an element carries this. |
| `type` | `text` · `image` · `table` · `line` · `rect` · `barcode` · `qrcode`. The set is closed; an eighth type is a load error. A document carrying a `barcode` or a `qrcode` requires `4.0`, and a `3.x` reader refuses it. |
| `x`, `y`, `width`, `height` | Band-relative position and size, in points. **A `table` declares `x`, `y`, and an authored total `width` for proportional columns; it never declares `height`** — see below. For a **text** element, `width` bounds the laid-out content: content wider than the declared `width` is clipped at the box's left/right edges, never reflowed and never dropped, and a warning (`TEXT_CLIPPED_WIDTH`) names the element. `height` on a **text** element is **not** a clip bound — content taller than the declared `height` renders in full and no diagnostic is reported. (`style.lineSpacing` lets an author set the leading, so a vertical bound is something a template can be tuned towards by hand; it still is not something the engine checks the box against. `valign: middle`/`bottom` seat the packed block inside the declared `height`.) For an **image** element, `height` (together with `width`) is honoured: the image is scaled to fit the box and centred, never cropped and never stretched. |
| `visibleIf` | *Optional.* A bare expression (no `{{ }}` wrapping — see [*Expressions*](#expressions)); the element is absent from the page model when it evaluates false, and its siblings do not move. Evaluated during binding, before pagination — it can never depend on the page an element lands on. Condition semantics are `if()`'s own, unchanged: `true`/`false` decide visibility directly; an explicit `null` result is silently `false` (no diagnostic); a path absent from the data is a located Error; a string or a number is a located Error (no truthiness). A **field that is absent, or present with the JSON value `null`** (`"visibleIf": null`) both mean "no condition declared" — the element is visible, and there is nothing to evaluate; this is a *different* null from the condition **resolving** to `null` at evaluation, which is what hides the element. Boolean/null literals are valid conditions; numeric/string outcomes (e.g. `"visibleIf": "42"`) are rejected at **load**, including known outcomes in either conditional branch. `"visibleIf": "null"` hides the element. **Not valid on a table column — rejected at load, naming the column id** (row-level visibility would make pagination a function of data). |
| `keepTogether` | *Optional.* A string naming a **keep-together group**, e.g. `"keepTogether": "signature"`. Every content-band element carrying the same tag paginates as **one indivisible unit**: the whole set stays within the window it started in, or the whole set moves to the next one — each member still at its own declared position, with no sibling moved, no gap invented and no page left empty. The members need not be adjacent in the element list, and a tag is scoped to the document; in a document with designed pages all members must be on one page. **Content band only** — rejected at load on a `pageHeader`/`pageFooter` element, which is repeated verbatim on every page and never paginated — and **not valid on a `table`**, whose rows already carry their own grouping, rejected at load naming the element and the field. An absent field and an explicit `null` both mean "no group declared". A group taller than a whole content window **only in aggregate** — every member fitting, the sum not — is *clipped*, not refused; a group holding an element that is by itself taller than a content window is **refused**, naming that element. The tag is what makes such an element unsatisfiable, so removing it is the author's fix. See [*Pagination*](#pagination). Declaring this key requires `1.2`. |
| `style` | *Optional.* See [*`style`*](#style). |

**`text`** — adds `"value"`, the string, which may contain `{{ }}` bindings.

**`image`** — adds `"asset"`, a key of the top-level `assets` object. The image is scaled to fit
its box preserving aspect ratio and centred.

**`line`**, **`rect`** — no extra fields; both are drawn from `style.border` and
`style.background`.

**`barcode`** — adds `"value"`, the content, bound exactly as a text element's `value`: literal text
and `{{ }}` expressions, through the same evaluator. The resolved string is encoded as **Code 128**
(ISO/IEC 15417) with the code set chosen deterministically for the shortest symbol and the
modulo-103 check character added automatically. It is drawn as black filled bars that fill the
box's full height, centred horizontally, with a quiet zone of at least 10 modules on each side
inside the box; every module is the same whole number of millipoints, the largest that fits the
box width. No human-readable text is printed. Control characters are stored as the characters
themselves, so a carriage return is the ordinary JSON escape `"\r"` (the designer's content box
shows a carriage return as a new line, so Enter types one, and shows and accepts `\n` and `\\`;
a typed `\r` is accepted too). `value: null`, or a value that resolves empty, draws
nothing. A path absent from the data is a located Error, as for text. Refused at load: any `style`
key (a barcode has no colour, border, font or alignment), a non-ASCII character in the text outside
`{{ }}`, and `{{page}}`/`{{pages}}`. At render, and never stopping the render: data that resolves to
a character above ASCII 127 omits that barcode with Warning `BARCODE_UNENCODABLE`; a symbol that
cannot fit at 1 millipoint per module omits it with Warning `BARCODE_DOES_NOT_FIT`; a module
narrower than 0.25 mm (709 millipoints) still draws, with Warning `BARCODE_MODULE_TOO_SMALL`.
A caller that treats warnings as failures (the command-line renderer's `-strict` option, for
example) turns each into a failure.

**`qrcode`** — adds `"value"`, the content, bound exactly as a barcode's `value` (the same evaluator,
the same stored control characters and designer escapes, the same `null`/empty and absent-path
rules, and the same refusal of any `style` key and of `{{page}}`/`{{pages}}`), and
adds `"errorCorrection"`, optional, one of the closed set `L` · `M` · `Q` · `H` (about 7, 15, 25 and 30% damage
recovered). An absent `errorCorrection` means `M` and is saved absent; `null`, any other value, or the
key on any other element type is a load error. The resolved string is encoded as a **QR Code**
(ISO/IEC 18004 Model 2) in byte mode over its UTF-8 bytes, with no ECI, at the smallest version
(1–40) that holds it at that level; the mask is the one with the lowest penalty score, ties to the
lowest mask number. It is drawn as black filled rectangles — each row's dark modules merged into
horizontal runs — in a square symbol with a 4-module quiet zone inside the box. The module is the
largest whole number of millipoints for which the symbol plus quiet zone fits the smaller of the
box's width and height, and the symbol is centred on both axes. Refused at load: static text outside
`{{ }}` already longer than a version-40 symbol holds at the element's level. At render, and never
stopping the render: data too long for version 40 omits that QR code with Warning `QRCODE_TOO_LONG`;
a symbol that cannot fit at 1 millipoint per module omits it with Warning `QRCODE_DOES_NOT_FIT`; a
module narrower than 0.5 mm (1418 millipoints) still draws, with Warning `QRCODE_MODULE_TOO_SMALL`.
The same strict mode turns each into a failure.

### `table`

```json
{
  "as": "transaction",
  "bind": "transactions[]",
  "columns": [
    { "align": "left",  "bind": "{{transaction.date}}", "id": "e10", "label": "Date",   "proportion": 1 },
    { "align": "right", "bind": "{{formatNumber(transaction.amount, \"#,##0.00\")}}",
      "footer": "sum", "id": "e11", "label": "Amount", "proportion": 2 }
  ],
  "headerHeight": 16,
  "id": "e9",
  "type": "table",
  "width": 450,
  "x": 0,
  "y": 0
}
```

New tables declare **`x`, `y`, and a positive total `width` in points**, with a positive
`proportion` on every column instead of a column `width`. The engine resolves these weights into
millipoint widths once, and the same widths are used by editing commands, the canvas and the PDF.
The exact allocations sum to the authored total: largest remainders receive the remaining
millipoints, with ties in column order. A zero-width allocation is refused with its column
identified. Height remains derived from rows; a table never stores `height`.

Proportions are exact positive decimal weights with at most three decimal places, carried as
signed 64-bit thousandths. They have no typographic bounds. For example, total 500 with weights
1:2:1 resolves to 125, 250, 125 points; ten weights of 1 receive 50 points each. In the designer,
adding a column starts its weight at 1; adding or removing retains the total and surviving weights;
removing the last column retains the authored total for the next column; and totals beyond the
band's available width are refused. Proportional tables require format version `3.0`.

A table with no total `width` uses absolute column `width`s instead, whose sum is the table width.
The two representations cannot be mixed within a table: a `proportion` on a table with no `width`,
or a column without a positive `proportion` (or with a `width`) on a table with one, is a load error
(`TEMPLATE_FIELD_INVALID`) naming the column; a non-positive total `width` is one naming the table.

| Field | Meaning |
|---|---|
| `bind` | The collection path, suffixed `[]`. |
| `as` | The row-scope alias. Optional; defaults to `row`. Inside the table, `<alias>.field` is the current row; unqualified paths still resolve from the document root. |
| `headerHeight` | **Required.** Height of the repeated header row, in points. Accounted for on **every** continuation page. Because it is required, no command may clear it, and it is always written on save. |
| `columns[]` | Ordered. Each carries its own `id` (same counter as elements, so a diagnostic can name a column), `label`, `proportion` (or `width` when the table has no total), `align`, and `bind`. `columns[].align` is its **own** closed set — `left` · `center` · `right` — and does **not** admit `justify`. Nor does a table's own `style.align` or its `headerStyle.align`, which feed the same cell alignment and therefore carry the same three values. The sets are separate declarations so that extending one cannot legalise another by accident. |
| `columns[].headerAlign` | *Optional.* Aligns **this column's header cell only** — `left` · `center` · `right`, its own closed set. A header cell resolves `columns[].headerAlign` → `columns[].align` → `headerStyle.align` → `style.align` → `left`; data and footer cells never consult it. Absent means the header follows the column's `align`. Declaring it requires `3.2`. |
| `columns[].proportion` | Required instead of column `width` when the table declares its total `width`. A positive dimensionless weight with at most three decimal places; the engine allocates the exact total by these weights. |
| `headerStyle` | *Optional.* A `Style` block governing the header row ONLY — the same vocabulary as an element's own `style` (below) **except for `align`**, which admits `left` · `center` · `right` here and never `justify`, because a header cell is a table cell (see [*Alignment sets*](#alignment-sets)). A table's own `style.align` carries the same three values, for the same reason — never a data row. A field the header style leaves absent falls back to the table's own `style` for that field, then to that field's documented default — and that fall-through is **per field**, so `headerStyle: {"bold": true}` on a table whose own `style` sets `italic` gives the header row both. `headerStyle.bold` and `headerStyle.italic` govern the header row's weight and slope, which resolve to faces through the table's declared font chain exactly as an element's own do (see [*`style`*](#style) and [*`fonts`*](#fonts)). `columns[].align` still wins over both for that column's own header cell, and `columns[].headerAlign` wins over that. |
| `columns[].footer` | *Optional.* `sum` · `count` · `avg`. **Names the operation only**; the numeric source is `columns[].footerOf`, below. Computed over the **whole collection**, never per page. Omitted means no footer cell for that column. |
| `columns[].footerOf` | *Optional.* A bare root-relative dotted value path (e.g. `"transactions.amount"`) naming the numeric source the footer aggregates — no `{{ }}`, no function call, no `[]`. Legal only alongside `footer`, and never alongside `footer: "count"` (storing it would be a second source of truth against `bind`). When `footer` is present and `footerOf` is omitted, it is **derived** from the column's own `bind`, but only when `bind` is one of exactly two syntactic shapes: (1) a bare row-scoped path `{{<alias>.<rest>}}` → `footerOf` = `<collection>.<rest>`; (2) a single `formatNumber(<bare row-scoped path>, <pattern literal>)` call → `footerOf` = `<collection>.<rest>` from the first argument, **and** `footerFormat` defaults to `<pattern>`. `<collection>` is the table's own `bind` with `[]` stripped. Any other `bind` shape is a load error — never a guess. The derivation runs at load, and the derived value is resolved alongside the document, never written back into it — a document that omits `footerOf` still serializes without it. The aggregate is computed (`sum`/`count`/`avg`), can be formatted (`formatNumber`), and is rendered into the footer cell through the same expression evaluator used by ordinary bindings. Diagnostic codes: `TABLE_FOOTER_SOURCE_UNRESOLVED` (derivation failed, or an explicit `footerOf` is not under the table's collection) and `TABLE_FOOTER_SOURCE_FORBIDDEN` (`footerOf` beside `footer: "count"`, or a footer field with no `footer`). |
| `columns[].footerFormat` | *Optional.* A `formatNumber` pattern applied to the computed footer value. Legal with all three `footer` operations. |
| `altRowBackground` | *Optional.* Colour for alternating rows. Collection index zero retains the ordinary body treatment; the alternate colour applies to odd zero-based collection indexes (the second, fourth, sixth rows). Alternation follows that collection index, so it does not reset per page. Colour-by-data is out of scope for this field exactly as it is for `style`'s own colour fields (see "Colours are `#RRGGBB`" below): a `{{ }}` placeholder here is a **load error** naming the element, under the same rejection. It is the **only** declaration that fills a data cell — a table's `style.background` paints the table's own frame (below) and reaches no cell. Intervening data rows remain unfilled. Headers and footers are not alternating data rows. |
| `rules` | *Optional.* The lines **inside** the table: `{"width": 0.5, "color": "#000000", "between": ["columns"]}`. `between` is a closed set — `columns` · `rows` — naming which **boundaries** carry a line: `columns` rules every boundary between two adjacent columns, `rows` every boundary between two adjacent rows. Both, either, or `[]` for none; an **absent** `between` rules nothing. A rule is drawn **once**, at a boundary, and **never on the table's own edge** — the outermost column has no outer vertical rule and the bottom-most row has no bottom rule, because those lines are the frame's own border. A `rows` rule at the header/first-row boundary is **skipped** when `headerStyle.border` already strokes its `bottom`, so that one coordinate never carries two strokes. Column rules run from the **top of each page's slice to its bottom**, through whatever empty area `minHeight` creates on that page; a `rows` rule lies between rows, so it never enters that empty area. `width` defaults to `0.5`, `color` to `#000000`, exactly as `style.border`'s do. Declaring this key requires `3.1`. |
| `rules.between` | The closed set naming which boundaries `rules` draws a line at: `columns` · `rows`. Both, either, or `[]` for none; absent rules nothing. Extending this set later is a MAJOR version change, so it is effectively permanent. |
| `rules.width`, `rules.color` | The interior lines' width in points and `#RRGGBB` colour, defaulting to `0.5` and `#000000` exactly as `style.border`'s do. A negative width is a load error (ISO 32000-1 §8.4.3.2); `0` is the thinnest device line. |
| `minHeight` | *Optional.* A length in points: a **floor under each page's slice** of the table, never a height. For a slice whose top is `t` and whose content bottom is `c`, on a page whose content window ends at `w`, the slice's drawn bottom is `max(c, min(t + minHeight, w))` — it never lifts the content, never passes the window, and never moves the table to another page. Rows are unaffected; `minHeight` never stretches, shrinks or pads one. A table still declares no `height`, and its extent is still derived. The frame and the column rules are drawn to each slice's floored bottom, which is what a pre-printed form's empty ruled area is, on every page. The floor is reserved **during pagination**, so an element following the table on that page starts below the floored bottom rather than being overlapped (and moves to the next page if it no longer fits). A non-positive value is a **load error**: `max(0, content)` is `content`, which is what omitting the key already means. A `minHeight` taller than the content window is a **load error** naming the element, `TABLE_MIN_HEIGHT_UNPLACEABLE` — the alternative is a table that can never be placed. Absent or `null` means no floor. Declaring this key requires `3.1`. |

**A table's `style.border` and `style.background` paint the table's own frame.**
They are stroked and filled **once per page**, around and behind **that page's slice** of the table —
not stamped onto every cell. A table whose rows span pages draws a complete, closed frame on every
page it occupies: on a continuation page the frame's top edge sits under the repeated header, and on
every page its bottom edge sits at that slice's (floored) bottom. It is never one rectangle around the
whole table, and it never changes the table's page count. **The perimeter belongs to the frame:** where
the frame strokes an edge, no header or cell edge is stroked on that same line. The interior lines are
`rules`, above. An author who wants a full grid declares `rules: {"between": ["columns", "rows"]}`
beside a `style.border`: the perimeter is the frame's, and each interior line is drawn once.

The other `style` members — `fontFamily`, `fontSize`, `lineSpacing`, `color`, `bold`, `italic`,
`align`, `valign` and `padding` — cascade into data cells, because they describe the text inside a
cell rather than the chrome around it. `headerStyle.border` and `headerStyle.background` are the
only declaration that puts chrome on a header cell, and they do not fall back to the table's own
`style.border`/`style.background`, which belong to the frame.

A **data cell** (every row the table's `bind` produces) cascades its font, padding, align and valign
from the table's own `style` **only** — there is no `headerStyle` arm for a data row (`headerStyle`
governs the header row exclusively), and no border or background arm at all. `columns[].align` still
wins over `style.align` for that column's own data cells, exactly as it does for the header. A
**header** cell resolves one step earlier: `columns[].headerAlign` → `columns[].align` →
`headerStyle.align` → `style.align` → `left`. A table declaring `headerStyle.fontFamily` and no
`style.fontFamily` therefore renders its header successfully and fails its data cells with the same
"no resolvable `fontFamily`" error any other text-bearing element without one produces (there is no
font default, see [*`style`*](#style)).

**A column `label` may be more than one line.** `columns[].label` is an unbounded Unicode string and
is laid out through the same packer a data cell uses: a `\n` in it starts a new line, and a label
wider than its column **wraps** rather than being clipped in silence. Residual overflow — a run with
no break opportunity narrow enough — is clipped and reported with `TEXT_CLIPPED_WIDTH`, the same
warning a data cell emits; no clip path in a table is silent. `headerHeight` is the header row's
**floor** rather than its exact height: the row grows to
`max(headerHeight, the packed labels' height + padding)` when a label needs **more than one line**.
The header's line metrics are the maximum across every column's, so labels in different scripts share
one line geometry. The field stays required, and the packed height is settled at layout, before
pagination, so a repeated header is the same height on every page it appears on. A header whose
labels are all a single line does not grow, even when that line overflows the declared
`headerHeight`. The designer's editing commands bound a label at **256 Unicode code points**, counted
the same way by the engine and the designer.

### Expressions

Full expression syntax — paths, parameters, row scope, the eight functions and formulas — is in the
[expression reference](expression-reference.md). The rules below are the ones the format itself
depends on.

Visibility takes a bare formula, without `=` or `{{ }}`: `loanAmount > 20000` shows the element for
25000 and hides it for 20000 or 19999. Empty Visibility, an absent field, or JSON `"visibleIf": null`
means always visible. The expression string `"visibleIf": "null"` hides the element.

Conditions accept booleans and null: `true` shows or selects the first branch; `false` and `null`
select the other branch. Numbers and strings have no truthiness. Bold, Italic and all other fixed
boolean properties remain literal controls.

Highest precedence first:

| Syntax | Association |
|---|---|
| (expression), paths, calls, string/number literals, true, false, null | Grouping |
| Unary +, - | Right |
| *, /, % | Left |
| +, - | Left |
| >, <, >=, <= | No repeated unparenthesized comparisons |
| != | No repeated unparenthesized comparisons |
| condition ? then : else | Right |

`x ? y : a ? b : c` means `x ? y : (a ? b : c)`. For example,
`vip ? true : (blocked ? false : loanAmount > 20000)` checks VIP, blocked status and the threshold.
`2 + 3 * 4` is 14; `(2 + 3) * 4` is 20.

Lowercase whole words `true`, `false` and `null` are literals and never look up data. `trueFlag`,
`True`, `record.true` and `params.null` remain paths. `true.field` and `true()` are syntax errors.
Quoted words stay strings. There is no `==`, `===`, `!==`, `&&`, `||`, `!`, assignment, or scripting.

Ordering requires two numbers. `!=` accepts same-kind scalar values, comparing numbers by
mathematical value, strings exactly and booleans directly. Null differs from every non-null scalar;
`null != null` is false. Missing paths remain errors, including `customer.middleName != null` when the
field is absent. Collections and non-null mixed scalar kinds are errors.

Arithmetic accepts numbers only and uses exact bounded Decimals, never floating point. Addition,
subtraction and remainder retain the smaller operand exponent; multiplication adds exponents; unary
signs retain scale. Remainder uses truncation toward zero: `-5 % 2` is -1 and `5 % -2` is 1.

Division uses `max(operand decimal scales, 0) + 4` fractional places, rounded half to even at each
`/`, retaining all result trailing zeros: `1 / 3` is `0.3333`, `1.00 / 3` is `0.333333`, `1 / 8` is
`0.1250`, and `12 / 3 / 2` is `2.00000000`. Half ties include `1 / 32 = 0.0312` and
`3 / 32 = 0.0938`. Tiny results can round to zero: `1 / 100000 = 0.0000`. Zero divisors, overflow and
exhausted limits are located errors. Coefficients must fit int64 at the required scale, and exponent
magnitude is at most 100000; trailing zeros cannot be removed to avoid overflow. Thus
`1000000000000000 / 1` fails.

Both branches of `if()` and ternaries are parsed and statically checked. Unknown functions and
provably wrong types such as `false ? upper(1) : "ok"` fail at load or commit. Only the selected
branch resolves data or runs calculations: `true ? true : missingFlag` succeeds. Each statically
known branch must satisfy the consuming field's kind. Text accepts a string, a number or null:
`{{true ? "Yes" : "No"}}` renders Yes, `{{null}}` renders empty, `{{1}}` renders 1, and `{{true}}` is
an error.

**A number in text prints as its exact decimal.** This holds for a data value and a computed number
(arithmetic, `count`, `sum`, `avg`) alike, in text elements and table data cells. Footer cells are
always formatted by `formatNumber`. The text is the engine Decimal's coefficient digits with a `.`
placed by its exponent and the scale kept: `1234.50` prints `1234.50`, `2067071865` prints
`2067071865`, `-3.5` prints `-3.5`, `1e3` prints `1000`, `-0` prints `0`, and `0.000` prints `0.000`.
There is no exponent notation, grouping, locale digit, rounding or floating point; `formatNumber`
stays the way to get styled output. The printer never rounds, but division and `avg` results carry
their rounded division scale: `{{1 / 3}}` prints `0.3333`, and `{{avg(...)}}` over 1 and 2 prints
`1.5000`. Booleans, arrays and objects in text stay located errors. Condition, function-operand and
footer-source kind rules are unchanged by this.

A document requires `3.3` on save when any text expression is statically known to be able to return a
number without data — `{{1}}`, `{{count(items)}}`, `{{a + b}}`, `{{avg(items.x)}}`. A reader older
than `3.3` refuses such a document at LOAD, because its static text check rejects a number-typed
expression; the plain-path case below loads and fails at render.

> ⚠ **A plain path cannot be detected, and this is disclosed rather than mechanised.** The kind of
> `{{row.amount}}` is a property of the data, not of the document, so a document whose only
> numbers-in-text arrive through plain paths keeps whatever version its other content requires. An
> older reader given such a document still loads it and, when the path resolves to a number, fails
> the render with a located error. That is a refusal, never a silently wrong output.

Expressions are bounded to 64 KiB, 4096 AST nodes, depth 64 and 1,000,000 evaluation work units
shared across nodes, strings, collection projection and decimal shifts. Errors identify the field and
element, with a source-relative UTF-8 byte offset when available. Refused edits leave document bytes
and history unchanged.

Formula syntax and boolean/null literals in any expression container require at least `2.0` on save.
The requirement is derived from the parsed expression; quoted punctuation and ordinary paths do not
raise it. Saving never lowers a loaded version.

No-data preview needs no fabricated data for literals. Other paths receive compatible defaults:
numbers zero, direct divisors one, strings empty, and collections empty. A path used bare in text may
take the zero (or divisor one) stand-in, so a path shared by text and `formatNumber` previews. Both
branches contribute requirements. Conflicting requirements or a computed zero divisor refuse preview
with sample-data guidance; valid formulas remain committable. Generated data does not guarantee a true
condition. Parameters still come from Preview inputs.

There are, and will only ever be, **eight** named functions: `sum`, `count`, `avg`, `formatDate`,
`formatNumber`, `upper`, `lower`, `if`. The set is closed in the engine — a ninth would be a
deliberate change to the library, never a runtime surprise. All eight are implemented. Aggregate
functions evaluate exact-decimal values over the whole collection, and `formatDate`/`formatNumber`
apply the declared locale rules; unsupported function names remain located load/evaluation errors
rather than silently wrong values.

**`upper(x)` / `lower(x)`** apply Unicode case mapping. `x` must resolve to a string; any other
kind (including an absent or null path) is a located error, never coerced. A script with no case
distinction (Thai, CJK) is unchanged, byte for byte.

**`if(condition, then, else)`** takes exactly three arguments and evaluates only the branch it
selects. Both branches are statically checked; an absent path in the unselected branch is not
resolved. `condition` must resolve to a **boolean or null** — there is no truthiness anywhere in this
grammar: a JSON `0`, an empty string `""`, and an empty array `[]` are all the WRONG KIND for a
condition, and each is a located error, exactly as any other wrong-kind value is — never treated as
false.

**An absent path as `condition` is a located error naming the path.** This is deliberately
different from the next rule:

**An explicit JSON `null` as `condition` silently selects the `else` branch — no error, no
diagnostic, no warning.** This is the one behaviour in the engine that leaves no signal anywhere in
its output: a reader of the rendered document cannot tell a section hidden by a null condition from
one that was simply never authored. This trade was made deliberately, with that cost stated plainly,
in preference to a warning that most template authors would never see. If a rendered document is
missing a section you expected, and its visibility is driven by `if(row.someFlag, …)`, check whether
`someFlag` can be `null` in your data — that is the one case this format will never flag for you.

### `style`

Every field optional; omitted fields inherit the documented default.

```json
"style": {
  "align": "left",
  "background": "#F1F4F7",
  "bold": false,
  "border": { "color": "#000000", "edges": ["bottom", "top"], "width": 0.5 },
  "color": "#1B2A4A",
  "fontFamily": "body",
  "fontSize": 9,
  "italic": false,
  "lineSpacing": 1.5,
  "padding": { "bottom": 2, "left": 3, "right": 3, "top": 2 },
  "valign": "top"
}
```

| Field | Default |
|---|---|
| `fontFamily` | **none — required on any element carrying text** |
| `fontSize` | `10` |
| `bold`, `italic` | `false`. **Booleans: what the author asked for, not what to draw it with.** The FACE each one resolves to is declared on the font chain, per entry — see [*`fonts`*](#fonts), where a chain entry's own `bold`, `italic` and `boldItalic` name faces. Resolution is **per rune, through the declared chain**: the entry that COVERS the rune is chosen on its base face first, and the declared variant is then applied within that entry. An entry that declares no face for the requested weight and slope draws the rune in **its own base face** and emits a warning; the chain is never walked for weight, and no bold or oblique is ever synthesized. A table cascades both to its cells like every other cell property, and `headerStyle.bold`/`headerStyle.italic` win for the header row. |
| `lineSpacing` | absent — the leading the declared font chain itself rules. A ratio scaling the baseline-to-baseline advance, and **only** that: the ascent above the first baseline and the descent below the last are untouched, so the ratio never re-measures a line, and a component's siblings never move. Under the default `valign` (`top`) the first baseline therefore stays exactly where it was. Note the one place the ratio is still visible in a first line's position: `valign: middle`/`bottom` seat the whole packed block inside the declared `height`, and a ratio makes that block taller, so the block is re-seated and its first baseline moves — measured at 11pt over two lines, `1.5` lifts a `bottom`-aligned first baseline by 7.491pt. That is `valign` doing its job on a taller block, not the ratio touching the first line. An exact decimal of at most three places, between `0.001` and `1000.0` inclusive; anything outside that, or a fourth decimal place, is a located load error (`TEMPLATE_FIELD_INVALID`) naming the component and the field — never a silent clamp. Values below `1` are legal and genuinely tight: one line's letters may reach into the line below, which is what tight leading is and what the page draws. Declaring it requires `1.1`. |
| `align` | `left` · also `center`, `right`, `justify` — **`justify` is for a non-table element's own `style` only**; `columns[].align`, a table's `style.align` and its `headerStyle.align` all keep the three-value set. `justify` flushes both edges by distributing the line's leftover width across its interior break opportunities, in whole millipoints: every gap receives `slack / gaps` and the first `slack mod gaps` gaps *in reading order* each receive one more, so the distributed amounts sum to the slack exactly and the last piece's right edge meets the declared `width` exactly. Three independent conditions leave a line ragged at the element's own start edge: it is the **last line** of the element; it was ended by a **mandatory break** the author typed; or it has **no interior break opportunity** to place slack in (an atomic unknown Thai run offers none). An element with no declared `width` has no box to justify to, and a line that meets or overflows its width has no slack — the clip-and-warn rule for text wider than its box applies unchanged. Declaring `justify` requires `2.0`. It is legal **only on a non-table element's own `style`**: a table's `style.align`, its `headerStyle.align` and its `columns[].align` all admit `left` · `center` · `right` alone, and `justify` at any of the three is a located load error naming the element and the field (see [*Alignment sets*](#alignment-sets)). So a table can never reach `2.0` through `align`. |
| `valign` | `top` · also `middle`, `bottom` |
| `padding` | `0` on all four edges. Honoured **only** inside a table's cell chrome, where it insets a cell's content from that cell's own edges. A table cascades it to its cells like every other cell property, and `headerStyle.padding` wins for the header row. It is the chrome of a **table cell**, and only of that: on a `text`, `image`, `line` or `rect` a declared `padding` is **accepted and inert** — it loads, it round-trips, and nothing ever consumes it. The designer authors only a **table's** own `style.padding.left`/`right`, from the Table Editor; the header row takes it too unless `headerStyle.padding` exists. The designer offers no padding off a table, does not author top/bottom or `headerStyle.padding`, and introduces no default padding. |
| `border` | absent — no border drawn |
| `border.width` | `0.5` pt |
| `border.color` | `"#000000"` |
| `border.edges` | all four; a subset draws only those edges |
| `background` | absent — transparent |
| `color` | absent — the PDF's own initial fill, black. The INK the element's text prints in, as against `background`, the box behind it. A table cascades it to its cells like every other cell property, and `headerStyle.color` wins for the header row. Declaring none emits no colour operator at all. It is the ink of **text**, and only of text: on a `rect`, `line` or `image` a declared `color` (still `#RRGGBB`) is **accepted and inert** — it loads, it round-trips, and nothing ever paints it. Declaring it requires `1.1`. |

There is no font default. An element with text and no `style.fontFamily` is a located error naming
the element. `fonts` is a mapping with no authored key order, so "the first key" is not well-defined.

Colours are `#RRGGBB`: `#` and six hex digits, in either case. Every colour field — `style.color`,
`style.background` and `style.border.color`, the same three under `headerStyle`, and a table's
`altRowBackground` and `rules.color` — is checked at load: a value that is not `#RRGGBB` is a load
error (`TEMPLATE_FIELD_INVALID`) naming the element and the field, whether or not the element would
ever be drawn, so a hidden element or a table with no rows is refused too. There is no colour-by-data:
conditional *visibility* is in scope, conditional *formatting* is not. A `{{ }}` placeholder found
inside any string-valued style field (`background`, `color`, `fontFamily`, `border.color`,
`border.edges`) — or in a table's own `altRowBackground` or `rules.color` above — is a **load error** naming the element
and the field — a component's condition turns it on or off (`visibleIf`), it never changes how the
component looks. Apart from the colour shape, this is not general style-field validation: a
font-family name is accepted at load with no format check. Closed-set fields such as `align` and
`valign` are governed by their own sets instead.

## `assets`

```json
"assets": {
  "a31f60866b4aa41953176fee9ddb90dc9bc53dce174421f8f567fac364c8bc27": {
    "data": [
      "iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAGklEQVR42mLhEpGTk5NjsbGxkZOT",
      "AwQAAP//CoABrYEc9NQAAAAASUVORK5CYII="
    ],
    "mediaType": "image/png"
  }
}
```

Keyed by the **lowercase hex SHA-256 of the raw bytes**, so identical images stored twice
deduplicate and emission order is stable. `data` is base64 hard-wrapped at 76 columns into an
array of strings — the file stays valid JSON, and the template's non-asset content stays readable
in a text diff. Elements reference an asset by its key.

*(The example is a real, supported, non-alpha 2×2 PNG, canonically wrapped.)*

Images are only ever embedded. folio8 never fetches by URL and never reads from disk at render
time.

### A font asset

An asset is not only an image. A **font face** is stored by exactly the same mechanism — same key
rule (the lowercase hex SHA-256 of the decoded bytes), same 76-column `data` wrapping, same
deduplication, same emission order — and differs only in its `mediaType` and in one additive,
optional record:

```json
"assets": {
  "9ab1e6c2f0d34b7a5c8e1f20d4b6a839c7e5024f1b8d63a09e4c7512fb3d8a6e": {
    "data": ["AAEAAAAP…"],
    "font": {
      "copyright": "Copyright 2022 The Noto Project Authors (https://github.com/notofonts/thai)",
      "family": "Noto Sans Thai",
      "licence": "SIL Open Font License 1.1",
      "licenceText": "Copyright 2022 The Noto Project Authors…\n\nThis Font Software is licensed under the SIL Open Font License, Version 1.1.\n…the whole text, verbatim…",
      "source": "https://github.com/notofonts/thai",
      "style": "Regular"
    },
    "mediaType": "font/ttf"
  }
}
```

It is a record *about* the face for the people reading and reusing the document — the `family` and
`style` a designer shows in a chain editor, the `licence`, `licenceText` and `copyright` that state
on what terms and by whose grant the bytes may be passed on, and the `source` that says where they
came from. The engine derives none of it from the bytes and none of it is required to **render**.

When the designer embeds a family fetched from its published font library, it fills the record from
the family itself: `licenceText` from **the family's own upstream licence file, carried verbatim**
(never a hand-copy, which would be a second authority on the terms), `copyright` from **name ID 0 of
the face's own `name` table**, and `licence` from a closed token table mapping the upstream
vocabulary to an SPDX identifier — **`licence` carries the SPDX id, never the upstream token**
(`OFL-1.1`, not `OFL`). The record's shape and load rules are the same whoever wrote it.

**Whether `font` is optional depends on whether a chain names the asset, and that is the whole
rule.**

- **No chain names this asset.** `font` is **optional**, and every key inside it is optional. Such
  an asset is not an embedded face — nothing draws with it and no face is redistributed on its
  account — so the document carries it verbatim and loads clean with the record absent, partial, or
  explicitly `null`. This is also why such an asset does not raise the document's `version` (see
  [*Versions*](#versions)): it is legible to a `1.x` reader exactly as it is.
- **A chain names this asset** by `{"asset": "<key>"}`, *or names it as a style variant of an
  embedded entry* — `{"asset": "<other key>", "bold": "<key>"}`. Either way it **is** an embedded face, and
  `licence`, `licenceText` and `copyright` are **REQUIRED**: each must be present, non-`null` and
  non-empty. A document that fails this is a **load error**, located at
  `assets.<key>.font.<the first missing key>` and naming the chain entry — `fonts.<chain>[<i>]`, or
  `fonts.<chain>[<i>].<variant>` when a style variant is what named it — that made the asset an
  embedded face. A variant asset key clears this bar exactly as the entry's own `asset` value does;
  a sibling that skipped it would let a document carry an unlicensed embedded bold. It is never a
  warning and never a best-effort render.

  A font that travels without its terms is not a font that may be passed on, and a `.folio` is a
  single file that travels alone: there is nowhere else for the terms to be. `licenceText` is the
  **actual text** of the licence, not its name — `licence` already carries the name. The
  duplication that costs (a document embedding three OFL families carries three near-identical
  copies of ~4 KB) is accepted deliberately, because an asset extracted from the document and
  passed on by itself must still carry its own terms, which a single document-level notice block
  would not survive.

These required keys add no version trigger. They can only be required on an asset a chain names by
`{"asset": key}` (or as an embedded variant), and that entry shape already requires `2.0`; they are a
record *about* the asset and reach no output byte.

#### `authorAcknowledged`

*Optional. A boolean.* The author's acknowledgement, recorded on the face it admitted: the person who
authored the document stated, when they imported this face from their own machine, that they hold the
right to use and redistribute it.

`true` — and only a present, non-`null` `true` — **excuses `licence`, `licenceText` and `copyright`
from the rule above**: on an acknowledged record all three may be absent, `null` or empty, and the
document still loads. A face an author supplies off their own disk carries whatever its binary's
`name` table happens to say, which is very often nothing, and the acknowledgement is what stands in
place of the terms it cannot state. It is honoured at **both** doors that ask a licence question: the
load rule above, and the writer's own refusal to embed a face whose `name` table names a copyleft or
share-alike licence. It excuses **nothing else** — `family`, `style` and `source` are identity, not
terms, and the acknowledgement says nothing about them.

`false` and `null` acknowledge nothing and are held to every rule an absent key is held to. The key
is written only when it is `true`, so a face that carries no acknowledgement carries no key either. Any
other value — a string, a number, an object — is a **load error** (`TEMPLATE_FIELD_INVALID`) located
at `assets.<key>.font.authorAcknowledged`. It is refused where it is written, not carried through as
an unknown key: the three-way distinction between absent, `null` and set is the whole of what this
key means, and a spelling outside it says nothing a reader could act on.

```json
"assets": {
  "3f1c8ab27d4e5069b1a3c7e0d582f46b9c0e13a7d4b6f28e05c9a71d3b4e6f80": {
    "data": ["AAEAAAAP…"],
    "font": {
      "authorAcknowledged": true,
      "family": "Brand Grotesk",
      "source": "imported from the author's own machine, acknowledged 2026-09-21",
      "style": "Regular"
    },
    "mediaType": "font/otf"
  }
}
```

> ⚠ **It asserts; it does not prove.** The file cannot say who made the acknowledgement or whether it
> was true, and anyone hand-writing a `.folio` can set it. That is accepted deliberately: holding the
> right licence for a font the author loads is the author's responsibility, not this format's. A
> reader must not treat this key as evidence of anything beyond the assertion having been recorded.

A face this library's **own catalogue** distributes never carries one: such a face passes a build-time
licence gate and states real terms, so an acknowledgement on a catalogue face is a defect rather than
a shortcut.

A document whose chain names an acknowledged asset requires `4.2`, and that row is an honest
declaration rather than a gate. An older reader carries this key through — the `font` record's key
set is open — and then refuses the document on exactly the terms the key exists to excuse. It does
that whatever the document declares, because a higher MINOR loads; so the row does not change what
any reader does, it states what the document needs in order to be rendered as written. The failure
is always a refusal, never a wrong page: the key only ever *excuses* a check, so a reader that
ignores it is strictly the stricter one. An acknowledged asset **no chain names** raises nothing, on the
same rule as every other font asset: the trigger is the entry, not the asset. A **style variant** naming an acknowledged asset
— `{"asset": "<regular>", "bold": "<the author's own cut>"}` — triggers it exactly as the entry's own
`asset` value does, because the load rule above is asked about that sibling too.

**And it says nothing about [`embedFonts`](#document).** The two keys never meet: this one lives on
an embedded asset's record, and `embedFonts: false` is a document that carries no font assets at all,
so there is nowhere for an acknowledgement to sit and the `4.2` row cannot apply. Turning embedding
off strips the faces and their records together — the acknowledgement goes with the bytes it was
about, because it was never a statement about the document. (`version` is never lowered on save, so a
document that once carried an acknowledged face keeps whatever version it was last written with.)
Neither key implies, permits or refuses the other.

**`mediaType` is an OPEN set for fonts exactly as it is for images.** It is not one of the closed
sets listed in [*Closed sets*](#closed-sets) and it never will be: a closed set can only be extended by
a MAJOR bump, which would make every new font container a breaking format change. So:

- A **recognised** font media type whose bytes are not actually that format is a **load error** —
  the file lies about itself, and that is reader-independent.
- An **unrecognised** font media type (`font/woff2`, say, on a library that cannot decode it)
  **loads clean**. The document is valid and the asset is preserved verbatim. The failure, if any,
  arrives at **render**, and only when something actually needs to draw that face — a library
  capability limit, not a format error.
- A media type that is not a font type **at all** — an `image/png` asset named by a chain entry —
  takes the same path as the unrecognised one, for the same reason: it **loads clean**, and it
  errors at render only when something must draw with it. See [*`fonts`*](#fonts) for the shape of
  that error.

Both render-time refusals report the same thing, and it is a statement about **this build**, never
about the document: *the document is valid — `mediaType` is an open set — and this library cannot
draw with these bytes.*

## Saving

Saving (`SerializeTemplate` in the Go library) writes one canonical form, so a template saved twice
is byte-identical and a text diff shows only real changes:

- Every object's keys are sorted; arrays keep their order. Indentation is two spaces, and the file
  ends with a newline.
- Lengths are written as exact decimals with at most three decimal places and no trailing zeros
  (`36`, not `36.000`).
- Unknown keys the library carried through at load are written back verbatim.
- `version` is raised as described in [*Versions*](#versions), never lowered.
- A `fonts` entry object with no variants is written back as its bare face-name string.
- Asset `data` is re-wrapped to 76-column base64 lines.
- `headerHeight` is always written; `errorCorrection` is written only when declared; an explicit
  `sectionBreakAnchor: true` is dropped; `pageBreak` is written on every page after the first and
  dropped from the first.
- A document with exactly one page is written in the one-page shape (`bands.content`, no `pages`).
- Saving refuses a table whose widths cannot be allocated, as loading does.

## Line breaking

Text wraps inside its element's declared `width`. Where a line may end is decided per script, and
what follows is the **whole** rule — this section is deliberately written as a list of what the
engine does **not** do as well as what it does, because every omission below is a deliberate
narrowing rather than an unfinished edge.

Two kinds of break exist, and the difference between them is the difference between the engine
**guessing** and the engine **being told**:

- an **inferred** break is an *opportunity*. The engine proposes it from the text's script, and the
  line packer takes it only if the line needs it. The three script rules below are all of this kind.
- a **mandatory** break is *not* an opportunity. It is a line feed the author, or the data, put in
  the text, and the packer may not decline it.

### Inferred breaks

Where a line *may* end:

| Script | Rule |
|---|---|
| Latin, and anything with no rule of its own | A line may end **after** a run of whitespace, and nowhere else. The whitespace run is consumed by the break: it is drawn on neither line. |
| CJK | A line may end between any two adjacent Han or kana characters. |
| Thai | Thai is written without interword spaces, so break positions come from an embedded dictionary. A stretch of Thai the dictionary cannot account for is kept whole. |

### Mandatory breaks

Where a line *must* end: a `U+000A` line feed in an element's text, or in a value bound into it,
**always** ends the line — however much width remained. It is the only character with this meaning.

- **Breaks are separators: *k* of them produce *k+1* lines.** `"a\nb"` is two lines. `"a\n\nb"` is
  three, the middle one empty — which is how a paragraph gap is expressed. `"a\n"` is two lines, the
  second empty; `"\na"` is two lines, the first empty; a value that is nothing but a line feed is two
  empty lines.
- **An empty line is a real line.** It draws nothing and occupies one full baseline-to-baseline
  advance, so it adds to the element's height and to a table row's height exactly as a drawn line
  does — and can therefore move a page break.
- **`\r\n` is one break, never two.** A carriage return carries no line feed of its own; a lone `\r`
  is ordinary whitespace and stays an inferred break.
- **Whitespace *before* the break is consumed; whitespace *after* it is an indent and is drawn.**
  `"a \n b"` is `"a"` / `" b"`: the space before the break is drawn on neither line — trailing
  whitespace never widens the line it ends, which matters for `right` and `justify` alignment — while
  the space after it opens the second line. So a run of spaces at the start of a line is the way to
  indent that line inside a single element:

  ```jsonc
  { "type": "text", "x": 101.08, "value": "{{approval.requesterName}}\n           {{approval.requestDate}}" }
  ```

  An **inferred** whitespace break, by contrast, still consumes its whole run in both directions —
  nobody typed it, so no run beside it is an authored indent.
- **A mandatory break is not affected by `unbreakableValues`** — see
  [*Values that must never be split*](#values-that-must-never-be-split).

The engine never *invents* a mandatory break: it does not break at a declared width, at a hyphen, or
anywhere else on its own initiative. Only a line feed in the input produces one.

**This is not UAX #14, and nothing in folio8 claims conformance to it.** Absent, by name:
hyphenation; a break at `-` or any other punctuation; the contextual pair rules that make up the
bulk of the standard.

**No break falls inside a dictionary headword, including a lexicalised compound a native reader
would accept breaking.** The Thai engine matches against a shipped dictionary and never infers word
membership; where a compound word happens to be a headword, it is kept whole even if some readers
would break it. This is a stated capability limit, not a hidden one, and it is fail-closed: the
compound moves to the next line whole and is never rendered with a break inside it.

**Kinsoku is not implemented.** A CJK line may begin with `，` or `。` and may end with an opening
bracket. Fullwidth punctuation and fullwidth digits are not break candidates at all.

### Vertical placement

Where an element's lines sit vertically is a function of the element's **declared font stack** and
its font size, and of nothing else — in particular it does **not** depend on which characters happen
to land on a given line. Adding one Chinese character to a paragraph never reflows it.

> **A stack declares what may appear in an element. Vertical placement must accommodate what may
> appear — not what does appear.**

"What does appear" is content-dependent, and content-dependent placement would make a box negotiate
with its contents. "What may appear" is exactly the declared stack.

**One rule, three spans, three maxima.** Write `A`, `D` and `gap` for a face's `hhea` ascent, the
absolute value of its `hhea` descent, and its `hhea` lineGap, each read from the face's own `hhea`
table and scaled to the font size. Then, over the faces of the declared stack:

| span | distance |
|---|---|
| top of the element → **first** baseline | `max(A)` |
| baseline → next baseline | `max(A) + max(D) + max(gap)` |
| **last** baseline → bottom of the text | `max(D)` |

**Each maximum is taken independently, over its own axis.** That is the whole of the rule and it is
easy to get subtly wrong: the natural-looking `max(A - D + gap)` — the largest single face — is
**not** the same quantity, and it is too small.

The space between two baselines has to hold the **descenders of the line above** and the
**ascenders of the line below**. Those are two different lines, and on a mixed stack they can resolve
to two different faces, so the constraint is the worst **adjacent pair**, not the worst single face.
On the shipped stack the two axes are won by different faces — Noto Sans Thai has the deepest
descender (450/1000 em) and Noto Sans SC the tallest ascender (1160/1000 em):

```
worst pair       = max(A) + max(D) = 1160 + 450          = 1610
largest one face = max(A - D + gap) = max(1362,1511,1448) = 1511
```

The single-face form is **99 units of the em short** — enough for a Thai line's below-vowels to
touch the next line's ideograph ascenders, on the default stack. For a stack resolving to a **single**
face the two forms are identical, since one face cannot fail to supply both axes.

The first baseline is placed by `max(A)` for the same reason and by the same argument, asked about
the ascent axis alone: the tallest thing that may appear on the first line must fit above it. It is
**not** the point size. The two coincide only by accident, and they diverge in **both** directions —
Noto Sans SC's ascent is 1160/em, so its first baseline sits *lower* than the point size implies,
while a face whose `hhea` ascent is below its em sits *higher*.

The cost is bounded by the author's own choice. A Latin-only element in a
`["Noto Sans", "Noto Sans Thai", "Noto Sans SC"]` stack gets taller lines than Noto Sans alone would
need — but the author declared that stack, and an author who wants Latin metrics declares a
Latin-only stack. **No element pays for a face its own stack does not name.**

There is no `lineHeight` key and no first-baseline key. Vertical placement is derived; the only
authored adjustment is `style.lineSpacing`, which scales the baseline-to-baseline advance.

### Values that must never be split

The engine does **not** guess which stretches of text are names. It cannot: Thai surnames are
coined, one per family, out of ordinary everyday words, so a dictionary genuinely cannot tell a
person's name from the words it was built from — `ศรีสุข` as a surname is character-for-character
the two common words `ศรี` and `สุข`.

So a template **declares** it, in the document-level `unbreakableValues` list. Every value
substituted from a listed path is kept on one line. This binds the break opportunities the engine
**infers** — from whitespace, a script, or the dictionary — not literal control characters present
in the input: a line feed the caller supplied is a break the engine was **told** about rather than
one it proposed, so it is still taken inside a declared value. Literal text around the
placeholder is unaffected — `"Statement for {{customer.name}}"` still breaks between *Statement* and
*for*.

**The declaration protects bound values only.** A Thai name that appears inside free-form literal
text carries no declaration and remains breakable. That limitation is stated, not fixed.

If a value that must not be split is wider than its box, it **overflows visibly** rather than being
re-broken, squeezed or silently dropped.

## What is *not* in the file

- **Report data.** Supplied by the caller at render time.
- **Sample data.** A separate local file the designer loads for binding discovery and preview. The
  template does not store it or its path.
- **Parameters.** Supplied at render time. The designer keeps an author-edited parameter document as
  a sibling file, passed as the third preview input.
- **Dates.** Nothing in a template is stamped with a wall-clock time.

## Worked example

A minimal but complete template, in canonical form.

```json
{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {
          "as": "transaction",
          "bind": "transactions[]",
          "columns": [
            {
              "align": "left",
              "bind": "{{transaction.date}}",
              "id": "e3",
              "label": "Date",
              "width": 80
            },
            {
              "align": "right",
              "bind": "{{formatNumber(transaction.amount, \"#,##0.00\")}}",
              "footer": "sum",
              "id": "e4",
              "label": "Amount",
              "width": 90
            }
          ],
          "headerHeight": 16,
          "id": "e2",
          "style": {
            "border": {
              "edges": [
                "bottom"
              ]
            },
            "fontFamily": "body",
            "fontSize": 8,
            "padding": {
              "left": 3,
              "right": 3
            }
          },
          "type": "table",
          "x": 0,
          "y": 0
        }
      ]
    },
    "pageFooter": {
      "elements": [
        {
          "height": 10,
          "id": "e5",
          "style": {
            "align": "center",
            "fontFamily": "body",
            "fontSize": 7
          },
          "type": "text",
          "value": "Page {{page}} of {{pages}}",
          "width": 523,
          "x": 0,
          "y": 8
        }
      ],
      "height": 30
    },
    "pageHeader": {
      "elements": [
        {
          "height": 16,
          "id": "e1",
          "style": {
            "bold": true,
            "fontFamily": "body",
            "fontSize": 12
          },
          "type": "text",
          "value": "Statement for {{customer.name}}",
          "width": 400,
          "x": 0,
          "y": 10
        }
      ],
      "height": 60
    }
  },
  "fonts": {
    "body": [
      "Noto Sans",
      "Noto Sans Thai",
      "Noto Sans SC"
    ]
  },
  "locale": "th",
  "nextId": 6,
  "page": {
    "margin": {
      "bottom": 36,
      "left": 36,
      "right": 36,
      "top": 36
    },
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+07:00",
  "version": "1.0"
}
```

It uses absolute column widths and no feature above `1.0`, so it declares `1.0`. Its structure,
indentation and key order are exactly what saving produces.

`{{page}}` and `{{pages}}` are **not** expressions and **not** a data namespace — they are the two
late-bound page-number slots, the only values in the format that depend on pagination. No `page`
namespace exists for expressions to reach, and none may be added.

These resolve in the page header and page footer bands. Elsewhere — in the content band — the
document fails to render, naming the element.
