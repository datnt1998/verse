/**
 * Filmstrip Storage
 *
 * Filmstrip frames live in the disposable OPFS cache under the owning media's
 * cache directory, versioned by the server content hash:
 *   media/{mediaId}/cache/filmstrip/
 *     meta.json - { width, height, isComplete, frameCount }
 *     0.jpg, 1.jpg, 2.jpg, ...
 *
 * On-disk entries for a replaced source are unreachable (new hash, new
 * directory). The object-URL maps below are keyed by mediaId, so they are
 * dropped explicitly when the content hash changes.
 */

import { createLogger } from '@editor/shared/logging/logger'
import type { CacheIdentity } from '@editor/storage/opfs-cache'
import {
  onMediaContentChange,
  resolveMediaIdentity,
} from '@editor/infrastructure/storage/workspace-fs/cache-identity'
import {
  readBlob,
  readDirectoryFiles,
  readJson,
  removeFromEveryMedia,
  writeBlob,
  writeJsonAtomic,
  removeEntry,
} from '@editor/infrastructure/storage/workspace-fs/fs-primitives'
import { requireWorkspaceRoot } from '@editor/infrastructure/storage/workspace-fs/root'
import {
  filmstripDir,
  filmstripFramePath,
  filmstripMetaPath,
} from '@editor/infrastructure/storage/workspace-fs/paths'

const logger = createLogger('FilmstripStorage')

const FRAME_RATE = 1 // Must match worker - 1fps for filmstrip thumbnails
const PRIMARY_FRAME_EXT = 'jpg'
const FILMSTRIP_FRAME_SCHEMA_VERSION = 2
const FILMSTRIP_URL_CREATION_SLICE_BUDGET_MS = 4

type CooperativeScheduler = {
  yield?: () => Promise<void>
}

async function yieldToMainThread(): Promise<void> {
  const scheduler = (globalThis as typeof globalThis & { scheduler?: CooperativeScheduler })
    .scheduler
  if (scheduler?.yield) {
    await scheduler.yield()
    return
  }

  await new Promise<void>((resolve) => setTimeout(resolve, 0))
}

function parseFrameFileName(name: string): number | null {
  const suffix = `.${PRIMARY_FRAME_EXT}`
  if (!name.endsWith(suffix)) return null
  const index = parseInt(name.slice(0, -suffix.length), 10)
  return Number.isNaN(index) ? null : index
}

interface FilmstripMetadata {
  version?: number
  width: number
  height: number
  isComplete: boolean
  frameCount: number
}

export interface FilmstripFrame {
  index: number
  timestamp: number
  url: string
  byteSize?: number
  bitmap?: ImageBitmap
}

interface LoadedFilmstrip {
  metadata: FilmstripMetadata
  frames: FilmstripFrame[]
  existingIndices: number[]
}

interface FrameUrlOperationToken {
  clearGeneration: number
  mediaGeneration: number
}

class FilmstripStorage {
  private objectUrls = new Map<string, Map<number, string>>()
  private frameUrlGenerations = new Map<string, number>()
  private clearGeneration = 0
  private pinnedIdentities = new Map<string, Promise<CacheIdentity | undefined>>()

  constructor() {
    // A replaced source changes the cache identity: object URLs and in-flight
    // loads for the old bytes must not outlive it (OPFS paths are hash-versioned,
    // this in-memory state is keyed by mediaId alone).
    onMediaContentChange(({ mediaId }) => this.revokeUrls(mediaId))
  }
  private beginFrameUrlOperation(mediaId: string): FrameUrlOperationToken {
    const mediaGeneration = (this.frameUrlGenerations.get(mediaId) ?? 0) + 1
    this.frameUrlGenerations.set(mediaId, mediaGeneration)
    return {
      clearGeneration: this.clearGeneration,
      mediaGeneration,
    }
  }

