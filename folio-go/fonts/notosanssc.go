//go:build !nocjkface

// THE DEFAULT HALF OF THE PAIR: the CJK face IS embedded, so
// fonts.Shipped() returns the eleven faces every consumer of this module
// has always received. Nothing outside folio-designer's own engine build
// sets `nocjkface`, so this is the file that compiles everywhere else —
// folio-js, folio-dotnet, the CLI, the CJK golden fixtures and every
// `go test ./...` run included.
//
// ITS TWIN IS notosanssc_absent.go (`//go:build nocjkface`). The two
// constraints are complementary and are held that way by
// TestCJKFaceIsBuildTagged in accounting_test.go: a build that matched
// both would declare buildTaggedFaces twice and a build that matched
// neither would not compile at all, so the pair is checked rather than
// trusted.

package fonts

import _ "embed"

// The SC face is a STATIC, Regular-only instance derived from its
// upstream variable build by tools/fontgen/instance_faces.py (D-2.2.4),
// exactly as the other two Story 2.2 faces are, and it is COMMITTED
// rather than generated at build time for the same reason. Its
// provenance record is notosanssc/NOTICE.md and accounting_test.go
// joins the two by bytes.
//
//go:embed notosanssc/NotoSansSC-Regular.ttf
var notoSansSC []byte

// buildTaggedFaces is what Shipped() merges over its ten unconditional
// faces. Here it is the one CJK face; under `nocjkface` it is empty.
func buildTaggedFaces() map[string][]byte {
	return map[string][]byte{"Noto Sans SC": notoSansSC}
}
