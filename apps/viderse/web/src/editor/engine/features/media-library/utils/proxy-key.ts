import type { MediaMetadata } from '@editor/types/storage'
import { getEditorStorage } from '@editor/storage/storage-backend'
import { cacheSegments, type CacheIdentity } from '@editor/storage/opfs-cache'

type ProxyKeyMedia = Pick<MediaMetadata, 'id' | 'contentHash'>

const PROXY_KEY_SEPARATOR = '@'

/**
 * In-memory identity of one proxy: the media and the exact source bytes it was encoded from.
 * A replaced source has a new content hash and therefore a new key, so a stale proxy can never
 * be served for new bytes. The workspace is implicit — one editor session talks to one server.
 */
export function getSharedProxyKey(media: ProxyKeyMedia): string {
  if (!media.contentHash) {
    throw new Error(`Media ${media.id} has no content hash; cannot derive its proxy key`)
  }
  return `${media.id}${PROXY_KEY_SEPARATOR}${media.contentHash}`
}

export function parseProxyKey(proxyKey: string): { mediaId: string; contentHash: string } | null {
  const separatorIndex = proxyKey.indexOf(PROXY_KEY_SEPARATOR)
  if (separatorIndex <= 0 || separatorIndex === proxyKey.length - 1) return null
  return {
    mediaId: proxyKey.slice(0, separatorIndex),
    contentHash: proxyKey.slice(separatorIndex + 1),
  }
}

/** Full cache identity (workspace + media + hash) a proxy key is stored under. */
export async function proxyCacheIdentity(proxyKey: string): Promise<CacheIdentity> {
  const parsed = parseProxyKey(proxyKey)
  if (!parsed) throw new Error(`Proxy key "${proxyKey}" does not identify a media version`)
  return { workspaceId: await getEditorStorage().workspaceId(), ...parsed }
}

/**
 * OPFS directory segments of a proxy: `<cache>/<workspaceId>/media/<mediaId>/<contentHash>/proxy`.
 * Identity is pinned from the key, so a source replaced mid-flight cannot redirect the write.
 */
export async function proxyCacheDirSegments(proxyKey: string): Promise<string[]> {
  const identity = await proxyCacheIdentity(proxyKey)
  return cacheSegments(['media', identity.mediaId, 'proxy'], identity)
}
