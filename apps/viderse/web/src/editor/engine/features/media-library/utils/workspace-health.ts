import { getEditorStorage } from '@editor/storage/storage-backend'
import type { MediaMetadata } from '@editor/types/storage'
import type { BrokenMediaInfo } from '../types'

export interface WorkspaceHealthScanSummary {
  healthyIds: string[]
  broken: BrokenMediaInfo[]
}

/**
 * Check which of `mediaItems` still exist on the server. The server owns every source byte,
 * so a media is healthy exactly when its record is listed; there is no per-device permission
 * or handle state to validate. One listing covers the whole batch.
 *
 * @param projectId - Scope the listing to a project's media; omit for the whole library.
 */
export async function scanWorkspaceMediaHealth(
  mediaItems: MediaMetadata[],
  projectId?: string,
): Promise<WorkspaceHealthScanSummary> {
  if (mediaItems.length === 0) return { healthyIds: [], broken: [] }

  const records = await getEditorStorage().listMedia(projectId)
  const presentIds = new Set(records.map((record) => record.id))

  const healthyIds: string[] = []
  const broken: BrokenMediaInfo[] = []
  for (const media of mediaItems) {
    if (presentIds.has(media.id)) {
      healthyIds.push(media.id)
    } else {
      broken.push({ mediaId: media.id, fileName: media.fileName, errorType: 'file_missing' })
    }
  }

  return { healthyIds, broken }
}
