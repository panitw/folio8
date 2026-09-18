# Expressions in a folio8 template

Anything between double braces is an expression: `{{customer.name}}`. folio8 evaluates it when the
report renders and substitutes the result into the document.

Expressions are deliberately small. There are **eight functions and no more** — no loops, no
assignment or general scripting. Comparisons, exact decimal arithmetic and nested conditionals compose
with the same eight functions across existing expression inputs.

> All eight functions are implemented. A function called with the wrong kind of argument, or with
> data that cannot support what it asks for, still produces an error naming the element rather than
> a plausible-looking value: a `sum` that quietly returned `0` on bad input would be a wrong total on
> a statement, and wrong totals are worse than errors.

These references cover the rest: [the `.folio` format](folio-format.md) is every field of a template, and the
[rendering library guide](rendering-library.md) renders one from Go — as [folio-js](folio-js.md) does from Node
and [folio-dotnet](folio-dotnet.md) does from .NET.

---

## Formulas and visibility

Visibility takes a bare formula, without `=` or `{{ }}`: `loanAmount > 20000` shows the element for 25000 and hides it for 20000 or 19999. Empty Visibility, an absent field, or JSON `"visibleIf": null` means always visible. The expression string `"visibleIf": "null"` hides the element.

Conditions accept booleans and null: `true` shows or selects the first branch; `false` and `null` select the other branch. Numbers and strings have no truthiness. Bold, Italic and all other fixed boolean properties remain literal controls.

Highest precedence first:

| Syntax | Association |
|---|---|
| `(expression)`, paths, calls, string/number literals, `true`, `false`, `null` | Grouping |
| Unary `+`, `-` | Right |
| `*`, `/`, `%` | Left |
| `+`, `-` | Left |
| `>`, `<`, `>=`, `<=` | No repeated unparenthesized comparisons |
| `!=` | No repeated unparenthesized comparisons |
| `condition ? then : else` | Right |

`x ? y : a ? b : c` means `x ? y : (a ? b : c)`. For example, `vip ? true : (blocked ? false : loanAmount > 20000)` checks VIP, blocked status and the threshold. `2 + 3 * 4` is 14; `(2 + 3) * 4` is 20.

Lowercase whole words `true`, `false` and `null` are literals and never look up data. `trueFlag`, `True`, `record.true` and `params.null` remain paths. `true.field` and `true()` are syntax errors. Quoted words stay strings. There is no `==`, `===`, `!==`, `&&`, `||`, `!`, assignment, or scripting.

Ordering requires two numbers. `!=` accepts same-kind scalar values, comparing numbers by mathematical value, strings exactly and booleans directly. Null differs from every non-null scalar; `null != null` is false. Missing paths remain errors, including `customer.middleName != null` when the field is absent. Collections and non-null mixed scalar kinds are errors.

Arithmetic accepts numbers only and uses exact bounded Decimals, never floating point. Addition, subtraction and remainder retain the smaller operand exponent; multiplication adds exponents; unary signs retain scale. Remainder uses truncation toward zero: `-5 % 2` is -1 and `5 % -2` is 1.

Division uses `max(operand decimal scales, 0) + 4` fractional places, rounded half to even at each `/`, retaining all result trailing zeros: `1 / 3` is `0.3333`, `1.00 / 3` is `0.333333`, `1 / 8` is `0.1250`, and `12 / 3 / 2` is `2.00000000`. Half ties include `1 / 32 = 0.0312` and `3 / 32 = 0.0938`. Tiny results can round to zero: `1 / 100000 = 0.0000`. Zero divisors, overflow and exhausted limits are located errors. Coefficients must fit int64 at the required scale, and exponent magnitude is at most 100000; trailing zeros cannot be removed to avoid overflow. Thus `1000000000000000 / 1` fails.

