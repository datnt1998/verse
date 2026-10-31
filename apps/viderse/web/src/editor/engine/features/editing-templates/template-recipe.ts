import type { TemplateBackgroundAppearance, TemplateBackdrop, TemplateScenesRecipe, TimelineTrack } from '@editor/types/timeline'
import { TEMPLATE_LIMITS } from './template-limits'

/**
 * Resolves a template project's clone-video recipe from its track settings.
 * Pure: the scenes-slot `templateScenes` field and the `background-video`
 * slot carry the owner's choices; everything absent falls back to the
 * behaviour clones had before recipe settings existed.
 */

/**
 * How the video's thumbnail is composed. `clean` is a plain photograph without
 * text or graphics (what clones always drew); the others add the CTR devices
 * listed in the clone image prompt.
 */
export const THUMBNAIL_STYLES = ['clean', 'hook', 'circle-arrow', 'hook-arrow', 'split', 'closeup'] as const
export type ThumbnailStyle = (typeof THUMBNAIL_STYLES)[number]

export const DEFAULT_BACKGROUND_APPEARANCE: Readonly<TemplateBackgroundAppearance> = Object.freeze({
  opacity: 1,
  blurRadius: 0,
  brightness: 0,
  contrast: 1,
  saturation: 1,
  hueShift: 0,
  speed: 1,
  zoom: 1,
  flipX: false,
  flipY: false,
})

export interface TemplateRecipe {
  style: TemplateScenesRecipe['style']
  images: TemplateScenesRecipe['images']
  motion: { kind: 'ken-burns' | 'wander'; intensity: number; speed: number }
  thumbnail: ThumbnailStyle
  /** Owner direction added to every generated image; empty when not set. */
  imagePrompt: string
  /** Shared opacity (0…1) of the built scene items; the thumbnail ignores it. */
  sceneOpacity: number
  /** Owner-uploaded cover (template media id) for image-free clones; null when not set. */
  coverMediaId: string | null
  /** Whether the clone's rendered MP4 is re-encoded smaller before it is kept. */
  compressExport: boolean
  backgroundTrackId: string | null
  /** Backdrop channels linked on the background track; each clone job fetches its own clips from them. */
  backdrop: TemplateBackdrop | null
  backgroundAppearance: TemplateBackgroundAppearance
}

/** Today's output, unchanged: full-frame images, five per video, Ken Burns zoom, clean thumbnail. */
const DEFAULT_RECIPE: TemplateRecipe = {
  style: 'full',
  images: { mode: 'fixed', count: 5 },
  motion: { kind: 'ken-burns', intensity: 1, speed: 1 },
  thumbnail: 'clean',
  imagePrompt: '',
  sceneOpacity: 1,
  coverMediaId: null,
  compressExport: true,
  backgroundTrackId: null,
  backdrop: null,
  backgroundAppearance: DEFAULT_BACKGROUND_APPEARANCE,
}

export function resolveTemplateRecipe(project: { tracks: readonly Pick<TimelineTrack, 'id' | 'templateSlot' | 'templateScenes' | 'templateBackdrop' | 'templateBackgroundAppearance'>[] }): TemplateRecipe {
  const scenes = project.tracks.find((track) => track.templateSlot === 'scenes')?.templateScenes
  const background = project.tracks.find((track) => track.templateSlot === 'background-video')
  return {
    style: scenes?.style ?? DEFAULT_RECIPE.style,
    images: scenes?.images ?? DEFAULT_RECIPE.images,
    motion: scenes?.motion ?? DEFAULT_RECIPE.motion,
    thumbnail: scenes?.thumbnail ?? DEFAULT_RECIPE.thumbnail,
    imagePrompt: scenes?.imagePrompt ?? DEFAULT_RECIPE.imagePrompt,
    sceneOpacity: scenes?.opacity ?? DEFAULT_RECIPE.sceneOpacity,
    coverMediaId: scenes?.coverMediaId ?? DEFAULT_RECIPE.coverMediaId,
    compressExport: scenes?.compress !== false,
    backgroundTrackId: background?.id ?? null,
    backdrop: background?.templateBackdrop ?? DEFAULT_RECIPE.backdrop,
    backgroundAppearance: background?.templateBackgroundAppearance ?? DEFAULT_BACKGROUND_APPEARANCE,
  }
}

/**
 * The image the pipeline draws and cuts for a recipe style. Both cut-out styles
 * share one transparent subject PNG; `cutout-close` only frames it differently
 * on the timeline, so switching between them never redraws an image.
 */
export function subjectImageStyle(style: TemplateScenesRecipe['style']): 'full' | 'cutout' {
  return style === 'full' ? 'full' : 'cutout'
}

/** How many subject images the clone needs for `durationSec` of source video; 0 when the template generates none. */
export function recipeImageCount(recipe: TemplateRecipe, durationSec: number): number {
  if (recipe.images.mode === 'none') return 0
  if (recipe.images.mode === 'fixed') return recipe.images.count
  return Math.max(TEMPLATE_LIMITS.fixedImages.min, Math.min(TEMPLATE_LIMITS.fixedImages.max, Math.ceil(durationSec / 60 / recipe.images.minutes)))
}
