// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'

const storageMocks = vi.hoisted(() => ({
  getAllMedia: vi.fn(),
  getMedia: vi.fn(),
  updateMedia: vi.fn(),
  deleteMedia: vi.fn(),
  saveThumbnail: vi.fn(async () => undefined),
  getThumbnailByMediaId: vi.fn(),
  getThumbnailsByMediaIds: vi.fn(),
  deleteThumbnailsByMediaId: vi.fn(async () => undefined),
  associateMediaWithProject: vi.fn(async () => undefined),
  removeMediaBatchFromProject: vi.fn(async () => undefined),
  removeMediaFromProject: vi.fn(async () => undefined),
  getProjectMediaIds: vi.fn(),
  getProjectsUsingMedia: vi.fn(),
  getMediaForProject: vi.fn(),
  deleteTranscript: vi.fn(async () => undefined),
  readAiOutput: vi.fn(),
  saveCaptions: vi.fn(async () => []),
  deleteCaptions: vi.fn(async () => undefined),
  deleteScenes: vi.fn(async () => undefined),
  importMediaSource: vi.fn(),
  replaceMediaSource: vi.fn(),
  mediaSourceUrl: vi.fn(
    (media: { id: string; contentHash?: string }) => `/media/${media.id}?v=${media.contentHash}`,
  ),
}))

const proxyMocks = vi.hoisted(() => ({
  deleteProxy: vi.fn(async () => undefined),
  clearProxyKey: vi.fn(),
}))

const mediaProcessorMocks = vi.hoisted(() => ({
  processMedia: vi.fn(),
  hasUnsupportedAudioCodec: vi.fn(() => ({ unsupported: false })),
}))

const compositionRuntimeMocks = vi.hoisted(() => ({
  needsCustomAudioDecoder: vi.fn(() => false),
  startPreviewAudioConform: vi.fn(async () => undefined),
  startPreviewAudioStartupWarm: vi.fn(async () => undefined),
  deletePreviewAudioConform: vi.fn(async () => undefined),
}))

const cacheMocks = vi.hoisted(() => ({
  gifFrameCache: { getGifFrames: vi.fn(), clearMedia: vi.fn(async () => undefined) },
  filmstripCache: { clearMedia: vi.fn(async () => undefined) },
  waveformCache: { clearMedia: vi.fn(async () => undefined) },
}))

const validationMocks = vi.hoisted(() => ({
  validateMediaFileContent: vi.fn(async () => ({ valid: true as boolean, error: '' })),
}))

vi.mock('@editor/infrastructure/storage', () => storageMocks)

vi.mock('./proxy-service', () => ({ proxyService: proxyMocks }))

vi.mock('./background-media-work', async () => {
  const { createBackgroundMediaWorkMocks } =
    await import('../test-utils/background-media-work-test-mocks')
  return createBackgroundMediaWorkMocks(vi)
})

vi.mock('./media-processor-service', () => ({ mediaProcessorService: mediaProcessorMocks }))

vi.mock('@editor/runtime/composition-runtime/utils/audio-codec-detection', () => ({
  needsCustomAudioDecoder: compositionRuntimeMocks.needsCustomAudioDecoder,
}))

vi.mock('@editor/runtime/composition-runtime/utils/audio-decode-cache', () => ({
  startPreviewAudioConform: compositionRuntimeMocks.startPreviewAudioConform,
  startPreviewAudioStartupWarm: compositionRuntimeMocks.startPreviewAudioStartupWarm,
}))

vi.mock('@editor/runtime/composition-runtime/utils/preview-audio-conform', () => ({
  deletePreviewAudioConform: compositionRuntimeMocks.deletePreviewAudioConform,
}))

vi.mock('@editor/features/media-library/deps/timeline-services', () => ({
  gifFrameCache: cacheMocks.gifFrameCache,
  importGifFrameCache: vi.fn(async () => ({ gifFrameCache: cacheMocks.gifFrameCache })),
  filmstripCache: cacheMocks.filmstripCache,
  importFilmstripCache: vi.fn(async () => ({ filmstripCache: cacheMocks.filmstripCache })),
  waveformCache: cacheMocks.waveformCache,
  importWaveformCache: vi.fn(async () => ({ waveformCache: cacheMocks.waveformCache })),
}))

