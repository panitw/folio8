package licence

import (
	"os"
	"testing"
)

// TestClassifyLicenceText is AC31's supplement: direct unit tests over
// licence-identification inputs, additional to the fixture graphs
// (AC29), never a substitute for them and never built from a recorded
// `go list -m -json all` output (D-1.3.8).
func TestClassifyLicenceText(t *testing.T) {
	cases := []struct {
		name       string
		text       string
		wantFamily Family
	}{
		{"SPDX MIT", "SPDX-License-Identifier: MIT\n", FamilyPermissive},
		{"SPDX Apache-2.0", "SPDX-License-Identifier: Apache-2.0\n", FamilyPermissive},
		{"SPDX BSD-3-Clause", "SPDX-License-Identifier: BSD-3-Clause\n", FamilyPermissive},
		{"SPDX GPL-3.0-only", "SPDX-License-Identifier: GPL-3.0-only\n", FamilyCopyleft},
		{"SPDX LGPL-3.0-only", "SPDX-License-Identifier: LGPL-3.0-only\n", FamilyCopyleft},
		{"SPDX AGPL-3.0-only", "SPDX-License-Identifier: AGPL-3.0-only\n", FamilyCopyleft},
		{"SPDX SSPL-1.0", "SPDX-License-Identifier: SSPL-1.0\n", FamilyCopyleft},
		{"SPDX CC0-1.0", "SPDX-License-Identifier: CC0-1.0\n", FamilyPermissive},                                  // AC8, D-2.1.3
		{"full CC0 legal code text marker", "Creative Commons Legal Code\n\nCC0 1.0 Universal", FamilyPermissive}, // AC8, D-2.1.3 — matches the committed LICENSE-CC0-1.0.txt shape
		// Red-proof for this story's second QA review, Major 1: the CC0
		// fallback marker must NOT match the boilerplate disclaimer
		// shared by every Creative Commons legal code. A NonCommercial
		// or ShareAlike licence classifying as permissive CC0-1.0 is a
		// silent wrong answer, not a loud miss (D-1.3.8, D-2.1.3). Before
		// this fix these three cases all classified FamilyPermissive/"CC0-1.0".
		{"CC BY-NC-SA legal code must NOT classify as CC0", "Creative Commons Legal Code\n\nCC BY-NC-SA 4.0\n\nCreative Commons Corporation is not a law firm and does not provide legal services or legal advice.", FamilyUnknown},
		{"CC BY-SA legal code must NOT classify as CC0", "Creative Commons Legal Code\n\nAttribution-ShareAlike 3.0\n\nCreative Commons Corporation is not a law firm and does not provide legal services or legal advice.", FamilyUnknown},
		{"CC BY-ND legal code must NOT classify as CC0", "Creative Commons Legal Code\n\nAttribution-NoDerivatives 4.0\n\nCreative Commons Corporation is not a law firm and does not provide legal services or legal advice.", FamilyUnknown},
		{"full GPL text marker", "GNU GENERAL PUBLIC LICENSE\nVersion 3", FamilyCopyleft},
		{"full AGPL text marker", "GNU AFFERO GENERAL PUBLIC LICENSE", FamilyCopyleft},
		{"full SSPL text marker", "Server Side Public License, Version 1", FamilyCopyleft},
		{"full MIT text marker", "MIT License\n\nPermission is hereby granted, free of charge", FamilyPermissive},
		{"empty", "", FamilyUnknown},
		{"unrecognised marker", "Some Proprietary Thing v2", FamilyUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotFamily, _ := ClassifyLicenceText(c.text)
			if gotFamily != c.wantFamily {
				t.Errorf("ClassifyLicenceText(%q) family = %v, want %v", c.text, gotFamily, c.wantFamily)
			}
		})
	}
}

// TestClassifyOFL covers the OFL branch, which shipped with no test at
// all — positive or negative — while the CC0 marker immediately below it
// carries three negative red-proofs added after an earlier QA finding on
// exactly this over-match class.
//
// The FAMILY is not the interesting assertion here: OFL is permissive
// whatever version it is, so the copyleft gate was never at risk. The
// SPDX ID is what lands in lint/MANIFEST.md and attributes each shipped
// face, so the id is asserted explicitly.
func TestClassifyOFL(t *testing.T) {
	// The real committed text's opening lines, verbatim in shape.
	const oflHeader = "Copyright 2022 The Noto Project Authors\n\n" +
		"This Font Software is licensed under the SIL Open Font License, Version 1.1.\n" +
		"This license is copied below, and is also available with a FAQ at:\n" +
		"https://openfontlicense.org\n\n" +
		"SIL OPEN FONT LICENSE\nVersion 1.1 - 26 February 2007\n"

	cases := []struct {
		name       string
		text       string
		wantFamily Family
		wantID     string
	}{
		{
			"OFL 1.1 full text classifies as OFL-1.1",
			oflHeader,
			FamilyPermissive, "OFL-1.1",
		},
		{
			// The defect the OFL branch exists to fix: OFL 1.1's grant
			// clause opens with MIT's detection phrase verbatim, so
			// before the branch was added ABOVE the MIT case, every
			// shipped face classified as "MIT".
			"OFL text carrying MIT's trigger phrase must NOT classify as MIT",
			oflHeader + "\nPermission is hereby granted, free of charge, to any person obtaining\n" +
				"a copy of the Font Software, to use, study, copy, merge, embed, modify,\n" +
				"redistribute, and sell modified and unmodified copies of the Font Software.\n",
			FamilyPermissive, "OFL-1.1",
		},
		{
			// Over-match negative. OFL 1.0's text contains the identical
			// licence name, so a name-only match labelled it OFL-1.1 —
			// a silent wrong answer. FamilyUnknown is a LOUD failure
			// (D-1.3.4 makes it a build failure), which is the correct
			// outcome for a licence this classifier does not know.
			"OFL 1.0 must NOT be labelled OFL-1.1",
			"SIL OPEN FONT LICENSE\nVersion 1.0 - 22 November 2005\n",
			FamilyUnknown, "",
		},
		{
			// ⚠ REVERSED AT STORY 8.4i (2026-09-02, D-8.4i.1), LOUDLY.
			//
			// OLD EXPECTATION, SHIPPED AT STORY 2.2 AND KEPT UNTIL NOW:
			// (FamilyPermissive, "MIT") — "a dependency LICENSE that
			// merely BUNDLES OFL text would otherwise outrank
			// MIT/Apache/BSD purely by ordering."
			//
			// NEW EXPECTATION: FamilyUnknown. This file says two
			// things — it is titled MIT and it names the SIL Open Font
			// License — and D-8.4i.1 rules that a file saying two
			// things is UNCLASSIFIABLE, not arbitrated by branch order.
			// The OFL name here resolves to no known (name, version)
			// pair, so it is an unresolved name signal on its own.
			//
			// THIS IS A GUARD REPLACED BY A STRICTER ONE, NOT WEAKENED:
			// the set of texts that pass SHRINKS, and this input moves
			// from "passes under a possibly-wrong label" to "refused".
			// Recorded here rather than silently edited so the reversal
			// is legible as deliberate rather than as a test edited to
			// make a change go green (Design Note 6).
			"a bundled OFL 1.0 notice makes the file unclassifiable",
			"MIT License\n\nPermission is hereby granted, free of charge\n\n" +
				"Bundled fonts are under the SIL Open Font License.\n",
			FamilyUnknown, "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotFamily, gotID := ClassifyLicenceText(c.text)
			if gotFamily != c.wantFamily {
				t.Errorf("family = %v, want %v", gotFamily, c.wantFamily)
			}
			if gotID != c.wantID {
				t.Errorf("SPDX id = %q, want %q", gotID, c.wantID)
			}
		})
	}
}

