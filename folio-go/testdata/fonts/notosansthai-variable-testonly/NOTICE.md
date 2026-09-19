# NotoSansThai-VF.ttf — third-party test fixture (variable font)

**Copyright 2022 The Noto Project Authors (https://github.com/notofonts/thai)**

Licensed under the SIL Open Font License, Version 1.1 (see `LICENSE-OFL.txt`
in this directory, the unmodified upstream licence text).

This is Story 2.2's variable-font (`fvar`/`gvar`/`avar`, glyf outlines)
test fixture, used ONLY by `internal/fontset`'s own unit tests to
exercise axis-pinning/instancing (AC6/AC7's discrimination fixture: two
different pinned instances of one face, same glyph set, must receive
different subset tags). It is not a shipped production face — the
shipped, embedded copy lives at `folio-go/fonts/notosansthai/` (Story
2.2, AC1/AC2).

Sourced from `github.com/google/fonts`, `ofl/notosansthai/NotoSansThai[wdth,wght].ttf`,
fetched 2026-08-24, unmodified upstream bytes.
sha256: `5a1c559bb539583c8a1fd99d1c5b9491e5e14478c9cd2bd0970d5c3096cc9ef8`

AD-26, verbatim: "Redistributed non-code assets keep their own terms and
their notices."
