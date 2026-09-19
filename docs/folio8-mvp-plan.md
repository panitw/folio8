# folio8 MVP Plan

## Product Goal

Build a modern report designer and rendering platform positioned as an alternative to JasperReports, with a strong focus on:

- Simple visual report design
- JSON-first data binding
- Deterministic PDF rendering
- Developer-friendly integration
- A native Go rendering library as a first-class MVP component

The MVP should prove one core workflow:

> **Design → Bind → Preview → Render**

---

## MVP Core Deliverables

The first MVP should include four core components:

1. **folio8 Designer**
2. **folio8 Template Format**
3. **folio-go Rendering Library**
4. **PDF Output**

```text
                FOLIO8 MVP

        ┌─────────────────────┐
        │   folio8 Designer    │
        └──────────┬──────────┘
                   │
                   ▼
        ┌─────────────────────┐
        │   folio8 Template    │
        │      (.folio)       │
        └──────────┬──────────┘
                   │
                   ▼
        ┌─────────────────────┐
        │      folio-go       │
        │                     │
        │ Expression Engine   │
        │ Layout Engine       │
        │ Pagination Engine   │
        │ PDF Renderer        │
        └──────────┬──────────┘
                   │
                   ▼
                  PDF
```

---

# 1. folio8 Designer

The visual designer is the main design-time experience.

## MVP Capabilities

### Canvas

- Drag-and-drop report canvas
- Page boundaries
- A4 page size
- Letter page size
- Custom page size
- Portrait / landscape orientation
- Page margins
- Grid / snapping
- Component resize
- Component positioning

### Components

Support only the most important components initially:

- Text
- Image
- Table
- Line
- Rectangle

These should cover the majority of common enterprise reporting use cases.

### Properties

Basic component properties should include:

- X / Y position
- Width / height
- Font family
- Font size
- Bold / italic
- Text alignment
- Vertical alignment
- Border
- Padding
- Background
- Visibility
- Data binding

### Report Sections

Keep report structure simple for the MVP:

```text
Report

├── Page Header
│
├── Content
│
└── Page Footer
```

More advanced concepts such as group headers, report headers, and group footers can come later.

---

# 2. JSON-First Data Model

folio8 should be independent from databases.

Applications should prepare report data and send it to folio8 as JSON.

Example:

```json
{
  "customer": {
    "name": "John Smith",
    "account": "1234567890"
  },
  "transactions": [
    {
      "date": "2026-08-01",
      "description": "Transfer",
      "amount": 5000
    },
    {
      "date": "2026-08-02",
      "description": "Payment",
      "amount": -1200
    }
  ]
}
```

Bindings:

```text
{{customer.name}}
{{customer.account}}
```

Table binding:

```text
transactions[]
```

Recommended architecture:

```text
Database
   │
   ▼
Application / Microservice
   │
   │ JSON
   ▼
folio8
   │
   ▼
PDF
```

Database connectivity should not be part of the first MVP.

---

# 3. folio8 Template Format

The designer should produce a portable, text-based template.

Recommended extension:

```text
customer-statement.folio
```

The content can remain JSON internally.

Example:

```json
{
  "version": "1.0",
  "page": {
    "size": "A4",
    "orientation": "portrait",
    "margin": {
      "top": 20,
      "right": 20,
      "bottom": 20,
      "left": 20
    }
  },
  "body": [
    {
      "type": "text",
      "value": "Account Statement"
    }
  ]
}
```

## Template Design Goals

Templates should be:

- Human-readable
- Git-friendly
- Versionable
- Diffable
- CI/CD friendly
- API friendly
- AI-editable

---

# 4. folio-go Rendering Library

The Go rendering library should be a **core MVP deliverable**, not a later SDK.

It should be the reference implementation of the folio8 rendering engine.

Suggested package:

```text
github.com/folio8-reports/folio8
```

or:

```text
github.com/folio8-reports/folio-go
```

## Primary API

The API should be extremely simple.

```go
pdf, err := folio8.Render(template, data)
```

Example:

```go
package main

import (
    "os"

    "github.com/folio8-reports/folio8"
)

func main() {
    template, err := folio8.LoadTemplate("statement.folio")
    if err != nil {
        panic(err)
    }

    data := map[string]any{
        "customer": map[string]any{
            "name": "John Smith",
        },
        "transactions": []map[string]any{
            {
                "date":   "2026-08-01",
                "amount": 1500.00,
            },
        },
    }

    pdf, err := folio8.Render(template, data)
    if err != nil {
        panic(err)
    }

    os.WriteFile("statement.pdf", pdf, 0644)
}
```

