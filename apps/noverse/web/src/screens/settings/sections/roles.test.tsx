// @vitest-environment happy-dom
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Config, ConfigScope } from '../../../api/types'
import { getPath, setPath } from '../lib/config'
import { hasOwn, inheritedFrom, ovrOf } from '../lib/labels'
import type { SettingsContext } from '../types'
import { putConfig } from '../../../api/client'

vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  Icon: () => null,  AgentOrb: () => null }))
vi.mock('../../../api/client', () => ({
  getConfigModels: vi.fn(async () => ({ models: [
    { name: 'gpt-6.1-sol', displayName: 'gpt-6.1-sol', contextWindow: 272000, registryWindow: 272000, contextSource: 'registry', summaryThreshold: 231200, automaticWindow: 272000, automaticSource: 'registry' },
    { name: 'gpt-6-luna', displayName: 'gpt-6-luna', contextWindow: 200000, registryWindow: 0, contextSource: 'default', summaryThreshold: 170000, automaticWindow: 200000, automaticSource: 'default' },
  ] })),
  getConfig: vi.fn(async () => ({ target: { config: globalDoc }, version: 'w' })),
  putConfig: vi.fn(async () => ({})),
}))
import { RolesSection } from './roles'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren() })

const globalDoc: Config = {
  provider: 'openai', model: 'gpt-6.1-sol',
  providers: { openai: { models: [{ name: 'gpt-6.1-sol' }, { name: 'gpt-6-luna' }] } } as Config['providers'],
}
const projectDoc: Config = { roles: { writer: { provider: 'openai', model: 'gpt-6-luna' } } }

/** Cấp truyện thật sự nối nhãn kế thừa như index.tsx: bản nháp của truyện + chuỗi cấp dưới. */
function Harness({ scope = 'story', initial = {}, admin = true }: { scope?: ConfigScope; initial?: Config; admin?: boolean }) {
  const [draft, setDraft] = useState<Config>(initial)
  const chain = [{ scope: 'global' as const, path: '', exists: true, config: globalDoc }, { scope: 'project' as const, path: '', exists: true, config: projectDoc }]
  const writer = { ...projectDoc.roles?.writer, ...draft.roles?.writer } as { provider: string; model: string }
  const effective: Config = { ...globalDoc, roles: { writer } }
  const sources: Record<string, ConfigScope> = { 'roles.writer.model': 'project', 'roles.writer.provider': 'project', provider: 'global', model: 'global' }
  for (const path of ['roles.writer.model', 'roles.writer.provider']) if (getPath(draft, path) !== undefined) sources[path] = 'story'
  const ctx = {
    scope, admin, canEdit: true, draft, effective, presets: null, snapshot: null, refreshConfig: () => {}, goSection: () => {},
    setField: (path: string, value: unknown) => setDraft((prev) => setPath(prev, path, value)),
    resetField: (path: string) => setDraft((prev) => setPath(prev, path, undefined)),
    ovr: (paths: string[]) => ovrOf(scope, hasOwn(draft, paths), inheritedFrom(sources, paths, scope, chain)),
  } as unknown as SettingsContext
  return <RolesSection ctx={ctx} />
}

async function mount(props?: Parameters<typeof Harness>[0]) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<Harness {...props} />) })
  return container
}
const row = (container: HTMLElement, key: string) => container.querySelector<HTMLElement>(`[data-role="${key}"]`)!
const picker = (container: HTMLElement, key: string) => row(container, key).querySelector<HTMLButtonElement>('[role="combobox"]')!
const chooseModel = async (container: HTMLElement, key: string, label: string) => {
  await act(async () => { picker(container, key).click() })
  const option = [...row(container, key).querySelectorAll<HTMLElement>('[role="option"]')].find((node) => node.textContent?.includes(label))
  expect(option).toBeTruthy()
  await act(async () => { option!.click() })
}

