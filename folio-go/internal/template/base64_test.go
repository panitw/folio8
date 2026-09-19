package template

import (
	"encoding/base64"
	"runtime"
	"strings"
	"testing"
)

// TestSplitBase64CanonicalWidthAndPadding is AC1's red-proof target
// (RP-1): every element except the last must be EXACTLY 76 characters,
// the last in [1,76], concatenation reproduces the input exactly, and
// '=' padding appears ONLY at the end of the last element.
//
// Finding 6 (Story 1.8 review): the original version of this test ran
// only against png3x2RGB (81 bytes — a multiple of 3), whose base64
// encoding has ZERO '=' padding. Every padding assertion below therefore
// ranged over strings containing no "=" at all and could not fail for
// any implementation; RP-1's named mutation ("serializer strips padding
// into its own element") left this test green. It is now table-driven
// over 0, 1 and 2 padding characters, and each case asserts its own
// fixture assumptions (padding count, and that the base64 length is not
// itself a multiple of 76) before trusting the rest of the checks —
// AC1's exact discriminating shape: "an asset whose base64 length is not
// a multiple of 76 and whose padding therefore lands mid-final-element."
func TestSplitBase64CanonicalWidthAndPadding(t *testing.T) {
	cases := []struct {
		name        string
		payload     []byte
		wantPadding int // trailing '=' characters, by construction of std base64
	}{
		{"zero padding characters (payload length is a multiple of 3)", png3x2RGB, 0},
		{"one padding character", paddingProbePayload(80), 1},
		{"two padding characters", paddingProbePayload(79), 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			joined := base64.StdEncoding.EncodeToString(c.payload)
			if gotPadding := strings.Count(joined, "="); gotPadding != c.wantPadding {
				t.Fatalf("fixture assumption broken: payload of length %d base64-encodes with %d padding character(s), want %d",
					len(c.payload), gotPadding, c.wantPadding)
			}
			if len(joined)%76 == 0 {
				t.Fatalf("fixture assumption broken: base64 length %d IS a multiple of 76 — "+
					"this case would not discriminate AC1's padding clause", len(joined))
			}

			parts := splitBase64Canonical(c.payload)
			if len(parts) == 0 {
				t.Fatal("expected at least one element")
			}
			for i, p := range parts[:len(parts)-1] {
				if len(p) != 76 {
					t.Fatalf("element %d has length %d, want exactly 76", i, len(p))
				}
			}
			last := parts[len(parts)-1]
			if len(last) < 1 || len(last) > 76 {
				t.Fatalf("last element has length %d, want in [1,76]", len(last))
			}

			reassembled := strings.Join(parts, "")
			if reassembled != joined {
				t.Fatalf("concatenation does not reproduce the base64 exactly:\ngot:  %s\nwant: %s", reassembled, joined)
			}

			// Padding falls where it falls — only at the end of the last
			// element, never stripped, moved, or given its own element.
			for i, p := range parts[:len(parts)-1] {
				if strings.Contains(p, "=") {
					t.Fatalf("element %d contains padding %q — padding must appear only at the end of the last element", i, p)
				}
			}
			if c.wantPadding > 0 && !strings.HasSuffix(last, strings.Repeat("=", c.wantPadding)) {
				t.Fatalf("last element %q does not end with the expected %d padding character(s)", last, c.wantPadding)
			}
		})
	}
}

// paddingProbePayload returns a deterministic byte slice of length n,
// used only to control base64 padding shape (AC1, Finding 6) — its
// content is arbitrary since splitBase64Canonical operates on raw
// bytes, never on image structure.
func paddingProbePayload(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(i % 251)
	}
	return out
}

// TestSplitBase64Canonical64To76 is D-1.8.9's discriminating fixture
// (RP-2): a payload wrapped at 64 columns on read must produce 76-column
// output on write. png1x1Gray's base64 is short enough that at 64-width
// wrapping it differs in shape from 76-width wrapping (2 elements either
// way for this size, but the WIDTHS differ — 64+32 vs 76+20), which is
// what proves this is a re-derivation and not an echo.
func TestSplitBase64Canonical64To76(t *testing.T) {
	joined := base64.StdEncoding.EncodeToString(png3x2RGB) // 108 chars
	// Hand-wrap the SAME payload at 64 columns (simulating a hand-edited file).
	var wrapped64 []string
	for i := 0; i < len(joined); i += 64 {
		end := i + 64
		if end > len(joined) {
			end = len(joined)
		}
		wrapped64 = append(wrapped64, joined[i:end])
	}
	if len(wrapped64) != 2 || len(wrapped64[0]) != 64 {
		t.Fatalf("test fixture assumption violated: wrapped64 = %v", wrapped64)
	}

	decoded, err := decodeBase64Asset(wrapped64)
	if err != nil {
		t.Fatalf("decodeBase64Asset(64-wrapped): %v", err)
	}
	canonical := splitBase64Canonical(decoded)
	if len(canonical) != 2 || len(canonical[0]) != 76 {
		t.Fatalf("expected canonical re-wrap at 76 columns, got %v", canonical)
	}
	if canonical[0] == wrapped64[0] {
		t.Fatal("canonical output must differ in SHAPE from the 64-wrapped input — an echoing serializer would fail this")
	}
}