## Streaming API

Large reports should support streaming output.

```go
err := folio8.RenderTo(
    writer,
    template,
    data,
)
```

Example HTTP handler:

```go
func reportHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/pdf")

    folio8.RenderTo(
        w,
        template,
        data,
    )
}
```

This avoids temporary files and supports server-side report generation efficiently.

---

# 5. folio-go Internal Architecture

Recommended internal modules:

```text
folio-go
│
├── template
│   ├── parser
│   ├── schema
│   └── validator
│
├── expression
│   ├── evaluator
│   └── functions
│
├── layout
│   ├── text
│   ├── table
│   ├── pagination
│   └── measurement
│
├── renderer
│   └── pdf
│
└── folio8
    └── public API
```

The processing pipeline should be:

```text
Template
   ↓
Expression Evaluation
   ↓
Layout Calculation
   ↓
Page Model
   ↓
PDF Renderer
```

Avoid this architecture:

```text
Template
   ↓
Draw Directly Onto PDF
```

Instead, build an intermediate page/layout model.

Conceptually:

```go
type Page struct {
    Width    float64
    Height   float64
    Elements []Element
}
```

This separation will make additional renderers possible later.

```text
                    ┌── PDF
Template → Layout ──┼── PNG
                    ├── SVG
                    └── HTML
```

Excel should likely use different layout semantics and should not be forced into the same rendering model.

---

# 6. Expressions

The MVP needs a small expression language.

Example bindings:

```text
{{customer.name}}

{{transaction.amount}}

{{sum(transactions.amount)}}

{{formatDate(transaction.date, "dd/MM/yyyy")}}

{{formatNumber(transaction.amount, "#,##0.00")}}
```

## MVP Functions

Aggregation:

- `sum()`
- `count()`
- `avg()`

Formatting:

- `formatDate()`
- `formatNumber()`

String:

- `upper()`
- `lower()`

Logic:

- `if()`

Avoid creating a general-purpose scripting language in the first MVP.

---

# 7. Parameters

Reports should support runtime parameters separately from report data.

Example:

```json
{
  "reportDate": "2026-08-22",
  "branchName": "Bangkok",
  "preparedBy": "Operations"
}
```

Bindings:

```text
Statement Date: {{params.reportDate}}
Branch: {{params.branchName}}
```

---

# 8. Table and Repeating Sections

Table rendering is one of the most important technical areas in the entire MVP.

Required table behavior:

```text
Table Header

Row
Row
Row
Row
...

--- Page Break ---

Table Header

Row
Row
Row
```

## MVP Table Features

- Dynamic rows
- Data collection binding
- Fixed column widths
- Column alignment
- Header row
- Borders
- Cell padding
- Page breaking
- Repeated table headers
- Sum footer
- Count footer
- Average footer

Optional if capacity permits:

- Alternating row style

Correct pagination should take priority over advanced table styling.

---

# 9. PDF Renderer

PDF should be the only required output format for the first MVP.

Do not initially attempt to support:

- Excel
- Word
- HTML
- CSV
- PNG
- PowerPoint

Different output formats have significantly different layout semantics.

The MVP rendering pipeline should be:

```text
Template
   +
Data
   +
Parameters
       │
       ▼
Rendering Engine
       │
       ▼
      PDF
```

---

# 10. Deterministic Rendering

Deterministic rendering should be an explicit MVP requirement.

> The same template and data should generate the same layout in every environment.

For example:

```text
Developer Laptop
      │
      ├── folio-go v0.1
      │
Production Linux Container
      │
      ├── folio-go v0.1
      │
      ▼
Identical Pagination
Identical Line Wrapping
Identical Table Breaks
```

This requires careful control of:

- Font handling
- Font embedding
- Text measurement
- Line wrapping
- Page dimensions
- Pagination rules
- Image sizing

The browser should not be the canonical renderer.

---

# 11. Preview Architecture

The designer should provide two preview modes.

## Design Canvas

Purpose:

- Fast interaction
- Drag-and-drop editing
- Approximate visual representation

## PDF Preview

Purpose:

- Exact production rendering
- Generated using folio-go
- Same semantics as production output

Recommended flow:

```text
Designer
   │
   │ template + sample JSON
   ▼
Preview API
   │
   ▼
folio-go
   │
   ▼
PDF
```

This avoids discrepancies between the designer and the production renderer.

---

# 12. Developer Rendering API

A REST rendering service can be included if time permits, although the Go library is the higher-priority runtime integration.

