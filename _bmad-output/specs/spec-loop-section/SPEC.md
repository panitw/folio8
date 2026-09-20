---
id: SPEC-loop-section
companions:
  - prior-art.md
  - ../spec-section-break/SPEC.md
  - ../spec-folio/folio-format.md
sources: []
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# Loop

## Why

A **pain to solve**. folio8 can repeat exactly one shape: a `table`, which is a grid — the same columns, one row per collection item. Every other repetition an author needs is impossible. An invoice that wants a bordered panel per line item, with the item's description, its own charges table and its own barcode, cannot be authored at all; nor can a statement with a per-account summary block, or a bill with a labelled card per service. The author's only recourse today is to place N copies by hand and hide the unused ones with `visibleIf`, which fixes N at design time and is wrong the first time the data has N+1. A **Loop** is a region the author draws on the content band and binds to an array: the elements inside it are laid out once per item, each reading that item's fields, and the content below the loop moves down by however much the loop grew.

The shape is the mainstream one. Visual report designers model repetition as a container — the Detail band, the SSRS **List** data region, the AEM/LiveCycle **repeating subform** — and the last two keep free-form absolute positioning *inside* a region that repeats, which is exactly folio8's situation. The alternative the owner first proposed, a Loop Start / Loop End marker pair, is the flow-document idiom (docxtemplater, Carbone, Handlebars) and does not nest without explicit pairing rules. `prior-art.md` records the comparison and why the container was chosen. The designer still shows the author the two markers they asked for: the container is drawn with a **LOOP START** bar on its top edge and a **LOOP END** bar on its bottom edge.

## Capabilities

- **CAP-1**
  - **intent:** An author can add a Loop to a content band from the palette, size it, and place elements inside it, then select, move and delete it as one thing that carries its contents.
  - **success:** Arm the Loop palette entry and drag on the content band: a loop lands with that box, showing a LOOP START bar naming its bind and alias and a LOOP END bar. The entry stays armed, so a second and third loop can be placed without re-arming. Drop a text element inside it and the element becomes the loop's child, its stored `x`/`y` now relative to the loop's top-left and its on-screen position unchanged. Drag the loop: every child moves with it and no child's stored offset changes. Arrow-nudge it (1pt, Shift 10pt) and type its box in Properties. Delete it and its children go with it. Drag a child out past the loop's edge and it leaves the loop, its coordinates converted back to the band; drag a band element in and it joins, converted the other way. Dragging or typing one loop into another's box is refused, naming the loop that blocked it. Each step round-trips through the engine as exactly one undo entry.
- **CAP-2**
  - **intent:** A Loop binds to a collection and names each item, and the elements inside it read that item's fields.
  - **success:** A loop declaring `bind: "lines[]"` and `as: "line"` renders once per entry of `lines`. A text child reading `{{line.description}}` shows each item's own description in its own iteration, and a child reading `{{customer.name}}` still reads the document root in every iteration. `{{params.x}}` resolves to the parameter in every iteration. Omitting `as` names the item `row`. A bind that does not end `[]`, or that resolves to something other than a collection at render, is a located error naming the loop.
- **CAP-3**
  - **intent:** A Loop can sit inside another Loop, binding a collection reached through the outer item, so an author can lay out an invoice line and the charges belonging to that line.
  - **success:** An outer loop `bind: "orders[]" as: "order"` holding an inner loop `bind: "order.items[]" as: "item"` renders every item of every order, in order, with `{{order.ref}}` and `{{item.sku}}` both resolving in the inner iteration. An inner loop may instead bind a root collection, rendering it whole once per outer item. Three levels render. A fourth is a load error naming the loop. An inner `as` equal to any enclosing loop's or table's alias is a load error naming both, and `params`, `page` and `pages` are refused as aliases as they are for a table.
- **CAP-4**
  - **intent:** The content below a Loop moves down by exactly the amount the loop grew, as one rigid block, so nothing is ever overdrawn.
  - **success:** Render the golden invoice. With one line the document is byte-identical to the same template with the loop replaced by its single iteration's elements. With twelve lines every element below the loop is lower by (total iteration extent − the loop's declared height), each keeping its own offset from its neighbours, and nothing overlaps. A loop whose iterations extend past the content window continues onto later pages under the four window rules, and the pages it adds carry the page header and footer and are counted by `{{page}}` and `{{pages}}`.
- **CAP-5**
  - **intent:** An iteration grows with its own data, so a variable-length table inside the loop makes that one iteration taller without disturbing the others' internal layout.
  - **success:** A loop 120pt tall holding a table bound to the item's own collection renders iteration heights that differ per item: an item with 2 rows occupies the declared 120pt, an item with 20 rows occupies the height its content reached, and each following iteration starts at the previous one's bottom. Every child keeps its declared offset from its own iteration's top. The declared height is a floor and never a ceiling and never a clip.
