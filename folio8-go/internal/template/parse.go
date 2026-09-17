package template

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/panitw/folio8/folio8-go/internal/geom"
)

// This file is the parser (AD-9). json.Decoder + UseNumber decodes every
// leaf number as a preserved literal (AC26); the two numeric kinds
// (points, nextId) are converted through decimal.go's exact path
// (AC24). Closed-set validation (closedsets.go) is kept visibly
// separate from unknown-key passthrough (rawvalue.go, decodehelpers.go)
// — AC10, AC11 clause 3. Every load error is a *LoadError naming
// field, element id and value (AC41). Nothing is ever coerced (AC40).

// parseCtx threads document-wide id bookkeeping through the recursive
// descent (AC35: a duplicate id anywhere is a load error, across bands,
// elements and columns alike — the counter is document-wide, AC36).
type parseCtx struct {
	ids        map[string]int64 // id string -> decoded counter
	maxCounter int64
}

func newParseCtx() *parseCtx {
	return &parseCtx{ids: map[string]int64{}}
}

// claimID validates id's spelling (AC34), rejects a duplicate anywhere
// in the document (AC35), and records it toward AC37's nextId check.
func (c *parseCtx) claimID(field, id string) (ElementID, error) {
	counter, err := validateElementID(id)
	if err != nil {
		return "", newLoadError(field, id, id, err.Error())
	}
	if _, dup := c.ids[id]; dup {
		return "", newLoadError(field, id, id, "duplicate id: ids are unique document-wide (AD-10, AC35, AC36)")
	}
	c.ids[id] = counter
	if counter > c.maxCounter {
		c.maxCounter = counter
	}
	return ElementID(id), nil
}

// ParseDocument parses b into a canonicalised Document (AC4). It is the
// sole entry point internal/template exposes for parsing; package
// folio8's ParseTemplate/LoadTemplate (folio8-go/folio8.go) call this.
func ParseDocument(b []byte) (*Document, error) {
	top, err := decodeObjectMap(json.RawMessage(b))
	if err != nil {
		return nil, fmt.Errorf("template: root: %w", err)
	}

	ctx := newParseCtx()
	doc := &Document{}
	consumed := map[string]bool{}

	// version (AC6, AC7)
	verRaw, ok := top["version"]
	if !ok {
		return nil, newLoadError("version", "", "", "missing required field")
	}
	consumed["version"] = true
	version, err := decodeStringRaw(verRaw)
	if err != nil {
		return nil, newLoadError("version", "", string(verRaw), "must be a string: "+err.Error())
	}
	if err := checkVersionLoadable(version); err != nil {
		return nil, err
	}
	doc.Version = version

	// locale (AD-12, AC5)
	if raw, ok := top["locale"]; ok {
		consumed["locale"] = true
		s, err := decodeStringRaw(raw)
		if err != nil {
			return nil, newLoadError("locale", "", string(raw), "must be a string: "+err.Error())
		}
		if !IsLocale(s) {
			// DERIVED, NEVER HAND-WRITTEN (Story 12.2). The tail used to be a
			// literal listing the four tags, which made this the SECOND place
			// the closed set was spelled for an author to read — the command
			// door derives its own list from LocaleTags, and a set that gained
			// or lost a member would have shipped two refusals disagreeing
			// about what is legal. closedSetMessage over LocaleTags produces
			// byte-identical output to the literal it replaced, so no test and
			// no golden moves.
			return nil, newLoadError("locale", "", s, closedSetMessage(LocaleTags)+" (AD-12)")
		}
		doc.Locale = s
	} else {
		return nil, newLoadError("locale", "", "", "missing required field")
	}

	// utcOffset
	if raw, ok := top["utcOffset"]; ok {
		consumed["utcOffset"] = true
		s, err := decodeStringRaw(raw)
		if err != nil {
			return nil, newLoadError("utcOffset", "", string(raw), "must be a string: "+err.Error())
		}
		if !IsUTCOffset(s) {
			return nil, newLoadError("utcOffset", "", s, "must match "+UTCOffsetSyntax)
		}
		doc.UTCOffset = s
	} else {
		return nil, newLoadError("utcOffset", "", "", "missing required field")
	}

	// page
	if raw, ok := top["page"]; ok {
		consumed["page"] = true
		p, err := decodePage(raw)
		if err != nil {
			return nil, err
		}
		doc.Page = p
	} else {
		return nil, newLoadError("page", "", "", "missing required field")
	}

	// assets — DECODED BEFORE fonts, and the order is load-bearing
	// (Story 8.3). A chain entry may be `{"asset": "<key>"}`, and
	// decodeFonts refuses a key that names no asset; it therefore needs
	// the decoded map to consult. Decoding fonts first would leave that
	// refusal with nothing to look in, and the natural "fix" — skipping
	// the check — is exactly the load error the format promises.
	//
	// The visible consequence: a document that is wrong in BOTH places
	// reports its assets error first. That is the right way round —
	// fonts.<name>[i].asset naming an absent key is not diagnosable
	// until the assets map is known to be well formed.
	if raw, ok := top["assets"]; ok {
		consumed["assets"] = true
		assets, err := decodeAssets(raw)
		if err != nil {
			return nil, err
		}
		doc.Assets = assets
	} else {
		doc.Assets = map[string]Asset{}
		consumed["assets"] = true
	}

	// fonts — see the assets block above for why it decodes second.
	if raw, ok := top["fonts"]; ok {
		consumed["fonts"] = true
		f, err := decodeFonts(raw, doc.Assets)
		if err != nil {
			return nil, err
		}
		doc.Fonts = f
	} else {
		doc.Fonts = Fonts{}
		consumed["fonts"] = true
	}

	// bands
	if raw, ok := top["bands"]; ok {
		consumed["bands"] = true
		bands, err := decodeBands(ctx, raw)
		if err != nil {
			return nil, err
		}
		doc.Bands = bands
	} else {
		return nil, newLoadError("bands", "", "", "missing required field")
	}

	// pages (SPEC-multi-pages, D-G.1) — optional; decoded after bands so
	// its ids are claimed after the three bands', through the same ctx.
	if raw, ok := top[pagesKey]; ok {
		consumed[pagesKey] = true
		pages, err := decodePages(ctx, raw, doc.Bands.Content)
		if err != nil {
			return nil, err
		}
		doc.Pages = pages
	}

	// unbreakableValues (Story 2.4; D-2.1.6 OWNER, D-2.4.1) — optional,
	// a list of bare root-relative dotted data paths. Absent stays
	// absent: the field is not defaulted to an empty list, because a
	// document that never declared it must round-trip WITHOUT the key
	// (the canonical-fixed-point property Story 1.4 pins).
	if raw, ok := top["unbreakableValues"]; ok {
		consumed["unbreakableValues"] = true
		paths, err := decodeUnbreakableValues(raw)
		if err != nil {
			return nil, err
		}
		doc.UnbreakableValues = paths
	}

	// nextId (AC32, AC33, AC37) — must be a plain decimal integer, never
	// base 36.
	nextRaw, ok := top["nextId"]
	if !ok {
		return nil, newLoadError("nextId", "", "", "missing required field (never repaired, never inferred, AD-10/AC37)")
	}
	consumed["nextId"] = true
	nextID, err := decodePlainInteger(nextRaw)
	if err != nil {
		return nil, newLoadError("nextId", "", string(nextRaw), "must be a plain decimal integer: "+err.Error())
	}
	if nextID <= ctx.maxCounter {
		return nil, newLoadError("nextId", "", fmt.Sprintf("%d", nextID),
			fmt.Sprintf("must be greater than the highest id present (%d) — never repaired, never renumbered (AD-10, AC37)", ctx.maxCounter))
	}
	doc.NextID = nextID

	extra, err := extraFields(top, consumed)
	if err != nil {
		return nil, fmt.Errorf("template: root: %w", err)
	}
	doc.Extra = extra

	return doc, nil
}

