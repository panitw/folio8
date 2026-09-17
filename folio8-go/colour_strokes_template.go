package folio8

// colourStrokesTemplateJSON is fixtures/colour-strokes/input.folio, kept
// byte-identical to it by TestColourStrokesGoldenFixture.
//
// It is the golden DW-147 asked for, landed before the folio8-go/v1.0.0 tag
// (SPEC-client-libraries story 3): until it, no committed fixture declared a
// text colour or a stroke colour, so the four-target byte-identity check had
// never rendered coloured ink or coloured strokes. One A4 page declares,
// every colour differing from #000000:
//
//   - e1, a heading: style.color ink on a filled style.background, with a
//     style.border stroked in colour on its bottom and left edges only;
//   - e2, a line of text in its own ink, naming the colours above;
//   - e3, a rect and e4, a line, each stroked in a colour;
//   - e5, a proportional table: style.color in its cells, a coloured frame
//     border, a headerStyle with its own ink and fill, rules between columns
//     and rows in a colour, and altRowBackground on odd collection indexes.
//
// Declaring `rules` makes it a 3.1 document. The data is synthetic.
const colourStrokesTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {
          "height": 32,
          "id": "e1",
          "style": {
            "background": "#FFF4D6",
            "bold": true,
            "border": {
              "color": "#C81E1E",
              "edges": [
                "bottom",
                "left"
              ],
              "width": 2
            },
            "color": "#1B2A4A",
            "fontFamily": "body",
            "fontSize": 20
          },
          "type": "text",
          "value": "Colour and strokes",
          "width": 523,
          "x": 0,
          "y": 0
        },
        {
          "height": 16,
          "id": "e2",
          "style": {
            "color": "#6A1B9A",
            "fontFamily": "body",
            "fontSize": 10
          },
          "type": "text",
          "value": "Above: #1B2A4A ink on a #FFF4D6 fill, stroked #C81E1E on the bottom and left edges only. This line: #6A1B9A ink.",
          "width": 523,
          "x": 0,
          "y": 44
        },
        {
          "height": 60,
          "id": "e3",
          "style": {
            "border": {
              "color": "#2E7D32",
              "width": 3
            }
          },
          "type": "rect",
          "width": 160,
          "x": 0,
          "y": 72
        },
        {
          "height": 1,
          "id": "e4",
          "style": {
            "border": {
              "color": "#1565C0",
              "edges": [
                "top"
              ],
              "width": 2
            }
          },
          "type": "line",
          "width": 343,
          "x": 180,
          "y": 102
        },
        {
          "altRowBackground": "#E3F2FD",
          "as": "item",
          "bind": "items[]",
          "columns": [
            {
              "align": "left",
              "bind": "{{item.name}}",
              "id": "e6",
              "label": "Item",
              "proportion": 3
            },
            {
              "align": "left",
              "bind": "{{item.colour}}",
              "id": "e7",
              "label": "Colour",
              "proportion": 2
            },
            {
              "align": "right",
              "bind": "{{item.qty}}",
              "id": "e8",
              "label": "Qty",
              "proportion": 1
            }
          ],
          "headerHeight": 20,
          "headerStyle": {
            "background": "#1B2A4A",
            "bold": true,
            "color": "#FFFFFF"
          },
          "id": "e5",
          "rules": {
            "between": [
              "columns",
              "rows"
            ],
            "color": "#8E24AA",
            "width": 0.75
          },
          "style": {
            "border": {
              "color": "#00838F",
              "width": 1
            },
            "color": "#37474F",
            "fontFamily": "body",
            "fontSize": 10,
            "padding": {
              "bottom": 2,
              "left": 4,
              "right": 4,
              "top": 2
            }
          },
          "type": "table",
          "width": 523,
          "x": 0,
          "y": 150
        }
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 20
    },
    "pageHeader": {
      "elements": [],
      "height": 20
    }
  },
  "fonts": {
    "body": [
      {
        "bold": "Roboto Bold",
        "face": "Roboto"
      }
    ]
  },
  "locale": "en",
  "nextId": 9,
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
  "utcOffset": "+00:00",
  "version": "3.1"
}
`

// colourStrokesDataJSON is fixtures/colour-strokes/data.json.
const colourStrokesDataJSON = `{"items":[{"colour":"#1B2A4A ink","name":"Navy heading","qty":1},{"colour":"#C81E1E stroke","name":"Red rule","qty":2},{"colour":"#2E7D32 stroke","name":"Green box","qty":3},{"colour":"#1565C0 stroke","name":"Blue line","qty":4},{"colour":"#8E24AA rules","name":"Purple rules","qty":5},{"colour":"#E3F2FD fill","name":"Pale alternate rows","qty":6}]}
`