// TestDecodeBase64AssetAcceptsAnyWrapping is AC2: the parser accepts ANY
// wrapping — concatenates with nothing inserted, decodes strictly.
func TestDecodeBase64AssetAcceptsAnyWrapping(t *testing.T) {
	joined := base64.StdEncoding.EncodeToString(png3x2RGB)
	cases := map[string][]string{
		"single element":    {joined},
		"64-wrapped":        {joined[:64], joined[64:]},
		"one char per elem": splitEachChar(joined),
	}
	for name, wrapped := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := decodeBase64Asset(wrapped)
			if err != nil {
				t.Fatalf("decodeBase64Asset(%s): %v", name, err)
			}
			if string(got) != string(png3x2RGB) {
				t.Fatalf("decoded bytes do not match for wrapping %q", name)
			}
		})
	}
}

func splitEachChar(s string) []string {
	out := make([]string, 0, len(s))
	for _, r := range s {
		out = append(out, string(r))
	}
	return out
}

// TestDecodeBase64AssetRejectsInvalidInput is AC4: bad alphabet, bad
// padding and embedded whitespace are all load errors (strict decode).
func TestDecodeBase64AssetRejectsInvalidInput(t *testing.T) {
	cases := map[string][]string{
		"bad alphabet":        {"not-valid-base64!!!"},
		"bad padding":         {"AAAA="},
		"embedded whitespace": {"AA AA"},
	}
	for name, wrapped := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeBase64Asset(wrapped); err == nil {
				t.Fatalf("expected an error for %s, got nil", name)
			}
		})
	}
}

// TestIsSHA256HexKeyShapeCheck is AC6a's shape check, red-proofed
// against RP-4a: a 62-character key must fail with a SHAPE diagnosis,
// distinguishable from a well-formed-but-wrong VALUE (RP-4, tested via
// decodeAssets in parse_test.go / assets_test.go).
func TestIsSHA256HexKeyShapeCheck(t *testing.T) {
	cases := map[string]bool{
		strings.Repeat("a", 64): true,
		strings.Repeat("a", 62): false, // RP-4a's exact shape
		strings.Repeat("A", 64): false, // uppercase is not lowercase hex
		strings.Repeat("g", 64): false, // 'g' is not hex
		"5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078": true,
	}
	for key, want := range cases {
		if got := isSHA256HexKey(key); got != want {
			t.Errorf("isSHA256HexKey(%q) = %v, want %v", key, got, want)
		}
	}
}

// TestDecodeBase64AssetJoinsInLinearSpace is Story 16.0's regression guard,
// and the defect it guards is not a slow test — it is a CRASH the designer's
// author saw as "The engine returned an invalid response".
//
// decodeBase64Asset used to join the canonical 76-column split with
// `joined += part`, which allocates a fresh string per element and copies
// everything accumulated so far. For a 598,060-byte face that is 10,493
// elements and sum(76, 152, …, 797,416) ≈ 4.18 GB of allocation. Natively
// that is merely slow and the GC absorbs it, which is exactly why
// `go test ./wasm` accepted all 21 catalogue faces at the plan gate and the
// fault stayed invisible to Go. Under js/wasm the heap is a 32-bit linear
// memory capped at 4 GiB, so the same call reached `fatal error: out of
// memory` inside SerializeTemplate — reached from applyFontChainCommand on
// every pick — and the Go program EXITED, taking Folio8WasmHost.handle with
// it. `handle` then returned undefined and the worker's JSON.parse threw.
//
// MEASURING ALLOCATION IS THE ONLY HONEST WITNESS HERE. Asserting on wall
// time would be flaky and would not name the fault; asserting the OUTPUT
// would pass on both implementations, because the quadratic one produced the
// right string. The gap being guarded is three orders of magnitude wide, so
// the generous multiple below is not a fudge — it is far below any plausible
// linear implementation's cost and far above nothing.
func TestDecodeBase64AssetJoinsInLinearSpace(t *testing.T) {
	// A face-sized payload at the canonical 76-column split.
	const width = 76
	const elements = 10493
	wrapped := make([]string, elements)
	for i := range wrapped {
		wrapped[i] = strings.Repeat("A", width)
	}
	joined := width * elements

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	decoded, err := decodeBase64Asset(wrapped)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatalf("decodeBase64Asset: %v", err)
	}
	if len(decoded) != joined/4*3 {
		t.Fatalf("decoded %d bytes, want %d", len(decoded), joined/4*3)
	}

	allocated := after.TotalAlloc - before.TotalAlloc
	// The quadratic join allocated ≈ joined*elements/2 — about 4.18 GB here.
	// A linear one allocates the joined string plus the decoded bytes plus
	// builder slack.
	if limit := uint64(joined) * 8; allocated > limit {
		t.Errorf("joining %d elements allocated %d bytes, over the %d-byte linear budget — the accumulate-into-a-string shape is back, and under js/wasm's 4 GiB linear memory it is a fatal out-of-memory, not a slow path", elements, allocated, limit)
	}
}
