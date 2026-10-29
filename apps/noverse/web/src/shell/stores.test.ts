// Nháp ô chat: set ở Trình đọc, Workspace consume đúng một lần (không tự gửi).
import { beforeEach, describe, expect, it, vi } from 'vitest'

const memory = new Map<string, string>()
vi.stubGlobal('sessionStorage', {
  getItem: (key: string) => memory.get(key) ?? null,
  setItem: (key: string, value: string) => { memory.set(key, value) },
  removeItem: (key: string) => { memory.delete(key) },
})

const { setComposerDraft, takeComposerDraft } = await import('./stores')

describe('setComposerDraft / takeComposerDraft', () => {
  beforeEach(() => memory.clear())

  it('chuyển nháp sang Workspace rồi xoá sau lần đọc đầu', () => {
    setComposerDraft('dit-dang-audio', '01-tra-xanh', '/reopen viết lại chương 3')
    expect(memory.get('ainovel.composer.dit-dang-audio/01-tra-xanh')).toBe('/reopen viết lại chương 3')
    expect(takeComposerDraft('dit-dang-audio', '01-tra-xanh')).toBe('/reopen viết lại chương 3')
    expect(takeComposerDraft('dit-dang-audio', '01-tra-xanh')).toBe('')
  })

  it('nháp của truyện khác không lẫn', () => {
    setComposerDraft('p', 's1', 'chương 1')
    expect(takeComposerDraft('p', 's2')).toBe('')
    expect(takeComposerDraft('p', 's1')).toBe('chương 1')
  })

  it('không có nháp → chuỗi rỗng', () => {
    expect(takeComposerDraft('p', 's')).toBe('')
  })
})
