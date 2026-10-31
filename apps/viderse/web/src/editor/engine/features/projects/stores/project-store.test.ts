// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Project } from '@editor/types/project'

const storageMocks = vi.hoisted(() => ({
  getAllProjects: vi.fn(),
  getProject: vi.fn(),
  ensureProject: vi.fn(),
  createProject: vi.fn(),
  updateProject: vi.fn(),
  deleteProject: vi.fn(),
  duplicateProject: vi.fn(),
}))

vi.mock('@editor/infrastructure/storage', () => storageMocks)

vi.mock('@editor/features/projects/deps/settings-contract', () => ({
  useSettingsStore: {
    getState: () => ({ maxUndoHistory: 100 }),
    subscribe: vi.fn(),
  },
}))

const { useProjectStore } = await import('./project-store')

function makeProject(id: string): Project {
  const now = Date.now()
  return {
    id,
    name: `Project ${id}`,
    description: '',
    createdAt: now,
    updatedAt: now,
    duration: 0,
    metadata: {
      width: 1920,
      height: 1080,
      fps: 30,
      backgroundColor: '#000000',
    },
  }
}

function resetStore() {
  vi.resetAllMocks()
  useProjectStore.setState({
    projects: [],
    currentProject: null,
    isLoading: false,
    error: null,
    searchQuery: '',
    sortField: 'updatedAt',
    sortDirection: 'desc',
    filterResolution: undefined,
    filterFps: undefined,
  })
}

describe('project-store deleteProject', () => {
  beforeEach(resetStore)

  it('keeps a deleted project pruned if a stale reload lands while the server delete is pending', async () => {
    const project = makeProject('p1')
    let finishDelete!: () => void
    storageMocks.deleteProject.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          finishDelete = resolve
        }),
    )

    useProjectStore.setState({ projects: [project], currentProject: project })

    const deletePromise = useProjectStore.getState().deleteProject(project.id)

    expect(useProjectStore.getState().projects).toEqual([])

    useProjectStore.setState({ projects: [project], currentProject: project })
    finishDelete()
    await deletePromise

    expect(storageMocks.deleteProject).toHaveBeenCalledWith('p1')
    expect(useProjectStore.getState().projects).toEqual([])
    expect(useProjectStore.getState().currentProject).toBeNull()
  })

  it('rolls the list back when the server refuses the delete', async () => {
    const project = makeProject('p1')
    storageMocks.deleteProject.mockRejectedValue(new Error('locked'))
    useProjectStore.setState({ projects: [project], currentProject: project })

    await expect(useProjectStore.getState().deleteProject('p1')).rejects.toThrow('locked')

    expect(useProjectStore.getState().projects).toEqual([project])
    expect(useProjectStore.getState().currentProject).toEqual(project)
    expect(useProjectStore.getState().error).toBe('locked')
  })
})

describe('project-store createProject', () => {
  beforeEach(resetStore)

  it('adds the server-assigned project and makes it current', async () => {
    const created = makeProject('server-id')
    storageMocks.createProject.mockResolvedValue(created)

    const result = await useProjectStore
      .getState()
      .createProject({ name: 'Film', description: 'd', width: 1920, height: 1080, fps: 30 })

    expect(result.id).toBe('server-id')
    expect(storageMocks.createProject).toHaveBeenCalledWith({
      name: 'Film',
      description: 'd',
      width: 1920,
      height: 1080,
      fps: 30,
      backgroundColor: undefined,
    })
    expect(useProjectStore.getState().projects).toEqual([created])
    expect(useProjectStore.getState().currentProject).toBe(created)
  })

  it('leaves the list untouched and records the error when creation fails', async () => {
    storageMocks.createProject.mockRejectedValue(new Error('disk full'))

    await expect(
      useProjectStore
        .getState()
        .createProject({ name: 'Film', width: 1920, height: 1080, fps: 30 }),
    ).rejects.toThrow('disk full')

    expect(useProjectStore.getState().projects).toEqual([])
    expect(useProjectStore.getState().error).toBe('disk full')
  })
})

describe('project-store duplicateProject', () => {
  beforeEach(resetStore)

  it('uses the server duplicate API and adds the returned copy', async () => {
    const original = makeProject('orig')
    const copy = makeProject('copy')
    storageMocks.duplicateProject.mockResolvedValue(copy)
    useProjectStore.setState({ projects: [original] })

    const result = await useProjectStore.getState().duplicateProject('orig')

    expect(storageMocks.duplicateProject).toHaveBeenCalledWith('orig', expect.any(String))
    expect(result).toBe(copy)
    expect(useProjectStore.getState().projects.map((p) => p.id)).toEqual(['orig', 'copy'])
  })

  it('throws for a project that is not in the list', async () => {
    await expect(useProjectStore.getState().duplicateProject('missing')).rejects.toThrow(
      'Project not found: missing',
    )
    expect(storageMocks.duplicateProject).not.toHaveBeenCalled()
  })
})

describe('project-store updateProject', () => {
  beforeEach(resetStore)

  it('saves only the edited fields through the retained snapshot', async () => {
    const project = makeProject('p1')
    storageMocks.ensureProject.mockResolvedValue(project)
    storageMocks.updateProject.mockImplementation(async (_id: string, updates: Partial<Project>) => ({
      ...project,
      ...updates,
    }))
    useProjectStore.setState({ projects: [project], currentProject: project })

    const updated = await useProjectStore.getState().updateProject('p1', { name: 'Renamed' })

    expect(storageMocks.ensureProject).toHaveBeenCalledWith('p1')
    expect(storageMocks.updateProject).toHaveBeenCalledWith('p1', {
      name: 'Renamed',
      description: '',
      metadata: project.metadata,
    })
    expect(updated.name).toBe('Renamed')
    expect(useProjectStore.getState().currentProject?.name).toBe('Renamed')
  })

  it('rolls back the optimistic edit when the save is rejected (e.g. a conflict)', async () => {
    const project = makeProject('p1')
    storageMocks.ensureProject.mockResolvedValue(project)
    storageMocks.updateProject.mockRejectedValue(new Error('conflict'))
    useProjectStore.setState({ projects: [project], currentProject: project })

    await expect(
      useProjectStore.getState().updateProject('p1', { name: 'Renamed' }),
    ).rejects.toThrow('conflict')

    expect(useProjectStore.getState().projects).toEqual([project])
    expect(useProjectStore.getState().currentProject).toEqual(project)
  })
})
