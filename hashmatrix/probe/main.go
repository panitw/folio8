// Command probe is the retained FMA-contraction demonstration for
// Story 1.2 (AC8). It is deliberately outside folio-go: the module it
// lives in, hashmatrix/, has no dependency on folio-go (no require, no
// replace, no go.work) and is never imported by it, so its float64 is
// outside AD-2/AD-23's scope by construction — that module's guards
// (folio-go/internal/arch_test.go's TestNoFloat64UnderInternal and Story
// 1.3's AD-1 import lint) bind folio-go/internal/ positively and never
// mention hashmatrix/, so there is nothing for either of them to exempt.
//
// Two design constraints are load-bearing, not stylistic:
//
//  1. The three operands are read from os.Args via strconv.ParseFloat,
//     never as Go literals. With literal constants the compiler folds
//     x*scale+origin at build time using exact arithmetic, and every
//     target then agrees on the unfused value — the probe would be
//     silently vacuous. Measured in this story's Dev Notes (F-8): a
//     literal-operand variant emits the identical 8 bytes on every
//     target. Reading from argv defeats constant folding by
//     construction.
//  2. The output is math.Float64bits(pos) written as 8 raw big-endian
//     bytes, never a formatted decimal. Two different float64 values can
//     round to the same decimal text, which would mask exactly the
//     low-bit difference this probe exists to expose.
package main

import (
	"encoding/binary"
	"math"
	"os"
	"strconv"
)

// layoutPos is the contraction case: a float64 multiply-add of exactly
// the shape a compiler is permitted to fuse into a single fused
// multiply-add instruction (x*scale + origin), the same shape as layout
// arithmetic such as "position on an axis = coordinate*scale + origin".
// Some architectures (measured: darwin/arm64, linux/arm64) fuse this into
// one rounding step; others (measured: linux/amd64, js/wasm) perform two
// roundings. The two results differ in their low bits, which this probe
// exposes as raw bytes.
func layoutPos(x, scale, origin float64) float64 {
	return x*scale + origin
}

func main() {
	if len(os.Args) != 4 {
		os.Stderr.WriteString("usage: probe <x> <scale> <origin>\n")
		os.Exit(2)
	}
	var v [3]float64
	for i := 0; i < 3; i++ {
		f, err := strconv.ParseFloat(os.Args[i+1], 64)
		if err != nil {
			os.Stderr.WriteString("bad operand " + os.Args[i+1] + ": " + err.Error() + "\n")
			os.Exit(2)
		}
		v[i] = f
	}
	pos := layoutPos(v[0], v[1], v[2])
	var out [8]byte
	binary.BigEndian.PutUint64(out[:], math.Float64bits(pos))
	os.Stdout.Write(out[:])
}
