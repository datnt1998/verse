// @vitest-environment happy-dom
// D15 · Não hệ thống: chữ khớp artboard SysBrain.dc.html, giữ/bỏ/hoàn tác tại chỗ ở tab Chờ duyệt.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Lesson, LessonAction, LessonsResponse, ViolationRow } from '../../api/types'
import { t } from '../../i18n/vi'
import { mockLessons, mockViolations } from '../../../mock/lessons'
import { SysBrainScreen, LessonList } from './index'
import { alertTip, pendingRows, showsScopeAll, violationBar } from './model'

vi.mock('../../api/session', () => ({
  useSession: () => ({ checked: true, user: { username: 'dat', name: 'Dat', role: 'admin' }, setPendingLessons: () => {} }),
}))

const calls = vi.hoisted(() => [] as { id: string; input: LessonAction }[])
vi.mock('../../api/client', async () => {
  // vi.mock được kéo lên trên mọi import tĩnh → factory phải tự nạp mock.
  const mock = await import('../../../mock/lessons')
  return {
    getLessons: async (state: Parameters<typeof mock.mockLessons>[0]) => mock.mockLessons(state),
    getViolations: async (pack: string) => mock.mockViolations(pack),
    actOnLesson: async (id: string, input: LessonAction) => {
      calls.push({ id, input })
      const { status, body } = mock.mockDecideLesson(id, input)
      if (status !== 200) throw new Error('error' in body ? body.error : String(status))
      return body
    },
  }
})

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | null = null
let container: HTMLElement | null = null
beforeEach(() => { calls.length = 0 })
afterEach(async () => {
  if (root) await act(() => root!.unmount())
  container?.remove()
  root = null; container = null
})

async function mount(tab: 'pending' | 'kept' | 'dropped') {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  await act(async () => { root!.render(<LessonList tab={tab} data={mockLessons(tab)} />) })
  return container
}
const card = (el: HTMLElement, id: string) => el.querySelector<HTMLElement>(`[data-lesson="${id}"]`)!

// Id thật của các bài học trong golden Go (lessons-pending/kept/dropped.json).
const L7 = 'lesson-aa4c23b5a78099e69721876e'   // Master §7 — đề xuất chung hai gói
const L13 = 'lesson-68e7ae019b8831ecd96ad853'  // Master §13
const LXH = 'lesson-user-xung-ho'             // Xưng hô — bài của người dùng (seed lessons.json)
const K12 = 'lesson-9694903220571ef8a4dd4a0a'  // Master §12 — giữ từ lessons.json
const K4 = 'lesson-97d5dc5c0626a74381b11a04'   // Master §4 — giữ sẵn trong lessons.json
const K10 = 'lesson-bb1c20cd5155e3d2fc9c6510'  // Master §10 — giữ sẵn
const D15 = 'lesson-4c36c9e294d209f762c5249f'  // Master §15 — bỏ qua Decide

const button = (scope: HTMLElement, label: string) => [...scope.querySelectorAll('button')].find((b) => b.textContent?.trim() === label)!

/** Dựng cả màn Não hệ thống theo `#/system/brain?tab=&pack=` (mock API của test). */
async function mountScreen(tab = 'pending', pack = 'nu-tan-bao-thu') {
  location.hash = `#/system/brain?tab=${tab}&pack=${pack}`
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  await act(async () => { root!.render(<SysBrainScreen />) })
  // useResource tải qua microtask: thêm một vòng để danh sách bài học vào DOM.
  await act(async () => {})
  return container
}

async function unmountScreen() {
  if (root) await act(() => root!.unmount())
  container?.remove()
  root = null; container = null
}

