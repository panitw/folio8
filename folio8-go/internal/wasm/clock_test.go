package wasm

// testClockStepMs is how many milliseconds testClock advances on every
// reading. Render reads the clock once on each side of folio8.Render, so every
// render under test reports exactly this many elapsed milliseconds.
const testClockStepMs = 7

// testClock is the fixed fake for NewEngine's nanosecond clock: deterministic, and never
// the wall clock, which `time`'s ban under internal/ keeps out of this package.
func testClock() func() int64 {
	var now int64
	return func() int64 {
		now += testClockStepMs * 1_000_000
		return now
	}
}
