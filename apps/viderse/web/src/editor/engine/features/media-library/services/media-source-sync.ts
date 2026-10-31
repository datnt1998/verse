import type { MediaMetadata } from '@editor/types/storage'
import { sourceRefreshPlan, type MediaSourceChanged } from '@editor/storage/media-source-refresh'

export interface SourceSyncDependencies {
  workspaceId: string
  current(mediaId: string): MediaMetadata | undefined
  fetchMedia(mediaId: string): Promise<MediaMetadata | undefined>
  invalidate(mediaId: string, oldUrl: string | null, contentHash: string): void
  apply(media: MediaMetadata, src: string): void
}

/** Authoritative metadata wins over delayed messages; concurrent refreshes share IO. */
export function createMediaSourceSync(deps: SourceSyncDependencies) {
  const pending = new Map<string, Promise<void>>()
  const handle = (message: unknown): Promise<void> => {
    if (!message || typeof message !== 'object') return Promise.resolve()
    const mediaId = (message as Partial<MediaSourceChanged>).mediaId
    if (typeof mediaId !== 'string' || !sourceRefreshPlan(message, deps.workspaceId, deps.current(mediaId))) return Promise.resolve()
    const active = pending.get(mediaId)
    if (active) return active.then(() => handle(message))
    const request = deps.fetchMedia(mediaId).then(media => {
      if (!media?.contentHash) return
      const plan = sourceRefreshPlan({ workspaceId: deps.workspaceId, mediaId, contentHash: media.contentHash }, deps.workspaceId, deps.current(mediaId))
      if (!plan) return
      deps.invalidate(mediaId, plan.oldUrl, media.contentHash)
      deps.apply(media, plan.newUrl)
    }).finally(() => { if (pending.get(mediaId) === request) pending.delete(mediaId) })
    pending.set(mediaId, request)
    return request
  }
  return { handle }
}
