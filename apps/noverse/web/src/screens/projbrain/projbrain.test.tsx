// @vitest-environment happy-dom
// Não dự án: tablist ARIA, 4 phép kiểm luôn hiện, ẩn cột Ngày 7 khi chưa có nguồn, dòng tên vũ trụ.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ProjectLedger, ProjectUniverse, UniverseCheckItem, UniverseKeptItem } from '../../api/types'
import type * as Client from '../../api/client'
import { mockLedger, mockLedgerCheck, mockProjectSearch, mockUniverse, mockUnkeep, MOCK_LEDGER_FIXED_BRIEF } from '../../../mock/projbrain'
import { Highlighted, ProjectBrainScreen, RecapCard, UniverseCheck, UniverseSummary } from './index'

vi.mock('../../api/session', () => ({
  useSession: () => ({ checked: true, user: { username: 'dat', name: 'Dat', role: 'admin', projects: {} }, setPendingLessons: () => {} }),
}))

const ledgers: Record<string, ProjectLedger> = {}
const universes: Record<string, ProjectUniverse> = {}
const unkeepCalls: [string, string, string][] = []
vi.mock('../../api/client', async (original) => ({
  ...await original<typeof Client>(),
  getProjectLedger: async (project: string) => ledgers[project],
  getProjectUniverse: async (project: string) => universes[project],
  searchProjectBrain: async () => mockProjectSearch('kenh', 'ghi am', () => true),
  unkeepReviewItem: async (project: string, story: string, itemId: string) => { unkeepCalls.push([project, story, itemId]) },
}))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | null = null
let container: HTMLElement | null = null
const canView = (id: string) => !id.endsWith('/hidden-story')

beforeEach(() => {
  unkeepCalls.length = 0
  for (const project of Object.keys(ledgers)) delete ledgers[project]
  for (const project of Object.keys(universes)) delete universes[project]
  ledgers.kenh = mockLedger('kenh', canView)
  universes.kenh = mockUniverse('kenh', canView, 'Thành phố Vân Châu')
})
afterEach(async () => {
  if (root) await act(() => root!.unmount())
  container?.remove()
  root = null; container = null
  location.hash = ''
})

async function mount(hash: string, project = 'kenh') {
  // Mount lần hai trong cùng một test (đổi dữ liệu) — dọn root cũ để không rò rỉ cây React.
  if (root) await act(() => root!.unmount())
  container?.remove()
  location.hash = hash
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  await act(async () => { root!.render(<ProjectBrainScreen project={project} />) })
  await act(async () => { await Promise.resolve() })
  return container
}

