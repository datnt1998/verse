// @vitest-environment node

import { describe, expect, it, beforeEach, vi, type Mock } from 'vitest'
import { createRelinkingActions } from './media-relinking-actions'
import { useMediaLibraryStore } from './media-library-store'
import { registerMediaRelinkingTimelineActions } from './media-relinking-timeline-actions'
import { blobUrlManager } from '@editor/infrastructure/browser/blob-url-manager'
import type { MediaLibraryState, MediaLibraryActions, BrokenMediaInfo } from '../types'
import type { MediaMetadata } from '@editor/types/storage'
import type { TimelineItem } from '@editor/types/timeline'

// Mock external dependencies
vi.mock('../services/media-library-service', () => ({
  mediaLibraryService: {
    relinkMediaFile: vi.fn(),
    getMedia: vi.fn(),
  },
}))

// The command executor binds to the real media-library store, so keep that store's
// heavy collaborators out of the module graph.
vi.mock('../services/proxy-service', () => ({
  proxyService: {
    canGenerateProxy: vi.fn(),
    clearProxyKey: vi.fn(),
    setProxyKey: vi.fn(),
    loadExistingProxies: vi.fn(),
    onStatusChange: vi.fn(),
    setMediaResolver: vi.fn(),
    setFilmstripPrewarm: vi.fn(),
  },
}))

vi.mock('../services/background-media-work', async () => {
  const { createBackgroundMediaWorkMocks } =
    await import('../test-utils/background-media-work-test-mocks')
  return createBackgroundMediaWorkMocks(vi)
})

vi.mock('./media-import-actions', () => ({
  createImportActions: vi.fn(() => ({})),
}))

vi.mock('./media-delete-actions', () => ({
  createDeleteActions: vi.fn(() => ({})),
}))

vi.mock('./media-library-service-access', () => ({
  loadMediaLibraryService: vi.fn(async () => import('../services/media-library-service')),
}))

// Import after mocks
import { mediaLibraryService } from '../services/media-library-service'

// Helpers
type RelinkingState = Partial<MediaLibraryState> & Partial<MediaLibraryActions>
type RelinkingUpdater =
  | Partial<MediaLibraryState>
  | ((state: MediaLibraryState & MediaLibraryActions) => Partial<MediaLibraryState>)

function createMockMediaMetadata(id: string, fileName: string): MediaMetadata {
  return {
    id,
    storageType: 'workspace',
    fileName,
    fileSize: 1,
    mimeType: fileName.endsWith('.mp3') ? 'audio/mpeg' : 'video/mp4',
    duration: 1,
    width: 1920,
    height: 1080,
    fps: 30,
    codec: 'h264',
    bitrate: 1,
    tags: [],
    createdAt: 0,
    updatedAt: 0,
  }
}

function createMockTimelineItem(
  overrides: Partial<TimelineItem> & {
    id: string
    type: TimelineItem['type']
    trackId: string
    label: string
  },
): TimelineItem {
  const { id, type, trackId, label, from, durationInFrames, ...rest } = overrides
  return {
    id,
    type,
    trackId,
    from: from ?? 0,
    durationInFrames: durationInFrames ?? 30,
    label,
    mediaId: 'media-1',
    src: 'blob:test',
    ...rest,
  } as TimelineItem
}

function applyStateUpdate(state: RelinkingState, updater: RelinkingUpdater): RelinkingState {
  if (typeof updater === 'function') {
    return {
      ...state,
      ...updater(state as MediaLibraryState & MediaLibraryActions),
    }
  }

  return { ...state, ...updater }
}

function createMockState(): MediaLibraryState & MediaLibraryActions {
  const state: RelinkingState = {
    mediaItems: [
      createMockMediaMetadata('media-1', 'video.mp4'),
      createMockMediaMetadata('media-2', 'audio.mp3'),
    ],
    brokenMediaIds: ['media-1'],
    brokenMediaInfo: new Map([
      [
        'media-1',
        { mediaId: 'media-1', fileName: 'video.mp4', errorType: 'file_missing' } as BrokenMediaInfo,
      ],
    ]),
    showNotification: vi.fn(),
    markMediaHealthy: vi.fn(),
    showMissingMediaDialog: false,
    orphanedClips: [],
    showOrphanedClipsDialog: false,
  }
  return state as MediaLibraryState & MediaLibraryActions
}

let blobUrlCounter = 0

function makeFile(name: string): File {
  return new File([name], name)
}

const updateProjectItem = vi.fn()
const removeProjectItems = vi.fn()
const getItemsState = vi.fn<
  () => {
    items: TimelineItem[]
    itemById: Record<string, TimelineItem | undefined>
  }
