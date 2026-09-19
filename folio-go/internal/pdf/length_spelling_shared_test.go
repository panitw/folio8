package pdf

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// TestAppendLengthMatchesSharedSpellingTable is Story 1.4's AC25 cross-
// check half living in internal/pdf: internal/template may not import
// internal/pdf (AD-1), so appendPoints there is a deliberate,
// independent reimplementation of this file's appendLength. Rather than
// a shared symbol, both packages' tests assert against the same value
// table at folio-go/testdata/shared/length-spelling-cases.txt (moved
// here from testdata/template/ by this story's finisher, D-1.4.14,
// Finding 13 — the table is shared by both packages and must not live
// under a path owned by just one of its two consumers) — a future
// divergence between the two spellings shows up as a red test here (or
// in internal/template's mirror test), never as a silent byte surprise
// in a rendered PDF or a serialized template.
func TestAppendLengthMatchesSharedSpellingTable(t *testing.T) {
	root, err := filepath.Abs("../../testdata/shared/length-spelling-cases.txt")
	if err != nil {
		t.Fatalf("resolve shared table path: %v", err)
	}
	f, err := os.Open(root)
	if err != nil {
		t.Fatalf("open shared spelling table: %v", err)
	}
	defer f.Close()

	cases := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed line in shared spelling table: %q", line)
		}
		mp, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			t.Fatalf("malformed millipoints in %q: %v", line, err)
		}
		cases++
		got := string(appendLength(nil, geom.Length(mp)))
		if got != parts[1] {
			t.Errorf("appendLength(%d) = %q, want %q", mp, got, parts[1])
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan shared spelling table: %v", err)
	}
	if cases == 0 {
		t.Fatal("coverage witness: zero cases read from the shared spelling table")
	}
}
