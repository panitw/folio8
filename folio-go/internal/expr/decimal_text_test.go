package expr

import (
	"math"
	"testing"
)

// TestDecimalTextIsExact pins the one printer a number bound into text goes
// through: coefficient digits, '.' placed by the exponent, scale kept, no
// exponent notation, grouping or rounding.
func TestDecimalTextIsExact(t *testing.T) {
	for _, tc := range []struct {
		literal string
		want    string
	}{
		{"1234.50", "1234.50"},
		{"2067071865", "2067071865"},
		{"-3.5", "-3.5"},
		{"1e3", "1000"},
		{"-0", "0"},
		{"0.000", "0.000"},
		{"-0.00", "0.00"},
		{"0", "0"},
		{"99.9", "99.9"},
		{"0.05", "0.05"},
		{"-0.005", "-0.005"},
		{"1.5e1", "15"},
		{"12.5e-2", "0.125"},
		{"0e3", "0"},
		{"9223372036854775807", "9223372036854775807"},
	} {
		d, err := NewDecimal(tc.literal)
		if err != nil {
			t.Fatalf("NewDecimal(%q): %v", tc.literal, err)
		}
		if got := d.Text(); got != tc.want {
			t.Errorf("Decimal(%q).Text() = %q, want %q", tc.literal, got, tc.want)
		}
	}
	for _, tc := range []struct {
		d    Decimal
		want string
	}{
		{Decimal{123450, -2}, "1234.50"},
		{Decimal{1, 3}, "1000"},
		{Decimal{-35, -1}, "-3.5"},
		{Decimal{0, -3}, "0.000"},
		{Decimal{math.MinInt64, 0}, "-9223372036854775808"},
		{Decimal{math.MinInt64, -19}, "-0.9223372036854775808"},
		{Decimal{math.MinInt64, -20}, "-0.09223372036854775808"},
	} {
		if got := tc.d.Text(); got != tc.want {
			t.Errorf("%+v.Text() = %q, want %q", tc.d, got, tc.want)
		}
	}
}
