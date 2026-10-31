import React from 'react'
import type { TextItem } from '@editor/types/timeline'
import { renderAnimatedCaption } from '@editor/donor/openreel/text/caption-animation-renderer'
import { activeCaptionPage, effectiveCaptionPreset, resolveCaptionPages } from '@editor/features/editing-templates/caption-pages'
import { resolveSpanStyles, resolveTextStyle } from '@editor/shared/typography/text-style'

export function WordCaptionContent({ item, frame, fps, scale = 1 }: { item: TextItem; frame: number; fps: number; scale?: number }) {
  const style = resolveTextStyle(item)
  const font = resolveSpanStyles({ ...item, textSpans: undefined })[0]!
  // The active page is the only phrase on screen: pages come from the shared
  // normaliser (memoised on the stored words/cues), so this stays O(page) per
  // frame even with thousands of stored words.
  const pages = resolveCaptionPages(item, fps)
  const page = activeCaptionPage(pages, frame)
  const preset = effectiveCaptionPreset(item, page)
  const states = renderAnimatedCaption(page?.words ?? [], frame, preset, fps)
  if (!page || page.words.length === 0) return null
  return <div style={{ width: '100%', height: '100%', display: 'flex', alignItems: style.verticalAlign === 'bottom' ? 'flex-end' : style.verticalAlign === 'top' ? 'flex-start' : 'center', justifyContent: style.textAlign === 'left' ? 'flex-start' : style.textAlign === 'right' ? 'flex-end' : 'center', padding: style.textPadding * scale, boxSizing: 'border-box' }}>
    <div style={{ textAlign: style.textAlign, fontFamily: font.fontFamily, fontSize: font.fontSize * scale, fontWeight: font.fontWeight, fontStyle: font.fontStyle, lineHeight: style.lineHeight, letterSpacing: style.letterSpacing * scale, color: style.color, background: style.backgroundColor, borderRadius: style.backgroundRadius * scale, padding: style.backgroundColor ? style.textPadding * scale : 0, textShadow: style.textShadow ? `${style.textShadow.offsetX * scale}px ${style.textShadow.offsetY * scale}px ${style.textShadow.blur * scale}px ${style.textShadow.color}` : undefined, WebkitTextStroke: style.stroke ? `${style.stroke.width * scale * 2}px ${style.stroke.color}` : undefined, paintOrder: 'stroke fill' }}>
      {states.map((state) => (state.visible || preset !== 'word-by-word') && <React.Fragment key={state.index}>
        {/* `backgroundImage`, not the `background` shorthand: React re-sets only the changed property each frame, and the shorthand resets background-clip, turning the highlight into a solid bar. */}
        <span data-caption-word={state.index} data-active={state.active} style={{ display: 'inline-block', visibility: state.visible ? 'visible' : 'hidden', opacity: state.opacity, transform: `translateY(${state.offsetY * scale}px) scale(${state.scale})`, backgroundImage: state.highlightProgress ? `linear-gradient(90deg,#facc15 ${state.highlightProgress * 100}%,${style.color} ${state.highlightProgress * 100}%)` : undefined, backgroundClip: state.highlightProgress ? 'text' : undefined, WebkitBackgroundClip: state.highlightProgress ? 'text' : undefined, color: state.highlightProgress ? 'transparent' : style.color }}>{state.text}</span>{' '}
      </React.Fragment>)}
    </div>
  </div>
}