describe('Trợ lý AI ở cấp truyện', () => {
  it('đổi model Người viết → nhãn “Riêng truyện này”; nút “Theo dự án” xoá khoá và nhãn về “Theo dự án”', async () => {
    const container = await mount()
    const writer = () => row(container, 'writer')
    expect(writer().querySelector('.st-ovr')!.textContent).toBe('Theo dự án')
    expect(writer().querySelector('button.hov-ghost')).toBeNull()

    await chooseModel(container, 'writer', 'gpt-6.1-sol')
    expect(writer().querySelector('.st-ovr')!.textContent).toBe('Riêng truyện này')
    expect(picker(container, 'writer').textContent).toContain('gpt-6.1-sol')
    const reset = [...writer().querySelectorAll('button')].find((button) => button.textContent === 'Theo dự án')!
    expect(reset).toBeTruthy()

    await act(async () => { reset.click() })
    expect(writer().querySelector('.st-ovr')!.textContent).toBe('Theo dự án')
    expect(picker(container, 'writer').textContent).toContain('gpt-6-luna')
  })

  it('đổi mức suy nghĩ cũng tạo khoá riêng và chép đủ provider+model để tệp truyện vẫn hợp lệ', async () => {
    const container = await mount()
    const high = [...row(container, 'writer').querySelectorAll('[role="radio"]')].find((node) => node.textContent === 'Cao') as HTMLElement
    await act(async () => { high.click() })
    expect(row(container, 'writer').querySelector('.st-ovr')!.textContent).toBe('Riêng truyện này')
  })

  it('Trọng tài không có hàng riêng; ghi rõ dùng Mặc định', async () => {
    const container = await mount()
    expect(container.textContent).toContain('Trọng tài luôn dùng model “Mặc định”.')
    expect(container.querySelectorAll('select')).toHaveLength(0)
    expect(container.querySelectorAll('[role="combobox"]')).toHaveLength(4)
  })

  it('hiện nguồn thật, ngưỡng tóm tắt và phạm vi áp dụng', async () => {
    const container = await mount()
    const trigger = row(container, 'writer').querySelector<HTMLButtonElement>('[data-context-trigger]')!
    expect(trigger?.textContent).toContain('dự phòng')
    await act(async () => trigger.click())
    expect(container.textContent).toContain('170K')
    expect(container.textContent).toContain('mọi trợ lý và mọi truyện')
    expect(row(container, 'default').textContent).toContain('tự nhận')
  })

  it('chỉ quản trị có Lưu, người viết chỉ xem', async () => {
    const container = await mount({ admin: false })
    await act(async () => row(container, 'writer').querySelector<HTMLButtonElement>('[data-context-trigger]')!.click())
    expect([...container.querySelectorAll('button')].some((button) => button.textContent === 'Lưu')).toBe(false)
    expect(container.querySelector<HTMLButtonElement>('[role="dialog"] button[role="radio"]')?.disabled).toBe(true)
  })

  it('quản trị lưu theo model ở cấp toàn cục và Tự nhận xoá giá trị riêng', async () => {
    const container = await mount()
    const open = () => row(container, 'writer').querySelector<HTMLButtonElement>('[data-context-trigger]')!.click()
    await act(async () => open())
    await act(async () => [...container.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find(button => button.textContent?.includes('Khi chạy qua máy chủ riêng'))!.click())
    await act(async () => [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent === '128K')!.click())
    await act(async () => [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent === 'Lưu')!.click())
    expect(vi.mocked(putConfig).mock.calls.at(-1)?.[0]).toBe('global')
    expect(vi.mocked(putConfig).mock.calls.at(-1)?.[1].providers?.openai.models?.find((model) => model.name === 'gpt-6-luna')?.context_window).toBe(128000)
    await act(async () => open())
    await act(async () => [...container.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find(button => button.textContent?.includes('Mức dự phòng'))!.click())
    await act(async () => [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent === 'Lưu')!.click())
    expect(vi.mocked(putConfig).mock.calls.at(-1)?.[1].providers?.openai.models?.find((model) => model.name === 'gpt-6-luna')).not.toHaveProperty('context_window')
  })

  it('kiểm tra 8–2.000 nghìn và mở lên ở hàng sát đáy; Esc đóng', async () => {
    const container = await mount()
    const trigger = row(container, 'editor').querySelector<HTMLButtonElement>('[data-context-trigger]')!
    trigger.getBoundingClientRect = () => ({ top: 740, bottom: 776, left: 100, right: 276, width: 176, height: 36, x: 100, y: 740, toJSON() {} })
    await act(async () => trigger.click())
    expect(container.querySelector('[role="dialog"]')?.getAttribute('data-placement')).toBe('top')
    const custom = [...container.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find(button => button.textContent?.includes('Khi chạy qua máy chủ riêng'))!
    await act(async () => custom.click())
    const input = container.querySelector<HTMLInputElement>('input[type="number"]')!
    await act(async () => { Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, '7'); input.dispatchEvent(new Event('input', { bubbles: true })); input.dispatchEvent(new Event('change', { bubbles: true })) })
    expect([...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent === 'Lưu')?.disabled).toBe(true)
    await act(async () => window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })))
    expect(container.querySelector('[role="dialog"]')).toBeNull()
  })
})
