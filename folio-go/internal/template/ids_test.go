package template

import (
	"strings"
	"testing"
)

// TestIDBase36DecimalAsymmetry is AC33's table test: counter -> id ->
// counter, across the boundaries that matter. It reports how many cases
// it exercised and fails on zero (D-000.9 shape, applied here even
// though this is not itself a lint guard: a table test that silently
// iterated zero cases would pass identically to one that covered the
// boundaries).
func TestIDBase36DecimalAsymmetry(t *testing.T) {
	cases := []struct {
		counter int64
		id      string
	}{
		{1, "e1"},
		{9, "e9"},
		{10, "ea"},
		{35, "ez"},
		{36, "e10"},
		{37, "e11"},
		{71, "e1z"},
		{72, "e20"},
		{123456789, "e21i3v9"},
	}
	if len(cases) == 0 {
		t.Fatal("coverage witness: zero cases exercised")
	}
	exercised := 0
	for _, c := range cases {
		exercised++
		got := renderElementID(c.counter)
		if string(got) != c.id {
			t.Errorf("renderElementID(%d) = %q, want %q", c.counter, got, c.id)
		}
		decoded, err := validateElementID(c.id)
		if err != nil {
			t.Errorf("validateElementID(%q): %v", c.id, err)
			continue
		}
		if decoded != c.counter {
			t.Errorf("validateElementID(%q) = %d, want %d", c.id, decoded, c.counter)
		}
	}
	if exercised == 0 {
		t.Fatal("coverage witness: zero cases exercised")
	}
}

func minimalDocWithElements(elementIDs []string, nextID int64) []byte {
	var sb strings.Builder
	sb.WriteString(`{
  "assets": {},
  "bands": {
    "content": {
      "elements": []
    },
    "pageFooter": {
      "elements": [],
      "height": 20
    },
    "pageHeader": {
      "elements": [
`)
	for i, id := range elementIDs {
		sb.WriteString(`        {
          "height": 10,
          "id": "` + id + `",
          "type": "text",
          "value": "v",
          "width": 10,
          "x": 0,
          "y": 0
        }`)
		if i != len(elementIDs)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString(`      ],
      "height": 20
    }
  },
  "fonts": {},
  "locale": "en",
  "nextId": `)
	sb.WriteString(itoa64(nextID))
	sb.WriteString(`,
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
  "utcOffset": "+00:00",
  "version": "1.0"
}
`)
	return []byte(sb.String())
}

// minimalDocMissingNextID is minimalDocWithElements' base document with
// the "nextId" key removed entirely — AC37's "nextId absent -> load
// error" polarity, retained explicitly (this story's finisher review,
// Finding 19, Minor: the prior comment claimed this was "exercised
// elsewhere" but no test actually removed the key).
func minimalDocMissingNextID(elementIDs []string) []byte {
	base := string(minimalDocWithElements(elementIDs, 999))
	const marker = "\n  \"nextId\": 999,"
	if !strings.Contains(base, marker) {
		panic("minimalDocMissingNextID: nextId marker not found — minimalDocWithElements format changed")
	}
	return []byte(strings.Replace(base, marker, "", 1))
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	b = appendPlainInt(b, n)
	return string(b)
}

// TestSaveGuardrailDeleteElement is AC38: load, delete an element,
// serialize — every surviving id is byte-identical AND nextId is
// unchanged, not decremented. A serializer that renumbers, or
// recomputes nextId from the surviving maximum, passes P1 and fails
// this.
func TestSaveGuardrailDeleteElement(t *testing.T) {
	b := minimalDocWithElements([]string{"e1", "e2", "e3"}, 4)
	d, err := ParseDocument(b)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Delete the middle element (e2).
	var kept []Element
	for _, e := range d.Bands.PageHeader.Elements {
		if e.ID != "e2" {
			kept = append(kept, e)
		}
	}
	d.Bands.PageHeader.Elements = kept

	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	d2, err := ParseDocument(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}

	if len(d2.Bands.PageHeader.Elements) != 2 {
		t.Fatalf("expected 2 surviving elements, got %d", len(d2.Bands.PageHeader.Elements))
	}
	if d2.Bands.PageHeader.Elements[0].ID != "e1" || d2.Bands.PageHeader.Elements[1].ID != "e3" {
		t.Fatalf("surviving ids not byte-identical: got %v", d2.Bands.PageHeader.Elements)
	}
	if d2.NextID != 4 {
		t.Fatalf("nextId must be unchanged (4), not decremented/renumbered: got %d", d2.NextID)
	}
}

// TestSaveGuardrailDeleteElement_HighestSurvives is the guardrail's
// other polarity: deleting the HIGHEST-numbered element must not let a
// future allocation reuse its id — proved by nextId staying above it.
func TestSaveGuardrailDeleteElement_HighestSurvives(t *testing.T) {
	b := minimalDocWithElements([]string{"e1", "e2", "e3"}, 4)
	d, err := ParseDocument(b)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var kept []Element
	for _, e := range d.Bands.PageHeader.Elements {
		if e.ID != "e3" {
			kept = append(kept, e)
		}
	}
	d.Bands.PageHeader.Elements = kept

	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	d2, err := ParseDocument(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if d2.NextID != 4 {
		t.Fatalf("nextId must stay 4 (never decremented to 3) even though e3 was the highest id and is now gone: got %d", d2.NextID)
	}
}

// TestIDSpellingLoadErrors is AC34's retained fixtures: leading zero,
// uppercase, e0, e alone, non-alphanumeric — each a load error, never
// normalised.
func TestIDSpellingLoadErrors(t *testing.T) {
	cases := []struct {
		name string
		id   string
	}{
		{"leading-zero", "e01"},
		{"uppercase", "eA"},
		{"zero-counter", "e0"},
		{"bare-e", "e"},
		{"non-alphanumeric", "e1!"},
	}
	if len(cases) == 0 {
		t.Fatal("coverage witness: zero cases exercised")
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := minimalDocWithElements([]string{c.id}, 100)
			_, err := ParseDocument(b)
			if err == nil {
				t.Fatalf("expected a load error for id %q, got none", c.id)
			}
		})
	}
}

