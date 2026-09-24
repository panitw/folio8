//go:build cgo

package main

import (
	"strings"
	"testing"
)

// TestSignalDispositionsAreReported pins the export's shape: an ok frame
// whose payload is the platform's one-line report. What the report SAYS is
// asserted where it can be true — folio-dotnet's SignalDispositionTests, on
// a Linux host that loaded the shared library. This binary is not that
// library: its constructors run before Go's own runtime is up, so the
// values here describe a test executable, not a dlopen.
func TestSignalDispositionsAreReported(t *testing.T) {
	var out outParams
	if status := folio8_signal_dispositions(&out.token, &out.result, &out.length); status != statusOK {
		t.Fatalf("status %d, want %d", status, statusOK)
	}
	buf := frameBytes(t, &out)
	if len(buf) == 0 || buf[0] != kindOK {
		t.Fatalf("frame kind %v, want %d", buf[:1], kindOK)
	}
	report := signalDispositionsReport()
	if !strings.HasPrefix(report, "linux ") && !strings.HasPrefix(report, "not-linux") {
		t.Fatalf("report %q names no platform", report)
	}
	if !strings.Contains(string(buf), report) {
		t.Fatalf("payload does not carry the report %q:\n% x", report, buf)
	}
	if strings.HasPrefix(report, "linux ") {
		for _, key := range []string{"snapshot=", "constructor-ran-before-go=", "restored-by=", "restored=", "restore-calls="} {
			if !strings.Contains(report, " "+key) {
				t.Errorf("report lacks %s: %q", key, report)
			}
		}
	}
}
