package fontset

import (
	"encoding/binary"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// hheaTableBytes returns a COPY of the `hhea` table's bytes from a
// well-formed sfnt, located through the table directory (12-byte header,
// then 16-byte records) rather than at a fixed offset — the same walk
// patchUnitsPerEm uses, so it survives a different test face later.
//
// This exists so the assertions below can be anchored to THE FONT FILE
// rather than to a literal written next to them. A test whose "expected"
// value is a constant in its own source can only tell you the constant
// has not changed.
func hheaTableBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	for i := 0; i < numTables; i++ {
		rec := data[12+i*16 : 12+i*16+16]
		if string(rec[0:4]) != "hhea" {
			continue
		}
		off := binary.BigEndian.Uint32(rec[8:12])
		length := binary.BigEndian.Uint32(rec[12:16])
		out := make([]byte, length)
		copy(out, data[off:off+length])
		return out
	}
	t.Fatal("precondition: the test face carries no hhea table, so nothing below can be anchored to it")
	return nil
}

// TestLineMetricsReadsTheHheaTableNotTheVendorAccessors is D-2.4.2
// constraint 2's STANDING guard: leading must be read from the `hhea`
// TABLE and must never inherit a vendor-substituted default.
//
// # Why a guard at this layer, and why the obvious one does not work
//
// (*ot.Face).Ascender / Descender / LineGap SUBSTITUTE 800 / -200 / 0
// when hhea is absent (textshape v0.0.15, ot/metrics.go:434, :442, :500)
// — the same substitution class Story 2.3a audited, and the same class
// as the /CapHeight 928 a face whose true value is 711 would have
// emitted.
//
// The obvious guard — assert each shipped face's leading is not 1000
// units — CANNOT FAIL. requireReadableTables makes an absent hhea a load
// error, so no *Font whose accessors substitute can ever exist; and for
// a face whose hhea IS present the accessors return the table's own
// numbers. That is measured below as leg 1: for a real face the two
// readings AGREE, which is precisely why rerouting LineMetrics through
// the accessors left the whole suite green when it was tried.
//
// So the property is asserted where the two readings can be told apart:
// LineMetrics must return the fields PARSED FROM THE TABLE at
// construction, whatever the face's accessors would say. Overwriting
// those fields with sentinels and requiring LineMetrics to follow them
// is a behavioural assertion that no source-text or unreachable-branch
// check can give.
//
// # Red-proof (leg 2)
//
// Route LineMetrics through scale(f.face.Ascender()) /
// scale(f.face.Descender()) / scale(f.face.LineGap()) — the exact
// mutation the review demonstrated leaves the rest of the suite green —
// and this test reddens: LineMetrics returns the face's real hhea
// numbers instead of the sentinels it was handed.
func TestLineMetricsReadsTheHheaTableNotTheVendorAccessors(t *testing.T) {
	data := testFontBytes(t)
	f, err := New("linemetrics-subject", data)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	upem := int64(f.UnitsPerEm())
	if upem <= 0 {
		t.Fatalf("precondition: unitsPerEm is %d", upem)
	}
	scale := func(v int16) int64 {
		return int64(geom.ScaleRound(geom.Length(int64(v)), 1000, upem))
	}

	// Leg 1 — ANCHORED TO THE ARTIFACT. The three numbers are read out
	// of the font file's own hhea table (ascender at offset 4, descender
	// at 6, lineGap at 8, per the sfnt spec), not from a literal in this
	// file, and LineMetrics must reproduce them scaled.
	hhea := hheaTableBytes(t, data)
	if len(hhea) < 10 {
		t.Fatalf("precondition: the hhea table is %d bytes, too short to carry ascender/descender/lineGap", len(hhea))
	}
	fileAscender := int16(binary.BigEndian.Uint16(hhea[4:6]))
	fileDescender := int16(binary.BigEndian.Uint16(hhea[6:8]))
	fileLineGap := int16(binary.BigEndian.Uint16(hhea[8:10]))

	wantFromFile := LineMetrics{
		Ascent:  scale(fileAscender),
		Descent: scale(fileDescender),
		LineGap: scale(fileLineGap),
	}
	if got := f.LineMetrics(); got != wantFromFile {
		t.Errorf("LineMetrics() = %+v, want %+v — the values read straight out of the face's own hhea table (ascender %d, descender %d, lineGap %d font units at %d/em)",
			got, wantFromFile, fileAscender, fileDescender, fileLineGap, upem)
	}

	// Leg 1b, STATED AS A MEASUREMENT AND NOT AN ASSUMPTION: for a face
	// whose hhea is present the vendor accessors return the same
	// numbers. This is the reason leg 2 is needed at all, so it is
	// recorded rather than left for a reader to rediscover.
	viaAccessors := LineMetrics{
		Ascent:  scale(f.face.Ascender()),
		Descent: scale(f.face.Descender()),
		LineGap: scale(f.face.LineGap()),
	}
	if viaAccessors != wantFromFile {
		t.Fatalf("precondition changed: this face's accessors report %+v against the table's %+v. The premise of leg 2 — that the two readings are INDISTINGUISHABLE for a face with hhea, which is why a sentinel is required — no longer holds, and this test's argument must be rewritten rather than its numbers updated",
			viaAccessors, wantFromFile)
	}
	t.Logf("leg 1: table reading and accessor reading AGREE at %+v for a face whose hhea is present — the reroute is invisible here, which is why leg 2 exists", wantFromFile)

	// Leg 2 — THE DISCRIMINATING ASSERTION. Sentinels that no real face
	// and no vendor substitution can produce.
	const (
		sentinelAscender  = int16(1234)
		sentinelDescender = int16(-321)
		sentinelLineGap   = int16(77)
	)

	// Vacuity precondition, against BOTH readings the sentinels have to
	// out-argue: the vendor's substituted 800 / -200 / 0, and this
	// face's own real numbers. If the sentinels collided with either,
	// leg 2 could pass under the very mutation it exists to catch.
	const (
		substitutedAscender  = int16(800)
		substitutedDescender = int16(-200)
		substitutedLineGap   = int16(0)
	)
	if sentinelAscender == substitutedAscender || sentinelDescender == substitutedDescender || sentinelLineGap == substitutedLineGap {
		t.Fatal("vacuity: a sentinel equals the vendor's substituted default, so leg 2 could not tell a substituted answer from a table one")
	}
	if sentinelAscender == fileAscender || sentinelDescender == fileDescender || sentinelLineGap == fileLineGap {
		t.Fatalf("vacuity: a sentinel equals this face's own hhea value (%d/%d/%d), so leg 2 could not tell a table read from an accessor read",
			fileAscender, fileDescender, fileLineGap)
	}

	f.hheaAscent = sentinelAscender
	f.hheaDescent = sentinelDescender
	f.hheaLineGap = sentinelLineGap

	wantSentinel := LineMetrics{
		Ascent:  scale(sentinelAscender),
		Descent: scale(sentinelDescender),
		LineGap: scale(sentinelLineGap),
	}
	got := f.LineMetrics()
	switch {
	case got != wantSentinel:
		t.Errorf("LineMetrics() = %+v after the hhea-derived fields were set to %d/%d/%d, want %+v.\n"+
			"LineMetrics is reading something OTHER than the fields parsed from the hhea table. If it now routes through "+
			"(*ot.Face).Ascender/Descender/LineGap, it has inherited the vendor's substituting accessors and D-2.4.2 "+
			"constraint 2 is broken: those return 800/-200/0 for a face with no hhea, three plausible numbers "+
			"indistinguishable from real ones. Accessor reading for this face is %+v.",
			got, sentinelAscender, sentinelDescender, sentinelLineGap, wantSentinel, viaAccessors)
	default:
		t.Logf("leg 2: LineMetrics followed the table-derived fields to %+v, declining the accessors' %+v", got, viaAccessors)
	}
}
