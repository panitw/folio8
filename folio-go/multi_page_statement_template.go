package folio8

// multiPageStatementTemplateJSON is fixtures/multi-page-statement/input.folio,
// kept byte-identical to it by TestMultiPageStatementGoldenFixture.
//
// It is SPEC-multi-pages' golden for CAP-5: a canonical two-page document.
// Page 1 holds a synthetic statement whose transactions table runs three
// output pages; page 2 holds static terms and a signature line, with Page
// Break on, so it starts on output page 4. No real customer data.
const multiPageStatementTemplateJSON = `{
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
  "nextId": 18,
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
        }
      ]
    },
    {
      "elements": [
        {
          "height": 18,
          "id": "ed",
          "style": {
            "fontFamily": "body",
            "fontSize": 12
          },
          "type": "text",
          "value": "Terms and conditions",
          "width": 523,
          "x": 0,
          "y": 0
        },
        {
          "height": 14,
          "id": "ee",
          "style": {
            "fontFamily": "body",
            "fontSize": 9
          },
          "type": "text",
          "value": "1. This statement is synthetic sample data made for testing.",
          "width": 523,
          "x": 0,
          "y": 24
        },
        {
          "height": 14,
          "id": "ef",
          "style": {
            "fontFamily": "body",
            "fontSize": 9
          },
          "type": "text",
          "value": "2. Amounts are shown in Thai baht.",
          "width": 523,
          "x": 0,
          "y": 40
        },
        {
          "height": 1,
          "id": "eg",
          "style": {
            "background": "#000000"
          },
          "type": "rect",
          "width": 200,
          "x": 0,
          "y": 110
        },
        {
          "height": 14,
          "id": "eh",
          "style": {
            "fontFamily": "body",
            "fontSize": 9
          },
          "type": "text",
          "value": "Account holder signature",
          "width": 200,
          "x": 0,
          "y": 114
        }
      ],
      "pageBreak": true
    }
  ],
  "utcOffset": "+07:00",
  "version": "4.1"
}
`

