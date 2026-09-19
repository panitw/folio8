import { describe, expect, it } from 'vitest'
import { MAX_ENGINE_PAYLOAD_BYTES } from './engine-protocol'
import { acceptSampleData, SampleDataError } from './sample-data'

const encode = (value: string) => new TextEncoder().encode(value).buffer

describe('bounded local sample discovery', () => {
  it('retains exact accepted bytes while projecting ordered nested paths without numeric coercion', () => {
    const raw = '{"customer":{"name":"<img src=x onerror=1>","active":true,"none":null},"transactions":[{"amount":12345678901234567890.123456789,"id":1}],"tags":["first","second"],"empty":{}}'
    const accepted = acceptSampleData('statement.json', encode(raw))
    expect(new TextDecoder().decode(accepted.bytes)).toBe(raw)
    expect(accepted.tree.children.map((node) => node.label)).toEqual(['customer', 'transactions[]', 'tags[]', 'empty'])
    const customer = accepted.tree.children[0]!
    expect(customer.children.map((node) => [node.path, node.kind, node.preview])).toEqual([
      ['$ › "customer" › "name"', 'string', '"<img src=x onerror=1>"'], ['$ › "customer" › "active"', 'boolean', 'true'], ['$ › "customer" › "none"', 'null', 'null'],
    ])
    const amount = accepted.tree.children[1]!.children[0]!.children.find((node) => node.label === 'amount')
    expect(amount?.preview).toBe('12345678901234567890.123456789')
  })

  it('rejects malformed, trailing, unreadable-size, and invalid UTF-8 input before replacement', () => {
    expect(() => acceptSampleData('bad.json', encode('{"a":}'))).toThrow(SampleDataError)
    expect(() => acceptSampleData('trailing.json', encode('{} trailing'))).toThrow('one valid JSON document')
    expect(() => acceptSampleData('huge.json', new ArrayBuffer(MAX_ENGINE_PAYLOAD_BYTES + 1))).toThrow('local preview limit')
    expect(() => acceptSampleData('bad-utf8.json', new Uint8Array([0xff]).buffer)).toThrow('valid UTF-8 JSON')
    expect(() => acceptSampleData('bom.json', new Uint8Array([0xef, 0xbb, 0xbf, 0x7b, 0x7d]).buffer)).toThrow('must not start with a UTF-8 BOM')
  })

  it('keeps every display token bounded and special-key paths unambiguous', () => {
    const key = 'a'.repeat(200)
    const number = `1${'2'.repeat(200)}`
    const accepted = acceptSampleData('bounded.json', encode(`{"a.b":1,"a":{"b":2},"${key}":${number},"[]":3,"":4}`))
    expect(accepted.tree.children.map((child) => child.path)).toEqual([
      '$ › "a.b"', '$ › "a"', `$ › "${'a'.repeat(120)}…"`, '$ › "[]"', '$ › ""',
    ])
    expect(accepted.tree.children[2]!.preview).toHaveLength(121)
    expect(accepted.tree.children[0]!.path).not.toBe(accepted.tree.children[1]!.children[0]!.path)
		// The command carries these decoded segments, not the display path. Go
		// rejects the first key because folio8 identifiers cannot represent dots,
		// rather than reinterpreting it as the second path.
		expect(accepted.tree.children[0]!.segments).toEqual(['a.b'])
		expect(accepted.tree.children[1]!.children[0]!.segments).toEqual(['a', 'b'])
    const nested = acceptSampleData('paths.json', encode(`{"${'x'.repeat(120)}":{"${'y'.repeat(120)}":{"${'z'.repeat(120)}":1}}}`))
    expect(nested.tree.children[0]!.children[0]!.children[0]!.path).toHaveLength(257)
  })

  it('bounds deep and wide inspection while retaining a valid raw input', () => {
    let deep = 'null'
    for (let index = 0; index < 30; index++) deep = `{ "level${index}": ${deep} }`
    const wide = `{${Array.from({ length: 70 }, (_, index) => `"key${index}":${index}`).join(',')}}`
    expect(acceptSampleData('deep.json', encode(deep)).truncated).toBe(true)
    expect(acceptSampleData('wide.json', encode(wide)).tree.children).toHaveLength(50)
  })

  it('retains root collection segments for empty and populated arrays but never row arrays or truncated ancestor keys', () => {
    const longKey = 'x'.repeat(121)
    const accepted = acceptSampleData('collections.json', encode(JSON.stringify({ empty: [], report: { transactions: [{ amount: 1, children: [{ ref: 2 }] }] }, [longKey]: { hidden: [{ id: 3 }], empty: [] } })))
    const [empty, report, truncated] = accepted.tree.children
    expect(empty?.segments).toEqual(['empty'])
    expect(report?.children[0]?.segments).toEqual(['report', 'transactions'])
    const row = report?.children[0]?.children[0]
    expect(row?.children.find((node) => node.label === 'amount')?.segments).toBeUndefined()
    expect(row?.children.find((node) => node.kind === 'collection')?.segments).toBeUndefined()
    expect(truncated?.children.map((node) => node.segments)).toEqual([undefined, undefined])
    const rootArray = acceptSampleData('root.json', encode('[{"nested":[]}]')).tree
    expect(rootArray.segments).toBeUndefined()
    expect(rootArray.children[0]?.children[0]?.segments).toBeUndefined()
  })
})
