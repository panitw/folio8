// Finding 6 (this story's QA review, Major): bannedImportPaths was an
// exact-match deny-list, so a banned package's own subpackages passed
// with zero findings — math/rand/v2 (Go 1.22+, the modern spelling of
// the exact package AD-1 bans), net/http, net/url and os/exec all
// resolved to "not banned". This fixture imports all four and must be
// reported, proving the prefix-aware match closes the gap.
package forbiddenimportsfixture

import (
	mathrandv2 "math/rand/v2"
	"net/http"
	"net/url"
	"os/exec"
)

func usesBannedSubpackages() {
	_ = mathrandv2.IntN(10)
	_ = &http.Client{}
	_, _ = url.Parse("https://example.test")
	_ = exec.Command("true")
}
