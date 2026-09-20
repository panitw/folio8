# Prior art: how reporting products express "repeat this block per array item"

Research companion to `SPEC.md`. It answers one question the owner asked directly: **is a Loop Start / Loop End marker pair a sound way to express repetition, compared with what other reporting products do?**

## The two families

Every product surveyed lands in one of two families.

### Family A — the repeating **container**

The repeating unit is a region object. Elements inside it are *children*, with coordinates relative to the region, and the region repeats once per row.

| Product | The construct | Layout inside it |
|---|---|---|
| Crystal Reports, JasperReports, FastReport, Stimulsoft, DevExpress XtraReports, Telerik Reporting | the **Detail band** | band-relative coordinates; the designer auto-stacks bands (header, detail, footer) down the page |
| SQL Server Reporting Services / Power BI Report Builder | the **List** data region | **free-form absolute layout inside**; the List is "a table with a single column and detail row and no headers" that repeats per row or group |
| Adobe LiveCycle / AEM Forms Designer | the **repeating subform**, Binding → *Repeat Subform For Each Data Item* | the subform's own content may be `Positioned` (absolute) while its parent is `Flowed`; `Min Count` sets a floor on instances |

The SSRS **List** and the AEM **repeating subform** are the two closest analogues to folio8's world, because both keep **absolute positioning inside a repeating region**. That combination — free-form inside, repeating outside — is exactly what Loop Start / Loop End is reaching for, and it is well-trodden.

### Family B — the paired **inline markers**

The repeating unit is delimited by a start token and an end token that are themselves content.

| Product | The construct |
|---|---|
| docxtemplater | `{#items}` … `{/items}`, a *paragraph loop* when both tags sit alone on their own paragraphs |
| Carbone, Docmosis, Windward/Fluent | equivalent inline start/end tags in the document body |
| Handlebars, Jinja, Liquid | `{{#each}}` … `{{/each}}` block helpers |

## Why the split exists

**Family B works because the host document is a flow.** A `.docx` body is a linear sequence of paragraphs, so "between the markers" needs no geometric rule at all — it is simply the run of paragraphs from one token to the other. Ambiguity is impossible.

**Family A exists because a 2-D canvas has no such linear order.** Once elements are placed at arbitrary `(x, y)`, "between" must be *defined*, and the natural definition is containment: an element is in the region because it is a child of the region.

folio8 sits between the two. Its content band is a 2-D absolutely-positioned canvas (pushing toward Family A), but its file model is flat — a band holds one list of elements, and structure is expressed as **band keys with geometric membership rules**, not as nesting (pushing toward Family B).

## The precedent that changes the answer

folio8 has already solved "a flat marker that partitions a 2-D band" once: the **Section Break**. `sectionBreak` is a single offset in points; every element at or below it is the below-line section "whatever its `x`"; and the ambiguous case is not resolved by a heuristic but **refused** — `SECTION_BREAK_STRADDLED` names any element whose declared box lies on both sides of the line.

That is the whole difficulty of Family B on a 2-D canvas, already faced and already answered. A Loop Start / Loop End pair is the same rule applied twice: an element is above the loop, inside the slab, or below it, and an element straddling either line is a load error naming that element.

So the honest verdict is: **the pattern is sound, and folio8 is unusually well positioned to make it rigorous** — but it is the minority form among visual designers, and the reasons the majority went the other way are worth pricing in rather than discovering later.

## Where the marker pair costs more than a container

1. **Nesting.** Containers nest for free. A flat pair needs an explicit pairing rule — which Loop End closes which Loop Start — plus a geometric rule that an inner slab lies wholly inside an outer one. Section Break dodged this entirely by being capped at one per page. If loops are capped at one non-nested loop per designed page, the marker pair stays cheap; the moment nested arrays are in scope (invoice → line items → per-line charges), a container earns its keep.
2. **Scope stacking.** One loop needs one `as` alias, resolved exactly like a table's. Nested loops need a scope *stack*, and the "a row never shadows the document root" rule has to be restated for several live aliases at once.
3. **Selection and editing.** A container gives the designer a real parent to drag, copy and delete as a unit. With markers, "move the loop" means moving two independent markers in step and dragging the slab's members with them — hand-written logic the Section Break did not need, because it has no lower bound and never moves its members.
4. **Discoverability.** In Family A the author sees a named region and drops elements into it. With markers the author has to understand that membership is a consequence of `y`, which is the same thing authors already learned for the Section Break — so this cost is real but already paid once.

