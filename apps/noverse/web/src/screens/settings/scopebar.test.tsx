// @vitest-environment happy-dom
// Gói R — thanh “Áp dụng cho”: nút ▾, danh sách 360 px có tìm, chip “Đang mở”, ✓, hộp cam “chưa lưu”,
// dải “Đang xem…”, phím ↑↓/Enter/Esc.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ProjectSummary, StorySummary } from '../../api/types'
import { ScopeBar } from './scopebar'
import type { OpenTarget, ScopeTarget } from './scope/model'

vi.mock('./scope/counts', () => ({ useOwnRuleCount: () => 0 }))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const projects: ProjectSummary[] = [
  { id: 'van-chau', name: 'Vân Châu', description: '', legacy: false, storyCount: 2, runningCount: 0, updatedAt: null, ruleCount: 0, style: '' },
  { id: 'tro-ve', name: 'Trở Về', description: '', legacy: false, storyCount: 1, runningCount: 0, updatedAt: null, ruleCount: 0, style: '' },
]
const story = (project: string, id: string, title: string): StorySummary => ({
  id: `${project}/${id}`, project, story: id, folder: `kich_ban/01-${id}`, title, status: 'writing', tags: [], idea: '',
  phase: 'writing', completedChapters: 3, totalChapters: 8, totalWords: 1000, volumes: 1, open: true, running: false,
  locked: false, updatedAt: null, exportsCount: 0, tempName: false,
})
const storiesBy = {
  'van-chau': [story('van-chau', 'dam-gio', 'Đám Giỗ Cả Họ'), story('van-chau', 'khac', 'Truyện Khác')],
  'tro-ve': [story('tro-ve', 'mot', 'Truyện Một')],
} as const

const open: OpenTarget = { project: 'van-chau', story: 'dam-gio', title: 'Đám Giỗ Cả Họ' }
let root: Root | null = null
let container: HTMLElement | null = null
let calls: { target: ScopeTarget[]; save: number; discard: number } = { target: [], save: 0, discard: 0 }

async function mount(target: ScopeTarget, dirtyCount = 0, saveResult: unknown = true) {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  const render = async (next: ScopeTarget, count: number) => act(async () => {
    root!.render(<ScopeBar target={next} projectName={projects.find((p) => p.id === next.project)?.name ?? ''}
      storyTitle={(storiesBy[next.project as keyof typeof storiesBy] ?? []).find((s) => s.story === next.story)?.title ?? ''}
      storyCount={(storiesBy[next.project as keyof typeof storiesBy] ?? []).length}
      open={open} projects={projects} stories={storiesBy[next.project as keyof typeof storiesBy] ?? []}
      storiesOf={(id) => storiesBy[id as keyof typeof storiesBy] ?? []}
      dirtyCount={count}
      onTarget={(value) => { calls.target.push(value) }}
      onSave={async () => { calls.save++; return saveResult }}
      onDiscard={() => { calls.discard++ }} />)
  })
  await render(target, dirtyCount)
  return { rerender: (next: ScopeTarget, count = dirtyCount) => render(next, count) }
}

const byLabel = (label: string) => container!.querySelector<HTMLElement>(`[aria-label="${label}"]`)
const click = async (el: Element | null | undefined) => { await act(async () => { (el as HTMLElement).click() }) }
const key = async (el: Element, k: string) => {
  await act(async () => {
    el.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true }))
  })
}

beforeEach(() => { calls = { target: [], save: 0, discard: 0 } })
afterEach(async () => {
  if (root) await act(() => root!.unmount())
  container?.remove()
  root = null; container = null
})

