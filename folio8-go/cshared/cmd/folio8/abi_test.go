//go:build cgo

package main

import (
	"encoding/binary"
	"testing"
	"unsafe"
)

// The ABI's two defensive statuses — FOLIO8_ERROR_ARGUMENT and
// FOLIO8_ERROR_PANIC — are published contract, so they are EXECUTED here
// rather than described. Without this file the nil/length guards and the
// recover could both be deleted and every other test would stay green.

// outParams is the three-out-parameter tail every export shares.
type outParams struct {
	token  cU64
	result unsafe.Pointer
	length cI32
}

// frameBytes copies the result buffer back into Go and frees its token,
// asserting the free is accepted exactly once.
func frameBytes(t *testing.T, out *outParams) []byte {
	t.Helper()
	if out.result == nil {
		t.Fatal("no result buffer was written")
	}
	buf := append([]byte(nil), unsafe.Slice((*byte)(out.result), int(out.length))...)
	if status := folio8_free(out.token); status != statusOK {
		t.Fatalf("folio8_free returned %d, want %d", status, statusOK)
	}
	return buf
}

// readStr reads one length-prefixed blob from the front of buf.
func readStr(t *testing.T, buf []byte) (string, []byte) {
	t.Helper()
	if len(buf) < 4 {
		t.Fatalf("frame is truncated before a length prefix: % x", buf)
	}
	n := int(binary.LittleEndian.Uint32(buf[:4]))
	if len(buf)-4 < n {
		t.Fatalf("frame declares a %d-byte blob but holds %d", n, len(buf)-4)
	}
	return string(buf[4 : 4+n]), buf[4+n:]
}

// TestMalformedCallsAreRefusedWithoutAllocating drives every shape of
// FOLIO8_ERROR_ARGUMENT. The status allocates nothing and issues no token, so
// the allocation table must be untouched afterwards — which is the half a
// reader cannot check by eye.
func TestMalformedCallsAreRefusedWithoutAllocating(t *testing.T) {
	before := int(folio8_allocation_count())

	// A font buffer whose first name length runs past the end of the buffer.
	truncatedFonts := make([]byte, 4)
	binary.LittleEndian.PutUint32(truncatedFonts, 64)

	// A well-formed name/face pair, repeated: a duplicate face name is a
	// malformed buffer, not a last-wins resolution.
	duplicateFonts := func() []byte {
		one := func(name string) []byte {
			b := make([]byte, 0, 8+len(name)+1)
			b = binary.LittleEndian.AppendUint32(b, uint32(len(name)))
			b = append(b, name...)
			b = binary.LittleEndian.AppendUint32(b, 1)
			return append(b, 0x00)
		}
		return append(one("Roboto"), one("Roboto")...)
	}()

	template := []byte("{}")

	cases := []struct {
		name string
		run  func(out *outParams) cI32
	}{
		{"negative template length", func(out *outParams) cI32 {
			return folio8_parse(unsafe.Pointer(&template[0]), -1, &out.token, &out.result, &out.length)
		}},
		{"nil pointer with a positive length", func(out *outParams) cI32 {
			return folio8_parse(nil, 5, &out.token, &out.result, &out.length)
		}},
		{"negative font length", func(out *outParams) cI32 {
			return folio8_render(unsafe.Pointer(&template[0]), cI32(len(template)),
				unsafe.Pointer(&template[0]), cI32(len(template)),
				nil, 0,
				unsafe.Pointer(&truncatedFonts[0]), -4,
				&out.token, &out.result, &out.length)
		}},
		{"truncated font buffer", func(out *outParams) cI32 {
			return folio8_render(unsafe.Pointer(&template[0]), cI32(len(template)),
				unsafe.Pointer(&template[0]), cI32(len(template)),
				nil, 0,
				unsafe.Pointer(&truncatedFonts[0]), cI32(len(truncatedFonts)),
				&out.token, &out.result, &out.length)
		}},
		{"duplicate face name", func(out *outParams) cI32 {
			return folio8_validate(unsafe.Pointer(&template[0]), cI32(len(template)),
				unsafe.Pointer(&template[0]), cI32(len(template)),
				nil, 0,
				unsafe.Pointer(&duplicateFonts[0]), cI32(len(duplicateFonts)),
				&out.token, &out.result, &out.length)
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out outParams
			if status := c.run(&out); status != statusErrorArgument {
				t.Fatalf("status %d, want %d (FOLIO8_ERROR_ARGUMENT)", status, statusErrorArgument)
			}
			if out.token != 0 || out.result != nil || out.length != 0 {
				t.Errorf("a refused call wrote token=%d result=%p length=%d; it must write nothing",
					uint64(out.token), out.result, int32(out.length))
			}
		})
	}

	if after := int(folio8_allocation_count()); after != before {
		t.Errorf("the allocation table moved %d -> %d across refused calls; FOLIO8_ERROR_ARGUMENT must allocate nothing", before, after)
	}
}

