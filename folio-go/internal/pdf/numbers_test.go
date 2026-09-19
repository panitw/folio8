package pdf

import (
	"math"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

func TestAppendLength(t *testing.T) {
	cases := []struct {
		v    geom.Length
		want string
	}{
		{0, "0"},
		{50000, "50"},
		{72500, "72.5"},
		{100250, "100.25"},
		{200125, "200.125"},
		{595276, "595.276"},
		{841890, "841.89"},
		{-500, "-0.5"},
		{-1, "-0.001"},
		{1, "0.001"},
		{-1000, "-1"},
		{1000, "1"},
		{-999, "-0.999"},
		// math.MinInt64-adjacent bounds: negating MinInt64 overflows, so
		// the emitter must not negate the raw value directly.
		{geom.Length(math.MinInt64), "-9223372036854775.808"},
		{geom.Length(math.MinInt64 + 1), "-9223372036854775.807"},
		{geom.Length(math.MaxInt64), "9223372036854775.807"},
	}

	for _, c := range cases {
		got := string(appendLength(nil, c.v))
		if got != c.want {
			t.Errorf("appendLength(%d) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestAppendLengthNeverEmitsNegativeZero(t *testing.T) {
	got := string(appendLength(nil, 0))
	if got != "0" {
		t.Errorf("appendLength(0) = %q, want %q (never -0)", got, "0")
	}
}

func TestAppendLengthAppendsToExistingSlice(t *testing.T) {
	dst := []byte("re ")
	got := string(appendLength(dst, 72500))
	if got != "re 72.5" {
		t.Errorf("appendLength did not append correctly: got %q", got)
	}
}

func TestAppendInt(t *testing.T) {
	cases := []struct {
		v    int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{123, "123"},
		{-123, "-123"},
		{math.MaxInt64, "9223372036854775807"},
		{math.MinInt64, "-9223372036854775808"},
	}
	for _, c := range cases {
		got := string(appendInt(nil, c.v))
		if got != c.want {
			t.Errorf("appendInt(%d) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestAppendIntPadded(t *testing.T) {
	cases := []struct {
		v     int64
		width int
		want  string
	}{
		{0, 5, "00000"},
		{1, 5, "00001"},
		{12345, 5, "12345"},
		{123456, 5, "123456"}, // wider than width: not truncated
		{0, 10, "0000000000"},
		{1, 10, "0000000001"},
		{9876543210, 10, "9876543210"},
		{-1, 5, "-00001"}, // width pads the digits, not the sign+digits

		{5, 0, "5"},
	}
	for _, c := range cases {
		got := string(appendIntPadded(nil, c.v, c.width))
		if got != c.want {
			t.Errorf("appendIntPadded(%d, %d) = %q, want %q", c.v, c.width, got, c.want)
		}
	}
}

func TestAppendIntPaddedAppendsToExistingSlice(t *testing.T) {
	dst := []byte("prefix-")
	got := string(appendIntPadded(dst, 7, 5))
	if got != "prefix-00007" {
		t.Errorf("appendIntPadded did not append correctly: got %q", got)
	}
}
