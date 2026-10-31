import type { ItemEffect } from '@editor/types/effects'
import type { TemplateBackgroundAppearance, VideoItem } from '@editor/types/timeline'
import { sourceToTimelineFrames } from '@editor/features/timeline/utils/source-calculations'

/** Retime before reserving transition handles, keeping the authored source window. */
export function retimeBackgroundClip(clip: VideoItem, globalSpeed: number, fps: number): VideoItem {
  // In particular, do not newly reject or rewrite legacy local speeds at identity.
  if (globalSpeed === 1) return clip
  const localSpeed = clip.speed ?? 1
  const speed = localSpeed * globalSpeed
  if (!Number.isFinite(speed) || speed < 0.1 || speed > 10) {
    throw new Error(`Tốc độ nền hiệu dụng của clip ${clip.id} phải trong khoảng 0.1–10.`)
  }
  const sourceFps = clip.sourceFps ?? fps
  const sourceStart = clip.sourceStart ?? clip.trimStart ?? clip.offset ?? 0
  const authoredEnd = clip.sourceEnd ?? clip.sourceDuration
    ?? sourceStart + Math.max(1, Math.ceil(clip.durationInFrames * localSpeed * sourceFps / fps))
  const sourceEnd = Math.min(authoredEnd, clip.sourceDuration ?? authoredEnd)
  if (!Number.isFinite(sourceFps) || sourceFps <= 0 || !Number.isFinite(sourceStart) || sourceStart < 0 || !Number.isFinite(sourceEnd) || sourceEnd <= sourceStart) {
    throw new Error(`Khoảng nguồn của clip nền ${clip.id} không hợp lệ.`)
  }
  // Loops retain their repeating window, even when the last placement is shorter.
  // Non-loop clips cannot consume more than either their authored duration or source window.
  const duration = clip.loop
    ? Math.floor(clip.durationInFrames / globalSpeed)
    : Math.min(Math.floor(clip.durationInFrames / globalSpeed), sourceToTimelineFrames(sourceEnd - sourceStart, speed, sourceFps, fps))
  return {
    ...clip,
    speed,
    sourceFps,
    sourceStart,
    sourceEnd,
    sourceDuration: clip.sourceDuration ?? sourceEnd,
    // A sub-frame window still supplies its first frame; never emit an empty tile.
    durationInFrames: Math.max(1, duration),
  }
}

/** Compose onto an output, never onto a reusable input/prototype or an audio item. */
export function applyBackgroundAppearance(
  clip: VideoItem,
  appearance: TemplateBackgroundAppearance,
  canvas: { width: number; height: number },
): VideoItem {
  let result = clip
  if (appearance.opacity !== 1 || appearance.zoom !== 1 || appearance.flipX || appearance.flipY) {
    const transform = { ...clip.transform }
    if (appearance.opacity !== 1) transform.opacity = (transform.opacity ?? 1) * appearance.opacity
    if (appearance.zoom !== 1) {
      const sourceWidth = clip.sourceWidth ?? canvas.width
      const sourceHeight = clip.sourceHeight ?? canvas.height
      const fit = Math.min(canvas.width / sourceWidth, canvas.height / sourceHeight)
      transform.width = (transform.width ?? sourceWidth * fit) * appearance.zoom
      transform.height = (transform.height ?? sourceHeight * fit) * appearance.zoom
      if (transform.anchorX !== undefined) transform.anchorX *= appearance.zoom
      if (transform.anchorY !== undefined) transform.anchorY *= appearance.zoom
    }
    if (appearance.flipX) transform.flipHorizontal = !transform.flipHorizontal
    if (appearance.flipY) transform.flipVertical = !transform.flipVertical
    result = { ...result, transform }
  }
  if (appearance.blurRadius === 0 && appearance.brightness === 0 && appearance.contrast === 1
    && appearance.saturation === 1 && appearance.hueShift === 0) return result
  const additions: ItemEffect[] = []
  const append = (field: string, gpuEffectType: string, parameter: string, value: number, identity: number) => {
    if (value === identity) return
    const baseId = `${clip.id}-background-${field}`
    let id = baseId
    let suffix = 1
    while (clip.effects?.some(effect => effect.id === id)) id = `${baseId}-${suffix++}`
    additions.push({ id, enabled: true, effect: { type: 'gpu-effect', gpuEffectType, params: { [parameter]: value } } })
  }
  append('blur', 'gpu-gaussian-blur', 'radius', appearance.blurRadius, 0)
  append('brightness', 'gpu-brightness', 'amount', appearance.brightness, 0)
  append('contrast', 'gpu-contrast', 'amount', appearance.contrast, 1)
  append('saturation', 'gpu-saturation', 'amount', appearance.saturation, 1)
  append('hue', 'gpu-hue-shift', 'shift', appearance.hueShift, 0)
  if (additions.length > 0) result = { ...result, effects: [...(clip.effects ?? []), ...additions] }
  return result
}