// decodePlainInteger decodes a JSON number literal that must be a plain
// decimal integer (no '.', no exponent) — AC32's nextId spelling.
func decodePlainInteger(raw json.RawMessage) (int64, error) {
	n, err := decodeNumberRaw(raw)
	if err != nil {
		return 0, err
	}
	return n.Int64()
}

func decodePage(raw json.RawMessage) (Page, error) {
	obj, err := decodeObjectMap(raw)
	if err != nil {
		return Page{}, fmt.Errorf("template: page: %w", err)
	}
	var p Page
	consumed := map[string]bool{"margin": true, "orientation": true, "size": true}

	marginRaw, ok := obj["margin"]
	if !ok {
		return Page{}, newLoadError("page.margin", "", "", "missing required field")
	}
	m, err := decodeMargin(marginRaw)
	if err != nil {
		return Page{}, err
	}
	p.Margin = m

	orRaw, ok := obj["orientation"]
	if !ok {
		return Page{}, newLoadError("page.orientation", "", "", "missing required field")
	}
	or, err := decodeStringRaw(orRaw)
	if err != nil {
		return Page{}, newLoadError("page.orientation", "", string(orRaw), "must be a string: "+err.Error())
	}
	if !closedPageOrientations[or] {
		return Page{}, newLoadError("page.orientation", "", or, "not one of the closed set portrait, landscape")
	}
	p.Orientation = or

	sizeRaw, ok := obj["size"]
	if !ok {
		return Page{}, newLoadError("page.size", "", "", "missing required field")
	}
	if s, err := decodeStringRaw(sizeRaw); err == nil {
		if !closedPageSizeNames[s] {
			return Page{}, newLoadError("page.size", "", s, `not one of the closed set "A4", "Letter", or an object with height/width`)
		}
		p.SizeIsName = true
		p.SizeName = s
	} else {
		sizeObj, oerr := decodeObjectMap(sizeRaw)
		if oerr != nil {
			return Page{}, newLoadError("page.size", "", string(sizeRaw), `must be "A4", "Letter", or an object with height/width`)
		}
		hRaw, hok := sizeObj["height"]
		wRaw, wok := sizeObj["width"]
		if !hok || !wok {
			return Page{}, newLoadError("page.size", "", string(sizeRaw), "custom size object must carry both height and width")
		}
		h, err := decodePointsRaw("page.size.height", "", hRaw)
		if err != nil {
			return Page{}, err
		}
		w, err := decodePointsRaw("page.size.width", "", wRaw)
		if err != nil {
			return Page{}, err
		}
		if len(sizeObj) != 2 {
			return Page{}, newLoadError("page.size", "", string(sizeRaw), "custom size object must carry exactly height and width")
		}
		p.SizeCustom = PageSize{Height: h, Width: w}
	}

	extra, err := extraFields(obj, consumed)
	if err != nil {
		return Page{}, fmt.Errorf("template: page: %w", err)
	}
	p.Extra = extra

	return p, nil
}

