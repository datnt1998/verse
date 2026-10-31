import { createLogger } from '@editor/shared/logging/logger'
import type {
  AudioMeasurementWorkerRequest,
  AudioMeasurementWorkerResponse,
} from './audio-measurement-worker.types'

// Some third-party browser libs assume `window` exists; dedicated workers
// alias it to `globalThis` (same convention as the export render worker).
type WorkerGlobalWithWindow = typeof globalThis & { window?: unknown }
const workerGlobal = globalThis as WorkerGlobalWithWindow
if (typeof workerGlobal.window === 'undefined') {
  Object.defineProperty(globalThis, 'window', { value: globalThis })
}

const log = createLogger('AudioMeasurementWorker')

self.addEventListener('unhandledrejection', (event) => {
  log.error('Measurement worker unhandled rejection', {
    reason: event.reason instanceof Error ? event.reason.message : String(event.reason),
  })
})

const activeRequests = new Map<string, AbortController>()
/** seq the uploader confirmed, per request — bounds in-flight PCM chunks. */
const ackedSeqs = new Map<string, number>()
const ackWaiters = new Map<string, () => void>()
const MAX_IN_FLIGHT_CHUNKS = 4

async function waitForUploadHeadroom(requestId: string, seq: number, signal: AbortSignal): Promise<void> {
  while ((ackedSeqs.get(requestId) ?? -1) < seq - MAX_IN_FLIGHT_CHUNKS) {
    if (signal.aborted) throw new DOMException('Measurement cancelled', 'AbortError')
    await new Promise<void>((resolve) => {
      ackWaiters.set(requestId, resolve)
      signal.addEventListener('abort', () => resolve(), { once: true })
    })
  }
}

self.onmessage = async (event: MessageEvent<AudioMeasurementWorkerRequest>) => {
  const message = event.data
  if (message.type === 'cancel') {
    activeRequests.get(message.requestId)?.abort()
    ackWaiters.get(message.requestId)?.()
    return
  }
  if (message.type === 'ack') {
    const previous = ackedSeqs.get(message.requestId) ?? -1
    if (message.seq > previous) ackedSeqs.set(message.requestId, message.seq)
    ackWaiters.get(message.requestId)?.()
    return
  }
  if (message.type !== 'start') return

  const { requestId, composition, media, compositions } = message
  const controller = new AbortController()
  activeRequests.set(requestId, controller)
  ackedSeqs.set(requestId, -1)

  try {
    const [audioModule, measurementModule, compositionsStore, mediaStore] = await Promise.all([
      import('../utils/canvas-audio'),
      import('../utils/audio-measurement'),
      import('@editor/features/timeline/stores/compositions-store'),
      import('@editor/features/media-library/stores/media-library-store'),
    ])
    // Hydrate only what the mixer consults: the media resolver store and the
    // reachable sub-composition registry. Nothing is persisted — a disposed
    // request discards these per-worker store instances.
    const mediaById: Record<string, (typeof media)[number]> = {}
    for (const record of media) mediaById[record.id] = record
    mediaStore.useMediaLibraryStore.setState({ mediaById })
    compositionsStore.useCompositionsStore.getState().setCompositions(compositions)

    let seq = 0
    let samples = 0
    let sampleRate = 0
    let channels = 0
    for await (const window of audioModule.processAudioWindows(composition, controller.signal)) {
      const bytes = measurementModule.interleavePlanarToF32le(window.samples)
      await waitForUploadHeadroom(requestId, seq, controller.signal)
      const chunk: AudioMeasurementWorkerResponse = {
        type: 'pcm',
        requestId,
        seq: seq++,
        sampleRate: window.sampleRate,
        channels: window.channels,
        bytes: bytes.buffer as ArrayBuffer,
      }
      samples += window.samples[0]?.length ?? 0
      sampleRate = window.sampleRate
      channels = window.channels
      self.postMessage(chunk, { transfer: [chunk.bytes] })
    }

    const done: AudioMeasurementWorkerResponse = {
      type: 'done',
      requestId,
      chunks: seq,
      samples,
      sampleRate,
      channels,
      durationSec: sampleRate > 0 ? samples / sampleRate : 0,
    }
    self.postMessage(done)
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      const cancelled: AudioMeasurementWorkerResponse = { type: 'cancelled', requestId }
      self.postMessage(cancelled)
      return
    }
    const failure: AudioMeasurementWorkerResponse = {
      type: 'error',
      requestId,
      error: error instanceof Error ? error.message : String(error),
    }
    self.postMessage(failure)
  } finally {
    activeRequests.delete(requestId)
    ackedSeqs.delete(requestId)
    ackWaiters.delete(requestId)
    // Dispose decoder caches owned by this request's hydration.
    try {
      const audioModule = await import('../utils/canvas-audio')
      audioModule.clearAudioDecodeCache()
    } catch {
      // Cache teardown is best-effort; the worker is short-lived anyway.
    }
  }
}

export {}
