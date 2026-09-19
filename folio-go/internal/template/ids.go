package template

import "fmt"

// This file is the id/nextId asymmetry's single home (AC31–AC38,
// D-1.4.4 reversed in part): ids render as "e" + lowercase base 36;
// nextId is a plain decimal integer. Base-36 DECODING exists only to
// validate nextId against the highest id present (AC33) — nowhere else
// in this package calls it.

const base36Digits = "0123456789abcdefghijklmnopqrstuvwxyz"

// idCounterToBase36 renders a decimal counter (>= 1) as the numeral
// half of an element id: "e" + lowercase base 36 (AC31). Counter must
// already be validated positive by the caller.
func idCounterToBase36(counter int64) string {
	if counter == 0 {
		return "0"
	}
	var buf [16]byte
	pos := len(buf)
	n := counter
	for n > 0 {
		pos--
		buf[pos] = base36Digits[n%36]
		n /= 36
	}
	return string(buf[pos:])
}

// renderElementID renders the full canonical id string "e"+base36(counter).
func renderElementID(counter int64) ElementID {
	return ElementID("e" + idCounterToBase36(counter))
}

// AllocateElementID is the sole mutation-time bridge for the authoritative
// nextId counter. The caller increments NextID only after its transaction has
// passed all validation, so failed commands never consume an id.
func AllocateElementID(d *Document) ElementID { return renderElementID(d.NextID) }

// validateElementID checks a decoded id string against AC34's rule:
// must match ^e[0-9a-z]+$, decode to >= 1, no leading zeros, no
// uppercase. It never normalises — an invalid spelling is a load error,
// not a value to repair (AD-10: ids are never renumbered on save).
// Returns the decoded decimal counter value on success.
func validateElementID(id string) (int64, error) {
	if len(id) < 2 || id[0] != 'e' {
		return 0, fmt.Errorf("template: id %q does not match ^e[0-9a-z]+$", id)
	}
	numeral := id[1:]
	for i := 0; i < len(numeral); i++ {
		c := numeral[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z')) {
			return 0, fmt.Errorf("template: id %q does not match ^e[0-9a-z]+$ (invalid character %q)", id, c)
		}
	}
	if len(numeral) > 1 && numeral[0] == '0' {
		return 0, fmt.Errorf("template: id %q has a leading zero — not a value to normalise (AD-10, AC34)", id)
	}

	counter, err := base36ToDecimal(numeral)
	if err != nil {
		return 0, fmt.Errorf("template: id %q: %w", id, err)
	}
	if counter < 1 {
		return 0, fmt.Errorf("template: id %q decodes to %d, which is < 1", id, counter)
	}
	return counter, nil
}

// base36ToDecimal decodes a lowercase base-36 numeral (already known to
// contain only [0-9a-z]) to its decimal value. Used only by
// validateElementID and validateNextID (AC33: "needed only to validate
// nextId against the highest id present").
func base36ToDecimal(numeral string) (int64, error) {
	var n int64
	for i := 0; i < len(numeral); i++ {
		c := numeral[i]
		var d int64
		switch {
		case c >= '0' && c <= '9':
			d = int64(c - '0')
		case c >= 'a' && c <= 'z':
			d = int64(c-'a') + 10
		default:
			return 0, fmt.Errorf("invalid base-36 digit %q", c)
		}
		next := n*36 + d
		if next < n {
			return 0, fmt.Errorf("base-36 value overflows int64")
		}
		n = next
	}
	return n, nil
}
