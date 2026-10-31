// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Project } from '@editor/types/project'
import {
  getStorageConflict,
  storageConflictEvents,
  useStorageConflictStore,
  type StorageConflict,
} from './conflict-store'
import { StorageClientError, StorageConflictError, isStorageConflict } from './errors'
import {
  acceptServerRevision,
  assertProjectWritable,
  createProject,
  deleteProject,
  duplicateProject,
  ensureProject,
  getProjectRevision,
  getProjectSnapshot,
  loadProject,
  resetProjectPersistenceForTesting,
  updateProject,
} from './project-persistence'

const storage = vi.hoisted(() => ({
  loadProject: vi.fn(),
  saveProject: vi.fn(),
  createProject: vi.fn(),
  duplicateProject: vi.fn(),
  deleteProject: vi.fn(),
}))

vi.mock('./storage-backend', () => ({ getEditorStorage: () => storage }))

function makeProject(id = 'p1', overrides: Partial<Project> = {}): Project {
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

describe('project persistence revisions', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    resetProjectPersistenceForTesting()
    useStorageConflictStore.setState({ conflicts: {} })
    storage.loadProject.mockResolvedValue({ data: makeProject(), revision: 'r1' })
  })

  it('saves with the revision captured at load and never refetches a base', async () => {
    storage.saveProject.mockResolvedValue('r2')
    await loadProject('p1')

    await updateProject('p1', { name: 'Renamed' })

    expect(storage.loadProject).toHaveBeenCalledTimes(1)
    expect(storage.saveProject).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'p1', name: 'Renamed' }),
      'r1',
    )
    expect(getProjectRevision('p1')).toBe('r2')
  })

  it('chains the server-returned revision across consecutive saves', async () => {
    storage.saveProject.mockResolvedValueOnce('r2').mockResolvedValueOnce('r3')
    await loadProject('p1')

    await updateProject('p1', { name: 'A' })
    await updateProject('p1', { description: 'B' })

    expect(storage.saveProject.mock.calls.map((call) => call[1])).toEqual(['r1', 'r2'])
    expect(storage.loadProject).toHaveBeenCalledTimes(1)
  })

  it('merges updates into the retained snapshot', async () => {
    storage.saveProject.mockResolvedValue('r2')
    await loadProject('p1')

    await updateProject('p1', { name: 'A' })
    const saved = await updateProject('p1', { description: 'B' })

    expect(saved).toMatchObject({ id: 'p1', name: 'A', description: 'B' })
    expect(getProjectSnapshot('p1')).toBe(saved)
  })

  it('serializes concurrent saves so the second uses the first save\'s revision', async () => {
    let finishFirst!: (revision: string) => void
    storage.saveProject
      .mockImplementationOnce(
        () => new Promise<string>((resolve) => { finishFirst = resolve }),
      )
      .mockResolvedValueOnce('r3')
    await loadProject('p1')

    const first = updateProject('p1', { name: 'A' })
    const second = updateProject('p1', { description: 'B' })
    await vi.waitFor(() => expect(storage.saveProject).toHaveBeenCalledTimes(1))
    // The second save waits for the first instead of reusing revision r1.
    await Promise.resolve()
    expect(storage.saveProject).toHaveBeenCalledTimes(1)
    finishFirst('r2')
    await Promise.all([first, second])

    expect(storage.saveProject.mock.calls.map((call) => call[1])).toEqual(['r1', 'r2'])
  })

  it('rejects saves for a project that was never loaded without touching the server', async () => {
    await expect(updateProject('p1', { name: 'A' })).rejects.toThrow('not loaded')
    expect(storage.saveProject).not.toHaveBeenCalled()
  })

  it('ensureProject reuses the snapshot and only loads when absent', async () => {
    await ensureProject('p1')
    await ensureProject('p1')
    expect(storage.loadProject).toHaveBeenCalledTimes(1)
  })

  it('returns undefined and forgets the snapshot when the server reports not_found', async () => {
    await loadProject('p1')
    storage.loadProject.mockRejectedValue(new StorageClientError('not_found', 'gone', 404))

    await expect(loadProject('p1')).resolves.toBeUndefined()
    expect(getProjectSnapshot('p1')).toBeUndefined()
  })
})