describe('Não hệ thống — chữ theo artboard', () => {
  it('dòng nguồn: mục dùng chung luôn ghi “· N gói” (kể cả 1), mục Bộ … ghi tên gói', () => {
    const all = [...mockLessons('pending').lessons, ...mockLessons('kept').lessons, ...mockLessons('dropped').lessons]
    // Số liệu nhóm là số liệu thật do lessons.Aggregate tính (golden của Go).
    const byRef = Object.fromEntries(all.map((lesson) => [lesson.refs[0], lesson]))
    const src = Object.fromEntries(all.map((lesson) => [lesson.refs[0], t.sysbrain.source(lesson)]))
    expect(src['Master §7']).toBe('Rút từ 6 lần Bắt sửa ở 5 truyện · 2 gói')
    expect(src['Master §13']).toBe('Rút từ 6 Cảnh báo ở 5 truyện · 2 gói')
    expect(src['Xưng hô']).toBe('Rút từ 3 lần Bắt sửa và 1 Cảnh báo ở 4 truyện · 2 gói')
    expect(src['Quy tắc của bạn']).toBe('Rút từ 9 Cảnh báo ở 6 truyện · 3 gói')
    expect(src['Master §4']).toBe('Rút từ 3 lần Bắt sửa ở 2 truyện · 1 gói')
    expect(src['Master §12']).toBe('Rút từ 2 lần bạn sửa kết chương ở 2 truyện · gói Nữ tần trọng sinh báo thù, vả mặt')
    expect(src['Master §15']).toBe('Rút từ 6 Cảnh báo ở 4 truyện · 2 gói')
    expect(byRef['Master §7'].evidence).toMatchObject({ errors: 6, warnings: 0, stories: 5, packsCount: 2, pack: '' })
    expect(byRef['Xưng hô'].evidence).toMatchObject({ errors: 3, warnings: 1 })
    expect(byRef['Master §12'].evidence).toMatchObject({ userEdits: 2, object: 'kết chương' })
    // Mục dùng chung nhóm qua mọi gói → đề xuất sẵn phạm vi “Mọi gói” (Câu 15); mục
    // Bộ A §4.3 riêng gói → “pack”.
    expect(byRef['Master §7'].scope).toBe('all')
    expect(byRef['Bộ A §4.3'].scope).toBe('pack')
    expect(byRef['Master §13'].text).toBe('Chương đầu hay thiếu thoại (dưới mức của gói). Mở bằng một cuộc đối đáp trước khi kể bối cảnh.')
  })

  it('gộp bằng chứng: không tách mức, mục chung gộp qua mọi gói, mục Bộ X §… theo gói', () => {
    // Quy tắc gộp do lessons.Aggregate của Go tính — mock không còn tự gom.
    // Golden: §13 nằm ở hai gói → một bài “5 truyện · 2 gói”; Bộ A §4.3 chỉ thuộc
    // gói A → bài scope pack, evidence mang tên gói.
    const pending = mockLessons('pending').lessons
    const byRef = Object.fromEntries(pending.map((lesson) => [lesson.refs[0], lesson]))
    expect(byRef['Master §13'].evidence).toMatchObject({ warnings: 6, errors: 0, stories: 5, packsCount: 2, pack: '' })
    expect(byRef['Master §13'].scope).toBe('all')
    expect(byRef['Bộ A §4.3'].evidence).toMatchObject({ warnings: 3, stories: 2, pack: 'Nữ tần trọng sinh báo thù, vả mặt' })
    expect(byRef['Bộ A §4.3'].scope).toBe('pack')
  })

  it('§M: câu nguồn ghép mọi phần khác 0 theo thứ tự bạn sửa → Bắt sửa → Cảnh báo', () => {
    const lesson = (evidence: Lesson['evidence']): Lesson => ({
      id: 'l', origin: 'review', refs: ['Master §1'], text: '', evidence,
      scope: 'all', packId: 'p', packName: 'P', state: 'pending', createdAt: '',
    })
    // Nhóm 2 Bắt sửa + 1 Cảnh báo: hai phần nối “ và ”, Cảnh báo đứng sau.
    expect(t.sysbrain.source(lesson({ errors: 2, warnings: 1, userEdits: 0, stories: 2, pack: '', packsCount: 1, chips: [] })))
      .toBe('Rút từ 2 lần Bắt sửa và 1 Cảnh báo ở 2 truyện · 1 gói')
    // Cả ba loại, bản ghi của người dùng (origin khác nhưng câu bám bằng chứng).
    expect(t.sysbrain.source({ ...lesson({ userEdits: 3, object: 'xưng hô', errors: 2, warnings: 0, stories: 4, pack: '', packsCount: 2, chips: [] }), origin: 'user' }))
      .toBe('Rút từ 3 lần bạn sửa xưng hô và 2 lần Bắt sửa ở 4 truyện · 2 gói')
    // Phần bằng 0 bị bỏ; mục Bộ … giữ hậu tố “· gói X”.
    expect(t.sysbrain.source(lesson({ errors: 0, warnings: 3, userEdits: 0, stories: 2, pack: 'Gói A', packsCount: 1, chips: [] })))
      .toBe('Rút từ 3 Cảnh báo ở 2 truyện · gói Gói A')
  })

  it('§M: chip loại bám origin, không đổi khi bằng chứng đếm lại là lần chấm', async () => {
    const data: LessonsResponse = mockLessons('kept')
    // Bài của người dùng bị lần quét mới ghi đè bằng chứng thành Bắt sửa — chip vẫn “Từ góp ý của bạn”.
    data.lessons = data.lessons.map((l) => l.id === K12
      ? { ...l, evidence: { ...l.evidence, userEdits: 0, object: undefined, errors: 2 } }
      : l)
    expect(t.sysbrain.source(data.lessons.find((l) => l.id === K12)!))
      .toBe('Rút từ 2 lần Bắt sửa ở 2 truyện · gói Nữ tần trọng sinh báo thù, vả mặt')
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => { root!.render(<LessonList tab="kept" data={data} />) })
    expect(card(container, K12).textContent).toContain('Từ góp ý của bạn')
  })

  it('mục luật hay vấp: thanh tỉ lệ, chuông tính trên mọi gói cho mục dùng chung, lời nhắc của gói', () => {
    const a = mockViolations('nu-tan-bao-thu')!
    expect(a.rows.map((row) => violationBar(row, a.stories).label)).toEqual(['4/6', '4/6', '3/6', '3/6', '3/6', '2/6', '1/6'])
    expect(violationBar(a.rows[0], a.stories).width).toBe('67%')
    expect(a.rows.filter((row) => row.alert).map((row) => row.ref)).toEqual(['Master §7', 'Master §11', 'Xưng hô'])
    // Câu 10: mục dùng chung mang `alertScope: 'all'` + số truyện Bắt sửa ở mọi gói; mục Bộ … không có.
    expect(a.rows.map((row) => [row.ref, row.alertScope, row.alertStories])).toEqual([
      ['Master §13', 'all', 0], ['Master §7', 'all', 5], ['Master §11', 'all', 3],
      ['Master §15', 'all', 0], ['Xưng hô', 'all', 3], ['Bộ A §4.3', undefined, undefined], ['Master §8', 'all', 2],
    ])
    // §J Câu 3: mục dùng chung có số truyện khác số trong gói thì lời nhắc ghi “N truyện ở mọi gói”;
    // §11 và Xưng hô trùng số trong gói nên vẫn n/tổng.
    expect(a.preamble).toBe('Truyện Nữ tần trọng sinh báo thù, vả mặt trước đây hay vấp Master §7 (đầu chương và cuối chương, 5 truyện ở mọi gói), Master §11 (công lý và kết cục phản diện (mới), 3/6 truyện), Xưng hô (bảng xưng hô của truyện, 3/6 truyện). Khi lập bản đồ giữ chân, kiểm kỹ các mục này trước khi giao Người viết.')
    expect(mockViolations('nien-dai-van')!.preamble).toBe('Chưa có mục Bắt sửa nào lặp ở 3 truyện trở lên trong gói Niên đại văn, trọng sinh thập niên, làm giàu, nên chưa có lời nhắc.')

    // Gói B: Master §7 chỉ có 1 truyện trong gói nhưng 5 truyện Bắt sửa ở mọi gói → chuông bật,
    // số trong bảng vẫn theo gói (1/4); Master §10 trùng số nên lời nhắc giữ 3/4.
    const b = mockViolations('nguoc-hon-nhan')!
    const seven = b.rows.find((row) => row.ref === 'Master §7')!
    expect([seven.stories, seven.total, seven.level, seven.alert, seven.alertScope, seven.alertStories]).toEqual([1, 4, 'error', true, 'all', 5])
    expect(b.rows.filter((row) => row.alert).map((row) => row.ref)).toEqual(['Master §10', 'Master §7', 'Xưng hô'])
    expect(b.preamble).toBe('Truyện Ngược hôn nhân, ly hôn, hối hận muộn trước đây hay vấp Master §10 (ức chế và lòng thương (mới), 3/4 truyện), Master §7 (đầu chương và cuối chương, 5 truyện ở mọi gói), Xưng hô (bảng xưng hô của truyện, 3 truyện ở mọi gói). Khi lập bản đồ giữ chân, kiểm kỹ các mục này trước khi giao Người viết.')
  })

  it('tab Chờ duyệt giữ bài vừa quyết đúng chỗ cũ', () => {
    const [a, b] = mockLessons('pending').lessons
    const rows = pendingRows([a.id, b.id], [b], { [a.id]: { ...a, state: 'kept' as const } })
    expect(rows.map((row) => [row.id, row.state])).toEqual([[a.id, 'kept'], [b.id, 'pending']])
  })
})

