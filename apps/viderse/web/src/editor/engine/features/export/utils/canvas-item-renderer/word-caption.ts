import type { SubtitleSegmentItem, TextItem } from '@editor/types/timeline'
import { renderAnimatedCaption, type CaptionWordState } from '@editor/donor/openreel/text/caption-animation-renderer'
import { activeCaptionPage, CAPTION_HIGHLIGHT_COLOR, effectiveCaptionPreset, resolveCaptionPages } from '@editor/features/editing-templates/caption-pages'
import { parseSubtitleCueText } from '@editor/shared/utils/subtitle-cue-format'
import { layoutTextBlock } from '@editor/shared/typography/text-block-layout'
import { applyCanvasLetterSpacing, createCanvasTextMeasurer } from '@editor/shared/typography/text-measurer'
import type { ItemRenderContext } from './types'

/**
 * Word-level caption painting shared by canvas preview and export. The visible
 * phrase is the normaliser's active page — the same pages the DOM preview and
 * the MP4 overlay draw — and every painted state comes from the shared
 * frame evaluator, so canvas output can't drift from either surface.
 */
export function renderWordCaptionItem(
  ctx: OffscreenCanvasRenderingContext2D,
  item: TextItem | SubtitleSegmentItem,
  transform: { x: number; y: number; width: number; height: number },
  frame: number,
  rctx: ItemRenderContext,
): void {
  const fps = rctx.canvasSettings.fps
  const relativeFrame = frame - item.from
  const pages = resolveCaptionPages(item, fps)
  const page = activeCaptionPage(pages, relativeFrame)
  if (!page) return
  const preset = effectiveCaptionPreset(item, page)
  // Text-only pages (cues without words) draw their plain text statically.
  const states: CaptionWordState[] = page.words.length
    ? renderAnimatedCaption(page.words, relativeFrame, preset, fps)
    : parseSubtitleCueText(page.text).plainText
        .split(/\s+/)
        .filter(Boolean)
        .map((text, index) => ({ text, index, active: false, visible: true, opacity: 1, scale: 1, offsetY: 0, highlightProgress: 0 }))
  if (!states.length) return
  const shown = preset === 'word-by-word' ? states.filter((state) => state.visible) : states

  const layoutItem: TextItem = { ...item, type: 'text', text: shown.map((word) => word.text).join(' '), textSpans: undefined }
  const measurer = createCanvasTextMeasurer(ctx, (text, spacing) => rctx.textMeasureCache.measure(ctx, text, spacing))
  const layout = layoutTextBlock(layoutItem, transform.width, transform.height, measurer)
  const left = rctx.canvasSettings.width / 2 + transform.x - transform.width / 2
  const top = rctx.canvasSettings.height / 2 + transform.y - transform.height / 2
  ctx.save()
  const bg = layout.background
  if (bg && item.backgroundColor) {
    ctx.fillStyle = item.backgroundColor
    ctx.beginPath()
    ctx.roundRect(left + bg.x, top + bg.y, bg.width, bg.height, bg.radius)
    ctx.fill()
  }
  let wordIndex = 0
  for (const line of layout.lines) {
    ctx.font = line.cssFont
    applyCanvasLetterSpacing(ctx, line.letterSpacing)
    ctx.textBaseline = 'alphabetic'
    ctx.textAlign = 'left'
    let prefix = ''
    for (const token of line.text.split(' ')) {
      if (!token) { prefix += ' '; continue }
      // Page words are single tokens, so laid-out tokens map 1:1 onto states.
      const state = shown[wordIndex++]
      const offset = measurer.measure(prefix, line.cssFont, line.letterSpacing)
      const width = measurer.measure(token, line.cssFont, line.letterSpacing)
      prefix += `${token} `
      if (!state || !state.visible || state.opacity <= 0) continue
      ctx.save()
      ctx.globalAlpha *= state.opacity
      ctx.translate(left + line.startX + offset + width / 2, top + line.baselineY + state.offsetY)
      ctx.scale(state.scale, state.scale)
      if (item.textShadow) {
        ctx.shadowColor = item.textShadow.color
        ctx.shadowBlur = item.textShadow.blur
        ctx.shadowOffsetX = item.textShadow.offsetX
        ctx.shadowOffsetY = item.textShadow.offsetY
      }
      if (item.stroke) {
        ctx.strokeStyle = item.stroke.color
        ctx.lineWidth = item.stroke.width * 2
        ctx.strokeText(token, -width / 2, 0)
      }
      // Whole-word highlight: spoken words repaint fully, never a partial clip.
      ctx.fillStyle = state.highlightProgress > 0 ? CAPTION_HIGHLIGHT_COLOR : item.color
      ctx.fillText(token, -width / 2, 0)
      ctx.restore()
    }
  }
  ctx.restore()
}
