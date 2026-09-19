package folio8

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
)

// THE ONE TEST THAT READS BOTH LANGUAGES.
//
// "A command means exactly what it names" has two enforcement points and needs
// both. The designer must be unable to ENCODE an ambiguous command, and the
// engine must REFUSE one rather than resolve it silently by last-wins. Shipping
// either half alone is the pattern that produced this project's existing
// untied invariants: without the Go half the only available assertion is "the
// encoder produces well-formed JSON", which is a test of the fix and goes green
// the moment a future encoder regresses; without the TypeScript half the engine
// refuses bytes the product never emits.
//
// So this file asserts the pair, in one test, in the mechanism
// canvas_projection_wire_test.go established for engine-protocol.ts: read the
// other language's source, extract with an ANCHORED regexp, compare against a
// record kept here, and t.Fatal — never t.Skip — if the file or the anchor has
// gone. The anchoring rule matters as much as the reading: an unanchored match
// reads some other declaration and compares it to this record, which is a green
// test asserting the wrong thing.

// commandJsonQuoter extracts the designer authority's STRING quoter. Anchored
// on its own exported name because `JSON.stringify` appears more than once in
// that module and an unanchored match would read whichever came first.
var commandJsonQuoter = regexp.MustCompile(`(?m)^export const jsonString = \(value: string\): string => (.+)$`)

// commandJsonNumberGrammar extracts the shape test the designer applies to a
// numeric draft, and it is the assertion this seam exists for.
//
// The designer must emit JSON *numbers*, unquoted — 52 shipped assertions pin
// that form — and it must emit the author's own literal, not a re-computation
// of it. An earlier implementation used `JSON.stringify(Number(v))` and thereby
// WIDENED this module's accept-set: `1e3` arrived as `1000`, `0x10` as `16`,
// `007` as `7`, `9007199254740993` as `...992`, each of which Go had refused or
// received exactly before. So what is pinned here is the grammar itself, and
// that it is the JSON number grammar rather than something narrower: a
// decimals-only check would send `1e3` as `null` and cost the located
// "must be a decimal with at most three places" this package answers with.
var commandJsonNumberGrammar = regexp.MustCompile(`(?m)^const JSON_NUMBER = /\^(.+)\$/$`)

// commandJsonNumberEmitter extracts the emitter body, to assert it TESTS the
// draft and passes it through rather than converting it.
var commandJsonNumberEmitter = regexp.MustCompile(`(?s)export const jsonNumber = \(value: number \| string\): string => \{(.*?)\n\}`)

// commandJsonEnvelope extracts the envelope every builder shares, which is what
// makes the version and the kind unspliceable.
var commandJsonEnvelope = regexp.MustCompile(`(?s)export const commandBytes = \(kind: string, fields: ReadonlyArray<JsonField>\): ArrayBuffer =>\s*\n\s*(.+?)\n`)

