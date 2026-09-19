package folio8

import (
	"os"
	"strings"
	"testing"
)

func TestPreviewIdentityIsStableFramedAndFiveInput(t *testing.T) {
	fonts := FontSet{"z": []byte("program-z"), "a": []byte("program-a")}
	base := previewIdentity([]byte(`{"a":1}`), Data(`{"d":1}`), Params(`{"p":1}`), fonts)
	if len(base) != 64 {
		t.Fatalf("identity length = %d", len(base))
	}
	if got := previewIdentity([]byte(`{"a":1}`), Data(`{"d":1}`), Params(`{"p":1}`), FontSet{"a": []byte("program-a"), "z": []byte("program-z")}); got != base {
		t.Fatalf("unordered FontSet changed identity: %s != %s", got, base)
	}
	for _, changed := range []string{
		previewIdentity([]byte(`{"a":2}`), Data(`{"d":1}`), Params(`{"p":1}`), fonts),
		previewIdentity([]byte(`{"a":1}`), Data(`{"d":2}`), Params(`{"p":1}`), fonts),
		previewIdentity([]byte(`{"a":1}`), Data(`{"d":1}`), Params(`{"p":2}`), fonts),
		previewIdentity([]byte(`{"a":1}`), Data(`{"d":1}`), Params(`{"p":1}`), FontSet{"a": []byte("program-a"), "z": []byte("program-y")}),
	} {
		if changed == base {
			t.Fatal("one preview input did not change identity")
		}
	}
	if left, right := previewIdentity([]byte("ab"), Data("c"), Params("d"), fonts), previewIdentity([]byte("a"), Data("bc"), Params("d"), fonts); left == right {
		t.Fatal("field framing admitted a concatenation collision")
	}
	// Version is a production input but intentionally not a mutable test hook.
	// This source assertion makes removing its exact framed contribution a red
	// proof instead of silently reducing the test to four runtime inputs.
	source, err := os.ReadFile("preview_identity.go")
	if err != nil || !strings.Contains(string(source), `writePreviewField(h, "folio8-version", []byte(Version))`) {
		t.Fatalf("folio8.Version is not a framed preview identity input: %v", err)
	}
}
