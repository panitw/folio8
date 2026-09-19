// Package expr is Story 3.2's expression language: a hand-written
// recursive-descent parser (AD-9, D-3.2.2 — no generator, no
// third-party dependency, no general-purpose expression library) over
// the small grammar `.folio` bindings use inside "{{ }}" — a bare
// dotted path, a function call over comma-separated arguments, a
// double-quoted string literal, or a number literal — plus the eight
// named functions FR18 promises (sum, count, avg, formatDate,
// formatNumber, upper, lower, if). As of Story 3.4, ALL EIGHT are
// implemented: upper/lower/if landed at Story 3.2, sum/count/avg at
// Story 3.3, and formatDate/formatNumber — the last two, AD-12's
// locale-aware date and number formatting — at this story. The
// registered-but-unimplemented machinery that used to distinguish
// these populations (funcEntry.implemented/owningStory,
// TestUnimplementedFunctionsAreLocatedErrors and friends) is REMOVED,
// not merely retired to inert, because the table is closed at eight
// (C1) and that population cannot refill (Story 3.4's AC16, D-000.59).
// TestImplementedEntriesMatchEvalCallSwitch (table_derivational_test.go)
// and the behavioural witness in table_behavioral_test.go now assert
// the OBLIGATION that every entry computes, derivationally, rather
// than re-reading a flag.
//
// internal/expr is rank 3 in the stage-rank table
// (lint/internal/rules/stagerank.go): it may import internal/template
// (rank 2), which it does for exactly one thing — SplitJSONNumber, the
// module's one JSON-number-literal splitter (D-1.6.1) — and it is
// imported BACK by internal/bind (rank 4), never the reverse. This
// direction is forced, not chosen (D-3.2.1, D-1.6.1): DW-8 moves
// Decimal here because Decimal is built on SplitJSONNumber, so expr
// must outrank template, and bind — which needs Decimal for its own
// report-data value tree — must therefore sit below expr.
//
// Decimal, NewDecimal, SumDecimals and AvgDecimals (decimal.go,
// reduce.go) moved here from internal/bind UNCHANGED in behaviour
// (D-3.2.1): this is the one and only "Decimal" type declaration in
// the module (AC22), and D-3.1a.3's reducer-inventory tripwire — which
// asserts SumDecimals/AvgDecimals live in the SAME package as the
// Decimal declaration — is RELATIONAL and follows this move with zero
// edits to that guard.
//
// Two-phase load/evaluate split (R3, originally forced by F3 — the
// canonical golden bound a still-unimplemented formatNumber(...) call,
// so an unimplemented function could not be a load error; the split
// stays now that all eight are implemented, because arity and
// literal-argument-kind remain decidable without data while a PATH
// argument's runtime kind is not): Parse produces an AST from raw
// grammar alone, with no knowledge of the eight-function table; Check
// walks that AST against the table and reports every statically
// decidable defect — a syntax error, a wrong arity, an unknown
// function name, a literal argument of the wrong kind, or (Story 3.4,
// AC10) a formatDate/formatNumber pattern literal outside its own
// closed grammar. Eval walks the AST against a Resolver
// (evaluation-time data lookup) and actually computes a result,
// calling only the branch of an if() that was selected (AC14's
// short-circuit). Story 3.3 additionally proves sum()/avg() actually
// ROUTE through SumDecimals/AvgDecimals rather than merely producing
// an equal-looking answer some other way (routing_arch_test.go;
// D-3.1a.4's own correction that the reducer inventory alone cannot
// force this). Story 3.4's formatDate/formatNumber are hand-rolled:
// formatDate does its own calendar arithmetic (calendar.go, integer
// only, no "time" import anywhere) and formatNumber scales through an
// integer power-of-ten lookup table (tenpow.go), never math.Pow
// (AD-23: math.Pow returns float64).
package expr