describe('nút ▾ và danh sách', () => {
  it('ô Truyện và ô Dự án có nút ▾; “Mọi truyện” không có', async () => {
    await mount({ scope: 'story', project: 'van-chau', story: 'dam-gio' })
    expect(byLabel('Chọn truyện khác trong dự án')?.getAttribute('aria-haspopup')).toBe('listbox')
    expect(byLabel('Chọn dự án khác')?.getAttribute('aria-haspopup')).toBe('listbox')
    expect(byLabel('Mọi truyện')).toBeNull()
    const global = Array.from(container!.querySelectorAll('button')).find((b) => b.textContent?.includes('Mọi truyện'))!
    expect(global.getAttribute('aria-haspopup')).toBeNull()
  })

  it('bấm lại ô Dự án đang chọn cũng mở danh sách (không chỉ nút ▾)', async () => {
    await mount({ scope: 'project', project: 'van-chau', story: 'dam-gio' })
    const card = Array.from(container!.querySelectorAll('button')).find((b) => b.textContent?.includes('Vân Châu · 2 truyện'))!
    await click(card)
    expect(container!.textContent).toContain('2 dự án')
    await click(card)
    expect(container!.textContent).not.toContain('2 dự án')
  })

  it('bấm ô Truyện đang chọn mở danh sách truyện; ô cấp khác vẫn chỉ đổi cấp', async () => {
    await mount({ scope: 'story', project: 'van-chau', story: 'dam-gio' })
    const storyCard = Array.from(container!.querySelectorAll('button')).find((b) => b.textContent?.includes('Truyện này'))!
    await click(storyCard)
    expect(container!.textContent).toContain('Truyện trong “Vân Châu”')
    await key(byLabel('Chọn truyện khác trong dự án')!, 'Escape')
    expect(container!.textContent).not.toContain('Truyện trong “Vân Châu”')
  })

  it('ô chưa được chọn thì bấm phần chữ chỉ đổi cấp, không mở danh sách', async () => {
    await mount({ scope: 'project', project: 'van-chau', story: 'dam-gio' })
    const storyCard = Array.from(container!.querySelectorAll('button')).find((b) => b.textContent?.includes('Truyện này'))!
    await click(storyCard)
    expect(container!.textContent).not.toContain('Truyện trong “Vân Châu”')
    expect(calls.target).toEqual([{ scope: 'story', project: 'van-chau', story: 'dam-gio' }])
  })

  it('danh sách dự án có tiêu đề, số dự án, ô tìm, chip “Đang mở” và ✓ cho dự án đang xem', async () => {
    await mount({ scope: 'project', project: 'tro-ve', story: 'mot' })
    await click(byLabel('Chọn dự án khác'))
    expect(container!.textContent).toContain('Chọn dự án')
    expect(container!.textContent).toContain('2 dự án')
    expect(byLabel('Tìm dự án')).toBeTruthy()
    expect(container!.textContent).toContain('Đang mở')
    const rows = container!.querySelectorAll('[role=option]')
    expect(rows).toHaveLength(2)
    expect(rows[0].getAttribute('aria-selected')).toBe('false')
    expect(rows[1].getAttribute('aria-selected')).toBe('true')
    // Dòng phụ của dự án đang xem: “1 truyện · theo “Mọi truyện”” (chưa có luật riêng).
    expect(rows[1].textContent).toContain('1 truyện · theo “Mọi truyện”')
  })

  it('gõ tìm lọc dòng dự án', async () => {
    await mount({ scope: 'story', project: 'van-chau', story: 'dam-gio' })
    await click(byLabel('Chọn dự án khác'))
    const search = byLabel('Tìm dự án') as HTMLInputElement
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
      setter.call(search, 'trở')
      search.dispatchEvent(new Event('input', { bubbles: true }))
    })
    expect(container!.querySelectorAll('[role=option]')).toHaveLength(1)
  })

  it('danh sách truyện của dự án khác: chọn dự án rồi mở ô Truyện', async () => {
    const view = await mount({ scope: 'project', project: 'van-chau', story: 'dam-gio' })
    await click(byLabel('Chọn dự án khác'))
    await click(container!.querySelectorAll('[role=option]')[1])
    expect(calls.target.at(-1)).toEqual({ scope: 'project', project: 'tro-ve', story: 'mot' })
    await view.rerender({ scope: 'project', project: 'tro-ve', story: 'mot' })
    await click(byLabel('Chọn truyện khác trong dự án'))
    expect(container!.textContent).toContain('Truyện trong “Trở Về”')
    expect(container!.querySelectorAll('[role=option]')).toHaveLength(1)
    expect(container!.textContent).toContain('Truyện Một')
  })

  it('dự án chưa có truyện: ô Truyện mờ và danh sách ghi “Dự án này chưa có truyện.”', async () => {
    await mount({ scope: 'project', project: 'trong', story: '' })
    const storyButton = Array.from(container!.querySelectorAll('button')).find((b) => b.textContent?.includes('Truyện'))!
    expect(storyButton.hasAttribute('disabled')).toBe(true)
    await click(byLabel('Chọn truyện khác trong dự án'))
    expect(container!.textContent).toContain('Dự án này chưa có truyện.')
  })

  it('phím ↑↓ chọn dòng, Enter nhận, Esc đóng', async () => {
    await mount({ scope: 'story', project: 'van-chau', story: 'dam-gio' })
    await click(byLabel('Chọn dự án khác'))
    const dialog = container!.querySelector('[role=dialog]')!
    await key(dialog, 'ArrowDown')
    expect(container!.querySelectorAll('[role=option]')[1].getAttribute('data-hi')).toBe('true')
    await key(dialog, 'Enter')
    expect(calls.target.at(-1)).toEqual({ scope: 'project', project: 'tro-ve', story: 'mot' })
    expect(container!.querySelector('[role=dialog]')).toBeNull()

    await click(byLabel('Chọn dự án khác'))
    expect(container!.querySelector('[role=dialog]')).toBeTruthy()
    await key(container!.querySelector('[role=dialog]')!, 'Escape')
    expect(container!.querySelector('[role=dialog]')).toBeNull()
  })

  it('bấm ra ngoài thì đóng danh sách', async () => {
    await mount({ scope: 'story', project: 'van-chau', story: 'dam-gio' })
    await click(byLabel('Chọn truyện khác trong dự án'))
    expect(container!.querySelector('[role=listbox]')).toBeTruthy()
    await act(async () => { document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true })) })
    expect(container!.querySelector('[role=listbox]')).toBeNull()
  })
})

