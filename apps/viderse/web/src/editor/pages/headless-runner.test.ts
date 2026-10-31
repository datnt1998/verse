import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { CompositionInputProps } from '@editor/types/export'
import type { runServerExport } from '../ui/export/export-runner'
import { getEditorStorage } from '@editor/storage/storage-backend'
import { HeadlessClient, type HeadlessSnapshot } from './headless-client'
import { runHeadlessRequest } from './headless-runner'

const engine = vi.hoisted(() => ({
  mix: vi.fn<() => AsyncGenerator<{ samples: Float32Array[]; sampleRate: number; channels: number }>>(),
  clear: vi.fn(),
  media: vi.fn(),
  compositions: vi.fn(),
  convert: vi.fn<() => { composition: CompositionInputProps; compositions: [] }>(),
  render: vi.fn<(args: Parameters<typeof runServerExport>[0]) => Promise<{ record: { id: string } }>>(),
}))
vi.mock('@editor/features/export/utils/canvas-audio', () => ({ processAudioWindows: engine.mix, clearAudioDecodeCache: engine.clear }))
vi.mock('@editor/features/media-library/stores/media-library-store', () => ({ useMediaLibraryStore: { setState: engine.media } }))
vi.mock('@editor/features/timeline/stores/compositions-store', () => ({ useCompositionsStore: { getState: () => ({ setCompositions: engine.compositions }) } }))
vi.mock('@editor/features/timeline/deps/media-library-resolver', () => ({ resolveMediaUrls: async (tracks: CompositionInputProps['tracks']) => tracks }))
vi.mock('../ui/clone/audio-measurement', () => ({ compositionFromProject: engine.convert }))
vi.mock('../ui/export/export-runner', () => ({ runServerExport: engine.render }))

function makeSnapshot(): HeadlessSnapshot {
  return {
    request: { id: 'request-1', nonce: 'nonce-1', projectId: 'project-1', status: 'queued', fence: null, jobId: 'job-1' },
    project: { id: 'project-1', name: 'Video', description: '', createdAt: 1, updatedAt: 1, duration: 1, metadata: { width: 1920, height: 1080, fps: 30 }, timeline: { tracks: [], items: [] } },
    revision: 'revision-1',
    media: [{ id: 'media-1', fileName: 'voice.wav', mime: 'audio/wav', size: 16, contentHash: 'sha256:voice', createdAt: 1, probe: { kind: 'audio', mime: 'audio/wav', durationSec: 1, width: null, height: null, fps: null, videoCodec: null, audioCodec: 'pcm', sampleRate: 48000, channels: 2 } }],
    runtime: { workspaceId: 'workspace-1', ready: true, capabilities: { audioDecode: true, gpuRequired: false } },
    clone: { jobId: 'job-1', gate: { allowed: true, blockers: [] } },
  }
}

function server(snapshot = makeSnapshot()) {
  const calls: { path: string; init: RequestInit }[] = []
  const fetcher = vi.fn<typeof fetch>(async (input, init = {}) => {
    const path = new URL(String(input), 'https://app.test').pathname
    calls.push({ path, init })
    if (path.endsWith('/claim')) return Response.json({ request: { ...snapshot.request, status: 'running', fence: 'fence-1' } })
    if (path.endsWith('/runtime')) return Response.json({ runtime: snapshot.runtime, clone: snapshot.clone })
    if (path.endsWith('/complete')) return Response.json({ request: { ...snapshot.request, status: 'measured', result: { lufs: -16.5, durationSec: 1, samples: 48000 } }, score: null })
    if (path.endsWith('/request-1')) return Response.json(snapshot)
    return new Response(null, { status: 204 })
  })
  return { calls, fetcher }
}

beforeEach(() => {
  engine.convert.mockReturnValue({ composition: { fps: 30, durationInFrames: 30, width: 1920, height: 1080, tracks: [] }, compositions: [] })
  engine.mix.mockImplementation(async function* () { yield { samples: [new Float32Array([0.5, 1]), new Float32Array([-0.5, -1])], sampleRate: 48000, channels: 2 } })
  engine.render.mockResolvedValue({ record: { id: 'export-1' } })
})
afterEach(() => { vi.clearAllMocks(); vi.unstubAllGlobals() })

