// Bảng route 02-ia-flows.md: mọi hình dạng hash phải parse đúng, đường dẫn tạo ra phải quay lại đúng route.
import { describe, expect, it } from 'vitest'
import { isPublicRoute, needsWelcomeRedirect, parseHash, paths } from './router'

describe('parseHash — bảng route 02', () => {
  it('gốc, dự án, truyện', () => {
    expect(parseHash('').route).toEqual({ name: 'project', project: null })
    expect(parseHash('#/').route).toEqual({ name: 'project', project: null })
    expect(parseHash('#/p/dit-dang-audio').route).toEqual({ name: 'project', project: 'dit-dang-audio' })
    expect(parseHash('#/p/dit-dang-audio/s/01-tra-xanh').route).toEqual({ name: 'story', project: 'dit-dang-audio', story: '01-tra-xanh' })
  })

  it('workspace, nhật ký, trình đọc', () => {
    expect(parseHash('#/p/p/s/s/work').route).toEqual({ name: 'workspace', project: 'p', story: 's' })
    expect(parseHash('#/p/p/s/s/activity').route).toEqual({ name: 'activity', project: 'p', story: 's' })
    expect(parseHash('#/p/p/s/s/read').route).toEqual({ name: 'reader', project: 'p', story: 's', chapter: null })
    expect(parseHash('#/p/p/s/s/read/12').route).toEqual({ name: 'reader', project: 'p', story: 's', chapter: 12 })
    expect(parseHash('#/p/p/s/s/read/abc').route).toEqual({ name: 'reader', project: 'p', story: 's', chapter: null })
  })

  it('audio: review, make, listen và tập', () => {
    expect(parseHash('#/p/p/s/s/audio').route).toEqual({ name: 'audio', project: 'p', story: 's', volume: null, mode: 'review' })
    expect(parseHash('#/p/p/s/s/audio/2').route).toEqual({ name: 'audio', project: 'p', story: 's', volume: 2, mode: 'review' })
    expect(parseHash('#/p/p/s/s/audio/2/make').route).toEqual({ name: 'audio', project: 'p', story: 's', volume: 2, mode: 'make' })
    expect(parseHash('#/p/p/s/s/audio/2/listen').route).toEqual({ name: 'audio', project: 'p', story: 's', volume: 2, mode: 'listen' })
    expect(parseHash('#/p/p/s/s/audio/2/khac').route).toEqual({ name: 'missing' })
  })

  it('truyện mới, tuỳ chỉnh, đăng nhập, ghép máy, onboarding', () => {
    expect(parseHash('#/p/p/new').route).toEqual({ name: 'newstory', project: 'p' })
    expect(parseHash('#/settings').route).toEqual({ name: 'settings', section: null })
    expect(parseHash('#/settings/writing').route).toEqual({ name: 'settings', section: 'writing' })
    expect(parseHash('#/login').route).toEqual({ name: 'login' })
    expect(parseHash('#/pair/AB12-CD34').route).toEqual({ name: 'pair', code: 'AB12-CD34' })
    expect(parseHash('#/welcome').route).toEqual({ name: 'welcome', step: null })
    expect(parseHash('#/welcome/2').route).toEqual({ name: 'welcome', step: '2' })
  })

  it('#/book/:id cũ → workspace của thư viện cũ', () => {
    expect(parseHash('#/book/nguoc-dong').route).toEqual({ name: 'workspace', project: '_', story: 'nguoc-dong' })
  })

  it('hộp thoại và query đi kèm route', () => {
    const parsed = parseHash('#/p/p/s/s/work?dlg=palette&tab=all')
    expect(parsed.route.name).toBe('workspace')
    expect(parsed.dialog).toBe('palette')
    expect(parsed.search.get('tab')).toBe('all')
  })

  it('Câu 11: “Thêm ghi chú cho Người viết” mở đúng chương và giữ `note=1` cho ô bình luận', () => {
    const path = paths.readerNote('p', 's', 3)
    expect(path).toBe('/p/p/s/s/read/3?note=1')
    const parsed = parseHash(`#${path}`)
    expect(parsed.route).toEqual({ name: 'reader', project: 'p', story: 's', chapter: 3 })
    expect(parsed.search.get('note')).toBe('1')
  })

  it('route lạ → missing', () => {
    expect(parseHash('#/p').route).toEqual({ name: 'missing' })
    expect(parseHash('#/p/p/s/s/khong-co').route).toEqual({ name: 'missing' })
    expect(parseHash('#/p/p/s').route).toEqual({ name: 'missing' })
  })

  it('id có ký tự đặc biệt được encode/decode tròn', () => {
    const path = paths.story('dự án', 'truyện 1')
    expect(path).toBe('/p/d%E1%BB%B1%20%C3%A1n/s/truy%E1%BB%87n%201')
    expect(parseHash(`#${path}`).route).toEqual({ name: 'story', project: 'dự án', story: 'truyện 1' })
  })
})

