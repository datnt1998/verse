/**
 * Per-project render-queue persistence.
 *
 * The queue is the server `render-queue` sidecar so a page refresh (or opening
 * the project in another tab) restores the queued/finished jobs. The schema is
 * owned by the export feature; this layer reads/writes the JSON object keyed by
 * project id. Saves are based on the revision retained by the last load/save —
 * call {@link loadRenderQueue} before {@link saveRenderQueue}.
 */

import { createSidecarChannel } from '@editor/storage/project-persistence'

const renderQueue = createSidecarChannel('render-queue')

/** The persisted queue, or `null` when none has been saved for the project yet. */
export async function loadRenderQueue<T>(projectId: string): Promise<T | null> {
  const { data, revision } = await renderQueue.load<T>(projectId)
  // The server answers a never-written sidecar with `{}` at revision `empty`.
  return revision === 'empty' ? null : data
}

export function saveRenderQueue(projectId: string, data: unknown): Promise<void> {
  return renderQueue.save(projectId, data)
}