func decodeMargin(raw json.RawMessage) (Margin, error) {
	obj, err := decodeObjectMap(raw)
	if err != nil {
		return Margin{}, fmt.Errorf("template: page.margin: %w", err)
	}
	var m Margin
	consumed := map[string]bool{}
	for _, kv := range []struct {
		key string
		dst *geom.Length
	}{
		{"top", &m.Top}, {"right", &m.Right}, {"bottom", &m.Bottom}, {"left", &m.Left},
	} {
		r, ok := obj[kv.key]
		if !ok {
			return Margin{}, newLoadError("page.margin."+kv.key, "", "", "missing required field")
		}
		consumed[kv.key] = true
		v, err := decodePointsRaw("page.margin."+kv.key, "", r)
		if err != nil {
			return Margin{}, err
		}
		*kv.dst = v
	}
	extra, err := extraFields(obj, consumed)
	if err != nil {
		return Margin{}, fmt.Errorf("template: page.margin: %w", err)
	}
	m.Extra = extra
	return m, nil
}

// decodeFonts walks each chain array ITSELF rather than delegating to
// decodeStringArrayRaw, and that is the whole reason it is written out
// (Story 8.3). The delegating version collapsed every defect in a chain
// into ONE error at field path `fonts.<name>`, with no entry index —
// so an author with a nine-entry chain was told the chain was wrong and
// left to find which entry. Every refusal below names
// `fonts.<name>[<i>]`, and the asset refusal names
// `fonts.<name>[<i>].asset`.
//
// Those field paths are load-bearing rather than cosmetic. No new
// diagnostic code is minted here (D-7.8.1: a specific code is minted
// only when a named consumer must BRANCH on it, and none does) — every
// refusal is newLoadError's TEMPLATE_FIELD_INVALID, and what tells the
// author WHICH entry is wrong is the Field, which is why the index has
// to be in it.
//
// assets is the already-decoded asset map: ParseDocument decodes assets
// BEFORE fonts precisely so this function can consult it. A nil map is
// not a special case — an absent-key lookup in one is simply always
// false, which is the correct answer for a document with no assets.
func decodeFonts(raw json.RawMessage, assets map[string]Asset) (Fonts, error) {
	obj, err := decodeObjectMap(raw)
	if err != nil {
		return nil, fmt.Errorf("template: fonts: %w", err)
	}
	out := Fonts{}
	for _, k := range slices.Sorted(maps.Keys(obj)) {
		v := obj[k]
		var raws []json.RawMessage
		if err := json.Unmarshal(v, &raws); err != nil {
			// NOT "an array of strings" — that wording predates Story
			// 8.3 and became false the moment an entry could be an
			// object. A refusal that names a legal shape the format no
			// longer has sends the author to fix the one thing that was
			// not wrong. The per-ENTRY refusals below name both shapes;
			// this one is only reached when the chain is not an array at
			// all, so there is no entry to locate and no index to give.
			return nil, newLoadError("fonts."+k, "", string(v), "must be an array of font chain entries: "+err.Error())
		}
		chain := make([]FontChainEntry, 0, len(raws))
		for i, entryRaw := range raws {
			entry, err := decodeFontChainEntry(entryRaw, fmt.Sprintf("fonts.%s[%d]", k, i), assets)
			if err != nil {
				return nil, err
			}
			chain = append(chain, entry)
		}
		out[k] = chain
	}
	return out, nil
}

// quotedKeyList spells a key enumeration for a refusal message —
// `"bold", "italic", "boldItalic"` — so no message hand-writes a set the
// parser no longer enforces.
func quotedKeyList(keys []string) string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = `"` + k + `"`
	}
	return strings.Join(out, ", ")
}

// fontChainEntryGrammar spells the object form's whole legal key set,
// DERIVED from the closed enumeration in model.go rather than written
// out here.
//
// Story 8.3's own note is the reason: "unpinned wording in a refusal is
// wording that goes stale silently and sends the author to fix the one
// thing that was not wrong." Three refusals in this function used to
// hand-write "exactly one key, \"asset\"", and all three became false
// the moment the key set widened. Deriving them means the set can only
// be widened in one place.
func fontChainEntryGrammar() string {
	return `{"face": "<face name>"} or {"asset": "<assets key>"} — exactly one of those two — optionally carrying the style-variant siblings ` +
		quotedKeyList(fontChainVariantKeys()) +
		`, each naming a face of the SAME kind as the entry itself (a face name beside "face", an assets key beside "asset") and no other key`
}

// THE THREE OBJECT-SHAPE REFUSALS SAY WHICH DEFECT THIS IS. They share
// the grammar sentence above — one authority for what is legal — and
// each opens with the specific thing that is wrong, because "here is the
// grammar" alone leaves an author with an unrecognised key, an author
// with both discriminants and an author with neither reading the same
// sentence and guessing which of the three they are. The pre-11.2
// message was narrower but at least said "exactly one key, \"asset\"".
//
// Each is DERIVED from the closed enumerations, never hand-written: the
// legal key set can only be widened in model.go, and every sentence
// below moves with it.

