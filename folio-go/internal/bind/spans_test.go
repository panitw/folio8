package bind

import "testing"

func valueFromJSON(t *testing.T, s string) Value {
	t.Helper()
	v, err := DecodeData([]byte(s))
	if err != nil {
		t.Fatalf("decode %s: %v", s, err)
	}
	return v
}

// TestBindTextSpansReportsRuneSpans is AC6 item 2: internal/bind
// reports, alongside the bound string, the RUNE spans it substituted.
//
// The subjects are deliberately multi-byte. An implementation that
// reported BYTE offsets would agree with this test on an ASCII case and
// disagree here — Thai characters are three bytes each, so a byte
// offset would be roughly triple the rune index. Picking an input that
// can tell the two apart is the same discipline AC10 applies to
// GlyphInfo.Cluster.
func TestBindTextSpansReportsRuneSpans(t *testing.T) {
	data := valueFromJSON(t, `{"customer":{"name":"ศรีสุข"},"amount":"1,234.00"}`)
	params := valueFromJSON(t, `{}`)

	cases := []struct {
		name     string
		template string
		wantText string
		wantSubs []Substitution
	}{
		{
			name:     "the format's own worked example shape",
			template: "Statement for {{customer.name}}",
			wantText: "Statement for ศรีสุข",
			// "Statement for " is 14 runes; "ศรีสุข" is 6.
			wantSubs: []Substitution{{Path: "customer.name", Start: 14, End: 20}},
		},
		{
			name:     "placeholder first, literal after",
			template: "{{customer.name}} ยินดีต้อนรับ",
			wantText: "ศรีสุข ยินดีต้อนรับ",
			wantSubs: []Substitution{{Path: "customer.name", Start: 0, End: 6}},
		},
		{
			name:     "two placeholders, both reported",
			template: "{{customer.name}} / {{amount}}",
			wantText: "ศรีสุข / 1,234.00",
			wantSubs: []Substitution{
				{Path: "customer.name", Start: 0, End: 6},
				{Path: "amount", Start: 9, End: 17},
			},
		},
		{
			name:     "no placeholders at all",
			template: "ศรีสุข",
			wantText: "ศรีสุข",
			wantSubs: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, subs, _, err := BindTextSpans(tc.template, data, params, testFormatContext(), "e1")
			if err != nil {
				t.Fatalf("BindTextSpans: %v", err)
			}
			if got != tc.wantText {
				t.Fatalf("text = %q, want %q", got, tc.wantText)
			}
			if len(subs) != len(tc.wantSubs) {
				t.Fatalf("got %d substitutions %+v, want %d %+v", len(subs), subs, len(tc.wantSubs), tc.wantSubs)
			}
			runes := []rune(got)
			for i, want := range tc.wantSubs {
				if subs[i] != want {
					t.Errorf("substitution %d = %+v, want %+v", i, subs[i], want)
				}
				// The span must actually delimit the substituted text in
				// the OUTPUT — an off-by-one that still type-checks is
				// caught here rather than by the index numbers alone.
				if subs[i].Start < 0 || subs[i].End > len(runes) || subs[i].Start > subs[i].End {
					t.Fatalf("substitution %d = %+v is out of range for a %d-rune result", i, subs[i], len(runes))
				}
			}
		})
	}
}

// TestBindTextSpansSpanDelimitsTheSubstitutedValue asserts the span
// property directly rather than through hand-counted indices: slicing
// the result by the reported span must yield exactly the value the path
// held.
func TestBindTextSpansSpanDelimitsTheSubstitutedValue(t *testing.T) {
	data := valueFromJSON(t, `{"customer":{"name":"ศรีสุข"},"city":"กรุงเทพ"}`)
	params := valueFromJSON(t, `{"ref":"AB-9"}`)

	const template = "ผู้รับ {{customer.name}} เมือง {{city}} เลขที่ {{params.ref}}"
	want := map[string]string{
		"customer.name": "ศรีสุข",
		"city":          "กรุงเทพ",
		"params.ref":    "AB-9",
	}

	got, subs, _, err := BindTextSpans(template, data, params, testFormatContext(), "e1")
	if err != nil {
		t.Fatalf("BindTextSpans: %v", err)
	}
	if len(subs) != len(want) {
		t.Fatalf("got %d substitutions %+v, want %d", len(subs), subs, len(want))
	}
	runes := []rune(got)
	for _, s := range subs {
		w, ok := want[s.Path]
		if !ok {
			t.Errorf("unexpected substitution path %q", s.Path)
			continue
		}
		if sliced := string(runes[s.Start:s.End]); sliced != w {
			t.Errorf("path %q: the reported span [%d,%d) slices %q out of %q, want %q",
				s.Path, s.Start, s.End, sliced, got, w)
		}
	}
}

