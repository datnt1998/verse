import { z } from 'zod'
import { getCommand, registerCommand } from '../registry'
import { getBuiltInEditingTemplate } from '@editor/features/editing-templates/registry'
import { resolveEditingTemplate } from '@editor/features/editing-templates/resolver'
import { buildSubscribeCommands, subscribePresetContextSchema } from '@editor/features/editing-templates/subscribe'
import type { ApplyContext } from '../types'
import type { SerializedCommand } from '../types'

const mediaSchema = z.object({ mediaId: z.string(), src: z.string() })
const contextSchema = z.object({
  idPrefix: z.string().min(1), fps: z.number().positive(), width: z.number().positive(), height: z.number().positive(), durationInFrames: z.number().int().positive(), from: z.number().int().nonnegative().optional(), title: z.string().optional(),
  narration: mediaSchema.optional(),
  sections: z.array(z.object({ title: z.string(), from: z.number().int().nonnegative(), durationInFrames: z.number().int().positive(), image: mediaSchema.optional(), lowerThird: z.string().optional() })),
  words: z.array(z.object({ text: z.string(), startFrame: z.number().int().nonnegative(), endFrame: z.number().int().positive(), estimated: z.boolean().optional() })).optional(),
  wordsEstimated: z.boolean().optional(),
})

/** Expand a macro command into canonical commands, applied in-order so the whole preset stays one undo step. */
function runExpandedCommands(ctx: ApplyContext, commands: SerializedCommand[]): void {
  for (const command of commands) {
    const definition = getCommand(command.type)
    if (!definition) throw new Error(`Lệnh chưa đăng ký: ${command.type}`)
    const parsed = definition.params.parse(command.params)
    const issues = definition.validate?.(ctx.draft, parsed) ?? []
    if (issues.length) throw new Error(issues.map((issue) => issue.message).join('; '))
    definition.apply(ctx, parsed)
  }
}

export function registerTemplateCommands(): void {
  registerCommand({
    type: 'template.apply',
    params: z.object({ templateId: z.string(), context: contextSchema }),
    validate: (_, params) => getBuiltInEditingTemplate(params.templateId) ? [] : [{ code: 'template-missing', message: 'Không tìm thấy mẫu.' }],
    apply: (ctx, params) => {
      const template = getBuiltInEditingTemplate(params.templateId)
      if (!template) throw new Error('Không tìm thấy mẫu.')
      const commands = resolveEditingTemplate(template, params.context)
      runExpandedCommands(ctx, commands)
      return { inverse: { kind: 'patch', patch: {} }, result: { templateId: template.id, commands } }
    },
  })
  registerCommand({
    type: 'template.subscribe',
    params: subscribePresetContextSchema,
    validate: (state, params) => params.trackIds.map((trackId) => state.tracks.find((track) => track.id === trackId)).flatMap((track, index) => track && track.kind !== 'audio' && !track.isGroup ? [] : [{ code: 'subscribe-track', message: `Rãnh Subscribe ${index + 1} không tồn tại hoặc là rãnh âm thanh.`, path: `trackIds.${index}` }]),
    apply: (ctx, params) => {
      const { commands, itemIds, groupId, timing } = buildSubscribeCommands(params)
      runExpandedCommands(ctx, commands)
      return { inverse: { kind: 'patch', patch: {} }, result: { itemIds, groupId, timing, commands } }
    },
  })
}
