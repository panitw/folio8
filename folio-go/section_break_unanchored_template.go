package folio8

// sectionBreakUnanchoredTemplateJSON is
// fixtures/section-break-unanchored/input.folio, kept byte-identical to it by
// TestSectionBreakUnanchoredGoldenFixture (line-spacing's hand-sync precedent).
//
// It is spec-section-break's golden for CAP-7: the synthetic bilingual
// statement of section-break-statement with an UNANCHORED break
// ("sectionBreakAnchor": false) at 390pt, 20pt above the transaction-code
// legend. Thirty-five rows end past the line on page 1 and the legend still
// fits below them, so it is pushed down on page 1, 20pt below the last row.
// No real customer data.
const sectionBreakUnanchoredTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e3", "type": "text", "x": 0, "y": 0, "width": 523, "height": 14, "value": "Customer / ลูกค้า: {{customer.name}}", "style": {"fontFamily": "body", "fontSize": 10}},
        {"id": "e4", "type": "text", "x": 0, "y": 16, "width": 523, "height": 14, "value": "Account / เลขที่บัญชี: {{account.number}}", "style": {"fontFamily": "body", "fontSize": 10}},
        {"id": "e5", "type": "table", "x": 0, "y": 38, "bind": "transactions[]", "as": "txn", "headerHeight": 22,
          "style": {"fontFamily": "body", "fontSize": 7, "padding": {"bottom": 1, "left": 3, "right": 3, "top": 1}},
          "columns": [
            {"id": "e6", "label": "Date", "width": 60, "align": "left", "bind": "{{txn.date}}"},
            {"id": "e7", "label": "Code", "width": 40, "align": "left", "bind": "{{txn.code}}"},
            {"id": "e8", "label": "Description", "width": 243, "align": "left", "bind": "{{txn.description}}"},
            {"id": "e9", "label": "Amount", "width": 90, "align": "right", "bind": "{{formatNumber(txn.amount, \"#,##0.00\")}}"},
            {"id": "ea", "label": "Balance", "width": 90, "align": "right", "bind": "{{formatNumber(txn.balance, \"#,##0.00\")}}"}
          ]},
        {"id": "eb", "type": "text", "x": 0, "y": 410, "width": 523, "height": 16, "value": "Transaction codes / รหัสรายการ", "style": {"fontFamily": "body", "fontSize": 10}},
        {"id": "ec", "type": "text", "x": 0, "y": 430, "width": 523, "height": 14, "value": "DEP  Deposit / ฝากเงิน", "style": {"fontFamily": "body", "fontSize": 8}},
        {"id": "ed", "type": "text", "x": 0, "y": 446, "width": 523, "height": 14, "value": "WDL  Withdrawal / ถอนเงิน", "style": {"fontFamily": "body", "fontSize": 8}},
        {"id": "ee", "type": "text", "x": 0, "y": 462, "width": 523, "height": 14, "value": "TRF  Transfer / โอนเงิน", "style": {"fontFamily": "body", "fontSize": 8}},
        {"id": "ef", "type": "text", "x": 0, "y": 478, "width": 523, "height": 14, "value": "INT  Interest / ดอกเบี้ย", "style": {"fontFamily": "body", "fontSize": 8}},
        {"id": "eg", "type": "text", "x": 0, "y": 494, "width": 523, "height": 14, "value": "FEE  Service fee / ค่าธรรมเนียม", "style": {"fontFamily": "body", "fontSize": 8}}
      ],
      "sectionBreak": 390,
      "sectionBreakAnchor": false
    },
    "pageFooter": {
      "elements": [
        {"id": "eh", "type": "text", "x": 0, "y": 8, "width": 380, "height": 12, "value": "Synthetic sample data - not a real account / ข้อมูลตัวอย่าง", "style": {"fontFamily": "body", "fontSize": 7}},
        {"id": "ei", "type": "text", "x": 400, "y": 8, "width": 123, "height": 12, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "body", "fontSize": 7, "align": "right"}}
      ],
      "height": 34
    },
    "pageHeader": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 10, "width": 523, "height": 16, "value": "STATEMENT OF ACCOUNT / ใบแจ้งยอดบัญชี", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 30, "width": 523, "height": 14, "value": "Folio Example Bank (synthetic)", "style": {"fontFamily": "body", "fontSize": 9}}
      ],
      "height": 54
    }
  },
  "fonts": {"body": ["Noto Sans", "Noto Sans Thai"]},
  "locale": "th",
  "nextId": 19,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+07:00",
  "version": "4.1"
}
`

// sectionBreakUnanchoredDataJSON is the record the fixture renders, kept
// byte-identical to fixtures/section-break-unanchored/data.json: the first
// thirty-five of section-break-statement's synthetic transactions.
const sectionBreakUnanchoredDataJSON = `{"account":{"number":"123-4-56789-0"},"customer":{"name":"Somchai Example / สมชาย ตัวอย่าง"},"transactions":[{"amount":45000.00,"balance":55000.00,"code":"DEP","date":"2026-08-01","description":"Salary deposit"},{"amount":-2000.00,"balance":53000.00,"code":"WDL","date":"2026-08-01","description":"ถอนเงินสด ATM"},{"amount":-1500.00,"balance":51500.00,"code":"TRF","date":"2026-08-02","description":"Transfer to savings"},{"amount":-20.00,"balance":51480.00,"code":"FEE","date":"2026-08-03","description":"ค่าธรรมเนียมรายเดือน"},{"amount":1200.50,"balance":52680.50,"code":"DEP","date":"2026-08-04","description":"รับโอนเงิน"},{"amount":-847.25,"balance":51833.25,"code":"TRF","date":"2026-08-04","description":"Bill payment - electricity"},{"amount":-1000.00,"balance":50833.25,"code":"WDL","date":"2026-08-05","description":"Cash withdrawal"},{"amount":12.34,"balance":50845.59,"code":"INT","date":"2026-08-06","description":"ดอกเบี้ยเงินฝาก"},{"amount":45000.00,"balance":95845.59,"code":"DEP","date":"2026-08-07","description":"Salary deposit"},{"amount":-2000.00,"balance":93845.59,"code":"WDL","date":"2026-08-07","description":"ถอนเงินสด ATM"},{"amount":-1500.00,"balance":92345.59,"code":"TRF","date":"2026-08-08","description":"Transfer to savings"},{"amount":-20.00,"balance":92325.59,"code":"FEE","date":"2026-08-09","description":"ค่าธรรมเนียมรายเดือน"},{"amount":1200.50,"balance":93526.09,"code":"DEP","date":"2026-08-10","description":"รับโอนเงิน"},{"amount":-847.25,"balance":92678.84,"code":"TRF","date":"2026-08-10","description":"Bill payment - electricity"},{"amount":-1000.00,"balance":91678.84,"code":"WDL","date":"2026-08-11","description":"Cash withdrawal"},{"amount":12.34,"balance":91691.18,"code":"INT","date":"2026-08-12","description":"ดอกเบี้ยเงินฝาก"},{"amount":45000.00,"balance":136691.18,"code":"DEP","date":"2026-08-13","description":"Salary deposit"},{"amount":-2000.00,"balance":134691.18,"code":"WDL","date":"2026-08-13","description":"ถอนเงินสด ATM"},{"amount":-1500.00,"balance":133191.18,"code":"TRF","date":"2026-08-14","description":"Transfer to savings"},{"amount":-20.00,"balance":133171.18,"code":"FEE","date":"2026-08-15","description":"ค่าธรรมเนียมรายเดือน"},{"amount":1200.50,"balance":134371.68,"code":"DEP","date":"2026-08-16","description":"รับโอนเงิน"},{"amount":-847.25,"balance":133524.43,"code":"TRF","date":"2026-08-16","description":"Bill payment - electricity"},{"amount":-1000.00,"balance":132524.43,"code":"WDL","date":"2026-08-17","description":"Cash withdrawal"},{"amount":12.34,"balance":132536.77,"code":"INT","date":"2026-08-18","description":"ดอกเบี้ยเงินฝาก"},{"amount":45000.00,"balance":177536.77,"code":"DEP","date":"2026-08-19","description":"Salary deposit"},{"amount":-2000.00,"balance":175536.77,"code":"WDL","date":"2026-08-19","description":"ถอนเงินสด ATM"},{"amount":-1500.00,"balance":174036.77,"code":"TRF","date":"2026-08-20","description":"Transfer to savings"},{"amount":-20.00,"balance":174016.77,"code":"FEE","date":"2026-08-21","description":"ค่าธรรมเนียมรายเดือน"},{"amount":1200.50,"balance":175217.27,"code":"DEP","date":"2026-08-22","description":"รับโอนเงิน"},{"amount":-847.25,"balance":174370.02,"code":"TRF","date":"2026-08-22","description":"Bill payment - electricity"},{"amount":-1000.00,"balance":173370.02,"code":"WDL","date":"2026-08-23","description":"Cash withdrawal"},{"amount":12.34,"balance":173382.36,"code":"INT","date":"2026-08-24","description":"ดอกเบี้ยเงินฝาก"},{"amount":45000.00,"balance":218382.36,"code":"DEP","date":"2026-08-25","description":"Salary deposit"},{"amount":-2000.00,"balance":216382.36,"code":"WDL","date":"2026-08-25","description":"ถอนเงินสด ATM"},{"amount":-1500.00,"balance":214882.36,"code":"TRF","date":"2026-08-26","description":"Transfer to savings"}]}
`