// noDiscriminantReason: the object names no KIND at all.
func noDiscriminantReason() string {
	return `a font chain entry object must name what kind of face it is: exactly one of "face" or "asset", and this object has neither` +
		` (a sibling such as ` + quotedKeyList(fontChainVariantKeys()) + ` decorates an entry and cannot stand alone).` +
		` The legal shape is ` + fontChainEntryGrammar() + `; a plain face name is written as a string`
}

// bothDiscriminantsReason: the object names TWO kinds at once.
func bothDiscriminantsReason() string {
	return `a font chain entry object must name exactly one kind of face, and this object carries BOTH "face" and "asset".` +
		` A face the renderer supplies and a face the document carries are different faces and neither stands in for the other (AD-8).` +
		` The legal shape is ` + fontChainEntryGrammar()
}

// unknownEntryKeyReason: the kind is clear and a key is not in the set.
// It NAMES THE OFFENDING KEY, which is why decodeFontChainEntry visits
// the object's keys in sorted order — an author with two unrecognised
// keys is always sent to the same one.
func unknownEntryKeyReason(key string) string {
	return `"` + key + `" is not a key a font chain entry may carry. The key set is CLOSED: ` +
		quotedKeyList(append([]string{"face", "asset"}, fontChainVariantKeys()...)) +
		` and nothing else — an unknown key is an entry of an unknown kind, never a known entry with an unknown decoration.` +
		` The legal shape is ` + fontChainEntryGrammar()
}

// selfReferentialVariantReason: the sibling names the entry's OWN base, on
// either arm (D-11.2.11 / DW-241).
//
// It joins the shared grammar like its three siblings above, and it DERIVES
// the key enumeration rather than spelling it: the three keys appear in this
// sentence only through quotedKeyList, so a set the parser no longer enforces
// cannot be named by a refusal.
//
// IT NAMES THE ONE AXIS, because the near-miss is the thing a reader will get
// wrong. The base is PRIVILEGED and it is the only comparison made: two
// DIFFERENT variants of one entry naming the same face —
// `{"face":"Roboto","bold":"X","italic":"X"}` — is legal and silent, and this
// narrowing must never quietly become "no two variants may agree".
//
// ⚠ THE CLAUSE ORDER IS LOAD-BEARING AND IT IS MEASURED, NOT ARRANGED BY TASTE.
// LoadError.Error() cuts Reason at loadErrorReasonRunes (256) with a `…`, and
// the first draft of this sentence was 917 runes: the rendered message an
// author actually reads stopped mid-clause, BEFORE the remedy and BEFORE "only
// the base is privileged" — the one sentence D-11.2.11 requires, without which
// the narrowing reads as "no two variants may agree", which is wider than what
// was ruled. So the claim, the remedy and the one-axis clause come FIRST, in
// 245 runes; the elaboration and the shared grammar follow and are the parts
// the cut may take. TestASelfReferentialVariantSaysWHYAndSaysItFromTheCLOSEDSet
// asserts on Error(), not on Reason, because Reason is not what the author sees.
func selfReferentialVariantReason() string {
	return `a style variant may not name the entry's OWN base. ` + quotedKeyList(fontChainVariantKeys()) +
		` name the face drawn INSTEAD of the base, so one naming the base declares no cut, silently.` +
		` Write no key at all, or name the cut's face; only the base is privileged` +
		` — two DIFFERENT variants of one entry may name the same face, and that is legal (D-11.2.11).` +
		` "The entry's OWN base" is the face name the entry declares, or its assets key when the entry is embedded.` +
		` An entry that DECLARES a variant is not an entry that declares none, so the warning that announces a missing cut never fires.` +
		` The legal shape is ` + fontChainEntryGrammar()
}