// TestNullOutParametersAreRefused covers the other half of the argument
// guard: an out-parameter the caller forgot.
func TestNullOutParametersAreRefused(t *testing.T) {
	var out outParams
	if status := folio8_version(nil, &out.result, &out.length); status != statusErrorArgument {
		t.Errorf("a nil token pointer gave status %d, want %d", status, statusErrorArgument)
	}
	if status := folio8_version(&out.token, nil, &out.length); status != statusErrorArgument {
		t.Errorf("a nil result pointer gave status %d, want %d", status, statusErrorArgument)
	}
	if status := folio8_version(&out.token, &out.result, nil); status != statusErrorArgument {
		t.Errorf("a nil length pointer gave status %d, want %d", status, statusErrorArgument)
	}
}

// TestAPanicBecomesAStatusAndAMessageFrame drives the recover in call. A
// panic crossing a c-shared boundary would take the host process with it, so
// this is the guard that keeps a defect in the engine from becoming a crash
// in someone's .NET application.
func TestAPanicBecomesAStatusAndAMessageFrame(t *testing.T) {
	before := int(folio8_allocation_count())

	var out outParams
	status := call(&out.token, &out.result, &out.length, func() (int32, []byte) {
		panic("deliberate, from the ABI's own test")
	})
	if status != statusErrorPanic {
		t.Fatalf("status %d, want %d (FOLIO8_ERROR_PANIC)", status, statusErrorPanic)
	}

	buf := frameBytes(t, &out)
	if len(buf) == 0 || buf[0] != kindErrorMessage {
		t.Fatalf("frame kind %v, want %d (a message frame)", buf[:min(1, len(buf))], kindErrorMessage)
	}
	message, rest := readStr(t, buf[1:])
	if len(rest) != 0 {
		t.Errorf("%d bytes trail the message in a kind-%d frame", len(rest), kindErrorMessage)
	}
	const want = "folio8 cshared: panic: deliberate, from the ABI's own test"
	if message != want {
		t.Errorf("message %q, want %q", message, want)
	}

	if after := int(folio8_allocation_count()); after != before {
		t.Errorf("the allocation table moved %d -> %d; a recovered panic still owes exactly one free", before, after)
	}
}

// TestAbiVersionIsReportedWithoutAllocating pins the one export a caller may
// make before anything else.
func TestAbiVersionIsReportedWithoutAllocating(t *testing.T) {
	before := int(folio8_allocation_count())
	if got := int(folio8_abi_version()); got != abiVersion {
		t.Errorf("folio8_abi_version() = %d, want %d", got, abiVersion)
	}
	if after := int(folio8_allocation_count()); after != before {
		t.Errorf("folio8_abi_version allocated; it must not (%d -> %d)", before, after)
	}
}

// TestUnknownAndDoubleFreesAreRefused pins the ownership rule at the ABI: the
// managed binding has no way to free a token twice, which is why this lives
// here.
func TestUnknownAndDoubleFreesAreRefused(t *testing.T) {
	var out outParams
	if status := folio8_version(&out.token, &out.result, &out.length); status != statusOK {
		t.Fatalf("folio8_version returned %d", status)
	}
	if status := folio8_free(out.token); status != statusOK {
		t.Fatalf("the first free returned %d, want %d", status, statusOK)
	}
	if status := folio8_free(out.token); status != statusErrorUnknownFree {
		t.Errorf("the second free returned %d, want %d (FOLIO8_ERROR_UNKNOWN_FREE)", status, statusErrorUnknownFree)
	}
	if status := folio8_free(0); status != statusErrorUnknownFree {
		t.Errorf("freeing token 0 returned %d, want %d", status, statusErrorUnknownFree)
	}
}