describe('Não hệ thống — giữ, bỏ, sửa câu chữ, hoàn tác', () => {
  it('thẻ chờ duyệt có chip, nhãn “mới · ngày”, bằng chứng, Áp dụng cho và ba nút', async () => {
    const el = await mount('pending')
    // Chín thẻ của golden: bài Xưng hô của người dùng (seed) + đề xuất Xưng hô của
    // máy (C1: thẻ xưng hô lưu trữ giờ vào Bài học) + Quy tắc của bạn + sáu ứng viên.
    expect(el.querySelectorAll('article')).toHaveLength(9)
    const l1 = card(el, L7)
    expect(l1.textContent).toContain('Từ lần chấm')
    expect(l1.textContent).toContain('Master §7')
    expect(l1.textContent).toContain('mới · 20/10')
    expect(l1.textContent).toContain('Em Gái Nuôi Mặc Váy Cưới · ch.5')
    const scope = l1.querySelector('[role=radiogroup][aria-label="Áp dụng cho"]')!
    // Mục chung (Master §…) nhóm qua mọi gói → đề xuất sẵn phạm vi “Mọi gói” (Câu 15).
    expect([...scope.querySelectorAll('[role=radio]')].map((r) => [r.textContent, r.getAttribute('aria-checked')]))
      .toEqual([['Gói Nữ tần trọng sinh báo thù, vả mặt', 'false'], ['Mọi gói', 'true']])
    expect(card(el, LXH).textContent).toContain('Từ góp ý của bạn')
    for (const label of ['Bỏ', 'Sửa câu chữ', 'Giữ']) expect(button(l1, label)).toBeTruthy()
  })

  it('Giữ với “Mọi gói” gửi scope, thẻ ở lại với dòng đã giữ và Hoàn tác', async () => {
    const expectedIds = mockLessons('pending').lessons.map((l) => l.id)
    const el = await mount('pending')
    const l2 = card(el, L13)
    await act(async () => { (l2.querySelectorAll<HTMLElement>('[role=radio]')[1]).click() })
    await act(async () => { button(l2, 'Giữ').click() })
    expect(calls).toEqual([{ id: L13, input: { action: 'keep', scope: 'all' } }])
    const kept = card(el, L13)
    expect(kept.textContent).toContain('Đã giữ · thành quy tắc cấp Mọi truyện, áp dụng từ truyện mới tiếp theo')
    expect(kept.className).toContain('rise')
    // Thứ tự thẻ giữ nguyên như danh sách pending ban đầu.
    expect([...el.querySelectorAll('article')].map((a) => a.getAttribute('data-lesson'))).toEqual(expectedIds)
    await act(async () => { button(kept, 'Hoàn tác').click() })
    expect(calls.at(-1)).toEqual({ id: L13, input: { action: 'undo' } })
    expect(button(card(el, L13), 'Giữ')).toBeTruthy()
  })

  it('Bỏ không ghi lý do; sửa câu chữ lưu trước rồi mới giữ', async () => {
    const el = await mount('pending')
    await act(async () => { button(card(el, L7), 'Bỏ').click() })
    // Câu 8: bỏ lý do khỏi dòng “Đã bỏ”; “Noverse không đề xuất lại” là điều hệ thống làm, không phải lý do.
    expect(card(el, L7).textContent).toContain('Đã bỏ · Noverse không đề xuất lại bài học này')
    await act(async () => { button(card(el, L7), 'Hoàn tác').click() })

    const l3 = card(el, LXH)
    await act(async () => { button(l3, 'Sửa câu chữ').click() })
    const area = l3.querySelector<HTMLTextAreaElement>('textarea[aria-label="Sửa câu chữ bài học"]')!
    expect(area.value).toContain('Khi hai nhân vật cắt đứt quan hệ')
    expect(button(l3, 'Xong')).toBeTruthy()
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
    await act(async () => { setter.call(area, 'Đổi cách gọi ngay trong cảnh cắt đứt.'); area.dispatchEvent(new Event('input', { bubbles: true })) })
    calls.length = 0
    await act(async () => { button(l3, 'Giữ').click() })
    expect(calls).toEqual([
      { id: LXH, input: { action: 'edit', text: 'Đổi cách gọi ngay trong cảnh cắt đứt.' } },
      { id: LXH, input: { action: 'keep', scope: 'all' } },
    ])
    expect(card(el, LXH).textContent).toContain('Đổi cách gọi ngay trong cảnh cắt đứt.')
    await act(async () => { button(card(el, LXH), 'Hoàn tác').click() })
  })

  it('tab Đã giữ / Đã bỏ ghi ngày quyết và cấp quy tắc', async () => {
    const kept = await mount('kept')
    // Quyết định giữ/bỏ được seed trong lessons.json của fixture với ngày cố định.
    expect(card(kept, K4).textContent).toContain('Giữ ngày 20/10 · quy tắc của gói Ngược hôn nhân, ly hôn, hối hận muộn')
    expect(card(kept, K10).textContent).toContain('Giữ ngày 21/10 · quy tắc của gói Ngược hôn nhân, ly hôn, hối hận muộn')
    expect(card(kept, K12).textContent).toContain('Giữ ngày 12/10 · quy tắc của gói Nữ tần trọng sinh báo thù, vả mặt')
    expect(card(kept, K4).textContent).not.toContain('mới ·')
    await act(() => root!.unmount()); container!.remove(); root = null
    const dropped = await mount('dropped')
    // Câu 8: chỉ ngày + điều hệ thống làm; không có lý do bỏ.
    expect(card(dropped, D15).textContent).toContain('Bỏ ngày 19/10 · Noverse không đề xuất lại')
  })
})