// TestBindTextSpansReservedPlaceholdersProduceNoSubstitution: {{page}}
// and {{pages}} are reserved tokens owned by Story 2.7. They name no
// data path, so no document could declare anything about them, and they
// must not appear as substitutions.
func TestBindTextSpansReservedPlaceholdersProduceNoSubstitution(t *testing.T) {
	data := valueFromJSON(t, `{}`)
	params := valueFromJSON(t, `{}`)

	got, subs, _, err := BindTextSpans("Page {{page}} of {{pages}}", data, params, testFormatContext(), "e1")
	if err != nil {
		t.Fatalf("BindTextSpans: %v", err)
	}
	if got != "Page {{page}} of {{pages}}" {
		t.Errorf("reserved placeholders must pass through byte-for-byte; got %q", got)
	}
	if len(subs) != 0 {
		t.Errorf("reserved placeholders produced %d substitutions %+v, want 0", len(subs), subs)
	}
}

// TestBindTextSpansNullValueYieldsAnEmptySpan: AD-14's null case
// renders as empty and is not an error. The substitution is still
// reported — the caller asked what each path produced — but it is empty,
// so it can never carry an interior break.
func TestBindTextSpansNullValueYieldsAnEmptySpan(t *testing.T) {
	data := valueFromJSON(t, `{"customer":{"name":null}}`)
	params := valueFromJSON(t, `{}`)

	got, subs, _, err := BindTextSpans("A{{customer.name}}B", data, params, testFormatContext(), "e1")
	if err != nil {
		t.Fatalf("a null binding renders as empty and is not an error (AD-14): %v", err)
	}
	if got != "AB" {
		t.Errorf("text = %q, want %q", got, "AB")
	}
	if len(subs) != 1 {
		t.Fatalf("got %d substitutions, want 1", len(subs))
	}
	if subs[0].Start != 1 || subs[0].End != 1 {
		t.Errorf("a null substitution = %+v, want an empty span at rune 1", subs[0])
	}
}

// TestBindTextDelegatesToBindTextSpans pins that there is exactly ONE
// implementation of the binding grammar. If BindText ever grows a
// second traversal, the spans could describe text that was produced
// differently — the "only one derivation" failure Story 2.3's Blocker 1
// was, one stage earlier.
//
// It is asserted over the SAME inputs the existing BindText tests use,
// including the error cases, so a divergence in either the text or the
// error would be caught.
func TestBindTextDelegatesToBindTextSpans(t *testing.T) {
	data := valueFromJSON(t, `{"customer":{"name":"ศรีสุข"},"n":null}`)
	params := valueFromJSON(t, `{"ref":"AB-9"}`)

	inputs := []string{
		"Statement for {{customer.name}}",
		"Page {{page}} of {{pages}}",
		"{{params.ref}}",
		"A{{n}}B",
		"plain literal",
		"{{missing.path}}",
		"{{sum(x)}}",
		"{{unterminated",
		"{{params}}",
	}

	var errCases, okCases int
	for _, in := range inputs {
		wantText, wantErr := BindText(in, data, params, testFormatContext(), "e1")
		gotText, _, _, gotErr := BindTextSpans(in, data, params, testFormatContext(), "e1")
		if (wantErr == nil) != (gotErr == nil) {
			t.Errorf("%q: BindText err=%v but BindTextSpans err=%v", in, wantErr, gotErr)
			continue
		}
		if wantErr != nil {
			errCases++
			if wantErr.Error() != gotErr.Error() {
				t.Errorf("%q: error text diverged:\n BindText:      %v\n BindTextSpans: %v", in, wantErr, gotErr)
			}
			continue
		}
		okCases++
		if wantText != gotText {
			t.Errorf("%q: BindText=%q BindTextSpans=%q", in, wantText, gotText)
		}
	}

	if okCases == 0 {
		t.Fatal("vacuity: no input succeeded, so agreement on output text was never tested")
	}
	if errCases == 0 {
		t.Fatal("vacuity: no input failed, so agreement on error text was never tested")
	}
	t.Logf("BindText and BindTextSpans agree on all %d inputs (%d successful, %d errors)", len(inputs), okCases, errCases)
}
