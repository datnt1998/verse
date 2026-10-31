import type { SerializedCommand } from '@editor/commands/types'
import type { TimelineItem } from '@editor/types/timeline'
import { buildCaptionPages } from './caption-pages'
import type { EditingTemplate, EditingTemplateContext } from './types'

/** Resolve a recipe to the same serializable commands accepted by browser and CLI. */
export function resolveEditingTemplate(template: EditingTemplate, context: EditingTemplateContext): SerializedCommand[] {
  if (!context.idPrefix || !Number.isInteger(context.durationInFrames) || context.durationInFrames <= 0 || context.fps <= 0 || context.width <= 0 || context.height <= 0) throw new Error('Thông số mẫu không hợp lệ.')
  if (context.sections.some((section) => section.from < 0 || section.durationInFrames <= 0 || section.from + section.durationInFrames > context.durationInFrames)) throw new Error('Phân đoạn phải nằm trong thời lượng mẫu.')
  return template.resolve(context)
}

/**
 * Ken Burns keyframes for a full-canvas still: linear drift between two scale
 * endpoints over the item's local frames. Shared by the faceless templates and
 * the clone recipe so the zoom factor stays one convention.
 */
export function stillImageZoomKeyframes(itemId: string, size: { width: number; height: number }, durationInFrames: number, fromScale: number, toScale: number): SerializedCommand[] {
  const commands: SerializedCommand[] = []
  for (const property of ['width', 'height'] as const) {
    const base = property === 'width' ? size.width : size.height
    commands.push(
      { type: 'keyframe.add', params: { itemId, property, frame: 0, value: base * fromScale, easing: 'linear' } },
      { type: 'keyframe.add', params: { itemId, property, frame: durationInFrames - 1, value: base * toScale, easing: 'linear' } },
    )
  }
  return commands
}

export function buildFacelessCommands(context: EditingTemplateContext, variant: 'narration' | 'listicle' | 'documentary'): SerializedCommand[] {
  const commands: SerializedCommand[] = []
  const prefix = context.idPrefix
  const from = context.from ?? 0
  const track = (suffix: string, name: string, order: number, kind: 'audio' | 'video' = 'video') => {
    const value = { id: `${prefix}-${suffix}`, name, order, height: 64, locked: false, visible: true, muted: false, solo: false, kind }
    commands.push({ type: 'track.add', params: value })
    return value.id
  }
  const visuals = track('visuals', 'Hình ảnh', 0)
  const titles = track('titles', 'Tiêu đề', 1)
  const captions = track('captions', 'Phụ đề', 2)
  const addItem = (item: TimelineItem) => commands.push({ type: 'item.add', params: { item } })
  for (const [index, section] of context.sections.entries()) {
    if (section.image) {
      const id = `${prefix}-still-${index}`
      addItem({ id, type: 'image', trackId: visuals, from: from + section.from, durationInFrames: section.durationInFrames, label: section.title, mediaId: section.image.mediaId, src: section.image.src, transform: { x: 0, y: 0, width: context.width, height: context.height } })
      commands.push(...stillImageZoomKeyframes(id, { width: context.width, height: context.height }, section.durationInFrames, 1, 1.12))
    }
    if (variant !== 'narration') {
      const lowerThird = variant === 'documentary'
      addItem({ id: `${prefix}-title-${index}`, type: 'text', trackId: titles, from: from + section.from, durationInFrames: Math.min(section.durationInFrames, Math.round(context.fps * (lowerThird ? 6 : 3))), label: section.title, text: lowerThird ? (section.lowerThird ?? section.title) : `${index + 1}. ${section.title}`, fontSize: context.height * (lowerThird ? 0.05 : 0.08), color: '#ffffff', backgroundColor: '#111827', textAlign: lowerThird ? 'left' : 'center', verticalAlign: 'middle', transform: { x: lowerThird ? -context.width * 0.2 : 0, y: lowerThird ? context.height * 0.3 : 0, width: context.width * (lowerThird ? 0.5 : 0.8), height: context.height * 0.18 } })
    }
  }
  if (context.narration) {
    const audio = track('narration', 'Lời dẫn', 3, 'audio')
    addItem({ id: `${prefix}-voice`, type: 'audio', trackId: audio, from, durationInFrames: context.durationInFrames, label: 'Lời dẫn', src: context.narration.src, mediaId: context.narration.mediaId })
  }
  if (context.words?.length) {
    // Pages come from the shared normaliser — same boundaries, word split and
    // estimated→fade rule as the clone recipe and the MP4 overlay.
    const { pages, degraded } = buildCaptionPages({ words: context.words, wordsEstimated: context.wordsEstimated, durationInFrames: context.durationInFrames }, context.fps)
    const estimated = context.wordsEstimated === true || degraded
    const cues = pages.map((page, index) => ({ id: `${prefix}-cue-${index}`, text: page.text, startSeconds: page.startFrame / context.fps, endSeconds: page.endFrame / context.fps }))
    const words = pages.flatMap((page) => page.words.map((word) => ({ text: word.text, startFrame: word.startFrame, endFrame: word.endFrame, ...(word.estimated ? { estimated: true } : {}) })))
    addItem({ id: `${prefix}-caption`, type: 'subtitle', trackId: captions, from, durationInFrames: context.durationInFrames, label: 'Phụ đề theo từ', source: { type: 'subtitle-import', fileName: 'captions.vtt', format: 'vtt', importedAt: 0 }, cues, words, wordsEstimated: estimated, captionPreset: estimated ? 'fade' : 'karaoke', color: '#ffffff', fontSize: context.height * 0.055, textAlign: 'center', verticalAlign: 'middle', backgroundColor: '#111827', transform: { x: 0, y: context.height * 0.35, width: context.width * 0.85, height: context.height * 0.2 } })
  }
  return commands
}