describe('Não dự án — sổ cái', () => {
  it('opens the matching stories from a phone frequency value', async () => {
    const original = window.matchMedia
    window.matchMedia = vi.fn(query => ({
      matches: query === '(max-width: 759px)', media: query, onchange: null,
      addListener: vi.fn(), removeListener: vi.fn(), addEventListener: vi.fn(), removeEventListener: vi.fn(), dispatchEvent: vi.fn(),
    }))
    try {
      const el = await mount('#/p/kenh/brain?tab=ledger')
      const value = [...el.querySelectorAll('button')].find(button => button.textContent?.startsWith('Thợ may'))
      expect(value).toBeDefined()
      await act(() => value!.click())
      const dialog = document.querySelector('[role=dialog]')!
      expect(dialog.textContent).toContain(ledgers.kenh.candidate!.title)
      for (const story of ledgers.kenh.stories.filter(story => story.values[2] === 'Thợ may').slice(0, 10)) {
        expect(dialog.textContent).toContain(story.title)
      }
      expect(el.querySelector('[data-story]')).toBeNull()
    } finally {
      window.matchMedia = original
    }
  })

  it('does not expose a hidden story in ledger, universe or search', () => {
    const data = mockLedger('kenh', canView)
    const uni = mockUniverse('kenh', canView)
    const search = mockProjectSearch('kenh', 'ghi am', canView)
    expect(data.stories.some(story => story.id.endsWith('/hidden-story'))).toBe(false)
    expect(uni.entities.some(entity => entity.stories.some(story => story.id === 'hidden-story'))).toBe(false)
    expect(search.passages.some(hit => hit.story === 'hidden-story')).toBe(false)
    expect(JSON.stringify({ uni, search })).not.toContain('Nhân vật bí mật')
    expect(search.passages.length).toBe(4)
  })

  it('renders all four checks — including the passing ones as “Đạt”', async () => {
    expect(mockLedgerCheck('kenh', 'brief').checks.map(check => check.rule)).toEqual(['combination', 'distance', 'overuse', 'opening'])
    const el = await mount('#/p/kenh/brain?tab=ledger')
    const rows = [...el.querySelectorAll<HTMLElement>('[data-ledger-check]')]
    expect(rows.map(row => row.dataset.ledgerCheck)).toEqual(['combination', 'distance', 'overuse', 'opening'])
    // Câu 1: phép đạt cũng hiện, ghi “Đạt”. Mock: combination Bắt sửa, distance + opening Đạt, overuse Cảnh báo.
    expect(rows.map(row => row.children[1]?.textContent)).toEqual(['Bắt sửa', 'Đạt', 'Cảnh báo', 'Đạt'])
  })

  it('switches ProjBrain sections with tablist/tab/tabpanel semantics', async () => {
    const el = await mount('#/p/kenh/brain?tab=ledger')
    const list = el.querySelector<HTMLElement>('[role=tablist]')!
    expect(list.getAttribute('aria-label')).toBe('Phần')
    const tabs = [...list.querySelectorAll<HTMLElement>('[role=tab]')]
    expect(tabs.map(tab => tab.id)).toEqual(['projbrain-tab-ledger', 'projbrain-tab-universe', 'projbrain-tab-search'])
    expect(tabs.map(tab => tab.getAttribute('aria-selected'))).toEqual(['true', 'false', 'false'])
    expect(tabs.every(tab => tab.getAttribute('aria-controls') === 'projbrain-body')).toBe(true)
    expect(list.querySelectorAll('[role=radio]').length).toBe(0)
    const panel = el.querySelector<HTMLElement>('[role=tabpanel]')!
    expect(panel.id).toBe('projbrain-body')
    expect(panel.getAttribute('aria-labelledby')).toBe('projbrain-tab-ledger')
  })

  it('hides the Ngày 7 column and the median footnote while every row has no data', async () => {
    expect(mockLedger('kenh', canView).stories.every(story => story.day7 === null)).toBe(true)
    const el = await mount('#/p/kenh/brain?tab=ledger')
    expect(el.textContent).not.toContain('Ngày 7')
    expect(el.textContent).not.toContain('trung vị kênh')
    // 1 cột tên + 10 trục, không còn ô Ngày 7.
    expect(el.querySelector<HTMLElement>('[data-story]')!.children.length).toBe(11)

    // AP removes the column even if an older API happens to send a measurement.
    const empty = mockLedger('kenh2', canView)
    ledgers.kenh2 = { ...empty, stories: empty.stories.map((story, index) => index === 0 ? { ...story, day7: 31_400 } : story), channelMedianDay7: 18_200 }
    const withData = await mount('#/p/kenh2/brain?tab=ledger', 'kenh2')
    expect(withData.textContent).not.toContain('Ngày 7')
    expect(withData.textContent).not.toContain('trung vị kênh')
    expect(withData.querySelector<HTMLElement>('[data-story]')!.children.length).toBe(11)
  })

  it('dòng phụ dưới tên truyện: ngày thêm từ createdAt, truyện mới ghi “Truyện mới · brief”', async () => {
    const el = await mount('#/p/kenh/brain?tab=ledger')
    const cell = (id: string) => el.querySelector<HTMLElement>(`[data-story="${id}"]`)!
    // Artboard: dòng phụ 11,5 px; truyện mới tím nhạt, truyện đã có xám.
    expect(cell('kenh/02-chua-co-ten').textContent).toContain('Truyện mới · brief')
    expect(cell('kenh/thien-kim').textContent).toContain('24/10')
    // Truyện chưa có story.json (createdAt null) → chỉ tiêu đề, không có dòng ngày.
    const undated = cell('kenh/01-story')
    expect(undated.textContent).not.toMatch(/\d\d\/\d\d/)
  })

  it('ô trục cắt 2 dòng ở lớp chữ bên trong ô có đệm (dòng 3 không lọt vào đệm dưới)', async () => {
    const el = await mount('#/p/kenh/brain?tab=ledger')
    const values = mockLedger('kenh', canView).stories.find(story => story.id === 'kenh/thien-kim')!.values
    const cell = el.querySelector<HTMLElement>('[data-story="kenh/thien-kim"]')!.children[1] as HTMLElement
    // Clamp nằm trên phần chữ, không trên khung đệm: `-webkit-line-clamp` trên khung
    // có padding dọc để Chromium vẽ dòng thứ 3 vào đệm (lỗi 26/10).
    const text = cell.querySelector<HTMLElement>('[data-cell-text]')
    expect(text).not.toBeNull()
    expect(text!.textContent).toBe(values[0])
  })

  it('slices UTF-16 highlights without losing Vietnamese or emoji', () => {
    const quote = '🎧 Bản ghi âm gốc'
    const from = quote.indexOf('ghi âm')
    const html = renderToStaticMarkup(<Highlighted quote={quote} ranges={[[from, from + 'ghi âm'.length]]} />)
    expect(html).toContain('🎧 Bản ')
    expect(html).toMatch(/<mark[^>]*>ghi âm<\/mark> gốc/)
  })

  it('inspects all matching cells, counts only prior stories, switches frequency and filters shared rows', async () => {
    const el = await mount('#/p/kenh/brain?tab=ledger')
    const cells = [...el.querySelectorAll<HTMLButtonElement>('[data-story] button')]
    const tailor = cells.filter(cell => cell.querySelector('[data-cell-text]')?.firstChild?.textContent === 'Thợ may')
    expect(tailor).toHaveLength(3)
    await act(async () => { tailor[0].click() })
    expect(el.querySelectorAll('[data-picked=true]')).toHaveLength(3)
    expect(el.textContent).toContain('dùng 2 lần trong 10 truyện gần nhất')
    await act(async () => { [...el.querySelectorAll<HTMLButtonElement>('button')].find(button => button.textContent === 'Bỏ chọn')!.click() })
    expect(el.querySelectorAll('[data-picked=true]')).toHaveLength(0)
    const before = el.querySelectorAll('[data-story]').length
    await act(async () => { [...el.querySelectorAll<HTMLButtonElement>('button')].find(button => button.textContent === 'Chỉ truyện có ô trùng truyện mới')!.click() })
    expect(el.querySelectorAll('[data-story]').length).toBeLessThan(before)
    await act(async () => { [...el.querySelectorAll<HTMLButtonElement>('[role=radio]')].find(button => button.textContent === 'Tần suất theo trục')!.click() })
    const frequencies = el.querySelector<HTMLElement>('section[aria-label="Tần suất theo trục"]')!
    expect(frequencies.children).toHaveLength(10)
    expect([...frequencies.children[2].querySelectorAll('span')].some(span => span.textContent === 'Thợ may2')).toBe(true)
  })

  it('filters universe by entity type, needs-review, name and handles unchecked results', async () => {
    const el = await mount('#/p/kenh/brain?tab=universe')
    expect(el.querySelectorAll('[data-entity]')).toHaveLength(6)
    await act(async () => { [...el.querySelectorAll<HTMLButtonElement>('[role=radio]')].find(button => button.textContent?.startsWith('Địa điểm'))!.click() })
    expect([...el.querySelectorAll<HTMLElement>('[data-entity]')].every(row => row.dataset.entity?.startsWith('place:'))).toBe(true)
    await act(async () => { [...el.querySelectorAll<HTMLButtonElement>('[role=radio]')].find(button => button.textContent?.startsWith('Tất cả'))!.click(); [...el.querySelectorAll<HTMLButtonElement>('button')].find(button => button.textContent?.startsWith('Chỉ mục cần xem'))!.click() })
    expect(el.querySelectorAll('[data-entity]')).toHaveLength(2)
    expect(el.querySelector('[data-entity]')?.textContent).toContain('Bà Tư bán xôi')
    universes.unchecked = { ...mockUniverse('unchecked', canView), checks: null, kept: [], keptCount: 0 }
    ledgers.unchecked = mockLedger('unchecked', canView)
    const unknown = await mount('#/p/unchecked/brain?tab=universe', 'unchecked')
    expect(unknown.querySelectorAll('[data-entity]')).toHaveLength(6)
    expect(unknown.textContent).toContain('Chưa có kết quả kiểm')
    expect(unknown.textContent).not.toContain('Chỉ mục cần xem')
    expect([...unknown.querySelectorAll('[data-entity]')].every(row => row.textContent?.includes('Chưa kiểm'))).toBe(true)
  })

  it('filters returned search results by kind and story without another request', async () => {
    const el = await mount('#/p/kenh/brain?tab=search&q=ghi%20am')
    const passages = () => el.querySelectorAll('section[aria-label="Đoạn văn khớp"] a')
    expect(passages()).toHaveLength(5)
    await act(async () => { [...el.querySelectorAll<HTMLButtonElement>('button')].find(button => button.title === 'Mẹ Chồng Đuổi Tôi Đi')!.click() })
    expect(passages()).toHaveLength(1)
    await act(async () => { [...el.querySelectorAll<HTMLButtonElement>('[role=radio]')].find(button => button.textContent?.startsWith('Thực thể'))!.click() })
    expect(el.querySelector('section[aria-label="Đoạn văn khớp"]')).toBeNull()
    expect(el.querySelector('section[aria-label="Thực thể khớp"]')).not.toBeNull()
    await act(async () => { [...el.querySelectorAll<HTMLButtonElement>('[role=radio]')].find(button => button.textContent?.startsWith('Đoạn văn'))!.click() })
    expect(el.querySelector('section[aria-label="Thực thể khớp"]')).toBeNull()
    expect(passages()).toHaveLength(1)
  })
})

