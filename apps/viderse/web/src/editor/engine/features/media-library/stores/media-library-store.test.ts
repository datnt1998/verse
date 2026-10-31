// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { MediaMetadata } from '@editor/types/storage'

const mediaLibraryServiceMocks = vi.hoisted(() => ({
  getMediaForProject: vi.fn(),
  getMediaFile: vi.fn(),
  prefetchThumbnails: vi.fn(async () => {}),
}))

const workspaceHealthMocks = vi.hoisted(() => ({
  scanWorkspaceMediaHealth: vi.fn(),
}))

const proxyStatusListenerRef = vi.hoisted(() => ({
  current: null as
    | ((
        mediaId: string,
        status: 'generating' | 'ready' | 'error' | 'idle',
        progress?: number,
      ) => void)
    | null,
}))


const proxyServiceMocks = vi.hoisted(() => ({
  canGenerateProxy: vi.fn(),
  clearProxyKey: vi.fn(),
  hasProxy: vi.fn(),
  setProxyKey: vi.fn(),
  loadExistingProxies: vi.fn(),
  generateProxy: vi.fn(),
  onStatusChange: vi.fn((listener) => {
    proxyStatusListenerRef.current = listener
  }),
  setMediaResolver: vi.fn(),
  setFilmstripPrewarm: vi.fn(),
}))

const indexedDbMocks = vi.hoisted(() => ({
  getTranscriptMediaIds: vi.fn(),
}))

const loggerEventMocks = vi.hoisted(() => ({
  set: vi.fn(),
  merge: vi.fn(),
  success: vi.fn(),
  failure: vi.fn(),
}))

const loggerMocks = vi.hoisted(() => ({
  warn: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
  debug: vi.fn(),
  event: vi.fn(),
  startEvent: vi.fn(() => loggerEventMocks),
  child: vi.fn(),
  setLevel: vi.fn(),
}))

vi.mock('../services/media-library-service', () => ({
  mediaLibraryService: mediaLibraryServiceMocks,
}))

vi.mock('./media-library-service-access', () => ({
  loadMediaLibraryService: vi.fn(async () => ({
    mediaLibraryService: mediaLibraryServiceMocks,
  })),
}))

vi.mock('../services/proxy-service', () => ({
  proxyService: proxyServiceMocks,
}))


vi.mock('../services/background-media-work', async () => {
  const { createBackgroundMediaWorkMocks } =
    await import('../test-utils/background-media-work-test-mocks')
  return createBackgroundMediaWorkMocks(vi)
})

vi.mock('../utils/proxy-key', () => ({
  getSharedProxyKey: vi.fn((media: { id: string }) => `proxy-${media.id}`),
}))

vi.mock('../utils/workspace-health', () => ({
  scanWorkspaceMediaHealth: workspaceHealthMocks.scanWorkspaceMediaHealth,
}))

vi.mock('@editor/infrastructure/storage', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  getTranscriptMediaIds: indexedDbMocks.getTranscriptMediaIds,
}))

vi.mock('./media-import-actions', () => ({
  createImportActions: vi.fn(() => ({})),
}))

vi.mock('./media-delete-actions', () => ({
  createDeleteActions: vi.fn(() => ({})),
}))

vi.mock('./media-relinking-actions', () => ({
  createRelinkingActions: vi.fn(() => ({})),
}))

vi.mock('@editor/shared/logging/logger', () => ({
  createOperationId: vi.fn(() => 'test-op-id'),
  createLogger: vi.fn(() => loggerMocks),
}))

import { useMediaLibraryStore } from './media-library-store'

function makeMedia(overrides: Partial<MediaMetadata> = {}): MediaMetadata {
  return {
    id: 'media-1',
    storageType: 'workspace',
    fileName: 'clip.mp4',
    fileSize: 1024,
    mimeType: 'video/mp4',
    duration: 5,
    width: 3840,
    height: 2160,
    fps: 30,
    codec: 'h264',
    bitrate: 5000,
    tags: [],
    createdAt: 1,
    updatedAt: 1,
    ...overrides,
  }
}

function resetStore(): void {
  useMediaLibraryStore.setState({
    currentProjectId: null,
    mediaItems: [],
    mediaById: {},
    isLoading: false,
    importingIds: [],
    error: null,
    errorLink: null,
    notification: null,
    selectedMediaIds: [],
    selectedCompositionIds: [],
    searchQuery: '',
    filterByType: null,
    sortBy: 'date',
    viewMode: 'grid',
    mediaItemSize: 1,
    brokenMediaIds: [],
    brokenMediaInfo: new Map(),
    showMissingMediaDialog: false,
    orphanedClips: [],
    showOrphanedClipsDialog: false,
    unsupportedCodecFiles: [],
    showUnsupportedCodecDialog: false,
    unsupportedCodecResolver: null,
    proxyStatus: new Map(),
    proxyProgress: new Map(),
    transcriptStatus: new Map(),
    transcriptProgress: new Map(),
  })
}