// multiPageStatementDataJSON is fixtures/multi-page-statement/data.json.
const multiPageStatementDataJSON = `{"account":{"number":"123-4-56789-0"},"customer":{"name":"Somchai Example"},"transactions":[{"amount":45000.00,"balance":55000.00,"code":"DEP","date":"2026-08-01","description":"Salary deposit"},{"amount":-2000.00,"balance":53000.00,"code":"WDL","date":"2026-08-02","description":"Cash withdrawal"},{"amount":-1500.00,"balance":51500.00,"code":"TRF","date":"2026-08-03","description":"Transfer to savings"},{"amount":-20.00,"balance":51480.00,"code":"FEE","date":"2026-08-04","description":"Monthly service fee"},{"amount":12.34,"balance":51492.34,"code":"INT","date":"2026-08-05","description":"Deposit interest"},{"amount":45000.00,"balance":96492.34,"code":"DEP","date":"2026-08-06","description":"Salary deposit"},{"amount":-2000.00,"balance":94492.34,"code":"WDL","date":"2026-08-07","description":"Cash withdrawal"},{"amount":-1500.00,"balance":92992.34,"code":"TRF","date":"2026-08-08","description":"Transfer to savings"},{"amount":-20.00,"balance":92972.34,"code":"FEE","date":"2026-08-09","description":"Monthly service fee"},{"amount":12.34,"balance":92984.68,"code":"INT","date":"2026-08-10","description":"Deposit interest"},{"amount":45000.00,"balance":137984.68,"code":"DEP","date":"2026-08-11","description":"Salary deposit"},{"amount":-2000.00,"balance":135984.68,"code":"WDL","date":"2026-08-12","description":"Cash withdrawal"},{"amount":-1500.00,"balance":134484.68,"code":"TRF","date":"2026-08-13","description":"Transfer to savings"},{"amount":-20.00,"balance":134464.68,"code":"FEE","date":"2026-08-14","description":"Monthly service fee"},{"amount":12.34,"balance":134477.02,"code":"INT","date":"2026-08-15","description":"Deposit interest"},{"amount":45000.00,"balance":179477.02,"code":"DEP","date":"2026-08-16","description":"Salary deposit"},{"amount":-2000.00,"balance":177477.02,"code":"WDL","date":"2026-08-17","description":"Cash withdrawal"},{"amount":-1500.00,"balance":175977.02,"code":"TRF","date":"2026-08-18","description":"Transfer to savings"},{"amount":-20.00,"balance":175957.02,"code":"FEE","date":"2026-08-19","description":"Monthly service fee"},{"amount":12.34,"balance":175969.36,"code":"INT","date":"2026-08-20","description":"Deposit interest"},{"amount":45000.00,"balance":220969.36,"code":"DEP","date":"2026-08-21","description":"Salary deposit"},{"amount":-2000.00,"balance":218969.36,"code":"WDL","date":"2026-08-22","description":"Cash withdrawal"},{"amount":-1500.00,"balance":217469.36,"code":"TRF","date":"2026-08-23","description":"Transfer to savings"},{"amount":-20.00,"balance":217449.36,"code":"FEE","date":"2026-08-24","description":"Monthly service fee"},{"amount":12.34,"balance":217461.70,"code":"INT","date":"2026-08-25","description":"Deposit interest"},{"amount":45000.00,"balance":262461.70,"code":"DEP","date":"2026-08-26","description":"Salary deposit"},{"amount":-2000.00,"balance":260461.70,"code":"WDL","date":"2026-08-27","description":"Cash withdrawal"},{"amount":-1500.00,"balance":258961.70,"code":"TRF","date":"2026-08-28","description":"Transfer to savings"},{"amount":-20.00,"balance":258941.70,"code":"FEE","date":"2026-08-01","description":"Monthly service fee"},{"amount":12.34,"balance":258954.04,"code":"INT","date":"2026-08-02","description":"Deposit interest"},{"amount":45000.00,"balance":303954.04,"code":"DEP","date":"2026-08-03","description":"Salary deposit"},{"amount":-2000.00,"balance":301954.04,"code":"WDL","date":"2026-08-04","description":"Cash withdrawal"},{"amount":-1500.00,"balance":300454.04,"code":"TRF","date":"2026-08-05","description":"Transfer to savings"},{"amount":-20.00,"balance":300434.04,"code":"FEE","date":"2026-08-06","description":"Monthly service fee"},{"amount":12.34,"balance":300446.38,"code":"INT","date":"2026-08-07","description":"Deposit interest"},{"amount":45000.00,"balance":345446.38,"code":"DEP","date":"2026-08-08","description":"Salary deposit"},{"amount":-2000.00,"balance":343446.38,"code":"WDL","date":"2026-08-09","description":"Cash withdrawal"},{"amount":-1500.00,"balance":341946.38,"code":"TRF","date":"2026-08-10","description":"Transfer to savings"},{"amount":-20.00,"balance":341926.38,"code":"FEE","date":"2026-08-11","description":"Monthly service fee"},{"amount":12.34,"balance":341938.72,"code":"INT","date":"2026-08-12","description":"Deposit interest"},{"amount":45000.00,"balance":386938.72,"code":"DEP","date":"2026-08-13","description":"Salary deposit"},{"amount":-2000.00,"balance":384938.72,"code":"WDL","date":"2026-08-14","description":"Cash withdrawal"},{"amount":-1500.00,"balance":383438.72,"code":"TRF","date":"2026-08-15","description":"Transfer to savings"},{"amount":-20.00,"balance":383418.72,"code":"FEE","date":"2026-08-16","description":"Monthly service fee"},{"amount":12.34,"balance":383431.06,"code":"INT","date":"2026-08-17","description":"Deposit interest"},{"amount":45000.00,"balance":428431.06,"code":"DEP","date":"2026-08-18","description":"Salary deposit"},{"amount":-2000.00,"balance":426431.06,"code":"WDL","date":"2026-08-19","description":"Cash withdrawal"},{"amount":-1500.00,"balance":424931.06,"code":"TRF","date":"2026-08-20","description":"Transfer to savings"},{"amount":-20.00,"balance":424911.06,"code":"FEE","date":"2026-08-21","description":"Monthly service fee"},{"amount":12.34,"balance":424923.40,"code":"INT","date":"2026-08-22","description":"Deposit interest"},{"amount":45000.00,"balance":469923.40,"code":"DEP","date":"2026-08-23","description":"Salary deposit"},{"amount":-2000.00,"balance":467923.40,"code":"WDL","date":"2026-08-24","description":"Cash withdrawal"},{"amount":-1500.00,"balance":466423.40,"code":"TRF","date":"2026-08-25","description":"Transfer to savings"},{"amount":-20.00,"balance":466403.40,"code":"FEE","date":"2026-08-26","description":"Monthly service fee"},{"amount":12.34,"balance":466415.74,"code":"INT","date":"2026-08-27","description":"Deposit interest"},{"amount":45000.00,"balance":511415.74,"code":"DEP","date":"2026-08-28","description":"Salary deposit"},{"amount":-2000.00,"balance":509415.74,"code":"WDL","date":"2026-08-01","description":"Cash withdrawal"},{"amount":-1500.00,"balance":507915.74,"code":"TRF","date":"2026-08-02","description":"Transfer to savings"},{"amount":-20.00,"balance":507895.74,"code":"FEE","date":"2026-08-03","description":"Monthly service fee"},{"amount":12.34,"balance":507908.08,"code":"INT","date":"2026-08-04","description":"Deposit interest"},{"amount":45000.00,"balance":552908.08,"code":"DEP","date":"2026-08-05","description":"Salary deposit"},{"amount":-2000.00,"balance":550908.08,"code":"WDL","date":"2026-08-06","description":"Cash withdrawal"},{"amount":-1500.00,"balance":549408.08,"code":"TRF","date":"2026-08-07","description":"Transfer to savings"},{"amount":-20.00,"balance":549388.08,"code":"FEE","date":"2026-08-08","description":"Monthly service fee"},{"amount":12.34,"balance":549400.42,"code":"INT","date":"2026-08-09","description":"Deposit interest"},{"amount":45000.00,"balance":594400.42,"code":"DEP","date":"2026-08-10","description":"Salary deposit"},{"amount":-2000.00,"balance":592400.42,"code":"WDL","date":"2026-08-11","description":"Cash withdrawal"},{"amount":-1500.00,"balance":590900.42,"code":"TRF","date":"2026-08-12","description":"Transfer to savings"},{"amount":-20.00,"balance":590880.42,"code":"FEE","date":"2026-08-13","description":"Monthly service fee"},{"amount":12.34,"balance":590892.76,"code":"INT","date":"2026-08-14","description":"Deposit interest"},{"amount":45000.00,"balance":635892.76,"code":"DEP","date":"2026-08-15","description":"Salary deposit"},{"amount":-2000.00,"balance":633892.76,"code":"WDL","date":"2026-08-16","description":"Cash withdrawal"},{"amount":-1500.00,"balance":632392.76,"code":"TRF","date":"2026-08-17","description":"Transfer to savings"},{"amount":-20.00,"balance":632372.76,"code":"FEE","date":"2026-08-18","description":"Monthly service fee"},{"amount":12.34,"balance":632385.10,"code":"INT","date":"2026-08-19","description":"Deposit interest"},{"amount":45000.00,"balance":677385.10,"code":"DEP","date":"2026-08-20","description":"Salary deposit"},{"amount":-2000.00,"balance":675385.10,"code":"WDL","date":"2026-08-21","description":"Cash withdrawal"},{"amount":-1500.00,"balance":673885.10,"code":"TRF","date":"2026-08-22","description":"Transfer to savings"},{"amount":-20.00,"balance":673865.10,"code":"FEE","date":"2026-08-23","description":"Monthly service fee"},{"amount":12.34,"balance":673877.44,"code":"INT","date":"2026-08-24","description":"Deposit interest"},{"amount":45000.00,"balance":718877.44,"code":"DEP","date":"2026-08-25","description":"Salary deposit"},{"amount":-2000.00,"balance":716877.44,"code":"WDL","date":"2026-08-26","description":"Cash withdrawal"},{"amount":-1500.00,"balance":715377.44,"code":"TRF","date":"2026-08-27","description":"Transfer to savings"},{"amount":-20.00,"balance":715357.44,"code":"FEE","date":"2026-08-28","description":"Monthly service fee"},{"amount":12.34,"balance":715369.78,"code":"INT","date":"2026-08-01","description":"Deposit interest"},{"amount":45000.00,"balance":760369.78,"code":"DEP","date":"2026-08-02","description":"Salary deposit"},{"amount":-2000.00,"balance":758369.78,"code":"WDL","date":"2026-08-03","description":"Cash withdrawal"},{"amount":-1500.00,"balance":756869.78,"code":"TRF","date":"2026-08-04","description":"Transfer to savings"},{"amount":-20.00,"balance":756849.78,"code":"FEE","date":"2026-08-05","description":"Monthly service fee"},{"amount":12.34,"balance":756862.12,"code":"INT","date":"2026-08-06","description":"Deposit interest"},{"amount":45000.00,"balance":801862.12,"code":"DEP","date":"2026-08-07","description":"Salary deposit"},{"amount":-2000.00,"balance":799862.12,"code":"WDL","date":"2026-08-08","description":"Cash withdrawal"},{"amount":-1500.00,"balance":798362.12,"code":"TRF","date":"2026-08-09","description":"Transfer to savings"},{"amount":-20.00,"balance":798342.12,"code":"FEE","date":"2026-08-10","description":"Monthly service fee"},{"amount":12.34,"balance":798354.46,"code":"INT","date":"2026-08-11","description":"Deposit interest"},{"amount":45000.00,"balance":843354.46,"code":"DEP","date":"2026-08-12","description":"Salary deposit"},{"amount":-2000.00,"balance":841354.46,"code":"WDL","date":"2026-08-13","description":"Cash withdrawal"},{"amount":-1500.00,"balance":839854.46,"code":"TRF","date":"2026-08-14","description":"Transfer to savings"},{"amount":-20.00,"balance":839834.46,"code":"FEE","date":"2026-08-15","description":"Monthly service fee"},{"amount":12.34,"balance":839846.80,"code":"INT","date":"2026-08-16","description":"Deposit interest"},{"amount":45000.00,"balance":884846.80,"code":"DEP","date":"2026-08-17","description":"Salary deposit"},{"amount":-2000.00,"balance":882846.80,"code":"WDL","date":"2026-08-18","description":"Cash withdrawal"},{"amount":-1500.00,"balance":881346.80,"code":"TRF","date":"2026-08-19","description":"Transfer to savings"},{"amount":-20.00,"balance":881326.80,"code":"FEE","date":"2026-08-20","description":"Monthly service fee"},{"amount":12.34,"balance":881339.14,"code":"INT","date":"2026-08-21","description":"Deposit interest"},{"amount":45000.00,"balance":926339.14,"code":"DEP","date":"2026-08-22","description":"Salary deposit"},{"amount":-2000.00,"balance":924339.14,"code":"WDL","date":"2026-08-23","description":"Cash withdrawal"},{"amount":-1500.00,"balance":922839.14,"code":"TRF","date":"2026-08-24","description":"Transfer to savings"},{"amount":-20.00,"balance":922819.14,"code":"FEE","date":"2026-08-25","description":"Monthly service fee"},{"amount":12.34,"balance":922831.48,"code":"INT","date":"2026-08-26","description":"Deposit interest"},{"amount":45000.00,"balance":967831.48,"code":"DEP","date":"2026-08-27","description":"Salary deposit"},{"amount":-2000.00,"balance":965831.48,"code":"WDL","date":"2026-08-28","description":"Cash withdrawal"},{"amount":-1500.00,"balance":964331.48,"code":"TRF","date":"2026-08-01","description":"Transfer to savings"},{"amount":-20.00,"balance":964311.48,"code":"FEE","date":"2026-08-02","description":"Monthly service fee"},{"amount":12.34,"balance":964323.82,"code":"INT","date":"2026-08-03","description":"Deposit interest"},{"amount":45000.00,"balance":1009323.82,"code":"DEP","date":"2026-08-04","description":"Salary deposit"},{"amount":-2000.00,"balance":1007323.82,"code":"WDL","date":"2026-08-05","description":"Cash withdrawal"},{"amount":-1500.00,"balance":1005823.82,"code":"TRF","date":"2026-08-06","description":"Transfer to savings"},{"amount":-20.00,"balance":1005803.82,"code":"FEE","date":"2026-08-07","description":"Monthly service fee"},{"amount":12.34,"balance":1005816.16,"code":"INT","date":"2026-08-08","description":"Deposit interest"},{"amount":45000.00,"balance":1050816.16,"code":"DEP","date":"2026-08-09","description":"Salary deposit"},{"amount":-2000.00,"balance":1048816.16,"code":"WDL","date":"2026-08-10","description":"Cash withdrawal"},{"amount":-1500.00,"balance":1047316.16,"code":"TRF","date":"2026-08-11","description":"Transfer to savings"},{"amount":-20.00,"balance":1047296.16,"code":"FEE","date":"2026-08-12","description":"Monthly service fee"},{"amount":12.34,"balance":1047308.50,"code":"INT","date":"2026-08-13","description":"Deposit interest"},{"amount":45000.00,"balance":1092308.50,"code":"DEP","date":"2026-08-14","description":"Salary deposit"},{"amount":-2000.00,"balance":1090308.50,"code":"WDL","date":"2026-08-15","description":"Cash withdrawal"},{"amount":-1500.00,"balance":1088808.50,"code":"TRF","date":"2026-08-16","description":"Transfer to savings"},{"amount":-20.00,"balance":1088788.50,"code":"FEE","date":"2026-08-17","description":"Monthly service fee"},{"amount":12.34,"balance":1088800.84,"code":"INT","date":"2026-08-18","description":"Deposit interest"}]}
`
