import { beforeEach, describe, expect, it, vi } from 'vitest'

const workerManagerMocks = vi.hoisted(() => ({
  getWorker: vi.fn(),
  peekWorker: vi.fn(() => null),
  terminate: vi.fn(),
}))

const objectUrlRegistryMocks = vi.hoisted(() => ({
  registerObjectUrl: vi.fn(),
  unregisterObjectUrl: vi.fn(),
}))

const loggerMocks = vi.hoisted(() => ({
  debug: vi.fn(),
  error: vi.fn(),
  warn: vi.fn(),
  info: vi.fn(),
  event: vi.fn(),
  startEvent: vi.fn(),
  child: vi.fn(),
  setLevel: vi.fn(),
}))

const filmstripPrewarmMock = vi.hoisted(() => vi.fn())

vi.mock('@editor/shared/utils/managed-worker', () => ({
  createManagedWorker: vi.fn(() => workerManagerMocks),
}))

vi.mock('@editor/infrastructure/browser/object-url-registry', () => ({
  registerObjectUrl: objectUrlRegistryMocks.registerObjectUrl,
  unregisterObjectUrl: objectUrlRegistryMocks.unregisterObjectUrl,
}))

vi.mock('@editor/shared/logging/logger', () => ({
  createLogger: vi.fn(() => loggerMocks),
}))

vi.mock('@editor/storage/storage-backend', async () =>
  (await import('../../../infrastructure/storage/workspace-fs/__tests__/in-memory-opfs'))
    .storageBackendMock,
)

vi.mock('./background-media-work', async () => {
  const { createBackgroundMediaWorkMocks } =
    await import('../test-utils/background-media-work-test-mocks')
  return createBackgroundMediaWorkMocks(vi)
})

import { memoryOpfs } from '../../../infrastructure/storage/workspace-fs/__tests__/in-memory-opfs'

const READY_META = {
  version: 4,
  width: 960,
  height: 540,
  sourceWidth: 3840,
  sourceHeight: 2160,
  status: 'ready',
  createdAt: 1,
}

/** `<mediaId>@<contentHash>` — the identity a proxy is keyed and cached under. */
function keyOf(mediaId: string, hash: string): string {
  return `${mediaId}@${hash}`
}

function proxyPath(mediaId: string, hash: string, ...segments: string[]): string[] {
  return memoryOpfs.mediaPathFor(mediaId, hash, 'proxy', ...segments)
}

async function plantProxy(
  mediaId: string,
  hash: string,
  meta: Record<string, unknown>,
  payloadSize?: number,
): Promise<void> {
  await memoryOpfs.plantText(proxyPath(mediaId, hash, 'meta.json'), JSON.stringify(meta))
  if (payloadSize !== undefined) {
    await memoryOpfs.plantText(proxyPath(mediaId, hash, 'proxy.mp4'), 'x'.repeat(payloadSize))
  }
}

