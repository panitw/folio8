// A deliberately tiny, reusable FIFO drain. The worker owns the queue; this
// module keeps the ordering property directly unit-testable without creating
// a second Worker or a second engine authority.
export class EngineWorkerQueue<T> {
  #items: T[] = []
  #draining = false
  private readonly execute: (item: T) => Promise<void>

  constructor(execute: (item: T) => Promise<void>) { this.execute = execute }

  enqueue(item: T): void {
    this.#items.push(item)
    void this.#drain()
  }

  async #drain(): Promise<void> {
    if (this.#draining) return
    this.#draining = true
    try {
      while (this.#items.length > 0) await this.execute(this.#items.shift()!)
    } finally {
      this.#draining = false
    }
  }
}
