import { z } from 'zod'
import { registerCommand } from '../registry'
import { itemJsonSchema, trackJsonSchema } from './item'
import { audioEqSchema } from './audio'
const id = z.string().min(1)
const markerSchema = z.object({ id, frame: z.number().int().nonnegative(), color: z.string(), label: z.string().optional() })
const transitionSchema = z.looseObject({ id, leftClipId: id, rightClipId: id, trackId: id, durationInFrames: z.number().positive() })
const keyframeSchema = z.looseObject({ itemId: id, properties: z.array(z.json()).optional() })
const compositionSchema = z.looseObject({ id, name: z.string(), items: z.array(itemJsonSchema), tracks: z.array(trackJsonSchema), transitions: z.array(transitionSchema), keyframes: z.array(keyframeSchema), fps: z.number().positive(), width: z.number().positive(), height: z.number().positive(), durationInFrames: z.number().nonnegative() })
const projectPatchSchema = z.object({
  items: z.array(itemJsonSchema).optional(), tracks: z.array(trackJsonSchema).optional(), transitions: z.array(transitionSchema).optional(), keyframes: z.array(keyframeSchema).optional(),
  markers: z.array(markerSchema).optional(), compositions: z.array(compositionSchema).optional(), topLevelSequenceIds: z.array(id).optional(),
  inPoint: z.number().nonnegative().nullable().optional(), outPoint: z.number().nonnegative().nullable().optional(), fps: z.number().positive().max(240).optional(),
  scrollPosition: z.number().nonnegative().optional(), snapEnabled: z.boolean().optional(), currentFrame: z.number().int().nonnegative().optional(),
  busAudioEq: audioEqSchema.optional(), masterBusDb: z.number().min(-60).max(12).optional(),
}).strict()
export function registerProjectCommands(): void {
  registerCommand({ type: 'project.setView', history: 'none', dirty: false, params: z.object({ scrollPosition: z.number().nonnegative() }).strict(), apply({ draft }, params) {
    draft.scrollPosition = params.scrollPosition
    return { kind: 'patch', patch: {} }
  } })
  registerCommand({ type: 'project.clearTimeline', history: 'none', dirty: false, params: z.object({}).strict(), apply({ draft }) {
    draft.items = []
    draft.tracks = []
    draft.transitions = []
    draft.keyframes = []
    draft.markers = []
    draft.inPoint = null
    draft.outPoint = null
    return { kind: 'patch', patch: {} }
  } })
  registerCommand({ type: 'project.rename', params: z.object({ name: z.string().trim().min(1) }).strict(), apply({ draft }, params) {
    draft.projectName = params.name
    return { kind: 'patch', patch: {} }
  } })
  registerCommand({
    type: 'project.updateSettings',
    params: z.object({ width: z.number().int().positive().optional(), height: z.number().int().positive().optional(), fps: z.number().positive().max(240).optional(), backgroundColor: z.string().optional() }).strict(),
    apply({ draft }, params) {
      const previous = draft.projectMetadata
      const metadata = { width: previous?.width ?? 1920, height: previous?.height ?? 1080, fps: previous?.fps ?? draft.fps, ...previous, ...params }
      draft.projectMetadata = metadata
      if (params.fps !== undefined) draft.fps = params.fps
      return { kind: 'patch', patch: {} }
    },
  })
  registerCommand({ type: 'project.setTimelineSettings', params: z.object({ snapEnabled: z.boolean().optional(), scrollPosition: z.number().nonnegative().optional() }).strict(), apply({ draft }, params) {
    if (params.snapEnabled !== undefined) draft.snapEnabled = params.snapEnabled
    if (params.scrollPosition !== undefined) draft.scrollPosition = params.scrollPosition
    return { kind: 'patch', patch: {} }
  } })
  registerCommand({ type: 'project.patch', params: z.object({ patch: projectPatchSchema }).strict(), apply({ draft }, { patch }) {
    for (const [key, value] of Object.entries(patch)) Reflect.set(draft, key, value)
    return { kind: 'patch', patch: {} }
  } })
}