  private invalidateFrameUrlOperations(mediaId: string): void {
    this.frameUrlGenerations.set(mediaId, (this.frameUrlGenerations.get(mediaId) ?? 0) + 1)
  }

  private ownsFrameUrlOperation(mediaId: string, token: FrameUrlOperationToken): boolean {
    return (
      token.clearGeneration === this.clearGeneration &&
      token.mediaGeneration === this.frameUrlGenerations.get(mediaId)
    )
  }

  private revokeUncommittedFrameUrls(entries: Array<{ url: string }>): void {
    for (const { url } of entries) {
      URL.revokeObjectURL(url)
    }
  }

  private scheduleRevoke(urls: string[]): void {
    if (urls.length === 0) return

    const revoke = () => {
      for (const url of urls) {
        URL.revokeObjectURL(url)
      }
    }

    if (typeof requestIdleCallback === 'function') {
      requestIdleCallback(revoke, { timeout: 10_000 })
      return
    }

    setTimeout(revoke, 0)
  }

  private setFrameUrl(mediaId: string, index: number, url: string): void {
    this.invalidateFrameUrlOperations(mediaId)
    const urlsByIndex = this.objectUrls.get(mediaId) ?? new Map<number, string>()
    const previous = urlsByIndex.get(index)
    urlsByIndex.set(index, url)
    this.objectUrls.set(mediaId, urlsByIndex)

    if (previous && previous !== url) {
      this.scheduleRevoke([previous])
    }
  }

  private replaceAllFrameUrls(
    mediaId: string,
    entries: Array<{ index: number; url: string }>,
    token: FrameUrlOperationToken,
  ): boolean {
    if (!this.ownsFrameUrlOperation(mediaId, token)) {
      return false
    }

    const previous = this.objectUrls.get(mediaId)
    const next = new Map<number, string>()
    for (const entry of entries) {
      next.set(entry.index, entry.url)
    }
    this.objectUrls.set(mediaId, next)

    if (!previous) return true

    const toRevoke: string[] = []
    for (const [index, url] of previous) {
      const nextUrl = next.get(index)
      if (nextUrl !== url) {
        toRevoke.push(url)
      }
    }
    this.scheduleRevoke(toRevoke)
    return true
  }

  /**
   * Observe the media's current server content hash. A change since the last
   * observation drops this media's in-memory state (via the change listeners), so
   * callers invoke it before reading or creating state. A lookup failure resurfaces
   * in the cache read that follows.
   */
  async syncContentHash(mediaId: string): Promise<void> {
    await resolveMediaIdentity(mediaId).catch(() => undefined)
  }

  private async readMetadata(mediaId: string): Promise<FilmstripMetadata | null> {
    return await readJson<FilmstripMetadata>(requireWorkspaceRoot(), filmstripMetaPath(mediaId))
  }

  private async ensureFilmstrip(mediaId: string): Promise<FilmstripMetadata | null> {
    const existing = await this.readMetadata(mediaId)
    if (!existing) return null
    if (existing.version === FILMSTRIP_FRAME_SCHEMA_VERSION) return existing
    this.invalidateFrameUrlOperations(mediaId)
    const staleUrls = this.objectUrls.get(mediaId)
    if (staleUrls) this.scheduleRevoke([...staleUrls.values()])
    await removeEntry(requireWorkspaceRoot(), filmstripDir(mediaId), { recursive: true }).catch(
      () => undefined,
    )
    this.objectUrls.delete(mediaId)
    return null
  }

  /**
   * Pin the content identity an extraction writes under. Frames are produced
   * over seconds to minutes; pinning at the start keeps them out of the new
   * hash's directory if the source is replaced mid-extraction. Must be paired
   * with {@link unpinIdentity}.
   */
  pinIdentity(mediaId: string): void {
    this.pinnedIdentities.set(
      mediaId,
      resolveMediaIdentity(mediaId).catch(() => undefined),
    )
  }

  unpinIdentity(mediaId: string): void {
    this.pinnedIdentities.delete(mediaId)
  }

