// @vitest-environment happy-dom
// Gói R — đổi dự án ngay trên thanh “Áp dụng cho” rồi thêm quy tắc: `apply` phải ghi `putRuleFile` vào
// dự án đang xem, không phải dự án đang mở ở Bàn viết. Không mount cả SettingsScreen.
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type * as Client from '../../../api/client'
import type { ProjectSummary, RulesEffective, StorySummary } from '../../../api/types'
import { ScopeBar } from '../scopebar'
import { useRulesEditor } from '../sections/rules/useRulesEditor'
import type { OpenTarget, ScopeTarget } from './model'

const api = vi.hoisted(() => ({
  putRuleFile: vi.fn(), understandRule: vi.fn(), applyRules: vi.fn(),
  getRuleFile: vi.fn(), getRulesEffective: vi.fn(),
}))

vi.mock('../../../api/client', async (original) => ({
  ...(await original<typeof Client>()), ...api,
  canCreateStory: () => true, canWriteStory: () => true, canReadProject: () => true,
}))
vi.mock('../../../api/session', () => ({
  useSession: () => ({ user: { username: 'dat', name: 'Dat', role: 'admin', projects: {} }, setUser: () => {}, checked: true }),
}))
vi.mock('./counts', () => ({ useOwnRuleCount: () => 0 }))

const empty: RulesEffective = { snapshot: null, items: [], runtime: [] }
vi.mock('../../../data', async (original) => ({
  ...(await original<typeof import('../../../data')>()),
  invalidate: vi.fn(async () => {}),
  setData: vi.fn(),
  useConnection: () => ({ writable: true, locked: false, online: true }),
  useResource: (key: readonly unknown[]) => key[0] === 'rules'
    ? { data: empty, error: null, status: 'ready', refreshing: false, updatedAt: null, refresh: vi.fn() }
    : { data: null, error: null, status: 'ready', refreshing: false, updatedAt: null, refresh: vi.fn() },
}))

const projects: ProjectSummary[] = [
  { id: 'van-chau', name: 'Vân Châu', description: '', legacy: false, storyCount: 1, runningCount: 0, updatedAt: null },
  { id: 'tro-ve', name: 'Trở Về', description: '', legacy: false, storyCount: 1, runningCount: 0, updatedAt: null },
]
const story = (project: string, id: string, title: string): StorySummary => ({
  id: `${project}/${id}`, project, story: id, folder: `kich_ban/01-${id}`, title, status: 'writing', tags: [], idea: '',
  phase: 'writing', completedChapters: 3, totalChapters: 8, totalWords: 1000, volumes: 1, open: true, running: false,
  locked: false, updatedAt: null, exportsCount: 0, tempName: false,
})
const open: OpenTarget = { project: 'van-chau', story: 'dam-gio', title: 'Đám Giỗ Cả Họ' }
const storiesOf = (id: string) => id === 'van-chau' ? [story('van-chau', 'dam-gio', 'Đám Giỗ Cả Họ')] : [story('tro-ve', 'mot', 'Truyện Một')]

function Harness() {
  const [view, setView] = useState<ScopeTarget>({ scope: 'story', project: 'van-chau', story: 'dam-gio' })
  const editor = useRulesEditor({ scope: view.scope, project: view.project, story: view.story })
  return <>
    <ScopeBar target={view} projectName={projects.find((p) => p.id === view.project)?.name ?? ''} storyTitle="Đám Giỗ Cả Họ" storyCount={1}
      open={open} projects={projects} stories={storiesOf(view.project)} storiesOf={storiesOf} dirtyCount={editor.count}
      onTarget={setView} onSave={async () => { await editor.apply('cấp đang xem'); return true }} onDiscard={() => editor.cancel()} />
    <button id="add" type="button" onClick={() => { void editor.add('Bỏ từ bỗng nhiên.') }}>add</button>
    <button id="apply" type="button" onClick={() => { void editor.apply('cấp đang xem') }}>apply</button>
    <span data-testid="dirty">{editor.count}</span>
  </>
}

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | null = null
let container: HTMLElement | null = null

beforeEach(() => {
  vi.clearAllMocks()
  api.understandRule.mockResolvedValue({ structured: {}, preferences: '', uncertain: [], chips: [{ kind: 'ban', text: 'bỗng nhiên' }], status: 'ready' })
  api.putRuleFile.mockResolvedValue({ name: 'quy-tac.md', size: 10, updatedAt: null })
  api.applyRules.mockResolvedValue({ snapshot: {}, effectiveFrom: 4 })
})
afterEach(async () => {
  if (root) await act(() => root!.unmount())
  container?.remove()
  root = null; container = null
})

const click = async (el: Element | null | undefined) => { await act(async () => { (el as HTMLElement).click(); await Promise.resolve() }) }

describe('đổi dự án → thêm quy tắc', () => {
  it('quy tắc ghi vào dự án đang xem, không phải dự án đang mở', async () => {
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () => { root!.render(<Harness />) })

    await click(container.querySelector('[aria-label="Chọn dự án khác"]'))
    await click(container.querySelectorAll('[role=option]')[1])
    expect(container.textContent).toContain('Đang xem quy tắc của dự án “Trở Về”')

    await click(container.querySelector('#add'))
    await act(async () => { await Promise.resolve() })
    expect(container.querySelector('[data-testid=dirty]')?.textContent).toBe('1')

    await click(container.querySelector('#apply'))
    await act(async () => { await Promise.resolve() })
    expect(api.putRuleFile).toHaveBeenCalledWith('project', 'quy-tac.md', '- Bỏ từ bỗng nhiên.\n', 'tro-ve', undefined)
    for (const call of api.putRuleFile.mock.calls) expect(call[3]).not.toBe('van-chau')
  })
})