// TestCommittedOFLTextClassifiesAsOFL11 reads the ACTUAL committed
// licence file rather than a hand-written approximation of it (D-000.21:
// assert on the artifact). A synthetic fixture that drifts from the real
// text would keep passing while the real classification broke.
func TestCommittedOFLTextClassifiesAsOFL11(t *testing.T) {
	const rel = "../../../folio-go/fonts/notosans/LICENSE-OFL.txt"
	data, err := os.ReadFile(rel)
	if err != nil {
		t.Fatalf("read committed OFL text %s: %v", rel, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s is empty — this assertion would be vacuous", rel)
	}
	family, id := ClassifyLicenceText(string(data))
	if family != FamilyPermissive || id != "OFL-1.1" {
		t.Fatalf(
			"the committed OFL text classifies as (%v, %q), want (FamilyPermissive, %q) — "+
				"this is the text that attributes all three shipped faces in lint/MANIFEST.md",
			family, id, "OFL-1.1",
		)
	}
}

// uflHeader is the SPDX-published Ubuntu Font Licence 1.0 text in
// shape: its own title line, then the grant clause that opens with
// MIT's detection phrase verbatim. The grant clause is not decoration
// here — it is the whole reason the branch exists (Design Note 4,
// Story 8.4h).
const uflHeader = "-------------------------------\n" +
	"UBUNTU FONT LICENCE Version 1.0\n" +
	"-------------------------------\n\n" +
	"PREAMBLE\n" +
	"This licence allows the licensed fonts to be used, studied, modified and\n" +
	"redistributed freely.\n"

const uflGrantClause = "\nPermission is hereby granted, free of charge, to any person obtaining a\n" +
	"copy of the Font Software, to use, study, copy, merge, embed, modify,\n" +
	"redistribute, and sell modified and unmodified copies of the Font\n" +
	"Software, subject to the following conditions:\n"

// TestClassifyUbuntuFontLicence covers the MARKER BRANCH added at Story
// 8.4h (D-8.5.3) for the fourth member of the owner's asset allowlist.
// It is the named test that reds if that branch is removed — the map
// entry alone cannot satisfy any case here, because full licence text
// never reaches classifyBySPDX. TestUbuntuFontLicenceSPDXLineIsPermissive
// below is the named test for the OTHER half: two independent
// mutations, two distinct reds (AC5).
//
// Shape follows TestClassifyOFL: {name, text, wantFamily, wantID}, the
// id asserted explicitly because the id is what lands in
// lint/MANIFEST.md and attributes the asset.
//
// NO CARDINALITY ASSERTION over permissiveSPDX, deliberately (D-8.5.4):
// a count written next to the thing it counts stops being true the
// moment the thing grows. The loudness comes from the set's fail-safe
// miss semantics, not from counting it.
func TestClassifyUbuntuFontLicence(t *testing.T) {
	cases := []struct {
		name       string
		text       string
		wantFamily Family
		wantID     string
	}{
		{
			"UFL 1.0 full text classifies as Ubuntu-font-1.0",
			uflHeader + uflGrantClause,
			FamilyPermissive, "Ubuntu-font-1.0",
		},
		{
			// THE MIT-COLLISION PROOF. The UFL 1.0 grant clause opens
			// with MIT's detection phrase verbatim, so placed below the
			// MIT case every UFL-licensed face classifies as
			// (permissive, "MIT") — right family, wrong label. This
			// case is the whole reason the branch exists and where it
			// is placed.
			"UFL text carrying MIT's trigger phrase must NOT classify as MIT",
			uflHeader + uflGrantClause,
			FamilyPermissive, "Ubuntu-font-1.0",
		},
		{
			// Version-lookalike negative, on the OFL 1.0 precedent: the
			// required "VERSION 1.0" conjunct means a future UFL 1.1
			// classifies as FamilyUnknown — a LOUD build failure at the
			// asset gate — rather than being silently labelled 1.0.
			"UFL 1.1 must NOT be labelled Ubuntu-font-1.0",
			"UBUNTU FONT LICENCE\nVersion 1.1\n",
			FamilyUnknown, "",
		},
		{
			// ⚠ REVERSED AT STORY 8.4i (2026-09-02, D-8.4i.1), LOUDLY.
			//
			// OLD EXPECTATION, SHIPPED AT STORY 8.4h: (FamilyPermissive,
			// "MIT") — "a dependency LICENSE that merely MENTIONS the
			// Ubuntu Font Licence must still classify as its own
			// licence, not outrank it purely by branch ordering."
			//
			// NEW EXPECTATION: FamilyUnknown. The premise was right —
			// branch ordering must not arbitrate — but the conclusion
			// was wrong: the answer is that the classifier does not
			// arbitrate at all. A file naming two licences is
			// unclassifiable, and the UFL name here resolves to no known
			// (name, version) pair besides.
			//
			// A STRICTER GUARD, NOT A WEAKER ONE: this input moves from
			// "passes, labelled MIT" to "refused" (Design Note 6).
			"a bundled UFL notice makes the file unclassifiable",
			"MIT License\n\nPermission is hereby granted, free of charge\n\n" +
				"Bundled fonts are under the Ubuntu Font Licence.\n",
			FamilyUnknown, "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotFamily, gotID := ClassifyLicenceText(c.text)
			if gotFamily != c.wantFamily {
				t.Errorf("family = %v, want %v", gotFamily, c.wantFamily)
			}
			if gotID != c.wantID {
				t.Errorf("SPDX id = %q, want %q", gotID, c.wantID)
			}
		})
	}
}

