//go:build !nocjkface

package fonts

import (
	"strings"
	"testing"
)

// unshippedFaceDirs is empty in the default build: every face directory
// under folio-go/fonts/ is embedded and shipped, so the accounting is
// total over all eleven — unchanged, and deliberately so
// (spec-deferred-offline-cache: "the eleven-face shipped contract is
// untouched").
var unshippedFaceDirs = []string{}

// unshippedFaceKeys is the fonts.Shipped() side of the same statement,
// and it is empty for the same reason.
var unshippedFaceKeys = []string{}

// TestShippedCarriesTheCJKFace is the default build's half of the
// measurement: eleven faces, the CJK one among them, with a byte floor
// so a truncated or empty embed cannot pass as present.
func TestShippedCarriesTheCJKFace(t *testing.T) {
	shipped := Shipped()
	if len(shipped) != 11 {
		t.Fatalf("fonts.Shipped() returned %d faces, and the untouched shipped contract is eleven: %s", len(shipped), strings.Join(sortedKeys(shipped), ", "))
	}
	face, ok := shipped["Noto Sans SC"]
	if !ok {
		t.Fatal(`fonts.Shipped() no longer carries "Noto Sans SC" in the DEFAULT build — spec-deferred-offline-cache removes it from the designer's wasm only, never from the shipped set`)
	}
	if len(face) < 10_000_000 {
		t.Fatalf(`fonts.Shipped()["Noto Sans SC"] is %d bytes, far under the face's ~10.6 MB — the embed is reading something else`, len(face))
	}
}