vi.mock('../utils/validation', () => ({
  validateMediaFile: vi.fn(() => ({ valid: true })),
  validateMediaFileContent: validationMocks.validateMediaFileContent,
  getMimeType: vi.fn((file: File) => file.type || 'application/octet-stream'),
  isLottieMime: vi.fn(() => false),
}))

vi.mock('../utils/proxy-key', () => ({
  getSharedProxyKey: vi.fn((media: { id: string }) => `proxy-${media.id}`),
}))

import { mediaLibraryService } from './media-library-service'
import type { MediaMetadata } from '@editor/types/storage'

function makeMedia(overrides: Partial<MediaMetadata> = {}): MediaMetadata {
  return {
    id: 'media-1',
    storageType: 'workspace',
    contentHash: 'sha256:aaa',
    fileName: 'video.mp4',
    fileSize: 1024,
    mimeType: 'video/mp4',
    duration: 10,
    width: 1920,
    height: 1080,
    fps: 30,
    codec: 'h264',
    bitrate: 0,
    tags: [],
    createdAt: 1,
    updatedAt: 1,
    ...overrides,
  }
}

const VIDEO_PROBE = {
  metadata: {
    type: 'video' as const,
    duration: 12,
    width: 1280,
    height: 720,
    fps: 24,
    codec: 'avc',
    audioCodec: 'aac',
    audioCodecSupported: true,
    videoCodecSupported: true,
    keyframeTimestamps: [0, 2],
    gopInterval: 2,
    bitrate: 4000,
  },
  thumbnail: new Blob(['thumb'], { type: 'image/jpeg' }),
}

function makeFile(name = 'clip.mp4', type = 'video/mp4', content = 'bytes'): File {
  return new File([content], name, { type })
}

beforeEach(() => {
  vi.clearAllMocks()
  mediaProcessorMocks.processMedia.mockResolvedValue(VIDEO_PROBE)
  mediaProcessorMocks.hasUnsupportedAudioCodec.mockReturnValue({ unsupported: false })
  validationMocks.validateMediaFileContent.mockResolvedValue({ valid: true, error: '' })
  storageMocks.getMediaForProject.mockResolvedValue([])
  storageMocks.importMediaSource.mockImplementation(async (file: File) =>
    makeMedia({ id: 'server-1', fileName: file.name, fileSize: file.size }),
  )
  storageMocks.updateMedia.mockImplementation(async (id: string, updates: Partial<MediaMetadata>) =>
    makeMedia({ id, ...updates }),
  )
  storageMocks.getProjectsUsingMedia.mockResolvedValue([])
})

