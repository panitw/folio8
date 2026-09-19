import { describe, expect, it } from 'vitest'
import { addTableColumnCommand, configureTableBindingCommand, moveTableColumnCommand, removeTableColumnCommand, updateTableColumnBindingCommand, updateTableColumnCommand, updateTableColumnFooterCommand, updateTableColumnExpressionCommand, tableColumnBindingSuggestion } from './table-column-command'

const decode = (command: ArrayBuffer): Record<string, unknown> => JSON.parse(new TextDecoder().decode(command)) as Record<string, unknown>

describe('table-column command bytes', () => {
  it('uses complete JSON string encoding while retaining the closed envelope order', () => {
    const control = 'A\u0000\b\f\n\r\t\\/" สวัสดี'
    expect(decode(updateTableColumnCommand('e7', 'e8', 'header', control))).toEqual({ kind: 'updateTableColumn', version: 1, id: 'e7', columnId: 'e8', field: 'header', value: control })
    expect(new TextDecoder().decode(addTableColumnCommand('e7', 1))).toBe('{"kind":"addTableColumn","version":1,"id":"e7","index":1}')
    expect(decode(removeTableColumnCommand('e7', 'e8'))).toMatchObject({ kind: 'removeTableColumn', id: 'e7', columnId: 'e8' })
    expect(decode(moveTableColumnCommand('e7', 'e8', 0))).toMatchObject({ kind: 'moveTableColumn', id: 'e7', columnId: 'e8', toIndex: 0 })
    expect(decode(configureTableBindingCommand('e7', 'transactions[]', 'transaction'))).toEqual({ kind: 'configureTableBinding', version: 1, id: 'e7', collection: 'transactions[]', alias: 'transaction' })
    expect(decode(updateTableColumnBindingCommand('e7', 'e8', 'amount'))).toEqual({ kind: 'updateTableColumnBinding', version: 1, id: 'e7', columnId: 'e8', field: 'amount' })
    expect(decode(updateTableColumnFooterCommand('e7', 'e8', 'sum', 'transactions.amount', '#,##0.00'))).toEqual({ kind: 'updateTableColumnFooter', version: 1, id: 'e7', columnId: 'e8', footer: 'sum', footerOf: 'transactions.amount', footerFormat: '#,##0.00' })
  })

  it('does not construct an invalid JSON number for non-finite local input', () => {
    expect(decode(updateTableColumnCommand('e7', 'e8', 'width', Number.NaN)).value).toBeNull()
  })

  it('passes relative fields, escaped input, and clears to Go without constructing expressions', () => {
    for (const field of ['customer.name', '', 'bad"\\\npath']) {
      expect(decode(updateTableColumnBindingCommand('e7', 'e8', field))).toEqual({ kind: 'updateTableColumnBinding', version: 1, id: 'e7', columnId: 'e8', field })
    }
  })
})

describe('complete table column expression commands and sample suggestions', () => {
  it('passes complete formulas, interpolation, line endings, literals and clears unchanged', () => {
    for (const binding of ['{{upper(row.trn_code)}}', '{{formatNumber(row.amount * 1.07, "#,##0.00")}}', ' Code: {{row.code}}\r\n', 'literal', '', 'bad"\\\nformula']) {
      expect(decode(updateTableColumnExpressionCommand('e7', 'c1', binding))).toEqual({ kind: 'updateTableColumnExpression', version: 1, id: 'e7', columnId: 'c1', binding })
    }
  })

  it('formats only safe projected paths within the binding bound for sample presentation', () => {
    expect(tableColumnBindingSuggestion('txn', 'customer.name')).toBe('{{txn.customer.name}}')
    expect(tableColumnBindingSuggestion('row', 'x'.repeat(248))).toHaveLength(256)
    for (const field of ['', 'x'.repeat(249), 'customer..name', 'customer[0]', 'bad field', 'é', '{{row.date}}']) {
      expect(tableColumnBindingSuggestion('row', field)).toBeUndefined()
    }
    expect(tableColumnBindingSuggestion('bad.alias', 'date')).toBeUndefined()
  })
})