describe('useMediaLibraryStore', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    workspaceHealthMocks.scanWorkspaceMediaHealth.mockResolvedValue({ healthyIds: [], broken: [] })
    resetStore()
  })

  it('clears loading without fetching when no project is selected', async () => {
    useMediaLibraryStore.setState({ isLoading: true, currentProjectId: null })

    await useMediaLibraryStore.getState().loadMediaItems()

    expect(useMediaLibraryStore.getState().isLoading).toBe(false)
    expect(mediaLibraryServiceMocks.getMediaForProject).not.toHaveBeenCalled()
  })

  it('loads media, transcript availability, and stale proxies for the current project', async () => {
    const video = makeMedia({ id: 'video-1', fileName: 'video.mp4' })
    const audio = makeMedia({
      id: 'audio-1',
      fileName: 'audio.mp3',
      mimeType: 'audio/mpeg',
      width: 0,
      height: 0,
    })

    mediaLibraryServiceMocks.getMediaForProject.mockResolvedValue([video, audio])
    indexedDbMocks.getTranscriptMediaIds.mockResolvedValue(new Set(['video-1']))
    proxyServiceMocks.canGenerateProxy.mockImplementation((mimeType: string) =>
      mimeType.startsWith('video/'),
    )
    proxyServiceMocks.hasProxy.mockReturnValue(false)
    proxyServiceMocks.loadExistingProxies.mockResolvedValue(['video-1'])

    useMediaLibraryStore.setState({ currentProjectId: 'project-1' })

    await useMediaLibraryStore.getState().loadMediaItems()

    const state = useMediaLibraryStore.getState()
    expect(state.isLoading).toBe(false)
    expect(state.mediaItems).toEqual([video, audio])
    expect(state.mediaById['video-1']).toEqual(video)
    expect(state.mediaById['audio-1']).toEqual(audio)
    expect(state.transcriptStatus.get('video-1')).toBe('ready')
    expect(state.transcriptStatus.get('audio-1')).toBe('idle')
    expect(proxyServiceMocks.setProxyKey).toHaveBeenCalledWith('video-1', 'proxy-video-1')
    expect(proxyServiceMocks.loadExistingProxies).toHaveBeenCalledWith(['video-1'])
  })

  it('falls back to idle transcript status when transcript lookup fails', async () => {
    const video = makeMedia({ id: 'video-1' })
    mediaLibraryServiceMocks.getMediaForProject.mockResolvedValue([video])
    indexedDbMocks.getTranscriptMediaIds.mockRejectedValue(new Error('boom'))
    proxyServiceMocks.canGenerateProxy.mockReturnValue(true)
    proxyServiceMocks.loadExistingProxies.mockResolvedValue([])

    useMediaLibraryStore.setState({ currentProjectId: 'project-1' })

    await useMediaLibraryStore.getState().loadMediaItems()

    const state = useMediaLibraryStore.getState()
    expect(state.transcriptStatus.get('video-1')).toBe('idle')
    expect(proxyServiceMocks.loadExistingProxies).toHaveBeenCalledWith(['video-1'])
    expect(proxyServiceMocks.generateProxy).not.toHaveBeenCalled()
  })

  it('clears proxy status and progress when proxy generation is cancelled', () => {
    useMediaLibraryStore.getState().setProxyStatus('media-1', 'generating')
    useMediaLibraryStore.getState().setProxyProgress('media-1', 0.5)

    proxyStatusListenerRef.current?.('media-1', 'idle')

    const state = useMediaLibraryStore.getState()
    expect(state.proxyStatus.has('media-1')).toBe(false)
    expect(state.proxyProgress.has('media-1')).toBe(false)
  })

  it('hydrates media records, scans health for the project, and no longer mirrors OPFS', async () => {
    const video = makeMedia({ id: 'video-1' })
    mediaLibraryServiceMocks.getMediaForProject.mockResolvedValue([video])
    indexedDbMocks.getTranscriptMediaIds.mockResolvedValue(new Set())
    proxyServiceMocks.canGenerateProxy.mockReturnValue(false)

    useMediaLibraryStore.setState({ currentProjectId: 'project-1' })
    await useMediaLibraryStore.getState().loadMediaItems()

    expect(useMediaLibraryStore.getState().mediaItems).toEqual([video])
    expect(workspaceHealthMocks.scanWorkspaceMediaHealth).toHaveBeenCalledWith(
      [video],
      'project-1',
    )
    expect(mediaLibraryServiceMocks.prefetchThumbnails).toHaveBeenCalledWith([
      { id: 'video-1', thumbnailId: undefined },
    ])
  })

  it('clears canonical media records and per-project UI state when switching projects', () => {
    const video = makeMedia({ id: 'video-1' })
    useMediaLibraryStore.setState({
      currentProjectId: 'project-1',
      mediaItems: [video],
      mediaById: { 'video-1': video },
      selectedMediaIds: ['video-1'],
    })

    useMediaLibraryStore.getState().setCurrentProject('project-2')

    const state = useMediaLibraryStore.getState()
    expect(state.currentProjectId).toBe('project-2')
    expect(state.mediaItems).toEqual([])
    expect(state.mediaById).toEqual({})
    expect(state.selectedMediaIds).toEqual([])
    expect(state.isLoading).toBe(true)
    expect(proxyServiceMocks.clearProxyKey).toHaveBeenCalledWith('video-1')
  })

  it('prepends completed media records via media.add', () => {
    const first = makeMedia({ id: 'media-a', fileName: 'a.mp4' })
    const second = makeMedia({ id: 'media-b', fileName: 'b.mp4' })

    useMediaLibraryStore.getState().prependMediaItem(first)
    useMediaLibraryStore.getState().prependMediaItem(second)

    const state = useMediaLibraryStore.getState()
    expect(state.mediaItems.map((media) => media.id)).toEqual(['media-b', 'media-a'])
    expect(state.mediaById['media-a']).toEqual(first)
    expect(state.mediaById['media-b']).toEqual(second)
  })

  it('updates captions on the stored record via media.update', () => {
    const video = makeMedia({ id: 'video-1' })
    const other = makeMedia({ id: 'video-2' })
    useMediaLibraryStore.setState({ mediaItems: [video, other], mediaById: {} })

    useMediaLibraryStore.getState().updateMediaCaptions('video-1', [{ timeSec: 1, text: 'cảnh một' }])

    const state = useMediaLibraryStore.getState()
    expect(state.mediaById['video-1']?.aiCaptions).toEqual([{ timeSec: 1, text: 'cảnh một' }])
    expect(state.mediaById['video-1']?.updatedAt).toBeGreaterThan(video.updatedAt)
    expect(state.mediaById['video-2']).toEqual(other)
  })

  it('ignores captions for media that was already removed', () => {
    const video = makeMedia({ id: 'video-1' })
    useMediaLibraryStore.setState({ mediaItems: [video] })

    expect(() =>
      useMediaLibraryStore.getState().updateMediaCaptions('gone', [{ timeSec: 0, text: 'x' }]),
    ).not.toThrow()

    expect(useMediaLibraryStore.getState().mediaItems).toEqual([video])
  })

  it('marks broken and healthy media from the server health scan', async () => {
    const video = makeMedia({ id: 'video-1' })
    const audio = makeMedia({ id: 'audio-1', mimeType: 'audio/mpeg' })
    const markMediaBroken = vi.fn()
    const markMediaHealthy = vi.fn()
    const original = useMediaLibraryStore.getState()
    workspaceHealthMocks.scanWorkspaceMediaHealth.mockResolvedValue({
      healthyIds: ['audio-1'],
      broken: [{ mediaId: 'video-1', fileName: 'clip.mp4', errorType: 'file_missing' }],
    })

    useMediaLibraryStore.setState({
      currentProjectId: 'project-1',
      mediaItems: [video, audio],
      markMediaBroken,
      markMediaHealthy,
    })

    try {
      await useMediaLibraryStore.getState().scanMediaHealth()
    } finally {
      useMediaLibraryStore.setState({
        markMediaBroken: original.markMediaBroken,
        markMediaHealthy: original.markMediaHealthy,
      })
    }

    expect(workspaceHealthMocks.scanWorkspaceMediaHealth).toHaveBeenCalledWith(
      [video, audio],
      'project-1',
    )
    expect(markMediaHealthy).toHaveBeenCalledWith('audio-1')
    expect(markMediaBroken).toHaveBeenCalledWith('video-1', {
      mediaId: 'video-1',
      fileName: 'clip.mp4',
      errorType: 'file_missing',
    })
    expect(useMediaLibraryStore.getState().isScanningMediaHealth).toBe(false)
  })
})