// decodeFontChainEntry decodes ONE chain entry at field, which already
// carries the chain name and the index.
//
// The shape is decided from the raw JSON's first non-space byte rather
// than by trying each decode in turn: `"` is a face name, `{` is an
// object entry, and anything else is refused with BOTH legal shapes
// named, so the message says what the author may write instead of only
// what they may not.
//
// STORY 11.2 REPLACED "EXACTLY ONE KEY" WITH "EXACTLY ONE OF face|asset,
// OVER A CLOSED SET". The one-key rule was the MECHANISM; the property
// stated below — an unknown key cannot ride along disguised as a
// decoration — is what it bought, and an exactly-one-of discriminant
// over a closed set preserves that property exactly. Nothing here is
// passthrough, and nothing is inferred from a face NAME: a sibling is
// read, never parsed, and never constructed.
func decodeFontChainEntry(raw json.RawMessage, field string, assets map[string]Asset) (FontChainEntry, error) {
	trimmed := strings.TrimSpace(string(raw))
	switch {
	case strings.HasPrefix(trimmed, `"`):
		face, err := decodeStringRaw(raw)
		if err != nil {
			return FontChainEntry{}, newLoadError(field, "", trimmed, "must be a string: "+err.Error())
		}
		if face == "" {
			return FontChainEntry{}, newLoadError(field, "", trimmed, "a font chain entry must name a face — an empty string names none")
		}
		return FaceEntry(face), nil

	case strings.HasPrefix(trimmed, "{"):
		entryObj, err := decodeObjectMap(raw)
		if err != nil {
			return FontChainEntry{}, newLoadError(field, "", trimmed, "must be an object of the form "+fontChainEntryGrammar()+": "+err.Error())
		}
		// EXACTLY ONE OF face|asset, BOTH DIRECTIONS, AND THE KEY SET
		// STAYS CLOSED. A missing discriminant, both discriminants and
		// an unrecognised key are the same defect — the object IS the
		// entry's kind, so a key outside the closed set is an entry of
		// an unknown kind, not a known entry with an unknown
		// decoration, and D-1.4.9's passthrough does not reach here.
		faceRaw, hasFace := entryObj["face"]
		assetRaw, hasAsset := entryObj["asset"]
		switch {
		case hasFace && hasAsset:
			return FontChainEntry{}, newLoadError(field, "", trimmed, bothDiscriminantsReason())
		case !hasFace && !hasAsset:
			return FontChainEntry{}, newLoadError(field, "", trimmed, noDiscriminantReason())
		}
		// SORTED, never a bare map range (AD-1 / NFR1.d): the key this
		// refusal NAMES reaches an output, so an author with two
		// unrecognised keys is always sent to the same one.
		for _, key := range slices.Sorted(maps.Keys(entryObj)) {
			if key == "face" || key == "asset" {
				continue
			}
			if !slices.Contains(fontChainVariantKeys(), key) {
				return FontChainEntry{}, newLoadError(field, "", trimmed, unknownEntryKeyReason(key))
			}
		}

		var entry FontChainEntry
		if hasFace {
			face, ferr := decodeStringRaw(faceRaw)
			if ferr != nil {
				return FontChainEntry{}, newLoadError(field+".face", "", string(faceRaw), "must be a string naming a face the renderer supplies: "+ferr.Error())
			}
			if face == "" {
				return FontChainEntry{}, newLoadError(field+".face", "", string(faceRaw), "must name a face — an empty string names none")
			}
			entry.Face = face
		} else {
			key, kerr := decodeStringRaw(assetRaw)
			if kerr != nil {
				return FontChainEntry{}, newLoadError(field+".asset", "", string(assetRaw), "must be a string: "+kerr.Error())
			}
			if key == "" {
				return FontChainEntry{}, newLoadError(field+".asset", "", string(assetRaw), "must name an assets key — an empty string names none")
			}
			asset, present := assets[key]
			if !present {
				return FontChainEntry{}, newLoadError(field+".asset", "", key,
					"names no entry in the document's assets map — an embedded face must be carried by the document that references it")
			}
			if lerr := requireEmbeddedFaceLicence(asset, key, field); lerr != nil {
				return FontChainEntry{}, lerr
			}
			entry.AssetKey = key
		}

		// THE SIBLINGS, IN THE CLOSED SET'S OWN FIXED ORDER — so a
		// document with two defective siblings is always refused at the
		// same one, whatever order the file happened to write them in.
		for _, v := range fontChainVariants {
			sibRaw, ok := entryObj[v.key]
			if !ok {
				continue
			}
			sibField := field + "." + v.key
			name, serr := decodeStringRaw(sibRaw)
			if serr != nil {
				// THE MESSAGE SAYS WHICH MEANING APPLIES HERE. `bold` is
				// a boolean on `style` and a FACE on a chain entry, and
				// an author who writes `"bold": true` here has reached
				// for the other one.
				return FontChainEntry{}, newLoadError(sibField, "", string(sibRaw),
					"must be a string naming a face: on a font chain entry \""+v.key+"\" is the FACE drawn at that weight and slope, not a boolean — the boolean of the same name lives on style")
			}
			if name == "" {
				return FontChainEntry{}, newLoadError(sibField, "", string(sibRaw), "an empty string names no face")
			}
			// D-11.2.11 / DW-241: A VARIANT MAY NOT NAME ITS OWN BASE.
			//
			// ONE PREDICATE PER ARM, AGAINST THAT ARM'S OWN DISCRIMINANT
			// VALUE. Both `entry.Face` and `entry.AssetKey` are already set
			// above — before this loop — which is what lets one check site
			// serve a `face` entry and an `asset` entry without either arm
			// growing its own copy of it.
			//
			// IT SITS AFTER THE EMPTY CHECK AND BEFORE THE NAMESPACE BLOCK,
			// deliberately: a self-reference is trivially IN namespace, so the
			// namespace check below can never fire on one, and running it
			// first would also make requireEmbeddedFaceLicence run a second
			// time over the asset this entry already cleared.
			//
			// ⚠ ITS LIMIT, STATED HERE BECAUSE THIS IS WHERE A READER MEETS
			// THE CHECK (DW-245). This is STRING EQUALITY against the entry's
			// own discriminant value, and that is all it can be. It cannot see
			// `{"face":"Roboto","bold":"Roboto Copy"}` where two distinct
			// FontSet keys happen to hold IDENTICAL BYTES — that renders
			// bold-as-regular just as silently, and the entry looks entirely
			// well-formed. Detecting it would mean comparing the two faces'
			// CONTENTS, and D-11.2.1 forbids the engine to resolve or compare
			// faces by anything read out of the binaries. So the failure is
			// closed for the case an author reaches by mistake and open for
			// the case an author reaches by a stranger route; a check whose
			// limit is unstated ages into a false reassurance.
			own := entry.Face
			if entry.Embedded() {
				own = entry.AssetKey
			}
			if name == own {
				return FontChainEntry{}, newLoadError(sibField, "", name, selfReferentialVariantReason())
			}
			// AD-8's NAMESPACE MATCH, BOTH DIRECTIONS. A sibling of a
			// `face` entry is a FontSet face name; a sibling of an
			// `asset` entry is an assets key. A cross-namespace sibling
			// is the substitution AD-8 forbids, arriving inside a single
			// entry where no precedence rule can see it.
			_, namesAnAsset := assets[name]
			if entry.Embedded() {
				if !namesAnAsset {
					return FontChainEntry{}, newLoadError(sibField, "", name,
						"names no entry in the document's assets map — an embedded entry's style variants are assets keys, never face names (AD-8)")
				}
				// AD-26 / I-7: a variant asset key IS an embedded face,
				// so it clears the licence requirement exactly as the
				// `asset` value does. A sibling that skipped it would
				// let a document carry an unlicensed embedded bold.
				if lerr := requireEmbeddedFaceLicence(assets[name], name, sibField); lerr != nil {
					return FontChainEntry{}, lerr
				}
			} else if namesAnAsset {
				return FontChainEntry{}, newLoadError(sibField, "", name,
					"names an entry in the document's assets map — a face entry's style variants are FontSet face names, never assets keys; write the whole entry as {\"asset\": …} if the document carries these faces (AD-8)")
			}
			*v.field(&entry) = name
		}
		return entry, nil

	default:
		return FontChainEntry{}, newLoadError(field, "", trimmed,
			"a font chain entry is either a face name (a string) or the object "+fontChainEntryGrammar())
	}
}