describe('importFile', () => {
  it('uploads the File itself, linked to the project, with abort and progress hooks', async () => {
    const file = makeFile()
    const controller = new AbortController()
    const onProgress = vi.fn()

    await mediaLibraryService.importFile(file, {
      projectId: 'project-1',
      signal: controller.signal,
      onProgress,
    })

    expect(storageMocks.importMediaSource).toHaveBeenCalledExactlyOnceWith(file, {
      projectId: 'project-1',
      mimeType: 'video/mp4',
      signal: controller.signal,
      onProgress,
    })
  })

  it('stores the browser-derived analysis against the server media id', async () => {
    const result = await mediaLibraryService.importFile(makeFile(), { projectId: 'project-1' })

    expect(storageMocks.updateMedia).toHaveBeenCalledWith(
      'server-1',
      expect.objectContaining({
        duration: 12,
        width: 1280,
        height: 720,
        fps: 24,
        keyframeTimestamps: [0, 2],
        gopInterval: 2,
        thumbnailId: expect.any(String),
      }),
    )
    expect(storageMocks.saveThumbnail).toHaveBeenCalledWith(
      expect.objectContaining({ mediaId: 'server-1', timestamp: 0 }),
    )
    expect(result.id).toBe('server-1')
  })

  it('persists user tags and attribution with the import', async () => {
    const attribution = { provider: 'LottieFiles', author: 'Ada' }

    await mediaLibraryService.importFile(makeFile(), { tags: ['b-roll'], attribution })

    expect(storageMocks.updateMedia).toHaveBeenCalledWith(
      'server-1',
      expect.objectContaining({ tags: ['b-roll'], attribution }),
    )
  })

  it('imports into the library without a project link when no project is given', async () => {
    await mediaLibraryService.importFile(makeFile())

    expect(storageMocks.getMediaForProject).not.toHaveBeenCalled()
    expect(storageMocks.importMediaSource).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ projectId: undefined }),
    )
  })

  it('returns the existing media for a duplicate in the project without uploading', async () => {
    const existing = makeMedia({ id: 'existing', fileName: 'clip.mp4', fileSize: 5 })
    storageMocks.getMediaForProject.mockResolvedValue([existing])

    const result = await mediaLibraryService.importFile(makeFile('clip.mp4'), {
      projectId: 'project-1',
    })

    expect(result).toMatchObject({ id: 'existing', isDuplicate: true })
    expect(storageMocks.importMediaSource).not.toHaveBeenCalled()
  })

  it('rejects an invalid file before analysing or uploading it', async () => {
    validationMocks.validateMediaFileContent.mockResolvedValue({ valid: false, error: 'Bad file' })

    await expect(mediaLibraryService.importFile(makeFile())).rejects.toThrow('Bad file')

    expect(mediaProcessorMocks.processMedia).not.toHaveBeenCalled()
    expect(storageMocks.importMediaSource).not.toHaveBeenCalled()
  })

  it('does not upload once the caller has aborted', async () => {
    const controller = new AbortController()
    controller.abort()

    await expect(
      mediaLibraryService.importFile(makeFile(), { signal: controller.signal }),
    ).rejects.toThrow()

    expect(storageMocks.importMediaSource).not.toHaveBeenCalled()
  })

  it('reports unsupported audio codecs on the result', async () => {
    mediaProcessorMocks.hasUnsupportedAudioCodec.mockReturnValue({ unsupported: true })

    const result = await mediaLibraryService.importFile(makeFile())

    expect(result.hasUnsupportedCodec).toBe(true)
  })

  it('leaves nothing behind when the upload itself fails', async () => {
    storageMocks.importMediaSource.mockRejectedValue(new Error('upload failed'))

    await expect(mediaLibraryService.importFile(makeFile())).rejects.toThrow('upload failed')

    expect(storageMocks.deleteMedia).not.toHaveBeenCalled()
    expect(storageMocks.saveThumbnail).not.toHaveBeenCalled()
  })

  it('removes the uploaded media and its thumbnail when storing the analysis fails', async () => {
    storageMocks.updateMedia.mockRejectedValue(new Error('metadata failed'))

    await expect(mediaLibraryService.importFile(makeFile())).rejects.toThrow('metadata failed')

    expect(storageMocks.deleteMedia).toHaveBeenCalledWith('server-1')
    expect(storageMocks.deleteThumbnailsByMediaId).toHaveBeenCalledWith('server-1')
  })
})

describe('importFiles', () => {
  it('imports each file in order and skips one that fails', async () => {
    storageMocks.importMediaSource
      .mockResolvedValueOnce(makeMedia({ id: 'a' }))
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce(makeMedia({ id: 'c' }))
    const onFileStart = vi.fn()

    const results = await mediaLibraryService.importFiles(
      [makeFile('a.mp4'), makeFile('b.mp4'), makeFile('c.mp4')],
      { projectId: 'project-1', onFileStart },
    )

    expect(results.map((media) => media.id)).toEqual(['a', 'c'])
    expect(onFileStart.mock.calls).toEqual([
      [1, 3, 'a.mp4'],
      [2, 3, 'b.mp4'],
      [3, 3, 'c.mp4'],
    ])
  })

  it('stops the batch when aborted instead of swallowing the abort', async () => {
    const controller = new AbortController()
    controller.abort()

    await expect(
      mediaLibraryService.importFiles([makeFile()], { signal: controller.signal }),
    ).rejects.toThrow()
    expect(storageMocks.importMediaSource).not.toHaveBeenCalled()
  })
})

