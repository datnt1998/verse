import { getEditorStorage } from './storage-backend'

export interface CacheIdentity { workspaceId: string; mediaId: string; contentHash: string }
export function mediaCacheKey(identity: CacheIdentity): string {
  return [identity.workspaceId, identity.mediaId, identity.contentHash].map(encodeURIComponent).join('/')
}
export interface CacheRoot { readonly kind: 'editor-opfs-cache' }
export const CACHE_ROOT: CacheRoot = { kind: 'editor-opfs-cache' }

export async function resolveCacheIdentity(mediaId: string): Promise<CacheIdentity> {
  const storage = getEditorStorage()
  const [workspaceId, media] = await Promise.all([storage.workspaceId(), storage.getMedia(mediaId)])
  return { workspaceId, mediaId, contentHash: media.contentHash }
}

/** Cache paths are versioned against authoritative metadata on every operation. */
export async function cacheSegments(segments: string[], identity?: CacheIdentity): Promise<string[]> {
  const workspaceId = identity?.workspaceId ?? await getEditorStorage().workspaceId()
  if (segments[0] === 'media' && segments[1]) {
    const source = identity ?? await resolveCacheIdentity(segments[1])
    if (source.mediaId !== segments[1]) throw new Error('Media không khớp danh tính cache')
    return ['ytf-editor-cache', encodeURIComponent(workspaceId), 'media', encodeURIComponent(source.mediaId), encodeURIComponent(source.contentHash), ...segments.slice(2)]
  }
  return ['ytf-editor-cache', encodeURIComponent(workspaceId), ...segments]
}

export async function cacheDirectory(segments: string[], create = false, identity?: CacheIdentity) {
  let directory = await navigator.storage.getDirectory()
  for (const segment of await cacheSegments(segments, identity)) {
    if (!segment || segment === '.' || segment === '..' || segment.includes('/')) throw new Error('Đường dẫn cache không hợp lệ')
    directory = await directory.getDirectoryHandle(segment, { create })
  }
  return directory
}

export async function readCacheBlob(segments: string[], identity?: CacheIdentity): Promise<Blob | null> {
  try {
    const directory = await cacheDirectory(segments.slice(0, -1), false, identity)
    return await (await directory.getFileHandle(segments[segments.length - 1])).getFile()
  } catch { return null }
}
export async function writeCacheBlob(segments: string[], data: Blob | ArrayBuffer | Uint8Array | string, identity?: CacheIdentity): Promise<void> {
  try {
    const directory = await cacheDirectory(segments.slice(0, -1), true, identity)
    const handle = await directory.getFileHandle(segments[segments.length - 1], { create: true })
    const writer = await handle.createWritable()
    try { await writer.write(data as FileSystemWriteChunkType); await writer.close() }
    catch (error) { await writer.abort().catch(() => undefined); throw error }
  } catch { /* Eviction, quota and unavailable OPFS are cache misses, never save failures. */ }
}
export async function removeCacheEntry(segments: string[], recursive = false, identity?: CacheIdentity): Promise<void> {
  try {
    const directory = await cacheDirectory(segments.slice(0, -1), false, identity)
    await directory.removeEntry(segments[segments.length - 1], { recursive })
  } catch { /* Cache already absent. */ }
}