  private async writeIdentity(mediaId: string): Promise<CacheIdentity | undefined> {
    return await this.pinnedIdentities.get(mediaId)
  }

  async saveMetadata(
    mediaId: string,
    metadata: { width: number; height: number; isComplete: boolean; frameCount: number },
  ): Promise<void> {
    await writeJsonAtomic(
      requireWorkspaceRoot(),
      filmstripMetaPath(mediaId),
      { ...metadata, version: FILMSTRIP_FRAME_SCHEMA_VERSION },
      await this.writeIdentity(mediaId),
    )
  }

  async saveFrameBlob(mediaId: string, index: number, blob: Blob): Promise<void> {
    await writeBlob(
      requireWorkspaceRoot(),
      filmstripFramePath(mediaId, index, PRIMARY_FRAME_EXT),
      blob,
      await this.writeIdentity(mediaId),
    )
  }

  async load(mediaId: string): Promise<LoadedFilmstrip | null> {
    // Observe the current content hash before taking the operation token: a
    // replaced source revokes this media's URLs, which must not cancel this load.
    await this.syncContentHash(mediaId)
    const urlOperation = this.beginFrameUrlOperation(mediaId)
    const nextUrls: Array<{ index: number; url: string }> = []
    let urlsCommitted = false

    try {
      const metadata = await this.ensureFilmstrip(mediaId)
      if (!metadata) return null
      if (!this.ownsFrameUrlOperation(mediaId, urlOperation)) return null

      // Resolve the filmstrip dir once and read all matching files via the
      // same handle iterator. readBlob-per-path would re-walk the 4-segment
      // media path for every frame; with N=60 that's 240 wasted lookups.
      const files = await readDirectoryFiles(
        requireWorkspaceRoot(),
        filmstripDir(mediaId),
        (entry) => parseFrameFileName(entry.name) !== null,
      )

      const frameFiles: Array<{ index: number; blob: Blob }> = []
      for (const { name, blob } of files) {
        const index = parseFrameFileName(name)
        if (index === null || !blob || blob.size <= 0) continue
        frameFiles.push({ index, blob })
      }
      frameFiles.sort((a, b) => a.index - b.index)

      const frames: FilmstripFrame[] = []
      let sliceStartedAt = performance.now()

      for (const [frameIndex, { index, blob }] of frameFiles.entries()) {
        if (!this.ownsFrameUrlOperation(mediaId, urlOperation)) {
          this.revokeUncommittedFrameUrls(nextUrls)
          return null
        }

        const url = URL.createObjectURL(blob)
        nextUrls.push({ index, url })
        frames.push({
          index,
          timestamp: index / FRAME_RATE,
          url,
          byteSize: blob.size,
        })

        const hasMoreFrames = frameIndex < frameFiles.length - 1
        if (
          hasMoreFrames &&
          performance.now() - sliceStartedAt >= FILMSTRIP_URL_CREATION_SLICE_BUDGET_MS
        ) {
          await yieldToMainThread()
          sliceStartedAt = performance.now()
        }
      }
      urlsCommitted = this.replaceAllFrameUrls(mediaId, nextUrls, urlOperation)
      if (!urlsCommitted) {
        this.revokeUncommittedFrameUrls(nextUrls)
        return null
      }

      const existingIndices = frameFiles.map((frame) => frame.index)

      if (metadata.isComplete && frames.length === 0) {
        logger.warn(`Filmstrip ${mediaId} marked complete but has 0 frames - resetting`)
        metadata.isComplete = false
        metadata.frameCount = 0
      }

      logger.debug(
        `Loaded filmstrip ${mediaId}: ${frames.length} frames, complete: ${metadata.isComplete}`,
      )
      return { metadata, frames, existingIndices }
    } catch (error) {
      if (!urlsCommitted) {
        this.revokeUncommittedFrameUrls(nextUrls)
      }
      logger.warn('Failed to load filmstrip:', error)
      return null
    }
  }

