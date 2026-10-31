import React, { useMemo } from 'react'

import { useSequenceContext } from '@editor/runtime/composition-runtime/deps/player'
import { activeCaptionPage, effectiveCaptionPreset, resolveCaptionPages } from '@editor/features/editing-templates/caption-pages'
import { parseSubtitleCueText } from '@editor/shared/utils/subtitle-cue-format'
import type { SubtitleSegmentItem, TextItem } from '@editor/types/timeline'

import { useVideoConfig } from '../hooks/use-player-compat'
import { TextContent } from './text-content'

/**
 * Renders the active caption page of a {@link SubtitleSegmentItem} per frame.
 *
 * The shared normaliser turns the item's stored cues+words into non-overlapping
 * pages (≤7 words, ≤42 chars, single-word pieces) once per stored-data version —
 * the same pages canvas export and the MP4 overlay draw. Only the active page's
 * own word slice reaches {@link TextContent}: no per-frame filtering over the
 * full word list and no stale phrase from an overlapping cue.
 */
export const SubtitleSegmentContent: React.FC<{
  item: SubtitleSegmentItem & { _sequenceFrameOffset?: number }
}> = ({ item }) => {
  const sequenceContext = useSequenceContext()
  const { fps } = useVideoConfig()
  const relativeFrame = (sequenceContext?.localFrame ?? 0) - (item._sequenceFrameOffset ?? 0)

  const pages = resolveCaptionPages(item, fps)
  const page = activeCaptionPage(pages, relativeFrame)

  // Parse inline markup (<i>, <b>, <u>, <font color>) into formatted spans
  // and pull off any ASS `{\anN}` positioning override so the page can land
  // top-of-screen (used for sign translations / on-screen labels). Word pages
  // draw their own tokens — the parse still supplies alignment.
  const parsed = useMemo(
    () => (page ? parseSubtitleCueText(page.text) : null),
    [page],
  )

  // Synthesize an ephemeral TextItem that carries the active page's text and
  // the segment's typography. Keyframe/gizmo lookups by id will miss (the
  // segment isn't a TextItem) — that's fine for now; segment-level keyframes
  // are a planned follow-up.
  const syntheticTextItem = useMemo<TextItem & { _sequenceFrameOffset?: number }>(
    () => ({
      id: item.id,
      type: 'text',
      trackId: item.trackId,
      from: item.from,
      durationInFrames: item.durationInFrames,
      label: item.label,
      mediaId: item.mediaId,
      transform: item.transform,
      text: parsed?.plainText ?? '',
      // textSpans drives styled per-run rendering — italic / bold / colored
      // fragments inside one cue. TextContent prefers spans over `text`
      // when both are present.
      textSpans: parsed?.spans,
      fontSize: item.fontSize,
      fontFamily: item.fontFamily,
      fontWeight: item.fontWeight,
      fontStyle: item.fontStyle,
      underline: item.underline,
      color: item.color,
      backgroundColor: item.backgroundColor,
      backgroundRadius: item.backgroundRadius,
      textAlign: parsed?.alignment?.textAlign ?? item.textAlign,
      verticalAlign: parsed?.alignment?.verticalAlign ?? item.verticalAlign,
      lineHeight: item.lineHeight,
      letterSpacing: item.letterSpacing,
      textPadding: item.textPadding,
      textShadow: item.textShadow,
      stroke: item.stroke,
      _sequenceFrameOffset: item._sequenceFrameOffset,
    }),
    [parsed, item],
  )

  if (!page || !parsed || parsed.isEmpty) return null
  if (page.words.length) {
    // The derived preset rides along on the synthetic item so the word
    // renderer applies the same karaoke-vs-fade rule every surface shares.
    const captionPreset = effectiveCaptionPreset(item, page)
    return <TextContent item={{ ...syntheticTextItem, words: page.words, wordsEstimated: item.wordsEstimated, captionPreset }} />
  }
  return <TextContent item={syntheticTextItem} />
}
