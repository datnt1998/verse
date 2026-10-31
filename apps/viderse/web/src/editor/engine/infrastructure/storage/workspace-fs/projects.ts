/**
 * Projects, backed by the server `EditorStorage` (`/api/editor/projects`).
 *
 * Revision handling lives in `@editor/storage/project-persistence`: a project
 * loaded through `getProject` keeps its snapshot and revision, and
 * `updateProject` merges into that snapshot and saves with the revision it was
 * loaded with. Deletion is permanent — there is no trash.
 */

import type { Project } from '@editor/types/project'
import { createLogger } from '@editor/shared/logging/logger'
import { StorageClientError } from '@editor/storage/errors'
import * as persistence from '@editor/storage/project-persistence'
import { getEditorStorage } from '@editor/storage/storage-backend'

export {
  acceptServerRevision,
  ensureProject,
  getProjectRevision,
  getProjectSnapshot,
} from '@editor/storage/project-persistence'

const logger = createLogger('WorkspaceFS:Projects')

/** Concurrent project downloads while building the list. */
const LIST_CONCURRENCY = 6

/** Fields a new project is created with; the server assigns the ID. */
export interface NewProjectInput {
  name: string
  description?: string
  width: number
  height: number
  fps: number
  backgroundColor?: string
}

/**
 * Every project, newest first. Snapshots are not retained for list entries —
 * only `getProject` establishes the base revision used for saving.
 */
export async function getAllProjects(): Promise<Project[]> {
  const storage = getEditorStorage()
  try {
    const summaries = await storage.listProjects()
    const projects: Project[] = []
    let next = 0
    const worker = async () => {
      while (next < summaries.length) {
        const summary = summaries[next++]!
        try {
          projects.push((await storage.loadProject(summary.id)).data)
        } catch (error) {
          // Deleted between listing and loading.
          if (!(error instanceof StorageClientError && error.code === 'not_found')) throw error
          logger.warn(`getAllProjects: project ${summary.id} disappeared`, error)
        }
      }
    }
    await Promise.all(Array.from({ length: Math.min(LIST_CONCURRENCY, summaries.length) }, worker))
    return projects.sort((a, b) => b.updatedAt - a.updatedAt)
  } catch (error) {
    logger.error('getAllProjects failed', error)
    throw new Error('Failed to load projects from server', { cause: error })
  }
}

/** Load from the server, retaining the snapshot/revision and clearing any conflict. */
export async function getProject(id: string): Promise<Project | undefined> {
  try {
    return await persistence.loadProject(id)
  } catch (error) {
    logger.error(`getProject(${id}) failed`, error)
    throw error
  }
}

/** Create on the server (which assigns the ID), then apply the form's fields. */
export async function createProject(input: NewProjectInput): Promise<Project> {
  const created = await persistence.createProject({ name: input.name })
  const metadata = {
    ...created.metadata,
    width: input.width,
    height: input.height,
    fps: input.fps,
    ...(input.backgroundColor !== undefined && { backgroundColor: input.backgroundColor }),
  }
  const description = input.description ?? ''
  const unchanged =
    description === created.description &&
    metadata.width === created.metadata.width &&
    metadata.height === created.metadata.height &&
    metadata.fps === created.metadata.fps &&
    metadata.backgroundColor === created.metadata.backgroundColor
  if (unchanged) return created
  try {
    await persistence.updateProject(created.id, { description, metadata })
    return persistence.getProjectSnapshot(created.id) ?? created
  } catch (error) {
    // Do not leave a half-configured project behind.
    await persistence.deleteProject(created.id).catch((cleanupError) => {
      logger.warn(`createProject: could not remove ${created.id} after failed setup`, cleanupError)
    })
    throw error
  }
}

/** Merge `updates` into the loaded snapshot and save with its original revision. */
export async function updateProject(id: string, updates: Partial<Project>): Promise<Project> {
  try {
    return await persistence.updateProject(id, updates)
  } catch (error) {
    logger.error(`updateProject(${id}) failed`, error)
    throw error
  }
}

/** Server-side duplicate: sidecars and media links follow the copy. */
export async function duplicateProject(id: string, name?: string): Promise<Project> {
  try {
    return await persistence.duplicateProject(id, name)
  } catch (error) {
    logger.error(`duplicateProject(${id}) failed`, error)
    throw error
  }
}

/** Permanent delete. */
export async function deleteProject(id: string): Promise<void> {
  try {
    await persistence.deleteProject(id)
  } catch (error) {
    logger.error(`deleteProject(${id}) failed`, error)
    throw new Error(`Failed to delete project: ${id}`, { cause: error })
  }
}
