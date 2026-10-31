import { z } from 'zod'
import { registerCommand } from '../registry'
import type { CommandIssue, ProjectStateView } from '../types'
import type { CaptionWord, TimelineItem } from '@editor/types/timeline'

const wordSchema = z.object({ text: z.string().min(1), startFrame: z.number().int().nonnegative(), endFrame: z.number().int().positive(), estimated: z.boolean().optional() }).refine((word) => word.endFrame > word.startFrame, { message: 'Mốc kết thúc phải sau mốc bắt đầu.' })
const presetSchema = z.enum(['none', 'word-highlight', 'word-by-word', 'karaoke', 'typewriter', 'bounce', 'pop', 'fade'])
const parsedSchema = z.object({ words: z.array(wordSchema), estimated: z.boolean(), cues: z.array(z.object({ id: z.string(), text: z.string(), startSeconds: z.number().nonnegative(), endSeconds: z.number().positive() })), durationInFrames: z.number().int().nonnegative() })

function validateTarget(state: ProjectStateView, itemId: string, words?: readonly CaptionWord[]): CommandIssue[] {
  const item = state.items.find((entry) => entry.id === itemId)
  if (!item || (item.type !== 'text' && item.type !== 'subtitle')) return [{ code: 'caption-target', message: 'Chọn một mục chữ hoặc phụ đề.' }]
  if (words?.some((word, index) => word.endFrame > item.durationInFrames || word.endFrame <= word.startFrame || (index > 0 && word.startFrame < words[index - 1]!.startFrame))) return [{ code: 'caption-timing', message: 'Mốc từ phải tăng dần và nằm trong thời lượng mục.' }]
  return []
}

export function registerCaptionCommands(): void {
  registerCommand({
    type: 'caption.setWords',
    params: z.object({ itemId: z.string(), words: z.array(wordSchema), estimated: z.boolean().optional() }),
    validate: (state, params) => validateTarget(state, params.itemId, params.words),
    apply: (ctx, params) => {
      ctx.draft.items = ctx.draft.items.map((item): TimelineItem => item.id === params.itemId && (item.type === 'text' || item.type === 'subtitle') ? { ...item, words: params.words, wordsEstimated: params.estimated ?? false } : item)
      return { kind: 'patch', patch: {} }
    },
    mergeKey: (params) => `caption-words:${params.itemId}`,
  })
  registerCommand({
    type: 'caption.updateWord',
    params: z.object({ itemId: z.string(), index: z.number().int().nonnegative(), word: z.object({ text: z.string().min(1).optional(), startFrame: z.number().int().nonnegative().optional(), endFrame: z.number().int().positive().optional() }) }),
    validate: (state, params) => {
      const issues = validateTarget(state, params.itemId)
      const item = state.items.find((entry) => entry.id === params.itemId)
      if (!item || (item.type !== 'text' && item.type !== 'subtitle') || !item.words?.[params.index]) return [...issues, { message: 'Không tìm thấy từ.' }]
      return validateTarget(state, params.itemId, item.words.map((word, index) => index === params.index ? { ...word, ...params.word } : word))
    },
    apply: (ctx, params) => {
      ctx.draft.items = ctx.draft.items.map((item): TimelineItem => item.id === params.itemId && (item.type === 'text' || item.type === 'subtitle') ? { ...item, words: item.words?.map((word, index) => index === params.index ? { ...word, ...params.word } : word) } : item)
      return { kind: 'patch', patch: {} }
    },
    mergeKey: (params) => `caption-word:${params.itemId}:${params.index}`,
  })
  registerCommand({
    type: 'caption.setPreset',
    params: z.object({ itemId: z.string(), preset: presetSchema }),
    validate: (state, params) => validateTarget(state, params.itemId),
    apply: (ctx, params) => {
      ctx.draft.items = ctx.draft.items.map((item): TimelineItem => item.id === params.itemId && (item.type === 'text' || item.type === 'subtitle') ? { ...item, captionPreset: params.preset } : item)
      return { kind: 'patch', patch: {} }
    },
  })
  registerCommand({
    type: 'caption.importTimed',
    params: z.object({ itemId: z.string(), data: parsedSchema }),
    validate: (state, params) => validateTarget(state, params.itemId, params.data.words),
    apply: (ctx, params) => {
      ctx.draft.items = ctx.draft.items.map((item): TimelineItem => {
        if (item.id !== params.itemId || (item.type !== 'text' && item.type !== 'subtitle')) return item
        const next = { ...item, words: params.data.words, wordsEstimated: params.data.estimated }
        if (next.type === 'subtitle') return { ...next, cues: params.data.cues }
        if (!params.data.cues.length) return { ...next, text: params.data.words.map((word) => word.text).join(' ') }
        const {
          text: _text, textSpans: _textSpans, spanLayout: _spanLayout,
          textLayoutDrafts: _textLayoutDrafts, textStylePresetId: _textStylePresetId,
          textStyleScale: _textStyleScale, textMotion: _textMotion,
          textRole: _textRole, captionSource, ...segment
        } = next
        return {
          ...segment,
          type: 'subtitle',
          cues: params.data.cues,
          source: {
            type: 'subtitle-import',
            fileName: captionSource?.fileName ?? item.label,
            format: captionSource?.format ?? (item.label.toLowerCase().endsWith('.srt') ? 'srt' : 'vtt'),
            importedAt: captionSource?.importedAt ?? 0,
          },
        }
      })
      return { kind: 'patch', patch: {} }
    },
  })
}
