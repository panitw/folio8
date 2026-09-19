package folio8

// AC5-AC8 (D-1.7.2): the honest half of source AC2 — what an io.Writer
// entry point genuinely gets wrong. A behavioural Render-vs-RenderTo
// byte comparison is deliberately NOT written here (AC4, D-1.7.2): the
// AST test in render_arch_test.go proves the shared-core shape, which
// makes such a comparison unfailable and therefore worthless as
// coverage.

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
)

// countingWriter records every Write call's payload (for AC8's
// concatenation/call-count assertion) and the total byte count it
// actually accepted (for AC7's counting-writer assertion) — a plain
// bytes.Buffer wrapper, never the buffer itself, so the byte count is
// derived from what Write reported, not re-measured from the buffer's
// own length (which would make the count trivially equal by
// construction).
type countingWriter struct {
	buf      bytes.Buffer
	total    int
	payloads [][]byte
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.buf.Write(p)
	w.total += n
	cp := make([]byte, len(p))
	copy(cp, p)
	w.payloads = append(w.payloads, cp)
	return n, err
}

// errWriter fails every Write with a distinct, located error (AC5).
type errWriter struct{ err error }

func (w errWriter) Write(p []byte) (int, error) { return 0, w.err }

// partialThenErrWriter is this story's review, Finding 8 (Minor):
// errWriter above never accepts any bytes before failing, so the
// "mid-write" shape AC5's wording names — a writer that accepts PART
// of the buffer and then reports an error, which render_entry.go's
// "write failed after %d of %d bytes" branch exists to report — had no
// test naming the partial count it actually reports.
type partialThenErrWriter struct{ err error }

func (w partialThenErrWriter) Write(p []byte) (int, error) {
	return len(p) / 2, w.err
}

// shortWriter reports fewer bytes written than it was given, with a
// nil error — exactly the shape io.Writer's contract permits and
// Story 1.1's TestMain learned to check for the hard way (Nit 25,
// AC6) — while ALSO recording the total it claims to have accepted,
// so AC7's counting assertion has a real shortfall number to name.
type shortWriter struct {
	max   int
	total int
}

func (w *shortWriter) Write(p []byte) (int, error) {
	n := len(p)
	if n > w.max {
		n = w.max
	}
	w.total += n
	return n, nil
}

func renderToFixture(t *testing.T) *Template {
	t.Helper()
	return mustParseMinimalTemplate(t)
}

// TestRenderToSurfacesWriterError is AC5: a writer that errors
// mid-write surfaces THAT error — never nil, never swallowed, never
// replaced by an unrelated render error.
func TestRenderToSurfacesWriterError(t *testing.T) {
	tpl := renderToFixture(t)
	wantErr := errors.New("boom: disk full")
	w := errWriter{err: wantErr}

	_, err := RenderTo(w, tpl, Data("{}"), nil, FontSet{})
	if err == nil {
		t.Fatal("AC5: RenderTo must return a non-nil error when the writer fails")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("AC5: RenderTo's error must wrap the writer's own error, got: %v", err)
	}
}

// TestRenderToSurfacesPartialThenErrWrite is Finding 8 (Minor, this
// story's review): AC5 is worded as a writer that errors MID-write —
// the shipped errWriter double never accepts any bytes first, so the
// (n > 0, err != nil) branch (render_entry.go's "write failed after %d
// of %d bytes") was reachable but unasserted. This pins both that the
// error is surfaced AND that it names the partial count actually
// reported.
func TestRenderToSurfacesPartialThenErrWrite(t *testing.T) {
	tpl := renderToFixture(t)
	fullRes, ferr := Render(tpl, Data("{}"), nil, FontSet{})
	if ferr != nil {
		t.Fatalf("Render() error: %v", ferr)
	}
	full := fullRes.Bytes
	wantErr := errors.New("boom: disk full, mid-write")
	w := partialThenErrWriter{err: wantErr}

	_, err := RenderTo(w, tpl, Data("{}"), nil, FontSet{})
	if err == nil {
		t.Fatal("AC5: RenderTo must return a non-nil error when the writer fails mid-write")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("AC5: RenderTo's error must wrap the writer's own error, got: %v", err)
	}
	wantN := strconv.Itoa(len(full) / 2)
	if !strings.Contains(err.Error(), wantN) {
		t.Fatalf("AC5: RenderTo's error must name the partial count actually reported (%s), got: %v", wantN, err)
	}
}

// TestRenderToRejectsNilWriter is Finding 4 (Major, this story's
// review): w is a new public argument this story introduces, and
// RenderTo(nil, ...) must return a located error rather than panic —
// the same D-1.5.9 reasoning already applied to Render(nil template,
// ...).
func TestRenderToRejectsNilWriter(t *testing.T) {
	tpl := renderToFixture(t)
	_, err := RenderTo(nil, tpl, Data("{}"), nil, FontSet{})
	if err == nil {
		t.Fatal("expected a located error for RenderTo(nil writer, ...), got nil")
	}
	if !errors.Is(err, errNilWriter) {
		t.Fatalf("expected errNilWriter, got: %v", err)
	}
}

