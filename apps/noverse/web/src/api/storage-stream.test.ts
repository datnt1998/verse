import { describe, expect, it, vi } from 'vitest'
import { readStorageTest } from './storage-stream'

const step = { step: 'write', ok: true, ms: 12, code: 'ok', reason: '' }
const done = { done: true, ok: true, steps: [step], corsProbeUrl: 'https://store.example/probe' }
const streamOf = (...chunks: string[]) => new ReadableStream<Uint8Array>({ start(controller) { for (const chunk of chunks) controller.enqueue(new TextEncoder().encode(chunk)); controller.close() } })

describe('real storage test progress stream', () => {
  it('emits complete steps as they arrive and returns the final result', async () => {
    const progress = vi.fn()
    const result = await readStorageTest(streamOf(`${JSON.stringify(step)}\n`, `${JSON.stringify(done)}\n`), progress)
    expect(progress).toHaveBeenCalledExactlyOnceWith(step)
    expect(result).toMatchObject({ ok: true, steps: [step], corsProbeUrl: done.corsProbeUrl })
  })
  it('reassembles split JSON lines, multiple lines per chunk and a final line without newline', async () => {
    const progress = vi.fn()
    const line = JSON.stringify({ ...step, reason: 'Đã ghi tệp' })
    const result = await readStorageTest(streamOf(line.slice(0, 13), line.slice(13) + '\n' + JSON.stringify(done)), progress)
    expect(progress).toHaveBeenCalledExactlyOnceWith({ ...step, reason: 'Đã ghi tệp' })
    expect(result.ok).toBe(true)
  })
  it('rejects invalid or incomplete results instead of allowing save', async () => {
    await expect(readStorageTest(streamOf('{bad}\n'), vi.fn())).rejects.toThrow()
    await expect(readStorageTest(streamOf(JSON.stringify(step) + '\n'), vi.fn())).rejects.toThrow()
    await expect(readStorageTest(streamOf(JSON.stringify({ done: true, ok: true, steps: [{ step: 'write', ok: 'yes' }] })), vi.fn())).rejects.toThrow()
    await expect(readStorageTest(streamOf(JSON.stringify({ error: 'Kiểm tra đã dừng' })), vi.fn())).rejects.toThrow('Kiểm tra đã dừng')
  })
  it('cancels the reader and propagates cancellation without delivering later progress', async () => {
    const abort = new AbortController(); const cancel = vi.fn(); const progress = vi.fn()
    const body = new ReadableStream<Uint8Array>({ start(controller) { controller.enqueue(new TextEncoder().encode(JSON.stringify(step) + '\n')) }, cancel })
    const result = readStorageTest(body, (value) => { progress(value); abort.abort(new DOMException('Aborted', 'AbortError')) }, abort.signal)
    await expect(result).rejects.toMatchObject({ name: 'AbortError' })
    expect(progress).toHaveBeenCalledTimes(1)
    expect(cancel).toHaveBeenCalledTimes(1)
  })
  it('handles split multibyte Vietnamese bytes', async () => {
    const encoded = new TextEncoder().encode(JSON.stringify({ ...step, reason: 'Đã đọc' }) + '\n' + JSON.stringify(done) + '\n')
    const body = new ReadableStream<Uint8Array>({ start(controller) { for (const byte of encoded) controller.enqueue(Uint8Array.of(byte)); controller.close() } })
    const progress = vi.fn()
    await readStorageTest(body, progress)
    expect(progress).toHaveBeenCalledWith({ ...step, reason: 'Đã đọc' })
  })
})