describe('relinkMediaFile', () => {
  beforeEach(() => {
    storageMocks.getMedia.mockResolvedValue(
      makeMedia({ tags: ['keep'], attribution: { provider: 'P' } }),
    )
    storageMocks.replaceMediaSource.mockResolvedValue(makeMedia({ contentHash: 'sha256:bbb' }))
  })

  it('replaces the source bytes and returns the refreshed media', async () => {
    const file = makeFile('new.mp4')
    const onProgress = vi.fn()

    const result = await mediaLibraryService.relinkMediaFile('media-1', file, { onProgress })

    expect(storageMocks.replaceMediaSource).toHaveBeenCalledWith('media-1', file, {
      mimeType: 'video/mp4',
      onProgress,
    })
    expect(storageMocks.updateMedia).toHaveBeenCalledWith(
      'media-1',
      expect.objectContaining({ width: 1280, height: 720, fps: 24 }),
    )
    expect(result.id).toBe('media-1')
  })

  it('discards every derivative of the previous bytes', async () => {
    await mediaLibraryService.relinkMediaFile('media-1', makeFile())

    expect(storageMocks.deleteThumbnailsByMediaId).toHaveBeenCalledWith('media-1')
    expect(cacheMocks.gifFrameCache.clearMedia).toHaveBeenCalledWith('media-1')
    expect(cacheMocks.filmstripCache.clearMedia).toHaveBeenCalledWith('media-1')
    expect(cacheMocks.waveformCache.clearMedia).toHaveBeenCalledWith('media-1')
    expect(compositionRuntimeMocks.deletePreviewAudioConform).toHaveBeenCalled()
    expect(proxyMocks.deleteProxy).toHaveBeenCalledWith('media-1', 'proxy-media-1')
  })

  it('never replaces a working source with an unreadable file', async () => {
    validationMocks.validateMediaFileContent.mockResolvedValue({ valid: false, error: 'Corrupt' })

    await expect(mediaLibraryService.relinkMediaFile('media-1', makeFile())).rejects.toThrow(
      'Corrupt',
    )

    expect(storageMocks.replaceMediaSource).not.toHaveBeenCalled()
  })

  it('fails for a media that does not exist', async () => {
    storageMocks.getMedia.mockResolvedValue(undefined)

    await expect(mediaLibraryService.relinkMediaFile('gone', makeFile())).rejects.toThrow(
      'Media not found: gone',
    )
    expect(storageMocks.replaceMediaSource).not.toHaveBeenCalled()
  })
})

describe('source access', () => {
  it('getMediaUrl returns the direct, hash-versioned server URL', async () => {
    storageMocks.getMedia.mockResolvedValue(makeMedia())

    await expect(mediaLibraryService.getMediaUrl('media-1')).resolves.toBe('/media/media-1?v=sha256:aaa')
    await expect(mediaLibraryService.getMediaUrl(makeMedia({ contentHash: 'sha256:zzz' }))).resolves.toBe(
      '/media/media-1?v=sha256:zzz',
    )
  })

  it('getMediaUrl is null for an unknown media', async () => {
    storageMocks.getMedia.mockResolvedValue(undefined)

    await expect(mediaLibraryService.getMediaUrl('gone')).resolves.toBeNull()
  })

  it('getMediaFile fetches the bytes and stamps the recorded MIME type', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('payload')))
    storageMocks.getMedia.mockResolvedValue(makeMedia())

    const blob = await mediaLibraryService.getMediaFile('media-1')

    expect(fetch).toHaveBeenCalledWith('/media/media-1?v=sha256:aaa')
    expect(blob?.type).toBe('video/mp4')
    expect(await blob?.text()).toBe('payload')
    vi.unstubAllGlobals()
  })

  it('getMediaFile is null when the server no longer has the bytes, and throws on other failures', async () => {
    storageMocks.getMedia.mockResolvedValue(makeMedia())

    vi.stubGlobal('fetch', vi.fn(async () => new Response('', { status: 404 })))
    await expect(mediaLibraryService.getMediaFile('media-1')).resolves.toBeNull()

    vi.stubGlobal('fetch', vi.fn(async () => new Response('', { status: 500 })))
    await expect(mediaLibraryService.getMediaFile('media-1')).rejects.toThrow(/500/)
    vi.unstubAllGlobals()
  })
})