describe('Não hệ thống — ARIA (Câu 14)', () => {
  it('ba tab bài học là tablist/tab/tabpanel; chọn gói là radiogroup', async () => {
    const el = await mountScreen()
    const tablist = el.querySelector<HTMLElement>('[role=tablist][aria-label="Bài học"]')!
    expect(tablist).toBeTruthy()
    expect([...tablist.querySelectorAll<HTMLElement>('[role=tab]')].map((tab) => [
      tab.id, tab.getAttribute('aria-selected'), tab.getAttribute('aria-controls'),
    ])).toEqual([
      ['sysbrain-tab-pending', 'true', 'sysbrain-lessons'],
      ['sysbrain-tab-kept', 'false', 'sysbrain-lessons'],
      ['sysbrain-tab-dropped', 'false', 'sysbrain-lessons'],
    ])
    const panel = el.querySelector<HTMLElement>('[role=tabpanel]')!
    expect(panel.id).toBe('sysbrain-lessons')
    expect(panel.getAttribute('aria-labelledby')).toBe('sysbrain-tab-pending')

    // Lọc dữ liệu (gói) là radiogroup, không phải tab.
    const packs = el.querySelector<HTMLElement>('[role=radiogroup][aria-label="Gói"]')!
    expect(packs.querySelectorAll('[role=tab]')).toHaveLength(0)
    expect([...packs.querySelectorAll<HTMLElement>('[role=radio]')].map((radio) => [radio.textContent, radio.getAttribute('aria-checked')]))
      .toEqual([['Ngược hôn nhân, ly hôn, hối hận muộn4', 'false'], ['Niên đại văn, trọng sinh thập niên, làm giàu2', 'false'], ['Nữ tần trọng sinh báo thù, vả mặt6', 'true']])
  })
})

