import { describe, expect, it } from 'vitest'
import { assistantCopy } from '../../i18n/assistant'
import { workbenchPlaceholder } from './placeholders'

const p = assistantCopy.composer.placeholders
const steps = [
  ['download', 'scout'], ['backdrop', 'scout'], ['transcript', 'scout'],
  ['metadata', 'director'], ['scenes', 'director'], ['images', 'artist'],
  ['build', 'qa'], ['score', 'qa'], ['export', 'qa'],
] as const

describe.each(['working', 'attention'])('composer role for actual %s pipeline step', stageGroup => {
  it.each(steps)('%s targets %s instead of assuming every unfinished step draws images', (activeStep, role) => {
    expect(workbenchPlaceholder({ stageGroup, activeStep })).toBe(p[role])
  })
})
it('preserves review and paused-image guidance without inventing a role for unknown or finished steps', () => {
  expect(workbenchPlaceholder({ stageGroup: 'review', activeStep: 'images' })).toBe(p.review)
  expect(workbenchPlaceholder({ stageGroup: 'paused', activeStep: 'images' })).toBe(p.artistPaused)
  expect(workbenchPlaceholder({ stageGroup: 'paused', activeStep: 'metadata' })).toBe(p.director)
  expect(workbenchPlaceholder({ stageGroup: 'working', activeStep: null })).toBe(p.assistant)
  expect(workbenchPlaceholder({ stageGroup: 'ready', activeStep: 'export' })).toBe(p.assistant)
  expect(workbenchPlaceholder({ stageGroup: 'posted', activeStep: 'export' })).toBe(p.assistant)
})
