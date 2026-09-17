const canonicalBytes = new WeakMap<Template, Uint8Array>()
const constructionToken = Symbol('folio-js template')

/**
 * A parsed `.folio` template. Opaque: it holds the engine's canonical bytes
 * and offers no way to read or change the document. Obtain one from
 * `parseTemplate` or `loadTemplate`.
 */
export class Template {
  /** @internal */
  constructor(token: symbol, bytes: Uint8Array) {
    if (token !== constructionToken) throw new TypeError('Template cannot be constructed directly; use parseTemplate or loadTemplate')
    canonicalBytes.set(this, bytes)
    Object.freeze(this)
  }
}

export function newTemplate(bytes: Uint8Array): Template {
  return new Template(constructionToken, bytes)
}

/** The canonical bytes of a template this package created, or undefined. */
export function templateBytes(value: unknown): Uint8Array | undefined {
  return typeof value === 'object' && value !== null ? canonicalBytes.get(value as Template) : undefined
}
