import { afterEach, describe, expect, it, vi } from 'vitest'
import { HeadlessApiError, HeadlessClient, parseHeadlessCredential, withHeadlessLease, type HeadlessSnapshot } from './headless-client'
import { HeadlessEditorStorage } from './headless-storage'

function snapshot(): HeadlessSnapshot {
  return {
    request: { id: 'request-1', nonce: 'claim-nonce', status: 'queued', projectId: 'project-1', fence: null, jobId: null },
    project: { id: 'project-1', name: 'Video', description: '', createdAt: 1, updatedAt: 1, duration: 1, metadata: { width: 1920, height: 1080, fps: 30 }, timeline: { tracks: [], items: [] } },
    revision: 'revision-1', media: [],
    runtime: { workspaceId: 'workspace-1', ready: true, capabilities: { audioDecode: true, gpuRequired: false } }, clone: null,
  }
}

function fakeServer(handler: (url: URL, init: RequestInit) => Response | Promise<Response>) {
  return vi.fn<typeof fetch>((input, init = {}) => handler(new URL(String(input), 'https://app.test'), init))
}

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

describe('headless scoped transport', () => {
  it('reads only the hash token and preserves escaped credentials', () => {
    expect(parseHeadlessCredential('request-1', undefined, '#/render/request-1?token=one%2Btwo.jwt')).toEqual({ requestId: 'request-1', token: 'one+two.jwt' })
    expect(parseHeadlessCredential('request-1', 'provided', '#/render/request-1')).toEqual({ requestId: 'request-1', token: 'provided' })
    for (const hash of ['#/render/request-1', '#/render/request-1?token=', '#/render/request-1?token=a&token=b', '#/render/request-1?token=has%20space']) {
      expect(() => parseHeadlessCredential('request-1', undefined, hash)).toThrow()
    }
    expect(() => parseHeadlessCredential('../other', 'token', '')).toThrow()
  })

  it('fetches the saved snapshot and claims with nonce using no session cookies', async () => {
    const fetcher = fakeServer((url, init) => {
      expect(init.credentials).toBe('omit')
      expect(new Headers(init.headers).get('authorization')).toBe('Bearer scoped-token')
      if (url.pathname.endsWith('/claim')) {
        expect(JSON.parse(String(init.body))).toMatchObject({ nonce: 'claim-nonce', ownerId: expect.stringMatching(/^headless-/) })
        return Response.json({ request: { ...snapshot().request, fence: 'fence-1', status: 'running' } })
      }
      return Response.json(snapshot())
    })
    const client = new HeadlessClient('measure', 'request-1', 'scoped-token', fetcher)
    const loaded = await client.snapshot()
    await expect(client.claim(loaded.request.nonce)).resolves.toBe('fence-1')
    expect(client.mediaUrl('media 1', 'sha256:value')).toBe('/api/measure/request-1/media/media%201?token=scoped-token&v=sha256%3Avalue')
    expect(fetcher.mock.calls.map(([url]) => String(url))).toEqual(['/api/measure/request-1', '/api/measure/request-1/claim'])
  })
  it('default fetcher binds the real global fetch (browser receiver regression)', async () => {
    const realFetch = globalThis.fetch
    const spy = vi.spyOn(globalThis, 'fetch').mockImplementation(async function (this: unknown) {
      expect(this).toBe(globalThis)
      return Response.json(snapshot())
    })
    const client = new HeadlessClient('render', 'request-1', 'scoped-token')
    await client.snapshot()
    expect(spy).toHaveBeenCalled()
    globalThis.fetch = realFetch
    const subtitled = { ...snapshot(), project: { ...snapshot().project, timeline: { tracks: [], items: [{ id: 'cap-1', trackId: 't-1', type: 'subtitle', from: 0, durationInFrames: 180 }] } } }
    await expect(new HeadlessClient('measure', 'request-1', 'token', fakeServer(() => Response.json(subtitled))).snapshot()).resolves.toMatchObject({ project: { id: 'project-1' } })
  })

  it('rejects malformed, mismatched and stale snapshot responses', async () => {
    for (const response of [{ ...snapshot(), revision: '' }, { ...snapshot(), project: { ...snapshot().project, id: 'other-project' } }, { ...snapshot(), request: { ...snapshot().request, binding: { projectRevision: 'old-revision' } } }, { ...snapshot(), media: [{ id: 'incomplete' }] }]) {
      const client = new HeadlessClient('render', 'request-1', 'token', fakeServer(() => Response.json(response)))
      await expect(client.snapshot()).rejects.toThrow()
    }
  })

  it('preserves fenced PCM bytes and reports progress separately from heartbeat', async () => {
    const calls: { url: URL; init: RequestInit }[] = []
    const client = new HeadlessClient('measure', 'request-1', 'token', fakeServer((url, init) => { calls.push({ url, init }); return new Response(null, { status: 204 }) }))
    client.fence = 'fence-1'
    const pcm = new Float32Array([0.5, -0.25, 1, -1])
    await client.uploadPcm(3, new Uint8Array(pcm.buffer))
    const call = calls[0]!
    expect(call.url.pathname).toBe('/api/measure/request-1/chunks')
    expect(call.url.searchParams.get('seq')).toBe('3')
    expect(call.url.searchParams.get('sampleRate')).toBe('48000')
    expect(call.url.searchParams.get('channels')).toBe('2')
    expect(new Headers(call.init.headers).get('X-Render-Fence')).toBe('fence-1')
    const pcmRead = Promise.withResolvers<ArrayBuffer>()
    const reader = new FileReader()
    reader.onload = () => {
      if (reader.result !== null && typeof reader.result !== 'string') pcmRead.resolve(reader.result)
      else pcmRead.reject(new Error('PCM upload did not contain an ArrayBuffer'))
    }
    reader.onerror = () => pcmRead.reject(reader.error ?? new Error('Unable to read PCM upload'))
    reader.onabort = () => pcmRead.reject(new Error('PCM upload read was aborted'))
    reader.readAsArrayBuffer(call.init.body as Blob)
    expect(new Float32Array(await pcmRead.promise)).toEqual(pcm)
    await expect(client.uploadPcm(4, new Uint8Array([1]))).rejects.toThrow('stereo')
    await client.heartbeat({ phase: 'upload', bytesSent: 16, chunksSent: 1 })
    expect(calls.slice(1).map(call => [call.url.pathname, JSON.parse(String(call.init.body))])).toEqual([
      ['/api/measure/request-1/heartbeat', { fence: 'fence-1' }],
      ['/api/measure/request-1/progress', { fence: 'fence-1', progress: { phase: 'upload', bytesSent: 16, chunksSent: 1 } }],
    ])
  })

  it('preserves HTTP failures and rejects an unfenced mutation', async () => {
    const client = new HeadlessClient('render', 'request-1', 'token', fakeServer(() => Response.json({ error: 'Yêu cầu đã được thay thế' }, { status: 409 })))
    await expect(client.snapshot()).rejects.toMatchObject({ name: 'HeadlessApiError', status: 409, message: 'Yêu cầu đã được thay thế' })
    await expect(client.bind('export-1')).rejects.toThrow('chưa được nhận')
  })
})

