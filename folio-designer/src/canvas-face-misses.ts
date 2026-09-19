/**
 * A CANVAS MISS SUBSTITUTES AND SAYS SO ONCE
 * (spec-deferred-offline-cache, story 2 — owner decision, 2026-09-19).
 *
 * WHAT CHANGED AND WHY THIS EXISTS. The canvas paints document text with the
 * CSS `@font-face` rules in `generated/runtime-fonts.css`, and since story 2
 * the faces behind some of those rules are DEFERRED: the release still carries
 * them, the service worker no longer precaches them, and each arrives the first
 * time something asks. So a face can now fail to arrive — offline, or bytes
 * that will not verify against the release's own digest — and when it does the
 * browser silently substitutes another and paints on.
 *
 * `loadingerror` FIRES FOR TWO CAUSES AND THE SENTENCE MUST COVER BOTH. The
 * bytes may never have arrived, or they may have arrived and failed to parse as
 * a font. The author's remedy and the consequence are identical either way, and
 * a message naming only the first would be wrong half the time — so the copy in
 * `App.tsx` says the face could not be LOADED rather than could not be fetched.
 *
 * WHAT IS NOT AT STAKE, WHICH IS MOST OF IT. The ENGINE is unaffected:
 * `folio-go/fonts/fonts.go` embeds its own copies of the shipped faces, so
 * metrics, line breaks, pagination and the previewed PDF are exactly what they
 * would have been. Only the glyphs drawn on this screen differ. That is why the
 * author gets a warning and never a block: a modal over a document that is
 * laying out correctly would misreport the severity of a cosmetic degradation.
 *
 * THIS IS NOT MEASUREMENT, AND THAT IS THE WHOLE OF ITS RELATIONSHIP TO AD-17.
 * `document.fonts` is prohibited across this designer because the browser may
 * not be an authority on how text is laid out. Nothing here reads a width, a
 * height, an advance or a line count; it subscribes to the font set's own
 * report that a face did not load, reads the family NAME out of it, and hands
 * that name to the caller. `canvas-authority-contract.test.ts` waives
 * `document.fonts` inside this one function, by name, and every other
 * prohibition — `getComputedStyle`, `offset*`, `ResizeObserver`,
 * `devicePixelRatio`, the pagination-from-paint rules — still applies here.
 *
 * NO FACE IS REGISTERED HERE. `new FontFace` does not appear in this module and
 * must not: `embedded-face-registry.ts` is still the one seam that may add a
 * face to the page at runtime.
 */
export function watchCanvasFaceMisses(missed: (families: ReadonlyArray<string>) => void): () => void {
  const fontSet: FontFaceSet | undefined = typeof document === 'undefined' ? undefined : document.fonts
  // A browser with no font set, or a test environment that stubs a partial one,
  // simply reports nothing. An absent warning is the safe direction: the canvas
  // is degraded either way, and inventing a family name would be worse.
  if (!fontSet || typeof fontSet.addEventListener !== 'function') return () => {}
  const onError = (event: Event) => {
    const failed = (event as FontFaceSetLoadEvent).fontfaces ?? []
    // A CSS family name arrives quoted; the author never typed the quotes.
    //
    // AND ONE EVENT CAN CARRY SEVERAL CUTS OF ONE FAMILY — a Regular and a Bold
    // of the same `@font-face` family fail together — so the families are
    // deduped HERE, where the duplication is created. A caller keying rows by
    // family would otherwise be handed the same name twice in one call.
    const families = [...new Set(failed.map((face) => face.family.replace(/^["']|["']$/g, '')).filter((family) => family !== ''))]
    if (families.length > 0) missed(families)
  }
  fontSet.addEventListener('loadingerror', onError)
  return () => fontSet.removeEventListener('loadingerror', onError)
}
