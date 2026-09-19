import { MAX_ENGINE_PAYLOAD_BYTES } from './engine-protocol'

export const SAMPLE_LIMITS = { depth: 12, nodes: 300, children: 50, items: 5, keyPreview: 120, stringPreview: 120, scalarPreview: 120, pathPreview: 256 } as const

export type SampleNodeKind = 'object' | 'collection' | 'string' | 'number' | 'boolean' | 'null' | 'truncated'
export type SampleNode = Readonly<{
  kind: SampleNodeKind
  // This inspection-only display path is deliberately not a future binding spelling.
  path: string
  label: string
  preview?: string
  count?: number
  children: ReadonlyArray<SampleNode>
  truncated?: boolean
  // Decoded keys are discovery data, never folio8 syntax. Root-addressable
  // scalar leaves and collections retain them; array rows and descendants of
  // truncated keys do not. Go admits the exact keys sent by either picker.
  segments?: ReadonlyArray<string>
}>
export type SampleData = Readonly<{ bytes: ArrayBuffer; name: string; tree: SampleNode; truncated: boolean }>
export class SampleDataError extends Error {}

export function acceptSampleData(name: string, bytes: ArrayBuffer): SampleData {
  if (bytes.byteLength === 0) throw new SampleDataError('Selected JSON file is empty')
  if (bytes.byteLength > MAX_ENGINE_PAYLOAD_BYTES) throw new SampleDataError(`Selected JSON exceeds the ${MAX_ENGINE_PAYLOAD_BYTES / 1024 / 1024} MiB local preview limit`)
  const raw = new Uint8Array(bytes)
  // Go's encoding/json rejects a BOM. The exact retained bytes must therefore
  // be rejected here too instead of being silently normalized by TextDecoder.
  if (raw.length >= 3 && raw[0] === 0xef && raw[1] === 0xbb && raw[2] === 0xbf) throw new SampleDataError('Selected JSON must not start with a UTF-8 BOM')
  let text: string
  try { text = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true }).decode(bytes) } catch { throw new SampleDataError('Selected file is not valid UTF-8 JSON') }
  try {
    // One bounded parser validates one whole document and creates the local
    // display projection; it never materializes a discarded JSON object graph.
    const parser = new DiscoveryParser(text)
    return { bytes: bytes.slice(0), name: bounded(name, SAMPLE_LIMITS.keyPreview), tree: parser.parse(), truncated: parser.truncated }
  } catch (error) {
    if (error instanceof SampleDataError) throw error
    throw new SampleDataError('Selected file is not one valid JSON document')
  }
}

class DiscoveryParser {
  private index = 0
  private nodeCount = 0
  truncated = false
  private readonly source: string
  constructor(source: string) { this.source = source }

  parse(): SampleNode {
    this.space()
    const node = this.value('$', '$', 0, true, [], true)
    this.space()
    if (this.index !== this.source.length) this.invalid()
    return node
  }

  private value(path: string, label: string, depth: number, project: boolean, segments: ReadonlyArray<string>, rootScoped: boolean): SampleNode {
    this.space()
    const visible = project && this.nodeCount < SAMPLE_LIMITS.nodes && depth <= SAMPLE_LIMITS.depth
    if (project && !visible) this.truncated = true
    if (visible) this.nodeCount++
    const token = this.source[this.index]
    if (token === '{') return this.object(path, label, depth, visible, segments, rootScoped)
    if (token === '[') return this.array(path, label, depth, visible, segments, rootScoped)
    const candidate = rootScoped && segments.length > 0 ? { segments } : {}
    if (token === '"') {
      const value = this.string(SAMPLE_LIMITS.stringPreview)
      return visible ? { kind: 'string', path, label, preview: JSON.stringify(value.text) + (value.truncated ? '…' : ''), children: [], ...candidate } : truncatedNode(path, label)
    }
    if (this.takeWord('true')) return visible ? { kind: 'boolean', path, label, preview: 'true', children: [], ...candidate } : truncatedNode(path, label)
    if (this.takeWord('false')) return visible ? { kind: 'boolean', path, label, preview: 'false', children: [], ...candidate } : truncatedNode(path, label)
    if (this.takeWord('null')) return visible ? { kind: 'null', path, label, preview: 'null', children: [], ...candidate } : truncatedNode(path, label)
    const number = this.number(SAMPLE_LIMITS.scalarPreview)
    return visible ? { kind: 'number', path, label, preview: number.text + (number.truncated ? '…' : ''), children: [], ...candidate } : truncatedNode(path, label)
  }

