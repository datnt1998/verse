import { z } from 'zod'
import { listCommands } from './registry'
import { GPU_EFFECT_REGISTRY } from '../infrastructure/gpu-effects/registry'
import { GPU_TRANSITION_REGISTRY } from '../infrastructure/gpu-transitions/registry'
import { BLEND_MODE_INDEX } from '../types/blend-modes'
import { easingFunctions } from '../shared/utils/easing'
export interface EditorCapabilities { commands: { type: string; params: Record<string, unknown> }[]; effects: string[]; transitions: string[]; blendModes: string[]; easings: string[] }
export function getCapabilityManifest(): EditorCapabilities {
  return {
    commands: listCommands().map(command => ({ type: command.type, params: z.toJSONSchema(command.params, { unrepresentable: 'any' }) })),
    effects: [...GPU_EFFECT_REGISTRY.keys()],
    transitions: [...GPU_TRANSITION_REGISTRY.keys()],
    blendModes: Object.keys(BLEND_MODE_INDEX),
    easings: Object.keys(easingFunctions),
  }
}