## Where the marker pair is genuinely the better fit for folio8

1. **It matches the file model.** `bands.content` holds one flat element list. A container element would be the first element type in the format that *contains* other elements, which changes id scoping, `keepTogether` scoping, the canvas projection and every command that walks elements. The marker pair adds two keys and changes no shape.
2. **It matches the designer the author already learned.** Section Break is an "element" in the palette that is really a band key with its own selection, excluded from bulk edits and group moves. Two more of those is a smaller conceptual step than a new nesting affordance.
3. **It matches the engine's rigid-block vocabulary.** Section Break already established "the block below a line moves as one, each member keeping its offset from the line". A loop is that same machinery run *N* times.

## The structural tension the spec must resolve

A loop is reflow, and folio8's stated rule is that **nothing moves because a neighbour grew**, with exactly two deliberate exceptions. Iteration *N+1* sits below iteration *N* at a data-dependent offset, and everything below Loop End is pushed down by the total growth. That makes the loop the **third** exception, and the largest: the section break's push is a rigid block moved once, while a loop's push is *repeated* and its magnitude depends on the data.

Three sub-problems follow directly, and every surveyed product had to answer all three:

- **Variable-height iterations.** If the slab contains a table, iterations are not all the same height. Banded engines handle this because a band's height is derived per instance; a fixed-height slab would forbid it. Whether folio8's first loop allows a growing element inside the slab is a real scope fork.
- **Iterations across a page boundary.** Banded engines and SSRS default to keeping a detail instance together and pushing a straddling one to the next page. folio8 already owns both halves of this answer — `keepTogether` semantics, and the "clip with a warning rather than refuse, when the height came from the data" rule.
- **The empty array.** Zero iterations means the slab collapses and everything below it moves **up**. folio8 has so far refused every upward move (an unanchored section is never pulled up on page 1), so "collapse to nothing" versus "reserve the slab" is a decision the format has not yet made in any form.

## Sources

- [Introduction to Banded Reports — DevExpress](https://docs.devexpress.com/XtraReports/2587/feature-guide-to-devexpress-reports/introduction-to-banded-reports)
- [How to make a repeating band — Fast Reports](https://www.fast-report.com/blogs/making-repeating-band)
- [FastReport.NET User's Manual (band-relative positioning, object Anchor)](https://www.fast-report.com/public_download/FRNetUserManual-en.pdf)
- [Reports Designer — Stimulsoft](https://www.stimulsoft.com/manuals/en/user-manual/reports_designer.htm)
- [Create invoices and forms with lists in a paginated report — Microsoft Learn](https://learn.microsoft.com/en-us/sql/reporting-services/report-design/create-invoices-and-forms-with-lists-report-builder-and-ssrs)
- [Tablix data region in a paginated report — Microsoft Learn](https://learn.microsoft.com/en-us/sql/reporting-services/report-design/tablix-data-region-report-builder-and-ssrs)
- [Basics of SSRS Tablix Data Region — Bold Reports](https://www.boldreports.com/blog/basics-of-ssrs-tablix-data-region/)
- [Using Designer, AEM 6.3 Forms (repeating subforms, Min Count)](https://helpx.adobe.com/pdf/aem-forms/6-3/using-designer.pdf)
- [Repeat subforms for each data item — Adobe Experience League Community](https://experienceleaguecommunities.adobe.com/t5/adobe-experience-manager-forms/repeat-subforms-for-each-data-item/td-p/373026)
- [Types of tags — Docxtemplater](https://docxtemplater.com/docs/tag-types/)
- [Repeating Section content control — TemplioniX](https://docs.templionix.com/word-template-guide/types-of-content-controls/repeating-section)
