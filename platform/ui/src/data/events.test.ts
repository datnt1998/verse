// Chuyển từ apps/noverse/web/src/data/data.test.ts — phần test client SSE chung.
import { afterEach, expect, it, vi } from 'vitest'
import { createCache } from './cache'
import { createEventsClient } from './events'

afterEach(() => { vi.useRealTimers() })

it('chỉ kiểm tra phiên sau khi nối lại, không probe trong lúc mất kết nối', async () => {
  vi.useFakeTimers()
  const sources: { target: EventTarget; onerror: ((event: Event) => void) | null; close: () => void; addEventListener: (type: string, fn: (event: Event) => void) => void }[] = []
  const probe = vi.fn().mockResolvedValue('ok')
  const client = createEventsClient({ eventTypes: ['hello'], probe, createSource: () => {
    const target = new EventTarget()
    const source = { target, onerror: null, close: vi.fn(), addEventListener: target.addEventListener.bind(target) }
    sources.push(source)
    return source
  } })
  client.start()
  sources[0].target.dispatchEvent(new MessageEvent('hello', { data: '{"subscriber":"one"}' }))
  sources[0].onerror?.(new Event('error'))
  await vi.advanceTimersByTimeAsync(1000)
  sources[1].onerror?.(new Event('error'))
  expect(probe).not.toHaveBeenCalled()
  await vi.advanceTimersByTimeAsync(2000)
  sources[2].target.dispatchEvent(new MessageEvent('hello', { data: '{"subscriber":"two"}' }))
  await Promise.resolve()
  expect(probe).toHaveBeenCalledTimes(1)
  client.stop()
})

it('nối lại 1→2→4→5 giây, giữ Last-Event-ID và làm mới mọi khoá hiển thị', async () => {
  vi.useFakeTimers()
  const sources: { target: EventTarget; onerror: ((event: Event) => void) | null; close: () => void; addEventListener: (type: string, fn: (event: Event) => void) => void }[] = []
  const urls: string[] = []
  const cache = createCache()
  const fetcher = vi.fn().mockResolvedValue(1)
  cache.retain(['a'], fetcher)
  await Promise.resolve(); await Promise.resolve()
  const client = createEventsClient({ eventTypes: ['hello'], createSource: (url) => {
    urls.push(url)
    const target = new EventTarget()
    const source = { target, onerror: null, close: vi.fn(), addEventListener: target.addEventListener.bind(target) }
    sources.push(source)
    return source
  } })
  client.onReconnect(() => { void cache.invalidate() })
  client.start()
  sources[0].target.dispatchEvent(new MessageEvent('hello', { data: '{"subscriber":"one"}', lastEventId: '41' }))
  for (const [index, delay] of [1000, 2000, 4000, 5000].entries()) {
    sources[index].onerror?.(new Event('error'))
    expect(client.status().reconnectAt).toBe(Date.now() + delay)
    await vi.advanceTimersByTimeAsync(delay - 1)
    expect(sources).toHaveLength(index + 1)
    await vi.advanceTimersByTimeAsync(1)
    expect(sources).toHaveLength(index + 2)
  }
  expect(urls.at(-1)).toContain('lastEventId=41')
  sources.at(-1)!.target.dispatchEvent(new MessageEvent('hello', { data: '{"subscriber":"two"}' }))
  await Promise.resolve(); await Promise.resolve()
  expect(fetcher).toHaveBeenCalledTimes(2)
  expect(client.status().state).toBe('open')
  client.stop()
})
