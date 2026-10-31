import type { CaptionAnimationPreset } from '@editor/types/timeline'

export const CAPTION_ANIMATION_PRESETS: readonly { id: CaptionAnimationPreset; name: string }[] = [
  { id: 'none', name: 'Tĩnh' },
  { id: 'word-highlight', name: 'Nhấn từ đang nói' },
  { id: 'word-by-word', name: 'Từng từ' },
  { id: 'karaoke', name: 'Karaoke' },
  { id: 'typewriter', name: 'Máy chữ' },
  { id: 'bounce', name: 'Nảy' },
  { id: 'pop', name: 'Bật lên' },
  { id: 'fade', name: 'Hiện dần từng từ' },
]

/** OpenReel's piecewise easeOutBounce, with a normalized input. */
export function easeOutBounce(progress: number): number {
  let t = Math.max(0, Math.min(1, progress))
  const n = 7.5625
  const d = 2.75
  if (t < 1 / d) return n * t * t
  if (t < 2 / d) return n * (t -= 1.5 / d) * t + 0.75
  if (t < 2.5 / d) return n * (t -= 2.25 / d) * t + 0.9375
  return n * (t -= 2.625 / d) * t + 0.984375
}