describe('headless engine execution', () => {
  it('streams interleaved engine PCM then completes and uses the server loudness result', async () => {
    const originalStorage = getEditorStorage()
    const { calls, fetcher } = server()
    const progress = vi.fn()
    const client = new HeadlessClient('measure', 'request-1', 'scoped-token', fetcher)
    await expect(runHeadlessRequest(client, new AbortController(), progress)).resolves.toEqual({ lufs: -16.5, durationSec: 1 })
    const uploads = calls.filter(call => call.path.endsWith('/chunks'))
    expect(uploads).toHaveLength(1)
    const pcmRead = Promise.withResolvers<ArrayBuffer>()
    const reader = new FileReader()
    reader.onload = () => {
      if (reader.result !== null && typeof reader.result !== 'string') pcmRead.resolve(reader.result)
      else pcmRead.reject(new Error('PCM upload did not contain an ArrayBuffer'))
    }
    reader.onerror = () => pcmRead.reject(reader.error ?? new Error('Unable to read PCM upload'))
    reader.onabort = () => pcmRead.reject(new Error('PCM upload read was aborted'))
    reader.readAsArrayBuffer(uploads[0]!.init.body as Blob)
    expect(new Float32Array(await pcmRead.promise)).toEqual(new Float32Array([0.5, -0.5, 1, -1]))
    expect(calls.at(-1)?.path).toBe('/api/measure/request-1/complete')
    expect(calls.some(call => call.path.endsWith('/fail'))).toBe(false)
    expect(engine.media).toHaveBeenCalledWith(expect.objectContaining({ currentProjectId: 'project-1', mediaById: { 'media-1': expect.objectContaining({ id: 'media-1', contentHash: 'sha256:voice' }) } }))
    expect(progress).toHaveBeenCalledWith({ phase: 'upload', bytesSent: 16, chunksSent: 1 })
    expect(getEditorStorage()).toBe(originalStorage)
    expect(engine.clear).toHaveBeenCalledOnce()
    for (const call of calls) {
      expect(new Headers(call.init.headers).get('Authorization')).toBe('Bearer scoped-token')
      expect(call.init.credentials).toBe('omit')
    }
  })

  it('rejects invalid engine PCM format and reports a fenced failure without completing', async () => {
    const originalStorage = getEditorStorage()
    engine.mix.mockImplementation(async function* () { yield { samples: [new Float32Array(2)], sampleRate: 44100, channels: 1 } })
    const { calls, fetcher } = server()
    await expect(runHeadlessRequest(new HeadlessClient('measure', 'request-1', 'token', fetcher), new AbortController(), vi.fn())).rejects.toThrow('48 kHz')
    expect(calls.some(call => call.path.endsWith('/complete') || call.path.endsWith('/chunks'))).toBe(false)
    const failure = calls.find(call => call.path.endsWith('/fail'))!
    expect(JSON.parse(String(failure.init.body))).toMatchObject({ fence: 'fence-1', error: { code: 'internal_error', message: expect.stringContaining('48 kHz') } })
    expect(getEditorStorage()).toBe(originalStorage)
  })

  it('claims before readiness checks so capability failures reach the supervisor', async () => {
    const snapshot = makeSnapshot()
    snapshot.runtime.ready = false
    const { calls, fetcher } = server(snapshot)
    await expect(runHeadlessRequest(new HeadlessClient('measure', 'request-1', 'token', fetcher), new AbortController(), vi.fn())).rejects.toThrow('chưa sẵn sàng')
    expect(calls.findIndex(call => call.path.endsWith('/claim'))).toBeLessThan(calls.findIndex(call => call.path.endsWith('/fail')))
    expect(engine.mix).not.toHaveBeenCalled()
  })

  it('uses the shared real export driver with scoped gate, binding and completion', async () => {
    vi.stubGlobal('navigator', { gpu: { requestAdapter: vi.fn().mockResolvedValue({}) } })
    vi.stubGlobal('VideoEncoder', { isConfigSupported: vi.fn().mockResolvedValue({ supported: true }) })
    const { calls, fetcher } = server()
    engine.render.mockImplementation(async options => {
      expect(options.revision).toBe('revision-1')
      expect(options.settings).toMatchObject({ codec: 'h264', videoContainer: 'mp4', fps: 30, resolution: { width: 1920, height: 1080 }, renderWholeProject: true })
      await expect(options.fetchCloneGate?.()).resolves.toEqual({ allowed: true, blockers: [] })
      expect(getEditorStorage().mediaUrl('media-1', 'sha256:voice')).toContain('/api/render/request-1/media/media-1?token=scoped-token')
      await options.onExportId?.('export-1')
      return { record: { id: 'export-1' } }
    })
    await expect(runHeadlessRequest(new HeadlessClient('render', 'request-1', 'scoped-token', fetcher), new AbortController(), vi.fn())).resolves.toEqual({ exportId: 'export-1' })
    const bind = calls.find(call => call.path.endsWith('/bind'))!
    expect(JSON.parse(String(bind.init.body))).toEqual({ fence: 'fence-1', exportId: 'export-1' })
    expect(calls.at(-1)?.path).toBe('/api/render/request-1/complete')
    expect(JSON.parse(String(calls.at(-1)?.init.body))).toEqual({ fence: 'fence-1', exportId: 'export-1' })
    expect(engine.mix).not.toHaveBeenCalled()
  })
})