- **CAP-6**
  - **intent:** An author decides what an empty array does — leave the space as designed, or close it up.
  - **success:** With the bound array empty and the default setting, the loop occupies its declared height as empty space and no element below it moves. With the author's opt-in, the loop occupies nothing and every element below it moves up by the declared height, as one rigid block. Toggling the setting is one engine command and one undo entry. With a non-empty array the setting changes nothing.
- **CAP-7**
  - **intent:** An author can require that an iteration is never split across a page.
  - **success:** By default an iteration splits at a page boundary under the four window rules, exactly as band content does. With the per-iteration keep-together setting on, an iteration that does not fit in the room left starts the next page whole, and the space it vacated stays empty. An iteration that is taller than a whole content window even on a page of its own is placed alone on a fresh page, drawn as far down as that page has room for, and cut off there; the render **succeeds** and returns a warning naming the loop and the iteration index. Whole lines are dropped, never half of one, and an image in the cut-off part is removed rather than moved. An iteration holding a single child that is by itself taller than a content window is still a hard **error** naming that child.
- **CAP-8**
  - **intent:** A Loop persists in the `.folio` text format, is documented there, and every invalid loop is refused with a located error.
  - **success:** Load then save round-trips byte-identically, nested and not, with and without the empty setting. Each of the following is a registered load error naming what is wrong: a loop on `pageHeader` or `pageFooter`; a child whose declared box leaves the loop's box; two sibling loops whose boxes overlap; a loop whose declared box straddles a `sectionBreak`; a loop nested more than three deep; an alias collision; an alias named `$`; a bare `$` with no dot; a `keepTogether` tag with members both inside and outside a loop. A document carrying a loop or a `$.` path saves as `5.0`, and one carrying neither is untouched. `folio-format.md` documents the element, the container-relative coordinate rule, the `$` root, the pagination rule and the version ladder entry.
- **CAP-9**
  - **intent:** The canvas shows the loop and its contents with geometry that comes from the engine, matching what the PDF does for the first iteration.
  - **success:** The canvas projection carries the loop's box, its bind, its alias, its settings and its children's container-relative positions. The loop is drawn once, at its declared position, showing one iteration — the canvas has no data and never draws where later iterations would land. A DOM-measurement ban test still passes, and preview of the golden invoice matches the rendered PDF.
- **CAP-10**
  - **intent:** Loops compose with the section break and with designed pages without either feature changing meaning.
  - **success:** A loop above a section break pushes the above-line end point that the break's rules already measure, so the below-line section lands correctly at 1, 12 and 60 items, anchored and unanchored. A loop on a designed page with Page Break off feeds that page's end point the same way. A loop whose declared box lies on both sides of a break is refused naming the loop.

- **CAP-11**
  - **intent:** An author can name the document root explicitly in any path, so a root field stays reachable however many aliases are live.
  - **success:** `{{$.customer.name}}` resolves the root's customer in a text element, inside a table row, and at every loop depth. With a loop whose `as` is `order` and a root field also named `order`, `{{order.ref}}` reads the item and `{{$.order.ref}}` reads the root field — which no spelling can reach today. `$.` is legal in a text `value`, a `visibleIf`, a column `bind`, a `footerOf`, and a table's or loop's `bind`; `$.orders[]` binds the root collection. A bare `$` with no dot is a located error saying it is a namespace, not a value. No alias may be named `$`. Every existing template renders byte-identically, because `$.` is never required.

## Constraints

