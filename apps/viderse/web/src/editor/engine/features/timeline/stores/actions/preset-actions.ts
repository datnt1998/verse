import type { AnimationPreset } from '@editor/infrastructure/storage'
import { executeCommands } from '@editor/commands'
export type { ApplyAnimationPresetResult } from '@editor/commands/handlers/preset-mutations'
import type { ApplyAnimationPresetResult } from '@editor/commands/handlers/preset-mutations'

export function applyAnimationPreset(targetItemId: string, preset: AnimationPreset, anchorFrame = 0, options: { replace?: boolean; retime?: boolean } = {}): ApplyAnimationPresetResult {
  const result = executeCommands([{ type: 'keyframe.applyPreset', params: JSON.parse(JSON.stringify({ targetItemId, preset, anchorFrame, options })) }])
  if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '))
  return result.results[0] as ApplyAnimationPresetResult
}
