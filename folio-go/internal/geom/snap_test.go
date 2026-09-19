package geom

import (
	"math"
	"testing"
)

func TestSnapNearestUsesFixedPointAndHalfAwayFromZero(t *testing.T) {
	for _, tc := range []struct{ in, want Length }{{0, 0}, {2999, 0}, {3000, 6000}, {-3000, -6000}, {6000, 6000}, {-9000, -12000}} {
		got, ok := tc.in.SnapNearest(6000)
		if !ok || got != tc.want {
			t.Fatalf("snapNearest(%d) = %d, %v; want %d, true", tc.in, got, ok, tc.want)
		}
	}
	if _, ok := Length(1).SnapNearest(0); ok {
		t.Fatal("zero increment must be rejected")
	}
	for _, increment := range []Length{-1, minInt64} {
		if _, ok := Length(1).SnapNearest(increment); ok {
			t.Fatalf("invalid increment %d was accepted", increment)
		}
	}
	if got, ok := Length(12000).SnapNearest(6000); !ok || got != 12000 {
		t.Fatalf("exact multiple = %d, %v", got, ok)
	}
	if _, ok := Length(math.MaxInt64).SnapNearest(10); ok {
		t.Fatal("positive overflow must be rejected")
	}
	if _, ok := Length(math.MinInt64).SnapNearest(10); ok {
		t.Fatal("negative overflow must be rejected")
	}
}