// TestUbuntuFontLicenceSPDXLineIsPermissive covers the MAP-ENTRY half
// of Story 8.4h's addition: it is the named test that reds if
// permissiveSPDX's "Ubuntu-font-1.0" entry is removed.
//
// ⚠ CORRECTED AT STORY 8.4i (2026-09-02). This comment used to say the
// marker branch could not satisfy it "because an SPDX line
// short-circuits ClassifyLicenceText before the marker switch is
// reached". THERE IS NO SHORT-CIRCUIT AND NO MARKER SWITCH ANY MORE.
// ClassifyLicenceText collects every signal and resolves them as one
// (licencesignals.go); the marker branches are now entries in the
// licenceNames table.
//
// ⚠ AND STORY 8.4h's AC5 PROPERTY — "two independent mutations, two
// DISTINCT reds" — NO LONGER HOLDS IN BOTH DIRECTIONS. Story 8.4i
// removed that bound, and it is recorded here rather than left to be
// discovered, because a bound a change removes must not disappear
// silently.
//
// The cause: addID routes EVERY identifier through classifyBySPDX,
// including the ones the name table produces, so permissiveSPDX is now
// on the name path too. MEASURED at 8.4i's implementation:
//
//   - deleting permissiveSPDX["Ubuntu-font-1.0"] reds THIS test AND
//     TestClassifyUbuntuFontLicence (2 subtests), TestClassifyCollects-
//     EverySignal (1 subtest), TestIsPermissiveSPDXReadsTheSameList and
//     TestLicenceSignalCensus. Five tests, not one.
//   - deleting the licenceNames UFL entry reds TestClassify-
//     UbuntuFontLicence and TestClassifyCollectsEverySignal — and NOT
//     this test.
//
// ⚠ CORRECTED AT STORY 8.4i's CLOSE (2026-09-02), and the correction is
// left in this form rather than by editing the two figures above,
// because the shape of the mistake is this story's own subject. THE TWO
// COUNTS ABOVE ARE TRUE OF THIS PACKAGE ONLY. They were measured with
// `go test ./internal/licence/`, and they are written as though they
// were the suite's whole mutation resolution, which is the property
// they are cited for. RE-MEASURED WITH `go test ./...` OVER THE WHOLE
// lint MODULE:
//
//   - deleting permissiveSPDX["Ubuntu-font-1.0"] reds SEVEN top-level
//     tests, not five: the five above, plus manifest's
//     TestResolveAssetsAcceptsEveryAllowlistedFontLicence
//     (subtest "Ubuntu-font-1.0 by marker") and rules'
//     TestLicenceGraphFixtureScan (subtest "permissive").
//   - deleting the licenceNames UFL entry reds THREE, not two: the two
//     above, plus TestResolveAssetsAcceptsEveryAllowlistedFontLicence.
//
// THE CONCLUSION IS UNCHANGED AND WAS RE-VERIFIED: the second mutation
// still does NOT red this test while the first does, so the
// independence really is one-way. Only the resolution figures were
// understated, and a figure quoted without the scope it was measured at
// is the same defect class as the comments this story corrected — a
// measurement that reads wider than it was taken.
//
// So the independence is now ONE-WAY: the name table can be removed
// without this test noticing, but not the reverse. This test remains the
// only named guard on the map entry; it is no longer the ONLY guard.
//
// The coupling is not accidental — it is what makes every id in the name
// table provably a recognised identifier rather than a free-text label —
// but it is a real reduction in mutation resolution, and D-8.5.4's
// standing preference for guards that red on their own message is why it
// is written down.
//
// The bare community alias "UFL" is asserted to classify as UNKNOWN,
// deliberately: accepting it would be the silent list-widening D-8.5.13
// forbids, and it would admit a string no licence authority issues
// (Design Note 3).
func TestUbuntuFontLicenceSPDXLineIsPermissive(t *testing.T) {
	family, id := ClassifyLicenceText("SPDX-License-Identifier: Ubuntu-font-1.0\n")
	if family != FamilyPermissive || id != "Ubuntu-font-1.0" {
		t.Fatalf(
			"SPDX line classifies as (%v, %q), want (FamilyPermissive, %q) — "+
				"the permissiveSPDX entry is the only thing that can make this pass",
			family, id, "Ubuntu-font-1.0",
		)
	}
	if !IsPermissiveSPDX("Ubuntu-font-1.0") {
		t.Error("IsPermissiveSPDX(\"Ubuntu-font-1.0\") = false, want true")
	}

	for _, alias := range []string{"UFL", "UFL-1.0", "Ufont-1.0"} {
		if IsPermissiveSPDX(alias) {
			t.Errorf("IsPermissiveSPDX(%q) = true; the non-SPDX alias must NOT be a live map key (Design Note 3)", alias)
		}
	}
}

// TestIsPermissiveSPDXReadsTheSameList is task 2's guard: the exported
// predicate must be a view onto permissiveSPDX, not a second copy of
// it. Asserted by sampling both ends of the map plus a known
// non-member, and by the copyleft prefix list staying outside it.
func TestIsPermissiveSPDXReadsTheSameList(t *testing.T) {
	for id, want := range map[string]bool{
		"MIT":             true,
		"CC0-1.0":         true,
		"OFL-1.1":         true,
		"Ubuntu-font-1.0": true,
		"GPL-3.0":         false,
		"":                false,
		"Proprietary":     false,
	} {
		if got := IsPermissiveSPDX(id); got != want {
			t.Errorf("IsPermissiveSPDX(%q) = %v, want %v", id, got, want)
		}
	}
}

// goStyleBSDText is the shape seven of the nine dependency LICENSE files
// in this repository's Go module graphs actually have (measured by
// TestLicenceSignalCensus): NO SPDX line, NO licence name anywhere, and
// the redistribution clause as the only signal. It is reproduced here in
// shape so the constraint is pinned by a unit test as well as by the
// census.
const goStyleBSDText = "Copyright (c) 2009 The Go Authors. All rights reserved.\n\n" +
	"Redistribution and use in source and binary forms, with or without\n" +
	"modification, are permitted provided that the following conditions are\n" +
	"met:\n"

