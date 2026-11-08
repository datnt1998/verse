import { describe, expect, it } from 'vitest'
import { progressText } from '../progress-copy'
import { cloneFailure, cloneStepWireSchema, type CloneStatusWire } from './clone-view'
import { cloneWaitReason } from './clone-wait'

const now = Date.parse('2026-11-08T17:16:36Z')
const retryAt = new Date(now + 30 * 60_000).toISOString()
const storedBot = 'yt-dlp: Sign in to confirm you’re not a bot. job96153efd-private'

function snapshot(step: string, message: string, waitReason?: string): Pick<CloneStatusWire, 'job' | 'steps'> {
  return {
    job: { id: 'private', sourceKey: 'private', status: 'running', activeStep: step, generation: 1, projectId: null, cancelRequestedAt: null, createdAt: new Date(now).toISOString(), noImages: false, postedAt: null, exportRemovedAt: null, updatedAt: new Date(now).toISOString() },
    steps: [cloneStepWireSchema.parse({ step, status: 'waiting', attempt: 1, progress: null, error: { code: 'quota', message, recovery: null, retryAt, retryAtSource: 'worker', ...(waitReason ? { waitReason } : {}) }, retryAt: null, startedAt: null, finishedAt: null })],
  }
}

describe('visible wait causes, independently of the parking code', () => {
  it.each([
    ['download', storedBot, undefined, 'source_auth', /YouTube/, 'cookies'],
    ['download', 'Không đủ chỗ: cần 4 GB', undefined, 'storage', /dung lượng/, null],
    ['download', 'Transcode nguồn bị timeout', undefined, 'transient', /tạm thời/, null],
    ['download', 'HTTP Error 429 private-id', undefined, 'source', /nguồn/, null],
    ['download', 'safe', 'source_auth', 'source_auth', /YouTube/, 'cookies'],
    ['download', 'safe', 'storage', 'storage', /dung lượng/, null],
    ['download', 'safe', 'transient', 'transient', /tạm thời/, null],
    ['backdrop', 'HTTP Error 429 private-id', undefined, 'source', /nguồn/, null],
    ['images', 'image limit', undefined, 'image_quota', /hạn mức.*ảnh/, null],
    ['build', storedBot, undefined, 'transient', /tạm thời/, null],
  ] as const)('%s %s %s', (step, message, reason, expectedReason, visible, settings) => {
    const data = snapshot(step, message, reason)
    const row = data.steps[0]
    expect(cloneWaitReason(step, row.error)).toBe(expectedReason)
    const text = progressText({ ...row, progress: row.progress ?? { unitsDone: 0, unitsTotal: null, phase: '' } })
    const failure = cloneFailure(data, now)!
    expect(text).toMatch(visible)
    expect(failure.sentence).toMatch(visible)
    expect(failure.settingsTool).toBe(settings)
    for (const shown of [text, failure.sentence]) {
      expect(shown).not.toContain('private')
      expect(shown).not.toContain('Sign in')
      expect(shown).not.toContain('HTTP Error')
      if (step !== 'images') expect(shown).not.toMatch(/hạn mức|hết lượt/)
    }
    expect(failure.retryAt).toBe(step === 'images' ? null : retryAt)
    expect(row.error?.code).toBe('quota')
  })

  it('only promises worker retry while the worker owns an eligible backoff', () => {
    const data = snapshot('download', storedBot)
    expect(cloneFailure(data, now)?.retryAt).toBe(retryAt)
    data.job.status = 'waiting'
    expect(cloneFailure(data, now)?.retryAt).toBeNull()
    data.job.status = 'running'
    data.steps[0].attempt = 3
    expect(cloneFailure(data, now)?.retryAt).toBeNull()
    data.steps[0].attempt = 1
    data.steps[0].error!.retryAtSource = 'provider'
    expect(cloneFailure(data, now)?.retryAt).toBeNull()
    data.steps[0].error!.retryAtSource = 'worker'
    data.steps[0].error!.retryAt = new Date(now + 30 * 60_000 + 1).toISOString()
    expect(cloneFailure(data, now)?.retryAt).toBeNull()
  })

  it('unknown future reasons degrade to a known step cause, not raw text', () => {
    const data = snapshot('download', 'HTTP Error 429 private-id', 'future_reason')
    expect(progressText({ ...data.steps[0], progress: { unitsDone: 0, unitsTotal: null, phase: '' } })).toMatch(/nguồn/)
    expect(cloneWaitReason('images', { code: 'quota', message: storedBot, waitReason: 'source_auth' })).toBe('image_quota')
    expect(cloneWaitReason('download', { code: 'invalid_request', message: storedBot })).toBeNull()
  })
})
