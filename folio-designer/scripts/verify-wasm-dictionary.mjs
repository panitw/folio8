import { webcrypto } from 'node:crypto'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { fileURLToPath } from 'node:url'

export async function emittedWasmDictionaryDigest(glueFile, wasmFile) {
  const cryptoDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'crypto')
  if (!globalThis.crypto) Object.defineProperty(globalThis, 'crypto', { configurable: true, value: webcrypto })
  try {
    vm.runInThisContext(readFileSync(glueFile, 'utf8'), { filename: glueFile })
    const go = new globalThis.Go()
    const { instance } = await WebAssembly.instantiate(readFileSync(wasmFile), go.importObject)
    go.run(instance)
    for (let attempts = 0; attempts < 100 && !globalThis.Folio8WasmHost; attempts++) await new Promise((resolve) => setTimeout(resolve, 1))
    if (!globalThis.Folio8WasmHost?.handle) throw new Error('emitted wasm did not register the folio8 host')
    const response = JSON.parse(globalThis.Folio8WasmHost.handle(JSON.stringify({ operation: 'offline-audit' })))
    if (!response.ok || !/^[a-f0-9]{64}$/.test(response.dictionarySha256)) throw new Error('emitted wasm returned no bounded dictionary witness')
    return response.dictionarySha256
  } finally {
    delete globalThis.Folio8WasmHost
    if (cryptoDescriptor) Object.defineProperty(globalThis, 'crypto', cryptoDescriptor)
  }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  emittedWasmDictionaryDigest(process.argv[2], process.argv[3]).then((digest) => {
    process.stdout.write(`${digest}\n`)
    process.exit(0)
  }).catch((error) => {
    process.stderr.write(`${error.message}\n`)
    process.exit(1)
  })
}
