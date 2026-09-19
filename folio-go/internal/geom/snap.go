package geom

// snapNearest rounds v to the nearest multiple of increment. Exact halfway
// values round away from zero. The editor uses this only for a proposed
// future placement; callers must not persist its UI preference.
func (v Length) SnapNearest(increment Length) (Length, bool) {
	if increment <= 0 || increment == minInt64 {
		return 0, false
	}
	q, r := int64(v)/int64(increment), int64(v)%int64(increment)
	if r == 0 {
		return Length(q * int64(increment)), true
	}
	absR := r
	if absR < 0 {
		absR = -absR
	}
	half := int64(increment) - absR
	if absR >= half {
		if v >= 0 {
			if q == (1<<63-1)/int64(increment) {
				return 0, false
			}
			q++
		} else {
			if q == (-1<<63)/int64(increment) {
				return 0, false
			}
			q--
		}
	}
	return Length(q * int64(increment)), true
}
