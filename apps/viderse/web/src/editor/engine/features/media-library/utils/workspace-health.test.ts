// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { MediaMetadata } from '@editor/types/storage'

const listMedia = vi.fn<(projectId?: string) => Promise<Array<{ id: string }>>>()

vi.mock('@editor/storage/storage-backend', () => ({
  getEditorStorage: () => ({ listMedia }),
}))

import { scanWorkspaceMediaHealth } from './workspace-health'

function makeMedia(id: string, fileName = `${id}.mp4`): MediaMetadata {
  return {
    id,
    storageType: 'workspace',
    fileName,
    fileSize: 1024,
    mimeType: 'video/mp4',
    duration: 5,
    width: 1920,
    height: 1080,
    fps: 30,
    codec: 'h264',
    bitrate: 5000,
    tags: [],
    createdAt: 1,
    updatedAt: 1,
  }
}

describe('scanWorkspaceMediaHealth', () => {
  beforeEach(() => {
    listMedia.mockReset()
  })

  it('reports media missing from the server listing as file_missing and keeps the rest healthy', async () => {
    listMedia.mockResolvedValue([{ id: 'healthy' }])

    const summary = await scanWorkspaceMediaHealth([makeMedia('healthy'), makeMedia('missing')])

    expect(summary).toEqual({
      healthyIds: ['healthy'],
      broken: [{ mediaId: 'missing', fileName: 'missing.mp4', errorType: 'file_missing' }],
    })
  })

  it('lists the server once for the whole batch, scoped to the project', async () => {
    listMedia.mockResolvedValue([{ id: 'a' }, { id: 'b' }])

    await scanWorkspaceMediaHealth([makeMedia('a'), makeMedia('b')], 'project-1')

    expect(listMedia).toHaveBeenCalledTimes(1)
    expect(listMedia).toHaveBeenCalledWith('project-1')
  })

  it('does not touch the server for an empty batch', async () => {
    const summary = await scanWorkspaceMediaHealth([])

    expect(summary).toEqual({ healthyIds: [], broken: [] })
    expect(listMedia).not.toHaveBeenCalled()
  })

  it('propagates server failures instead of marking every media broken', async () => {
    listMedia.mockRejectedValue(new Error('server down'))

    await expect(scanWorkspaceMediaHealth([makeMedia('a')])).rejects.toThrow('server down')
  })
})
