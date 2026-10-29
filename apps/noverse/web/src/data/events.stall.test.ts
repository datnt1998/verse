// @vitest-environment happy-dom
import { afterEach, expect, it, vi } from 'vitest'
import { fetchSource, STREAM_STALL_MS, STREAM_WAKE_STALL_MS } from './events'

// Luồng SSE "sống" nhưng im lặng: mở thành công rồi không bao giờ gửi thêm byte —
// đúng tình huống điện thoại đổi mạng hay iOS treo tab mà fetch không báo lỗi.
function silentStream(chunks: string[] = []) {
  const encoder = new TextEncoder()
  const body = new ReadableStream<Uint8Array>({
    start(controller) { for (const chunk of chunks) controller.enqueue(encoder.encode(chunk)) },
  })
  vi.stubGlobal('fetch', vi.fn(async (_url: unknown, init?: RequestInit) => {
    init?.signal?.throwIfAborted()
    return new Response(body, { status: 200, headers: { 'Content-Type': 'text/event-stream' } })
  }))
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

it('reports a silent stream as an error so the client reconnects and refreshes', async () => {
  vi.useFakeTimers()
  silentStream(['event: hello\ndata: {}\n\n'])
  const source = fetchSource('/api/events')
  const onerror = vi.fn()
  source.onerror = onerror
  await vi.advanceTimersByTimeAsync(STREAM_STALL_MS - 5000)
  expect(onerror).not.toHaveBeenCalled()
  await vi.advanceTimersByTimeAsync(10_000)
  expect(onerror).toHaveBeenCalledTimes(1)
  source.close()
})

it('reconnects at once when the app returns to the foreground after missing pings', async () => {
  vi.useFakeTimers()
  silentStream()
  const source = fetchSource('/api/events')
  const onerror = vi.fn()
  source.onerror = onerror
  await vi.advanceTimersByTimeAsync(STREAM_WAKE_STALL_MS - 1000)
  document.dispatchEvent(new Event('visibilitychange'))
  await vi.advanceTimersByTimeAsync(0)
  expect(onerror).not.toHaveBeenCalled()
  await vi.advanceTimersByTimeAsync(2000)
  window.dispatchEvent(new Event('online'))
  await vi.advanceTimersByTimeAsync(0)
  expect(onerror).toHaveBeenCalledTimes(1)
  source.close()
})

it('stays quiet after the client closes the stream itself', async () => {
  vi.useFakeTimers()
  silentStream()
  const source = fetchSource('/api/events')
  const onerror = vi.fn()
  source.onerror = onerror
  source.close()
  await vi.advanceTimersByTimeAsync(STREAM_STALL_MS * 2)
  window.dispatchEvent(new Event('online'))
  expect(onerror).not.toHaveBeenCalled()
})
