// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'

const workerManagerMocks = vi.hoisted(() => ({
  getWorker: vi.fn(),
  peekWorker: vi.fn(() => null),
  terminate: vi.fn(),
}))

vi.mock('@editor/shared/utils/managed-worker', () => ({
  createManagedWorker: vi.fn(() => workerManagerMocks),
  rejectAndDeletePendingRequests: (pending: Map<string, { reject: (e: Error) => void }>, e: Error) => {
    for (const request of pending.values()) request.reject(e)
    pending.clear()
  },
}))

vi.mock('@editor/shared/logging/logger', () => ({
  createLogger: vi.fn(() => ({
    debug: vi.fn(),
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
  })),
}))

import { mediaProcessorService } from './media-processor-service'

const file = new File(['x'], 'clip.wav', { type: 'audio/wav' })

beforeEach(() => {
  vi.clearAllMocks()
  workerManagerMocks.getWorker.mockReturnValue({ postMessage: vi.fn() } as unknown as Worker)
  // Singleton: drop requests orphaned by previous tests.
  ;(mediaProcessorService as unknown as { pendingRequests: Map<string, unknown> }).pendingRequests.clear()
})

describe('mediaProcessorService.processMedia abort', () => {
  it('rejects immediately when the signal is already aborted, without starting the worker', async () => {
    const signal = AbortSignal.abort()

    await expect(mediaProcessorService.processMedia(file, 'audio/wav', { signal })).rejects.toMatchObject({
      name: 'AbortError',
    })
    expect(workerManagerMocks.getWorker).not.toHaveBeenCalled()
  })

  it('aborts an in-flight request: rejects it and terminates the worker when nothing else runs', async () => {
    const worker = { postMessage: vi.fn() } as unknown as Worker
    workerManagerMocks.getWorker.mockReturnValue(worker)
    const controller = new AbortController()

    const pending = mediaProcessorService.processMedia(file, 'audio/wav', {
      signal: controller.signal,
    })
    expect(worker.postMessage).toHaveBeenCalledTimes(1)

    controller.abort()

    await expect(pending).rejects.toMatchObject({ name: 'AbortError' })
    expect(workerManagerMocks.terminate).toHaveBeenCalledTimes(1)
  })

  it('still aborts the request but keeps a busy worker when other imports are queued', async () => {
    const worker = { postMessage: vi.fn() } as unknown as Worker
    workerManagerMocks.getWorker.mockReturnValue(worker)
    const controller = new AbortController()

    // Two in-flight requests: aborting one must not kill the worker the other needs.
    const cancelled = mediaProcessorService.processMedia(file, 'audio/wav', {
      signal: controller.signal,
    })
    void mediaProcessorService.processMedia(file, 'video/mp4').catch(() => undefined)

    controller.abort()

    await expect(cancelled).rejects.toMatchObject({ name: 'AbortError' })
    expect(workerManagerMocks.terminate).not.toHaveBeenCalled()
  })

  it('does not forward the abort signal to the worker payload', async () => {
    const worker = { postMessage: vi.fn() } as unknown as Worker
    workerManagerMocks.getWorker.mockReturnValue(worker)
    const controller = new AbortController()

    void mediaProcessorService.processMedia(file, 'audio/wav', {
      thumbnailTimestamp: 1,
      signal: controller.signal,
    }).catch(() => undefined)

    expect(vi.mocked(worker.postMessage).mock.calls[0]?.[0]).toMatchObject({
      type: 'process',
      options: { thumbnailTimestamp: 1 },
    })
    expect(vi.mocked(worker.postMessage).mock.calls[0]?.[0].options).not.toHaveProperty('signal')
    controller.abort()
  })
})
