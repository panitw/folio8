package folio8

// multiPageFlowTemplateJSON is fixtures/multi-page-flow/input.folio, kept
// byte-identical to it by TestMultiPageFlowGoldenFixture.
//
// It is SPEC-multi-pages' golden for CAP-6 and CAP-9: a canonical three-page
// document. Page 1 holds a synthetic statement whose transactions table runs
// two output pages, with an unanchored section break above a code legend.
// Page 2, Page Break off, holds a service-charges table with its own anchored
// section break above a note; it needs two output pages, so it does not fit
// after page 1 and starts a new one. Page 3, Page Break off, holds a signature
// block that fits under page 2's note on output page 4. No real customer data.
const multiPageFlowTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": []
    },
    "pageFooter": {
      "elements": [
        {
          "height": 12,
          "id": "e3",
          "style": {
            "fontFamily": "body",
            "fontSize": 7
          },
          "type": "text",
          "value": "Synthetic sample data - not a real account",
          "width": 380,
          "x": 0,
          "y": 8
        },
        {
          "height": 12,
          "id": "e4",
          "style": {
            "align": "right",
            "fontFamily": "body",
            "fontSize": 7
          },
          "type": "text",
          "value": "Page {{page}} of {{pages}}",
          "width": 123,
          "x": 400,
          "y": 8
        }
      ],
      "height": 34
    },
    "pageHeader": {
      "elements": [
        {
          "height": 16,
          "id": "e1",
          "style": {
            "fontFamily": "body",
            "fontSize": 12
          },
          "type": "text",
          "value": "STATEMENT OF ACCOUNT",
          "width": 523,
          "x": 0,
          "y": 10
        },
        {
          "height": 14,
          "id": "e2",
          "style": {
            "fontFamily": "body",
            "fontSize": 9
          },
          "type": "text",
          "value": "Folio Example Bank (synthetic)",
          "width": 523,
          "x": 0,
          "y": 30
        }
      ],
      "height": 54
    }
  },
  "fonts": {
    "body": [
      "Noto Sans"
    ]
  },
  "locale": "en",
  "nextId": 23,
  "page": {
    "margin": {
      "bottom": 36,
      "left": 36,
      "right": 36,
      "top": 36
    },
    "orientation": "portrait",
    "size": "A4"
  },
  "pages": [
    {
      "elements": [
        {
          "height": 14,
          "id": "e5",
          "style": {
            "fontFamily": "body",
            "fontSize": 10
          },
          "type": "text",
          "value": "Customer: {{customer.name}}",
          "width": 523,
          "x": 0,
          "y": 0
        },
        {
          "height": 14,
          "id": "e6",
          "style": {
            "fontFamily": "body",
            "fontSize": 10
          },
          "type": "text",
          "value": "Account: {{account.number}}",
          "width": 523,
          "x": 0,
          "y": 16
        },
        {
          "as": "txn",
          "bind": "transactions[]",
          "columns": [
            {
              "align": "left",
              "bind": "{{txn.date}}",
              "id": "e8",
              "label": "Date",
              "width": 60
            },
            {
              "align": "left",
              "bind": "{{txn.code}}",
              "id": "e9",
              "label": "Code",
              "width": 40
            },
            {
              "align": "left",
              "bind": "{{txn.description}}",
              "id": "ea",
              "label": "Description",
              "width": 243
            },
            {
              "align": "right",
              "bind": "{{formatNumber(txn.amount, \"#,##0.00\")}}",
              "id": "eb",
              "label": "Amount",
              "width": 90
            },
            {
              "align": "right",
              "bind": "{{formatNumber(txn.balance, \"#,##0.00\")}}",
              "id": "ec",
              "label": "Balance",
              "width": 90
            }
          ],
          "headerHeight": 22,
          "id": "e7",
          "style": {
            "fontFamily": "body",
            "fontSize": 7,
            "padding": {
              "bottom": 1,
              "left": 3,
              "right": 3,
              "top": 1
            }
          },
          "type": "table",
          "x": 0,
          "y": 38
        },
        {
          "height": 14,
          "id": "ed",
          "style": {
            "fontFamily": "body",
            "fontSize": 8
          },
          "type": "text",
          "value": "Codes: DEP deposit, WDL withdrawal, TRF transfer, FEE fee, INT interest",
          "width": 523,
          "x": 0,
          "y": 610
        }
      ],
      "sectionBreak": 600,
      "sectionBreakAnchor": false
    },
    {
      "elements": [
        {
          "height": 18,
          "id": "ee",
          "style": {
            "fontFamily": "body",
            "fontSize": 12
          },
          "type": "text",
          "value": "Service charges",
          "width": 523,
          "x": 0,
          "y": 0
        },
        {
          "as": "fee",
          "bind": "charges[]",
          "columns": [
            {
              "align": "left",
              "bind": "{{fee.date}}",
              "id": "eg",
              "label": "Date",
              "width": 60
            },
            {
              "align": "left",
              "bind": "{{fee.description}}",
              "id": "eh",
              "label": "Description",
              "width": 373
            },
            {
              "align": "right",
              "bind": "{{formatNumber(fee.amount, \"#,##0.00\")}}",
              "id": "ei",
              "label": "Amount",
              "width": 90
            }
          ],
          "headerHeight": 22,
          "id": "ef",
          "style": {
            "fontFamily": "body",
            "fontSize": 7,
            "padding": {
              "bottom": 1,
              "left": 3,
              "right": 3,
              "top": 1
            }
          },
          "type": "table",
          "x": 0,
          "y": 24
        },
        {
          "height": 14,
          "id": "ej",
          "style": {
            "fontFamily": "body",
            "fontSize": 8
          },
          "type": "text",
          "value": "Charges are debited on the last business day of the month.",
          "width": 523,
          "x": 0,
          "y": 410
        }
      ],
      "pageBreak": false,
      "sectionBreak": 400
    },
    {
      "elements": [
        {
          "height": 14,
          "id": "ek",
          "style": {
            "fontFamily": "body",
            "fontSize": 9
          },
          "type": "text",
          "value": "Acknowledged by the account holder",
          "width": 523,
          "x": 0,
          "y": 12
        },
        {
          "height": 1,
          "id": "el",
          "style": {
            "background": "#000000"
          },
          "type": "rect",
          "width": 200,
          "x": 0,
          "y": 60
        },
        {
          "height": 14,
          "id": "em",
          "style": {
            "fontFamily": "body",
            "fontSize": 9
          },
          "type": "text",
          "value": "Account holder signature",
          "width": 200,
          "x": 0,
          "y": 64
        }
      ],
      "pageBreak": false
    }
  ],
  "utcOffset": "+07:00",
  "version": "4.1"
}
`

// multiPageFlowDataJSON is fixtures/multi-page-flow/data.json.
const multiPageFlowDataJSON = `{"account":{"number":"123-4-56789-0"},"charges":[{"amount":50.00,"date":"2026-08-01","description":"Statement copy fee"},{"amount":100.00,"date":"2026-08-02","description":"Card replacement fee"},{"amount":25.00,"date":"2026-08-03","description":"Transfer fee"},{"amount":10.00,"date":"2026-08-04","description":"SMS alert fee"},{"amount":50.00,"date":"2026-08-05","description":"Statement copy fee"},{"amount":100.00,"date":"2026-08-06","description":"Card replacement fee"},{"amount":25.00,"date":"2026-08-07","description":"Transfer fee"},{"amount":10.00,"date":"2026-08-08","description":"SMS alert fee"},{"amount":50.00,"date":"2026-08-09","description":"Statement copy fee"},{"amount":100.00,"date":"2026-08-10","description":"Card replacement fee"},{"amount":25.00,"date":"2026-08-11","description":"Transfer fee"},{"amount":10.00,"date":"2026-08-12","description":"SMS alert fee"},{"amount":50.00,"date":"2026-08-13","description":"Statement copy fee"},{"amount":100.00,"date":"2026-08-14","description":"Card replacement fee"},{"amount":25.00,"date":"2026-08-15","description":"Transfer fee"},{"amount":10.00,"date":"2026-08-16","description":"SMS alert fee"},{"amount":50.00,"date":"2026-08-17","description":"Statement copy fee"},{"amount":100.00,"date":"2026-08-18","description":"Card replacement fee"},{"amount":25.00,"date":"2026-08-19","description":"Transfer fee"},{"amount":10.00,"date":"2026-08-20","description":"SMS alert fee"},{"amount":50.00,"date":"2026-08-21","description":"Statement copy fee"},{"amount":100.00,"date":"2026-08-22","description":"Card replacement fee"},{"amount":25.00,"date":"2026-08-23","description":"Transfer fee"},{"amount":10.00,"date":"2026-08-24","description":"SMS alert fee"},{"amount":50.00,"date":"2026-08-25","description":"Statement copy fee"},{"amount":100.00,"date":"2026-08-26","description":"Card replacement fee"},{"amount":25.00,"date":"2026-08-27","description":"Transfer fee"},{"amount":10.00,"date":"2026-08-28","description":"SMS alert fee"},{"amount":50.00,"date":"2026-08-01","description":"Statement copy fee"},{"amount":100.00,"date":"2026-08-02","description":"Card replacement fee"},{"amount":25.00,"date":"2026-08-03","description":"Transfer fee"},{"amount":10.00,"date":"2026-08-04","description":"SMS alert fee"},{"amount":50.00,"date":"2026-08-05","description":"Statement copy fee"},{"amount":100.00,"date":"2026-08-06","description":"Card replacement fee"},{"amount":25.00,"date":"2026-08-07","description":"Transfer fee"},{"amount":10.00,"date":"2026-08-08","description":"SMS alert fee"},{"amount":50.00,"date":"2026-08-09","description":"Statement copy fee"},{"amount":100.00,"date":"2026-08-10","description":"Card replacement fee"},{"amount":25.00,"date":"2026-08-11","description":"Transfer fee"},{"amount":10.00,"date":"2026-08-12","description":"SMS alert fee"}],"customer":{"name":"Somchai Example"},"transactions":[{"amount":45000.00,"balance":55000.00,"code":"DEP","date":"2026-08-01","description":"Salary deposit"},{"amount":-2000.00,"balance":53000.00,"code":"WDL","date":"2026-08-02","description":"Cash withdrawal"},{"amount":-1500.00,"balance":51500.00,"code":"TRF","date":"2026-08-03","description":"Transfer to savings"},{"amount":-20.00,"balance":51480.00,"code":"FEE","date":"2026-08-04","description":"Monthly service fee"},{"amount":12.34,"balance":51492.34,"code":"INT","date":"2026-08-05","description":"Deposit interest"},{"amount":45000.00,"balance":96492.34,"code":"DEP","date":"2026-08-06","description":"Salary deposit"},{"amount":-2000.00,"balance":94492.34,"code":"WDL","date":"2026-08-07","description":"Cash withdrawal"},{"amount":-1500.00,"balance":92992.34,"code":"TRF","date":"2026-08-08","description":"Transfer to savings"},{"amount":-20.00,"balance":92972.34,"code":"FEE","date":"2026-08-09","description":"Monthly service fee"},{"amount":12.34,"balance":92984.68,"code":"INT","date":"2026-08-10","description":"Deposit interest"},{"amount":45000.00,"balance":137984.68,"code":"DEP","date":"2026-08-11","description":"Salary deposit"},{"amount":-2000.00,"balance":135984.68,"code":"WDL","date":"2026-08-12","description":"Cash withdrawal"},{"amount":-1500.00,"balance":134484.68,"code":"TRF","date":"2026-08-13","description":"Transfer to savings"},{"amount":-20.00,"balance":134464.68,"code":"FEE","date":"2026-08-14","description":"Monthly service fee"},{"amount":12.34,"balance":134477.02,"code":"INT","date":"2026-08-15","description":"Deposit interest"},{"amount":45000.00,"balance":179477.02,"code":"DEP","date":"2026-08-16","description":"Salary deposit"},{"amount":-2000.00,"balance":177477.02,"code":"WDL","date":"2026-08-17","description":"Cash withdrawal"},{"amount":-1500.00,"balance":175977.02,"code":"TRF","date":"2026-08-18","description":"Transfer to savings"},{"amount":-20.00,"balance":175957.02,"code":"FEE","date":"2026-08-19","description":"Monthly service fee"},{"amount":12.34,"balance":175969.36,"code":"INT","date":"2026-08-20","description":"Deposit interest"},{"amount":45000.00,"balance":220969.36,"code":"DEP","date":"2026-08-21","description":"Salary deposit"},{"amount":-2000.00,"balance":218969.36,"code":"WDL","date":"2026-08-22","description":"Cash withdrawal"},{"amount":-1500.00,"balance":217469.36,"code":"TRF","date":"2026-08-23","description":"Transfer to savings"},{"amount":-20.00,"balance":217449.36,"code":"FEE","date":"2026-08-24","description":"Monthly service fee"},{"amount":12.34,"balance":217461.70,"code":"INT","date":"2026-08-25","description":"Deposit interest"},{"amount":45000.00,"balance":262461.70,"code":"DEP","date":"2026-08-26","description":"Salary deposit"},{"amount":-2000.00,"balance":260461.70,"code":"WDL","date":"2026-08-27","description":"Cash withdrawal"},{"amount":-1500.00,"balance":258961.70,"code":"TRF","date":"2026-08-28","description":"Transfer to savings"},{"amount":-20.00,"balance":258941.70,"code":"FEE","date":"2026-08-01","description":"Monthly service fee"},{"amount":12.34,"balance":258954.04,"code":"INT","date":"2026-08-02","description":"Deposit interest"},{"amount":45000.00,"balance":303954.04,"code":"DEP","date":"2026-08-03","description":"Salary deposit"},{"amount":-2000.00,"balance":301954.04,"code":"WDL","date":"2026-08-04","description":"Cash withdrawal"},{"amount":-1500.00,"balance":300454.04,"code":"TRF","date":"2026-08-05","description":"Transfer to savings"},{"amount":-20.00,"balance":300434.04,"code":"FEE","date":"2026-08-06","description":"Monthly service fee"},{"amount":12.34,"balance":300446.38,"code":"INT","date":"2026-08-07","description":"Deposit interest"},{"amount":45000.00,"balance":345446.38,"code":"DEP","date":"2026-08-08","description":"Salary deposit"},{"amount":-2000.00,"balance":343446.38,"code":"WDL","date":"2026-08-09","description":"Cash withdrawal"},{"amount":-1500.00,"balance":341946.38,"code":"TRF","date":"2026-08-10","description":"Transfer to savings"},{"amount":-20.00,"balance":341926.38,"code":"FEE","date":"2026-08-11","description":"Monthly service fee"},{"amount":12.34,"balance":341938.72,"code":"INT","date":"2026-08-12","description":"Deposit interest"},{"amount":45000.00,"balance":386938.72,"code":"DEP","date":"2026-08-13","description":"Salary deposit"},{"amount":-2000.00,"balance":384938.72,"code":"WDL","date":"2026-08-14","description":"Cash withdrawal"},{"amount":-1500.00,"balance":383438.72,"code":"TRF","date":"2026-08-15","description":"Transfer to savings"},{"amount":-20.00,"balance":383418.72,"code":"FEE","date":"2026-08-16","description":"Monthly service fee"},{"amount":12.34,"balance":383431.06,"code":"INT","date":"2026-08-17","description":"Deposit interest"},{"amount":45000.00,"balance":428431.06,"code":"DEP","date":"2026-08-18","description":"Salary deposit"},{"amount":-2000.00,"balance":426431.06,"code":"WDL","date":"2026-08-19","description":"Cash withdrawal"},{"amount":-1500.00,"balance":424931.06,"code":"TRF","date":"2026-08-20","description":"Transfer to savings"},{"amount":-20.00,"balance":424911.06,"code":"FEE","date":"2026-08-21","description":"Monthly service fee"},{"amount":12.34,"balance":424923.40,"code":"INT","date":"2026-08-22","description":"Deposit interest"},{"amount":45000.00,"balance":469923.40,"code":"DEP","date":"2026-08-23","description":"Salary deposit"},{"amount":-2000.00,"balance":467923.40,"code":"WDL","date":"2026-08-24","description":"Cash withdrawal"},{"amount":-1500.00,"balance":466423.40,"code":"TRF","date":"2026-08-25","description":"Transfer to savings"},{"amount":-20.00,"balance":466403.40,"code":"FEE","date":"2026-08-26","description":"Monthly service fee"},{"amount":12.34,"balance":466415.74,"code":"INT","date":"2026-08-27","description":"Deposit interest"},{"amount":45000.00,"balance":511415.74,"code":"DEP","date":"2026-08-28","description":"Salary deposit"},{"amount":-2000.00,"balance":509415.74,"code":"WDL","date":"2026-08-01","description":"Cash withdrawal"},{"amount":-1500.00,"balance":507915.74,"code":"TRF","date":"2026-08-02","description":"Transfer to savings"},{"amount":-20.00,"balance":507895.74,"code":"FEE","date":"2026-08-03","description":"Monthly service fee"},{"amount":12.34,"balance":507908.08,"code":"INT","date":"2026-08-04","description":"Deposit interest"}]}
`
