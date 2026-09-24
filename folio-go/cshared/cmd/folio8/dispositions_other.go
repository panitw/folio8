//go:build cgo && !linux

package main

// Off Linux the engine does nothing to signal dispositions and says so. The
// defect this answers (folio-dotnet's DW-398) is measured on Linux amd64;
// Darwin's Go runtime makes the same edit to a differently laid-out struct
// on a platform where it has not been measured, and is deliberately left
// alone. Windows has no signals.
func signalDispositionsReport() string {
	return "not-linux: the engine leaves signal dispositions alone on this platform"
}
