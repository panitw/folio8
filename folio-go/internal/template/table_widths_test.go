package template

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

func proportionalElement(total geom.Length, weights ...int64) Element {
	columns := make([]Column, len(weights))
	for i, weight := range weights {
		columns[i] = Column{ID: ElementID(fmt.Sprintf("e%d", i+2)), Proportion: present(weight)}
	}
	return Element{ID: "e1", Type: ElementTable, Width: present(total), Table: present(TableExt{Columns: columns})}
}

func proportionalFixture() []byte {
	return []byte(strings.Replace(tableWidthDoc, `"width": 40`, `"proportion": 1`, 1))
}

func TestTableProportionAllocation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		total   geom.Length
		weights []int64
		want    []geom.Length
	}{
		{"ten equal", 500000, []int64{1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000}, []geom.Length{50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000}},
		{"five equal", 500000, []int64{1000, 1000, 1000, 1000, 1000}, []geom.Length{100000, 100000, 100000, 100000, 100000}},
		{"unequal", 500000, []int64{1000, 2000, 1000}, []geom.Length{125000, 250000, 125000}},
		{"unequal remainders", 500000, []int64{1000, 2000}, []geom.Length{166667, 333333}},
		{"largest remainder in last column", 10, []int64{2000, 3000, 4000}, []geom.Length{2, 3, 5}},
		{"changed total", 400000, []int64{1000, 2000, 1000}, []geom.Length{100000, 200000, 100000}},
		{"round ties in column order", 500000, []int64{1000, 1000, 1000}, []geom.Length{166667, 166667, 166666}},
		{"fractional weights", 500000, []int64{1, 2, 1}, []geom.Length{125000, 250000, 125000}},
		{"sum and product overflow int64", 500000, []int64{math.MaxInt64, math.MaxInt64}, []geom.Length{250000, 250000}},
		{"int64 total", math.MaxInt64, []int64{math.MaxInt64, math.MaxInt64}, []geom.Length{4611686018427387904, 4611686018427387903}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			widths, err := TableColumnWidths(proportionalElement(tc.total, tc.weights...))
			if err != nil || !reflect.DeepEqual(widths, tc.want) {
				t.Fatalf("widths=%v err=%v, want %v", widths, err, tc.want)
			}
			var total geom.Length
			for _, w := range widths {
				total += w
			}
			if total != tc.total {
				t.Fatalf("sum=%d want %d", total, tc.total)
			}
		})
	}
}

func TestTableProportionFileValidationAndPersistence(t *testing.T) {
	if _, err := DecodeProportion("9223372036854775.808"); err == nil || err.Error() != "proportion must not exceed 9223372036854775.807" {
		t.Fatalf("overflow must explain the proportion limit: %v", err)
	}
	for _, literal := range []string{"", "0", "-1", "null", `"1"`, "1.0000", "0.0001", "1e2", "oops", "9223372036854775.808"} {
		t.Run(literal, func(t *testing.T) {
			if _, err := DecodeProportion(literal); err == nil {
				t.Fatalf("accepted %q", literal)
			}
		})
	}
	for _, literal := range []string{"0.001", "1", "1.234", "1001", "9223372036854775.807"} {
		if _, err := DecodeProportion(literal); err != nil {
			t.Fatalf("%q: %v", literal, err)
		}
	}
	for _, tc := range []struct{ name, before, after, located string }{
		{"nonpositive total", `"width": 500`, `"width": 0`, "e1"},
		{"null total", `"width": 500`, `"width": null`, "e1"},
		{"absolute in proportional table", `"proportion": 1`, `"width": 40`, "e2"},
		{"both column units", `"proportion": 1`, `"proportion": 1, "width": 0`, "e2"},
		{"missing total", `"width": 500,`, ``, "e2"},
		{"invalid proportion", `"proportion": 1`, `"proportion": 1.0001`, "e2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseDocument(bytes.Replace(proportionalFixture(), []byte(tc.before), []byte(tc.after), 1))
			if err == nil || !strings.Contains(err.Error(), tc.located) {
				t.Fatalf("located refusal=%v", err)
			}
		})
	}
	d, err := ParseDocument(proportionalFixture())
	if err != nil {
		t.Fatal(err)
	}
	b, err := SerializeDocument(d)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"version": "3.0"`)) || !bytes.Contains(b, []byte(`"proportion": 1`)) {
		t.Fatalf("proportional persistence: %s", b)
	}
	d, err = ParseDocument(b)
	if err != nil {
		t.Fatal(err)
	}
	again, err := SerializeDocument(d)
	if err != nil || !bytes.Equal(b, again) {
		t.Fatalf("round-trip: %v", err)
	}
	d.Version = "3.7"
	b, _ = SerializeDocument(d)
	if !bytes.Contains(b, []byte(`"version": "3.7"`)) {
		t.Fatal("downgraded declared version")
	}
}

func TestTableProportionEmptyAndZeroAllocation(t *testing.T) {
	el := proportionalElement(500000)
	widths, err := TableColumnWidths(el)
	if err != nil || len(widths) != 0 || el.Width.Value != 500000 {
		t.Fatalf("empty total lost: %v %v", widths, err)
	}
	el.Table.Value.Columns = proportionalElement(500000, 1000).Table.Value.Columns
	widths, err = TableColumnWidths(el)
	if err != nil || len(widths) != 1 || widths[0] != 500000 {
		t.Fatalf("replacement column: %v %v", widths, err)
	}
	for _, el := range []Element{proportionalElement(1, 1000, 1000), proportionalElement(500000, 1, math.MaxInt64)} {
		if _, err := TableColumnWidths(el); err == nil || !strings.Contains(err.Error(), "zero width") {
			t.Fatalf("zero allocation accepted: %v", err)
		}
	}
}
