/**
 * Project ↔ media links, owned by the server.
 *
 * `listMedia(projectId)` is the single source of truth for what a project references;
 * link and unlink are server operations that serialize per project. The server has no
 * reverse index, so `getProjectsUsingMedia` scans project links on demand — it is only
 * called when deciding whether a media can be purged, never on a hot path.
 */

import type { MediaMetadata } from '@editor/types/storage'
import { getEditorStorage } from '@editor/storage/storage-backend'
import { StorageClientError } from '@editor/storage/errors'

import { hydrateServerMedia } from './media'

// Editor startup asks for project media from both the media-library hydrator and timeline
// orphan validation. Share concurrent reads so a cold open lists a project's media once.
const mediaForProjectInFlight = new Map<string, Promise<MediaMetadata[]>>()

export async function associateMediaWithProject(projectId: string, mediaId: string): Promise<void> {
  await getEditorStorage().linkMedia(projectId, mediaId)
}

export async function removeMediaFromProject(projectId: string, mediaId: string): Promise<void> {
  await getEditorStorage().unlinkMedia(projectId, mediaId)
}

export async function removeMediaBatchFromProject(
  projectId: string,
  mediaIds: string[],
): Promise<void> {
  // Sequential: each unlink takes the server's per-project lock.
  for (const mediaId of new Set(mediaIds.filter(Boolean))) {
    await getEditorStorage().unlinkMedia(projectId, mediaId)
  }
}

export async function getProjectMediaIds(projectId: string): Promise<string[]> {
  const records = await getEditorStorage().listMedia(projectId)
  return records.map((record) => record.id)
}

export async function getProjectsUsingMedia(mediaId: string): Promise<string[]> {
  const storage = getEditorStorage()
  const projects = await storage.listProjects()
  const linked = await Promise.all(
    projects.map(async (project) => {
      try {
        const records = await storage.listMedia(project.id)
        return records.some((record) => record.id === mediaId) ? project.id : null
      } catch (error) {
        // A project deleted mid-scan no longer references anything.
        if (error instanceof StorageClientError && error.code === 'not_found') return null
        throw error
      }
    }),
  )
  return linked.filter((projectId): projectId is string => projectId !== null)
}

export function getMediaForProject(projectId: string): Promise<MediaMetadata[]> {
  const existing = mediaForProjectInFlight.get(projectId)
  if (existing) return existing

  const load = getEditorStorage()
    .listMedia(projectId)
    .then((records) => Promise.all(records.map(hydrateServerMedia)))
    .finally(() => {
      mediaForProjectInFlight.delete(projectId)
    })
  mediaForProjectInFlight.set(projectId, load)
  return load
}
