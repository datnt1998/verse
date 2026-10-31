// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { StorageConflictError } from '@editor/storage/errors'
import { readUserEffectPresets, saveUserEffectPresets, type UserEffectPreset } from './effect-presets'

const storage = vi.hoisted(() => ({
  loadWorkspaceEffectPresets: vi.fn(),
  saveWorkspaceEffectPresets: vi.fn(),
}))

vi.mock('@editor/storage/storage-backend', () => ({ getEditorStorage: () => storage }))

const preset: UserEffectPreset = {
  id: 'grade-1',
  name: 'Warm',
  effects: [{ type: 'gpu-effect', gpuEffectType: 'gpu-blur', params: { amount: 1 } }],
  createdAt: 5,
}

describe('workspace effect presets storage', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('reads presets and saves them against the revision that was read', async () => {
    storage.loadWorkspaceEffectPresets.mockResolvedValue({
      data: { version: 1, presets: [preset] },
      revision: 'rev-1',
    })
    storage.saveWorkspaceEffectPresets.mockResolvedValue('rev-2')

    await expect(readUserEffectPresets()).resolves.toEqual([preset])
    await saveUserEffectPresets([preset])
    await saveUserEffectPresets([preset])

    expect(storage.saveWorkspaceEffectPresets).toHaveBeenNthCalledWith(
      1,
      { version: 1, presets: [preset] },
      'rev-1',
    )
    expect(storage.saveWorkspaceEffectPresets).toHaveBeenNthCalledWith(
      2,
      { version: 1, presets: [preset] },
      'rev-2',
    )
  })

  it('treats a never-written document as empty and bases the first save on `empty`', async () => {
    storage.loadWorkspaceEffectPresets.mockResolvedValue({ data: {}, revision: 'empty' })
    storage.saveWorkspaceEffectPresets.mockResolvedValue('rev-1')

    await expect(readUserEffectPresets()).resolves.toEqual([])
    await saveUserEffectPresets([preset])

    expect(storage.saveWorkspaceEffectPresets).toHaveBeenCalledWith(
      { version: 1, presets: [preset] },
      'empty',
    )
  })

  it('keeps the stale revision after a 409 so the next save conflicts again until re-read', async () => {
    storage.loadWorkspaceEffectPresets.mockResolvedValue({ data: {}, revision: 'rev-1' })
    await readUserEffectPresets()
    const conflict = new StorageConflictError('stale', 'rev-9')
    storage.saveWorkspaceEffectPresets.mockRejectedValue(conflict)

    await expect(saveUserEffectPresets([preset])).rejects.toBe(conflict)
    await expect(saveUserEffectPresets([preset])).rejects.toBe(conflict)

    expect(storage.saveWorkspaceEffectPresets).toHaveBeenLastCalledWith(expect.anything(), 'rev-1')
  })

  it('returns an empty list when reading fails', async () => {
    storage.loadWorkspaceEffectPresets.mockRejectedValue(new Error('offline'))
    await expect(readUserEffectPresets()).resolves.toEqual([])
  })
})