// requireEmbeddedFaceLicence is Story 8.6's load refusal: AN ASSET A CHAIN
// NAMES MUST STATE ITS TERMS.
//
// A font that travels without its terms is not a font that may be passed on,
// so a `.folio` carrying an embedded face with no licence identifier, no
// licence text or no copyright is refused when it is opened — never warned
// about, never rendered best-effort. The owner settled this on 2026-09-02:
// folio8 is unreleased and no `.folio` documents exist, so the format is made
// right here rather than softened to spare files that do not exist.
//
// THE RULE IS SCOPED TO REFERENCE, AND THAT SCOPE IS LOAD-BEARING. It is not
// "a font asset must carry a licence"; it is "an asset a chain names by
// {"asset": key} must". An UNREFERENCED font asset is not an embedded face —
// nothing draws with it, nothing redistributes a face on its account — and it
// stays legal, `font` record absent or partial, which is what keeps D-1.4.13
// intact: such an asset must not raise the document to 2.0 and must not raise
// an error either. Hence this is called from decodeFontChainEntry, at the one
// point where an asset and the entry naming it are both in hand.
//
// THE ERROR LOCATES BOTH HALVES because either one alone sends the reader to
// the wrong place. `assets.<key>.font.licenceText` says which record is short
// but not why that matters; `fonts.<chain>[<i>]` says which entry made it
// matter but not what to write. The Field is the asset's record — that is
// where the fix goes — and the message names the chain entry that makes the
// asset an embedded face.
//
// EVERY GUARD IS WRITTEN FOR THREE STATES, not two. Presence distinguishes
// absent, explicit null and set (presence.go), and a check written only for
// the absent case would admit `"licenceText": null` — a document that says,
// in as many words, that it has no terms. Empty is refused for the same
// reason: `""` is a key that is present and states nothing.
//
// AND BLANK IS EMPTY. The test is TrimSpace, not `== ""`, because `" "` and
// `"\n"` are keys that state exactly as much as `""` does — nothing — while
// passing a length check. A rule that can be satisfied by a space is a rule
// about typing rather than about terms.
//
// ---------------------------------------------------------------------------
// THE ONE-DOOR ASYMMETRY IS DELIBERATE, AND IT IS NOT A GAP TO TIDY UP
// (D-16.R.11).
//
// This function requires that an embedded face STATE its terms. It does not ask
// whether the bytes AGREE with the terms stated. That second question — the
// licence tie — is asked at exactly one door, `fontset.RefuseContradictedLicence`
// called from `embedFontFamily` in `component_commands.go`, so a document
// written by this product cannot carry a face whose own `name` table
// contradicts its declared licence. A document HAND-AUTHORED with a
// contradictory pair still loads.
//
// THE SECOND DOOR IS DEFERRED PAST `folio8-go/v1.0.0`, AND NO MIGRATION IS OWED.
// Registered with its trigger; the deferral is a ruling, not an oversight, and
// the reasoning is short enough to keep here so the next reader does not close
// the gap as a tidy-up and thereby ship an unpriced narrowing:
//
//   - It is a DIFFERENT CLASS from the `fvar` case that D-16.6 ruled "both, or
//     halt" on. `fvar`'s guard already HAD two doors and its defect was a
//     document that saves cleanly and fails at RENDER. Here the bytes can be
//     honoured, the render is correct, and what is wrong is a label the
//     hand-author wrote themselves. "Both, or halt" governs REMOVING a guard,
//     not declining to duplicate a new one.
//   - The deadline does not bite, because the migration prices at nothing.
//     Under the three-valued tie (D-16.R.7) silence ADMITS, so the only
//     documents this narrowing could ever break are documents whose font bytes
//     CONTRADICT their declared licence — measured at 1.0% upstream, and the
//     product's own writer can no longer produce one. The "migration" is a
//     located diagnostic saying the document declares terms its own font bytes
//     contradict: a refusal telling the truth, not a compatibility break.
//
// ⚠ THE TRIGGER IS A CONDITION, NOT A DATE, and it is the reason this paragraph
// exists rather than a backlog line. Widening the tie's ADMIT side stays free
// forever. ADDING A REFUSE-SIGNATURE AFTER THE TAG IS ITSELF THE NARROWING,
// because a face that admits today as NO EVIDENCE becomes a CONTRADICTION
// tomorrow. So: close this second door before any refuse-signature is added to
// `internal/fontset/licencesignature.go` after `folio8-go/v1.0.0` is cut, or
// accept that the addition is itself the narrowing and price it then. Owner:
// whoever adds that signature. folio8-go/v1.0.0 shipped with this door still
// open.
//
// AND THE ONE DOOR THAT DOES EXIST MUST STAY PROVABLY PRESENT: Story 16.1b's
// test that a contradiction is refused at the command may not be deleted on the
// reasoning that the load path covers it. It does not.
func requireEmbeddedFaceLicence(asset Asset, key, entryField string) error {
	missing := func(name string) error {
		return newLoadError("assets."+key+".font."+name, "", "",
			"an embedded face must state its terms: the chain entry "+entryField+
				" names this asset, so its font record requires a non-empty licence, licenceText and copyright — a font that travels without its terms is not a font that may be passed on")
	}
	if !asset.Font.Set || asset.Font.Null {
		return missing("licence")
	}
	for _, required := range []struct {
		name  string
		value Presence[string]
	}{
		{"licence", asset.Font.Value.Licence},
		{"licenceText", asset.Font.Value.LicenceText},
		{"copyright", asset.Font.Value.Copyright},
	} {
		if !required.value.Set || required.value.Null || strings.TrimSpace(required.value.Value) == "" {
			return missing(required.name)
		}
	}
	return nil
}