describe('hộp cam “chưa lưu”', () => {
  const dirty = 2
  it('hiện trong danh sách khi đổi chỗ xem, đủ 3 nút', async () => {
    await mount({ scope: 'story', project: 'van-chau', story: 'dam-gio' }, dirty)
    await click(byLabel('Chọn dự án khác'))
    await click(container!.querySelectorAll('[role=option]')[1])
    expect(container!.textContent).toContain('Có 2 thay đổi chưa lưu cho dự án “Trở Về”. Lưu trước khi chuyển?')
    const actions = Array.from(container!.querySelectorAll('[data-scope-action]')).map((b) => b.textContent)
    expect(actions).toEqual(['Lưu rồi chuyển', 'Bỏ thay đổi', 'Ở lại'])
    expect(calls.target).toHaveLength(0)
  })

  it('“Ở lại” không đổi chỗ xem, danh sách vẫn mở', async () => {
    await mount({ scope: 'story', project: 'van-chau', story: 'dam-gio' }, dirty)
    await click(byLabel('Chọn dự án khác'))
    await click(container!.querySelectorAll('[role=option]')[1])
    await click(container!.querySelector('[data-scope-action=stay]'))
    expect(container!.textContent).not.toContain('thay đổi chưa lưu')
    expect(container!.querySelector('[role=dialog]')).toBeTruthy()
    expect(calls.target).toHaveLength(0)
  })

  it('“Bỏ thay đổi” huỷ bản nháp rồi chuyển', async () => {
    await mount({ scope: 'story', project: 'van-chau', story: 'dam-gio' }, dirty)
    await click(byLabel('Chọn dự án khác'))
    await click(container!.querySelectorAll('[role=option]')[1])
    await click(container!.querySelector('[data-scope-action=discard]'))
    expect(calls.discard).toBe(1)
    expect(calls.target.at(-1)).toEqual({ scope: 'project', project: 'tro-ve', story: 'mot' })
    expect(container!.querySelector('[role=dialog]')).toBeNull()
  })

  it('“Lưu rồi chuyển” chờ lưu xong mới chuyển', async () => {
    await mount({ scope: 'story', project: 'van-chau', story: 'dam-gio' }, dirty, true)
    await click(byLabel('Chọn dự án khác'))
    await click(container!.querySelectorAll('[role=option]')[1])
    await click(container!.querySelector('[data-scope-action=save]'))
    expect(calls.save).toBe(1)
    expect(calls.target.at(-1)).toEqual({ scope: 'project', project: 'tro-ve', story: 'mot' })
    expect(container!.querySelector('[role=dialog]')).toBeNull()
  })

  it('lưu hỏng thì ở lại danh sách, không chuyển', async () => {
    await mount({ scope: 'story', project: 'van-chau', story: 'dam-gio' }, dirty, false)
    await click(byLabel('Chọn dự án khác'))
    await click(container!.querySelectorAll('[role=option]')[1])
    await click(container!.querySelector('[data-scope-action=save]'))
    expect(calls.save).toBe(1)
    expect(calls.target).toHaveLength(0)
    expect(container!.querySelector('[data-scope-action=save]')).toBeTruthy()
  })

  it('chọn “Mọi truyện” khi còn thay đổi cũng hỏi trước', async () => {
    await mount({ scope: 'story', project: 'van-chau', story: 'dam-gio' }, dirty)
    const global = Array.from(container!.querySelectorAll('button')).find((b) => b.textContent?.includes('Mọi truyện'))!
    await click(global)
    expect(container!.textContent).toContain('cho “Mọi truyện”')
    expect(container!.querySelector('[data-scope-action=stay]')).toBeTruthy()
  })
})

describe('dải “Đang xem…”', () => {
  it('hiện khi xem dự án khác và nút quay về chọn truyện đang mở', async () => {
    await mount({ scope: 'project', project: 'tro-ve', story: '' })
    expect(container!.textContent).toContain('Đang xem quy tắc của dự án “Trở Về”. Truyện đang mở ở Bàn viết vẫn là “Đám Giỗ Cả Họ”.')
    const back = Array.from(container!.querySelectorAll('button')).find((b) => b.textContent === 'Quay về truyện đang mở')!
    await click(back)
    expect(calls.target.at(-1)).toEqual({ scope: 'story', project: 'van-chau', story: 'dam-gio' })
  })

  it('không hiện khi đang xem truyện đang mở', async () => {
    await mount({ scope: 'story', project: 'van-chau', story: 'dam-gio' })
    expect(container!.textContent).not.toContain('Đang xem quy tắc')
  })

  it('danh sách có dòng “Quay về truyện đang mở · …” khi xem chỗ khác', async () => {
    await mount({ scope: 'story', project: 'tro-ve', story: 'mot' })
    await click(byLabel('Chọn dự án khác'))
    expect(container!.textContent).toContain('Quay về truyện đang mở · Đám Giỗ Cả Họ')
  })
})
