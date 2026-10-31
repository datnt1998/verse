/**
 * Persists durable media metadata (user tags and attribution) after canonical commits.
 *
 * `media.update` (and its undo/redo) changes tags/attribution on the in-memory media records
 * synchronously; the server copy is updated here from the commit's before/after diff, so the
 * same path covers a fresh edit, an undo and a redo. Imports persist their own tags with the
 * upload, and hydration/rollback (`media.setRecords`) replaces records with data that already
 * is — or intentionally is not — durable, so those commits are never written back.
 */

import type { MediaAttribution, MediaMetadata } from '@editor/types/storage'
import { createLogger } from '@editor/shared/logging/logger'
import { updateMedia, type MediaMetadataUpdate } from '@editor/features/media-library/deps/storage'

const logger = createLogger('MediaMetadataSync')

/** Commit shape this module reads; structurally satisfied by the command executor's event. */
export interface MediaCommitEvent {
  kind: 'execute' | 'undo' | 'redo'
  commands: ReadonlyArray<{ type: string }>
  before: { mediaItems?: ReadonlyArray<MediaMetadata> }
  after: { mediaItems?: ReadonlyArray<MediaMetadata> }
}

export interface MediaCommitSource {
  on(event: 'commit', listener: (event: MediaCommitEvent) => void): () => void
}

export interface MediaDurablePatch {
  mediaId: string
  tags?: string[]
  attribution?: MediaAttribution | null
}

/** Commands whose resulting records are not user edits and must never be written back. */
const NON_EDIT_COMMANDS: Record<string, true> = {
  'media.setRecords': true,
  'media.add': true,
  'media.removeRecords': true,
}

function sameTags(a: readonly string[], b: readonly string[]): boolean {
  return a.length === b.length && a.every((tag, index) => tag === b[index])
}

function sameAttribution(a: MediaAttribution | undefined, b: MediaAttribution | undefined): boolean {
  if (a === b) return true
  if (!a || !b) return false
  return (
    a.provider === b.provider &&
    a.author === b.author &&
    a.authorUrl === b.authorUrl &&
    a.sourceUrl === b.sourceUrl &&
    a.sourceId === b.sourceId &&
    a.license === b.license
  )
}

/**
 * Tags/attribution changes between two commits' media records, for media present in both.
 * Added and removed records are not changes of existing metadata.
 */
export function diffDurableMediaMetadata(
  before: ReadonlyArray<MediaMetadata> | undefined,
  after: ReadonlyArray<MediaMetadata> | undefined,
): MediaDurablePatch[] {
  if (!before || !after) return []

  const beforeById = new Map(before.map((media) => [media.id, media]))
  const patches: MediaDurablePatch[] = []
  for (const media of after) {
    const previous = beforeById.get(media.id)
    if (!previous) continue

    const patch: MediaDurablePatch = { mediaId: media.id }
    if (!sameTags(previous.tags, media.tags)) patch.tags = [...media.tags]
    if (!sameAttribution(previous.attribution, media.attribution)) {
      patch.attribution = media.attribution ?? null
    }
    if (patch.tags !== undefined || patch.attribution !== undefined) patches.push(patch)
  }
  return patches
}

function toUpdate(patch: MediaDurablePatch): MediaMetadataUpdate {
  const update: MediaMetadataUpdate = {}
  if (patch.tags !== undefined) update.tags = patch.tags
  if (patch.attribution !== undefined) update.attribution = patch.attribution
  return update
}

/**
 * Subscribe to commits and write durable metadata changes to the server, one media at a time in
 * commit order so a later edit can never be overwritten by an earlier, slower request.
 *
 * `onFailure` receives the media whose server copy could not be updated; the in-memory records
 * keep the user's edit, so the caller decides how to surface the divergence.
 * @returns Unsubscribe function.
 */
export function startMediaMetadataSync(
  source: MediaCommitSource,
  onFailure?: (mediaId: string, error: unknown) => void,
): () => void {
  const queues = new Map<string, Promise<void>>()

  const enqueue = (patch: MediaDurablePatch): void => {
    const previous = queues.get(patch.mediaId) ?? Promise.resolve()
    const next = previous
      .then(() => updateMedia(patch.mediaId, toUpdate(patch)))
      .then(
        () => undefined,
        (error: unknown) => {
          logger.error(`Failed to persist metadata for media ${patch.mediaId}:`, error)
          onFailure?.(patch.mediaId, error)
        },
      )
      .finally(() => {
        if (queues.get(patch.mediaId) === next) queues.delete(patch.mediaId)
      })
    queues.set(patch.mediaId, next)
  }

  return source.on('commit', (event) => {
    if (
      event.commands.length > 0 &&
      event.commands.every((command) => NON_EDIT_COMMANDS[command.type] === true)
    ) {
      return
    }
    for (const patch of diffDurableMediaMetadata(event.before.mediaItems, event.after.mediaItems)) {
      enqueue(patch)
    }
  })
}
