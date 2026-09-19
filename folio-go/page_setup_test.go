package folio8

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
)

func TestPageSetupCommandChangesOnlyValidCanonicalPageState(t *testing.T) {
	input, err := os.ReadFile("testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := ParseTemplate(input)
	if err != nil {
		t.Fatal(err)
	}
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := applyPageSetupCommand(tpl, []byte(`{"kind":"pageSetup","version":1,"preset":"custom","orientation":"landscape","width":300.125,"height":400.5,"margin":{"top":10,"right":11.5,"bottom":12,"left":13}}`))
	if err != nil {
		t.Fatal(err)
	}
	if projection.Width != 400500 || projection.Height != 300125 || projection.Bands[0].Name != "pageHeader" || projection.Bands[1].Name != "content" || projection.Bands[2].Name != "pageFooter" {
		t.Fatalf("unexpected projection: %#v", projection)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, after) {
		t.Fatal("valid page setup did not change canonical bytes")
	}
	stable := append([]byte(nil), after...)
	for _, command := range [][]byte{[]byte(`{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":0,"height":200,"margin":{"top":1,"right":1,"bottom":1,"left":1}}`), []byte(`{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":1.0001,"height":2,"margin":{"top":1,"right":1,"bottom":1,"left":1}}`), []byte(`{"kind":"pageSetup","version":1,"preset":"A4","orientation":"diagonal","width":0,"height":0,"margin":{"top":1,"right":1,"bottom":1,"left":1}}`)} {
		if _, err := applyPageSetupCommand(tpl, command); err == nil {
			t.Fatalf("invalid command succeeded: %s", command)
		}
		got, err := SerializeTemplate(tpl)
		if err != nil || !bytes.Equal(got, stable) {
			t.Fatal("invalid command changed canonical bytes")
		}
	}
}

func TestCustomLandscapeApplyIsByteStableAndTransportSafe(t *testing.T) {
	input, err := os.ReadFile("testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := ParseTemplate(input)
	if err != nil {
		t.Fatal(err)
	}
	command := []byte(`{"kind":"pageSetup","version":1,"preset":"custom","orientation":"landscape","width":300.125,"height":400.5,"margin":{"top":10,"right":11.5,"bottom":12,"left":13}}`)
	projection, err := applyPageSetupCommand(tpl, command)
	if err != nil || projection.CommandWidth != 300125 || projection.CommandHeight != 400500 || projection.Width != 400500 || projection.Height != 300125 {
		t.Fatalf("custom landscape projection = %#v, %v", projection, err)
	}
	before, _ := SerializeTemplate(tpl)
	if _, err := applyPageSetupCommand(tpl, command); err != nil {
		t.Fatal(err)
	}
	after, _ := SerializeTemplate(tpl)
	if !bytes.Equal(before, after) {
		t.Fatal("unchanged custom landscape command changed canonical bytes")
	}
	unsafe := []byte(`{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":9007199254740.992,"height":200,"margin":{"top":1,"right":1,"bottom":1,"left":1}}`)
	if _, err := applyPageSetupCommand(tpl, unsafe); err == nil {
		t.Fatal("first JavaScript-unsafe millipoint was accepted")
	}
}

// TestApplyPageSetupCommandRefusesDuplicateKeys narrows the SECOND exported
// door. Both of its own gates are duplicate-blind: `len(raw) != 7` counts a map
// encoding/json has already deduped, `len(margins) != 4` counts another one
// nested inside it, and `equalNumber(raw["version"], "1")` reads the last
// version key. Every payload below was ACCEPTED at the baseline.
//
// THE SAME-ARITY KIND ESCALATION LEG DOES NOT APPLY HERE AND IS NOT COUNTED AS
// COVERED. This door dispatches no kinds — it gates on the single literal
// "pageSetup" — so there is no second handler to escalate into. That leg lives
// on the component door, where it is asserted.
func TestApplyPageSetupCommandRefusesDuplicateKeys(t *testing.T) {
	const valid = `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"landscape","width":300.125,"height":400.5,"margin":{"top":10,"right":11.5,"bottom":12,"left":13}}`
	for _, probe := range []struct {
		name    string
		command string
		level   string
	}{
		{
			name:    "top level",
			command: `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":300,"height":400,"margin":{"top":10,"right":10,"bottom":10,"left":10},"orientation":"landscape"}`,
			level:   "$",
		},
		{
			name:    "the nested margin object",
			command: `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":300,"height":400,"margin":{"top":1,"top":2,"right":10,"bottom":10,"left":10}}`,
			level:   "$.margin",
		},
		{
			name:    "a repeated version",
			command: `{"kind":"pageSetup","version":0,"preset":"custom","orientation":"portrait","width":300,"height":400,"margin":{"top":10,"right":10,"bottom":10,"left":10},"version":1}`,
			level:   "$",
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			input, err := os.ReadFile("testdata/template/golden/worked-example.json")
			if err != nil {
				t.Fatal(err)
			}
			tpl, err := ParseTemplate(input)
			if err != nil {
				t.Fatal(err)
			}
			before, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			_, err = applyPageSetupCommand(tpl, []byte(probe.command))
			if err == nil {
				t.Fatal("duplicate-key bytes were accepted at the page-setup door")
			}
			// THE CODE OF THE DOOR IT WAS RAISED AT. A page-setup refusal must
			// reach the host as PAGE_SETUP_INVALID, and the host decides that
			// by the message prefix — so a *ComponentCommandError here would be
			// matched first by engineFailure and reported as COMPONENT_INVALID,
			// which the designer's only code-branching page-setup consumer
			// never sees.
			var componentShaped *designer.ComponentCommandError
			if errors.As(err, &componentShaped) {
				t.Fatalf("the page-setup door raised a ComponentCommandError, so the host reports COMPONENT_INVALID for a page-setup command: %v", err)
			}
			if !strings.HasPrefix(err.Error(), pageSetupFailurePrefix) {
				t.Fatalf("error = %q, want it to open with %q so the host answers PAGE_SETUP_INVALID", err.Error(), pageSetupFailurePrefix)
			}
			if !strings.Contains(err.Error(), probe.level) {
				t.Fatalf("message %q does not name the level %s the duplicate is at", err.Error(), probe.level)
			}
			after, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("a refused page setup command changed the canonical bytes")
			}
			// The other direction, in the same test, so the guard cannot be
			// satisfied by refusing everything.
			if _, err := applyPageSetupCommand(tpl, []byte(valid)); err != nil {
				t.Fatalf("an unambiguous page setup command was refused: %v", err)
			}
		})
	}
}

