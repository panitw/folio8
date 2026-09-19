import { describe, expect, it } from 'vitest'
import { EngineWorkerQueue } from './engine-worker-queue'

describe('engine worker FIFO queue', () => {
  it('does not start a later request until the earlier request settles', async () => {
    const observed: string[] = []
    let release!: () => void
    const first = new Promise<void>((resolve) => { release = resolve })
    const queue = new EngineWorkerQueue(async (value: string) => {
      observed.push(`start:${value}`)
      if (value === 'first') await first
      observed.push(`finish:${value}`)
    })
    queue.enqueue('first')
    queue.enqueue('second')
    await Promise.resolve()
    expect(observed).toEqual(['start:first']) // Red proof: concurrent drain starts second here.
    release()
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(observed).toEqual(['start:first', 'finish:first', 'start:second', 'finish:second'])
  })
})
