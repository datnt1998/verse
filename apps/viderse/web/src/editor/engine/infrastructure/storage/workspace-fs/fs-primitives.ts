/**
 * Filesystem primitives over the disposable, versioned OPFS cache.
 *
 * Every cache module in workspace-fs calls these. Paths are arrays of
 * segments; the `CacheRoot` token routes them through `cacheDirectory`, which
 * scopes them by workspace id and — for `media/<mediaId>/...` paths — by the
 * server's current content hash. Nothing here is durable: the server owns
 * project data and source media, so every failure that means "the cache entry
 * is gone or cannot be stored" (eviction, quota, deleted media) is a miss
 * rather than an error.
 *
 * - JSON/binary reads resolve to `null` on a miss.
 * - Writes go through `FileSystemWritableFileStream`, which commits on
 *   `close()` (an aborted write leaves the previous file intact), so no
 *   tmp-file + rename dance is needed.
 * - Writes that cannot be stored (quota, eviction, media deleted on the
 *   server) are dropped with a warning instead of failing the caller.
 */

import { createLogger } from '@editor/shared/logging/logger'
import { StorageClientError } from '@editor/storage/errors'
import { cacheDirectory, type CacheIdentity, type CacheRoot } from '@editor/storage/opfs-cache'
import { resolveMediaIdentity } from './cache-identity'
import { withKeyLock } from './with-key-lock'

const logger = createLogger('CacheFS')

/** A cached file exists but its content is not valid JSON. */
export class WorkspaceFileCorruptError extends Error {
  constructor(
    public readonly path: string,
    cause: unknown,
  ) {
    super(`Corrupt JSON at ${path}`)
    this.name = 'WorkspaceFileCorruptError'
    this.cause = cause
  }
}

const MISS_ERROR_NAMES: ReadonlySet<string> = new Set([
  'NotFoundError',
  'NotReadableError',
  'TypeMismatchError',
])
const DROPPED_WRITE_ERROR_NAMES: ReadonlySet<string> = new Set([
  'QuotaExceededError',
  'NotFoundError',
  'NotReadableError',
  'NoModificationAllowedError',
  'InvalidStateError',
])

/** Entry missing, evicted mid-read, or the media it belongs to no longer exists. */
function isCacheMiss(error: unknown): boolean {
  if (error instanceof StorageClientError) return error.code === 'not_found'
  return error instanceof DOMException && MISS_ERROR_NAMES.has(error.name)
}

/** Write that the browser/server can no longer host — dropped, never surfaced. */
function isDroppedWrite(error: unknown): boolean {
  if (error instanceof StorageClientError) return error.code === 'not_found'
  return error instanceof DOMException && DROPPED_WRITE_ERROR_NAMES.has(error.name)
}

/**
 * Open the directory for `segments`. For `media/<id>/...` paths the identity is
 * resolved here (feeding the content-change tracker) and handed to
 * `cacheDirectory` so the lookup happens exactly once per operation.
 */
async function resolveDirectory(
  _root: CacheRoot,
  segments: string[],
  create: boolean,
  identity?: CacheIdentity,
) {
  const mediaId = segments[0] === 'media' ? segments[1] : undefined
  const resolved = identity ?? (mediaId ? await resolveMediaIdentity(mediaId) : undefined)
  return cacheDirectory(segments, create, resolved)
}

/** Resolve segments to (parent dir, file name). Segments must have length >= 1. */
async function resolveFileParent(
  root: CacheRoot,
  segments: string[],
  create: boolean,
  identity?: CacheIdentity,
) {
  if (segments.length === 0) {
    throw new Error('fs-primitives: empty path segments')
  }
  const parent = await resolveDirectory(root, segments.slice(0, -1), create, identity)
  return { parent, fileName: segments[segments.length - 1]! }
}

/* ────────────────────────────── Read helpers ─────────────────────────── */

