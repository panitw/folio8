package folio8

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
)

// THE CLAIM UNDER TEST IS ATOMICITY, AND A REFUSAL TEST THAT ONLY ASSERTS "an
// error came back" DOES NOT TOUCH IT. Several handlers mutate their own
// candidate before refusing, so the only assertion that means anything is a
// byte comparison of the serialized template across the refused call. Every
// refusal helper in this file makes that comparison, and none of them would
// notice a missing error alone.

func commandUnit(members ...string) string {
	return `{"kind":"` + unitCommandKind + `","version":1,"commands":[` + strings.Join(members, ",") + `]}`
}

// commandUnitRefusal applies unit, requires it to be refused, and requires the
// document to be byte-identical afterwards.
func commandUnitRefusal(t *testing.T, tpl *Template, unit string) error {
	t.Helper()
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	_, applyErr := applyComponentCommand(tpl, []byte(unit))
	if applyErr == nil {
		t.Fatalf("unit unexpectedly succeeded: %s", unit)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("a refused unit mutated the document: %s", unit)
	}
	return applyErr
}

// commandUnitLocatedRefusal is commandUnitRefusal for the unit's OWN refusals:
// the bounds, the nesting rule and an unreadable member list. They name no
// element and carry the unit's own path.
func commandUnitLocatedRefusal(t *testing.T, tpl *Template, unit string) *designer.ComponentCommandError {
	t.Helper()
	applyErr := commandUnitRefusal(t, tpl, unit)
	var failure *designer.ComponentCommandError
	if !errors.As(applyErr, &failure) {
		t.Fatalf("refusal for %s is %T (%v), want *ComponentCommandError", unit, applyErr, applyErr)
	}
	if failure.ElementID != "" {
		t.Errorf("a unit's own refusal carries elementId %q; a unit is not addressed to an element", failure.ElementID)
	}
	if failure.DataPath != unitCommandPath {
		t.Errorf("dataPath = %q, want %q", failure.DataPath, unitCommandPath)
	}
	return failure
}

