// The eleven faces fonts.Shipped() returns, as folio-go/fonts/fonts.go names
// them: face name -> the path of its .ttf below folio-go/fonts/. This is the
// one copy of that table in folio-js; build-fonts.mjs writes it into
// fonts/manifest.json and src/fonts.ts reads only the manifest, so the shipped
// package never restates it. Drift from Go is caught by checking every entry
// against test/data/go-parity.json's shippedFaces, which
// folio-go/wasm/cmd/render/parity_test.go records from the engine itself.
export const shippedFaces = [
  { name: 'Noto Sans', dir: 'notosans', file: 'NotoSans-Regular.ttf' },
  { name: 'Noto Sans Bold', dir: 'notosans-bold', file: 'NotoSans-Bold.ttf' },
  { name: 'Noto Sans Italic', dir: 'notosans-italic', file: 'NotoSans-Italic.ttf' },
  { name: 'Noto Sans Bold Italic', dir: 'notosans-bolditalic', file: 'NotoSans-BoldItalic.ttf' },
  { name: 'Noto Sans Thai', dir: 'notosansthai', file: 'NotoSansThai-Regular.ttf' },
  { name: 'Noto Sans Thai Bold', dir: 'notosansthai-bold', file: 'NotoSansThai-Bold.ttf' },
  { name: 'Noto Sans SC', dir: 'notosanssc', file: 'NotoSansSC-Regular.ttf' },
  { name: 'Roboto', dir: 'roboto', file: 'Roboto-Regular.ttf' },
  { name: 'Roboto Bold', dir: 'roboto-bold', file: 'Roboto-Bold.ttf' },
  { name: 'Roboto Italic', dir: 'roboto-italic', file: 'Roboto-Italic.ttf' },
  { name: 'Roboto Bold Italic', dir: 'roboto-bolditalic', file: 'Roboto-BoldItalic.ttf' },
]

// AD-26: a directory holding a font binary ships its own terms and notice.
// Both travel beside every face into the tarball.
export const faceLicenceFiles = ['LICENSE-OFL.txt', 'NOTICE.md']