export async function readJson<T>(
  root: CacheRoot,
  segments: string[],
  identity?: CacheIdentity,
): Promise<T | null> {
  const blob = await readBlob(root, segments, identity)
  if (!blob) return null
  const text = await blob.text()
  if (text.length === 0) return null
  try {
    return JSON.parse(text) as T
  } catch (error) {
    throw new WorkspaceFileCorruptError(segments.join('/'), error)
  }
}

export async function readBlob(
  root: CacheRoot,
  segments: string[],
  identity?: CacheIdentity,
): Promise<Blob | null> {
  try {
    const { parent, fileName } = await resolveFileParent(root, segments, false, identity)
    const file = await parent.getFileHandle(fileName, { create: false })
    return await file.getFile()
  } catch (error) {
    if (isCacheMiss(error)) return null
    throw error
  }
}

export async function readArrayBuffer(
  root: CacheRoot,
  segments: string[],
  identity?: CacheIdentity,
): Promise<ArrayBuffer | null> {
  const blob = await readBlob(root, segments, identity)
  if (!blob) return null
  return blob.arrayBuffer()
}

/* ────────────────────────────── Write helpers ────────────────────────── */

/**
 * Write one cache file. Serialized per path: two concurrent writers on the
 * same file would otherwise race on the writable lock. Different paths use
 * different keys, so unrelated writes stay parallel. Returns false when the
 * write was dropped (quota, eviction, media gone).
 */
async function putFile(
  root: CacheRoot,
  segments: string[],
  data: Blob | ArrayBuffer | Uint8Array | string,
  identity?: CacheIdentity,
): Promise<boolean> {
  return withKeyLock(`cache-write:${segments.join('/')}`, async () => {
    let created: { parent: Awaited<ReturnType<typeof resolveDirectory>>; fileName: string } | null =
      null
    try {
      const { parent, fileName } = await resolveFileParent(root, segments, true, identity)
      const existed = await parent
        .getFileHandle(fileName, { create: false })
        .then(() => true)
        .catch(() => false)
      const handle = await parent.getFileHandle(fileName, { create: true })
      if (!existed) created = { parent, fileName }
      const writable = await handle.createWritable()
      try {
        await writable.write(data as FileSystemWriteChunkType)
        await writable.close()
      } catch (error) {
        await writable.abort().catch(() => undefined)
        throw error
      }
      return true
    } catch (error) {
      if (!isDroppedWrite(error)) throw error
      logger.warn(`Cache write dropped for ${segments.join('/')}`, error)
      // A file this write created must not outlive it as an empty placeholder
      // that readers would mistake for a cached entry.
      if (created) {
        await created.parent.removeEntry(created.fileName).catch(() => undefined)
      }
      return false
    }
  })
}

/**
 * JSON write. Named for its previous contract: readers never observe a torn
 * file because the writable stream only publishes on close. Resolves to the
 * serialized size, or 0 when the write was dropped.
 */
export async function writeJsonAtomic(
  root: CacheRoot,
  segments: string[],
  data: unknown,
  identity?: CacheIdentity,
): Promise<number> {
  const json = JSON.stringify(data, null, 2)
  return (await putFile(root, segments, json, identity)) ? json.length : 0
}

export async function writeBlob(
  root: CacheRoot,
  segments: string[],
  data: Blob | ArrayBuffer | Uint8Array | string,
  identity?: CacheIdentity,
): Promise<void> {
  await putFile(root, segments, data, identity)
}

/* ────────────────────────────── Delete helpers ───────────────────────── */

/** Remove a file or a whole subtree. No-op when missing. */
export async function removeEntry(
  root: CacheRoot,
  segments: string[],
  options: { recursive?: boolean } = {},
): Promise<void> {
  if (segments.length === 0) {
    throw new Error('fs-primitives: refusing to remove empty path')
  }
  try {
    const { parent, fileName } = await resolveFileParent(root, segments, false)
    await parent.removeEntry(fileName, { recursive: options.recursive ?? false })
  } catch (error) {
    if (isCacheMiss(error)) return
    throw error
  }
}

/* ────────────────────────────── Enumeration ──────────────────────────── */

export interface DirectoryEntry {
  name: string
  kind: 'file' | 'directory'
}