// mitGrantClause is the sentence MIT, OFL 1.0, OFL 1.1 and UFL 1.0 all
// carry VERBATIM. That sharing is the root of DW-124: as the MIT case's
// trigger it made the MIT branch a greedy catch-all that swallowed every
// near-miss on a licence NAME.
const mitGrantClause = "Permission is hereby granted, free of charge, to any person obtaining a\n" +
	"copy of the Font Software, to use, study, copy, merge, embed, modify,\n" +
	"redistribute, and sell modified and unmodified copies of the Font\n" +
	"Software, subject to the following conditions:\n"

// TestClassifyCollectsEverySignal is Story 8.4i's arm-by-arm proof
// (D-8.4i.1, D-8.4i.2). Every case is a row of the story's I/O &
// Edge-Case Matrix, and every case asserts the ID as well as the family
// — the id is what lands in lint/MANIFEST.md, a release artifact under
// AD-26, so a right family under a wrong label is still a defect.
//
// RED-PROVED BY DELETION, NOT ONLY BY SUBSTITUTION (D-8.4h.4's lesson:
// falsifying a condition proves arm ORDER; deleting it proves the arm is
// REACHED). Each mutation was applied ALONE to licencesignals.go and
// `cd lint && go test -count=1 ./...` was run; the red sets below are
// MEASURED, at implementation, and are DISJOINT at subtest level:
//
//	M1 — delete `if len(sig.copyleftIDs) > 0 { return FamilyCopyleft, … }`
//	  reds: TestClassifyLicenceText's 7 copyleft rows; this test's 4
//	  copyleft rows; all 3 of TestCopyleftTieBreakIsDeterministic;
//	  manifest's TestResolveAssetsRefusesACopyleftFontLicence and the
//	  wordlist site's copyleft arm. Nothing else in any package.
//
//	M2 — delete `if sig.unresolved { return FamilyUnknown, … }`
//	  reds: the two REVERSED bundled-notice cases (TestClassifyOFL,
//	  TestClassifyUbuntuFontLicence); this test's unrecognised-SPDX-id
//	  row; the wordlist site's non-permissive arm. Nothing else.
//
//	M3 — weaken `len(sig.permissiveIDs) != 1` to `== 0`, which deletes
//	  the CONFLICT half of the arity arm while leaving the empty half
//	  reds: exactly this test's three two-identifier rows. Nothing else
//	  in any package — which is the proof that conflict detection is a
//	  NEW refusal and not a restatement of an existing one.
//
//	M4 — delete the `if !namedAny` guard, so grant clauses are consulted
//	  even when the text names a licence
//	  reds: TestCommittedOFLTextClassifiesAsOFL11 (the real artifact),
//	  both OFL/UFL grant-clause cases, this test's two still-resolves
//	  rows, and five manifest/rules tests including
//	  TestCommittedAssetPopulationClassifiesCleanly. That blast radius
//	  IS the reason a clause is a weaker signal than a name: made a
//	  peer, the shared grant clause turns all ten committed OFL 1.1
//	  files into two-identifier conflicts.
func TestClassifyCollectsEverySignal(t *testing.T) {
	const gplBody = "GNU GENERAL PUBLIC LICENSE\nVersion 3, 29 June 2007\n\n" +
		"Everyone is permitted to copy and distribute verbatim copies of this\n" +
		"license document, but changing it is not allowed.\n"

	cases := []struct {
		name       string
		text       string
		wantFamily Family
		wantID     string
	}{
		// --- unchanged behaviour: the single-signal cases ---
		{
			"a single SPDX id is unchanged",
			"SPDX-License-Identifier: MIT\n",
			FamilyPermissive, "MIT",
		},
		{
			// AC5. The MIT fallback SURVIVES: a text with the grant
			// clause and no licence name at all is still MIT. The
			// anchor narrows the catch-all; it does not remove it.
			"the bare grant clause with no name signal is still MIT",
			"Copyright (c) 2020 Somebody\n\n" + mitGrantClause,
			FamilyPermissive, "MIT",
		},
		{
			// AC5, and the load-bearing constraint on the name anchor
			// (Design Note 1): "REDISTRIBUTION AND USE IN SOURCE" is a
			// CLAUSE, not a name. If the anchor ever treats it as a
			// name, seven of the nine dependency licences become
			// FamilyUnknown and the dependency scan reds on the Go
			// standard library's own licence.
			"a Go-style BSD text with no name signal is still BSD-3-Clause",
			goStyleBSDText,
			FamilyPermissive, "BSD-3-Clause",
		},

		// --- DW-125: the gate bypass ---
		{
			// AC2. The headline defect. Before this story: (permissive,
			// "MIT") — a full GPL text PASSING the fail-closed asset
			// gate, defeating Story 8.4h's central claim that copyleft
			// is refused by name.
			"a GPL text carrying a stray permissive SPDX line is refused AS COPYLEFT",
			gplBody + "\nSPDX-License-Identifier: MIT\n",
			FamilyCopyleft, "GPL-3.0",
		},
		{
			// AC2, and the finding that was in no register until
			// 8.4i's plan gate: the bypass worked with a copyleft SPDX
			// LINE too, not only with copyleft marker text. The
			// rejected alternative — "a copyleft marker in the body
			// outranks a permissive SPDX line" — would NOT have caught
			// this row.
			"a permissive SPDX line followed by a copyleft one is refused as copyleft",
			"SPDX-License-Identifier: MIT\nSPDX-License-Identifier: GPL-3.0-only\n",
			FamilyCopyleft, "GPL-3.0-only",
		},
		{
			"and in the other order, with the same verdict",
			"SPDX-License-Identifier: GPL-3.0-only\nSPDX-License-Identifier: MIT\n",
			FamilyCopyleft, "GPL-3.0-only",
		},
		{
			// COPYLEFT OUTRANKS CONFLICT, and that order is not
			// stylistic (D-8.4i.1): a maintainer who reads
			// "conflicting identifiers" adds an SPDX line; one who
			// reads "GPL detected" removes the dependency. Hazard
			// indicators fail toward the LOUDEST, never the most
			// precise. Three distinct ids here — two permissive, one
			// copyleft — and the answer is still copyleft, not
			// unknown.
			"copyleft outranks a conflict between permissive ids",
			"SPDX-License-Identifier: MIT\nSPDX-License-Identifier: BSD-3-Clause\n" +
				"SPDX-License-Identifier: AGPL-3.0-only\n",
			FamilyCopyleft, "AGPL-3.0-only",
		},

		// --- D-8.4i.1: two identifiers means we do not know ---
		{
			// AC3.
			"two distinct permissive ids are unclassifiable",
			"SPDX-License-Identifier: MIT\nSPDX-License-Identifier: BSD-3-Clause\n",
			FamilyUnknown, "",
		},
		{
			// AC3's order-independence half, stated as its own case
			// rather than as a loop so a failure names which order
			// broke. Before this story these two rows gave DIFFERENT
			// answers — "MIT" and "BSD-3-Clause" respectively — which
			// is order-dependence measured, not argued.
			"the same two ids in the other order give the identical verdict",
			"SPDX-License-Identifier: BSD-3-Clause\nSPDX-License-Identifier: MIT\n",
			FamilyUnknown, "",
		},
		{
			// The same rule reached through NAMES rather than SPDX
			// lines.
			"a text naming two different licences is unclassifiable",
			"MIT License\n\nApache License\nVersion 2.0, January 2004\n",
			FamilyUnknown, "",
		},
		{
			// An SPDX id on neither list is not nothing: not knowing
			// must not read as fine (D-8.5.2), even when a second,
			// recognised id is present.
			//
			// THE ID IS STILL NAMED, deliberately (AD-14). A declared
			// identifier the classifier cannot place is a located
			// diagnostic; the wordlist gate's three refusal arms are
			// proved disjoint on exactly that distinction
			// (TestWordlistSiteEnforcesThePermissiveSetNotTheFontAllowlist).
			// An unresolved licence NAME has no identifier to name, and
			// the cases below assert it returns "".
			"an unrecognised SPDX id makes the whole text unclassifiable, and is named",
			"MIT License\n\nSPDX-License-Identifier: Proprietary-2.0\n",
			FamilyUnknown, "Proprietary-2.0",
		},

		{
			// AN UNRESOLVED NAME OUTRANKS THE TEXT'S OWN EXPLICIT
			// DECLARATION, and that direction is deliberate. The file
			// declares MIT and also names a licence this classifier
			// cannot place; its governing licence is therefore not
			// established. This is DW-125's shape with the roles
			// swapped — letting the declaration win IS what the bypass
			// did — so the fail-safe answer is to refuse. Refusing
			// costs a maintainer one SPDX line; believing the
			// declaration costs a wrong label on a release artifact.
			"an unresolved name outranks an explicit SPDX declaration",
			"SPDX-License-Identifier: MIT\n\nBundled fonts are under the UBUNTU FONT LICENSE.\n",
			FamilyUnknown, "",
		},

		// --- D-8.4i.2: the name anchor ---
		{
			// AC4, and DW-124 itself. Before this story: (permissive,
			// "MIT") — a UFL face PASSING the gate, published under
			// MIT's label in lint/MANIFEST.md.
			"the American spelling of the Ubuntu Font Licence is unclassifiable, not MIT",
			"-------------------------------\n" +
				"UBUNTU FONT LICENSE Version 1.0\n" +
				"-------------------------------\n" + uflGrantClause,
			FamilyUnknown, "",
		},
		{
			// AC4, and D-8.4i.2's PREDICTED FOURTH INSTANCE, now
			// pinned. The OFL branch's comment claimed a 1.0 file
			// "classifies FamilyUnknown — a LOUD build failure". It
			// did not: OFL 1.0's text carries the same grant clause
			// OFL 1.1's does, so it fell into MIT.
			"OFL 1.0 full text is unclassifiable, not MIT",
			"SIL OPEN FONT LICENSE\nVersion 1.0 - 22 November 2005\n" + mitGrantClause,
			FamilyUnknown, "",
		},
		{
			// The anchor is GENERAL, not a spelling patch: a licence
			// name nobody has thought of yet is refused by the same
			// arm. This case is the difference between D-8.4i.2's
			// ruling and the "accept both spellings" alternative it
			// rejected.
			"a future UFL 1.1 carrying the grant clause is unclassifiable",
			"UBUNTU FONT LICENCE Version 1.1\n" + uflGrantClause,
			FamilyUnknown, "",
		},
		{
			"an unversioned OFL notice carrying the grant clause is unclassifiable",
			"SIL OPEN FONT LICENSE\n" + mitGrantClause,
			FamilyUnknown, "",
		},

		// --- the branches that still resolve ---
		{
			"OFL 1.1 with the shared grant clause still resolves to OFL-1.1",
			"SIL OPEN FONT LICENSE\nVersion 1.1 - 26 February 2007\n" + mitGrantClause,
			FamilyPermissive, "OFL-1.1",
		},
		{
			"UFL 1.0, British spelling, with the shared grant clause still resolves",
			uflHeader + uflGrantClause,
			FamilyPermissive, "Ubuntu-font-1.0",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotFamily, gotID := ClassifyLicenceText(c.text)
			if gotFamily != c.wantFamily || gotID != c.wantID {
				t.Errorf("ClassifyLicenceText() = (%v, %q), want (%v, %q)", gotFamily, gotID, c.wantFamily, c.wantID)
			}
		})
	}
}