  async getExistingIndices(
    mediaId: string,
    startIndex?: number,
    endIndex?: number,
  ): Promise<number[]> {
    const metadata = await this.ensureFilmstrip(mediaId)
    if (!metadata) return []

    // Resolve the dir once and read matching files concurrently — was a
    // sequential await loop that re-walked the four-segment path per frame.
    const files = await readDirectoryFiles(
      requireWorkspaceRoot(),
      filmstripDir(mediaId),
      (entry) => {
        const index = parseFrameFileName(entry.name)
        if (index === null) return false
        if (typeof startIndex === 'number' && index < startIndex) return false
        if (typeof endIndex === 'number' && index >= endIndex) return false
        return true
      },
    )

    const indices = new Set<number>()
    for (const { name, blob } of files) {
      const index = parseFrameFileName(name)
      if (index === null) continue
      if (blob && blob.size > 0) {
        indices.add(index)
      }
    }

    return Array.from(indices).sort((a, b) => a - b)
  }

  async loadSingleFrame(mediaId: string, index: number): Promise<FilmstripFrame | null> {
    const metadata = await this.ensureFilmstrip(mediaId)
    if (!metadata) return null

    const blob = await readBlob(
      requireWorkspaceRoot(),
      filmstripFramePath(mediaId, index, PRIMARY_FRAME_EXT),
    )
    if (!blob || blob.size === 0) return null

    const url = URL.createObjectURL(blob)
    this.setFrameUrl(mediaId, index, url)

    return {
      index,
      timestamp: index / FRAME_RATE,
      url,
      byteSize: blob.size,
    }
  }

  createFrameFromBlob(mediaId: string, index: number, blob: Blob): FilmstripFrame | null {
    if (!blob || blob.size === 0) return null

    const url = URL.createObjectURL(blob)
    this.setFrameUrl(mediaId, index, url)

    return {
      index,
      timestamp: index / FRAME_RATE,
      url,
      byteSize: blob.size,
    }
  }

  createFrameFromBitmap(
    _mediaId: string,
    index: number,
    bitmap: ImageBitmap,
  ): FilmstripFrame | null {
    if (!bitmap || bitmap.width === 0) return null

    return {
      index,
      timestamp: index / FRAME_RATE,
      url: '',
      byteSize: bitmap.width * bitmap.height * 4,
      bitmap,
    }
  }

  async isComplete(mediaId: string): Promise<boolean> {
    const metadata = await this.ensureFilmstrip(mediaId)
    return metadata?.isComplete ?? false
  }

  async delete(mediaId: string): Promise<void> {
    this.revokeUrls(mediaId)
    await removeEntry(requireWorkspaceRoot(), filmstripDir(mediaId), {
      recursive: true,
    })
    logger.debug(`Deleted filmstrip ${mediaId}`)
  }

  revokeUrls(mediaId: string): void {
    this.invalidateFrameUrlOperations(mediaId)
    const urlsByIndex = this.objectUrls.get(mediaId)
    if (!urlsByIndex) return

    for (const url of urlsByIndex.values()) {
      URL.revokeObjectURL(url)
    }
    this.objectUrls.delete(mediaId)
  }

  async clearAll(): Promise<void> {
    // Invalidate loads that started before or during the clear, including
    // media that have not published an object URL map yet.
    this.clearGeneration++
    try {
      for (const mediaId of this.objectUrls.keys()) {
        this.revokeUrls(mediaId)
      }

      // Filmstrips live per media under `media/<id>/<contentHash>/cache/filmstrip/`;
      // prune every version, including media the server no longer lists.
      await removeFromEveryMedia(requireWorkspaceRoot(), ['cache', 'filmstrip'])
    } finally {
      this.clearGeneration++
      for (const mediaId of this.objectUrls.keys()) {
        this.revokeUrls(mediaId)
      }
    }
  }
}

export const filmstripStorage = new FilmstripStorage()
