# folio8 — repository-level make targets.
#
# This Makefile deliberately holds only targets that span modules or that
# drive non-Go tooling. Each Go module is built and tested with the
# ordinary `go` commands from inside its own directory, exactly as
# .github/workflows/ci.yml does — CI invokes the guardrails, it never
# re-implements them (D-1.2.6), and the same rule applies here.

# Where the upstream VARIABLE builds live when regenerating the shipped
# faces. They are NOT committed: 20 MB of inputs for 11 MB of outputs,
# and the outputs are what ship. Each face's NOTICE.md records the
# release URL and the source sha256 to fetch them by.
FONT_SOURCES ?= $(CURDIR)/.font-sources
PYTHON       ?= python3

.PHONY: help fonts fonts-verify

help:
	@echo "folio8 make targets:"
	@echo "  fonts         regenerate the shipped static faces from FONT_SOURCES"
	@echo "  fonts-verify  assert the COMMITTED faces still reproduce, without writing"
	@echo ""
	@echo "  FONT_SOURCES=$(FONT_SOURCES)"
	@echo "  PYTHON=$(PYTHON)"

## fonts — derive folio-go/fonts/*/​*-Regular.ttf from the upstream variable
## builds (D-2.2.4). The OUTPUT is committed, not generated at build time:
## generating at build time would make the shipped font a function of the
## build environment — a different fontTools produces a different font,
## which produces a different PDF — reintroducing AD-22's drift class at
## the asset layer. This target exists so the derivation can be REPLAYED,
## not so it can be run automatically.
##
## Python rather than a Go generator, deliberately: lint's
## `absence-source-date-epoch` content check keys on the literal string
## it needs to set appearing in any .go file under folio-go/ (D-2.1.5),
## so a Go generator would make that tripwire fire on legitimate work.
fonts:
	$(PYTHON) tools/fontgen/instance_faces.py --sources "$(FONT_SOURCES)" --repo-root "$(CURDIR)"

## fonts-verify — the same derivation, compared against the committed
## files without writing anything. This is what folio-go's
## //go:build matrix regeneration test runs at the epic gate.
fonts-verify:
	$(PYTHON) tools/fontgen/instance_faces.py --sources "$(FONT_SOURCES)" --repo-root "$(CURDIR)" --verify-only
