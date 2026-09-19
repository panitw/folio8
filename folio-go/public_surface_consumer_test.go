package folio8

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDesignerSurfaceIsUnreachableFromOutsideTheModule holds the public API to
// render and validate from the one place that matters: a module that is not
// folio-go. The designer surface lives unexported in this package and in
// internal/designer and internal/wasm, and each probe below must fail to
// compile for the stated reason.
//
// THE CONTROL IS WHAT MAKES THE REFUSALS EVIDENCE. A consumer module that could
// not build at all — a bad go.mod, a missing go.sum, no toolchain — would fail
// every probe, and the refusals would read as a healthy surface. So a consumer
// using the kept API must BUILD first, and each refusal must name its cause.
func TestDesignerSurfaceIsUnreachableFromOutsideTheModule(t *testing.T) {
	if testing.Short() {
		t.Skip("builds separate modules")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go command not available")
	}
	root := repoRootFromTest(t)
	sum, err := os.ReadFile(filepath.Join(root, "folio-go", "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	build := func(t *testing.T, source string) (string, error) {
		t.Helper()
		work := t.TempDir()
		files := map[string]string{
			"go.mod":  "module example.com/folio8-consumer\n\ngo 1.25.0\n\nrequire github.com/panitw/folio8/folio-go v0.0.0\n\nreplace github.com/panitw/folio8/folio-go => " + filepath.Join(root, "folio-go") + "\n",
			"go.sum":  string(sum),
			"main.go": "package main\n\n" + source + "\n",
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(work, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(goBin, "build", "./...")
		cmd.Dir = work
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	t.Run("control: the kept API builds", func(t *testing.T) {
		out, err := build(t, `import folio8 "github.com/panitw/folio8/folio-go"

func main() {
	_, _ = folio8.ParseTemplate(nil)
	_ = folio8.Render
	_ = folio8.RenderTo
	_ = folio8.Validate
	_ = folio8.SerializeTemplate
	_ = folio8.ParameterReferences
}`)
		if err != nil {
			t.Fatalf("a consumer of the kept API did not build, so no refusal below is evidence:\n%s", out)
		}
	})

	for _, probe := range []struct{ name, source, cause string }{
		{"folio8.Canvas", `import folio8 "github.com/panitw/folio8/folio-go"

func main() { _ = folio8.Canvas }`, "undefined: folio8.Canvas"},
		{"folio8.ApplyComponentCommand", `import folio8 "github.com/panitw/folio8/folio-go"

func main() { _ = folio8.ApplyComponentCommand }`, "undefined: folio8.ApplyComponentCommand"},
		{"folio8.CanvasProjection", `import folio8 "github.com/panitw/folio8/folio-go"

func main() { var _ folio8.CanvasProjection }`, "undefined: folio8.CanvasProjection"},
		{"folio-go/wasm", `import _ "github.com/panitw/folio8/folio-go/wasm"

func main() {}`, "does not contain package github.com/panitw/folio8/folio-go/wasm"},
		{"internal/wasm", `import _ "github.com/panitw/folio8/folio-go/internal/wasm"

func main() {}`, "use of internal package github.com/panitw/folio8/folio-go/internal/wasm not allowed"},
		{"internal/designer", `import _ "github.com/panitw/folio8/folio-go/internal/designer"

func main() {}`, "use of internal package github.com/panitw/folio8/folio-go/internal/designer not allowed"},
	} {
		t.Run(probe.name, func(t *testing.T) {
			out, err := build(t, probe.source)
			if err == nil {
				t.Fatalf("an outside module reached %s: it is public API again", probe.name)
			}
			if !strings.Contains(out, probe.cause) {
				t.Fatalf("%s failed to build, but not because it is unreachable (want %q):\n%s", probe.name, probe.cause, out)
			}
		})
	}
}