func decodeAssets(raw json.RawMessage) (map[string]Asset, error) {
	obj, err := decodeObjectMap(raw)
	if err != nil {
		return nil, fmt.Errorf("template: assets: %w", err)
	}
	out := map[string]Asset{}
	for _, k := range slices.Sorted(maps.Keys(obj)) {
		v := obj[k]
		aObj, err := decodeObjectMap(v)
		if err != nil {
			return nil, newLoadError("assets."+k, "", string(v), "must be an object: "+err.Error())
		}
		consumed := map[string]bool{}
		dataRaw, ok := aObj["data"]
		if !ok {
			return nil, newLoadError("assets."+k+".data", "", "", "missing required field")
		}
		consumed["data"] = true
		data, err := decodeStringArrayRaw(dataRaw)
		if err != nil {
			return nil, newLoadError("assets."+k+".data", "", string(dataRaw), "must be an array of strings: "+err.Error())
		}
		mtRaw, ok := aObj["mediaType"]
		if !ok {
			return nil, newLoadError("assets."+k+".mediaType", "", "", "missing required field")
		}
		consumed["mediaType"] = true
		mt, err := decodeStringRaw(mtRaw)
		if err != nil {
			return nil, newLoadError("assets."+k+".mediaType", "", string(mtRaw), "must be a string: "+err.Error())
		}

		// AC6a (D-1.8.8): validate SHAPE first, then VALUE — two
		// distinct error classes. A key that is not even 64 lowercase
		// hex characters is evidence nothing looked at the key at all;
		// that is a different diagnosis from a well-formed key that
		// does not match its data, and the shape check is the cheaper
		// one, so it runs first.
		if !isSHA256HexKey(k) {
			return nil, newLoadError("assets."+k, "", k, "asset key is not a 64-character lowercase hex digest (AC6a)")
		}

		// AC4/AC1-AC2 (D-1.8.2): accept ANY input wrapping, decode
		// strictly. AC4: invalid base64 and an empty decoded asset are
		// both load errors.
		decoded, derr := decodeBase64Asset(data)
		if derr != nil {
			return nil, newLoadError("assets."+k+".data", "", string(dataRaw), derr.Error())
		}
		if len(decoded) == 0 {
			return nil, newLoadError("assets."+k+".data", "", string(dataRaw), "decoded asset data is empty — it cannot render, and its key would be the SHA-256 of nothing (AC4)")
		}

		// AC5/AC6 (D-1.8.3): the key is the SHA-256 of the DECODED
		// bytes, validated on load; a mismatch is a load error naming
		// both digests.
		gotDigest := sha256HexOf(decoded)
		if gotDigest != k {
			return nil, newLoadError("assets."+k, "", k,
				fmt.Sprintf("asset key does not match the SHA-256 of its data (expected %s, got %s) (AC6)", k, gotDigest))
		}

		// AC9/AC11a/AC11b (D-1.8.1 as amended): a RECOGNISED mediaType
		// whose bytes are not that format is a load error — the file
		// lies about itself, reader-independent. An UNRECOGNISED
		// mediaType is never inspected here and never refused at load
		// (D-1.8.1 amended); it only becomes a located error at render
		// time, only when an element actually needs to draw it
		// (DecodeImageForRender, image.go) — this loader never calls
		// that predicate.
		if _, recognised, ierr := decodeRecognisedImage(mt, decoded); recognised && ierr != nil {
			return nil, newLoadError("assets."+k+".data", "", mt, ierr.Error())
		}

		// Story 8.3: the SAME rule, for fonts, through a second
		// recognised-type predicate beside the image one rather than a
		// widened single one. `recognised` is what gates the inspection
		// in both — an unrecognised font container (font/woff2, say) is
		// never inspected and never refused here, exactly as an
		// unrecognised image type is not. The two predicates are
		// disjoint by construction: no media type is in both switches.
		if recognised, ferr := decodeRecognisedFont(mt, decoded); recognised && ferr != nil {
			return nil, newLoadError("assets."+k+".data", "", mt, ferr.Error())
		}

		font, err := decodeAssetFont(aObj, consumed, "assets."+k+".font")
		if err != nil {
			return nil, err
		}

		extra, err := extraFields(aObj, consumed)
		if err != nil {
			return nil, fmt.Errorf("template: assets.%s: %w", k, err)
		}
		out[k] = Asset{Data: data, MediaType: mt, Font: font, Extra: extra}
	}
	return out, nil
}

