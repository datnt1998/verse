import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ProcessMediaRequest } from './media-processor.worker'

const mocks = vi.hoisted(() => ({ register: vi.fn(), codec: 'h264', sink: vi.fn(), post: vi.fn() }))
vi.mock('@editor/infrastructure/browser/register-prores-decoder', () => ({ ensureProResDecoderRegistered: mocks.register }))
vi.mock('mediabunny', () => ({
  ALL_FORMATS: [],
  BlobSource: class { constructor(public blob: Blob) {} },
  Input: class {
    computeDuration = async () => 1
    getPrimaryAudioTrack = async () => ({ codec: 'pcm-s16', sampleRate: 48000, channels: 2 })
    getPrimaryVideoTrack = async () => ({ codec: mocks.codec, displayWidth: 640, displayHeight: 360, canDecode: async () => true })
    dispose() {}
  },
  CanvasSink: class {
    constructor() { mocks.sink() }
    getCanvas = async () => ({ canvas: { convertToBlob: async () => new Blob(['thumbnail']) } })
    dispose() {}
  },
}))

interface WorkerRealm { onmessage: ((event: MessageEvent<ProcessMediaRequest>) => Promise<void>) | null; postMessage: typeof mocks.post }
let realm: WorkerRealm
beforeEach(() => {
  vi.clearAllMocks(); vi.resetModules()
  mocks.codec = 'h264'
  mocks.register.mockResolvedValue(undefined)
  realm = { onmessage: null, postMessage: mocks.post }
  vi.stubGlobal('self', realm)
})

async function process(mimeType: string, generateThumbnail: boolean) {
  // Exercise the worker's actual message entry point in an isolated realm.
  await import('./media-processor.worker')
  const request: ProcessMediaRequest = { type: 'process', requestId: 'import', file: new File(['media'], 'media'), mimeType, options: { generateThumbnail, fastMetadata: true } }
  await realm.onmessage?.({ data: request } as MessageEvent<ProcessMediaRequest>)
}

describe('media worker lazy ProRes registration', () => {
  it('completes audio metadata while unrelated ProRes initialization would never resolve', async () => {
    mocks.register.mockImplementation(() => Promise.withResolvers<void>().promise)
    await process('audio/wav', false)
    expect(mocks.register).not.toHaveBeenCalled()
    expect(mocks.post).toHaveBeenCalledWith(expect.objectContaining({ type: 'complete', metadata: expect.objectContaining({ type: 'audio', codec: 'pcm-s16', duration: 1 }) }))
  })
  it('does not load a ProRes decoder for H264 thumbnails', async () => {
    await process('video/mp4', true)
    expect(mocks.register).not.toHaveBeenCalled()
    expect(mocks.sink).toHaveBeenCalledTimes(1)
    expect(mocks.post).toHaveBeenCalledWith(expect.objectContaining({ type: 'complete', thumbnail: expect.any(Blob) }))
  })
  it('registers before the first ProRes thumbnail decode', async () => {
    mocks.codec = 'prores'
    await process('video/quicktime', true)
    expect(mocks.register).toHaveBeenCalledTimes(1)
    expect(mocks.register.mock.invocationCallOrder[0]).toBeLessThan(mocks.sink.mock.invocationCallOrder[0])
    expect(mocks.post).toHaveBeenCalledWith(expect.objectContaining({ type: 'complete', metadata: expect.objectContaining({ videoCodecSupported: false }), thumbnail: expect.any(Blob) }))
  })
  it('probes ProRes metadata without initializing its decoder when no frame is requested', async () => {
    mocks.codec = 'prores'
    await process('video/quicktime', false)
    expect(mocks.register).not.toHaveBeenCalled()
    expect(mocks.sink).not.toHaveBeenCalled()
    expect(mocks.post).toHaveBeenCalledWith(expect.objectContaining({ type: 'complete' }))
  })
})
