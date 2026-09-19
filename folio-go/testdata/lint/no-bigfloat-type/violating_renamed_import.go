package nobigfloattype

import bf "math/big"

// RenamedImportFloat is the AC18 "renamed import" case: the local name
// `bf` resolves to math/big, so `bf.Float` is the exact banned type
// reached through an alias — never through the literal source text
// "big.Float", which a source-text ban would miss entirely.
func RenamedImportFloat() bf.Float {
	var f bf.Float
	return f
}
