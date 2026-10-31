// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Project } from '@editor/types/project'
import { StorageClientError } from '@editor/storage/errors'
import { resetProjectPersistenceForTesting } from '@editor/storage/project-persistence'
import {
  createProject,
  deleteProject,
  duplicateProject,
  getAllProjects,
  getProject,
  getProjectRevision,
  getProjectSnapshot,
  updateProject,
} from './projects'

const storage = vi.hoisted(() => ({
  listProjects: vi.fn(),
  loadProject: vi.fn(),
  createProject: vi.fn(),
  saveProject: vi.fn(),
  duplicateProject: vi.fn(),
  deleteProject: vi.fn(),
}))

vi.mock('@editor/storage/storage-backend', () => ({ getEditorStorage: () => storage }))

function makeProject(id: string, overrides: Partial<Project> = {}): Project {
  return {
    id,
    name: `Project ${id}`,
    description: '',
    createdAt: 1,
    updatedAt: 1,
    duration: 0,
    metadata: { width: 1920, height: 1080, fps: 30 },
    ...overrides,
  }
}

const form = { name: 'Film', description: 'About', width: 1080, height: 1920, fps: 24 }

describe('workspace-fs projects', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    resetProjectPersistenceForTesting()
  })

  describe('getAllProjects', () => {
    it('lists newest first and does not retain snapshots from the list', async () => {
      storage.listProjects.mockResolvedValue([{ id: 'a' }, { id: 'b' }])
      storage.loadProject.mockImplementation(async (id: string) => ({
        data: makeProject(id, { updatedAt: id === 'a' ? 10 : 20 }),
        revision: `rev-${id}`,
      }))

      const projects = await getAllProjects()

      expect(projects.map((project) => project.id)).toEqual(['b', 'a'])
      expect(getProjectSnapshot('a')).toBeUndefined()
      expect(getProjectRevision('b')).toBeUndefined()
    })

    it('skips a project deleted between listing and loading', async () => {
      storage.listProjects.mockResolvedValue([{ id: 'a' }, { id: 'gone' }])
      storage.loadProject.mockImplementation(async (id: string) => {
        if (id === 'gone') throw new StorageClientError('not_found', 'gone', 404)
        return { data: makeProject(id), revision: 'r' }
      })

      await expect(getAllProjects()).resolves.toEqual([expect.objectContaining({ id: 'a' })])
    })

    it('fails the whole list on a non-not_found error', async () => {
      storage.listProjects.mockResolvedValue([{ id: 'a' }])
      storage.loadProject.mockRejectedValue(new Error('network'))
      await expect(getAllProjects()).rejects.toThrow('Failed to load projects')
    })
  })

  describe('getProject', () => {
    it('retains snapshot and revision for later saves', async () => {
      storage.loadProject.mockResolvedValue({ data: makeProject('p1'), revision: 'r1' })
      storage.saveProject.mockResolvedValue('r2')

      await getProject('p1')
      await updateProject('p1', { timeline: { tracks: [], items: [] } })

      expect(storage.saveProject).toHaveBeenCalledWith(
        expect.objectContaining({ id: 'p1', timeline: { tracks: [], items: [] } }),
        'r1',
      )
    })

    it('returns undefined for a missing project', async () => {
      storage.loadProject.mockRejectedValue(new StorageClientError('not_found', 'gone', 404))
      await expect(getProject('nope')).resolves.toBeUndefined()
    })
  })

  describe('createProject', () => {
    it('uses the server-assigned id and applies the form with the created revision', async () => {
      storage.createProject.mockResolvedValue({
        data: makeProject('srv-1', { name: 'Film' }),
        revision: 'c1',
      })
      storage.saveProject.mockResolvedValue('c2')

      const project = await createProject(form)

      expect(storage.createProject).toHaveBeenCalledWith({ name: 'Film' })
      expect(project.id).toBe('srv-1')
      expect(project).toMatchObject({
        description: 'About',
        metadata: { width: 1080, height: 1920, fps: 24 },
      })
      expect(storage.saveProject).toHaveBeenCalledWith(
        expect.objectContaining({ id: 'srv-1', description: 'About' }),
        'c1',
      )
      expect(getProjectRevision('srv-1')).toBe('c2')
    })

    it('skips the second request when the server defaults already match the form', async () => {
      storage.createProject.mockResolvedValue({
        data: makeProject('srv-1', { name: 'Film', description: '' }),
        revision: 'c1',
      })

      await createProject({ name: 'Film', width: 1920, height: 1080, fps: 30 })

      expect(storage.saveProject).not.toHaveBeenCalled()
    })

    it('removes the half-configured project when applying the form fails', async () => {
      storage.createProject.mockResolvedValue({ data: makeProject('srv-1'), revision: 'c1' })
      storage.saveProject.mockRejectedValue(new Error('network'))
      storage.deleteProject.mockResolvedValue(undefined)

      await expect(createProject(form)).rejects.toThrow('network')

      expect(storage.deleteProject).toHaveBeenCalledWith('srv-1')
    })
  })

  it('duplicates through the server API so sidecars follow the copy', async () => {
    storage.duplicateProject.mockResolvedValue({ data: makeProject('copy'), revision: 'd1' })

    const copy = await duplicateProject('orig', 'Copy of orig')

    expect(storage.duplicateProject).toHaveBeenCalledWith('orig', 'Copy of orig')
    expect(copy.id).toBe('copy')
    expect(getProjectRevision('copy')).toBe('d1')
  })

  it('deletes permanently on the server and forgets the snapshot', async () => {
    storage.loadProject.mockResolvedValue({ data: makeProject('p1'), revision: 'r1' })
    storage.deleteProject.mockResolvedValue(undefined)
    await getProject('p1')

    await deleteProject('p1')

    expect(storage.deleteProject).toHaveBeenCalledWith('p1')
    expect(getProjectSnapshot('p1')).toBeUndefined()
  })

  it('wraps delete failures with the project id', async () => {
    storage.deleteProject.mockRejectedValue(new Error('locked'))
    await expect(deleteProject('p1')).rejects.toThrow('Failed to delete project: p1')
  })
})