export async function listDirectory(
  root: CacheRoot,
  segments: string[],
  identity?: CacheIdentity,
): Promise<DirectoryEntry[]> {
  try {
    const dir = await resolveDirectory(root, segments, false, identity)
    const entries: DirectoryEntry[] = []
    for await (const entry of dir.values()) {
      entries.push({ name: entry.name, kind: entry.kind })
    }
    return entries
  } catch (error) {
    if (isCacheMiss(error)) return []
    throw error
  }
}

/**
 * List a directory and read all matching files in a single resolved-dir pass.
 *
 * Calling readBlob() per file re-resolves the identity and re-walks the
 * segments on every read. This helper resolves the directory once and reads
 * all matching files concurrently.
 */
export async function readDirectoryFiles(
  root: CacheRoot,
  segments: string[],
  filter?: (entry: DirectoryEntry) => boolean,
): Promise<Array<{ name: string; blob: Blob }>> {
  try {
    const dir = await resolveDirectory(root, segments, false)
    const fileHandles: Array<{ name: string; handle: FileSystemFileHandle }> = []
    for await (const entry of dir.values()) {
      if (entry.kind !== 'file') continue
      if (filter && !filter({ name: entry.name, kind: entry.kind })) continue
      fileHandles.push({ name: entry.name, handle: entry as FileSystemFileHandle })
    }
    const results = await Promise.all(
      fileHandles.map(async ({ name, handle }) => {
        try {
          return { name, blob: (await handle.getFile()) as Blob }
        } catch (error) {
          // A file can disappear between iterating dir.values() and getFile()
          // (eviction or concurrent delete). Skip it instead of failing the batch.
          if (isCacheMiss(error)) return null
          throw error
        }
      }),
    )
    return results.filter((r): r is { name: string; blob: Blob } => r !== null)
  } catch (error) {
    if (isCacheMiss(error)) return []
    throw error
  }
}

export async function exists(root: CacheRoot, segments: string[]): Promise<boolean> {
  try {
    const { parent, fileName } = await resolveFileParent(root, segments, false)
    try {
      await parent.getFileHandle(fileName, { create: false })
      return true
    } catch (error) {
      if (!isCacheMiss(error)) throw error
    }
    try {
      await parent.getDirectoryHandle(fileName, { create: false })
      return true
    } catch (error) {
      if (!isCacheMiss(error)) throw error
    }
    return false
  } catch (error) {
    if (isCacheMiss(error)) return false
    throw error
  }
}

/**
 * Remove `relativeSegments` (e.g. `['cache', 'filmstrip']`) from every cached
 * media, across all content-hash versions — including versions of media the
 * server no longer lists, which per-media paths cannot address. Used by
 * "clear all" operations on one cache kind.
 */
export async function removeFromEveryMedia(
  _root: CacheRoot,
  relativeSegments: string[],
): Promise<void> {
  if (relativeSegments.length === 0) {
    throw new Error('fs-primitives: refusing to remove empty path')
  }
  const parentSegments = relativeSegments.slice(0, -1)
  const target = relativeSegments[relativeSegments.length - 1]!
  try {
    const mediaRoot = await cacheDirectory(['media'], false)
    const mediaDirs: Array<{ name: string }> = []
    for await (const entry of mediaRoot.values()) {
      if (entry.kind === 'directory') mediaDirs.push(entry)
    }
    for (const { name } of mediaDirs) {
      const mediaDir = await mediaRoot.getDirectoryHandle(name)
      const versions: string[] = []
      for await (const entry of mediaDir.values()) {
        if (entry.kind === 'directory') versions.push(entry.name)
      }
      for (const version of versions) {
        try {
          let directory = await mediaDir.getDirectoryHandle(version)
          for (const segment of parentSegments) {
            directory = await directory.getDirectoryHandle(segment)
          }
          await directory.removeEntry(target, { recursive: true })
        } catch (error) {
          if (!isCacheMiss(error)) throw error
        }
      }
    }
  } catch (error) {
    if (!isCacheMiss(error)) throw error
  }
}
