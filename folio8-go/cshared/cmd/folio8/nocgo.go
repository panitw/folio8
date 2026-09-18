//go:build !cgo

// This stub exists so `go build ./...` and `go vet ./...` stay green with
// CGO_ENABLED=0 — which matrix.yml sets on every leg. Without it the cgo file
// is excluded by the build constraint, the directory holds no Go files, and
// the toolchain reports "build constraints exclude all Go files" instead of
// quietly skipping a package that genuinely cannot be built without cgo.
//
// It is not a degraded engine: there is no ABI here at all. Building the
// native library requires cgo, and folio-dotnet/build/build-native.sh and
// .ps1 set CGO_ENABLED=1 explicitly.
package main

func main() {}