  private object(path: string, label: string, depth: number, visible: boolean, segments: ReadonlyArray<string>, rootScoped: boolean): SampleNode {
    this.expect('{'); this.space()
    const children: SampleNode[] = []; let count = 0; let limited = false
    if (this.take('}')) return visible ? { kind: 'object', path, label, count, children } : truncatedNode(path, label)
    while (true) {
      this.space(); const key = this.string(SAMPLE_LIMITS.keyPreview); this.space(); this.expect(':'); this.space(); count++
      const childVisible = visible && count <= SAMPLE_LIMITS.children
      if (!childVisible) { limited = true; this.truncated = true }
      const keyText = key.text + (key.truncated ? '…' : '')
      const childPath = this.path(`${path} › ${JSON.stringify(keyText)}`)
      const child = this.value(childPath.text, keyText, depth + 1, childVisible, key.truncated ? [] : [...segments, key.text], rootScoped && !key.truncated)
      if (childVisible) children.push(childPath.truncated ? { ...child, truncated: true } : child)
      this.space(); if (this.take('}')) break; this.expect(',')
    }
    return visible ? { kind: 'object', path, label, count, children, truncated: limited || undefined } : truncatedNode(path, label)
  }

  private array(path: string, label: string, depth: number, visible: boolean, segments: ReadonlyArray<string>, rootScoped: boolean): SampleNode {
    this.expect('['); this.space()
    const collection = this.path(`${path} › []`)
    const children: SampleNode[] = []; let count = 0; let limited = collection.truncated; const collectionPath = collection.text
    const candidate = rootScoped && segments.length > 0 ? { segments } : {}
    if (this.take(']')) return visible ? { kind: 'collection', path: collectionPath, label: `${label}[]`, count, children, ...candidate } : truncatedNode(path, label)
    while (true) {
      count++
      const childVisible = visible && count <= SAMPLE_LIMITS.items
      if (!childVisible) { limited = true; this.truncated = true }
      const child = this.value(collectionPath, `item ${count}`, depth + 1, childVisible, segments, false)
      if (childVisible) children.push(child)
      this.space(); if (this.take(']')) break; this.expect(',')
    }
    return visible ? { kind: 'collection', path: collectionPath, label: `${label}[]`, count, children, truncated: limited || undefined, ...candidate } : truncatedNode(path, label)
  }

  // Validate and decode only a bounded prefix so a huge value/key does not
  // become a full decoded browser string merely to supply an example.
  private string(limit: number): Readonly<{ text: string; truncated: boolean }> {
    this.expect('"'); let text = ''; let length = 0; let truncated = false
    const append = (value: string) => { length += value.length; if (text.length < limit) text += value.slice(0, limit - text.length); if (length > limit) truncated = true }
    while (this.index < this.source.length) {
      const char = this.source[this.index++]!
      if (char === '"') return { text, truncated }
      if (char < ' ') this.invalid()
      if (char !== '\\') { append(char); continue }
      const escape = this.source[this.index++]
      if (escape === undefined) this.invalid()
      const simple: Record<string, string> = { '"': '"', '\\': '\\', '/': '/', b: '\b', f: '\f', n: '\n', r: '\r', t: '\t' }
      if (escape === 'u') { const hex = this.source.slice(this.index, this.index + 4); if (!/^[0-9a-fA-F]{4}$/.test(hex)) this.invalid(); this.index += 4; append(String.fromCharCode(Number.parseInt(hex, 16))); continue }
      if (!(escape in simple)) this.invalid()
      append(simple[escape]!)
    }
    this.invalid()
  }

  private number(limit: number): Readonly<{ text: string; truncated: boolean }> {
    const start = this.index
    if (this.take('-') && !this.digit(this.source[this.index])) this.invalid()
    if (this.take('0')) { if (this.digit(this.source[this.index])) this.invalid() }
    else { if (!this.nonzero(this.source[this.index])) this.invalid(); while (this.digit(this.source[this.index])) this.index++ }
    if (this.take('.')) { if (!this.digit(this.source[this.index])) this.invalid(); while (this.digit(this.source[this.index])) this.index++ }
    if (this.take('e') || this.take('E')) { if (!this.take('+')) { this.take('-') }; if (!this.digit(this.source[this.index])) this.invalid(); while (this.digit(this.source[this.index])) this.index++ }
    return { text: this.source.slice(start, Math.min(this.index, start + limit)), truncated: this.index - start > limit }
  }

  private path(value: string): Readonly<{ text: string; truncated: boolean }> {
    const truncated = value.length > SAMPLE_LIMITS.pathPreview
    if (truncated) this.truncated = true
    return { text: bounded(value, SAMPLE_LIMITS.pathPreview), truncated }
  }

  private takeWord(word: string): boolean { if (!this.source.startsWith(word, this.index)) return false; this.index += word.length; return true }
  private space(): void { while (/\s/.test(this.source[this.index] ?? '')) this.index++ }
  private expect(char: string): void { if (!this.take(char)) this.invalid() }
  private take(char: string): boolean { if (this.source[this.index] !== char) return false; this.index++; return true }
  private digit(char: string | undefined): boolean { return char !== undefined && char >= '0' && char <= '9' }
  private nonzero(char: string | undefined): boolean { return char !== undefined && char >= '1' && char <= '9' }
  private invalid(): never { throw new SampleDataError('Selected file is not one valid JSON document') }
}

const truncatedNode = (path: string, label: string): SampleNode => ({ kind: 'truncated', path, label, preview: 'inspection limit reached', children: [], truncated: true })
const bounded = (value: string, limit: number): string => value.length <= limit ? value : `${value.slice(0, limit)}…`