describe('project persistence conflicts', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    resetProjectPersistenceForTesting()
    useStorageConflictStore.setState({ conflicts: {} })
    storage.loadProject.mockResolvedValue({ data: makeProject(), revision: 'r1' })
  })

  it('records a typed conflict, emits the event and rethrows on 409', async () => {
    const heard: StorageConflict[] = []
    const unsubscribe = storageConflictEvents.on('conflict', (conflict) => heard.push(conflict))
    storage.saveProject.mockRejectedValue(new StorageConflictError('stale', 'r7'))
    await loadProject('p1')

    const failure = await updateProject('p1', { name: 'Mine' }).catch((error: unknown) => error)
    unsubscribe()

    expect(isStorageConflict(failure)).toBe(true)
    expect(getStorageConflict('p1')).toEqual({
      projectId: 'p1',
      baseRevision: 'r1',
      serverRevision: 'r7',
      timestamp: expect.any(Number),
    })
    expect(heard).toEqual([getStorageConflict('p1')])
    // The rejected merge is not retained as the snapshot.
    expect(getProjectSnapshot('p1')?.name).toBe('Project p1')
    expect(getProjectRevision('p1')).toBe('r1')
  })

  it('blocks every later write while the conflict is unresolved', async () => {
    storage.saveProject.mockRejectedValueOnce(new StorageConflictError('stale', 'r7'))
    await loadProject('p1')
    await updateProject('p1', { name: 'Mine' }).catch(() => undefined)
    storage.saveProject.mockClear()

    await expect(updateProject('p1', { name: 'Again' })).rejects.toBeInstanceOf(StorageConflictError)
    expect(() => assertProjectWritable('p1')).toThrow(StorageConflictError)
    expect(storage.saveProject).not.toHaveBeenCalled()
  })

  it('does not record a conflict for non-409 failures', async () => {
    storage.saveProject.mockRejectedValue(new Error('network down'))
    await loadProject('p1')

    await expect(updateProject('p1', { name: 'A' })).rejects.toThrow('network down')
    expect(getStorageConflict('p1')).toBeUndefined()
    expect(getProjectRevision('p1')).toBe('r1')
  })

  it('loading again resets the conflict and adopts the server revision and data', async () => {
    storage.saveProject.mockRejectedValueOnce(new StorageConflictError('stale', 'r7'))
    await loadProject('p1')
    await updateProject('p1', { name: 'Mine' }).catch(() => undefined)

    storage.loadProject.mockResolvedValue({ data: makeProject('p1', { name: 'Theirs' }), revision: 'r7' })
    await loadProject('p1')

    expect(getStorageConflict('p1')).toBeUndefined()
    expect(getProjectSnapshot('p1')?.name).toBe('Theirs')
    expect(getProjectRevision('p1')).toBe('r7')
  })

  it('keep-mine only happens through an explicit acceptServerRevision call', async () => {
    storage.saveProject.mockRejectedValueOnce(new StorageConflictError('stale', 'r7'))
    await loadProject('p1')
    await updateProject('p1', { name: 'Mine' }).catch(() => undefined)
    // Nothing adopts the server revision implicitly.
    expect(getProjectRevision('p1')).toBe('r1')

    await acceptServerRevision('p1')
    expect(getStorageConflict('p1')).toBeUndefined()
    expect(getProjectRevision('p1')).toBe('r7')

    storage.saveProject.mockResolvedValue('r8')
    await updateProject('p1', { name: 'Mine' })
    expect(storage.saveProject).toHaveBeenLastCalledWith(expect.objectContaining({ name: 'Mine' }), 'r7')
  })

  it('looks up the server revision when the 409 did not carry one', async () => {
    storage.saveProject.mockRejectedValueOnce(new StorageConflictError('stale', null))
    await loadProject('p1')
    await updateProject('p1', { name: 'Mine' }).catch(() => undefined)

    storage.loadProject.mockResolvedValue({ data: makeProject('p1', { name: 'Theirs' }), revision: 'r9' })
    await acceptServerRevision('p1')

    expect(getProjectRevision('p1')).toBe('r9')
    // The local snapshot is kept; only the base moved.
    expect(getProjectSnapshot('p1')?.name).toBe('Project p1')
  })

  it('acceptServerRevision is a no-op without a conflict', async () => {
    await loadProject('p1')
    await acceptServerRevision('p1')
    expect(getProjectRevision('p1')).toBe('r1')
  })

  it('keeps conflicts independent per project', async () => {
    storage.saveProject.mockRejectedValueOnce(new StorageConflictError('stale', 'r7'))
    await loadProject('p1')
    await updateProject('p1', { name: 'Mine' }).catch(() => undefined)

    expect(() => assertProjectWritable('p2')).not.toThrow()
  })
})

describe('project persistence lifecycle', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    resetProjectPersistenceForTesting()
    useStorageConflictStore.setState({ conflicts: {} })
  })

  it('retains the server-assigned project from create and duplicate', async () => {
    storage.createProject.mockResolvedValue({ data: makeProject('srv-1'), revision: 'c1' })
    storage.duplicateProject.mockResolvedValue({ data: makeProject('srv-2'), revision: 'd1' })
    storage.saveProject.mockResolvedValue('c2')

    const created = await createProject({ name: 'New' })
    const copy = await duplicateProject('srv-1', 'Copy')

    expect(created.id).toBe('srv-1')
    expect(copy.id).toBe('srv-2')
    expect(storage.duplicateProject).toHaveBeenCalledWith('srv-1', 'Copy')
    await updateProject('srv-1', { name: 'Edited' })
    expect(storage.saveProject).toHaveBeenCalledWith(expect.objectContaining({ id: 'srv-1' }), 'c1')
  })

  it('delete forgets the snapshot and any conflict', async () => {
    storage.loadProject.mockResolvedValue({ data: makeProject(), revision: 'r1' })
    storage.saveProject.mockRejectedValueOnce(new StorageConflictError('stale', 'r7'))
    storage.deleteProject.mockResolvedValue(undefined)
    await loadProject('p1')
    await updateProject('p1', { name: 'Mine' }).catch(() => undefined)

    await deleteProject('p1')

    expect(getProjectSnapshot('p1')).toBeUndefined()
    expect(getStorageConflict('p1')).toBeUndefined()
  })
})
