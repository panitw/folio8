package template

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// This file is the base64 canonicalisation D-1.8.2 requires (AC1–AC4):
// RFC 4648 §4 standard alphabet, WITH padding, split at exactly 76
// columns on write; ANY wrapping accepted on read.

// decodeBase64Asset concatenates every element of wrapped (whatever the
// wrapping happens to be — AC2: "accept ANY wrapping") with nothing
// inserted, then strictly decodes it as standard base64 WITH padding.
// Bad alphabet, bad padding and embedded whitespace are all rejected —
// the standard-encoding STRICT decoder, not a tolerant one (AC4). An
// empty decoded result is also an error here (AC4: "it cannot render,
// and its key would be the SHA-256 of nothing") — checked by the caller,
// which is why this function itself does not special-case it.
//
// THE JOIN IS BUILT, NOT ACCUMULATED (Story 16.0), and the reason is a
// measured fatal fault rather than a tidiness preference. This loop used to
// be `joined += part`, which allocates a fresh string per element and copies
// everything accumulated so far into it: for a 598,060-byte face the canonical
// 76-column split is 10,493 elements, and the total allocated is
// sum(76, 152, …, 797,416) ≈ 4.18 GB. On a 64-bit host that is merely slow and
// the GC reclaims it, which is why `go test ./wasm` accepted all 21 catalogue
// faces natively. Under js/wasm the heap is a 32-bit linear memory with a
// 4 GiB ceiling, so the same call reached
// `runtime: out of memory: cannot allocate 524288-byte block (4234510336 in
// use)` and the Go program EXITED — taking `Folio8WasmHost.handle` with it, so
// the designer's worker received `undefined` where a JSON response belongs.
// strings.Builder with one Grow makes it a single allocation and one pass.
func decodeBase64Asset(wrapped []string) ([]byte, error) {
	size := 0
	for _, part := range wrapped {
		size += len(part)
	}
	var builder strings.Builder
	builder.Grow(size)
	for _, part := range wrapped {
		builder.WriteString(part)
	}
	joined := builder.String()
	decoded, err := base64.StdEncoding.Strict().DecodeString(joined)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	return decoded, nil
}

// DecodeAssetBytes decodes an already-loaded Document's Asset.Data to
// its raw bytes. Safe to call unconditionally on any Asset obtained
// from a *Document that ParseDocument produced: decodeAssets already
// validated every asset's base64 at load (D-1.8.2/AC4), so this cannot
// fail for a document that loaded successfully — the error return
// exists only for a hand-built *Document that skipped that validation.
// This is package folio8's (module root) route to the decoded bytes
// DecodeImageForRender needs; render.go is its only caller outside this
// package.
func DecodeAssetBytes(a Asset) ([]byte, error) {
	return decodeBase64Asset(a.Data)
}

// splitBase64Canonical re-encodes decoded as standard base64 WITH
// padding and splits it into elements of EXACTLY 76 characters, the
// final element carrying the remainder (1–76) — AC1, verbatim: "Padding
// falls where it falls — inside the split, at the end of the last
// element. Never stripped, never moved, never given its own element."
// This is D-1.8.9's mechanism: it is called on DECODED bytes, never on
// the wrapping an asset happened to arrive with, so it cannot echo an
// input array by construction — writeAssets (serialize.go) is the only
// caller, and it always calls this with freshly re-derived decoded
// bytes.
func splitBase64Canonical(decoded []byte) []string {
	joined := base64.StdEncoding.EncodeToString(decoded)
	if joined == "" {
		return nil
	}
	const width = 76
	n := (len(joined) + width - 1) / width
	out := make([]string, 0, n)
	for i := 0; i < len(joined); i += width {
		end := i + width
		if end > len(joined) {
			end = len(joined)
		}
		out = append(out, joined[i:end])
	}
	return out
}
