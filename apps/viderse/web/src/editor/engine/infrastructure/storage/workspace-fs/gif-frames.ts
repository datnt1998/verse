/**
 * Pre-extracted GIF frames in the disposable OPFS cache.
 *
 *   media/{mediaId}/cache/gif-frames/meta.json      ← record minus frames
 *   media/{mediaId}/cache/gif-frames/frame-{N}.png  ← per-frame binary
 *
 * Binary frames let you inspect/replace them with normal tools. Saving
 * writes all frames in parallel; reading pulls them back as Blobs.
 */

import type { GifFrameData } from '@editor/types/storage'
import { createLogger } from '@editor/shared/logging/logger'

import { tryResolveMediaIdentity } from './cache-identity'
import { requireWorkspaceRoot } from './root'
import {
  readBlob,
  readJson,
  removeEntry,
  removeFromEveryMedia,
  writeBlob,
  writeJsonAtomic,
} from './fs-primitives'
import { cacheMetaPath, gifFramePath, gifFramesDir } from './paths'
import { blobToArrayBuffer } from './blob-utils'

const logger = createLogger('WorkspaceFS:GifFrames')

type GifFramesMeta = Omit<GifFrameData, 'frames'>

export async function saveGifFrames(data: GifFrameData): Promise<void> {
  const root = requireWorkspaceRoot()
  try {
    // One lookup, pinned: every file of this save lands under the same hash.
    const identity = await tryResolveMediaIdentity(data.mediaId)
    // Media deleted on the server: there is nothing to cache frames for.
    if (!identity) return
    const meta: GifFramesMeta = {
      id: data.id,
      mediaId: data.mediaId,
      durations: data.durations,
      totalDuration: data.totalDuration,
      width: data.width,
      height: data.height,
      frameCount: data.frameCount,
      createdAt: data.createdAt,
    }
    await Promise.all(
      data.frames.map(async (frame, i) => {
        const bytes = new Uint8Array(await blobToArrayBuffer(frame))
        await writeBlob(root, gifFramePath(data.mediaId, i), bytes, identity)
      }),
    )
    // Meta last: it is the completeness marker readers check first.
    await writeJsonAtomic(root, cacheMetaPath(gifFramesDir(data.mediaId)), meta, identity)
  } catch (error) {
    logger.error('saveGifFrames failed', error)
    throw new Error('Failed to save GIF frames')
  }
}

export async function getGifFrames(id: string): Promise<GifFrameData | undefined> {
  const root = requireWorkspaceRoot()
  try {
    const identity = await tryResolveMediaIdentity(id)
    if (!identity) return undefined
    const meta = await readJson<GifFramesMeta>(root, cacheMetaPath(gifFramesDir(id)), identity)
    if (!meta) return undefined
    const frames: Blob[] = []
    for (let i = 0; i < meta.frameCount; i++) {
      const blob = await readBlob(root, gifFramePath(id, i), identity)
      if (!blob) return undefined
      frames.push(blob)
    }
    return { ...meta, frames }
  } catch (error) {
    logger.error(`getGifFrames(${id}) failed`, error)
    return undefined
  }
}

export async function deleteGifFrames(id: string): Promise<void> {
  const root = requireWorkspaceRoot()
  try {
    await removeEntry(root, gifFramesDir(id), { recursive: true })
  } catch (error) {
    logger.error(`deleteGifFrames(${id}) failed`, error)
    throw new Error(`Failed to delete GIF frames: ${id}`)
  }
}

export async function clearAllGifFrames(): Promise<void> {
  try {
    await removeFromEveryMedia(requireWorkspaceRoot(), ['cache', 'gif-frames'])
  } catch (error) {
    logger.error('clearAllGifFrames failed', error)
    throw new Error('Failed to clear GIF frames')
  }
}
