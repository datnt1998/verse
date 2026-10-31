// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { TimelineTrack, VideoItem } from '@editor/types/timeline'

const renderCompositionMock = vi.fn()
const readWorkspaceBlobMock = vi.fn()
const mirrorBlobToWorkspaceMock = vi.fn()
const resolveMediaUrlsMock = vi.fn()
let mediaByIdMock: Record<
  string,
  {
    duration: number
    fps: number
    fileSize?: number
    fileLastModified?: number
    width?: number
    height?: number
    contentHash?: string
  }
> = {}

vi.mock('../deps/export-contract', () => ({
  importCanvasRenderOrchestrator: vi.fn(async () => ({
    renderComposition: renderCompositionMock,
  })),
}))

vi.mock('@editor/infrastructure/storage/workspace-fs/cache-mirror', () => ({
  mirrorBlobToWorkspace: mirrorBlobToWorkspaceMock,
  readWorkspaceBlob: readWorkspaceBlobMock,
}))

vi.mock('../deps/media-library-resolver', () => ({
  resolveMediaUrls: resolveMediaUrlsMock,
}))

vi.mock('../deps/media-library-store', () => ({
  useMediaLibraryStore: {
    getState: () => ({ mediaById: mediaByIdMock }),
  },
}))

function makeVideoItem(overrides: Partial<VideoItem> = {}): VideoItem {
  return {
    id: 'video-1',
    type: 'video',
    trackId: 'track-1',
    from: 0,
    durationInFrames: 60,
    label: 'clip.mp4',
    src: 'blob:video',
    mediaId: 'media-1',
    sourceStart: 0,
    sourceEnd: 60,
    sourceFps: 30,
    ...overrides,
  }
}

