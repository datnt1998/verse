/**
 * Cache orphan sweeper.
 *
 * Per-media caches live under `media/<mediaId>/<contentHash>/...` in the
 * OPFS cache. Two things leave dead directories behind:
 *   - a media is deleted on the server (its whole `media/<mediaId>` tree is dead);
 *   - a media's bytes are replaced (the previous `<contentHash>` directory is
 *     dead — nothing will ever read it again).
 *
 * The server's media list is the only authority on what is live, so the sweep
 * compares the OPFS tree against `listMedia()` and removes everything else.
 * It never touches live entries and never touches durable data (there is none
 * in the cache).
 *
 * Usage:
 *   - Dev / debug: `window.__DEBUG__.cleanOrphans({ dryRun: true })`
 *     then `window.__DEBUG__.cleanOrphans()` to actually remove.
 */

import { createLogger } from '@editor/shared/logging/logger'
import { cacheDirectory } from '@editor/storage/opfs-cache'
import { getEditorStorage } from '@editor/storage/storage-backend'

const logger = createLogger('WorkspaceFS:OrphanSweep')

export interface OrphanSweepReport {
  liveMediaCount: number
  /** Total cache directories that were (or would be, in dry-run) removed. */
  totalRemoved: number
  /** Dry-run = report only, do not touch disk. */
  dryRun: boolean
  durationMs: number
}

export interface OrphanSweepOptions {
  dryRun?: boolean
}

/** The slice of an OPFS directory handle the sweep enumerates. */
interface EnumerableDirectory {
  values(): AsyncIterable<{ kind: 'file' | 'directory'; name: string }>
}

async function directoryNames(directory: EnumerableDirectory): Promise<string[]> {
  const names: string[] = []
  for await (const entry of directory.values()) {
    if (entry.kind === 'directory') names.push(entry.name)
  }
  return names
}

export async function sweepWorkspaceOrphans(
  options: OrphanSweepOptions = {},
): Promise<OrphanSweepReport> {
  const dryRun = options.dryRun ?? false
  const started = Date.now()

  const liveMedia = await getEditorStorage().listMedia()
  // OPFS directory names are the URI-encoded ids/hashes the cache layer wrote.
  const liveHashByDir = new Map(
    liveMedia.map((media) => [encodeURIComponent(media.id), encodeURIComponent(media.contentHash)]),
  )

  let totalRemoved = 0
  const mediaRoot = await cacheDirectory(['media'], false).catch((error: unknown) => {
    // No media cached yet — nothing to sweep.
    if (error instanceof DOMException && error.name === 'NotFoundError') return null
    throw error
  })

  for (const mediaDirName of mediaRoot ? await directoryNames(mediaRoot) : []) {
    const liveHash = liveHashByDir.get(mediaDirName)
    if (liveHash === undefined) {
      totalRemoved += 1
      if (!dryRun) await mediaRoot!.removeEntry(mediaDirName, { recursive: true })
      continue
    }
    const mediaDir = await mediaRoot!.getDirectoryHandle(mediaDirName)
    for (const version of await directoryNames(mediaDir)) {
      if (version === liveHash) continue
      totalRemoved += 1
      if (!dryRun) await mediaDir.removeEntry(version, { recursive: true })
    }
  }

  const report: OrphanSweepReport = {
    liveMediaCount: liveMedia.length,
    totalRemoved,
    dryRun,
    durationMs: Date.now() - started,
  }
  logger.info(
    `Orphan sweep ${dryRun ? '(dry run) ' : ''}removed ${totalRemoved} dead cache director${totalRemoved === 1 ? 'y' : 'ies'}`,
  )
  return report
}
