//go:build cgo && linux

package main

// The Go side of dispositions_linux.c: the fallback restore, and the report.
//
// init runs on the runtime's own thread, after initsig and before any export
// can return — cgocallbackg1 waits on main_init_done before running any
// callback — so if the C constructor found itself ahead of Go's entry
// (verify-linux-natives.sh should have refused the file, but a host could be
// linking this package some other way), this call does the same work a few
// milliseconds later. When the constructor has already restored, this finds
// nothing to do and the report still says "constructor".

/*
#include <stdlib.h>
int folio8_dispositions_restore(const char *who);
int folio8_dispositions_report(char *buf, int cap);
*/
import "C"

import "unsafe"

func init() {
	who := C.CString("init")
	defer C.free(unsafe.Pointer(who))
	C.folio8_dispositions_restore(who)
}

// signalDispositionsReport is the payload of folio8_signal_dispositions.
func signalDispositionsReport() string {
	buf := make([]byte, 512)
	n := C.folio8_dispositions_report((*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf)))
	if n < 0 {
		return "linux report-too-long"
	}
	return string(buf[:int(n)])
}
