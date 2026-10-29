import { describe, expect, it } from 'vitest'
import { briefSummary, formatCost, formatDuration, hasActiveRun, statusLabel, validateBrief } from './lib'
import type { Run } from './api'

describe('validateBrief', () => {
  it('rejects empty topic', () => {
    expect(validateBrief({ topic: ' ', audience: '', lengthKind: 'seconds', lengthValue: '60', wpm: '' }).error).toBeTruthy()
  })

  it('rejects non-positive length', () => {
    expect(validateBrief({ topic: 'x', audience: '', lengthKind: 'words', lengthValue: '0', wpm: '' }).error).toBeTruthy()
    expect(validateBrief({ topic: 'x', audience: '', lengthKind: 'seconds', lengthValue: 'abc', wpm: '' }).error).toBeTruthy()
  })

  it('builds a seconds brief and trims fields', () => {
    const { brief, error } = validateBrief({
      topic: '  Lịch sử hình thành Trái Đất  ',
      audience: ' người thích khoa học ',
      lengthKind: 'seconds',
      lengthValue: '90',
      wpm: '150',
    })
    expect(error).toBeUndefined()
    expect(brief).toEqual({
      topic: 'Lịch sử hình thành Trái Đất',
      audience: 'người thích khoa học',
      target_seconds: 90,
      words_per_minute: 150,
    })
  })

  it('builds a words brief without optional fields', () => {
    const { brief } = validateBrief({ topic: 'x', audience: '', lengthKind: 'words', lengthValue: '800', wpm: '' })
    expect(brief).toEqual({ topic: 'x', target_words: 800 })
  })
})

describe('hasActiveRun', () => {
  const run = (status: Run['status']) => ({ status }) as Run
  it('true when queued or running exists', () => {
    expect(hasActiveRun([run('succeeded'), run('queued')])).toBe(true)
    expect(hasActiveRun([run('running')])).toBe(true)
  })
  it('false when all finished', () => {
    expect(hasActiveRun([run('succeeded'), run('failed')])).toBe(false)
    expect(hasActiveRun([])).toBe(false)
  })
})

describe('formatters', () => {
  it('formats duration', () => {
    expect(formatDuration(90)).toBe('1m30s')
    expect(formatDuration(45)).toBe('45s')
    expect(formatDuration(undefined)).toBe('—')
  })
  it('formats cost', () => {
    expect(formatCost(0.012345)).toBe('$0.0123')
    expect(formatCost(undefined)).toBe('—')
  })
  it('labels statuses', () => {
    expect(statusLabel('queued')).toBe('Đang chờ')
    expect(statusLabel('failed')).toBe('Thất bại')
  })
  it('summarizes briefs', () => {
    expect(briefSummary({ topic: 't', target_seconds: 60 })).toBe('t · 1m0s')
    expect(briefSummary({ topic: 't', target_words: 500 })).toBe('t · 500 từ')
    expect(briefSummary(undefined)).toBe('—')
  })
})