describe('Não dự án — vũ trụ chung', () => {
  it('Kiểm: chưa chấm được → “—”, đã chấm mà sạch → Đạt, có vi phạm → Nhắc dài', () => {
    // Ba truyện có vòng chấm `rule_aware` trong mock (01-tra-xanh, dam-gio) và vòng sạch (me-chong).
    const unknown = mockUniverse('kenh', id => !id.endsWith('/dam-gio') && !id.endsWith('/me-chong') && !id.endsWith('/ganh-xoi') && !id.endsWith('/ban-than') && !id.endsWith('/01-tra-xanh-cuop-kich-ban'))
    expect(unknown.checks).toBeNull()
    const unknownHtml = renderToStaticMarkup(<UniverseCheck checks={unknown.checks} stories={unknown.entities[0].stories} />)
    expect(unknownHtml).toContain('Chưa kiểm')

    const clean = mockUniverse('kenh', id => !id.endsWith('/dam-gio') && !id.endsWith('/01-tra-xanh-cuop-kich-ban'))
    expect(clean.checks).toEqual([])
    expect(renderToStaticMarkup(<UniverseCheck checks={clean.checks} stories={clean.entities[0].stories} />)).toContain('Đạt')

    const violated = mockUniverse('kenh', canView)
    expect(violated.checks?.map(check => check.quoteId)).toEqual(['ch6-45b2123c', 'ch2-e9502bef'])
    const entity = violated.entities.find(item => item.id === 'char:ba-tu-ban-xoi')!
    expect(renderToStaticMarkup(<UniverseCheck checks={violated.checks} stories={entity.stories} />)).toContain('Nhắc dài')
    const unrelated = violated.entities.find(item => item.id === 'char:co-lao-cong-lua')!
    expect(renderToStaticMarkup(<UniverseCheck checks={violated.checks} stories={unrelated.stories} />)).toContain('Đạt')
  })

  it('shows the name row only when the API sends a name (§H câu 3)', () => {
    const spanCount = (html: string) => (html.match(/<span/g) ?? []).length
    const named = renderToStaticMarkup(<UniverseSummary name="Thành phố Vân Châu" counts={{ stories: 7, chars: 9, places: 4 }} />)
    expect(named).toContain('Thành phố Vân Châu')
    const anonymous = renderToStaticMarkup(<UniverseSummary name="" counts={{ stories: 7, chars: 9, places: 4 }} />)
    // Không có chuỗi rỗng: bớt hẳn một hàng span so với bản có tên.
    expect(spanCount(anonymous)).toBe(spanCount(named) - 1)
    expect(anonymous).toContain('Vũ trụ chung của dự án: 7 truyện, 9 nhân vật và 4 địa điểm dùng lại.')
  })

  it('renders the violation card from checks[]', async () => {
    const el = await mount('#/p/kenh/brain?tab=universe')
    expect(el.textContent).toContain('Thành phố Vân Châu')
    expect(el.textContent).toContain('Nhắc chuyện cũ quá dài')
    expect(el.textContent).toContain('Mở đoạn đó')
  })

  it('đặt dòng “đã giữ nguyên” ngay dưới thẻ nhắc cuối ở cột phải, ✓ 13 px, mũi tên xoay khi mở (câu 1)', async () => {
    const uni = mockUniverse('kenh', canView)
    expect(uni.keptCount).toBe(2)
    expect(uni.kept.map(item => item.quoteId)).toEqual(['ch3-4a037216', 'ch5-944914af'])
    const el = await mount('#/p/kenh/brain?tab=universe')
    const aside = el.querySelector('aside')!
    const toggle = aside.querySelector<HTMLButtonElement>(':scope > [data-universe-kept]')!
    expect(toggle).not.toBeNull()
    // Không còn nằm ở cuối bảng thực thể bên trái.
    expect(el.querySelector('section > [data-universe-kept]')).toBeNull()
    // Ngay sau thẻ RecapCard cuối cùng (thẻ nhắc, không phải thẻ tóm tắt) và là phần tử cuối cột phải.
    const previous = toggle.previousElementSibling as HTMLElement
    expect(previous.tagName).toBe('SECTION')
    expect(previous.hasAttribute('data-kept')).toBe(false)
    expect(previous.textContent).toContain('Nhắc chuyện cũ quá dài')
    expect([...aside.children].slice([...aside.children].indexOf(toggle) + 1).some(child => child.tagName === 'SECTION')).toBe(false)
    // Nội dung: ✓ 13 px, chữ đếm, mũi tên xoay; aria-expanded theo trạng thái mở.
    expect(toggle.textContent).toBe('2 mục đã giữ nguyên')
    const tick = toggle.firstElementChild as HTMLElement
    expect(tick.style.maskImage).toContain('tick-circle.svg')
    const chevron = toggle.querySelector<HTMLElement>('[data-kept-chevron]')!.lastElementChild as HTMLElement
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
    expect(chevron.style.transform).toBe('none')
    expect(el.querySelector('#universe-kept')).toBeNull()

    await act(async () => { toggle.click() })
    expect(toggle.getAttribute('aria-expanded')).toBe('true')
    expect(chevron.style.transform).toBe('rotate(180deg)')
    const list = el.querySelector<HTMLElement>('#universe-kept')!
    expect(list.textContent).toContain(uni.kept[0].text)
    expect(list.textContent).toContain(uni.kept[1].text)

    await act(async () => { toggle.click() })
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
    expect(el.querySelector('#universe-kept')).toBeNull()
  })

  it('thẻ đã giữ dùng biến thể: ✓ xám, chip, dòng lý do, “Mở đoạn đó ›” và “Bỏ giữ” (câu 1)', async () => {
    const el = await mount('#/p/kenh/brain?tab=universe')
    const toggle = el.querySelector<HTMLButtonElement>('[data-universe-kept]')!
    await act(async () => { toggle.click() })
    const list = el.querySelector<HTMLElement>('#universe-kept')!
    const cards = [...list.querySelectorAll<HTMLElement>('section[data-kept]')]
    expect(cards.length).toBe(2)
    const [first] = cards
    expect(first.innerHTML).toContain('tick-circle-bulk')
    expect(first.innerHTML).not.toContain('warning-2')
    expect(first.textContent).toContain('Đã giữ nguyên')
    expect(first.textContent).toContain('Lý do: “Người nghe mới cần biết vì sao bà Tư có mặt ở đám giỗ” · Dat, 24/10')
    expect(first.querySelector('a')!.textContent).toBe('Mở đoạn đó ›')
    expect([...first.querySelectorAll('button')].map(button => button.textContent)).toEqual(['Bỏ giữ'])
    // Thẻ nhắc thường vẫn dùng ⚠ cảnh báo.
    const normal = [...el.querySelectorAll<HTMLElement>('aside > section')].find(section => !section.hasAttribute('data-kept') && section.textContent?.includes('Nhắc chuyện cũ quá dài'))!
    expect(normal.innerHTML).toContain('warning-2')
  })

  it('dòng lý do bỏ đoạn rỗng, cả ba rỗng thì không vẽ dòng (câu 1)', () => {
    const base: UniverseCheckItem = { ref: 'Master §19', story: 'ganh-xoi', ch: 3, text: 'Nhắc lại 4 câu.', quoteId: 's19-recap-ganh-xoi-3' }
    const html = (extra: Partial<UniverseKeptItem>) => renderToStaticMarkup(<RecapCard project="kenh" check={{ ...base, ...extra }} kept />)
    // Thiếu lý do → chỉ còn “Dat, 24/10”.
    const whoOnly = html({ reason: '', decidedBy: 'Dat', decidedAt: '2026-10-24T12:00:00Z' })
    expect(whoOnly).not.toContain('Lý do:')
    expect(whoOnly).toContain('>Dat, 24/10</span>')
    // Thiếu người/ngày → chỉ còn “Lý do: “…””, không có dấu chấm giữa.
    const reasonOnly = html({ reason: 'Cố ý mở chuyện cũ', decidedBy: '', decidedAt: '' })
    expect(reasonOnly).toContain('Lý do: “Cố ý mở chuyện cũ”')
    expect(reasonOnly).not.toContain(' · ')
    // Cả ba rỗng → bỏ hẳn dòng.
    expect(html({ reason: '', decidedBy: '', decidedAt: '' })).not.toContain('data-kept-reason')
  })

  it('“Bỏ giữ” gọi GAP-47 rồi tải lại vũ trụ: mục quay lại checks (câu 1)', async () => {
    universes.kenh4 = mockUniverse('kenh4', canView)
    const el = await mount('#/p/kenh4/brain?tab=universe', 'kenh4')
    const toggle = el.querySelector<HTMLButtonElement>('[data-universe-kept]')!
    await act(async () => { toggle.click() })
    const before = universes.kenh4.kept[0]
    universes.kenh4 = { ...universes.kenh4, kept: universes.kenh4.kept.slice(1), keptCount: 1, checks: [...(universes.kenh4.checks ?? []), before] }
    const drop = el.querySelector<HTMLButtonElement>('#universe-kept section[data-kept] button')!
    await act(async () => { drop.click() })
    await act(async () => { await Promise.resolve(); await Promise.resolve() })
    expect(unkeepCalls).toEqual([['kenh4', 'ganh-xoi', 'ch3-4a037216']])
    expect(el.querySelector<HTMLButtonElement>('[data-universe-kept]')!.textContent).toBe('1 mục đã giữ nguyên')
    // Mục vừa bỏ giữ hiện lại như thẻ nhắc thường trong cột phải (không phải thẻ đã giữ).
    const returned = [...el.querySelectorAll<HTMLElement>('aside > section')].filter(section => !section.hasAttribute('data-kept'))
    expect(returned.some(section => section.textContent?.includes(before.text))).toBe(true)
  })

  it('bỏ giữ mục cuối → “Không còn mục nào đã giữ nguyên”; keptCount 0 từ đầu thì ẩn hẳn dòng (câu 1)', async () => {
    const full = mockUniverse('kenh5', canView)
    universes.kenh5 = { ...full, kept: [full.kept[0]], keptCount: 1, checks: [...(full.checks ?? []), full.kept[1]] }
    const el = await mount('#/p/kenh5/brain?tab=universe', 'kenh5')
    const toggle = el.querySelector<HTMLButtonElement>('[data-universe-kept]')!
    await act(async () => { toggle.click() })
    const before = universes.kenh5.kept[0]
    universes.kenh5 = { ...universes.kenh5, kept: [], keptCount: 0, checks: [...(universes.kenh5.checks ?? []), before] }
    await act(async () => { el.querySelector<HTMLButtonElement>('#universe-kept section[data-kept] button')!.click() })
    await act(async () => { await Promise.resolve(); await Promise.resolve() })
    expect(el.querySelector<HTMLButtonElement>('[data-universe-kept]')!.textContent).toBe('Không còn mục nào đã giữ nguyên')
    expect(el.querySelector('#universe-kept')).toBeNull()

    // 0 ngay từ đầu → không có dòng.
    universes.kenh6 = { ...mockUniverse('kenh6', canView), kept: [], keptCount: 0 }
    const fresh = await mount('#/p/kenh6/brain?tab=universe', 'kenh6')
    expect(fresh.querySelector('[data-universe-kept]')).toBeNull()
    expect(fresh.textContent).not.toContain('mục đã giữ nguyên')
  })

  it('mock: kept[] mang lý do/người/ngày; bỏ giữ đưa mục về checks (câu 1)', () => {
    const uni = mockUniverse('kenh7', canView)
    expect(uni.kept[0]).toMatchObject({ reason: 'Người nghe mới cần biết vì sao bà Tư có mặt ở đám giỗ', decidedBy: 'Dat', decidedAt: '2026-10-24T12:00:00Z' })
    expect(mockUnkeep('kenh7', 'ganh-xoi', 'ch3-4a037216')).toBe(true)
    expect(mockUnkeep('kenh7', 'ganh-xoi', 'không-có')).toBe(false)
    const after = mockUniverse('kenh7', canView)
    expect(after.keptCount).toBe(1)
    expect(after.kept.map(item => item.quoteId)).toEqual(['ch5-944914af'])
    expect(after.checks?.map(check => check.quoteId)).toContain('ch3-4a037216')
  })
})

