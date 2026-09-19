package layout

import (
	"reflect"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// wantTableGeometryFields, wantColumnGeometryFields (AC1, instrument 1c)
// are TEST-OWNED literals containing NO width field on TableGeometry
// itself — R4's compiler-anchored property: there is nowhere for a
// stale total to live.
var wantTableGeometryFields = map[string]bool{"Columns": true}
var wantColumnGeometryFields = map[string]bool{"X": true, "Width": true}

func fieldNames(t *testing.T, typ reflect.Type) map[string]bool {
	t.Helper()
	got := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		got[typ.Field(i).Name] = true
	}
	if len(got) == 0 {
		t.Fatal("vacuity guard (D-000.9): reflected zero fields")
	}
	return got
}

func assertFieldSetsEqualExactly(t *testing.T, name string, want, got map[string]bool) {
	t.Helper()
	compared := 0
	for k := range want {
		compared++
		if !got[k] {
			t.Errorf("%s: missing expected field %q, got %v", name, k, got)
		}
	}
	for k := range got {
		compared++
		if !want[k] {
			t.Errorf("%s: unexpected field %q (R4: width is a computed value, never a field), got %v", name, k, got)
		}
	}
	if compared == 0 {
		t.Fatal("vacuity guard (D-000.9): zero comparisons made")
	}
}

// TestTableGeometryHasNoWidthField is AC1 instrument 1c.
//
// Discriminating mutation (run and recorded): add a `Width geom.Length`
// field to TableGeometry and populate it once at construction — reds on
// the set equality, before any staleness could even be observed.
func TestTableGeometryHasNoWidthField(t *testing.T) {
	assertFieldSetsEqualExactly(t, "TableGeometry", wantTableGeometryFields, fieldNames(t, reflect.TypeOf(TableGeometry{})))
	assertFieldSetsEqualExactly(t, "ColumnGeometry", wantColumnGeometryFields, fieldNames(t, reflect.TypeOf(ColumnGeometry{})))
}

// TestTableGeometryWidthIsComputed confirms TableGeometry.Width() is
// reachable ONLY as a method over the column slice (the compiler is the
// anchor: there is no field to read instead).
func TestTableGeometryWidthIsComputed(t *testing.T) {
	g := ColumnWidths(0, []geom.Length{70000, 80000})
	if got, want := g.Width(), geom.Length(150000); got != want {
		t.Errorf("Width() = %d, want %d", got, want)
	}
}

// TestColumnWidthsVectors is AC1, instrument 1d: a TEST-OWNED table of
// column-width vectors, with sums written out — never computed by
// calling the function under test.
//
// Discriminating mutation (run and recorded): return `sum + headerHeight`,
// clamp to a band width, or use `max()` instead of `+` in Width() or in
// ColumnWidths' running-sum loop — reds on at least three vectors below.
func TestColumnWidthsVectors(t *testing.T) {
	cases := []struct {
		name    string
		widths  []geom.Length
		wantSum geom.Length
	}{
		{"empty", nil, 0},
		{"single", []geom.Length{70000}, 70000},
		{"five", []geom.Length{70000, 80000, 60000, 40000, 110000}, 360000},
		{"tiny", []geom.Length{1}, 1},
		// Finisher fix (Story 4.1 review Finding 10 / Minor, and its
		// Nit-16 duplicate "these three" describing a TWO-element
		// vector): the ORIGINAL comment here claimed this vector
		// "exercises geom.ScaleRound's sibling overflow-aware path" —
		// FALSE. Width() is a plain `total += c.Width` loop with no
		// overflow detection of any kind, and calls into geom.ScaleRound
		// nowhere; a sum that truly exceeded int64 would wrap silently.
		// This vector is honestly a LARGE-VALUE SMOKE CASE (two large
		// values whose sum still fits int64), not a proof that overflow
		// is handled — no such handling exists in this function, and
		// none is added here (adding one is a design question of its
		// own, raised as a follow-up rather than solved inside this
		// vector's doc comment).
		{"near-limit", []geom.Length{4611686018427387903, 4611686018427387903}, 9223372036854775806},
		// Third MULTI-COLUMN vector (Finding 10's other half): AC1 1d
		// requires the sum-vs-max mutation to redden "at least three"
		// vectors, and only a vector with 2+ DISTINCT-valued columns can
		// ever distinguish sum from max — "single" and "tiny" (one
		// column each) cannot, by construction. With only "five" and
		// "near-limit" as multi-column vectors, the mutation reddened
		// exactly 2. This third one brings it to 3.
		{"three-columns", []geom.Length{10000, 20000, 30000}, 60000},
	}
	exercised := 0
	for _, c := range cases {
		exercised++
		g := ColumnWidths(0, c.widths)
		if got := g.Width(); got != c.wantSum {
			t.Errorf("%s: Width() = %d, want %d", c.name, got, c.wantSum)
		}
	}
	if exercised == 0 {
		t.Fatal("vacuity guard (D-000.9): zero vectors exercised")
	}
}

// TestColumnWidthsEmptyVectorGivesZeroNotError is 1d's explicit
// requirement: the EMPTY vector must give zero, not an error (there is
// no error return to omit — this pins the zero-value behaviour).
func TestColumnWidthsEmptyVectorGivesZeroNotError(t *testing.T) {
	g := ColumnWidths(0, nil)
	if len(g.Columns) != 0 {
		t.Errorf("empty widths must produce zero columns, got %d", len(g.Columns))
	}
	if got := g.Width(); got != 0 {
		t.Errorf("empty vector Width() = %d, want 0", got)
	}
}

// TestColumnWidthsXOrigins is AC2's positive half at the layout-geometry
// level: x-origins are a running sum of declared widths, from startX,
// and never anything else.
func TestColumnWidthsXOrigins(t *testing.T) {
	g := ColumnWidths(10000, []geom.Length{70000, 80000, 60000})
	want := []geom.Length{10000, 80000, 160000}
	if len(g.Columns) != len(want) {
		t.Fatalf("got %d columns, want %d", len(g.Columns), len(want))
	}
	for i, c := range g.Columns {
		if c.X != want[i] {
			t.Errorf("column %d: X = %d, want %d", i, c.X, want[i])
		}
	}
}
