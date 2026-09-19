package template

import (
	"strings"
	"testing"
)

// TestNeverCoerceOnLoad is AC40: a string where a number belongs (and
// vice versa) is an error naming field, element id and value — never a
// parse-and-convert.
func TestNeverCoerceOnLoad(t *testing.T) {
	b := minimalDocWithElements([]string{"e1"}, 2)
	// x is a number field; feed it a string.
	bad := strings.Replace(string(b), `"x": 0,`, `"x": "0",`, 1)
	_, err := ParseDocument([]byte(bad))
	if err == nil {
		t.Fatal("expected a load error rather than a coerced string-to-number conversion")
	}

	// nextId is a number field; feed it a numeric string.
	bad2 := strings.Replace(string(b), `"nextId": 2,`, `"nextId": "2",`, 1)
	_, err = ParseDocument([]byte(bad2))
	if err == nil {
		t.Fatal("expected a load error rather than a coerced numeric-string nextId")
	}
}

// TestClosedSetLoadErrors is AC5: the closed sets are enforced at load,
// each a load error naming field and offending value.
func TestClosedSetLoadErrors(t *testing.T) {
	b := string(minimalDocWithElements(nil, 1))
	cases := []struct {
		name string
		from string
		to   string
	}{
		{"locale", `"locale": "en",`, `"locale": "xx",`},
		{"orientation", `"orientation": "portrait",`, `"orientation": "sideways",`},
		{"pageSize", `"size": "A4"`, `"size": "B5"`},
		{"utcOffset", `"utcOffset": "+00:00",`, `"utcOffset": "0700",`},
	}
	if len(cases) == 0 {
		t.Fatal("coverage witness: zero cases exercised")
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bad := strings.Replace(b, c.from, c.to, 1)
			if bad == b {
				t.Fatalf("fixture substitution %q -> %q did not match anything", c.from, c.to)
			}
			if _, err := ParseDocument([]byte(bad)); err == nil {
				t.Fatalf("expected a load error for an unlisted %s value", c.name)
			}
		})
	}
}

// TestElementTypeClosedSet is AC5: a sixth element type is a load error.
func TestElementTypeClosedSet(t *testing.T) {
	bad := strings.Replace(string(minimalDocWithElements([]string{"e1"}, 2)), `"type": "text",`, `"type": "chart",`, 1)
	if _, err := ParseDocument([]byte(bad)); err == nil {
		t.Fatal("expected a load error for an unrecognised element type")
	}
}
