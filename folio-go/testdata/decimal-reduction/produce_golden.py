#!/usr/bin/env python3
"""AC7's producing script for Story 3.1a's Layer-1 oracle golden.

AD-25's one-time offline reference run, hand-checked, never a runtime
dependency (D-2.3.1's precedent, applied here the way it was applied to
shaping via hb-shape). Run once, by hand, to produce golden.json; never
imported or executed by any Go test.

Corpus provenance (D-000.50 population check, Story 3.1a Findings F3):
  A - a large balance near float64's integer-exactness limit (2**53)
      absorbing 32 small addends: DISCRIMINATES for both sum and avg.
  B - mixed-scale operands exercising the alignment path: does NOT
      discriminate once the mutant's result is rounded to the declared
      scale (labelled non-discriminating, AC13, shipped as evidence).
  C - seven small two-decimal amounts, the intuitive bank-statement
      fixture: does NOT discriminate at all (D-000.50's hazard,
      labelled non-discriminating, AC13, shipped as evidence).

avg's scale is `max operand scale + 4` (D-3.1a.1, illustrative
constant, MEDIUM confidence, Flag F3): the +4 is declared here exactly
once, identically to how internal/bind's kernel must declare it.
"""

import decimal
import json
from decimal import Decimal, Context, ROUND_HALF_EVEN

CONTEXT = Context(prec=50, rounding=ROUND_HALF_EVEN)
decimal.setcontext(CONTEXT)

CORPORA = {
    "A": ["12345678901234.56"] + ["0.01"] * 32,
    "B": ["100.00", "0.0001", "3", "0.000007", "2.5"],
    "C": ["0.10", "0.20", "0.30", "1.15", "2.35", "0.05", "0.07"],
}


def scale_of(literal: str) -> int:
    if "." in literal:
        return len(literal.split(".")[1])
    return 0


def main() -> None:
    result = {}
    for name, operands in CORPORA.items():
        max_scale = max(scale_of(s) for s in operands)

        total = sum((Decimal(s) for s in operands), Decimal(0))
        total = total.quantize(Decimal(1).scaleb(-max_scale), context=CONTEXT)

        avg_scale = max_scale + 4
        avg = (total / Decimal(len(operands))).quantize(
            Decimal(1).scaleb(-avg_scale), context=CONTEXT
        )

        result[name] = {
            "operands": operands,
            "sum": str(total),
            "avg": str(avg),
        }

    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()
