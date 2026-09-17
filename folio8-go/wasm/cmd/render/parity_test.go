//go:build !(js && wasm)

package main

// TestFolioJSParityExpectations records what the Go engine returns for the
// inputs folio-js's diagnostic-parity tests replay through the render wasm
// (folio-js/test/data/go-parity.json). folio-js compares its own results to
// this file, so the expectation is Go's, never the binding's. The test fails
// when the recorded file drifts from Go; regenerate it deliberately with
//
//	FOLIO8_UPDATE_JS_PARITY=1 go test ./wasm/cmd/render
//
// It runs on the host, not under js/wasm, and uses only the public API.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio8-go"
	"github.com/panitw/folio8/folio8-go/fonts"
)

type parityInput struct {
	File string `json:"file,omitempty"`
	Text string `json:"text,omitempty"`
}

type parityDiagnostic struct {
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	ElementID string `json:"elementId"`
	DataPath  string `json:"dataPath"`
	Message   string `json:"message"`
}

type parityError struct {
	Diagnostic *parityDiagnostic `json:"diagnostic,omitempty"`
	Message    string            `json:"message,omitempty"`
}

type parityOutcome struct {
	SHA256      string              `json:"sha256,omitempty"`
	Diagnostics *[]parityDiagnostic `json:"diagnostics,omitempty"`
	References  *[]string           `json:"references,omitempty"`
	Error       *parityError        `json:"error,omitempty"`
}

type parityCase struct {
	Name     string        `json:"name"`
	Op       string        `json:"op"`
	Template parityInput   `json:"template"`
	Data     *parityInput  `json:"data,omitempty"`
	Params   *parityInput  `json:"params"`
	Expect   parityOutcome `json:"expect"`
}

type parityFace struct {
	Name       string `json:"name"`
	ByteLength int    `json:"byteLength"`
}

type parityFile struct {
	Comment string `json:"comment"`
	Version string `json:"folio8Version"`
	// ShippedFaces is fonts.Shipped(), by name, sorted, with each face's
	// length, so folio-js's hand-built font map cannot drift from it.
	ShippedFaces []parityFace `json:"shippedFaces"`
	Cases        []parityCase `json:"cases"`
}

func parityCases() []parityCase {
	file := func(p string) *parityInput { return &parityInput{File: p} }
	text := func(s string) *parityInput { return &parityInput{Text: s} }
	const wrapped = "fixtures/wrapped-text/input.folio"
	const named = "folio-js/test/data/wrapped-text-named.json"
	return []parityCase{
		{Name: "render with a clip warning", Op: "render", Template: *file(wrapped), Data: file(named)},
		{Name: "render of an absent data path", Op: "render", Template: *file(wrapped), Data: text("{}")},
		{Name: "render of data that is not JSON", Op: "render", Template: *file(wrapped), Data: text("not json")},
		{Name: "render with params", Op: "render", Template: *file("fixtures/statement-5/input.folio"), Data: file("fixtures/statement-5/data.json"), Params: file("fixtures/statement-5/params.json")},
		{Name: "parse of a malformed template", Op: "parse", Template: *text("{")},
		{Name: "validate of a clean template", Op: "validate", Template: *file("fixtures/colour-strokes/input.folio"), Data: file("fixtures/colour-strokes/data.json")},
		{Name: "validate with a clip warning", Op: "validate", Template: *file(wrapped), Data: file(named)},
		{Name: "validate of an absent data path", Op: "validate", Template: *file(wrapped), Data: text("{}")},
		{Name: "validate of a malformed template", Op: "validate", Template: *text("{"), Data: text("{}")},
		{Name: "parameter references", Op: "parameterReferences", Template: *file("folio-js/test/data/document-date.folio")},
	}
}

func shippedFaces() []parityFace {
	set := fonts.Shipped()
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]parityFace, 0, len(names))
	for _, name := range names {
		out = append(out, parityFace{Name: name, ByteLength: len(set[name])})
	}
	return out
}

func repoRoot(t *testing.T) string {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Join(filepath.Dir(here), "..", "..", "..", "..")
}