describe('Não dự án — gợi ý sửa của phép kiểm', () => {
  it('renders checks[].fix at the row end only when the server sends one (câu 3)', async () => {
    // Artboard ProjBrain: chỉ tổ hợp (Bắt sửa) và vượt 2 lần (Cảnh báo) có gợi ý; phép đạt thì không.
    expect(mockLedgerCheck('kenh', 'brief').checks.map(check => check.fix)).toEqual(['Đổi ít nhất 1 trong 4 trục', null, 'Cân nhắc đổi', null])
    const el = await mount('#/p/kenh/brain?tab=ledger')
    const rows = [...el.querySelectorAll<HTMLElement>('[data-ledger-check]')]
    expect(rows.map(row => row.querySelector('[data-check-fix]')?.textContent ?? null)).toEqual(['Đổi ít nhất 1 trong 4 trục', null, 'Cân nhắc đổi', null])
    // Căn phải: gợi ý đứng cuối hàng, sau ô chữ co giãn; hàng không có gợi ý kết thúc bằng ô chữ.
    expect(rows[0].lastElementChild?.hasAttribute('data-check-fix')).toBe(true)
    expect(rows[1].lastElementChild?.hasAttribute('data-check-fix')).toBe(false)

    // Bản đã sửa: artboard `check=fixed` chỉ còn Cảnh báo vượt 2 lần → đúng một gợi ý, tổ hợp hết lỗi.
    const fixed = mockLedgerCheck('kenh', MOCK_LEDGER_FIXED_BRIEF, canView)
    expect(fixed.checks.map(check => check.fix)).toEqual([null, null, 'Cân nhắc đổi', null])
    const base = mockLedger('kenh3', canView)
    ledgers.kenh3 = { ...base, candidate: { ...base.candidate!, checks: fixed.checks } }
    const done = await mount('#/p/kenh3/brain?tab=ledger', 'kenh3')
    const doneRows = [...done.querySelectorAll<HTMLElement>('[data-ledger-check]')]
    expect(doneRows.length).toBe(4)
    expect(doneRows.map(row => row.querySelector('[data-check-fix]')?.textContent ?? null)).toEqual([null, null, 'Cân nhắc đổi', null])
  })
})
