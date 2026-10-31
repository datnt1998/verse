import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { TemplateBackdrop, TimelineTrack } from '@editor/types/timeline'
import { DEFAULT_BACKGROUND_APPEARANCE, recipeImageCount, resolveTemplateRecipe, type TemplateRecipe } from './template-recipe'

type TrackSlice = Pick<TimelineTrack, 'id' | 'templateSlot' | 'templateScenes' | 'templateBackdrop'>

const perMinutes = (minutes: number): TemplateRecipe => ({
  style: 'full',
  images: { mode: 'per-minutes', minutes },
  motion: { kind: 'ken-burns', intensity: 1, speed: 1 },
  thumbnail: 'clean',
  imagePrompt: '',
  sceneOpacity: 1,
  coverMediaId: null,
  compressExport: true,
  backgroundTrackId: null,
  backdrop: null,
  backgroundAppearance: DEFAULT_BACKGROUND_APPEARANCE,
})

test('a template without recipe settings resolves to the classic behaviour', () => {
  const tracks: TrackSlice[] = [{ id: 'scenes', templateSlot: 'scenes' }, { id: 'audio', templateSlot: 'source-audio' }]
  const recipe = resolveTemplateRecipe({ tracks })
  assert.deepEqual(recipe, {
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
  })
  assert.equal(recipeImageCount(recipe, 60 * 90), 5)
})

test('the scenes recipe and the background track come from the tracks', () => {
  const tracks: TrackSlice[] = [
    { id: 'bg', templateSlot: 'background-video' },
    { id: 'scenes', templateSlot: 'scenes', templateScenes: { style: 'cutout', images: { mode: 'per-minutes', minutes: 4 }, motion: { kind: 'wander', intensity: 1.5, speed: 2 } } },
  ]
  const recipe = resolveTemplateRecipe({ tracks })
  assert.equal(recipe.style, 'cutout')
  assert.equal(recipe.backgroundTrackId, 'bg')
  assert.deepEqual(recipe.motion, { kind: 'wander', intensity: 1.5, speed: 2 })
  assert.equal(recipe.thumbnail, 'clean', 'a recipe saved before thumbnail variants keeps the clean thumbnail')
})

test('the backdrop comes from the background track only', () => {
  const templateBackdrop: TemplateBackdrop = { channels: [{ channelId: 'bilibili:12345', kinds: ['video'], pinned: ['BV1xx411c7mD'] }], clipCount: 4, clipSec: 30 }
  assert.deepEqual(resolveTemplateRecipe({ tracks: [{ id: 'bg', templateSlot: 'background-video', templateBackdrop }] }).backdrop, templateBackdrop)
  assert.equal(resolveTemplateRecipe({ tracks: [{ id: 'bg', templateSlot: 'background-video' }] }).backdrop, null)
  assert.equal(resolveTemplateRecipe({ tracks: [{ id: 'other', templateBackdrop }] }).backdrop, null, 'a stray field off the background slot is ignored')
})

test('the thumbnail variant comes from the scenes recipe', () => {
  const templateScenes = { style: 'full', images: { mode: 'fixed', count: 5 }, motion: { kind: 'ken-burns', intensity: 1, speed: 1 }, thumbnail: 'circle-arrow' } as const
  const recipe = resolveTemplateRecipe({ tracks: [{ id: 'scenes', templateSlot: 'scenes', templateScenes }] })
  assert.equal(recipe.thumbnail, 'circle-arrow')
})

test('the scene opacity comes from the scenes recipe and defaults to opaque', () => {
  const base = { style: 'full', images: { mode: 'fixed', count: 5 }, motion: { kind: 'ken-burns', intensity: 1, speed: 1 } } as const
  assert.equal(resolveTemplateRecipe({ tracks: [{ id: 'scenes', templateSlot: 'scenes', templateScenes: base }] }).sceneOpacity, 1)
  assert.equal(resolveTemplateRecipe({ tracks: [{ id: 'scenes', templateSlot: 'scenes', templateScenes: { ...base, opacity: 0.6 } }] }).sceneOpacity, 0.6)
  assert.equal(resolveTemplateRecipe({ tracks: [{ id: 'other', templateScenes: { ...base, opacity: 0.6 } }] }).sceneOpacity, 1, 'a stray field off the scenes slot is ignored')
})

test('per-minutes derives the image count from the source duration', () => {
  const recipe = perMinutes(4)
  for (const [minutes, expected] of [[73, 19], [95, 24], [125, 32]] as const) {
    assert.equal(recipeImageCount(recipe, minutes * 60), expected)
  }
})

test('the derived count is clamped to 1..60', () => {
  assert.equal(recipeImageCount(perMinutes(30), 60), 1)
  assert.equal(recipeImageCount(perMinutes(1), 125 * 60), 60)
  assert.equal(recipeImageCount(perMinutes(4), 0), 1)
})

test('common appearance resolves identity unless stored on the background slot', () => {
  const appearance = { ...DEFAULT_BACKGROUND_APPEARANCE, speed: 2, blurRadius: 8, flipX: true }
  assert.deepEqual(resolveTemplateRecipe({ tracks: [] }).backgroundAppearance, {
    opacity: 1, blurRadius: 0, brightness: 0, contrast: 1, saturation: 1,
    hueShift: 0, speed: 1, zoom: 1, flipX: false, flipY: false,
  })
  assert.deepEqual(resolveTemplateRecipe({ tracks: [{ id: 'bg', templateSlot: 'background-video', templateBackgroundAppearance: appearance }] }).backgroundAppearance, appearance)
  assert.deepEqual(resolveTemplateRecipe({ tracks: [{ id: 'other', templateBackgroundAppearance: appearance }] }).backgroundAppearance, DEFAULT_BACKGROUND_APPEARANCE)
})

test('images mode none resolves and counts zero images', () => {
  const base = { style: 'full', images: { mode: 'none' }, motion: { kind: 'ken-burns', intensity: 1, speed: 1 } } as const
  const recipe = resolveTemplateRecipe({ tracks: [{ id: 'scenes', templateSlot: 'scenes', templateScenes: base }] })
  assert.deepEqual(recipe.images, { mode: 'none' })
  assert.equal(recipeImageCount(recipe, 60 * 90), 0)
})

test('the cover media id and compress choice come from the scenes recipe', () => {
  const base = { style: 'full', images: { mode: 'none' }, motion: { kind: 'ken-burns', intensity: 1, speed: 1 } } as const
  const recipe = resolveTemplateRecipe({ tracks: [{ id: 'scenes', templateSlot: 'scenes', templateScenes: { ...base, coverMediaId: 'm1', compress: false } }] })
  assert.equal(recipe.coverMediaId, 'm1')
  assert.equal(recipe.compressExport, false)
  const plain = resolveTemplateRecipe({ tracks: [{ id: 'scenes', templateSlot: 'scenes', templateScenes: base }] })
  assert.equal(plain.coverMediaId, null)
  assert.equal(plain.compressExport, true, 'absent compress means the export is compressed')
})