>(() => ({
  items: [],
  itemById: {},
}))
const getSynchronizedLinkedItems = vi.fn<(items: TimelineItem[], itemId: string) => TimelineItem[]>(
  () => [],
)
let unregisterTimelineActions: (() => void) | null = null

beforeEach(() => {
  vi.clearAllMocks()
  useMediaLibraryStore.setState({
    mediaItems: [
      createMockMediaMetadata('media-1', 'video.mp4'),
      createMockMediaMetadata('media-2', 'audio.mp3'),
    ],
  })
  unregisterTimelineActions?.()
  unregisterTimelineActions = registerMediaRelinkingTimelineActions({
    updateProjectItem,
    removeProjectItems,
    getItemsState,
    getFps: () => 30,
    getSynchronizedLinkedItems,
  })
  blobUrlManager.releaseAll()
  blobUrlCounter = 0

  updateProjectItem.mockReturnValue(true)
  removeProjectItems.mockReturnValue(true)
  getSynchronizedLinkedItems.mockReturnValue([])
  getItemsState.mockReturnValue({
    items: [],
    itemById: {},
  })

  vi.stubGlobal('URL', {
    ...URL,
    createObjectURL: () => `blob:test-${++blobUrlCounter}`,
    revokeObjectURL: vi.fn(),
  })
})