// TestPageSetupNumberDiagnosticNamesTheCauseItFound is the red-proof pair, and
// it runs BOTH DIRECTIONS because fixing a message must not delete the detector
// underneath it. `parseMillipoints`'s whole-part loop used to mix two causes in
// one condition — `c < '0' || c > '9' || whole > (1<<63-1)/10` — so `null` and
// a twenty-digit overflow were INDISTINGUISHABLE, both reporting overflow.
//
// `null` is not a hypothetical: it is exactly what the designer's encoder emits
// for a draft that is not a number, and for a box the author emptied. The
// convention shipped with a comment claiming Go answered with a field-specific
// diagnostic, and what an author with an empty margin actually got was
// "page.margin.top overflows millipoints".
func TestPageSetupNumberDiagnosticNamesTheCauseItFound(t *testing.T) {
	for _, probe := range []struct {
		name    string
		command string
		want    string
		notWant string
	}{
		{
			name:    "the null an unparseable draft encodes to",
			command: `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":null,"height":400,"margin":{"top":10,"right":10,"bottom":10,"left":10}}`,
			want:    "page.width: width must be a number",
			notWant: "overflows millipoints",
		},
		{
			name:    "an emptied margin draft",
			command: `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":300,"height":400,"margin":{"top":null,"right":10,"bottom":10,"left":10}}`,
			want:    "page.margin.top: top must be a number",
			notWant: "overflows millipoints",
		},
		{
			name:    "a genuine overflow, whose message must not move",
			command: `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":99999999999999999999,"height":400,"margin":{"top":10,"right":10,"bottom":10,"left":10}}`,
			want:    "page.width: width overflows millipoints",
			notWant: "must be a number",
		},
		{
			name:    "an overflow in a margin, on the same loop",
			command: `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":300,"height":400,"margin":{"top":99999999999999999999,"right":10,"bottom":10,"left":10}}`,
			want:    "page.margin.top: top overflows millipoints",
			notWant: "must be a number",
		},
		{
			// The fraction loop, seventeen lines below the split, whose wording
			// the split adopted. It has always said this, and still does.
			name:    "a non-digit in the fraction, unchanged",
			command: `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":300,"height":400,"margin":{"top":1.0e1,"right":10,"bottom":10,"left":10}}`,
			want:    "must be a decimal with at most three places",
			notWant: "overflows millipoints",
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			input, err := os.ReadFile("testdata/template/golden/worked-example.json")
			if err != nil {
				t.Fatal(err)
			}
			tpl, err := ParseTemplate(input)
			if err != nil {
				t.Fatal(err)
			}
			_, err = applyPageSetupCommand(tpl, []byte(probe.command))
			if err == nil {
				t.Fatal("the command was accepted; no silent write may follow either cause")
			}
			if !strings.Contains(err.Error(), probe.want) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), probe.want)
			}
			if strings.Contains(err.Error(), probe.notWant) {
				t.Fatalf("error = %q, which still reports %q — the two causes are not distinguishable", err.Error(), probe.notWant)
			}
		})
	}
}