describe('proxyService', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.resetModules()
    memoryOpfs.reset()
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn(() => 'blob:proxy'),
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: vi.fn(),
    })
  })

  it('allows manual proxy generation for any video clip', async () => {
    const { proxyService } = await import('./proxy-service')

    expect(proxyService.canGenerateProxy('video/mp4')).toBe(true)
    expect(proxyService.canGenerateProxy('video/webm')).toBe(true)
    expect(proxyService.canGenerateProxy('audio/mpeg')).toBe(false)
  })

  describe('loadExistingProxies', () => {
    it('removes partial output for interrupted proxy generations and asks for a regenerate', async () => {
      await plantProxy('video-1', 'sha256:a', { ...READY_META, status: 'generating' })

      const { proxyService } = await import('./proxy-service')
      proxyService.setProxyKey('video-1', keyOf('video-1', 'sha256:a'))

      await expect(proxyService.loadExistingProxies(['video-1'])).resolves.toEqual(['video-1'])
      expect(await memoryOpfs.list(proxyPath('video-1', 'sha256:a'))).toEqual([])
    })

    it('cleans failed proxies without auto-retrying them on startup', async () => {
      await plantProxy('video-2', 'sha256:a', { ...READY_META, status: 'error' })

      const { proxyService } = await import('./proxy-service')
      proxyService.setProxyKey('video-2', keyOf('video-2', 'sha256:a'))

      await expect(proxyService.loadExistingProxies(['video-2'])).resolves.toEqual([])
      expect(await memoryOpfs.list(proxyPath('video-2', 'sha256:a'))).toEqual([])
    })

    it('requeues ready proxies whose file payload is empty', async () => {
      await plantProxy('video-3', 'sha256:a', READY_META, 0)

      const { proxyService } = await import('./proxy-service')
      proxyService.setProxyKey('video-3', keyOf('video-3', 'sha256:a'))

      await expect(proxyService.loadExistingProxies(['video-3'])).resolves.toEqual(['video-3'])
      expect(await memoryOpfs.list(proxyPath('video-3', 'sha256:a'))).toEqual([])
    })

    it('requeues proxies written by an older proxy schema', async () => {
      await plantProxy('video-old', 'sha256:a', { ...READY_META, version: 3 }, 16)

      const { proxyService } = await import('./proxy-service')
      proxyService.setProxyKey('video-old', keyOf('video-old', 'sha256:a'))

      await expect(proxyService.loadExistingProxies(['video-old'])).resolves.toEqual(['video-old'])
    })

    it('registers the loaded proxy under its versioned OPFS path', async () => {
      await plantProxy('video-golden', 'sha256:golden', READY_META, 2048)

      const { proxyService } = await import('./proxy-service')
      proxyService.setProxyKey('video-golden', keyOf('video-golden', 'sha256:golden'))

      await expect(proxyService.loadExistingProxies(['video-golden'])).resolves.toEqual([])

      expect(objectUrlRegistryMocks.registerObjectUrl).toHaveBeenCalledWith(
        'blob:proxy',
        expect.objectContaining({ size: 2048 }),
        {
          opfsPath: proxyPath('video-golden', 'sha256:golden', 'proxy.mp4').join('/'),
          fileSize: 2048,
        },
      )
      expect(proxyService.hasProxy('video-golden')).toBe(true)
    })

    it('never loads a proxy cached for other bytes of the same media', async () => {
      await plantProxy('video-5', 'sha256:old', READY_META, 2048)

      const { proxyService } = await import('./proxy-service')
      proxyService.setProxyKey('video-5', keyOf('video-5', 'sha256:new'))

      await expect(proxyService.loadExistingProxies(['video-5'])).resolves.toEqual([])
      expect(objectUrlRegistryMocks.registerObjectUrl).not.toHaveBeenCalled()
      expect(proxyService.hasProxy('video-5')).toBe(false)
      expect(await memoryOpfs.list(proxyPath('video-5', 'sha256:old'))).toContain('proxy.mp4')
    })

    it('skips (but keeps) a ready proxy whose codec this machine cannot play', async () => {
      // jsdom's HTMLMediaElement.canPlayType returns '' for HEVC, so an HEVC proxy (e.g.
      // generated on another machine) is treated as unplayable here.
      await plantProxy('video-hevc', 'sha256:a', { ...READY_META, codec: 'hevc' }, 2048)

      const { proxyService } = await import('./proxy-service')
      proxyService.setProxyKey('video-hevc', keyOf('video-hevc', 'sha256:a'))

      // Not treated as stale, not loaded, and NOT removed (kept for machines that can play it).
      await expect(proxyService.loadExistingProxies(['video-hevc'])).resolves.toEqual([])
      expect(objectUrlRegistryMocks.registerObjectUrl).not.toHaveBeenCalled()
      expect(await memoryOpfs.list(proxyPath('video-hevc', 'sha256:a'))).toContain('proxy.mp4')
      expect(proxyService.hasProxy('video-hevc')).toBe(false)
    })

    it('treats an unavailable cache as no proxy rather than an error', async () => {
      memoryOpfs.failReads('NotAllowedError')

      const { proxyService } = await import('./proxy-service')
      proxyService.setProxyKey('video-6', keyOf('video-6', 'sha256:a'))

      await expect(proxyService.loadExistingProxies(['video-6'])).resolves.toEqual([])
      expect(loggerMocks.error).not.toHaveBeenCalled()
    })
  })

  describe('deleteProxy', () => {
    it('removes only the proxy of the keyed version, leaving other versions cached', async () => {
      await plantProxy('video-7', 'sha256:old', READY_META, 8)
      await plantProxy('video-7', 'sha256:new', READY_META, 8)

      const { proxyService } = await import('./proxy-service')
      await proxyService.deleteProxy('video-7', keyOf('video-7', 'sha256:old'))

      expect(await memoryOpfs.list(proxyPath('video-7', 'sha256:old'))).toEqual([])
      expect(await memoryOpfs.list(proxyPath('video-7', 'sha256:new'))).toContain('proxy.mp4')
    })
  })

  it('prewarms the first filmstrip window when a proxy finishes loading', async () => {
    await memoryOpfs.plantText(proxyPath('video-4', 'sha256:a', 'proxy.mp4'), 'x'.repeat(1024))

    const { proxyService } = await import('./proxy-service')
    const proxyKey = keyOf('video-4', 'sha256:a')
    proxyService.setProxyKey('video-4', proxyKey)
    proxyService.setFilmstripPrewarm(filmstripPrewarmMock)
    proxyService.setMediaResolver((mediaId) =>
      mediaId === 'video-4' ? { mimeType: 'video/mp4', duration: 20 } : undefined,
    )

    await (
      proxyService as unknown as {
        loadCompletedProxy: (proxyKey: string) => Promise<void>
      }
    ).loadCompletedProxy(proxyKey)

    expect(filmstripPrewarmMock).toHaveBeenNthCalledWith(
      1,
      'video-4',
      expect.objectContaining({ size: 1024 }),
      20,
      { startTime: 0, endTime: 1 },
    )
    expect(filmstripPrewarmMock).toHaveBeenNthCalledWith(
      2,
      'video-4',
      expect.objectContaining({ size: 1024 }),
      20,
      { startTime: 0, endTime: 12 },
    )
  })

  describe('generateProxy', () => {
    it('loads the source on the main thread, then posts the versioned OPFS directory to the worker', async () => {
      const worker = { postMessage: vi.fn() } as unknown as Worker
      workerManagerMocks.getWorker.mockReturnValue(worker)
      const sourceBlob = new Blob(['proxy-source'], { type: 'video/mp4' })
      const loadSource = vi.fn(async () => sourceBlob)
      const proxyKey = keyOf('video-src', 'sha256:a')

      const { proxyService } = await import('./proxy-service')
      proxyService.generateProxy('video-src', loadSource, 1920, 1080, proxyKey)

      await vi.waitFor(() => {
        expect(loadSource).toHaveBeenCalledTimes(1)
        expect(worker.postMessage).toHaveBeenCalledWith({
          type: 'generate',
          mediaId: proxyKey,
          dirSegments: proxyPath('video-src', 'sha256:a'),
          source: sourceBlob,
          sourceWidth: 1920,
          sourceHeight: 1080,
        })
      })
    })

    it('accepts an already-loaded Blob as the source', async () => {
      const worker = { postMessage: vi.fn() } as unknown as Worker
      workerManagerMocks.getWorker.mockReturnValue(worker)
      const sourceBlob = new Blob(['x'], { type: 'video/mp4' })
      const proxyKey = keyOf('video-blob', 'sha256:a')

      const { proxyService } = await import('./proxy-service')
      proxyService.generateProxy('video-blob', sourceBlob, 640, 360, proxyKey)

      await vi.waitFor(() =>
        expect(worker.postMessage).toHaveBeenCalledWith(
          expect.objectContaining({ type: 'generate', mediaId: proxyKey, source: sourceBlob }),
        ),
      )
    })

    it('fails the job when the source bytes are gone instead of posting to the worker', async () => {
      const worker = { postMessage: vi.fn() } as unknown as Worker
      workerManagerMocks.getWorker.mockReturnValue(worker)
      const statuses: string[] = []

      const { proxyService } = await import('./proxy-service')
      proxyService.onStatusChange((_mediaId, status) => statuses.push(status))
      proxyService.generateProxy('video-gone', async () => null, 1920, 1080, keyOf('video-gone', 'sha256:a'))

      await vi.waitFor(() => expect(statuses).toContain('error'))
      expect(worker.postMessage).not.toHaveBeenCalled()
    })

    it('cancels an automatically scheduled background proxy during preview cleanup', async () => {
      const worker = { postMessage: vi.fn() } as unknown as Worker
      workerManagerMocks.getWorker.mockReturnValue(worker)
      const proxyKey = keyOf('video-background', 'sha256:a')
      const { proxyService } = await import('./proxy-service')

      proxyService.generateProxy(
        'video-background',
        async () => new Blob(['x']),
        1920,
        1080,
        proxyKey,
        { priority: 'background' },
      )
      await vi.waitFor(() => expect(worker.postMessage).toHaveBeenCalledTimes(1))
      proxyService.cancelBackgroundProxy('video-background', proxyKey)

      expect(worker.postMessage).toHaveBeenLastCalledWith({ type: 'cancel', mediaId: proxyKey })
    })

    it('does not cancel a background proxy after a user promotes it', async () => {
      const worker = { postMessage: vi.fn() } as unknown as Worker
      workerManagerMocks.getWorker.mockReturnValue(worker)
      const proxyKey = keyOf('video-promoted', 'sha256:a')
      const source = async () => new Blob(['x'])
      const { proxyService } = await import('./proxy-service')

      proxyService.generateProxy('video-promoted', source, 1920, 1080, proxyKey, {
        priority: 'background',
      })
      await vi.waitFor(() => expect(worker.postMessage).toHaveBeenCalledTimes(1))
      proxyService.generateProxy('video-promoted', source, 1920, 1080, proxyKey)
      proxyService.cancelBackgroundProxy('video-promoted', proxyKey)

      expect(worker.postMessage).toHaveBeenCalledTimes(1)
    })
  })
})
