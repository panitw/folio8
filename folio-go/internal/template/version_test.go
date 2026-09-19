package template

import (
	"strings"
	"testing"
)

func withVersion(v string) []byte {
	base := minimalDocWithElements(nil, 1)
	return []byte(strings.Replace(string(base), `"version": "1.0"`, `"version": "`+v+`"`, 1))
}

// TestHigherMajorIsLoadError is AC6: a higher MAJOR than the library
// supports is a load error naming the declared version and the
// supported version, and no render is attempted.
//
// The barcode element raised the supported ceiling to 4.0. This fixture
// stays one major above it and must name both versions in the refusal.
func TestHigherMajorIsLoadError(t *testing.T) {
	_, err := ParseDocument(withVersion("5.0"))
	if err == nil {
		t.Fatal("expected a load error for a higher MAJOR version")
	}
	if !strings.Contains(err.Error(), "5.0") || !strings.Contains(err.Error(), SupportedVersion) {
		t.Fatalf("error must name both the declared and supported version, got: %v", err)
	}
}

// The supported ceiling itself must load and round-trip.
func TestSupportedMajorIsLoadable(t *testing.T) {
	d, err := ParseDocument(withVersion("3.0"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Version != "3.0" {
		t.Fatalf("Version=%q, want 3.0", d.Version)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"version": "3.0"`) {
		t.Fatalf("ceiling did not round-trip: %s", out)
	}
}

// TestNewContentIsNeverStampedWithTheLibraryCeiling is D-7.2.1's
// guardrail asserted DIRECTLY rather than, as until now, only indirectly
// through the fixtures.
//
// SupportedVersion is 2.0 from Story 7.3 onward, so the failure this
// guards against is no longer hypothetical bookkeeping: a document that
// carries no version-raising content at all must still serialize 1.0,
// and one that carries only a 1.1 key must still serialize 1.1. Stamping
// either with the library's ceiling would orphan it from every reader
// that could in fact have read it — and, for the first time, would make
// a plain three-element document unreadable to the 1.x readers that
// exist.
func TestNewContentIsNeverStampedWithTheLibraryCeiling(t *testing.T) {
	for _, c := range []struct {
		label string
		style string
		want  string
	}{
		{"no style keys at all", "", "1.0"},
		{"a 1.1 key only", `"lineSpacing": 1.5, `, "1.1"},
		{"a 1.0 alignment", `"align": "center", `, "1.0"},
		{"the 2.0 alignment", `"align": "justify", `, "2.0"},
	} {
		t.Run(c.label, func(t *testing.T) {
			d, err := ParseDocument([]byte(lineSpacingRoundTripDocWithStyle(c.style)))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			out, err := SerializeDocument(d)
			if err != nil {
				t.Fatalf("serialize: %v", err)
			}
			if !strings.Contains(string(out), `"version": "`+c.want+`"`) {
				t.Fatalf("want version %s, got:\n%s", c.want, out)
			}
			if c.want != SupportedVersion && strings.Contains(string(out), `"version": "`+SupportedVersion+`"`) {
				t.Fatalf("the document was stamped with the library ceiling %s rather than the %s its content requires", SupportedVersion, c.want)
			}
		})
	}

	// STORY 7.7's ELEMENT-LEVEL SHAPE. Every case above is built through
	// a STYLE helper, so a key that hangs off the element itself is
	// invisible to all of them: a version rule that never ran for
	// `keepTogether` would be indistinguishable here without its own
	// builder. 1.2 is also the first version this library can stamp that
	// is neither the base, the previous MINOR, nor the ceiling.
	for _, c := range []struct {
		label string
		attr  string
		want  string
	}{
		{"no element key at all", "", "1.0"},
		{"a 1.2 element key", `"keepTogether": "signature", `, "1.2"},
		{"an explicitly null 1.2 element key still declares it", `"keepTogether": null, `, "1.2"},
	} {
		t.Run(c.label, func(t *testing.T) {
			d, err := ParseDocument([]byte(keepTogetherRoundTripDocWithAttr(c.attr)))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			out, err := SerializeDocument(d)
			if err != nil {
				t.Fatalf("serialize: %v", err)
			}
			if !strings.Contains(string(out), `"version": "`+c.want+`"`) {
				t.Fatalf("want version %s, got:\n%s", c.want, out)
			}
			if c.want != SupportedVersion && strings.Contains(string(out), `"version": "`+SupportedVersion+`"`) {
				t.Fatalf("the document was stamped with the library ceiling %s rather than the %s its content requires", SupportedVersion, c.want)
			}
		})
	}
}

// TestVersionRoundTripsVerbatim pins D-1.4.13: a 1.0 file round-trips
// as 1.0.
func TestVersionRoundTripsVerbatim_1_0(t *testing.T) {
	b := withVersion("1.0")
	d, err := ParseDocument(b)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if d.Version != "1.0" {
		t.Fatalf("Version = %q, want 1.0", d.Version)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.Contains(string(out), `"version": "1.0"`) {
		t.Fatalf("serialized output does not carry version 1.0 verbatim:\n%s", out)
	}
}

// TestHigherMinorLoadsAndRoundTripsVerbatim is AC7: a higher MINOR
// loads, and a synthetic higher-MINOR file round-trips as that same
// MINOR with its unknown keys intact — never lowered to the library's
// own SupportedVersion.
func TestHigherMinorLoadsAndRoundTripsVerbatim(t *testing.T) {
	base := string(minimalDocWithElements(nil, 1))
	synthetic := strings.Replace(base, `"version": "1.0"`, `"version": "1.1"`, 1)
	synthetic = strings.Replace(synthetic, "\"locale\": \"en\",", "\"futureMinorKey\": true,\n  \"locale\": \"en\",", 1)

	d, err := ParseDocument([]byte(synthetic))
	if err != nil {
		t.Fatalf("a higher MINOR must load: %v", err)
	}
	if d.Version != "1.1" {
		t.Fatalf("Version = %q, want 1.1", d.Version)
	}
	found := false
	for _, f := range d.Extra {
		if f.Key == "futureMinorKey" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unknown key futureMinorKey to survive parse, Extra=%v", d.Extra)
	}

	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.Contains(string(out), `"version": "1.1"`) {
		t.Fatalf("version must not be lowered to %s on save:\n%s", SupportedVersion, out)
	}
	if !strings.Contains(string(out), `"futureMinorKey": true`) {
		t.Fatalf("unknown key must survive serialize:\n%s", out)
	}
}

// TestVersionMustBeMajorDotMinor guards the format itself.
func TestVersionMustBeMajorDotMinor(t *testing.T) {
	for _, v := range []string{"1", "1.0.0", "a.b", ""} {
		if _, _, err := parseVersion(v); err == nil {
			t.Errorf("parseVersion(%q) should have failed", v)
		}
	}
}

// TestVersionForRankIsStrictlyAscending is the anchor versionForRank
// itself lacks (D-7.7.2's guardrail).
//
// versionForRank is a [...]string indexed by an `iota` rank, and the
// whole "highest requirement in the document wins" rule is a comparison
// of RANKS standing in for a comparison of VERSIONS. Nothing about the
// array's shape enforces that the two orders agree: inserting a rank
// renumbers every rank above it, and an entry written out of order — or
// an insertion made in the const block but not here — would silently
// make versionRequiredByContent return the LOWER of two requirements.
// The hand-enumerated lists in linespacing_test.go
// (TestContentVersionNeverExceedsTheLibraryCeiling) go vacuous against
// that failure; this does not.
func TestVersionForRankIsStrictlyAscending(t *testing.T) {
	if len(versionForRank) < 2 {
		t.Fatalf("coverage witness: versionForRank has %d entries; the ordering property is vacuous below two", len(versionForRank))
	}
	prevMajor, prevMinor := -1, -1
	for rank, v := range versionForRank {
		if v == "" {
			t.Fatalf("versionForRank[%d] is empty — a rank exists in the const block with no version behind it, so versionRequiredByContent can return \"\"", rank)
		}
		major, minor, err := parseVersion(v)
		if err != nil {
			t.Fatalf("versionForRank[%d] = %q does not parse: %v", rank, v, err)
		}
		if major < prevMajor || (major == prevMajor && minor <= prevMinor) {
			t.Errorf("versionForRank[%d] = %q is not strictly above versionForRank[%d] (%d.%d) — rank order and version order must agree, or the \"highest requirement wins\" comparison silently returns the lower one", rank, v, rank-1, prevMajor, prevMinor)
		}
		prevMajor, prevMinor = major, minor
	}
	// And the ranks the rule actually names sit where their names say:
	// a bare ascending walk would pass over an array whose entries had
	// been renamed as well as reordered.
	if versionForRank[rankBase] != baseVersion ||
		versionForRank[rankMinorFeature] != minorFeatureVersion ||
		versionForRank[rankKeepTogether] != keepTogetherVersion ||
		versionForRank[rankMajorFeature] != majorFeatureVersion {
		t.Errorf("versionForRank does not map each named rank to its own constant: got %v", versionForRank)
	}
}

// TestATableStyleJustifyIsRefusedBeforeAnyVersionIsComputed is Story
// 7.8's version half, asserted rather than coded — because on the file
// path there is nothing to code.
//
// versionRequiredByContent runs ONLY at save: its one non-test caller is
// versionForSave, whose one non-test caller is serialize.go. The load
// path touches version only at parse.go's checkVersionLoadable, on the
// DECLARED STRING, before bands are decoded at all. So once decodeStyle
// refuses a table's justify, ParseDocument returns (nil, err) and no
// *Document ever exists for the version probe to observe. The 2.0 raise
// is closed by construction; styleVersionRank needs no element type and
// is not a defect for lacking one.
//
// THE TWO LEGS TOGETHER ARE THE DISCRIMINATOR. Either alone is equally
// consistent with a blanket ban on `justify`, which Story 7.3, Story 7.4
// and two shipped goldens forbid. The text leg is what proves the
// refusal is keyed on the CONSUMER and not on the word.
func TestATableStyleJustifyIsRefusedBeforeAnyVersionIsComputed(t *testing.T) {
	// Leg 1 — the TABLE. Refused at load, at both of its style
	// attachment points, so no *Document reaches the probe.
	for _, c := range []struct {
		name      string
		doc       []byte
		wantField string
	}{
		{"table style.align", alignDocWithTable(`"align": "justify"`, "", ""), "style.align"},
		{"table headerStyle.align", alignDocWithTable("", `"align": "justify"`, ""), "headerStyle.align"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, err := ParseDocument(c.doc)
			if err == nil {
				t.Fatalf("a table carrying justify at %s must be refused at load", c.wantField)
			}
			if d != nil {
				t.Fatalf("ParseDocument returned a non-nil *Document alongside its error; versionRequiredByContent could observe it: %+v", d)
			}
			le, ok := err.(*LoadError)
			if !ok {
				t.Fatalf("want a located *LoadError, got %T: %v", err, err)
			}
			if le.Field != c.wantField || le.ElementID != "e1" {
				t.Fatalf("refusal = field %q element %q, want field %q element %q", le.Field, le.ElementID, c.wantField, "e1")
			}
			// And the refusal PRECEDES the version entirely: the
			// document declares 2.0 in its own bytes and is still
			// refused, so nothing about this outcome came from a
			// version comparison.
			if !strings.Contains(string(c.doc), `"version": "2.0"`) {
				t.Fatal("fixture precondition: the document must declare 2.0 itself, so the refusal cannot be mistaken for a version check")
			}
		})
	}

	// Leg 2 — the TEXT element, unchanged. It loads, and it still
	// requires 2.0. Remove this and a blanket ban passes the file.
	d, err := ParseDocument(alignDocWithStyle(`"align": "justify"`))
	if err != nil {
		t.Fatalf("a TEXT element's style.align: \"justify\" must still load (FR47, Story 7.3): %v", err)
	}
	if got := versionRequiredByContent(d); got != majorFeatureVersion {
		t.Errorf("versionRequiredByContent over a justified TEXT element = %q, want %q — this story narrows the set by consumer, it does not ban the value", got, majorFeatureVersion)
	}
}

// TestTableRuleKeysRequire31 is SPEC-table-rules' version rule: `rules` and
// `minHeight` are ADDITIVE keys a 3.0 reader does not know, so a document
// declaring either must say 3.1 — and a document declaring neither must not
// move at all, which is the half that keeps the corpus where it is.
//
// Presence.Set, on `color`'s terms: an explicit `rules: null` is still the KEY
// appearing in a file a 3.0 reader does not recognise.
func TestTableRuleKeysRequire31(t *testing.T) {
	base := func(extra string) []byte {
		return []byte(`{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10` + extra + `,
        "columns": [{"id": "e2", "label": "A", "width": 60, "bind": "{{row.a}}"}]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`)
	}
	for _, c := range []struct{ name, extra, want string }{
		{"neither key", "", baseVersion},
		{"rules", `, "rules": {"between": ["columns"]}`, tableRulesVersion},
		{"an explicit null rules", `, "rules": null`, tableRulesVersion},
		{"minHeight", `, "minHeight": 600`, tableRulesVersion},
		{"both", `, "rules": {"between": ["rows"]}, "minHeight": 600`, tableRulesVersion},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, err := ParseDocument(base(c.extra))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := versionRequiredByContent(d); got != c.want {
				t.Errorf("versionRequiredByContent = %q, want %q", got, c.want)
			}
		})
	}
}
