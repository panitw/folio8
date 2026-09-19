package template

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// TestAppendPointsMatchesSharedSpellingTable is AC25's cross-check half
// living in internal/template — see internal/pdf's mirror test for the
// full rationale. Both assert against the same shared value table.
func TestAppendPointsMatchesSharedSpellingTable(t *testing.T) {
	root := repoRootFromTest(t)
	path := filepath.Join(root, "folio-go", "testdata", "shared", "length-spelling-cases.txt")
	f, err := os.Open(path)
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
		got := string(appendPoints(nil, geom.Length(mp)))
		if got != parts[1] {
			t.Errorf("appendPoints(%d) = %q, want %q", mp, got, parts[1])
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan shared spelling table: %v", err)
	}
	if cases == 0 {
		t.Fatal("coverage witness: zero cases read from the shared spelling table")
	}
}