func TestCommandJsonAuthorityAndTheEnginesRefusalLandTogether(t *testing.T) {
	path := filepath.Join(repoRootFromTest(t), "folio-designer", "src", "command-json.ts")
	source, err := os.ReadFile(path)
	if err != nil {
		// Not a skip. The encoder on the other side of this seam is half of the
		// property, and a missing one is a finding, not an excuse.
		t.Fatalf("read the designer's command-JSON authority: %v", err)
	}

	quoter := commandJsonQuoter.FindSubmatch(source)
	if quoter == nil {
		t.Fatal("command-json.ts no longer exports a jsonString whose body this test can read; if the authority was restructured, re-derive this extraction rather than deleting the check")
	}
	if got, want := strings.TrimSpace(string(quoter[1])), "JSON.stringify(value)"; got != want {
		t.Errorf("the designer's string quoter is\n\t%s\nand this side records\n\t%s — a hand-rolled escape table is how a bind segment and an asset key silently bound somewhere the author never typed, twice", got, want)
	}

	grammar := commandJsonNumberGrammar.FindSubmatch(source)
	if grammar == nil {
		t.Fatal("command-json.ts no longer declares a JSON_NUMBER grammar this test can read; if the authority was restructured, re-derive this extraction rather than deleting the check")
	}
	if got, want := string(grammar[1]), `-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?`; got != want {
		t.Errorf("the designer tests a numeric draft against\n\t%s\nand this side records the JSON number grammar\n\t%s — narrower would send `1e3` as null and cost the located refusal this package answers with; wider would let the browser decide what a number is", got, want)
	}

	emitter := commandJsonNumberEmitter.FindSubmatch(source)
	if emitter == nil {
		t.Fatal("command-json.ts no longer exports a jsonNumber whose body this test can read; if the authority was restructured, re-derive this extraction rather than deleting the check")
	}
	body := string(emitter[1])
	if !strings.Contains(body, "JSON_NUMBER.test(literal) ? literal : 'null'") {
		t.Errorf("the designer's number emitter body is\n\t%s\nand this side requires it to pass a matching draft through VERBATIM and emit `null` otherwise", strings.TrimSpace(body))
	}
	// The coercion this replaced, forbidden by name. It is the one edit that
	// would widen this package's accept-set while every other assertion here
	// stayed green.
	if strings.Contains(body, "Number(") {
		t.Errorf("the designer's number emitter converts the draft instead of testing it:\n\t%s\nA Number() round trip makes the browser a second authority — measured, it turned a typed `1e3` into 1000 and `9007199254740993` into ...992", strings.TrimSpace(body))
	}

	envelope := commandJsonEnvelope.FindSubmatch(source)
	if envelope == nil {
		t.Fatal("command-json.ts no longer exports a commandBytes envelope this test can read; if the authority was restructured, re-derive this extraction rather than deleting the check")
	}
	for _, required := range []string{"'kind'", "jsonString(kind)", "'version'", "jsonNumber(1)"} {
		if !strings.Contains(string(envelope[1]), required) {
			t.Errorf("the command envelope no longer builds %s through the authority:\n\t%s", required, strings.TrimSpace(string(envelope[1])))
		}
	}

	// THE GO HALF, asserted against the ENGINE and not against the encoder.
	// These are the exact bytes the designer's splices used to produce, handed
	// straight to the exported door. The property is that the engine refuses
	// them WHATEVER produced them.
	for _, probe := range []struct {
		name    string
		command string
	}{
		{
			// DW-32's payload, typed into a numeric field: valid JSON that
			// parsed to a different target AND a different change, with the
			// author's own selection gone.
			name:    "the typed-draft injection",
			command: `{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"set","value":0}},"ids":["other"],"changes":{"width":{"op":"set","value":10}}}`,
		},
		{
			// The hostile document id, which needed no typing at all: open a
			// .folio, select the element, press Delete.
			name:    "the hostile document id",
			command: `{"kind":"deleteComponent","version":1,"id":"a","id":"victim"}`,
		},
		{
			// DW-73's payload, typed into the page width box.
			name:    "the page-setup splice",
			command: `{"kind":"pageSetup","version":1,"preset":"A4","orientation":"portrait","width":0,"preset":"custom","orientation":"landscape","height":9999,"margin":{"top":10,"right":10,"bottom":10,"left":10}}`,
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			tpl := componentTemplate(t)
			pageSetup := strings.Contains(probe.command, `"pageSetup"`)
			var err error
			if pageSetup {
				_, err = applyPageSetupCommand(tpl, []byte(probe.command))
			} else {
				_, err = applyComponentCommand(tpl, []byte(probe.command))
			}
			if err == nil {
				t.Fatal("the engine accepted the bytes an unquoted splice used to produce")
			}
			var failure *designer.ComponentCommandError
			// EACH DOOR IN ITS OWN SHAPE. The page-setup door must NOT raise a
			// ComponentCommandError: the host matches that type before the
			// page-setup fallback, so it would answer COMPONENT_INVALID for a
			// page-setup command and miss the only consumer that branches on
			// PAGE_SETUP_INVALID.
			if pageSetup {
				if errors.As(err, &failure) {
					t.Fatalf("the page-setup door raised a ComponentCommandError, so the host reports COMPONENT_INVALID: %v", err)
				}
				if !strings.HasPrefix(err.Error(), pageSetupFailurePrefix) {
					t.Fatalf("error = %q, want the prefix %q the host answers PAGE_SETUP_INVALID for", err.Error(), pageSetupFailurePrefix)
				}
				return
			}
			if !errors.As(err, &failure) {
				t.Fatalf("the component door did not refuse with a located error: %v", err)
			}
			if failure.ElementID != "" {
				t.Fatalf("refusal named the element %q, which the duplicate has made untrustworthy", failure.ElementID)
			}
		})
	}
}

// TestEveryDesignerCommandFactoryRoutesThroughTheAuthority is the soleness half
// of the seam, read from THIS side. The designer has its own allowlist guard
// over the same question (command-json-soleness.test.ts); this one exists
// because a Go-side narrowing and a TypeScript-side consolidation land in one
// commit, and a reader of the Go half must be able to see the encoder set the
// refusal was designed against without changing languages.
func TestEveryDesignerCommandFactoryRoutesThroughTheAuthority(t *testing.T) {
	root := filepath.Join(repoRootFromTest(t), "folio-designer", "src")
	factories := []string{
		"component-command.ts",
		"component-asset-command.ts",
		"component-property-command.ts",
		"page-setup-command.ts",
		"table-column-command.ts",
		"font-chain-command.ts",
	}
	// Anchored on the import statement's own shape, so a file that merely
	// mentions the authority in a comment does not read as routed through it.
	importsAuthority := regexp.MustCompile(`(?m)^import \{[^}]*\} from '\./command-json'$`)
	var routed []string
	for _, factory := range factories {
		source, err := os.ReadFile(filepath.Join(root, factory))
		if err != nil {
			t.Fatalf("read the designer command factory %s: %v", factory, err)
		}
		if importsAuthority.Match(source) {
			routed = append(routed, factory)
		}
		// The escape table is JSON.stringify and is never re-derived. A
		// charCodeAt-based table is the exact defect this story deleted twice,
		// and both copies of it read the FIRST UTF-16 unit of a value iterated
		// by code point — so an astral character was escaped from its high
		// surrogate alone and its low unit was never emitted.
		//
		// Read with line comments dropped, because the files that used to carry
		// this table now carry PROSE about it. A guard that reddens on the note
		// explaining what not to write gets answered by deleting the note.
		if strings.Contains(withoutLineComments(string(source)), "charCodeAt(") {
			t.Errorf("%s hand-rolls an escape table again; JSON.stringify is the only correct answer and it lives in command-json.ts", factory)
		}
	}
	if !reflect.DeepEqual(routed, factories) {
		t.Errorf("the designer command factories routed through the authority are\n\t%v\nand this side records\n\t%v — a factory that builds its own command JSON is a second answer to \"escape this\", which is what made a command able to name one thing and change another", routed, factories)
	}
}

// withoutLineComments drops whole-line `//` comments. It is deliberately not a
// general comment stripper: the check above only has to survive the prose these
// six files now carry, and a half-correct scanner that mistook the `//` inside
// a URL for a comment would make the guard vacuous exactly where it matters.
func withoutLineComments(source string) string {
	var kept []string
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