func TestFolioJSParityExpectations(t *testing.T) {
	root := repoRoot(t)
	read := func(in *parityInput) []byte {
		if in == nil {
			return nil
		}
		if in.File == "" {
			return []byte(in.Text)
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(in.File)))
		if err != nil {
			t.Fatalf("read %s: %v", in.File, err)
		}
		return b
	}
	convert := func(ds []folio8.Diagnostic) *[]parityDiagnostic {
		out := make([]parityDiagnostic, 0, len(ds))
		for _, d := range ds {
			out = append(out, parityDiagnostic{strings.ToLower(d.Severity.String()), d.Code, d.ElementID, d.DataPath, d.Message})
		}
		return &out
	}
	failure := func(err error) *parityError {
		var re *folio8.RenderError
		if errors.As(err, &re) {
			d := (*convert([]folio8.Diagnostic{re.Diagnostic}))[0]
			return &parityError{Diagnostic: &d}
		}
		return &parityError{Message: err.Error()}
	}

	cases := parityCases()
	for i := range cases {
		c := &cases[i]
		tplBytes := read(&c.Template)
		var params folio8.Params
		if c.Params != nil {
			params = read(c.Params)
		}
		switch c.Op {
		case "parse":
			if _, err := folio8.ParseTemplate(tplBytes); err != nil {
				c.Expect.Error = failure(err)
			}
		case "validate":
			ds, err := folio8.Validate(tplBytes, read(c.Data), params, fonts.Shipped())
			if err != nil {
				c.Expect.Error = failure(err)
			} else {
				c.Expect.Diagnostics = convert(ds)
			}
		case "render":
			tpl, err := folio8.ParseTemplate(tplBytes)
			if err != nil {
				t.Fatalf("%s: parse: %v", c.Name, err)
			}
			res, err := folio8.Render(tpl, read(c.Data), params, fonts.Shipped())
			if err != nil {
				c.Expect.Error = failure(err)
			} else {
				sum := sha256.Sum256(res.Bytes)
				c.Expect.SHA256 = hex.EncodeToString(sum[:])
				c.Expect.Diagnostics = convert(res.Diagnostics)
			}
		case "parameterReferences":
			tpl, err := folio8.ParseTemplate(tplBytes)
			if err != nil {
				t.Fatalf("%s: parse: %v", c.Name, err)
			}
			refs, err := folio8.ParameterReferences(tpl)
			if err != nil {
				c.Expect.Error = failure(err)
			} else {
				if refs == nil {
					refs = []string{}
				}
				c.Expect.References = &refs
			}
		default:
			t.Fatalf("unknown op %q", c.Op)
		}
	}

	// The cases must exercise what folio-js's tests rely on.
	byName := map[string]parityCase{}
	for _, c := range cases {
		byName[c.Name] = c
	}
	if d := byName["render with a clip warning"].Expect.Diagnostics; d == nil || len(*d) == 0 {
		t.Error("the warning render case produced no warning")
	}
	// Go's Validate reports an absent data path as a returned *RenderError,
	// never as an error-severity entry in its slice, so folio-js must reject
	// here too; the case records that rather than a prediction slice.
	if e := byName["validate of an absent data path"].Expect.Error; e == nil || e.Diagnostic == nil {
		t.Error("the absent-path validate case did not fail with a RenderError")
	}
	if e := byName["render of an absent data path"].Expect.Error; e == nil || e.Diagnostic == nil {
		t.Error("the absent-path render case did not fail with a RenderError")
	}
	if e := byName["render of data that is not JSON"].Expect.Error; e == nil || e.Diagnostic != nil {
		t.Error("the non-JSON data case did not fail with a plain error")
	}

	got, err := json.MarshalIndent(parityFile{
		Comment:      "Generated by folio8-go/wasm/cmd/render/parity_test.go (FOLIO8_UPDATE_JS_PARITY=1). Do not edit by hand.",
		Version:      folio8.Version,
		ShippedFaces: shippedFaces(),
		Cases:        cases,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join(root, "folio-js", "test", "data", "go-parity.json")
	if os.Getenv("FOLIO8_UPDATE_JS_PARITY") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (regenerate with FOLIO8_UPDATE_JS_PARITY=1)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("folio-js/test/data/go-parity.json has drifted from what Go returns; regenerate with FOLIO8_UPDATE_JS_PARITY=1 and review the diff")
	}
}
