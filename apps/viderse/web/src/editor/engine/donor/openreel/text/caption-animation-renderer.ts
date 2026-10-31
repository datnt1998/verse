import type { CaptionAnimationPreset, CaptionWord } from '@editor/types/timeline'
import { easeOutBounce } from './text-animation-presets'

export interface CaptionWordState {
  text: string
  index: number
  active: boolean
  visible: boolean
  opacity: number
  scale: number
  offsetY: number
  highlightProgress: number
}

/** Frame-only evaluator shared by DOM playback and Canvas preview/export. */
export function renderAnimatedCaption(words: readonly CaptionWord[], frame: number, preset: CaptionAnimationPreset = 'none', fps = 30): CaptionWordState[] {
  return words.map((word, index) => {
    const active = frame >= word.startFrame && frame < word.endFrame
    const started = frame >= word.startFrame
    const entrance = Math.max(0, Math.min(1, (frame - word.startFrame) / Math.max(1, fps * 0.3)))
    const state: CaptionWordState = { text: word.text, index, active, visible: true, opacity: 1, scale: 1, offsetY: 0, highlightProgress: 0 }
    switch (preset) {
      case 'word-highlight':
        state.highlightProgress = active ? 1 : 0
        state.scale = active ? 1.15 : 1
        state.offsetY = active ? -2 : 0
        break
      case 'karaoke':
        // Step, not sweep: the whole word flips at its start and stays lit
        // (never mid-word) — the same shape ASS `\k` produces in the MP4.
        state.highlightProgress = started ? 1 : 0
        break
      case 'word-by-word':
        state.visible = active || (index === words.length - 1 && frame >= word.endFrame)
        break
      case 'typewriter':
        state.visible = started
        state.opacity = Math.max(0, Math.min(1, (frame - word.startFrame) / Math.max(1, fps * 0.1)))
        break
      case 'bounce': {
        const eased = easeOutBounce(entrance)
        state.visible = started
        state.opacity = eased
        state.scale = 0.5 + eased * 0.5
        state.offsetY = 20 * (1 - eased)
        break
      }
      case 'pop':
        state.visible = started
        state.opacity = entrance
        state.scale = 0.5 + 0.5 * (1 + (entrance - 1) ** 3 + 2.70158 * (entrance - 1) ** 2 * entrance)
        break
      case 'fade':
        state.visible = started
        state.opacity = entrance
        break
    }
    return state
  })
}
