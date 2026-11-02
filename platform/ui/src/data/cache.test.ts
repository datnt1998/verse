// Chuyển từ apps/noverse/web/src/data/data.test.ts — phần test bộ nhớ đệm chung.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createCache } from './cache'

// target ES2022 — Promise.withResolvers chưa có trong lib.
const deferred = <T>() => {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
afterEach(() => { vi.useRealTimers() })

describe('bộ nhớ đệm', () => {
  it('gộp yêu cầu trùng và chỉ huỷ khi không còn người đọc', async () => {
    const work = deferred<number>()
    const fetcher = vi.fn(({ signal }: { signal: AbortSignal }) => { expect(signal.aborted).toBe(false); return work.promise })
    const cache = createCache({ defer: (fn) => fn() })
    const release = cache.retain(['a'], fetcher)
    const second = cache.retain(['a'], fetcher)
    expect(fetcher).toHaveBeenCalledTimes(1)
    release(); work.resolve(7); await work.promise; await Promise.resolve()
    expect(cache.get(['a']).data).toBe(7)
    second()
  })
  it('SWR giữ dữ liệu cũ khi tải lại thất bại', async () => {
    const cache = createCache()
    const fetcher = vi.fn().mockResolvedValueOnce('bản cũ').mockRejectedValueOnce(new Error('mạng lỗi'))
    cache.retain(['a'], fetcher)
    await cache.refresh(['a']); await Promise.resolve(); await Promise.resolve()
    expect(cache.get(['a']).data).toBe('bản cũ')
    expect(cache.get(['a']).status).toBe('ready')
    expect(cache.get(['a']).error).toBeInstanceOf(Error)
  })
  it('invalidate khớp prefix và không chạm khoá bên cạnh', async () => {
    const cache = createCache()
    const story = vi.fn().mockResolvedValue(1)
    const other = vi.fn().mockResolvedValue(2)
    cache.retain(['story', 'p', 's', 'chapter', 1], story)
    cache.retain(['story', 'p', 't', 'chapter', 1], other)
    await Promise.resolve(); await Promise.resolve()
    await cache.invalidate(['story', 'p', 's'])
    expect(story).toHaveBeenCalledTimes(2)
    expect(other).toHaveBeenCalledTimes(1)
  })
  it('focus giới hạn 15 giây cho mỗi khoá đang hiển thị', async () => {
    let now = 0
    const cache = createCache({ now: () => now })
    const fetcher = vi.fn().mockResolvedValue(1)
    cache.retain(['a'], fetcher)
    await Promise.resolve(); await Promise.resolve()
    now = 14999; await cache.revalidateActive(15000)
    expect(fetcher).toHaveBeenCalledTimes(1)
    now = 15000; await cache.revalidateActive(15000)
    expect(fetcher).toHaveBeenCalledTimes(2)
    now = 15001; await cache.revalidateActive(15000)
    expect(fetcher).toHaveBeenCalledTimes(2)
  })
})

it('AbortError bên trong fetcher không giữ yêu cầu đang bay mãi', async () => {
  const cache = createCache()
  const fetcher = vi.fn().mockRejectedValueOnce(new DOMException('Đã huỷ yêu cầu', 'AbortError')).mockResolvedValueOnce('đã tải lại')
  cache.retain(['aborted'], fetcher)
  await Promise.resolve(); await Promise.resolve()
  expect(cache.get(['aborted']).status).toBe('error')
  await cache.refresh(['aborted'])
  expect(fetcher).toHaveBeenCalledTimes(2)
  expect(cache.get(['aborted']).data).toBe('đã tải lại')
})

it('imperative reuse respects invalidation and freshness at the boundary', async () => {
  let now = 0
  let revision = 1
  const cache = createCache({ now: () => now })
  const fetcher = async () => ({ revision })
  await cache.fetch(['image', 'a'], fetcher, { staleTime: 100 })
  revision = 2
  now = 99
  await cache.fetch(['image', 'a'], fetcher, { staleTime: 100 })
  expect(cache.get<{ revision: number }>(['image', 'a']).data?.revision).toBe(1)
  await cache.invalidate(['image', 'a'])
  await cache.fetch(['image', 'a'], fetcher, { staleTime: 100 })
  expect(cache.get<{ revision: number }>(['image', 'a']).data?.revision).toBe(2)
  revision = 3
  now = 199
  await cache.fetch(['image', 'a'], fetcher, { staleTime: 100 })
  expect(cache.get<{ revision: number }>(['image', 'a']).data?.revision).toBe(3)
})