// TestRenderToDetectsShortWrite is AC6: a writer that accepts fewer
// bytes than it was given, with err == nil (which io.Writer's
// contract permits), must still produce a non-nil error from
// RenderTo.
func TestRenderToDetectsShortWrite(t *testing.T) {
	tpl := renderToFixture(t)
	fullRes, ferr := Render(tpl, Data("{}"), nil, FontSet{})
	if ferr != nil {
		t.Fatalf("Render() error: %v", ferr)
	}
	full := fullRes.Bytes
	w := &shortWriter{max: len(full) - 1}

	_, err := RenderTo(w, tpl, Data("{}"), nil, FontSet{})
	if err == nil {
		t.Fatal("AC6: a short write (n < len(p), err == nil) must produce a non-nil error from RenderTo")
	}
	// AC7 (the counting half): the error must name the actual
	// shortfall — a writer that reported accepting only w.total bytes
	// out of len(full) — not merely "an error happened".
	if w.total != len(full)-1 {
		t.Fatalf("test double bug: writer's own recorded total is %d, want %d", w.total, len(full)-1)
	}
	wantN := strconv.Itoa(w.total)
	wantLen := strconv.Itoa(len(full))
	if !strings.Contains(err.Error(), wantN) || !strings.Contains(err.Error(), wantLen) {
		t.Fatalf("AC7: RenderTo's error must name the byte shortfall (wrote %s of %s), got: %v", wantN, wantLen, err)
	}
}

// TestRenderToWritesExactByteCount is AC7: the honest half of source
// AC2 — a counting writer's TOTAL observed byte count must equal
// len(b), where b is what Render returns for the same inputs. This
// compares a number derived from the writer path against a number
// derived from the byte path, and it CAN fail (unlike the vacuous
// behavioural comparison AC4 refuses).
func TestRenderToWritesExactByteCount(t *testing.T) {
	tpl := renderToFixture(t)
	wantRes, rerr := Render(tpl, Data("{}"), nil, FontSet{})
	if rerr != nil {
		t.Fatalf("Render() error: %v", rerr)
	}
	want := wantRes.Bytes

	w := &countingWriter{}
	if _, err := RenderTo(w, tpl, Data("{}"), nil, FontSet{}); err != nil {
		t.Fatalf("RenderTo() error: %v", err)
	}
	if w.total != len(want) {
		t.Fatalf("AC7: writer observed %d total bytes, want %d (len of Render's output for the same inputs)", w.total, len(want))
	}
}

// TestRenderToWritesNothingExtra is AC8: no trailing newline, no
// second Write call carrying a tail. Both the concatenated payload
// AND the call count are asserted — a single extra flush/tail Write
// call would pass a payload-only check if it wrote zero bytes, so the
// call count matters too.
func TestRenderToWritesNothingExtra(t *testing.T) {
	tpl := renderToFixture(t)
	wantRes, rerr := Render(tpl, Data("{}"), nil, FontSet{})
	if rerr != nil {
		t.Fatalf("Render() error: %v", rerr)
	}
	want := wantRes.Bytes

	w := &countingWriter{}
	if _, err := RenderTo(w, tpl, Data("{}"), nil, FontSet{}); err != nil {
		t.Fatalf("RenderTo() error: %v", err)
	}

	// Both checks run — never short-circuited by t.Fatal — so a
	// mutation that adds one extra byte AND one extra call names both
	// (D-1.7.2's register: "Names the extra byte and the extra call").
	var all []byte
	for _, p := range w.payloads {
		all = append(all, p...)
	}
	if !bytes.Equal(all, want) {
		t.Errorf("AC8: concatenation of all Write payloads (%d extra byte(s)) must equal Render's output exactly, nothing extra", len(all)-len(want))
	}
	if len(w.payloads) != 1 {
		t.Errorf("AC8: expected exactly ONE Write call, got %d", len(w.payloads))
	}
}

// TestRenderToPropagatesRenderError confirms RenderTo never attempts
// to write when Render itself fails (a nil template, AC14b) — the
// writer must see no calls at all.
func TestRenderToPropagatesRenderError(t *testing.T) {
	w := &countingWriter{}
	_, err := RenderTo(w, nil, Data("{}"), nil, FontSet{})
	if err == nil {
		t.Fatal("expected an error for RenderTo(nil template, ...)")
	}
	if len(w.payloads) != 0 {
		t.Fatalf("RenderTo must not write anything when Render fails, got %d Write calls", len(w.payloads))
	}
}

var _ io.Writer = (*countingWriter)(nil)
var _ io.Writer = errWriter{}
var _ io.Writer = (*shortWriter)(nil)
