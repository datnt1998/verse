import { describe, expect, it } from 'vitest'
import type { ProjectSummary, StorySummary } from '../../../api/types'
import {
  clipLabel, displayStoryTitle, pendingObject, projectRuleSub, showsScope, staleScopeFallback, storyBoxLabel,
  storyForProject, storyRuleSub, unsavedText, viewingElsewhereText, type OpenTarget,
} from './model'

const open: OpenTarget = { project: 'van-chau', story: 'dam-gio', title: 'Đám Giỗ Cả Họ' }
const story = (id: string, title = id): StorySummary => ({
  id: `van-chau/${id}`, project: 'van-chau', story: id, folder: `kich_ban/01-${id}`, title,
  status: 'writing', tags: [], idea: '', phase: 'writing', completedChapters: 3, totalChapters: 8,
  totalWords: 1000, volumes: 1, open: true, running: false, locked: false, updatedAt: null, exportsCount: 0, tempName: false,
})

describe('thanh “Áp dụng cho” chỉ ở mục 3 cấp', () => {
  it('hiện ở Cách viết, Trợ lý AI, Ngân sách, Thông tin', () => {
    for (const id of ['writing', 'models', 'budget', 'info'] as const) expect(showsScope(id)).toBe(true)
  })

  it('không hiện ở Quy tắc chung, mục Hệ thống hay Giọng đọc', () => {
    for (const id of ['rules', 'connect', 'export', 'notify', 'app', 'adv', 'voice'] as const) expect(showsScope(id)).toBe(false)
  })
})

describe('nhãn ô và dòng danh sách', () => {
  it('“Truyện này” chỉ khi đúng truyện đang mở', () => {
    expect(storyBoxLabel({ project: 'van-chau', story: 'dam-gio' }, open)).toBe('Truyện này')
    expect(storyBoxLabel({ project: 'van-chau', story: 'khac' }, open)).toBe('Truyện')
    expect(storyBoxLabel({ project: 'khac', story: 'dam-gio' }, open)).toBe('Truyện')
  })

  it('cắt tên ở 22 ký tự rồi thêm “…”', () => {
    expect(clipLabel('Đám Giỗ Cả Họ')).toBe('Đám Giỗ Cả Họ')
    expect(clipLabel('Một cái tên truyện rất dài không vừa ô')).toBe('Một cái tên truyện rất…')
  })

  it('“N truyện · M quy tắc riêng”, hoặc theo “Mọi truyện” khi chưa có luật riêng', () => {
    expect(projectRuleSub(3, 2)).toBe('3 truyện · 2 quy tắc riêng')
    expect(projectRuleSub(3, 0)).toBe('3 truyện · theo “Mọi truyện”')
    expect(storyRuleSub(4)).toBe('4 quy tắc riêng')
    expect(storyRuleSub(0)).toBe('theo dự án')
  })
})

describe('dải xanh “Đang xem…”', () => {
  const names = { projectName: 'Vân Châu', storyTitle: 'Đám Giỗ Cả Họ', open }

  it('hiện khi xem dự án khác', () => {
    expect(viewingElsewhereText({ scope: 'project', project: 'khac', story: '' }, names))
      .toBe('Đang xem quy tắc của dự án “Vân Châu”. Truyện đang mở ở Bàn viết vẫn là “Đám Giỗ Cả Họ”.')
  })

  it('hiện khi xem truyện khác', () => {
    expect(viewingElsewhereText({ scope: 'story', project: 'van-chau', story: 'khac' }, { ...names, storyTitle: 'Truyện Khác' }))
      .toBe('Đang xem quy tắc của truyện “Truyện Khác”. Truyện đang mở ở Bàn viết vẫn là “Đám Giỗ Cả Họ”.')
  })

  it('không hiện khi đang ở truyện đang mở, dự án của nó, hay “Mọi truyện”', () => {
    expect(viewingElsewhereText({ scope: 'story', project: 'van-chau', story: 'dam-gio' }, names)).toBeNull()
    expect(viewingElsewhereText({ scope: 'project', project: 'van-chau', story: 'dam-gio' }, names)).toBeNull()
    expect(viewingElsewhereText({ scope: 'global', project: 'van-chau', story: 'dam-gio' }, names)).toBeNull()
  })

  it('không có truyện đang mở thì không có dải', () => {
    expect(viewingElsewhereText({ scope: 'story', project: 'van-chau', story: 'khac' }, { ...names, open: null })).toBeNull()
  })
})

