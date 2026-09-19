# Example templates

The four bundled examples (CAP-4). Data is fictional and English: invented company, people,
addresses and account numbers — no real organization, and account/meter numbers that cannot be
mistaken for real ones. Currency is neutral (two decimals via `formatNumber`, no locale-specific
symbol); dates are RFC 3339 strings formatted with `formatDate`. Each example is `<id>.folio` +
`<id>.sample.json` in `folio-designer/public/templates/examples/`, listed in `exampleIds` in dialog
order: `invoice`, `bank-statement`, `legal-contract`, `electricity-bill`.

"Must exercise" is the load-bearing column: the example exists to show an author that feature
working. Layout details beyond it are the implementer's. Pages are pinned by the Go examples test.

| Example | Pages with sample | Must exercise | Sample data shape (top level) |
|---|---|---|---|
| **Blank** | 1 | — (today's `starter.folio`, unchanged) | none |
| **Invoice** | 1 | Issuer and bill-to blocks; line-item `table` with qty, unit price and a precomputed amount column carrying `footer: "sum"`; subtotal/tax/total via `formatNumber`; issue/due dates via `formatDate`; payment reference as `qrcode` | `invoice{number, issued, due, reference}`, `seller{name, address, city, email, taxId}`, `customer{name, contact, address, city}`, `items[]{description, qty, unitPrice, amount}` (6–10 rows), `totals{subtotal, taxRate, tax, total}` |
| **Bank Statement** | 2 | `transactions[]` table paginating with a repeated header and `altRowBackground`; page header and `Page {{page}} of {{pages}}` footer; opening/closing balances; a static legend below an unanchored `sectionBreak`; empty debit/credit as `null` bound with a `!= null` conditional | `account{holder, number, address, city, period{from, to}}`, `balances{opening, closing}`, `transactions[]{date, description, debit, credit, balance}` (debit and credit keys on every row, `null` when empty; enough rows to span 2 pages) |
| **Legal Contract** | 2 | Parties block bound from data; clauses as a `clauses[]` table — number column plus a `{{clause.heading}}\n{{clause.body}}` column — whose rows move whole to the next page; footer with document reference and page numbers; signature block below an unanchored `sectionBreak`, tagged `keepTogether` | `agreement{title, reference, effective}`, `partyA`/`partyB{name, shortName, address, city, companyNumber, signatory{name, role}}`, `clauses[]{no, heading, body}` (8–12) |
| **Electricity Bill** | 1 | Account/meter summary; meter `readings[]` table; `charges[]` table with `footer: "sum"` on amount; 6-month `history[]` table (no chart element exists); overdue notice with `visibleIf: "overdue"`; payment `barcode` (Code 128, ASCII value) | `customer{name, address, city}`, `account{number, meter, tariff, paymentRef}`, `period{from, to}`, `readings[]{register, previous, current, units}`, `charges[]{label, units, rate, amount}`, `history[]{month, units}`, `arrears`, `amountDue`, `dueDate`, `overdue` (`true` in the sample so Preview shows the notice) |

## Engine limits the examples work within

- A table column has one style, so a clause heading cannot be bold apart from its body.
- No row index exists; clause numbers come from data.
- A text element never pushes the elements after it; content that must follow growing content sits
  below a `sectionBreak`.
- No chart element; usage history is a table.
- `barcode` is Code 128 only and needs an ASCII value.
- `style.bold` needs the bold face declared in the font chain (every example uses the starter's chain).

## Rules every example follows

- Declares the lowest format version its features require and stays in canonical form, so a
  no-op load/save round trip is byte-identical.
- Renders against its sample with zero diagnostics — checked in the build, which also emits the
  thumbnail from that render, and in the Go examples test.
- Sample JSON is accepted by `acceptSampleData` (the tree display may truncate long collections;
  the render receives the whole file).
