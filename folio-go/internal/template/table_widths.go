package template

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"sort"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

const ProportionUnit int64 = 1000

var proportionLiteral = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]{1,3})?$`)

// DecodeProportion accepts the same exact positive decimal at file and command
// boundaries. Weights have no line-spacing bounds, and never pass through floats.
func DecodeProportion(literal string) (int64, error) {
	if !proportionLiteral.MatchString(literal) {
		return 0, fmt.Errorf("proportion must be a positive decimal with at most three decimal places")
	}
	v, err := decodePoints(literal)
	if err != nil {
		return 0, fmt.Errorf("proportion must not exceed %s", FormatProportion(math.MaxInt64))
	}
	if v <= 0 {
		return 0, fmt.Errorf("proportion must be positive")
	}
	return int64(v), nil
}

func DecodeProportionRaw(raw json.RawMessage) (int64, error) {
	return DecodeProportion(string(raw))
}

func FormatProportion(v int64) string { return string(appendPoints(nil, geom.Length(v))) }

// TableColumnWidths is the single allocation used by parsing, commands, canvas,
// and PDF. Layout receives only its resolved millipoints. Legacy point widths
// retain their existing validation and rendering behavior.
func TableColumnWidths(el Element) ([]geom.Length, error) {
	cols := el.Table.Value.Columns
	widths := make([]geom.Length, len(cols))
	if !el.Width.Set {
		for i, col := range cols {
			if col.Proportion.Set {
				return nil, newLoadError("proportion", string(col.ID), "", "proportions require a table width; mixed sizing is not allowed")
			}
			widths[i] = col.Width
		}
		return widths, nil
	}
	if el.Width.Null || el.Width.Value <= 0 {
		return nil, newLoadError("width", string(el.ID), FormatPoints(el.Width.Value), "table total width must be positive")
	}
	sum := new(big.Int)
	for _, col := range cols {
		if !col.Proportion.Set || col.Proportion.Null || col.Proportion.Value <= 0 || col.Width != 0 {
			return nil, newLoadError("proportion", string(col.ID), "", "a proportional table requires a positive proportion instead of width on every column")
		}
		sum.Add(sum, big.NewInt(col.Proportion.Value))
	}
	if len(cols) == 0 {
		return widths, nil
	} // preserve the authored total
	type remainder struct {
		index int
		value *big.Int
	}
	remainders := make([]remainder, len(cols))
	remaining := int64(el.Width.Value)
	for i, col := range cols {
		product := new(big.Int).Mul(big.NewInt(int64(el.Width.Value)), big.NewInt(col.Proportion.Value))
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(product, sum, r)
		widths[i] = geom.Length(q.Int64())
		remaining -= q.Int64()
		remainders[i] = remainder{i, r}
	}
	// Stable order gives an equal remainder to the earlier authored column.
	sort.SliceStable(remainders, func(i, j int) bool { return remainders[i].value.Cmp(remainders[j].value) > 0 })
	for i := int64(0); i < remaining; i++ {
		widths[remainders[i].index]++
	}
	for i, width := range widths {
		if width <= 0 {
			return nil, newLoadError("proportion", string(cols[i].ID), FormatProportion(cols[i].Proportion.Value), "allocation gives this column zero width; increase its proportion or the table total")
		}
	}
	return widths, nil
}
