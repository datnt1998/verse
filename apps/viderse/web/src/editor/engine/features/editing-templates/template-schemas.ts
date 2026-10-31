import { z } from 'zod'
import { TEMPLATE_LIMITS } from './template-limits'
import { THUMBNAIL_STYLES } from './template-recipe'

/*
 * Template fields stored on slot tracks. Kept free of command-layer imports so server code can
 * validate template settings without loading the editor's handlers and stores.
 */

/** Scene-subject recipe stored on the `scenes` slot track (`null` in updates clears it). */
export const templateScenesSchema = z.object({
  style: z.enum(['full', 'cutout', 'cutout-close']),
  images: z.discriminatedUnion('mode', [
    z.object({ mode: z.literal('fixed'), count: z.number().int().min(TEMPLATE_LIMITS.fixedImages.min).max(TEMPLATE_LIMITS.fixedImages.max) }).strict(),
    z.object({ mode: z.literal('per-minutes'), minutes: z.number().int().min(TEMPLATE_LIMITS.perMinutes.min).max(TEMPLATE_LIMITS.perMinutes.max) }).strict(),
    z.object({ mode: z.literal('none') }).strict(),
  ]),
  motion: z.object({
    kind: z.enum(['ken-burns', 'wander']),
    intensity: z.number().min(TEMPLATE_LIMITS.motionIntensity.min).max(TEMPLATE_LIMITS.motionIntensity.max),
    speed: z.number().min(TEMPLATE_LIMITS.motionSpeed.min).max(TEMPLATE_LIMITS.motionSpeed.max),
  }).strict(),
  thumbnail: z.enum(THUMBNAIL_STYLES).optional(),
  // Stored trimmed and non-empty; clearing the prompt drops the field.
  imagePrompt: z.string().min(1).max(TEMPLATE_LIMITS.imagePrompt.max).refine((value) => value === value.trim(), 'imagePrompt must be trimmed').optional(),
  // Absent means fully opaque; the thumbnail never reads this.
  opacity: z.number().finite().min(TEMPLATE_LIMITS.sceneOpacity.min).max(TEMPLATE_LIMITS.sceneOpacity.max).optional(),
  // A media id of the template project; the cover of image-free clones.
  coverMediaId: z.string().min(1).optional(),
  // Only `false` is stored: absent means the export is compressed.
  compress: z.literal(false).optional(),
}).strict()

/** Backdrop-channel settings stored on the `background-video` slot track (`null` in updates clears it). */
export const templateBackdropSchema = z.object({
  channels: z.array(z.object({
    channelId: z.string().regex(/^(twitch:[a-z0-9_]{1,25}|bilibili:\d{1,20})$/),
    kinds: z.array(z.enum(['clip', 'highlight', 'vod', 'video'])).min(TEMPLATE_LIMITS.kindsPerChannel.min).max(TEMPLATE_LIMITS.kindsPerChannel.max),
    pinned: z.array(z.string().min(1).max(64)).max(TEMPLATE_LIMITS.pinnedPerChannel.max), // empty = every video of the chosen kinds
  }).strict()).min(TEMPLATE_LIMITS.backdropChannels.min).max(TEMPLATE_LIMITS.backdropChannels.max),
  clipCount: z.number().int().min(TEMPLATE_LIMITS.backdropClips.min).max(TEMPLATE_LIMITS.backdropClips.max).optional(), // absent = pick clips until they cover the video
  clipSec: z.number().int().min(TEMPLATE_LIMITS.backdropClipSec.min).max(TEMPLATE_LIMITS.backdropClipSec.max),
}).strict()

const appearanceLimits = TEMPLATE_LIMITS.backgroundAppearance

/** Complete common settings; no coercion, partial objects or unknown fields at ingestion. */
export const templateBackgroundAppearanceSchema = z.object({
  opacity: z.number().finite().min(appearanceLimits.opacity.min).max(appearanceLimits.opacity.max),
  blurRadius: z.number().finite().min(appearanceLimits.blurRadius.min).max(appearanceLimits.blurRadius.max),
  brightness: z.number().finite().min(appearanceLimits.brightness.min).max(appearanceLimits.brightness.max),
  contrast: z.number().finite().min(appearanceLimits.contrast.min).max(appearanceLimits.contrast.max),
  saturation: z.number().finite().min(appearanceLimits.saturation.min).max(appearanceLimits.saturation.max),
  hueShift: z.number().finite().min(appearanceLimits.hueShift.min).max(appearanceLimits.hueShift.max),
  speed: z.number().finite().min(appearanceLimits.speed.min).max(appearanceLimits.speed.max),
  zoom: z.number().finite().min(appearanceLimits.zoom.min).max(appearanceLimits.zoom.max),
  flipX: z.boolean(),
  flipY: z.boolean(),
}).strict()