// TestCopyleftTieBreakIsDeterministic pins D-8.4i.1's requirement that
// multi-signal resolution be DETERMINISTIC AND DOCUMENTED: a gate whose
// message varies by map iteration order is not a gate.
//
// The rule, stated in resolveLicenceSignals and asserted here: SPDX
// lines in document order first, then licence names in licenceNames'
// most-specific-first table order. The first copyleft candidate wins.
//
// This is not hypothetical. The AGPL and LGPL legal codes both refer to
// the GNU General Public License in their own bodies, so a real AGPL
// LICENSE file carries TWO copyleft name signals. Under the old
// first-match switch the table order decided it; the new resolver keeps
// the identical answer, which is what makes this a repair rather than a
// behaviour change on the copyleft path.
func TestCopyleftTieBreakIsDeterministic(t *testing.T) {
	for _, c := range []struct{ name, text, wantID string }{
		{
			"AGPL text also naming the GNU GPL reports AGPL-3.0",
			"GNU AFFERO GENERAL PUBLIC LICENSE\nVersion 3\n\n" +
				"13. Remote Network Interaction; Use with the GNU GENERAL PUBLIC LICENSE.\n",
			"AGPL-3.0",
		},
		{
			"LGPL text also naming the GNU GPL reports LGPL-3.0",
			"GNU LESSER GENERAL PUBLIC LICENSE\nVersion 3\n\n" +
				"as published under the GNU GENERAL PUBLIC LICENSE.\n",
			"LGPL-3.0",
		},
		{
			"a declared copyleft SPDX id outranks a copyleft NAME in the body",
			"SPDX-License-Identifier: AGPL-3.0-only\n\nGNU GENERAL PUBLIC LICENSE\nVersion 3\n",
			"AGPL-3.0-only",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			// Run repeatedly: an answer that came from map iteration
			// order would not survive this loop.
			for i := 0; i < 32; i++ {
				family, id := ClassifyLicenceText(c.text)
				if family != FamilyCopyleft || id != c.wantID {
					t.Fatalf("iteration %d: got (%v, %q), want (copyleft, %q)", i, family, id, c.wantID)
				}
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────
// STORY 8.4j — A COMPOUND LICENCE LINE IS READ WHOLE (D-8.4j.1).
//
// Story 8.4i taught this classifier to collect every signal ACROSS the
// lines of a licence text. It did not teach it to read a single line
// whole: spdxLineRE captured ONE TOKEN, so
// "SPDX-License-Identifier: OFL-1.1 OR GPL-3.0-only" classified
// (permissive, "OFL-1.1") — an id on the owner's four-id font
// allowlist — and passed the fail-closed asset gate under a permissive
// label in lint/MANIFEST.md, a release artifact.
//
// THE TELL THAT THIS WAS READING AND NOT JUDGING: swapping the two
// identifiers changed the answer. That asymmetry is not a policy; it is
// an artefact of where the reading stopped. TestCompoundSPDXLineIsOrderIndependent
// below is the pin for exactly that.
//
// ClassifySPDXExpression has resolved such expressions correctly since
// Story 1.3 and ClassifyLicenceText simply never asked it. These tests
// pin that it now does.

// TestCompoundSPDXLineWithACopyleftTermResolvesAsCopyleft is RP1: the
// reported defect, at the classifier.
func TestCompoundSPDXLineWithACopyleftTermResolvesAsCopyleft(t *testing.T) {
	const text = "SPDX-License-Identifier: OFL-1.1 OR GPL-3.0-only\n"

	family, id := ClassifyLicenceText(text)
	if family != FamilyCopyleft {
		t.Errorf("RP1: a declaration offering a copyleft term must resolve COPYLEFT — reading only "+
			"its first token published a GPL-offered font under %q on a green build; got family %v", "OFL-1.1", family)
	}
	if id != "OFL-1.1 OR GPL-3.0-only" {
		t.Errorf("RP1: the label must be the WHOLE declaration, not the term the reader stopped at; got %q", id)
	}
}

// TestCompoundSPDXLineIsOrderIndependent is RP2, and it is the test that
// distinguishes reading from judging. The verdict for a disjunction must
// not depend on which of its two identifiers was written first: under
// the single-token capture, "GPL-3.0-only OR OFL-1.1" refused correctly
// while "OFL-1.1 OR GPL-3.0-only" was waved through, and the difference
// between them was nothing but word order.
func TestCompoundSPDXLineIsOrderIndependent(t *testing.T) {
	forward, forwardID := ClassifyLicenceText("SPDX-License-Identifier: OFL-1.1 OR GPL-3.0-only\n")
	reversed, reversedID := ClassifyLicenceText("SPDX-License-Identifier: GPL-3.0-only OR OFL-1.1\n")

	if forward != reversed {
		t.Fatalf("RP2: swapping the two identifiers changed the FAMILY (%v vs %v). Order-dependence is "+
			"the defect itself, not a policy", forward, reversed)
	}
	if forward != FamilyCopyleft {
		t.Errorf("RP2: both orders must resolve copyleft, got %v", forward)
	}
	if forwardID != "OFL-1.1 OR GPL-3.0-only" || reversedID != "GPL-3.0-only OR OFL-1.1" {
		t.Errorf("RP2: each order must be labelled with its OWN whole declaration, got %q and %q", forwardID, reversedID)
	}
}

// TestWhollyPermissiveCompoundSPDXLineResolves is RP3, and it is what
// proves this fix is a CLASSIFIER and not a blanket refusal of anything
// listing more than one name. A file offered under either of two
// acceptable licences says one thing, clearly; refusing it would trade
// the bypass for a regression on a legitimate asset.
//
// Both orders, because RP2's point cuts this way too.
func TestWhollyPermissiveCompoundSPDXLineResolves(t *testing.T) {
	for _, c := range []struct{ text, wantID string }{
		{"SPDX-License-Identifier: MIT OR Apache-2.0\n", "MIT OR Apache-2.0"},
		{"SPDX-License-Identifier: Apache-2.0 OR MIT\n", "Apache-2.0 OR MIT"},
	} {
		family, id := ClassifyLicenceText(c.text)
		if family != FamilyPermissive {
			t.Errorf("RP3: %q must RESOLVE permissive — a fix that refused every compound expression "+
				"would be a ban, not a classifier; got %v", c.wantID, family)
		}
		if id != c.wantID {
			t.Errorf("RP3: label = %q, want the whole expression %q", id, c.wantID)
		}
	}
}

// TestCompoundSPDXLineComposesWithTheOtherSignals pins the composition
// rule's edges — one line is ONE signal whatever its arity; composition
// ACROSS lines stays resolveLicenceSignals' job; and a resolved
// expression's own terms do not count twice against it.
func TestCompoundSPDXLineComposesWithTheOtherSignals(t *testing.T) {
	const oflBody = "SIL OPEN FONT LICENSE\nVersion 1.1 - 26 February 2007\n"
	const gplBody = "GNU GENERAL PUBLIC LICENSE\nVersion 3, 29 June 2007\n"

	for _, c := range []struct {
		name       string
		text       string
		wantFamily Family
		wantID     string
	}{
		{
			// A conjunction is not special-cased: ANY copyleft term
			// makes the whole expression copyleft, which is
			// ClassifySPDXExpression's rule, read not restated.
			"a conjunction with a copyleft term is copyleft",
			"SPDX-License-Identifier: MIT AND GPL-3.0-only\n",
			FamilyCopyleft, "MIT AND GPL-3.0-only",
		},
		{
			// THE MARKING RULE. A real dual-licensed font ships the
			// full text of one of its licences. Without recording the
			// expression's own TERMS as declared, the body's OFL-1.1
			// name signal would be a SECOND permissive identifier and
			// arm 3 would refuse a file that says one thing. The
			// marking is scoped to the NAME/BODY signal space only —
			// TestASecondSPDXLineIsNeverSwallowedByAnEarlierExpressionsTerms
			// is the guard on that scope (D-8.4j.10).
			"a compound line over the body of one of its own terms is not a conflict",
			"SPDX-License-Identifier: OFL-1.1 OR Apache-2.0\n\n" + oflBody,
			FamilyPermissive, "OFL-1.1 OR Apache-2.0",
		},
		{
			// The marking cannot mask a DIFFERENT licence's signal:
			// only the terms the expression itself declared are marked.
			"a compound line over an unrelated permissive body is still a conflict",
			"SPDX-License-Identifier: MIT OR Apache-2.0\n\n" + oflBody,
			FamilyUnknown, "",
		},
		{
			// DW-125 with the roles swapped, still caught.
			"a compound permissive line over a GPL body is still copyleft",
			"SPDX-License-Identifier: MIT OR Apache-2.0\n\n" + gplBody,
			FamilyCopyleft, "GPL-3.0",
		},
		{
			"two DISTINCT compound lines conflict, as two distinct ids do",
			"SPDX-License-Identifier: MIT OR Apache-2.0\nSPDX-License-Identifier: BSD-3-Clause OR ISC\n",
			FamilyUnknown, "",
		},
		{
			// One line is one signal, and the same line twice is the
			// same signal — spacing normalized, so the dedup key is the
			// expression and not its typography.
			"the same compound line twice, differently spaced, is ONE signal",
			"SPDX-License-Identifier: MIT OR Apache-2.0\nSPDX-License-Identifier: MIT  OR   Apache-2.0\n",
			FamilyPermissive, "MIT OR Apache-2.0",
		},
		{
			// Single-term controls: byte-identical to pre-8.4j
			// behaviour, because ClassifySPDXExpression on one term IS
			// classifyBySPDX on that term.
			"a single identifier is unchanged",
			"SPDX-License-Identifier: MIT\n",
			FamilyPermissive, "MIT",
		},
		{
			// A single UNRECOGNISED id still NAMES itself, so the asset
			// gate reaches its off-allowlist arm rather than its
			// "could not be classified" neighbour.
			"a single unrecognised identifier still names itself",
			"SPDX-License-Identifier: CC-BY-SA-4.0\n",
			FamilyUnknown, "CC-BY-SA-4.0",
		},
		{
			// A COMPOUND expression that fails to resolve names
			// NOTHING — load-bearing, see the composition rule in
			// licencesignals.go: naming it would make the gate's
			// off-allowlist message assert that the text classifies as
			// something it demonstrably does not.
			"a compound expression with an unrecognised term names nothing",
			"SPDX-License-Identifier: MIT OR CC-BY-SA-4.0\n",
			FamilyUnknown, "",
		},
		{
			// DW-131's own requested pin. The parenthesised form is
			// byte-identical to pre-8.4j behaviour: fail-closed, and
			// unnamed.
			"the parenthesised form stays unclassifiable and unnamed",
			"SPDX-License-Identifier: (MIT OR Apache-2.0)\n",
			FamilyUnknown, "",
		},
		{
			// A SINGLE-FIELD expression that enumerates to ZERO terms.
			// It is not one term; it is none. Deciding "single term" by
			// counting whitespace FIELDS named it, which made spdx
			// non-empty and pushed both asset gates off their "could not
			// be classified" arm onto an arm asserting the text
			// CLASSIFIES AS "(MIT)" — a statement that is false, and
			// exactly the wrong-ground refusal the composition rule
			// calls load-bearing to avoid.
			"a parenthesised SINGLE term is unnamed too",
			"SPDX-License-Identifier: (MIT)\n",
			FamilyUnknown, "",
		},
		{
			"a single field carrying parentheses is unnamed",
			"SPDX-License-Identifier: MIT()\n",
			FamilyUnknown, "",
		},
		{
			// The other side of the same rule. This expression
			// enumerates exactly ONE term — "MIT", found before the
			// operator failed — while plainly not being a single-term
			// expression, so a bare term COUNT would name it. An
			// expression is a single term iff its one enumerated term IS
			// the whole expression.
			"a partially enumerated expression is unnamed",
			"SPDX-License-Identifier: MIT XOR Apache-2.0\n",
			FamilyUnknown, "",
		},
		{
			// The deferred cost of reading the line whole, PINNED so it
			// is watched rather than rediscovered as a build red.
			"trailing content after the identifier fails closed",
			"SPDX-License-Identifier: MIT */\n",
			FamilyUnknown, "",
		},
		{
			// The separator narrowed from \s* to [ \t]*: a capture that
			// could cross a newline plus a rest-of-line capture would
			// swallow an arbitrary following line as an expression.
			"an identifier on the FOLLOWING line is not a declaration",
			"SPDX-License-Identifier:\nMIT\n",
			FamilyUnknown, "",
		},
		{
			// THE OTHER HALF OF THAT NARROWING. The row above proves
			// [ \t]* EXCLUDES a newline; on its own it leaves the
			// separator proved to exclude and never proved to still
			// INCLUDE. \s* admitted a TAB, so [ \t]* naming \t
			// explicitly is a promise the comment on spdxLineRE makes
			// and nothing checked. A narrowing to `[ ]*` would pass
			// every other row in this table.
			"a TAB separator is still a declaration",
			"SPDX-License-Identifier:\tMIT OR Apache-2.0\n",
			FamilyPermissive, "MIT OR Apache-2.0",
		},
		{
			// The capture is [^\n\r]* rather than .* so that it stops at
			// the line end AND drops the \r of a CRLF ending, which
			// would otherwise ride into the LABEL — "MIT OR
			// Apache-2.0\r" is what lands in lint/MANIFEST.md and what
			// the gate tests term by term.
			//
			// MEASURED, AND HONEST ABOUT WHAT IT PINS: this property is
			// DOUBLY defended, so no single mutation reds this row. Go's
			// `.` already excludes \n, and collectLicenceSignals
			// normalizes the capture through strings.Fields, which drops
			// a stray \r on its own. Removing EITHER defence alone
			// leaves this row green; removing BOTH (capture `.*` plus
			// the raw m[1] as the expression) reds it, on
			// (permissive, "MIT OR Apache-2.0\r"). It is kept as a pin on
			// the OBSERVABLE promise — a CRLF LICENSE classifies and
			// labels exactly as an LF one — rather than on either
			// mechanism, because the promise is what the comment on
			// spdxLineRE makes and what a reader depends on.
			"a CRLF line ending does not poison the expression",
			"SPDX-License-Identifier: MIT OR Apache-2.0\r\n",
			FamilyPermissive, "MIT OR Apache-2.0",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			family, id := ClassifyLicenceText(c.text)
			if family != c.wantFamily || id != c.wantID {
				t.Errorf("got (%v, %q), want (%v, %q)", family, id, c.wantFamily, c.wantID)
			}
		})
	}
}

// TestClassifySPDXExpressionTermsEnumeratesWithoutChangingItsVerdict
// pins the ONE sanctioned modification to ClassifySPDXExpression: it
// gained a sibling that returns the terms it always walked. The family
// and error it reports must be unchanged, and the wrapper must agree
// with the sibling — otherwise the "one parser, one term enumeration,
// THREE consumers" rule has quietly become two parsers. Three, not two:
// collectLicenceSignals, manifest's SITE A (the font gate) and
// manifest's SITE B (the wordlist gate), which D-8.4j.9 pulled inside
// this story. classify.go states the same count on
// ClassifySPDXExpressionTerms itself.
func TestClassifySPDXExpressionTermsEnumeratesWithoutChangingItsVerdict(t *testing.T) {
	for _, c := range []struct {
		expression string
		wantFamily Family
		wantTerms  []string
		wantErr    bool
	}{
		{"MIT", FamilyPermissive, []string{"MIT"}, false},
		{"MIT OR Apache-2.0", FamilyPermissive, []string{"MIT", "Apache-2.0"}, false},
		{"OFL-1.1 OR GPL-3.0-only", FamilyCopyleft, []string{"OFL-1.1", "GPL-3.0-only"}, false},
		{"GPL-3.0-only OR OFL-1.1", FamilyCopyleft, []string{"GPL-3.0-only", "OFL-1.1"}, false},
		{"MIT AND GPL-3.0-only", FamilyCopyleft, []string{"MIT", "GPL-3.0-only"}, false},
		// The unrecognised term is itself named in the slice — that is
		// what lets the gate refuse naming the term that failed.
		{"MIT OR CC-BY-SA-4.0", FamilyUnknown, []string{"MIT", "CC-BY-SA-4.0"}, true},
		// Too malformed to enumerate at all: no terms, and a caller
		// must read "no terms" as "not admissible".
		{"(MIT OR Apache-2.0)", FamilyUnknown, nil, true},
		{"MIT */", FamilyUnknown, nil, true},
		{"", FamilyUnknown, nil, true},
	} {
		t.Run(c.expression, func(t *testing.T) {
			family, terms, err := ClassifySPDXExpressionTerms(c.expression)
			if family != c.wantFamily || (err != nil) != c.wantErr {
				t.Errorf("got (%v, err=%v), want (%v, err=%v)", family, err, c.wantFamily, c.wantErr)
			}
			if len(terms) != len(c.wantTerms) {
				t.Fatalf("terms = %v, want %v", terms, c.wantTerms)
			}
			for i := range terms {
				if terms[i] != c.wantTerms[i] {
					t.Errorf("terms[%d] = %q, want %q", i, terms[i], c.wantTerms[i])
				}
			}
			wrapperFamily, wrapperErr := ClassifySPDXExpression(c.expression)
			if wrapperFamily != family || (wrapperErr != nil) != (err != nil) {
				t.Errorf("the wrapper disagrees with the sibling: (%v, %v) vs (%v, %v) — that would be "+
					"two parsers, not one", wrapperFamily, wrapperErr, family, err)
			}
		})
	}
}

// TestASecondSPDXLineIsNeverSwallowedByAnEarlierExpressionsTerms is
// RP6, and it is the guard on the correction D-8.4j.10 made at the
// build gate.
//
// The marking that stops a dual-licensed font's own BODY from counting
// twice was first written into the SHARED `seen` map, which dedups SPDX
// LINES as well as name signals. That made a resolved expression
// swallow a LATER SPDX LINE: "MIT" then "MIT OR Apache-2.0" resolved
// (unknown, "") while the SAME TWO LINES REVERSED resolved
// (permissive, "MIT OR Apache-2.0"). A NEW cross-line order dependence,
// introduced by the story whose entire subject is order dependence.
//
// The rule is that two SPDX lines are two explicit declarations. A file
// declaring "MIT" on one line and "MIT OR Apache-2.0" on another
// genuinely says two things, so D-8.4i.1's conflict arm fires — and it
// must fire IN BOTH ORDERS.
//
// BOTH ORDERS ARE ASSERTED DELIBERATELY: order-independence is the
// property, and a single-order proof passes by luck.
func TestASecondSPDXLineIsNeverSwallowedByAnEarlierExpressionsTerms(t *testing.T) {
	const single = "SPDX-License-Identifier: MIT\n"
	const compound = "SPDX-License-Identifier: MIT OR Apache-2.0\n"

	forwardFamily, forwardID := ClassifyLicenceText(single + compound)
	reversedFamily, reversedID := ClassifyLicenceText(compound + single)

	if forwardFamily != reversedFamily || forwardID != reversedID {
		t.Fatalf("RP6: the two SPDX lines gave (%v, %q) in one order and (%v, %q) in the other. A verdict "+
			"that depends on which declaration was written first is the defect this story exists to "+
			"remove, not a new place to introduce it",
			forwardFamily, forwardID, reversedFamily, reversedID)
	}
	if forwardFamily != FamilyUnknown || forwardID != "" {
		t.Errorf("RP6: two SPDX lines are two explicit declarations saying different things, so the "+
			"conflict arm must fire and name nothing; got (%v, %q)", forwardFamily, forwardID)
	}

	// The same in the copyleft direction, so the property is not an
	// artefact of both verdicts happening to be unknown.
	const compoundCopyleft = "SPDX-License-Identifier: MIT OR GPL-3.0-only\n"
	copyleftForward, copyleftForwardID := ClassifyLicenceText(single + compoundCopyleft)
	copyleftReversed, copyleftReversedID := ClassifyLicenceText(compoundCopyleft + single)
	if copyleftForward != FamilyCopyleft || copyleftReversed != FamilyCopyleft {
		t.Errorf("RP6: a copyleft term must be refused whichever line declared it; got %v and %v",
			copyleftForward, copyleftReversed)
	}
	if copyleftForwardID != "MIT OR GPL-3.0-only" || copyleftReversedID != "MIT OR GPL-3.0-only" {
		t.Errorf("RP6: both orders must name the whole declaration carrying the copyleft term; got %q and %q",
			copyleftForwardID, copyleftReversedID)
	}
}