Example:

```http
POST /api/reports/customer-statement/render
```

Body:

```json
{
  "parameters": {
    "reportDate": "2026-08-22"
  },
  "data": {
    "customer": {},
    "transactions": []
  }
}
```

Response:

```text
Content-Type: application/pdf
```

Potential future APIs:

```text
POST /render/pdf
POST /render/html
POST /render/xlsx
```

Only PDF is required for MVP.

---

# 13. MVP Feature Matrix

| Capability | MVP Priority |
|---|---|
| Web visual designer | Must |
| JSON template format | Must |
| `.folio` template file | Must |
| JSON data binding | Must |
| Text | Must |
| Image | Must |
| Line | Must |
| Rectangle | Must |
| Table | Must |
| Page header / footer | Must |
| Page number | Must |
| Parameters | Must |
| Basic expressions | Must |
| Sum / Count / Avg | Must |
| Table pagination | Must |
| Repeated table header | Must |
| PDF generation | Must |
| PDF preview | Must |
| **Go rendering library** | **Core** |
| Streaming rendering API | Important |
| REST rendering service | Optional |
| Java SDK | Later |
| .NET SDK | Later |
| Node.js SDK | Later |
| Excel export | Later |
| Charts | Later |
| Subreports | Later |

---

# 14. Explicitly Out of Scope for MVP

Do not attempt to reproduce JasperReports feature-for-feature.

Postpone:

- Subreports
- Charts
- Excel export
- Word export
- Barcode
- QR code
- Pivot tables
- SQL query designer
- Direct database connections
- Scheduled reports
- Email delivery
- Role-based access control
- Multi-tenancy
- Report repository
- General-purpose scripting
- Advanced conditional formatting
- Internationalization
- Advanced font management UI
- Digital signatures
- Report bursting
- Jasper `.jrxml` compatibility

---

# 15. Recommended Development Order

The Go rendering engine should be implemented before investing heavily in the designer.

Recommended sequence:

## Phase 1 — Template and Renderer Foundation

Build:

- `.folio` schema
- Template parser
- Template validator
- Text rendering
- Page model
- PDF renderer
- Font handling

Goal:

```go
folio8.Render(template, data)
```

works for simple one-page reports.

## Phase 2 — Layout and Pagination

Build:

- Text wrapping
- Element measurement
- Page boundaries
- Headers
- Footers
- Page numbers
- Multi-page rendering

## Phase 3 — Tables

Build:

- Data collection binding
- Table rows
- Column layout
- Page breaking
- Repeated headers
- Aggregation

This is expected to be one of the highest-risk engineering areas.

## Phase 4 — Expression Engine

Build:

- Field binding
- Parameters
- Formatting
- Aggregation
- Basic conditional logic

## Phase 5 — Designer

Build:

- Canvas
- Components
- Properties panel
- Drag / resize
- Data binding UI
- Template save/load

## Phase 6 — Production Preview

Integrate the designer with folio-go so that final preview uses the real production renderer.

---

# 16. Golden Report for MVP Validation

Use one representative enterprise report as the main acceptance test.

Recommended example:

## Customer Account Statement

Contents:

```text
Logo

Customer Information
Account Information
Statement Period

Transaction Table
├── Date
├── Description
├── Debit
├── Credit
└── Balance

Footer
├── Confidentiality Text
├── Generated Date
└── Page X of Y
```

Test with:

- 1 page
- 5 pages
- 20 pages
- 50 pages

The report should reliably handle:

- Correct text wrapping
- Exact page size
- Correct page breaks
- Repeated table headers
- Totals
- Images
- Headers
- Footers
- `Page X of Y`
- Embedded fonts
- Deterministic rendering

---

# 17. MVP Success Criteria

folio8 v0.1 should be considered successful when:

1. A user can create a report in the visual designer.
2. The report can bind to JSON data.
3. The template can be saved as a portable `.folio` file.
4. A Go application can load that template.
5. The application can render it using:

```go
pdf, err := folio8.Render(template, data)
```

6. A professional multi-page PDF is produced.
7. Tables paginate correctly.
8. Table headers repeat correctly.
9. Headers and footers are rendered correctly.
10. Page numbering works.
11. Fonts and layouts are deterministic across environments.
12. The exact production renderer can be used for designer preview.

---

# Product Principle

The first release should optimize for reliability rather than feature breadth.

> **If folio8 v0.1 can reliably generate a professional 20–50 page enterprise statement from JSON using a `.folio` template and `folio-go`, the MVP has proven its core value.**