describe('hộp cam “chưa lưu”', () => {
  it('gọi đúng đối tượng theo cấp', () => {
    expect(pendingObject({ scope: 'project', project: 'van-chau', story: '' }, 'Vân Châu', '')).toBe('dự án “Vân Châu”')
    expect(pendingObject({ scope: 'story', project: 'van-chau', story: 'dam-gio' }, 'Vân Châu', 'Đám Giỗ Cả Họ')).toBe('truyện “Đám Giỗ Cả Họ”')
    expect(pendingObject({ scope: 'global', project: 'van-chau', story: '' }, 'Vân Châu', '')).toBe('“Mọi truyện”')
  })

  it('câu đúng mẫu spec', () => {
    expect(unsavedText(2, 'dự án “Vân Châu”')).toBe('Có 2 thay đổi chưa lưu cho dự án “Vân Châu”. Lưu trước khi chuyển?')
  })
})

describe('đổi dự án thì chọn truyện', () => {
  it('cùng dự án thì giữ truyện đang mở', () => {
    expect(storyForProject('van-chau', [story('khac'), story('dam-gio')], open)).toBe('dam-gio')
  })

  it('dự án khác thì lấy truyện đầu', () => {
    expect(storyForProject('khac', [story('mot'), story('hai')], open)).toBe('mot')
  })

  it('dự án chưa có truyện thì rỗng', () => {
    expect(storyForProject('khac', [], open)).toBe('')
  })
})

describe('ngữ cảnh cũ trỏ vào dự án/truyện không có trên máy chủ', () => {
  const projects: ProjectSummary[] = [
    { id: 'cu', name: 'Cũ', description: '', legacy: true, storyCount: 1, runningCount: 0, updatedAt: null },
    { id: 'van-chau', name: 'Vân Châu', description: '', legacy: false, storyCount: 2, runningCount: 0, updatedAt: null },
    { id: 'tro-ve', name: 'Trở Về', description: '', legacy: false, storyCount: 0, runningCount: 0, updatedAt: null },
  ]
  const lists: Record<string, readonly StorySummary[]> = {
    cu: [story('cu-mot')],
    'van-chau': [story('khac'), story('dam-gio')],
    'tro-ve': [],
  }
  const ctx = (load: (id: string) => readonly StorySummary[] | null = (id) => lists[id]) =>
    ({ projects, open, storiesOf: load })

  it('dự án không còn trên máy chủ → về truyện đang mở', () => {
    expect(staleScopeFallback({ project: 'dit-dang-audio', story: '01-tra-xanh' }, ctx()))
      .toEqual({ project: 'van-chau', story: 'dam-gio' })
  })

  it('dự án không còn, không có truyện đang mở → dự án đầu (bỏ dự án legacy)', () => {
    expect(staleScopeFallback({ project: 'dit-dang-audio', story: '' }, { ...ctx(), open: null }))
      .toEqual({ project: 'van-chau', story: 'khac' })
  })

  it('truyện không còn trong dự án → truyện đang mở nếu cùng dự án, không thì truyện đầu', () => {
    expect(staleScopeFallback({ project: 'van-chau', story: 'da-xoa' }, ctx()))
      .toEqual({ project: 'van-chau', story: 'dam-gio' })
    expect(staleScopeFallback({ project: 'cu', story: 'da-xoa' }, ctx()))
      .toEqual({ project: 'cu', story: 'cu-mot' })
  })

  it('chưa tải xong danh sách truyện thì chưa kết luận', () => {
    expect(staleScopeFallback({ project: 'van-chau', story: 'da-xoa' }, ctx(() => null))).toBeNull()
    expect(staleScopeFallback({ project: 'tro-ve', story: 'x' }, ctx(() => null))).toBeNull()
  })

  it('ngữ cảnh còn đúng thì không đụng vào', () => {
    expect(staleScopeFallback({ project: 'van-chau', story: 'dam-gio' }, ctx())).toBeNull()
    expect(staleScopeFallback({ project: 'tro-ve', story: '' }, ctx())).toBeNull()
    expect(staleScopeFallback({ project: '', story: '' }, ctx())).toBeNull()
  })

  it('máy chủ không có dự án nào thì không có gì để rơi về', () => {
    expect(staleScopeFallback({ project: 'dit-dang-audio', story: 'x' }, { ...ctx(), projects: [] })).toBeNull()
  })
})

describe('tên truyện hiển thị', () => {
  it('dùng tên máy chủ trả, không lộ slug', () => {
    expect(displayStoryTitle(story('dam-gio', 'Đám Giỗ Cả Họ'))).toBe('Đám Giỗ Cả Họ')
    expect(displayStoryTitle({ ...story('01-truyen-moi'), title: '01-truyen-moi', tempName: true })).toBe('Truyện chưa đặt tên')
  })
})