describe('deleting media', () => {
  beforeEach(() => {
    storageMocks.getMedia.mockResolvedValue(makeMedia())
  })

  it('keeps media that another project still uses', async () => {
    storageMocks.getProjectsUsingMedia.mockResolvedValue(['other-project'])

    await mediaLibraryService.deleteMediaFromProject('project-1', 'media-1')

    expect(storageMocks.removeMediaFromProject).toHaveBeenCalledWith('project-1', 'media-1')
    expect(storageMocks.deleteMedia).not.toHaveBeenCalled()
    expect(proxyMocks.deleteProxy).not.toHaveBeenCalled()
  })

  it('deletes the server media and every cache once no project references it', async () => {
    await mediaLibraryService.deleteMediaFromProject('project-1', 'media-1')

    expect(storageMocks.deleteMedia).toHaveBeenCalledWith('media-1')
    expect(storageMocks.deleteThumbnailsByMediaId).toHaveBeenCalledWith('media-1')
    expect(cacheMocks.waveformCache.clearMedia).toHaveBeenCalledWith('media-1')
    expect(proxyMocks.deleteProxy).toHaveBeenCalledWith('media-1', 'proxy-media-1')
  })

  it('global delete unlinks every project, then deletes server media', async () => {
    storageMocks.getProjectsUsingMedia.mockResolvedValue(['p1', 'p2'])

    await mediaLibraryService.deleteMedia('media-1')

    expect(storageMocks.removeMediaFromProject.mock.calls).toEqual([
      ['p1', 'media-1'],
      ['p2', 'media-1'],
    ])
    expect(storageMocks.deleteMedia).toHaveBeenCalledWith('media-1')
  })

  it('fails for a media that does not exist', async () => {
    storageMocks.getMedia.mockResolvedValue(undefined)

    await expect(mediaLibraryService.deleteMedia('gone')).rejects.toThrow('Media not found: gone')
  })
})

describe('copyMediaToProject', () => {
  it('links the existing server media instead of copying bytes', async () => {
    storageMocks.getMedia.mockResolvedValue(makeMedia())

    await mediaLibraryService.copyMediaToProject('media-1', 'project-2')

    expect(storageMocks.associateMediaWithProject).toHaveBeenCalledWith('project-2', 'media-1')
    expect(storageMocks.importMediaSource).not.toHaveBeenCalled()
  })

  it('fails for a media that does not exist', async () => {
    storageMocks.getMedia.mockResolvedValue(undefined)

    await expect(mediaLibraryService.copyMediaToProject('gone', 'project-2')).rejects.toThrow(
      'Media not found: gone',
    )
  })
})

describe('generated media', () => {
  it('uploads a generated audio take with its tags and links it to the project', async () => {
    mediaProcessorMocks.processMedia.mockResolvedValue({
      metadata: { type: 'audio', duration: 3, codec: 'mp3', bitrate: 128 },
      thumbnail: undefined,
    })
    const file = makeFile('tone.mp3', 'audio/mpeg')

    await mediaLibraryService.importGeneratedAudio(file, 'project-1', { tags: ['sfx'] })

    expect(storageMocks.importMediaSource).toHaveBeenCalledWith(
      file,
      expect.objectContaining({ projectId: 'project-1', mimeType: 'audio/mpeg' }),
    )
    expect(storageMocks.updateMedia).toHaveBeenCalledWith(
      'server-1',
      expect.objectContaining({ duration: 3, codec: 'mp3', tags: ['sfx'] }),
    )
  })

  it('keeps the caller-supplied frame rate of a generated video over the probe', async () => {
    const file = makeFile('interp.mp4')

    await mediaLibraryService.importGeneratedVideo(file, 'project-1', { fps: 120 })

    expect(storageMocks.updateMedia).toHaveBeenCalledWith(
      'server-1',
      expect.objectContaining({ fps: 120 }),
    )
  })

  it('refuses a generated image that is not an image', async () => {
    await expect(
      mediaLibraryService.importGeneratedImage(makeFile('x.mp4', 'video/mp4'), 'project-1'),
    ).rejects.toThrow(/must be an image/)
    expect(storageMocks.importMediaSource).not.toHaveBeenCalled()
  })

  it('requires a project for generated media', async () => {
    await expect(mediaLibraryService.importGeneratedAudio(makeFile('a.mp3', 'audio/mpeg'), '')).rejects.toThrow(
      'No project selected',
    )
  })
})
