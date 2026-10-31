// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { MediaMetadata } from '@editor/types/storage'
import { createDeleteActions } from './media-delete-actions'
import { useMediaLibraryStore } from './media-library-store'
import { getCommandExecutor } from '@editor/commands'

const service = vi.hoisted(() => ({
  deleteMediaFromProject: vi.fn(), deleteMedia: vi.fn(), deleteMediaBatchFromProject: vi.fn(), deleteMediaBatch: vi.fn(),
}))
const proxy = vi.hoisted(() => ({ clearProxyKey: vi.fn(), onStatusChange: vi.fn(), setMediaResolver: vi.fn(), setFilmstripPrewarm: vi.fn() }))
const blobs = vi.hoisted(() => ({ release: vi.fn() }))
const captions = vi.hoisted(() => ({ invalidateMediaCaptionCaches: vi.fn() }))
vi.mock('./media-library-service-access', () => ({ loadMediaLibraryService: vi.fn(async () => ({ mediaLibraryService: service })) }))
vi.mock('../services/media-library-service', () => ({ mediaLibraryService: service }))
vi.mock('../services/proxy-service', () => ({ proxyService: proxy }))
vi.mock('@editor/infrastructure/browser/blob-url-manager', () => ({ blobUrlManager: blobs }))
vi.mock('../services/media-caption-cache-events', () => captions)
vi.mock('@editor/infrastructure/storage', () => ({ getTranscriptMediaIds: vi.fn(async () => new Set<string>()) }))
vi.mock('./media-import-actions', () => ({ createImportActions: vi.fn(() => ({})) }))
vi.mock('./media-relinking-actions', () => ({ createRelinkingActions: vi.fn(() => ({})) }))

function makeMedia(id: string): MediaMetadata {
  return { id, storageType: 'workspace', fileName: `${id}.mp4`, fileSize: 1024, mimeType: 'video/mp4', duration: 5, width: 1920, height: 1080, fps: 30, codec: 'h264', bitrate: 5000, tags: [], createdAt: 1, updatedAt: 1 }
}
const actions = () => createDeleteActions(useMediaLibraryStore.setState, useMediaLibraryStore.getState)

describe('media deletion commands', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getCommandExecutor().clearHistory()
    useMediaLibraryStore.setState({ currentProjectId: 'project-1', mediaItems: [makeMedia('media-1'), makeMedia('media-2')], selectedMediaIds: ['media-1'], error: null })
  })

  it('optimistically removes records and releases resources after project-scoped delete', async () => {
    let complete!: () => void
    service.deleteMediaFromProject.mockReturnValue(new Promise<void>((resolve) => { complete = resolve }))
    const deletion = actions().deleteMedia('media-1')
    expect(useMediaLibraryStore.getState().mediaItems.map((media) => media.id)).toEqual(['media-2'])
    expect(useMediaLibraryStore.getState().selectedMediaIds).toEqual([])
    expect(blobs.release).not.toHaveBeenCalled()
    complete()
    await deletion
    expect(service.deleteMediaFromProject).toHaveBeenCalledWith('project-1', 'media-1')
    expect(blobs.release).toHaveBeenCalledWith('media-1')
    expect(proxy.clearProxyKey).toHaveBeenCalledWith('media-1')
    expect(captions.invalidateMediaCaptionCaches).toHaveBeenCalledWith('media-1', [])
    expect(getCommandExecutor().canUndo).toBe(false)
  })

  it('restores records and selection and leaves resources intact after failure', async () => {
    service.deleteMediaFromProject.mockRejectedValue(new Error('Delete failed hard'))
    await expect(actions().deleteMedia('media-1')).rejects.toThrow('Delete failed hard')
    expect(useMediaLibraryStore.getState().mediaItems.map((media) => media.id)).toEqual(['media-1', 'media-2'])
    expect(useMediaLibraryStore.getState().mediaById['media-1']).toBeDefined()
    expect(useMediaLibraryStore.getState().selectedMediaIds).toEqual(['media-1'])
    expect(useMediaLibraryStore.getState().error).toBe('Delete failed hard')
    expect(blobs.release).not.toHaveBeenCalled()
    expect(proxy.clearProxyKey).not.toHaveBeenCalled()
  })

  it('uses global batch deletion when no project is selected', async () => {
    useMediaLibraryStore.setState({ currentProjectId: null, selectedMediaIds: ['media-1', 'media-2'] })
    service.deleteMediaBatch.mockResolvedValue(undefined)
    await actions().deleteMediaBatch(['media-1', 'media-2'])
    expect(service.deleteMediaBatch).toHaveBeenCalledWith(['media-1', 'media-2'])
    expect(useMediaLibraryStore.getState().mediaItems).toEqual([])
    expect(useMediaLibraryStore.getState().selectedMediaIds).toEqual([])
    expect(blobs.release).toHaveBeenCalledTimes(2)
    expect(proxy.clearProxyKey).toHaveBeenCalledTimes(2)
  })

  it('uses project-scoped batch deletion when a project is active', async () => {
    service.deleteMediaBatchFromProject.mockResolvedValue(undefined)
    await actions().deleteMediaBatch(['media-1', 'media-2'])
    expect(service.deleteMediaBatchFromProject).toHaveBeenCalledWith('project-1', ['media-1', 'media-2'])
    expect(useMediaLibraryStore.getState().mediaItems).toEqual([])
  })
})