describe('Não hệ thống — §J Câu 2, 3, 4 (tab × gói)', () => {
  // Dòng nguồn từng tab theo artboard; gói không đổi chữ của bài học.
  const SOURCES: Record<string, string[]> = {
    pending: [
      'Rút từ 3 lần bạn sửa xưng hô ở 2 truyện · 2 gói', // Xưng hô — bài của người dùng (giữ danh từ người viết ghi)
      'Rút từ 9 Cảnh báo ở 6 truyện · 3 gói',      // Quy tắc của bạn
      'Rút từ 3 Cảnh báo ở 2 truyện · gói Nữ tần trọng sinh báo thù, vả mặt', // Bộ A §4.3
      'Rút từ 3 lần Bắt sửa và 1 Cảnh báo ở 4 truyện · 2 gói', // Xưng hô — đề xuất từ vi phạm lưu trữ (§M: tách mức)
      'Rút từ 3 lần Bắt sửa ở 3 truyện · 1 gói',   // Master §11
      'Rút từ 6 Cảnh báo ở 5 truyện · 2 gói',      // Master §13
      'Rút từ 2 lần Bắt sửa ở 2 truyện · 1 gói',   // Master §6
      'Rút từ 6 lần Bắt sửa ở 5 truyện · 2 gói',   // Master §7
      'Rút từ 2 lần Bắt sửa ở 2 truyện · 2 gói',   // Master §8
    ],
    kept: [
      'Rút từ 2 lần bạn sửa kết chương ở 2 truyện · gói Nữ tần trọng sinh báo thù, vả mặt', // §12
      'Rút từ 3 lần Bắt sửa ở 2 truyện · 1 gói',   // Master §4
      'Rút từ 3 lần Bắt sửa ở 3 truyện · 1 gói',          // §10 — Master §… là mục dùng chung
    ],
    dropped: ['Rút từ 6 Cảnh báo ở 4 truyện · 2 gói'],
  }

  it('tab × gói: dòng nguồn khớp artboard và giống nhau ở cả ba gói', async () => {
    const packs = ['nu-tan-bao-thu', 'nguoc-hon-nhan', 'nien-dai-van']
    for (const [tab, expected] of Object.entries(SOURCES)) {
      for (const pack of packs) {
        const el = await mountScreen(tab, pack)
        const panel = el.querySelector<HTMLElement>('[role=tabpanel]')!
        const lines = [...panel.querySelectorAll('article')].map((article) =>
          [...article.querySelectorAll('span')].map((span) => span.textContent ?? '').find((text) => text.startsWith('Rút từ ')))
        expect([tab, pack, lines]).toEqual([tab, pack, expected])
        await unmountScreen()
      }
    }
  })

  /**
   * Số, chuông và nhãn “· mọi gói” đọc từ một hàng. `bell`/`label` = tooltip; null = không có phần tử.
   * Nhãn mang cùng tooltip với chuông (artboard: `v.allLbl` + `v.tip`).
   */
  const rowParts = (row: HTMLElement) => ({
    count: [...row.querySelectorAll('span')].map((span) => span.textContent ?? '').find((text) => /^\d+\/\d+$/.test(text))!,
    bell: row.querySelector<HTMLElement>('[role=img]')?.getAttribute('aria-label') ?? null,
    label: [...row.querySelectorAll<HTMLElement>('[title]')].find((node) => node.textContent === '· mọi gói')?.getAttribute('title') ?? null,
  })

  // Bảng mục luật hay vấp của artboard: [mục, số trong gói, tooltip của chuông/nhãn — null = không chuông].
  // Gói A: chuông §7 (6 truyện mọi gói), §11 và Xưng hô (3); §5 chỉ 2 truyện nên không chuông.
  // Gói B: chuông §10 (3/4) và §7 (1/4 nhưng 6 truyện ở mọi gói). Gói C: không chuông.
  const VIOL: Record<string, [string, string, string | null][]> = {
    'nu-tan-bao-thu': [
      ['Master §13', '4/6', null],
      ['Master §7', '4/6', 'Bắt sửa ở 5 truyện, tính trên mọi gói · được nhắc trước khi viết truyện mới'],
      ['Master §11', '3/6', 'Bắt sửa ở 3 truyện, tính trên mọi gói · được nhắc trước khi viết truyện mới'],
      ['Master §15', '3/6', null],
      ['Xưng hô', '3/6', 'Bắt sửa ở 3 truyện, tính trên mọi gói · được nhắc trước khi viết truyện mới'],
      ['Bộ A §4.3', '2/6', null],
      ['Master §8', '1/6', null],
    ],
    'nguoc-hon-nhan': [
      ['Master §10', '3/4', 'Bắt sửa ở 3 truyện, tính trên mọi gói · được nhắc trước khi viết truyện mới'],
      ['Master §4', '2/4', null],
      ['Master §13', '1/4', null],
      ['Master §7', '1/4', 'Bắt sửa ở 5 truyện, tính trên mọi gói · được nhắc trước khi viết truyện mới'],
      ['Master §8', '1/4', null],
      ['Xưng hô', '1/4', 'Bắt sửa ở 3 truyện, tính trên mọi gói · được nhắc trước khi viết truyện mới'],
    ],
    'nien-dai-van': [['Master §6', '2/2', null], ['Master §15', '1/2', null], ['Master §9', '1/2', null]],
  }

  it('9 trạng thái tab × gói: số, chuông, nhãn “· mọi gói”, tooltip và legend khớp artboard', async () => {
    for (const tab of ['pending', 'kept', 'dropped']) {
      for (const [pack, expected] of Object.entries(VIOL)) {
        const el = await mountScreen(tab, pack)
        // Hàng mục luật = các div con có chip mục (chip mang `title` = ref); bỏ đầu mục và legend.
        const rows = [...el.querySelectorAll<HTMLElement>('section[aria-label="Mục luật hay vi phạm theo gói"] > div')]
          .filter((row) => row.querySelector('[title]') && row.getAttribute('role') !== 'radiogroup')
        expect([tab, pack, rows.map((row) => { const { count, bell } = rowParts(row); return [count, bell] })]).toEqual([
          tab, pack, expected.map(([ref, count, tip]) => [count, tip]),
        ])
        // §J Câu 2: “· mọi gói” chỉ đứng ở mục dùng chung ĐANG bật chuông, và mang đúng tooltip của chuông.
        expect([tab, pack, rows.map((row) => { const { bell, label } = rowParts(row); return [bell !== null, label === bell] })])
          .toEqual([tab, pack, expected.map(([, , tip]) => [tip !== null, true])])
        expect(el.querySelector('section[aria-label="Mục luật hay vi phạm theo gói"]')!.textContent)
          .toContain('· mọi gói: mục dùng chung, chuông tính trên cả 3 gói')
        await unmountScreen()
      }
    }
  })

  it('Câu 3: lời nhắc gói B ghi “6 truyện ở mọi gói” cho §7 và giữ “3/4 truyện” cho §10', async () => {
    const el = await mountScreen('pending', 'nguoc-hon-nhan')
    expect(el.querySelector('section[aria-label="Lời nhắc trước khi viết"] p')!.textContent)
      .toBe('Truyện Ngược hôn nhân, ly hôn, hối hận muộn trước đây hay vấp Master §10 (ức chế và lòng thương (mới), 3/4 truyện), Master §7 (đầu chương và cuối chương, 5 truyện ở mọi gói), Xưng hô (bảng xưng hô của truyện, 3 truyện ở mọi gói). Khi lập bản đồ giữ chân, kiểm kỹ các mục này trước khi giao Người viết.')
    await unmountScreen()
  })

  it('Câu 2: mục Bộ … dùng tooltip n/tổng của gói; nhãn chỉ hiện khi chuông bật', () => {
    const row: ViolationRow = { ref: 'Bộ A §4.3', refName: 'Trà xanh và phản diện', stories: 3, total: 6, level: 'error', alert: true }
    expect(alertTip(row)).toBe('Bắt sửa ở 3/6 truyện của gói · được nhắc trước khi viết truyện mới')
    expect(showsScopeAll(row)).toBe(false)
    expect(showsScopeAll({ ...row, alertScope: 'all', alertStories: 4, alert: false })).toBe(false)
    expect(showsScopeAll({ ...row, alertScope: 'all', alertStories: 4 })).toBe(true)
  })
})
