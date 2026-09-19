package bind

import (
	"strings"
	"testing"
)

// TestDecodeDataAcceptsAWellFormedSingleDocument pins the ordinary
// case DecodeData must keep accepting once trailing-content validation
// is added (QA Finding 4).
func TestDecodeDataAcceptsAWellFormedSingleDocument(t *testing.T) {
	v, err := DecodeData([]byte(`{"customer":{"name":"Ada"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, presence := v.Lookup([]string{"customer", "name"})
	if presence != Present || got.Kind != KindString || got.Str != "Ada" {
		t.Fatalf("Lookup(customer.name) = (%+v, %v), want (Ada, Present)", got, presence)
	}
}

// TestDecodeDataRejectsTrailingContent is QA Finding 4 (this story's
// review, Major): json.Decoder.Decode stops at the end of the first
// JSON value and silently discards anything after it. Render's own
// documented precondition (render_entry.go: "d must be syntactically
// valid JSON", AC24) was previously unenforced — measured through the
// public Render, trailing garbage and a second concatenated document
// both produced err == nil and rendered the first value, silently
// discarding the rest.
func TestDecodeDataRejectsTrailingContent(t *testing.T) {
	cases := []string{
		`{"customer":{"name":"Ada"}} THIS IS NOT JSON`,
		`{"customer":{"name":"Ada"}}{"customer":{"name":"Bob"}}`,
		`{"customer":{"name":"Ada"}}   {}`,
	}
	for _, c := range cases {
		if _, err := DecodeData([]byte(c)); err == nil {
			t.Errorf("DecodeData(%q): expected an error for trailing content after the first JSON value, got nil", c)
		}
	}
}

// TestDecodeDataEmptyIsStillEOF pins the pre-existing "empty Data"
// error path (a bare io.EOF from the FIRST Decode call, before the new
// trailing-content check is ever reached) — this must stay a decode
// error, not accidentally be reclassified as "trailing content".
func TestDecodeDataEmptyIsStillEOF(t *testing.T) {
	_, err := DecodeData([]byte(``))
	if err == nil {
		t.Fatal("expected an error for empty Data")
	}
	if !strings.Contains(err.Error(), "invalid JSON report data") {
		t.Errorf("expected the original decode-error wrapping for empty input, got: %v", err)
	}
}

// TestDecodeDataTrailingWhitespaceOnlyIsLegal confirms the fix does not
// over-reject: trailing whitespace after the single top-level value
// (the ordinary shape of a file with a trailing newline) is not
// "trailing content".
func TestDecodeDataTrailingWhitespaceOnlyIsLegal(t *testing.T) {
	if _, err := DecodeData([]byte("{\"a\":1}\n")); err != nil {
		t.Errorf("trailing whitespace/newline alone must remain legal: %v", err)
	}
}