describe('headless lease lifecycle', () => {
  it('stops and flushes heartbeat before terminal completion', async () => {
    vi.useFakeTimers()
    const calls: string[] = []
    let terminal = false
    const client = new HeadlessClient('render', 'request-1', 'token', fakeServer(url => {
      expect(terminal).toBe(false)
      calls.push(url.pathname)
      if (url.pathname.endsWith('/complete')) {
        terminal = true
        return Response.json({ request: { ...snapshot().request, status: 'done' } })
      }
      return new Response(null, { status: 204 })
    }))
    client.fence = 'fence-1'
    await expect(withHeadlessLease(client, new AbortController(), () => ({ phase: 'uploading' }), async () => 'export-1', async exportId => { await client.complete(exportId) })).resolves.toBe('export-1')
    await vi.advanceTimersByTimeAsync(10_000)
    expect(calls.at(-1)).toBe('/api/render/request-1/complete')
    expect(calls).not.toContain('/api/render/request-1/fail')
    expect(vi.getTimerCount()).toBe(0)
  })

  it('aborts active engine work on lease loss and reports the original failure', async () => {
    vi.useFakeTimers()
    const client = new HeadlessClient('measure', 'request-1', 'token', fakeServer(() => new Response(null, { status: 204 })))
    client.fence = 'fence-1'
    const failure = new HeadlessApiError('Lease replaced', 409, {})
    vi.spyOn(client, 'heartbeat').mockResolvedValueOnce(undefined).mockRejectedValueOnce(failure)
    const fail = vi.spyOn(client, 'fail').mockResolvedValue(undefined)
    const controller = new AbortController()
    const engine = Promise.withResolvers<never>()
    const running = withHeadlessLease(client, controller, () => undefined, () => {
      controller.signal.addEventListener('abort', () => engine.reject(controller.signal.reason), { once: true })
      return engine.promise
    })
    const rejected = expect(running).rejects.toBe(failure)
    await vi.advanceTimersByTimeAsync(2000)
    await rejected
    expect(controller.signal.aborted).toBe(true)
    expect(fail).toHaveBeenCalledWith(failure)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('does not silently discard a failure-report error', async () => {
    const client = new HeadlessClient('render', 'request-1', 'token', fakeServer(() => new Response(null, { status: 204 })))
    vi.spyOn(client, 'heartbeat').mockResolvedValue(undefined)
    const engineError = new Error('encoder failed')
    const reportError = new Error('network failed')
    vi.spyOn(client, 'fail').mockRejectedValue(reportError)
    await expect(withHeadlessLease(client, new AbortController(), () => undefined, async () => { throw engineError })).rejects.toMatchObject({ cause: engineError, errors: [engineError, reportError] })
  })
})

describe('scoped engine storage', () => {
  it('chunks video uploads, commits metadata, and aborts with scoped authorization', async () => {
    const calls: { url: URL; init: RequestInit }[] = []
    const client = new HeadlessClient('render', 'request-1', 'token', fakeServer((url, init) => {
      calls.push({ url, init })
      if (url.pathname.endsWith('/exports')) return Response.json({ exportId: 'export-1' })
      if (url.pathname.endsWith('/commit')) return Response.json({ export: { id: 'export-1', fileName: 'video.mp4', mime: 'video/mp4', size: 16 * 1024 ** 2 + 8, status: 'ready', createdAt: 1, committedAt: 2 } })
      return new Response(null, { status: 204 })
    }))
    client.fence = 'fence-1'
    const storage = new HeadlessEditorStorage(client, snapshot())
    const exportId = await storage.beginExport('project-1', { fileName: 'video.mp4', mime: 'video/mp4' })
    const bytes = new Blob([new Uint8Array(16 * 1024 ** 2 + 8)])
    const progress = vi.fn()
    await storage.uploadExport('project-1', exportId, bytes, { onProgress: progress })
    const uploads = calls.filter(call => call.init.method === 'PUT')
    expect(uploads.map(call => call.url.searchParams.get('offset'))).toEqual(['0', String(16 * 1024 ** 2)])
    expect(uploads.map(call => (call.init.body as Blob).size)).toEqual([16 * 1024 ** 2, 8])
    for (const call of uploads) expect(new Headers(call.init.headers).get('X-Render-Fence')).toBe('fence-1')
    expect(progress).toHaveBeenLastCalledWith(bytes.size, bytes.size)
    await expect(storage.commitExport('project-1', exportId)).resolves.toMatchObject({ id: 'export-1', status: 'ready' })
    await storage.abortExport('project-1', exportId)
    expect(calls.at(-1)?.init.method).toBe('DELETE')
    for (const call of calls) expect(new Headers(call.init.headers).get('Authorization')).toBe('Bearer token')
  })

  it('denies other projects and editor mutations without making a request', async () => {
    const fetcher = fakeServer(() => Response.json({}))
    const storage = new HeadlessEditorStorage(new HeadlessClient('render', 'request-1', 'token', fetcher), snapshot())
    await expect(storage.loadProject('another-project')).rejects.toThrow('phạm vi')
    await expect(storage.getMedia('another-media')).rejects.toThrow('phạm vi')
    await expect(storage.deleteProject('project-1')).rejects.toThrow('không được phép sửa')
    expect(() => storage.mediaUrl('another-media', 'hash')).toThrow('phạm vi')
    expect(fetcher).not.toHaveBeenCalled()
  })
})