describe('createRelinkingActions', () => {
  describe('markMediaBroken', () => {
    it('adds a media ID to brokenMediaIds', () => {
      let currentState: RelinkingState = {
        brokenMediaIds: [],
        brokenMediaInfo: new Map<string, BrokenMediaInfo>(),
      }
      const set = vi.fn((updater: RelinkingUpdater) => {
        currentState = applyStateUpdate(currentState, updater)
      })
      const get = vi.fn(() => currentState as MediaLibraryState & MediaLibraryActions)

      const actions = createRelinkingActions(set, get)
      actions.markMediaBroken('media-1', {
        mediaId: 'media-1',
        fileName: 'video.mp4',
        errorType: 'file_missing',
      })

      expect(set).toHaveBeenCalled()
      expect(currentState.brokenMediaIds).toContain('media-1')
      expect(currentState.brokenMediaInfo!.has('media-1')).toBe(true)
    })

    it('is idempotent — does not duplicate entries', () => {
      let currentState: RelinkingState = {
        brokenMediaIds: ['media-1'],
        brokenMediaInfo: new Map([
          ['media-1', { mediaId: 'media-1', fileName: 'video.mp4', errorType: 'file_missing' }],
        ]),
      }
      const set = vi.fn((updater: RelinkingUpdater) => {
        currentState = applyStateUpdate(currentState, updater)
      })
      const get = vi.fn(() => currentState as MediaLibraryState & MediaLibraryActions)

      const actions = createRelinkingActions(set, get)
      actions.markMediaBroken('media-1', {
        mediaId: 'media-1',
        fileName: 'video.mp4',
        errorType: 'file_missing',
      })

      // set was called but returned same state (no mutation)
      expect(currentState.brokenMediaIds).toEqual(['media-1'])
    })
  })

  describe('markMediaHealthy', () => {
    it('removes a media ID from brokenMediaIds', () => {
      let currentState: RelinkingState = {
        brokenMediaIds: ['media-1', 'media-2'],
        brokenMediaInfo: new Map([
          ['media-1', { mediaId: 'media-1', fileName: 'a.mp4', errorType: 'file_missing' }],
          ['media-2', { mediaId: 'media-2', fileName: 'b.mp4', errorType: 'file_missing' }],
        ]),
      }
      const set = vi.fn((updater: RelinkingUpdater) => {
        currentState = applyStateUpdate(currentState, updater)
      })
      const get = vi.fn(() => currentState as MediaLibraryState & MediaLibraryActions)

      const actions = createRelinkingActions(set, get)
      actions.markMediaHealthy('media-1')

      expect(currentState.brokenMediaIds).toEqual(['media-2'])
      expect(currentState.brokenMediaInfo!.has('media-1')).toBe(false)
    })
  })

  describe('dismissMissingMediaWarnings', () => {
    it('snoozes dialog warnings without marking broken media healthy', () => {
      let currentState: RelinkingState = {
        brokenMediaIds: ['media-1', 'media-2'],
        brokenMediaInfo: new Map([
          ['media-1', { mediaId: 'media-1', fileName: 'a.mp4', errorType: 'file_missing' }],
          ['media-2', { mediaId: 'media-2', fileName: 'b.mp4', errorType: 'file_missing' }],
        ]),
        dismissedMissingMediaIds: [],
        showMissingMediaDialog: true,
      }
      const set = vi.fn((updater: RelinkingUpdater) => {
        currentState = applyStateUpdate(currentState, updater)
      })
      const get = vi.fn(() => currentState as MediaLibraryState & MediaLibraryActions)

      const actions = createRelinkingActions(set, get)
      actions.dismissMissingMediaWarnings(['media-1', 'media-2'])

      expect(currentState.brokenMediaIds).toEqual(['media-1', 'media-2'])
      expect(currentState.brokenMediaInfo!.has('media-1')).toBe(true)
      expect(currentState.dismissedMissingMediaIds).toEqual(['media-1', 'media-2'])
      expect(currentState.showMissingMediaDialog).toBe(false)
    })
  })

  describe('relinkMedia', () => {
    it('invalidates blob URL cache on successful relink', async () => {
      const updatedMedia = createMockMediaMetadata('media-1', 'relocated.mp4')
      ;(mediaLibraryService.relinkMediaFile as Mock).mockResolvedValue(updatedMedia)

      // Pre-populate a stale blob URL
      blobUrlManager.acquire('media-1', new Blob(['stale']))
      expect(blobUrlManager.has('media-1')).toBe(true)

      const mockState = createMockState()
      const set = vi.fn()
      const get = vi.fn(() => mockState)

      const actions = createRelinkingActions(set, get)
      const result = await actions.relinkMedia('media-1', makeFile('relocated.mp4'))

      expect(result).toBe(true)
      // Blob URL must be invalidated so preview re-fetches
      expect(blobUrlManager.has('media-1')).toBe(false)
    })

    it('uploads the picked file through the media service', async () => {
      const updatedMedia = createMockMediaMetadata('media-1', 'relocated.mp4')
      ;(mediaLibraryService.relinkMediaFile as Mock).mockResolvedValue(updatedMedia)

      const mockState = createMockState()
      const actions = createRelinkingActions(
        vi.fn(),
        vi.fn(() => mockState),
      )
      const file = makeFile('relocated.mp4')
      await actions.relinkMedia('media-1', file)

      expect(mediaLibraryService.relinkMediaFile).toHaveBeenCalledWith('media-1', file)
    })

    it('calls markMediaHealthy after successful relink', async () => {
      const updatedMedia = createMockMediaMetadata('media-1', 'relocated.mp4')
      ;(mediaLibraryService.relinkMediaFile as Mock).mockResolvedValue(updatedMedia)

      const mockState = createMockState()
      const set = vi.fn()
      const get = vi.fn(() => mockState)

      const actions = createRelinkingActions(set, get)
      await actions.relinkMedia('media-1', makeFile('relocated.mp4'))

      expect(mockState.markMediaHealthy).toHaveBeenCalledWith('media-1')
    })

    it('does not invalidate blob URL on failed relink', async () => {
      ;(mediaLibraryService.relinkMediaFile as Mock).mockRejectedValue(new Error('Upload failed'))

      blobUrlManager.acquire('media-1', new Blob(['existing']))

      const mockState = createMockState()
      const set = vi.fn()
      const get = vi.fn(() => mockState)

      const actions = createRelinkingActions(set, get)
      const result = await actions.relinkMedia('media-1', makeFile('relocated.mp4'))

      expect(result).toBe(false)
      expect(mockState.showNotification).toHaveBeenCalledWith({
        type: 'error',
        message: 'Upload failed',
      })
      // Blob URL should still be cached since relink failed
      expect(blobUrlManager.has('media-1')).toBe(true)
      expect(useMediaLibraryStore.getState().mediaItems[0]?.fileName).toBe('video.mp4')
    })

    it('updates mediaItems with the relinked metadata through media.update', async () => {
      const updatedMedia = createMockMediaMetadata('media-1', 'relocated.mp4')
      ;(mediaLibraryService.relinkMediaFile as Mock).mockResolvedValue(updatedMedia)

      const mockState = createMockState()
      const set = vi.fn()
      const get = vi.fn(() => mockState)

      const actions = createRelinkingActions(set, get)
      await actions.relinkMedia('media-1', makeFile('relocated.mp4'))

      const { mediaItems, mediaById } = useMediaLibraryStore.getState()
      expect(mediaItems.find((media) => media.id === 'media-1')?.fileName).toBe('relocated.mp4')
      expect(mediaById['media-1']?.fileName).toBe('relocated.mp4')
      // Other records are untouched
      expect(mediaItems.find((media) => media.id === 'media-2')?.fileName).toBe('audio.mp3')
    })
  })

  describe('relinkMediaBatch', () => {
    it('invalidates blob URLs for all successfully relinked media', async () => {
      ;(mediaLibraryService.relinkMediaFile as Mock)
        .mockResolvedValueOnce(createMockMediaMetadata('media-1', 'a.mp4'))
        .mockResolvedValueOnce(createMockMediaMetadata('media-2', 'b.mp4'))

      blobUrlManager.acquire('media-1', new Blob(['stale-1']))
      blobUrlManager.acquire('media-2', new Blob(['stale-2']))

      const mockState = createMockState()
      mockState.brokenMediaIds = ['media-1', 'media-2']
      const set = vi.fn()
      const get = vi.fn(() => mockState)

      const actions = createRelinkingActions(set, get)
      const result = await actions.relinkMediaBatch([
        { mediaId: 'media-1', file: makeFile('a.mp4') },
        { mediaId: 'media-2', file: makeFile('b.mp4') },
      ])

      expect(result.success).toEqual(['media-1', 'media-2'])
      expect(result.failed).toEqual([])
      expect(blobUrlManager.has('media-1')).toBe(false)
      expect(blobUrlManager.has('media-2')).toBe(false)
      const names = useMediaLibraryStore.getState().mediaItems.map((media) => media.fileName)
      expect(names).toEqual(['a.mp4', 'b.mp4'])
    })

    it('only invalidates blob URLs for successful relinks in mixed batch', async () => {
      ;(mediaLibraryService.relinkMediaFile as Mock)
        .mockResolvedValueOnce(createMockMediaMetadata('media-1', 'a.mp4'))
        .mockRejectedValueOnce(new Error('fail'))

      blobUrlManager.acquire('media-1', new Blob(['stale-1']))
      blobUrlManager.acquire('media-2', new Blob(['stale-2']))

      const mockState = createMockState()
      const set = vi.fn()
      const get = vi.fn(() => mockState)

      const actions = createRelinkingActions(set, get)
      const result = await actions.relinkMediaBatch([
        { mediaId: 'media-1', file: makeFile('a.mp4') },
        { mediaId: 'media-2', file: makeFile('b.mp4') },
      ])

      expect(result.success).toEqual(['media-1'])
      expect(result.failed).toEqual(['media-2'])
      // media-1 invalidated (success), media-2 kept (failure)
      expect(blobUrlManager.has('media-1')).toBe(false)
      expect(blobUrlManager.has('media-2')).toBe(true)
      const names = useMediaLibraryStore.getState().mediaItems.map((media) => media.fileName)
      expect(names).toEqual(['a.mp4', 'audio.mp3'])
    })
  })

  describe('relinkOrphanedClip', () => {
    it('relinks a synchronized audio-video pair together', async () => {
      const replacementMedia = createMockMediaMetadata('media-2', 'replacement.mp4')
      const videoItem = createMockTimelineItem({
        id: 'video-1',
        type: 'video',
        trackId: 'track-v1',
        mediaId: 'missing-media',
        linkedGroupId: 'group-1',
        label: 'missing.mp4',
      })
      const audioItem = createMockTimelineItem({
        id: 'audio-1',
        type: 'audio',
        trackId: 'track-a1',
        mediaId: 'missing-media',
        linkedGroupId: 'group-1',
        label: 'missing.wav',
        src: 'blob:audio',
      })

      ;(mediaLibraryService.getMedia as Mock).mockResolvedValue(replacementMedia)
      getSynchronizedLinkedItems.mockReturnValue([videoItem, audioItem])
      getItemsState.mockReturnValue({
        items: [videoItem, audioItem],
        itemById: {
          'video-1': videoItem,
          'audio-1': audioItem,
        },
      })

      const mockState = createMockState()
      mockState.orphanedClips = [
        {
          itemId: 'video-1',
          mediaId: 'missing-media',
          itemType: 'video',
          fileName: 'missing.mp4',
          trackId: 'track-v1',
        },
        {
          itemId: 'audio-1',
          mediaId: 'missing-media',
          itemType: 'audio',
          fileName: 'missing.wav',
          trackId: 'track-a1',
        },
      ]
      let currentState: RelinkingState = mockState
      const set = vi.fn((updater: RelinkingUpdater) => {
        currentState = applyStateUpdate(currentState, updater)
      })
      const get = vi.fn(() => currentState as MediaLibraryState & MediaLibraryActions)

      const actions = createRelinkingActions(set, get)
      const result = await actions.relinkOrphanedClip('video-1', 'media-2')

      expect(result).toBe(true)
      expect(updateProjectItem).toHaveBeenCalledTimes(2)
      expect(updateProjectItem).toHaveBeenNthCalledWith(
        1,
        'video-1',
        expect.objectContaining({
          mediaId: 'media-2',
          label: 'replacement.mp4',
        }),
      )
      expect(updateProjectItem).toHaveBeenNthCalledWith(
        2,
        'audio-1',
        expect.objectContaining({
          mediaId: 'media-2',
          label: 'replacement.mp4',
        }),
      )
      expect(currentState.orphanedClips).toEqual([])
      expect(currentState.showNotification).toHaveBeenCalledWith({
        type: 'success',
        message: 'Linked clips relinked to "replacement.mp4"',
      })
    })

    it('skips duplicate work when the linked pair was already relinked', async () => {
      const replacementMedia = createMockMediaMetadata('media-2', 'replacement.mp4')
      const videoItem = createMockTimelineItem({
        id: 'video-1',
        type: 'video',
        trackId: 'track-v1',
        mediaId: 'media-2',
        linkedGroupId: 'group-1',
        label: 'replacement.mp4',
      })
      const audioItem = createMockTimelineItem({
        id: 'audio-1',
        type: 'audio',
        trackId: 'track-a1',
        mediaId: 'media-2',
        linkedGroupId: 'group-1',
        label: 'replacement.mp4',
        src: 'blob:audio',
      })

      ;(mediaLibraryService.getMedia as Mock).mockResolvedValue(replacementMedia)
      getSynchronizedLinkedItems.mockReturnValue([videoItem, audioItem])
      getItemsState.mockReturnValue({
        items: [videoItem, audioItem],
        itemById: {
          'video-1': videoItem,
          'audio-1': audioItem,
        },
      })

      const mockState = createMockState()
      mockState.orphanedClips = [
        {
          itemId: 'video-1',
          mediaId: 'missing-media',
          itemType: 'video',
          fileName: 'missing.mp4',
          trackId: 'track-v1',
        },
        {
          itemId: 'audio-1',
          mediaId: 'missing-media',
          itemType: 'audio',
          fileName: 'missing.wav',
          trackId: 'track-a1',
        },
      ]
      let currentState: RelinkingState = mockState
      const set = vi.fn((updater: RelinkingUpdater) => {
        currentState = applyStateUpdate(currentState, updater)
      })
      const get = vi.fn(() => currentState as MediaLibraryState & MediaLibraryActions)

      const actions = createRelinkingActions(set, get)
      const result = await actions.relinkOrphanedClip('audio-1', 'media-2')

      expect(result).toBe(true)
      expect(updateProjectItem).not.toHaveBeenCalled()
      expect(currentState.orphanedClips).toEqual([])
      expect(currentState.showNotification).not.toHaveBeenCalled()
    })
  })

  describe('removeOrphanedClips', () => {
    it('removes a synchronized audio-video pair from one orphan entry', () => {
      const videoItem = createMockTimelineItem({
        id: 'video-1',
        type: 'video',
        trackId: 'track-v1',
        mediaId: 'missing-media',
        linkedGroupId: 'group-1',
        label: 'missing.mp4',
      })
      const audioItem = createMockTimelineItem({
        id: 'audio-1',
        type: 'audio',
        trackId: 'track-a1',
        mediaId: 'missing-media',
        linkedGroupId: 'group-1',
        label: 'missing.wav',
        src: 'blob:audio',
      })

      getSynchronizedLinkedItems.mockReturnValue([videoItem, audioItem])
      getItemsState.mockReturnValue({
        items: [videoItem, audioItem],
        itemById: {
          'video-1': videoItem,
          'audio-1': audioItem,
        },
      })

      let currentState: RelinkingState = {
        ...createMockState(),
        orphanedClips: [
          {
            itemId: 'video-1',
            mediaId: 'missing-media',
            itemType: 'video',
            fileName: 'missing.mp4',
            trackId: 'track-v1',
          },
        ],
      }
      const set = vi.fn((updater: RelinkingUpdater) => {
        currentState = applyStateUpdate(currentState, updater)
      })
      const get = vi.fn(() => currentState as MediaLibraryState & MediaLibraryActions)

      const actions = createRelinkingActions(set, get)
      actions.removeOrphanedClips(['video-1'])

      expect(removeProjectItems).toHaveBeenCalledWith(['video-1', 'audio-1'])
      expect(currentState.orphanedClips).toEqual([])
      expect(currentState.showNotification).toHaveBeenCalledWith({
        type: 'info',
        message: 'Removed 1 orphaned clip',
      })
    })
  })
})
