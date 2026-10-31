/**
 * User-saved effect presets (grades).
 *
 * Stored on the server at `GET|PUT /api/editor/workspace/effect-presets` —
 * presets are workspace-level (shared across projects) and guarded by ETag /
 * If-Match like every other revisioned document.
 */

import type { VisualEffect } from '@editor/types/effects'
import { createLogger } from '@editor/shared/logging/logger'
import { serializeWrites } from '@editor/storage/project-persistence'
import { getEditorStorage } from '@editor/storage/storage-backend'

const logger = createLogger('WorkspaceFS:EffectPresets')

export interface UserEffectPreset {
  id: string
  name: string
  effects: VisualEffect[]
  createdAt: number
}

interface EffectPresetsFile {
  version: 1
  presets: UserEffectPreset[]
}

function isVisualEffect(value: unknown): value is VisualEffect {
  if (!value || typeof value !== 'object') return false
  const candidate = value as Partial<VisualEffect>
  return (
    candidate.type === 'gpu-effect' &&
    typeof candidate.gpuEffectType === 'string' &&
    typeof candidate.params === 'object' &&
    candidate.params !== null
  )
}

function sanitizePresets(value: unknown): UserEffectPreset[] {
  if (!value || typeof value !== 'object') return []
  const presets = (value as Partial<EffectPresetsFile>).presets
  if (!Array.isArray(presets)) return []

  return presets.flatMap((preset) => {
    if (!preset || typeof preset !== 'object') return []
    const candidate = preset as Partial<UserEffectPreset>
    if (
      typeof candidate.id !== 'string' ||
      typeof candidate.name !== 'string' ||
      !Array.isArray(candidate.effects)
    ) {
      return []
    }
    const effects = candidate.effects.filter(isVisualEffect)
    if (effects.length === 0) return []
    return [
      {
        id: candidate.id,
        name: candidate.name,
        effects,
        createdAt: typeof candidate.createdAt === 'number' ? candidate.createdAt : 0,
      },
    ]
  })
}

/** Revision retained by the last read/save; saves are always based on it. */
let revision: string | undefined
const EFFECT_PRESETS_KEY = 'workspace:effect-presets'

export function readUserEffectPresets(): Promise<UserEffectPreset[]> {
  return serializeWrites(EFFECT_PRESETS_KEY, async () => {
    try {
      const loaded = await getEditorStorage().loadWorkspaceEffectPresets<EffectPresetsFile>()
      revision = loaded.revision
      return sanitizePresets(loaded.data)
    } catch (error) {
      logger.warn('readUserEffectPresets failed', error)
      return []
    }
  })
}

/** Saves against the revision from the last {@link readUserEffectPresets}; a 409 rejects. */
export function saveUserEffectPresets(presets: UserEffectPreset[]): Promise<void> {
  return serializeWrites(EFFECT_PRESETS_KEY, async () => {
    if (revision === undefined) throw new Error('Effect presets chưa được tải')
    const file: EffectPresetsFile = { version: 1, presets }
    try {
      revision = await getEditorStorage().saveWorkspaceEffectPresets(file, revision)
    } catch (error) {
      logger.error('saveUserEffectPresets failed', error)
      throw error
    }
  })
}
