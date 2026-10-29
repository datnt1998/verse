// Dock việc chạy nền: thứ tự ưu tiên (§1.6), máy trạng thái (§1.2), thời gian còn lại, lọc Workspace.
import { describe, expect, it } from 'vitest'
import type { Job } from '../../api/types'
import {
  aggregateProgress, byPriority, dockReducer, formatEta, groupJobs, initialDock, kindColor, kindIcon, pillLines, showsAsDot,
  toastKindFor, unitText,
  visibleJobs, type DockState,
} from './logic'

const job = (id: string, patch: Partial<Job> = {}): Job => ({
  id, kind: 'export', title: id, status: 'running', progress: 0.5, href: '', project: 'p', story: 's',
  startedAt: '2026-10-25T10:00:00Z', ...patch,
})

describe('byPriority', () => {
  it('Cần bạn → Lỗi → Đang chạy → Tạm dừng → Xong, cùng mức theo giờ bắt đầu', () => {
    const list = [
      job('done', { status: 'done' }),
      job('paused', { status: 'paused' }),
      job('run-late', { startedAt: '2026-10-25T10:05:00Z' }),
      job('run-early', { startedAt: '2026-10-25T09:00:00Z' }),
      job('failed', { status: 'failed' }),
      job('waiting', { status: 'waiting' }),
    ]
    expect(byPriority(list).map((item) => item.id)).toEqual(['waiting', 'failed', 'run-early', 'run-late', 'paused', 'done'])
  })

  it('không đổi mảng gốc', () => {
    const list = [job('a', { status: 'done' }), job('b', { status: 'waiting' })]
    byPriority(list)
    expect(list.map((item) => item.id)).toEqual(['a', 'b'])
  })

  it('pill nói việc ưu tiên nhất trước; hết việc → ✓ của việc vừa xong', () => {
    const lines = pillLines([job('Xuất EPUB', { progress: 0.62 }), job('Nhập', { status: 'waiting', step: 'Chờ xác nhận' })], undefined)
    expect(lines[0]).toMatchObject({ a: 'Cần bạn · Nhập', b: 'Chờ xác nhận' })
    expect(lines[1].a).toBe('Xuất EPUB · 62%')
    const done = job('x', { status: 'done', result: { title: 'Đã xuất EPUB', sub: '', actions: [] } })
    expect(pillLines([], done)).toEqual([{ a: '✓ Đã xuất EPUB', b: 'Mọi việc đã xong', fg: '#A6EDC1' }])
  })

  it('nhóm bảng: Cần bạn gồm cả chờ lẫn lỗi; nhóm rỗng bị bỏ', () => {
    const groups = groupJobs([job('a'), job('b', { status: 'failed' }), job('c', { status: 'waiting' })])
    expect(groups.map((group) => [group.id, group.jobs.map((item) => item.id)])).toEqual([['need', ['c', 'b']], ['run', ['a']]])
  })
})

describe('formatEta', () => {
  it.each([
    [undefined, ''], [null, ''], [-5, ''], [Number.NaN, ''],
    [0, 'dưới 1 phút'], [59, 'dưới 1 phút'], [60, 'còn khoảng 1 phút'], [61, 'còn khoảng 2 phút'],
    [6 * 60, 'còn khoảng 6 phút'], [59 * 60, 'còn khoảng 59 phút'], [59 * 60 + 1, 'còn khoảng 1 giờ'],
    [3600, 'còn khoảng 1 giờ'], [2 * 3600 + 5 * 60, 'còn khoảng 2 giờ 5 phút'],
  ])('%s giây → "%s"', (sec, text) => {
    expect(formatEta(sec as number | undefined | null)).toBe(text)
  })

  it('Chương N/T: đang chạy hiện mục đang làm, xong hiện tổng', () => {
    expect(unitText(job('a', { unit: { done: 4, total: 12, label: 'Chương' } }))).toBe('Chương 5/12')
    expect(unitText(job('a', { unit: { done: 12, total: 12, label: 'Chương' } }))).toBe('Chương 12/12')
    expect(unitText(job('a', { status: 'done', unit: { done: 11, total: 12, label: 'Chương' } }))).toBe('Chương 12/12')
  })
})

describe('aggregateProgress', () => {
  it('trung bình việc đang chạy có %, bỏ qua việc không rõ % và tạm dừng', () => {
    expect(aggregateProgress([job('a', { progress: 0.2 }), job('b', { progress: 0.6 }), job('c', { progress: null }), job('d', { status: 'paused', progress: 0.9 })])).toBeCloseTo(0.4)
    expect(aggregateProgress([job('c', { progress: null })])).toBeNull()
  })
})