describe('paths — dựng lại route', () => {
  it('các màn chính', () => {
    expect(paths.workspace('p', 's')).toBe('/p/p/s/s/work')
    expect(paths.activity('p', 's')).toBe('/p/p/s/s/activity')
    expect(paths.reader('p', 's')).toBe('/p/p/s/s/read')
    expect(paths.reader('p', 's', 3)).toBe('/p/p/s/s/read/3')
    expect(paths.newstory('p')).toBe('/p/p/new')
    expect(paths.settings('writing')).toBe('/settings/writing')
  })

  it('audio theo chế độ', () => {
    expect(paths.audio('p', 's')).toBe('/p/p/s/s/audio')
    expect(paths.audio('p', 's', 2)).toBe('/p/p/s/s/audio/2')
    expect(paths.audio('p', 's', 2, 'make')).toBe('/p/p/s/s/audio/2/make')
    expect(paths.audio('p', 's', 2, 'listen')).toBe('/p/p/s/s/audio/2/listen')
    expect(parseHash(`#${paths.audio('p', 's', 2, 'listen')}`).route).toEqual({ name: 'audio', project: 'p', story: 's', volume: 2, mode: 'listen' })
  })
})

describe('needsWelcomeRedirect — không chặn các bước onboarding', () => {
  const fresh = { onboardedAt: null }
  const done = { onboardedAt: '2026-01-01T00:00:00Z' }

  it('đang ở welcome (mọi bước) → không chuyển', () => {
    expect(needsWelcomeRedirect(parseHash('#/welcome').route, fresh)).toBe(false)
    expect(needsWelcomeRedirect(parseHash('#/welcome/password').route, fresh)).toBe(false)
    expect(needsWelcomeRedirect(parseHash('#/welcome/2').route, fresh)).toBe(false)
  })

  it('route công khai → không chuyển', () => {
    expect(isPublicRoute(parseHash('#/login').route)).toBe(true)
    expect(isPublicRoute(parseHash('#/pair/AB12').route)).toBe(true)
    expect(isPublicRoute(parseHash('#/p/x').route)).toBe(false)
    expect(needsWelcomeRedirect(parseHash('#/login').route, fresh)).toBe(false)
  })

  it('route thường + chưa onboard → chuyển về welcome', () => {
    expect(needsWelcomeRedirect(parseHash('#/p/dit-dang-audio').route, fresh)).toBe(true)
    expect(needsWelcomeRedirect(parseHash('#/settings').route, fresh)).toBe(true)
  })

  it('đã onboard hoặc chưa đăng nhập → không chuyển', () => {
    expect(needsWelcomeRedirect(parseHash('#/p/dit-dang-audio').route, done)).toBe(false)
    expect(needsWelcomeRedirect(parseHash('#/p/dit-dang-audio').route, null)).toBe(false)
  })
})