// unexpectedKeys returns the keys of obj not present in consumed, sorted
// (D-1.3.5/NFR1.d bans ranging a map anywhere under internal/, so the
// sorted-then-index idiom is used even though only the first result is
// ever read by a caller).
func unexpectedKeys(obj map[string]json.RawMessage, consumed map[string]bool) []string {
	var out []string
	for _, k := range slices.Sorted(maps.Keys(obj)) {
		if !consumed[k] {
			out = append(out, k)
		}
	}
	return out
}

// decodePointsRaw decodes a points-kind numeric field (AC24) from raw.
func decodePointsRaw(field, elementID string, raw json.RawMessage) (geom.Length, error) {
	n, err := decodeNumberRaw(raw)
	if err != nil {
		return 0, newLoadError(field, elementID, string(raw), "must be a JSON number: "+err.Error())
	}
	v, err := decodePoints(string(n))
	if err != nil {
		return 0, newLoadError(field, elementID, string(n), err.Error())
	}
	return v, nil
}

// decodeAssetFont decodes the optional `font` record on one asset
// (Story 8.3). It is optional in three separate senses, and each is
// distinguishable from the others on the way back out:
//
//   - the key is ABSENT — Presence{} — and the asset serializes without
//     it, which is what keeps every existing image asset byte-identical;
//   - the key is present and NULL — a legal spelling that round-trips as
//     `"font": null` rather than vanishing;
//   - the key is present with an object, whose six keys are each
//     optional in the same three-way manner AT THIS LEVEL.
//
// "Optional at this level" is exact, and since Story 8.6 it is no longer
// the whole story. Three of the six — licence, licenceText, copyright —
// are REQUIRED of an asset a font chain names by {"asset": key}. That is
// a rule about the pair (asset, chain entry), not about the asset alone,
// so it cannot be decided here: this function does not know whether
// anything references the asset it is decoding, and `fonts` is decoded
// AFTER `assets` precisely because the reference direction runs that
// way. The refusal therefore lives in decodeFontChainEntry, where both
// halves are in hand — see requireEmbeddedFaceLicence.
//
// AN EXPLICIT NULL IS NOT ABSENCE. Every refusal below is written so it
// fires on `null` as well as on a wrong type where a wrong type is what
// is refused — decodeStringRaw is what refuses `"family": 3`, and the
// Null branch is taken FIRST so a null never reaches it and is never
// mistaken for a missing key.
func decodeAssetFont(aObj map[string]json.RawMessage, consumed map[string]bool, field string) (Presence[FontRecord], error) {
	raw, ok := aObj["font"]
	if !ok {
		return absent[FontRecord](), nil
	}
	consumed["font"] = true
	if rawIsNull(raw) {
		return presentNull[FontRecord](), nil
	}
	obj, err := decodeObjectMap(raw)
	if err != nil {
		return absent[FontRecord](), newLoadError(field, "", string(raw), "must be an object: "+err.Error())
	}
	var rec FontRecord
	inner := map[string]bool{}
	for _, kv := range []struct {
		key string
		dst *Presence[string]
	}{
		{"copyright", &rec.Copyright},
		{"family", &rec.Family},
		{"licence", &rec.Licence},
		{"licenceText", &rec.LicenceText},
		{"source", &rec.Source},
		{"style", &rec.Style},
	} {
		v, declared := obj[kv.key]
		if !declared {
			continue
		}
		inner[kv.key] = true
		if rawIsNull(v) {
			*kv.dst = presentNull[string]()
			continue
		}
		sv, serr := decodeStringRaw(v)
		if serr != nil {
			return absent[FontRecord](), newLoadError(field+"."+kv.key, "", string(v), "must be a string: "+serr.Error())
		}
		*kv.dst = present(sv)
	}
	extra, err := extraFields(obj, inner)
	if err != nil {
		return absent[FontRecord](), fmt.Errorf("template: %s: %w", field, err)
	}
	rec.Extra = extra
	return present(rec), nil
}