Both branches of `if()` and ternaries are parsed and statically checked. Unknown functions and provably wrong types such as `false ? upper(1) : "ok"` fail at load or commit. Only the selected branch resolves data or runs calculations: `true ? true : missingFlag` succeeds. Each statically known branch must satisfy the consuming field's kind. Text accepts a string, a number or null: `{{true ? "Yes" : "No"}}` renders Yes, `{{null}}` renders empty, `{{1}}` renders 1, and `{{true}}` is an error. A number in text — from data or computed, including count results — prints as its exact decimal; use `formatNumber` for grouping, fixed decimals or locale styling. The printer never rounds, but division and `avg` results carry their rounded division scale: `{{1 / 3}}` prints `0.3333`, and `{{avg(...)}}` over 1 and 2 prints `1.5000`.

Expressions are bounded to 64 KiB, 4096 AST nodes, depth 64 and 1,000,000 evaluation work units shared across nodes, strings, collection projection and decimal shifts. Errors identify the field and element, with a source-relative UTF-8 byte offset when available. Refused edits leave document bytes and history unchanged. The Visibility editor retains its separate 512-byte field limit; a valid longer engine formula is refused by that editor without changing the document.

Formula syntax and boolean/null literals in any expression container require the unreleased `2.0` format on save. The requirement is derived from the parsed AST; quoted punctuation and ordinary paths do not raise it. Saving never lowers a loaded version. There is no migration or additional major version.

No-data preview needs no fabricated data for literals. Other paths receive compatible defaults: numbers zero, direct divisors one, strings empty, and collections empty. A path used bare in text may take the zero (or divisor one) stand-in, so a path shared by text and `formatNumber` previews. Both branches contribute requirements. Conflicting requirements or a computed zero divisor refuse preview with sample-data guidance; valid formulas remain committable. Generated data does not guarantee a true condition. Parameters still come from Preview inputs.

## Reading values

### A plain path

Paths read from the data you supply with the report.

```
{{customer.name}}                    Somchai Srisuk
{{account.number}}                   123-4-56789-0
{{statement.closingBalance}}         48250.75
```

A path always reads from the top of your data, wherever it appears in the document.

### Parameters

Values supplied separately from the report data — a run date, a branch code, a report title — live in
their own namespace.

```
{{params.reportDate}}
{{params.branchCode}}
```

`params` can never be shadowed. A path beginning `params.` always means the parameter you supplied,
even inside a repeating region that happens to use the same word.

### Rows in a repeating region

A repeating region binds to a collection and gives each row a name:

```json
{ "bind": "transactions[]", "as": "transaction" }
```

Inside that region, the name you chose reads from the current row:

```
{{transaction.date}}
{{transaction.description}}
{{transaction.amount}}
```

Leave `as` out and the rows are called `row`. An unqualified path still reads from the top of your
data — **a row never shadows the document root**, so `{{customer.name}}` inside a transaction row
still means the customer, not a `name` field on the transaction.

You cannot name a region `params`, `page`, or `pages`. Those are reserved: `params` because it can
never be shadowed, and `page`/`pages` because nothing in a folio8 template may ever refer to the page
it sits on. Using one is an error naming the element, raised **when the report renders**.

---

## Text

### `upper(text)` · `lower(text)`

```
{{upper(customer.name)}}             SOMCHAI SRISUK
{{lower(customer.email)}}            somchai@example.com
```

---

## Choosing between two values

### `if(condition, then, else)`

```
{{if(customer.isVip, "VIP", "Standard")}}
{{if(hasDiscount, discount.amount, "N/A")}}
```

**The condition must be a boolean or null** — from a literal, a path, or a formula. folio8 does not treat `0`,
`""`, or an empty list as false. If the condition is some other kind of value, that is an error
naming the element, not a guess about what you meant.

Three cases worth knowing, because they differ:

| the condition is | what happens |
|---|---|
| `true` or `false` | takes that branch |
| **missing entirely** | **an error**, naming the element and the path — usually a typo |
| **explicitly `null`** | **treated as false**, silently |

Only the branch actually taken is evaluated. That is what makes the second example above work:
`discount.amount` does not exist on rows without a discount, and folio8 never looks at it on those
rows.

A missing path in the unselected branch remains unresolved. Syntax, unknown functions and statically provable type errors in either branch are rejected before rendering.

---

## Totals

### `sum(collection.field)` · `count(collection)` · `avg(collection.field)`