- **A Loop is the third deliberate exception to "nothing moves because a neighbour grew", and the largest.** The existing two — the section break's below-line section and a Page Break off designed page — each move a rigid block once. A loop's displacement is *repeated* and its magnitude comes from the data. The exception reaches only the content below the loop in the loop's own container, which moves as one rigid block with every member keeping its offset. No other element gains the ability to move. The exception must be written into the *Pagination* section of `folio-format.md` beside the other two.
- **`loop` is a new member of closed set #1 (element `type`), which the format's own compatibility rules make a MAJOR change: `4.1` → `5.0`.** Older readers refuse a document carrying a loop outright rather than rendering it best-effort. This is the price of the container shape over the marker pair, and it is accepted deliberately. The `$` root notation rides the same MAJOR, because an older reader's expression parser fails on the `$` character. Only documents that carry a loop or a `$.` path declare `5.0`; every other document keeps its version and its bytes.
- **`$` is a resolution root, evaluated by first-path-segment equality like the others, and it can be shadowed by nothing.** The engine's order becomes `params` → `$` → the live aliases, innermost first → the data root. It is **optional everywhere and required nowhere**, so no existing template changes meaning; it exists because today a root field whose name equals a live alias is unreachable, and nesting multiplies that trap. No alias may be named `$`.
- **Several loops may share a band, and their declared boxes may not overlap.** Two intersecting loops are a load error naming both. A loop's growth pushes every later sibling loop down as ordinary content below it. Unlike the Section Break, the palette entry does not disable after one placement.
- **A loop is the first element that contains other elements.** Its children live in the loop, not in the band's element list. Element ids stay unique across the whole document, children included, so every diagnostic can still name an element by id. A diagnostic raised inside a loop also names the **iteration index**, because the same child id renders many times.
- **A child's `x` and `y` are relative to its container's top-left.** This generalises the format's existing rule that an element's `x` and `y` are relative to its band's top-left: a band and a loop are both containers. The loop's own `x`/`y` are relative to *its* container.
- **The loop's declared `height` is a floor on each iteration, never a fixed height and never a clip** — the same idiom as a table's `headerHeight` and `minHeight`. Iteration *i* occupies `max(declared height, the measured extent of that iteration's content)`. Iteration *i+1* begins at iteration *i*'s bottom; iterations are flush, with no invented gap.
- **A child's declared box must lie wholly inside the loop's declared box.** A child that leaves it is a load error naming the child — the same discipline as `SECTION_BREAK_STRADDLED`, so membership is never decided by a heuristic. A child may still *grow* past the declared box at render; that is what raises the iteration's height.
- **Aliases never shadow.** A nested loop whose `as` equals any enclosing loop's or table's alias is a load error naming both, matching the existing rule that a row never shadows the document root. Unqualified paths still resolve from the document root at every depth, and `params` can never be shadowed. `params`, `page` and `pages` are refused as aliases.
- **Nesting is capped at three levels** (invoice → line → charge). A fourth is a load error naming the loop.
- **A `keepTogether` tag inside a loop is scoped to one iteration.** Members of the same tag in different iterations are different groups, never one. A tag with members both inside and outside a loop is a load error naming the tag and both sides. The existing clip-not-refuse answer for a group too tall only in aggregate applies unchanged inside an iteration.
- **Loops are content-band only**, refused at load on `pageHeader` and `pageFooter`, for the same reason `keepTogether` and `sectionBreak` are: those bands are repeated verbatim on every page and never paginated.
- **A loop's declared box may not straddle a `sectionBreak`,** so the loop is unambiguously above-line or below-line content. An above-line loop's growth counts toward "where the above-line content ends", which section-break pagination rule 1 already defines as the lowest bottom of any above-line item, counting a table's row displacement and its `minHeight` floor; a loop's iteration displacement joins that list. A below-line loop grows the section.
- **An empty array collapsing the loop is folio8's first upward move, and it is opt-in for that reason.** The format has so far refused every upward move — an unanchored section is never pulled up on page 1. The setting defaults to *reserved*, so existing behaviour and output are never changed by the default, and the key is written only when the author opts in.
- **A loop never repeats horizontally.** Iterations stack downward only, in the collection's own order.
- Output stays byte-identical across darwin/arm64, linux/amd64, linux/arm64 and js/wasm (AD-21). All arithmetic is integer fixed-point. The engine is the only layout authority, including on the canvas.
- **In the designer the loop reads as the two markers the author asked for.** It is drawn as a region with a solid top edge carrying a mono LOOP START tab showing the bind path and alias, and a solid bottom edge carrying a LOOP END tab. Its chrome is never drawn in the PDF. A printed border around an iteration is still a `rect` element inside the loop.
- A golden fixture covering CAP-4, CAP-5 and CAP-7 joins the four-platform hash matrix. Every new diagnostic is registered in the diagnostic registry.

## Non-goals

- Grouping, sorting, filtering or de-duplicating inside a loop. A loop renders the collection it is given, in the order it is given, once per entry.
- An iteration index, counter or first/last test in expressions. The expression language stays at eight functions.
- Horizontal or multi-column repetition (labels across a sheet, newspaper columns).
- A loop in the page header or page footer, and a page header or footer that varies per iteration.
- A section break inside a loop, or a loop that forces a page per iteration.
- Replacing or subsuming `table`. A grid of one collection stays a table; a loop is for shapes a grid cannot express, and a table inside a loop bound to the item's own collection is the intended way to nest a grid.
- A minimum or maximum iteration count, or padding an under-full loop to a fixed number of iterations.
- Nesting deeper than three levels.
- Editing or adding iterations in the designer. The canvas shows one.

## Success signal

- A billing invoice is authored with one Loop bound to `lines[]`, each iteration holding a bordered panel with the line's description, a barcode and a nested table of that line's charges, and a signature block declared below the loop. It is rendered at 1, 12 and 60 lines, and again with `lines` empty under both empty-array settings. On no page is the signature block overdrawn, every iteration reads its own line's data, iterations of different charge counts have different heights and none is clipped, and the PDF hash is identical on all four platforms.

## Assumptions

- Nesting is capped at three levels because invoice → line → charge is the deepest shape the input implied; the number is a chosen bound, not a stated requirement.
- Iterations split across pages by default, with per-iteration keep-together as an opt-in, because that matches folio8's own defaults (no widow/orphan control, nothing kept together unless the author tags it) rather than the banded-engine default, which keeps a detail band whole.
- The loop is one element in the file even though the designer presents two markers, following the owner's choice of the container shape over the marker pair.
