import { describe, expect, it } from 'vitest'
import { forgetProjectDraft, projectDraftKey, readProjectDraft, rememberProjectDraft } from './cocreateDraft'

const fakeStorage = () => {
  const data = new Map<string, string>()
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => { data.set(key, value) },
    removeItem: (key: string) => { data.delete(key) },
    data,
  }
}

describe('phiên nháp đồng sáng tác cấp dự án (G8)', () => {
  it('nhớ và đọc lại id nháp theo từng dự án', () => {
    const store = fakeStorage()
    expect(readProjectDraft('dit-dang-audio', store)).toBeNull()
    rememberProjectDraft('dit-dang-audio', 'nhap-7', store)
    expect(store.data.get(projectDraftKey('dit-dang-audio'))).toBe('nhap-7')
    expect(readProjectDraft('dit-dang-audio', store)).toBe('nhap-7')
    expect(readProjectDraft('du-an-khac', store)).toBeNull()
  })

  it('quên nháp sau commit/cancel', () => {
    const store = fakeStorage()
    rememberProjectDraft('p', 'nhap-1', store)
    forgetProjectDraft('p', store)
    expect(readProjectDraft('p', store)).toBeNull()
  })

  it('coi id rỗng là không có nháp', () => {
    const store = fakeStorage()
    rememberProjectDraft('p', '   ', store)
    expect(readProjectDraft('p', store)).toBeNull()
  })

  it('sessionStorage bị chặn thì trả null thay vì ném lỗi', () => {
    const blocked = {
      getItem: () => { throw new Error('blocked') },
      setItem: () => { throw new Error('blocked') },
      removeItem: () => { throw new Error('blocked') },
    }
    expect(readProjectDraft('p', blocked)).toBeNull()
    expect(() => rememberProjectDraft('p', 'x', blocked)).not.toThrow()
    expect(() => forgetProjectDraft('p', blocked)).not.toThrow()
  })
})