`sum` and `avg` take a **projection path**: a collection, followed by the field to add up or
average across every element. `count` takes the **collection path alone** — it never looks at any
field, so an element missing the field `sum`/`avg` would need still counts.

**An aggregate is a number, and a number in text prints as its exact decimal.** A bare
aggregate prints the engine's exact value — its digits with the scale kept, never grouped,
localised, rounded or written with an exponent — exactly as a bare `{{transactions.amount}}` does:

```
{{sum(transactions.amount)}}    1234.56
{{avg(transactions.amount)}}    411.520000
{{count(transactions)}}         3
```

For styled output — grouping, a fixed number of decimals, locale digits — wrap the number in
`formatNumber(...)`; see *Dates and numbers*, below.

```
{{formatNumber(sum(transactions.amount), "#,##0.00")}}     1,234.56
```

A number in text is the one kind rule that changed: booleans, lists and objects in text are still
errors naming the element.

**Totals are exact.** folio8 adds money as decimal digits, never as binary floating point, so a
statement total is correct to the last satang no matter how many rows it covers. `avg` divides at
the greatest number of decimal places any operand carries, plus a fixed number of extra digits —
four today; **the constant is illustrative, the rule is not** — with round-half-to-even, so a
repeating average never silently loses the tie-breaking digit.

**A total always covers the whole collection**, never just the rows printed on the current page.
There is no per-page subtotal, and no expression anywhere can refer to the page it is on.

**An explicit `null` value is a zero observation, not a missing one.** A row whose amount is JSON
`null` contributes zero to `sum` and counts as one observation in `avg`'s divisor — it pulls the
average down, exactly as a real zero would. A row whose amount is **absent entirely** is a different
thing: that is an error, naming the row and the field, because the document genuinely does not say
what belongs there. Two rows that look almost the same in JSON — `{"amount": null}` and `{}` — are
treated very differently for exactly this reason.

**The same rule extends to the collection itself.** A document where `transactions` is JSON `null`
(rather than a missing key, and rather than an empty list) is treated as one zero observation, the
same as a single `null` row would be: `sum` is `0`, `count` is `1`, and `avg` is `0` at its usual
scale. This is different again from `transactions` being genuinely **absent** from the data, which
is still an error.

**On an empty collection**, `sum` and `count` are legitimately zero. `avg` cannot divide by zero
observations, so it is not a number — but this is a **caveat the render survives**, not a failure:
a raw null result renders empty in text. Comparing `avg(items.amount) != 0` yields true and `avg(items.amount) != null` yields false, both with one empty-average warning, even when Visibility hides the element. A formatter still requires a numeric operand. This is different from an all-null collection, whose average **is** a real number
(zero, at the scale the rule above derives) — a collection with rows that happen to be blank and a
collection with no rows at all are not the same subject, and render differently on purpose.

---

## Dates and numbers

### `formatDate(value, pattern)`

```
{{formatDate(statement.date, "d MMMM yyyy")}}

  locale th     15 สิงหาคม 2569        ← Buddhist era
  locale en     15 August 2026
```

The value must be either an RFC 3339 date string (`2026-08-15T00:00:00Z`) or a number of milliseconds
since the epoch. Anything else is an error.

### `formatNumber(value, pattern)`

```
{{formatNumber(transaction.amount, "#,##0.00")}}     1,234.56
{{formatNumber(statement.balance,  "#,##0")}}        48,251
```

### Locale

The document declares its locale and a fixed UTC offset. folio8 ships tables for exactly four:

`en` · `th` · `zh-Hans` · `ja`

Any other tag is reported when the template loads — folio8 will not quietly fall back to something
close. **The machine rendering the report never affects the output**: its locale, its time zone and
its clock are all ignored, so the same template and the same data produce the same bytes anywhere.

---

## The whole list

Eight, and the engine enforces that count.

| | function | status |
|---|---|---|
| Text | `upper` · `lower` | implemented |
| Choice | `if` | implemented |
| Totals | `sum` · `count` · `avg` | implemented |
| Formatting | `formatDate` · `formatNumber` | implemented |

Adding a ninth is not a small change and is not meant to be — the table is closed, and a new entry
has to be made deliberately and visibly.
