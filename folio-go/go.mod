module github.com/panitw/folio8/folio-go

// The go directive below is deliberately LOWER than the toolchain directive.
// Go treats a `toolchain` line that is not strictly greater than the `go`
// line as redundant and strips it on `go mod tidy` — measured on this
// machine (Go 1.26.0 darwin/arm64, GOTOOLCHAIN=auto): with `go 1.26.0` alone,
// `go mod tidy` deletes `toolchain go1.26.0`, and a developer with a newer
// local toolchain silently builds with it instead. That is exactly the
// AD-22 hazard (undocumented toolchain drift changing the byte-identity
// fingerprint). `go 1.25.0` is a genuine language/minimum floor, not a
// mistake — do not "fix" the version gap by raising it to 1.26.0.
go 1.25.0

// toolchain is the actual pin, honoured by GOTOOLCHAIN=auto. Go 1.27 is
// deliberately not adopted: compress/flate byte-stability is verified only
// through 1.26, and adopting 1.27 is a re-measurement exercise under AD-22,
// not a routine upgrade. See D-1.1.a in folio-mvp-decision-log.md.
toolchain go1.26.0

require github.com/boxesandglue/textshape v0.0.15