describe('reverseConformService', () => {
  beforeEach(() => {
    renderCompositionMock.mockReset()
    readWorkspaceBlobMock.mockReset()
    mirrorBlobToWorkspaceMock.mockReset()
    resolveMediaUrlsMock.mockReset()
    mediaByIdMock = {}

    readWorkspaceBlobMock.mockResolvedValue(null)
    mirrorBlobToWorkspaceMock.mockResolvedValue(undefined)
    resolveMediaUrlsMock.mockImplementation(async (tracks) => tracks)
    renderCompositionMock.mockResolvedValue({
      blob: {
        arrayBuffer: async () => new ArrayBuffer(0),
        type: 'video/mp4',
      },
    })
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:reverse')
  })

  it('renders the whole known source instead of only the selected clip range', async () => {
    const { reverseConformService } = await import('./reverse-conform-service')

    await reverseConformService.prepareVideo(
      makeVideoItem({
        durationInFrames: 0,
        sourceStart: 30,
        sourceEnd: 90,
      }),
      30,
    )

    const renderRequest = renderCompositionMock.mock.calls[0]?.[0]
    expect(renderRequest.composition.durationInFrames).toBe(90)
    expect(renderRequest.composition.tracks[0].items[0]).toMatchObject({
      durationInFrames: 90,
      sourceStart: 0,
      sourceEnd: 90,
      speed: 1,
    })
  })

  it('falls back to media library duration when optimistic source bounds are zero', async () => {
    mediaByIdMock = {
      'media-1': {
        duration: 2,
        fps: 30,
      },
    }
    const { reverseConformService } = await import('./reverse-conform-service')

    await reverseConformService.prepareVideo(
      makeVideoItem({
        durationInFrames: 0,
        sourceStart: 0,
        sourceEnd: 0,
        sourceDuration: 0,
      }),
      30,
    )

    const renderRequest = renderCompositionMock.mock.calls[0]?.[0]
    expect(renderRequest.composition.durationInFrames).toBe(60)
    expect(renderRequest.composition.tracks[0].items[0]).toMatchObject({
      durationInFrames: 60,
      sourceStart: 0,
      sourceEnd: 60,
      sourceDuration: 60,
    })
  })

  it('allows mediaId-only clips to resolve their source during reverse preparation', async () => {
    mediaByIdMock = {
      'media-1': {
        duration: 2,
        fps: 30,
      },
    }
    const { reverseConformService } = await import('./reverse-conform-service')
    resolveMediaUrlsMock.mockImplementationOnce(async (tracks: TimelineTrack[]) =>
      tracks.map((track) => ({
        ...track,
        items: track.items.map((item) =>
          item.type === 'video' ? { ...item, src: 'blob:resolved-video' } : item,
        ),
      })),
    )

    await reverseConformService.prepareVideo(
      makeVideoItem({
        src: '',
        durationInFrames: 0,
        sourceStart: 0,
        sourceEnd: 0,
        sourceDuration: 0,
      }),
      30,
    )

    expect(resolveMediaUrlsMock).toHaveBeenCalled()
    expect(renderCompositionMock).toHaveBeenCalled()
  })

  it('shares one source-level preview across differently trimmed instances', async () => {
    mediaByIdMock = {
      'media-1': {
        duration: 10,
        fps: 30,
        fileSize: 1234,
        fileLastModified: 5678,
        width: 1920,
        height: 1080,
      },
    }
    const cachedBlob = new Blob(['reverse'], { type: 'video/mp4' })
    readWorkspaceBlobMock.mockResolvedValue(cachedBlob)
    const { reverseConformService } = await import('./reverse-conform-service')

    const first = await reverseConformService.prepareVideo(
      makeVideoItem({ id: 'clip-a', sourceStart: 0, sourceEnd: 60 }),
      30,
    )
    const second = await reverseConformService.prepareVideo(
      makeVideoItem({ id: 'clip-b', sourceStart: 120, sourceEnd: 210, speed: 2 }),
      60,
    )

    expect(first.key).toBe(second.key)
    expect(first.path).toBe(second.path)
    expect(first.src).toBe(second.src)
    expect(renderCompositionMock).not.toHaveBeenCalled()
  })
  it('stores a rendered conform only in the versioned cache (no second copy)', async () => {
    mediaByIdMock = { 'media-1': { duration: 2, fps: 30 } }
    const { reverseConformService } = await import('./reverse-conform-service')

    const result = await reverseConformService.prepareVideo(
      makeVideoItem({ durationInFrames: 0, sourceStart: 0, sourceEnd: 0, sourceDuration: 0 }),
      30,
    )

    expect(mirrorBlobToWorkspaceMock).toHaveBeenCalledTimes(1)
    const [segments] = mirrorBlobToWorkspaceMock.mock.calls[0]!
    expect(segments[0]).toBe('media')
    expect(segments[1]).toBe('media-1')
    expect(result.path).toBe(segments.join('/'))
  })

  it('treats an evicted cache entry as a miss and re-renders', async () => {
    mediaByIdMock = { 'media-1': { duration: 2, fps: 30 } }
    readWorkspaceBlobMock.mockResolvedValue(null)
    const { reverseConformService } = await import('./reverse-conform-service')

    await reverseConformService.prepareVideo(
      makeVideoItem({ durationInFrames: 0, sourceStart: 0, sourceEnd: 0, sourceDuration: 0 }),
      30,
    )

    expect(renderCompositionMock).toHaveBeenCalledTimes(1)
  })

  it('drops the shared object URL of a media whose server content changed', async () => {
    mediaByIdMock = {
      'media-1': { duration: 10, fps: 30, fileSize: 1, fileLastModified: 2, width: 4, height: 4 },
    }
    readWorkspaceBlobMock.mockResolvedValue(new Blob(['reverse'], { type: 'video/mp4' }))
    const revokeSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined)
    const { reverseConformService } = await import('./reverse-conform-service')
    const { observeMediaContentHash } = await import(
      '@editor/infrastructure/storage/workspace-fs/cache-identity'
    )

    await reverseConformService.prepareVideo(makeVideoItem({ id: 'clip-a' }), 30)
    observeMediaContentHash('media-1', 'hash-1')
    expect(revokeSpy).not.toHaveBeenCalled()

    observeMediaContentHash('media-1', 'hash-2')
    expect(revokeSpy).toHaveBeenCalledWith('blob:reverse')
  })
})
