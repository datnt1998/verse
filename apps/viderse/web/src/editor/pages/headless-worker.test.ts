import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { getEditorStorage, setEditorStorage } from '@editor/storage/storage-backend'
import { serverMediaToMetadata } from '@editor/storage/media-adapter'
import type { ExportRenderStartRequest } from '@editor/features/export/workers/export-render-worker.types'

const renderer = vi.hoisted(() => ({ render: vi.fn(), media: vi.fn(), compositions: vi.fn() }))
vi.mock('@editor/shared/utils/media-utils', () => ({ isGifUrl: () => false, isWebpUrl: () => false }))
vi.mock('@editor/shared/logging/logger', () => ({ createLogger: () => ({ error: vi.fn() }) }))
vi.mock('@editor/features/media-library/stores/media-library-store', () => ({ useMediaLibraryStore: { setState: renderer.media } }))
vi.mock('@editor/features/timeline/stores/compositions-store', () => ({ useCompositionsStore: { getState: () => ({ setCompositions: renderer.compositions }) } }))
vi.mock('@editor/features/export/utils/canvas-render-orchestrator', () => ({ renderComposition: renderer.render, renderAudioOnly: renderer.render }))

const scope = {
  addEventListener: vi.fn(),
  postMessage: vi.fn(),
  onmessage: undefined as ((event: { data: unknown }) => Promise<void>) | undefined,
}
const originalStorage = getEditorStorage()

function start(): ExportRenderStartRequest {
  const media = serverMediaToMetadata({ id: 'voice-1', fileName: 'voice.wav', mime: 'audio/wav', size: 8, contentHash: 'hash-1', createdAt: 1, probe: { kind: 'audio', mime: 'audio/wav', durationSec: 1, width: null, height: null, fps: null, audioCodec: 'pcm', videoCodec: null, sampleRate: 48000, channels: 2 } })
  return {
    type: 'start', requestId: 'export-1',
    settings: { mode: 'video', codec: 'avc', container: 'mp4', quality: 'medium', resolution: { width: 1920, height: 1080 }, fps: 30 },
    composition: { fps: 30, durationInFrames: 30, tracks: [] },
    media: [media], mediaUrls: { 'voice-1': '/api/render/request-1/media/voice-1?token=scoped' },
    compositions: [{ id: 'nested-1', name: 'Nested', items: [], tracks: [], transitions: [], keyframes: [], fps: 30, width: 1920, height: 1080, durationInFrames: 30 }],
  }
}

  // Exercise worker module initialization only after installing its platform global.
beforeAll(async () => {
  vi.stubGlobal('self', scope)
  await import('@editor/features/export/workers/export-render.worker')
})
afterEach(() => { vi.clearAllMocks(); setEditorStorage(originalStorage) })
afterAll(() => { vi.unstubAllGlobals() })

describe('headless export worker context', () => {
  it('hydrates nested compositions and retains the host-scoped media URL before rendering', async () => {
    const message = start()
    renderer.render.mockImplementation(async () => {
      expect(getEditorStorage().mediaUrl('voice-1', 'hash-1')).toBe('/api/render/request-1/media/voice-1?token=scoped')
      expect(renderer.media).toHaveBeenCalledWith({ mediaItems: message.media, mediaById: { 'voice-1': message.media[0] } })
      expect(renderer.compositions).toHaveBeenCalledWith(message.compositions)
      return { blob: new Blob(['video']), mimeType: 'video/mp4', duration: 1, fileSize: 5 }
    })
    await scope.onmessage!({ data: message })
    expect(scope.postMessage).toHaveBeenCalledWith(expect.objectContaining({ type: 'complete', requestId: 'export-1' }))
  })

  it('rejects media outside the snapshot instead of falling back to the session API', async () => {
    renderer.render.mockImplementation(async () => {
      await getEditorStorage().getMedia('outside-snapshot')
    })
    await scope.onmessage!({ data: start() })
    expect(scope.postMessage).toHaveBeenCalledWith(expect.objectContaining({ type: 'error', requestId: 'export-1', error: expect.stringContaining('outside its media snapshot') }))
    expect(() => getEditorStorage().mediaUrl('outside-snapshot', '')).toThrow('no source')
  })
})
