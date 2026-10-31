import type { Project } from '@editor/types/project'
import { getStorageConflict, useStorageConflictStore } from './conflict-store'
import { StorageClientError, StorageConflictError, isStorageConflict } from './errors'
import { getEditorStorage, type Revisioned, type SidecarName } from './storage-backend'

/**
 * Client-side persistence state for server-owned projects.
 *
 * The snapshot and revision captured when a project is loaded are the only
 * base a save ever uses. Nothing here refreshes `baseRevision` before an
 * autosave: doing so would silently overwrite a second tab's newer changes.
 * A 409 records a typed conflict and blocks every later write for that project
 * until `loadProject` (take theirs) or `acceptServerRevision` (keep mine).
 */

interface LoadedProject {
  project: Project
  revision: string
}

const loaded = new Map<string, LoadedProject>()
const queues = new Map<string, Promise<unknown>>()

/** Serialize work per key; a failed task never poisons the next one. */
export async function serializeWrites<T>(key: string, task: () => Promise<T>): Promise<T> {
  const previous = queues.get(key) ?? Promise.resolve()
  const result = previous.catch(() => undefined).then(task)
  const tail = result.catch(() => undefined)
  queues.set(key, tail)
  try {
    return await result
  } finally {
    if (queues.get(key) === tail) queues.delete(key)
  }
}

const projectKey = (id: string) => `project:${id}`

function isNotFound(error: unknown): boolean {
  return error instanceof StorageClientError && error.code === 'not_found'
}

/** Throws `StorageConflictError` while the project has an unresolved conflict. */
export function assertProjectWritable(projectId: string): void {
  const conflict = getStorageConflict(projectId)
  if (!conflict) return
  throw new StorageConflictError(
    'Dự án đã được thay đổi ở nơi khác; hãy tải lại hoặc giữ bản của bạn trước khi lưu tiếp.',
    conflict.serverRevision,
  )
}

/** Fetch the project from the server and retain it as the new snapshot. Resets any conflict. */
export function loadProject(id: string): Promise<Project | undefined> {
  return serializeWrites(projectKey(id), async () => {
    try {
      const { data, revision } = await getEditorStorage().loadProject(id)
      loaded.set(id, { project: data, revision })
      useStorageConflictStore.getState().clearConflict(id)
      return data
    } catch (error) {
      if (!isNotFound(error)) throw error
      loaded.delete(id)
      useStorageConflictStore.getState().clearConflict(id)
      return undefined
    }
  })
}

/** Snapshot retained from the last load/save, or undefined if the project was never loaded. */
export function getProjectSnapshot(id: string): Project | undefined {
  return loaded.get(id)?.project
}

/** Revision the next save will send as `If-Match`. */
export function getProjectRevision(id: string): string | undefined {
  return loaded.get(id)?.revision
}

/** The retained snapshot, loading it only when this tab never loaded the project. */
export async function ensureProject(id: string): Promise<Project | undefined> {
  return loaded.get(id)?.project ?? loadProject(id)
}

/**
 * Merge `updates` into the retained snapshot and save with the snapshot's
 * original revision. Rejects with `StorageConflictError` when the project is
 * in conflict or the server rejects the revision (which records the conflict).
 */
export function updateProject(id: string, updates: Partial<Project>): Promise<Project> {
  return serializeWrites(projectKey(id), async () => {
    assertProjectWritable(id)
    const entry = loaded.get(id)
    if (!entry) throw new Error(`Project not loaded: ${id}`)

    const next: Project = { ...entry.project, ...updates, id, updatedAt: Date.now() }
    let revision: string
    try {
      revision = await getEditorStorage().saveProject(next, entry.revision)
    } catch (error) {
      if (isStorageConflict(error)) {
        useStorageConflictStore.getState().reportConflict({
          projectId: id,
          baseRevision: entry.revision,
          serverRevision: error.serverRevision,
          timestamp: Date.now(),
        })
      }
      throw error
    }
    loaded.set(id, { project: next, revision })
    return next
  })
}

/**
 * Keep-mine resolution: adopt the server's current revision as the base so the
 * next save overwrites it with local state. Must be called explicitly; autosave
 * never does this on its own.
 */
export function acceptServerRevision(id: string): Promise<void> {
  return serializeWrites(projectKey(id), async () => {
    const conflict = getStorageConflict(id)
    if (!conflict) return
    const entry = loaded.get(id)
    if (!entry) throw new Error(`Project not loaded: ${id}`)
    const revision = conflict.serverRevision ?? (await getEditorStorage().loadProject(id)).revision
    loaded.set(id, { project: entry.project, revision })
    useStorageConflictStore.getState().clearConflict(id)
  })
}

/** Create on the server and retain the returned project and revision. */
export async function createProject(input: { name: string; template?: string }): Promise<Project> {
  const { data, revision } = await getEditorStorage().createProject(input)
  loaded.set(data.id, { project: data, revision })
  return data
}

/** Duplicate on the server (sidecars and media links follow) and retain the copy. */
export async function duplicateProject(id: string, name?: string): Promise<Project> {
  const { data, revision } = await getEditorStorage().duplicateProject(id, name)
  loaded.set(data.id, { project: data, revision })
  return data
}

/** Permanent server delete; drops the snapshot and any conflict. */
export function deleteProject(id: string): Promise<void> {
  return serializeWrites(projectKey(id), async () => {
    await getEditorStorage().deleteProject(id)
    loaded.delete(id)
    useStorageConflictStore.getState().clearConflict(id)
  })
}

/** Test seam: forget every retained snapshot and queued write. */
export function resetProjectPersistenceForTesting(): void {
  loaded.clear()
  queues.clear()
}

/* ------------------------------ sidecars ------------------------------ */

export interface SidecarChannel {
  /** Fetch from the server and retain the revision later saves are based on. */
  load: <T>(projectId: string) => Promise<Revisioned<T>>
  /** Save against the revision retained by `load` or the previous save. */
  save: (projectId: string, data: unknown) => Promise<void>
}

/**
 * One revision map per sidecar. A save never refetches its base: without a
 * prior `load` it fails rather than overwrite unseen server state, and a 409
 * keeps failing until the sidecar is loaded again.
 */
export function createSidecarChannel(name: SidecarName): SidecarChannel {
  const revisions = new Map<string, string>()
  const key = (projectId: string) => `sidecar:${name}:${projectId}`
  return {
    load: <T>(projectId: string) =>
      serializeWrites(key(projectId), async () => {
        const result = await getEditorStorage().loadSidecar<T>(projectId, name)
        revisions.set(projectId, result.revision)
        return result
      }),
    save: (projectId, data) =>
      serializeWrites(key(projectId), async () => {
        const base = revisions.get(projectId)
        if (base === undefined) throw new Error(`Sidecar ${name} chưa được tải cho dự án ${projectId}`)
        revisions.set(projectId, await getEditorStorage().saveSidecar(projectId, name, data, base))
      }),
  }
}