describe('dockReducer', () => {
  const run = (events: Parameters<typeof dockReducer>[1][], from: DockState = initialDock) => events.reduce(dockReducer, from)

  it('ẩn → pill khi có việc đầu tiên; thêm việc khi đang hiện không đổi', () => {
    const shown = run([{ type: 'jobAdded' }])
    expect(shown).toMatchObject({ mode: 'pill', enterN: 1 })
    expect(dockReducer(shown, { type: 'jobAdded' })).toBe(shown)
  })

  it('pill → bảng (morph vào) → settle; Esc/Thu gọn → morph ra rồi về pill', () => {
    const open = run([{ type: 'jobAdded' }, { type: 'clickPill', workspace: false }])
    expect(open).toMatchObject({ mode: 'panel', panelAnim: 'in' })
    const settled = dockReducer(open, { type: 'settle' })
    expect(settled.panelAnim).toBe('')
    const closing = dockReducer(settled, { type: 'close' })
    expect(closing).toMatchObject({ mode: 'panel', panelAnim: 'out', closingTo: 'pill' })
    expect(dockReducer(closing, { type: 'settle' })).toMatchObject({ mode: 'pill', panelAnim: '', closingTo: null })
  })

  it('chấm: ngoài Workspace bấm → pill; ở Workspace bấm → bảng', () => {
    const mini = run([{ type: 'jobAdded' }, { type: 'toMini' }])
    expect(mini.mode).toBe('mini')
    expect(dockReducer(mini, { type: 'clickPill', workspace: false }).mode).toBe('pill')
    expect(dockReducer(mini, { type: 'clickPill', workspace: true }).mode).toBe('panel')
  })

  it('bảng → thu nhỏ đi qua morph rồi thành chấm', () => {
    const toMini = run([{ type: 'jobAdded' }, { type: 'open' }, { type: 'settle' }, { type: 'toMini' }])
    expect(toMini).toMatchObject({ panelAnim: 'out', closingTo: 'mini' })
    expect(dockReducer(toMini, { type: 'settle' }).mode).toBe('mini')
  })

  it('⌥J: ẩn → mở bảng (chạy lại animation vào), mở → thu; bấm khi đang thu → mở lại', () => {
    const open = run([{ type: 'toggle' }])
    expect(open).toMatchObject({ mode: 'panel', panelAnim: 'in', enterN: 1 })
    const closing = run([{ type: 'settle' }, { type: 'toggle' }], open)
    expect(closing.panelAnim).toBe('out')
    expect(dockReducer(closing, { type: 'toggle' })).toMatchObject({ mode: 'panel', panelAnim: 'in', closingTo: null })
  })

  it('xong hết: idleOut → leaving → hide; việc mới tới giữa chừng huỷ biến mất; bảng mở thì không tự ẩn', () => {
    const leaving = run([{ type: 'jobAdded' }, { type: 'idleOut' }])
    expect(leaving.leaving).toBe(true)
    expect(dockReducer(leaving, { type: 'hide' })).toMatchObject({ mode: 'hidden', leaving: false })
    expect(dockReducer(leaving, { type: 'jobAdded' })).toMatchObject({ mode: 'pill', leaving: false })
    const panel = run([{ type: 'open' }])
    expect(dockReducer(panel, { type: 'idleOut' })).toBe(panel)
    expect(dockReducer(panel, { type: 'hide' })).toBe(panel)
  })

  it('khôi phục trạng thái đã lưu chỉ khi đang ẩn', () => {
    expect(run([{ type: 'restore', mode: 'mini' }])).toMatchObject({ mode: 'mini', enterN: 1 })
    const pill = run([{ type: 'jobAdded' }])
    expect(dockReducer(pill, { type: 'restore', mode: 'mini' })).toBe(pill)
  })

  it('Workspace: pill luôn hiện thành chấm', () => {
    expect(showsAsDot('pill', true)).toBe(true)
    expect(showsAsDot('pill', false)).toBe(false)
    expect(showsAsDot('mini', false)).toBe(true)
  })
})

describe('lọc & toast', () => {
  it('Workspace bỏ việc agent đang chạy của chính truyện đang mở, giữ việc khác & việc truyện khác', () => {
    const list = [
      job('agent', { agent: 'writer', kind: 'write' }),
      job('agent-done', { agent: 'writer', kind: 'write', status: 'done' }),
      job('other-story', { agent: 'writer', kind: 'write', story: 'x' }),
      job('audio', { kind: 'audio' }),
    ]
    expect(visibleJobs(list, { workspace: true, project: 'p', story: 's' }).map((item) => item.id)).toEqual(['agent-done', 'other-story', 'audio'])
    expect(visibleJobs(list, { workspace: false, project: 'p', story: 's' })).toHaveLength(4)
  })

  it('toast chỉ khi đổi sang xong / chờ bạn / lỗi', () => {
    expect(toastKindFor('running', 'done')).toBe('done')
    expect(toastKindFor('running', 'waiting')).toBe('wait')
    expect(toastKindFor(undefined, 'failed')).toBe('err')
    expect(toastKindFor('running', 'running')).toBeNull()
    expect(toastKindFor('running', 'paused')).toBeNull()
    expect(toastKindFor('waiting', 'waiting')).toBeNull()
  })
})

// Việc của máy (cài bộ đọc) dùng loại `voice`: cùng màu/icon bộ đọc, nhưng khác
// loại `audio` để không mang cột sóng và câu “Bỏ qua chương này” của việc tạo giọng.
describe('loại việc voice — cài bộ đọc trên máy', () => {
  const install = (patch: Partial<Job> = {}) => job('voice/install', {
    kind: 'voice', title: 'Cài bộ đọc trên máy', project: '', story: '', ...patch,
  })

  it('có màu và icon của bộ đọc', () => {
    expect(kindColor(install())).toBe('#C9A6F5')
    expect(kindIcon(install())).toBe('voice-square')
  })

  it('không bị lọc khỏi Workspace dù không thuộc truyện nào', () => {
    expect(visibleJobs([install()], { workspace: true, project: 'p', story: 's' })).toHaveLength(1)
  })

  it('xếp vào nhóm theo trạng thái như mọi việc khác', () => {
    const groups = groupJobs([install(), install({ id: 'voice/install-2', status: 'failed', error: 'chưa có uv' })])
    expect(groups.map((group) => group.label)).toEqual(['Cần bạn', 'Đang chạy'])
    expect(groups[0]?.jobs[0]?.error).toBe('chưa có uv')
  })
})