// TestCommandUnitAppliesItsMembersInOrderAgainstOneDocument is the matrix's
// "two members both succeed" row at the public seam. The second member NAMES
// the chain the first member creates, so the test would fail on any
// implementation that applied members to separate copies or out of order —
// fontFamily refuses a chain the document does not declare.
func TestCommandUnitAppliesItsMembersInOrderAgainstOneDocument(t *testing.T) {
	tpl := fontChainTemplate(t)
	unit := commandUnit(
		`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`,
		`{"kind":"updateComponentProperties","version":1,"ids":["e7"],"changes":{"fontFamily":{"op":"set","value":"caption"}}}`,
	)
	if _, err := applyComponentCommand(tpl, []byte(unit)); err != nil {
		t.Fatalf("unit refused: %v", err)
	}
	if got := fontChainOf(t, tpl, "caption"); len(got) != 1 || got[0] != "Noto Sans" {
		t.Fatalf("caption chain = %v, want the entry the first member added", got)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(after, []byte(`"fontFamily": "caption"`)) {
		t.Fatal("the second member's property change is absent; a unit applies every member")
	}
}

// TestCommandUnitFusesTheEmbedAndPropertyShape is the pair story 2 will send:
// a font-chain edit and a property edit in one unit. It is kept separate from
// the ordering test above so the matrix row that names these two kinds is
// asserted in its own words.
func TestCommandUnitFusesTheEmbedAndPropertyShape(t *testing.T) {
	tpl := fontChainTemplate(t)
	unit := commandUnit(
		`{"kind":"addFontChainEntry","version":1,"name":"body","index":0,"face":"Noto Sans SC"}`,
		`{"kind":"updateComponentProperties","version":1,"ids":["e7"],"changes":{"fontSize":{"op":"set","value":10}}}`,
	)
	if _, err := applyComponentCommand(tpl, []byte(unit)); err != nil {
		t.Fatalf("unit refused: %v", err)
	}
	if got, want := strings.Join(fontChainOf(t, tpl, "body"), ","), "Noto Sans SC,Noto Sans,Noto Sans Thai"; got != want {
		t.Fatalf("body chain = %q, want %q", got, want)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(after, []byte(`"fontSize": 10`)) {
		t.Fatal("the property member did not land")
	}
}

// TestCommandUnitOfOneIsAdmitted is D-6.4's minimum, from the admitting side:
// a caller assembling a list whose length it does not know in advance needs no
// special case.
func TestCommandUnitOfOneIsAdmitted(t *testing.T) {
	tpl := fontChainTemplate(t)
	unit := commandUnit(`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`)
	if _, err := applyComponentCommand(tpl, []byte(unit)); err != nil {
		t.Fatalf("a unit of one was refused: %v", err)
	}
	if got := fontChainOf(t, tpl, "caption"); len(got) != 1 {
		t.Fatalf("caption chain = %v", got)
	}
}

// TestCommandUnitMemberRefusalPropagatesVerbatimAndCommitsNothing covers the
// matrix's two refusing-member rows. The expected error is not spelled out
// here: it is MEASURED by applying the same member alone to a fresh document,
// which is the only way to assert "verbatim" without copying the member's
// wording into this file and letting the two drift.
func TestCommandUnitMemberRefusalPropagatesVerbatimAndCommitsNothing(t *testing.T) {
	for _, row := range []struct {
		name    string
		member  string
		members []string
	}{
		{
			name:   "later member names a missing element",
			member: `{"kind":"updateComponentProperties","version":1,"ids":["ezmissing"],"changes":{"fontSize":{"op":"set","value":10}}}`,
			members: []string{
				`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`,
				`{"kind":"updateComponentProperties","version":1,"ids":["ezmissing"],"changes":{"fontSize":{"op":"set","value":10}}}`,
			},
		},
		{
			name:   "first member is malformed",
			member: `{"kind":"addFontChain","version":1,"name":"caption"}`,
			members: []string{
				`{"kind":"addFontChain","version":1,"name":"caption"}`,
				`{"kind":"addFontChain","version":1,"name":"aside","entries":["Noto Sans"]}`,
			},
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			alone := fontChainTemplate(t)
			_, bare := applyComponentCommand(alone, []byte(row.member))
			if bare == nil {
				t.Fatalf("the member was expected to refuse on its own: %s", row.member)
			}
			tpl := fontChainTemplate(t)
			inUnit := commandUnitRefusal(t, tpl, commandUnit(row.members...))
			if inUnit.Error() != bare.Error() {
				t.Fatalf("in a unit the refusal is %q; alone it is %q — a member is unaware it is in a group", inUnit.Error(), bare.Error())
			}
			var inUnitFailure, bareFailure *designer.ComponentCommandError
			if errors.As(bare, &bareFailure) {
				if !errors.As(inUnit, &inUnitFailure) {
					t.Fatalf("the bare refusal is located and the unit's is %T", inUnit)
				}
				if inUnitFailure.ElementID != bareFailure.ElementID || inUnitFailure.DataPath != bareFailure.DataPath || inUnitFailure.Message != bareFailure.Message {
					t.Fatalf("located refusal = %q/%q/%q, want %q/%q/%q",
						inUnitFailure.ElementID, inUnitFailure.DataPath, inUnitFailure.Message,
						bareFailure.ElementID, bareFailure.DataPath, bareFailure.Message)
				}
			}
		})
	}
}

// TestCommandUnitNetNoOpLeavesTheDocumentByteIdentical is the matrix's no-op
// row at the public seam. Members that cancel are individually valid and
// individually committed to the working copy; what must be unchanged is the
// NET result.
func TestCommandUnitNetNoOpLeavesTheDocumentByteIdentical(t *testing.T) {
	tpl := fontChainTemplate(t)
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	unit := commandUnit(
		`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`,
		`{"kind":"deleteFontChain","version":1,"name":"caption"}`,
	)
	if _, err := applyComponentCommand(tpl, []byte(unit)); err != nil {
		t.Fatalf("a net no-op unit was refused: %v", err)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("members that cancel left the document changed")
	}
}

// TestCommandUnitInheritsTheExistingDuplicateKeyGuard is the matrix row that
// must NOT be answered by new code: refuseDuplicateCommandKeys already walks
// arrays and nested objects, so a member's duplicate key is caught at the outer
// door before any member runs. The first member here would change the document
// if it ever ran.
func TestCommandUnitInheritsTheExistingDuplicateKeyGuard(t *testing.T) {
	tpl := fontChainTemplate(t)
	unit := commandUnit(
		`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`,
		`{"kind":"updateComponentProperties","version":1,"ids":["e7"],"ids":["e2"],"changes":{"fontSize":{"op":"set","value":10}}}`,
	)
	err := commandUnitRefusal(t, tpl, unit)
	var failure *designer.ComponentCommandError
	if !errors.As(err, &failure) {
		t.Fatalf("refusal is %T (%v), want the duplicate-key guard's located error", err, err)
	}
	if !strings.Contains(failure.Message, `declares the key "ids" twice`) {
		t.Fatalf("message = %q, want the existing duplicate-key refusal", failure.Message)
	}
	if failure.DataPath != componentCommandPath {
		t.Fatalf("dataPath = %q, want the outer door's %q — this refusal is the existing guard's, not the unit's", failure.DataPath, componentCommandPath)
	}
}

// TestCommandUnitRefusesMembersOutsideTheClosedVocabulary covers the "unknown
// kind" and "pageSetup member" rows together, because under D-6.1 they are
// literally the same refusal: the component switch's own default.
func TestCommandUnitRefusesMembersOutsideTheClosedVocabulary(t *testing.T) {
	for _, member := range []string{
		`{"kind":"notACommand","version":1}`,
		`{"kind":"pageSetup","version":1,"size":"A4","orientation":"landscape","margin":{"top":36,"right":36,"bottom":36,"left":36}}`,
	} {
		tpl := fontChainTemplate(t)
		unit := commandUnit(
			`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`,
			member,
		)
		err := commandUnitRefusal(t, tpl, unit)
		if err.Error() != "folio8: unknown component command" {
			t.Fatalf("refusal for %s is %q, want the closed switch's own default", member, err.Error())
		}
	}
}

// TestCommandUnitRefusesAUnitInsideAUnit is D-6.3. It matters that the refusal
// arrives BEFORE any member runs: the first member would otherwise commit.
func TestCommandUnitRefusesAUnitInsideAUnit(t *testing.T) {
	tpl := fontChainTemplate(t)
	inner := commandUnit(`{"kind":"addFontChain","version":1,"name":"aside","entries":["Noto Sans"]}`)
	unit := commandUnit(
		`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`,
		inner,
	)
	failure := commandUnitLocatedRefusal(t, tpl, unit)
	if !strings.Contains(failure.Message, "another unit") {
		t.Fatalf("message = %q, want a refusal naming the nesting rule", failure.Message)
	}
}

// TestCommandUnitBoundsAreStatedAndProvedOnBothSides is D-6.4.
//
// THE NUMBERS ARE WRITTEN OUT HERE, not read from minUnitCommands and
// maxUnitCommands, and that is the difference between a test and a mirror. A
// test that sized its input from the constant would follow the constant
// wherever it went and go green on 8 or on 8 000; this one reds the moment
// either bound moves, which is what "the bound is proved by a red-proof that
// edits the constant" asks for.
//
// The maximum is asserted on both sides here — a test that only refused 65
// would pass on a mechanism that refused 64 as well. The minimum is asserted on
// its refusing side here; its ADMITTING side is TestCommandUnitOfOneIsAdmitted,
// which is the assertion that stops a mechanism refusing a unit of one from
// going green.
func TestCommandUnitBoundsAreStatedAndProvedOnBothSides(t *testing.T) {
	const (
		mostCommandsAUnitCarries  = 64
		fewestCommandsAUnitNeeds  = 1
		commandsBeyondTheMaximum  = mostCommandsAUnitCarries + 1
		lastAdmittedMemberOrdinal = mostCommandsAUnitCarries - 1
	)
	if maxUnitCommands != mostCommandsAUnitCarries || minUnitCommands != fewestCommandsAUnitNeeds {
		t.Fatalf("the bounds are %d..%d and this file records %d..%d; the wire contract changed, so change it here deliberately rather than letting a test follow it",
			minUnitCommands, maxUnitCommands, fewestCommandsAUnitNeeds, mostCommandsAUnitCarries)
	}
	member := func(i int) string {
		return fmt.Sprintf(`{"kind":"addFontChain","version":1,"name":"c%d","entries":["Noto Sans"]}`, i)
	}
	members := make([]string, 0, commandsBeyondTheMaximum)
	for i := 0; i < commandsBeyondTheMaximum; i++ {
		members = append(members, member(i))
	}

	full := fontChainTemplate(t)
	if _, err := applyComponentCommand(full, []byte(commandUnit(members[:mostCommandsAUnitCarries]...))); err != nil {
		t.Fatalf("a unit of exactly %d commands was refused: %v", mostCommandsAUnitCarries, err)
	}
	if got := fontChainOf(t, full, fmt.Sprintf("c%d", lastAdmittedMemberOrdinal)); len(got) != 1 {
		t.Fatalf("the last admitted member did not run: %v", got)
	}

	over := commandUnitLocatedRefusal(t, fontChainTemplate(t), commandUnit(members...))
	if !strings.Contains(over.Message, fmt.Sprintf("%d", mostCommandsAUnitCarries)) {
		t.Fatalf("message = %q, want a refusal naming the bound %d", over.Message, mostCommandsAUnitCarries)
	}

	empty := commandUnitLocatedRefusal(t, fontChainTemplate(t), commandUnit())
	if !strings.Contains(empty.Message, fmt.Sprintf("%d", fewestCommandsAUnitNeeds)) {
		t.Fatalf("message = %q, want a refusal naming the minimum %d", empty.Message, fewestCommandsAUnitNeeds)
	}
}

// TestCommandUnitRefusesAnUnreadableMemberList closes the gap between "commands
// is missing" and "commands is not an array".
//
// IT ASSERTS THE WORDING, not merely that something was refused, because an
// unreadable member list decodes to no members at all and would otherwise fall
// through to the minimum's refusal — which would tell an author their unit
// carries too few commands when what they actually sent was not a list. A test
// that accepted either message would be a test the guard could be deleted
// under.
func TestCommandUnitRefusesAnUnreadableMemberList(t *testing.T) {
	for _, unit := range []string{
		`{"kind":"` + unitCommandKind + `","version":1,"commands":"addFontChain"}`,
		`{"kind":"` + unitCommandKind + `","version":1,"commands":7}`,
		`{"kind":"` + unitCommandKind + `","version":1,"commands":{"kind":"addFontChain"}}`,
	} {
		failure := commandUnitLocatedRefusal(t, fontChainTemplate(t), unit)
		if !strings.Contains(failure.Message, "an array of commands") {
			t.Fatalf("message = %q for %s, want a refusal saying the member list is not a list", failure.Message, unit)
		}
	}
	// A null member list decodes to no members at all and is therefore the
	// minimum's refusal, not the unreadable one.
	null := commandUnitLocatedRefusal(t, fontChainTemplate(t), `{"kind":"`+unitCommandKind+`","version":1,"commands":null}`)
	if !strings.Contains(null.Message, "at least") {
		t.Fatalf("message = %q, want the minimum's refusal", null.Message)
	}
}

// TestCommandUnitRefusesAMissingMemberListInItsOwnWords is the case the arity
// check cannot catch: three keys, so componentFields is satisfied, but the
// third is not `commands`. Telling that author their command list is not an
// array would describe a list they never sent, so the two mistakes are worded
// apart.
func TestCommandUnitRefusesAMissingMemberListInItsOwnWords(t *testing.T) {
	missing := commandUnitLocatedRefusal(t, fontChainTemplate(t), `{"kind":"`+unitCommandKind+`","version":1,"atomic":true}`)
	if !strings.Contains(missing.Message, "declare a commands field") {
		t.Fatalf("message = %q, want a refusal saying the member list is absent", missing.Message)
	}
	if strings.Contains(missing.Message, "an array of commands") {
		t.Fatalf("message = %q — an absent member list is not an unreadable one", missing.Message)
	}
}

// TestCommandUnitArityIsTheOrdinaryComponentFieldsCheck is the matrix's "wrong
// arity on the unit itself" row: three keys and no more, answered by the same
// componentFields every other command uses rather than by a bespoke check.
func TestCommandUnitArityIsTheOrdinaryComponentFieldsCheck(t *testing.T) {
	for _, unit := range []string{
		`{"kind":"` + unitCommandKind + `","version":1,"commands":[{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}],"atomic":true}`,
		`{"kind":"` + unitCommandKind + `","version":1}`,
	} {
		err := commandUnitRefusal(t, fontChainTemplate(t), unit)
		if err.Error() != "folio8: component command has unknown or missing fields" {
			t.Fatalf("refusal for %s is %q, want componentFields' own error", unit, err.Error())
		}
	}
}

// TestCommandUnitVersionIsTheOrdinaryVersionCheck keeps the unit inside the
// versioned vocabulary rather than beside it.
func TestCommandUnitVersionIsTheOrdinaryVersionCheck(t *testing.T) {
	unit := `{"kind":"` + unitCommandKind + `","version":2,"commands":[{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}]}`
	if err := commandUnitRefusal(t, fontChainTemplate(t), unit); err.Error() != "folio8: unknown component command" {
		t.Fatalf("refusal = %q, want the door's version refusal", err.Error())
	}
}

// TestExactlyOneFunctionReadsAUnitsMemberList is D-6.2's design obligation,
// asserted STRUCTURALLY because it is a claim about the source tree and
// nothing about a passing behaviour test would notice a second parser
// appearing. The obligation was accepted knowingly: generality was chosen over
// a stated limit, and duplicating a unit's structure into a second package is
// the defect that choice risks.
//
// It is a source read, not a skip: a missing file or a moved call is a finding.
func TestExactlyOneFunctionReadsAUnitsMemberList(t *testing.T) {
	root := filepath.Join(repoRootFromTest(t), "folio-go")
	readers := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(source, []byte(`"commands"`)) {
			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			readers = append(readers, relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(readers) != 1 || readers[0] != "component_commands.go" {
		t.Fatalf("the unit's member list is read in %v; exactly one function in component_commands.go may know a unit's shape, and every other package must ask through internal/designer's bridge", readers)
	}

	// ONE FILE IS NOT ONE FUNCTION. The scan above would stay green with five
	// functions in component_commands.go each reading raw["commands"] — which
	// is precisely what the message it prints forbids — so the number of reads
	// inside that one file is pinned too.
	owner, err := os.ReadFile(filepath.Join(root, "component_commands.go"))
	if err != nil {
		t.Fatal(err)
	}
	if reads := bytes.Count(owner, []byte(`"commands"`)); reads != 1 {
		t.Fatalf("component_commands.go reads the member list %d times; exactly ONE function may, or the applier and the fence can be given different answers", reads)
	}

	engine, err := os.ReadFile(filepath.Join(root, "internal", "wasm", "engine.go"))
	if err != nil {
		t.Fatalf("read the engine that holds the staleness fence: %v", err)
	}
	if !bytes.Contains(engine, []byte("designer.CarriedCommands(command)")) {
		t.Fatal("internal/wasm no longer asks folio8 what a command carries; if the fence was restructured, re-derive this check rather than deleting it — a second parser for a unit's members is exactly what it exists to catch")
	}
}

// ---------------------------------------------------------------------------
// SPEC-INSTALL-ALL-FACE-CUTS STORY 2: THE REAL PAIR.
//
// TestCommandUnitFusesTheEmbedAndPropertyShape above stands in for this pair
// with two kinds that were already writable when story 6 landed. These two
// tests are the pair itself, now that `embedFontCut` exists: pressing B on a
// family whose bold is held sends ONE unit carrying the cut embed and the
// ordinary property commit, and they travel as one undo step WITHOUT being
// fused into one kind.

// TestCommandUnitEmbedsACutAndCommitsTheProperty is the story's first matrix
// row at the public seam: one unit, both effects, one document.
//
// THE ORDER IS FORCED BY THE ENGINE AND THE TEST MEASURES IT. The cut must be
// embedded before the property is committed, and a unit applies its members in
// order against ONE candidate — so an implementation that applied them to
// separate copies, or reordered them, could not produce this state.
func TestCommandUnitEmbedsACutAndCommitsTheProperty(t *testing.T) {
	tpl, _, baseKey := embeddedChainTemplate(t)
	fontChainAccepted(t, tpl, `{"kind":"updateComponentProperties","version":1,"ids":["e7"],"changes":{"fontFamily":{"op":"set","value":"Noto Sans Thai"}}}`)
	bold := testShippedNotoSans
	boldKey := embeddedKeyOf(bold)

	unit := commandUnit(
		embedCutCommand(t, "Noto Sans Thai", 0, "bold", bold),
		`{"kind":"updateComponentProperties","version":1,"ids":["e7"],"changes":{"bold":{"op":"set","value":true}}}`,
	)
	if _, err := applyComponentCommand(tpl, []byte(unit)); err != nil {
		t.Fatalf("unit refused: %v", err)
	}

	entry := tpl.doc.Fonts["Noto Sans Thai"][0]
	if entry.AssetKey != baseKey {
		t.Errorf("the base moved to %q; the embedded Regular is never touched", entry.AssetKey)
	}
	if entry.Bold != boldKey {
		t.Fatalf("entry.Bold = %q, want %q — the embed member did not land", entry.Bold, boldKey)
	}
	if _, ok := tpl.doc.Assets[boldKey]; !ok {
		t.Fatal("the cut's bytes are not in the document")
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(after, []byte(`"bold": true`)) {
		t.Fatal("the property member did not land, so the author pressed B and nothing was bolded")
	}
}

// TestCommandUnitRefusesTheWholePairWhenTheCutIsRefused is the matrix's
// "member refused" row: the WHOLE unit is refused, so `bold` does not commit,
// the document is unchanged, and the member's refusal travels out verbatim
// with its own located path.
//
// commandUnitRefusal asserts the byte identity, which is the strongest form of
// "bold was not committed" — a handler that mutated its candidate and then
// refused would pass a bare "an error came back" check.
func TestCommandUnitRefusesTheWholePairWhenTheCutIsRefused(t *testing.T) {
	tpl, _, _ := embeddedChainTemplate(t)
	fontChainAccepted(t, tpl, `{"kind":"updateComponentProperties","version":1,"ids":["e7"],"changes":{"fontFamily":{"op":"set","value":"Noto Sans Thai"}}}`)

	// A VARIABLE face: admitted by nothing, and refused at the same gate the
	// pick is refused at.
	member := embedCutCommand(t, "Noto Sans Thai", 0, "bold", testNotoSansThaiVariableFontBytes)
	unit := commandUnit(member, `{"kind":"updateComponentProperties","version":1,"ids":["e7"],"changes":{"bold":{"op":"set","value":true}}}`)

	fromUnit := commandUnitRefusal(t, tpl, unit)

	// VERBATIM, MEASURED rather than copied: the same member applied alone to a
	// fresh document must produce the identical located refusal.
	alone, _, _ := embeddedChainTemplate(t)
	fromMember := fontChainRefusal(t, alone, member)
	var located *designer.ComponentCommandError
	if !errors.As(fromUnit, &located) {
		t.Fatalf("the unit's refusal is %T, want the member's own *ComponentCommandError", fromUnit)
	}
	if located.Message != fromMember.Message || located.DataPath != fromMember.DataPath || located.ElementID != fromMember.ElementID {
		t.Fatalf("the member's refusal was not passed through verbatim:\n  unit: %#v\nmember: %#v", located, fromMember)
	}
	if located.DataPath != fontChainEntryPath("Noto Sans Thai", 0) {
		t.Errorf("the refusal lost the member's ENTRY location: %q", located.DataPath)
	}
}

// TestAUnitIsBoundedByTheWHOLEPayloadAndNotOnlyPerMember closes the gap the
// per-member bound structurally cannot see.
//
// maxComponentAssetBytes is derived so that ONE member carrying one asset,
// base64-inflated, still fits inside engineProtocolMaxPayloadBytes. A unit
// carries several — spec-install-all-face-cuts story 2 sends one embedFontCut
// per family needing a cut, so a multi-family selection puts three or four
// faces in one command. Every member can clear its own bound while the unit is
// many times the envelope, and that unit fails at TRANSPORT with no located
// diagnostic at all: the threshold disagreement D-5.13.4 forbids, one level up.
//
// BOTH SIDES OF THE BOUND ARE MEASURED. A bound asserted only from above is
// satisfied by a function that refuses everything.
func TestAUnitIsBoundedByTheWHOLEPayloadAndNotOnlyPerMember(t *testing.T) {
	// A member that is LEGAL on its own terms and enormous: the chain name is
	// padding, so nothing else about it can be what is refused. It is checked
	// against the per-member rule first, below.
	oversizeMember := func(bytes int) string {
		return `{"kind":"deleteFontChain","version":1,"name":"` + strings.Repeat("x", bytes) + `"}`
	}

	t.Run("a unit past the envelope is refused, located, with nothing written", func(t *testing.T) {
		tpl := fontChainTemplate(t)
		unit := commandUnit(oversizeMember(maxUnitPayloadBytes/2), oversizeMember(maxUnitPayloadBytes/2), oversizeMember(maxUnitPayloadBytes/2))
		if len(unit) <= maxUnitPayloadBytes {
			t.Fatalf("precondition: the fixture unit is %d bytes, which is inside the bound it is meant to exceed", len(unit))
		}
		failure := commandUnitLocatedRefusal(t, tpl, unit)
		for _, want := range []string{"at most", "in total", "one member"} {
			if !strings.Contains(failure.Message, want) {
				t.Errorf("the refusal does not say this is a UNIT-level bound rather than a member one (%q): %s", want, failure.Message)
			}
		}
	})

	t.Run("a unit inside the envelope is admitted", func(t *testing.T) {
		// THE OTHER SIDE, and it is what stops the bound from being "refuse
		// everything". One ordinary member, well inside the envelope.
		tpl := fontChainTemplate(t)
		unit := commandUnit(`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`)
		if len(unit) > maxUnitPayloadBytes {
			t.Fatalf("precondition: an ordinary one-member unit is already past the bound at %d bytes", len(unit))
		}
		if _, err := applyComponentCommand(tpl, []byte(unit)); err != nil {
			t.Fatalf("an ordinary unit was refused by the payload bound: %v", err)
		}
	})

	t.Run("the bound is the envelope and not the per-member derivation", func(t *testing.T) {
		// ⚠ THE TWO NUMBERS ARE DIFFERENT, AND THE TEST ABOVE IS ONLY A
		// MEASUREMENT BECAUSE THEY ARE. maxComponentAssetBytes subtracts a
		// skeleton and applies the 4/3 base64 expansion to reach a DECODED
		// size; a unit's members are already encoded, so neither adjustment
		// applies twice and the bound is the envelope itself. If these ever
		// became equal, a unit-level check would be indistinguishable from a
		// member-level one.
		if maxUnitPayloadBytes != engineProtocolMaxPayloadBytes {
			t.Errorf("maxUnitPayloadBytes = %d, want the envelope %d", maxUnitPayloadBytes, engineProtocolMaxPayloadBytes)
		}
		if maxUnitPayloadBytes <= maxComponentAssetBytes {
			t.Errorf("the unit bound (%d) is not above the per-member decoded bound (%d), so one legal member could not fit in a unit", maxUnitPayloadBytes, maxComponentAssetBytes)
		}
	})
}
