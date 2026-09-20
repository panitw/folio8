package folio8_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/internal/designer"
)

// designerBridge names every function variable internal/designer declares,
// with whether it is set. The list is written by hand, so
// TestDesignerBridgeListsEveryVariable holds it to the package's source.
func designerBridge() map[string]bool {
	return map[string]bool{
		"Canvas":                designer.Canvas != nil,
		"CanvasWithTextPaint":   designer.CanvasWithTextPaint != nil,
		"ApplyComponentCommand": designer.ApplyComponentCommand != nil,
		"ApplyPageSetupCommand": designer.ApplyPageSetupCommand != nil,
		"PreviewComponentMove":  designer.PreviewComponentMove != nil,
		"TableColumns":          designer.TableColumns != nil,
		"PreviewIdentity":       designer.PreviewIdentity != nil,
		"AssetBytes":            designer.AssetBytes != nil,
		"StandInData":           designer.StandInData != nil,
		"CarriedCommands":       designer.CarriedCommands != nil,
	}
}

// TestDesignerBridgeIsAssignedOnceFolio8IsImported: internal/designer's
// functions are variables folio8's init assigns. A package that imported
// designer without folio8 would call a nil func; importing folio8 — as this
// test does — must leave none unset.
func TestDesignerBridgeIsAssignedOnceFolio8IsImported(t *testing.T) {
	for name, set := range designerBridge() {
		if !set {
			t.Errorf("designer.%s is nil after importing folio8: assign it in folio8's designer_bridge.go init", name)
		}
	}
}

// TestDesignerBridgeListsEveryVariable keeps designerBridge complete: every
// package-level func-typed variable in internal/designer's non-test source
// must be in it, and it must name nothing else.
func TestDesignerBridgeListsEveryVariable(t *testing.T) {
	dir := filepath.Join("internal", "designer")
	fset := token.NewFileSet()
	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var declared []string
	for _, path := range matches {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value := spec.(*ast.ValueSpec)
				if _, ok := value.Type.(*ast.FuncType); !ok {
					continue
				}
				for _, name := range value.Names {
					declared = append(declared, name.Name)
				}
			}
		}
	}
	if len(declared) == 0 {
		t.Fatalf("found no function variables under %s: the scan read the wrong directory", dir)
	}
	listed := designerBridge()
	sort.Strings(declared)
	for _, name := range declared {
		if _, ok := listed[name]; !ok {
			t.Errorf("internal/designer declares %s, which designerBridge does not check", name)
		}
	}
	if len(listed) != len(declared) {
		t.Errorf("designerBridge lists %d variables, internal/designer declares %d: %v", len(listed), len(declared), declared)
	}
}

// TestDesignerBridgeRefusesAForeignHandle: a handle that is not a
// *folio8.Template is a programmer error, and the panic names the type the
// bridge expected.
func TestDesignerBridgeRefusesAForeignHandle(t *testing.T) {
	defer func() {
		got, _ := recover().(string)
		if !strings.Contains(got, "*folio8.Template") {
			t.Fatalf("panic = %q, want one naming *folio8.Template", got)
		}
	}()
	_, _ = designer.Canvas("not a template")
	t.Fatal("a foreign handle did not panic")
}

// TestDesignerBridgeReachesTheEngine: a component refusal through the bridge
// still matches *designer.ComponentCommandError via errors.As, and a nil
// handle keeps its ordinary refusal rather than panicking.
func TestDesignerBridgeReachesTheEngine(t *testing.T) {
	tpl, err := folio8.LoadTemplate(filepath.Join("testdata", "example", "first-pdf.folio"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := designer.Canvas(tpl); err != nil {
		t.Fatalf("canvas through the bridge: %v", err)
	}
	_, err = designer.ApplyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"ids":["e2"]}`))
	var failure *designer.ComponentCommandError
	if !errors.As(err, &failure) {
		t.Fatalf("want a *designer.ComponentCommandError, got %T: %v", err, err)
	}
	if _, err := designer.Canvas(nil); err == nil {
		t.Fatal("a nil handle projected without error")
	}
	var nilTemplate *folio8.Template
	if _, err := designer.StandInData(nilTemplate); err == nil {
		t.Fatal("a nil *folio8.Template produced stand-in data without error")
	}
}

// TestDesignerBridgeCarriedCommandsAnswersForEveryShape exercises the bridge
// wrapper's own branches rather than only its non-nilness. The unreadable-list
// branch in particular is the one that decides whether internal/wasm's fence
// reports a refusal that belongs to the command door, so "it is assigned" is
// not enough to know it is right.
func TestDesignerBridgeCarriedCommandsAnswersForEveryShape(t *testing.T) {
	for _, row := range []struct {
		name    string
		command string
		members []string
		unit    bool
	}{
		{
			name:    "an ordinary command carries itself",
			command: `{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`,
			members: []string{`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`},
		},
		{
			name:    "bytes that do not decode carry themselves",
			command: `not json`,
			members: []string{`not json`},
		},
		{
			name:    "a unit carries its members in order",
			command: `{"kind":"applyCommands","version":1,"commands":[{"kind":"a"},{"kind":"b"}]}`,
			members: []string{`{"kind":"a"}`, `{"kind":"b"}`},
			unit:    true,
		},
		{
			name:    "a unit whose member list cannot be read carries nothing, but is still a unit",
			command: `{"kind":"applyCommands","version":1,"commands":7}`,
			unit:    true,
		},
		{
			name:    "a unit whose member list is absent carries nothing, but is still a unit",
			command: `{"kind":"applyCommands","version":1,"atomic":true}`,
			unit:    true,
		},
		{
			// The door refuses anything that is not version 1 as an unknown
			// command, so this is not a unit here either.
			name:    "a version the door does not know is not a unit",
			command: `{"kind":"applyCommands","version":2,"commands":[{"kind":"a"}]}`,
			members: []string{`{"kind":"applyCommands","version":2,"commands":[{"kind":"a"}]}`},
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			members, unit := designer.CarriedCommands([]byte(row.command))
			if unit != row.unit {
				t.Errorf("unit = %v, want %v", unit, row.unit)
			}
			got := make([]string, 0, len(members))
			for _, member := range members {
				got = append(got, string(member))
			}
			if strings.Join(got, "|") != strings.Join(row.members, "|") {
				t.Errorf("members = %v, want %v", got, row.members)
			}
		})
	}
}