// TestDuplicateIDIsLoadError is AC35: a duplicate id anywhere in the
// document is a load error.
func TestDuplicateIDIsLoadError(t *testing.T) {
	b := minimalDocWithElements([]string{"e1", "e1"}, 5)
	_, err := ParseDocument(b)
	if err == nil {
		t.Fatal("expected a load error for a duplicate id, got none")
	}
}

// TestNextIDAbsentIsLoadError is AC37's "nextId absent" polarity,
// retained explicitly (this story's finisher review, Finding 19,
// Minor). The prior comment on TestNextIDValidation below claimed this
// was "exercised elsewhere", but no retained test actually removed the
// key before this one.
func TestNextIDAbsentIsLoadError(t *testing.T) {
	_, err := ParseDocument(minimalDocMissingNextID([]string{"e1"}))
	if err == nil {
		t.Fatal("expected a load error when nextId is absent (AD-10/AC37: never repaired, never inferred)")
	}
	le, ok := err.(*LoadError)
	if !ok {
		t.Fatalf("expected a *LoadError, got %T: %v", err, err)
	}
	if le.Field != "nextId" {
		t.Fatalf("LoadError.Field = %q, want %q", le.Field, "nextId")
	}
}

// TestNextIDValidation is AC37: nextId <= highest id present -> load
// error. See TestNextIDAbsentIsLoadError above for the "absent" polarity.
func TestNextIDValidation(t *testing.T) {
	cases := []struct {
		name    string
		ids     []string
		nextID  int64
		wantErr bool
	}{
		{"equal-to-highest", []string{"e1", "e2"}, 2, true},
		{"below-highest", []string{"e1", "e5"}, 3, true},
		{"exactly-above-highest", []string{"e1", "e5"}, 6, false},
		{"no-elements-zero", []string{}, 0, true},
		{"no-elements-one", []string{}, 1, false},
	}
	if len(cases) == 0 {
		t.Fatal("coverage witness: zero cases exercised")
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := minimalDocWithElements(c.ids, c.nextID)
			_, err := ParseDocument(b)
			if c.wantErr && err == nil {
				t.Fatalf("expected a load error, got none")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
