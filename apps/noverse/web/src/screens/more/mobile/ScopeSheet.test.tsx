// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MobileScopeSheet } from './ScopeSheet'
import { BottomSheet, ConfirmSheet } from '../../../mobile/primitives'

vi.mock('../../../api/library', () => ({ useLibrary: () => ({ library: { projects: [] }, projectDetail: () => undefined, ensureProject: vi.fn() }) }))
vi.mock('../../../api/session', () => ({ useSession: () => ({ user: { role: 'admin' } }) }))
vi.mock('../../../mobile/runtime', () => ({ useMobileScroll: vi.fn() }))
vi.mock('../../settings/scope/counts', () => ({ useOwnRuleCount: () => 0 }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | undefined
let host: HTMLDivElement | undefined
async function mount(children: React.ReactNode) {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(async () => { root!.render(children) })
  return document.querySelector<HTMLElement>('.m-sheet')!
}
afterEach(async () => {
  if (root) await act(async () => { root!.unmount() })
  host?.remove()
  root = undefined
})

describe('mobile scope sheet sizing contract', () => {
  it('fits its content rather than occupying the 86% detent, while retaining a viewport cap', async () => {
    const sheet = await mount(<MobileScopeSheet open onClose={vi.fn()} target={{ scope: 'global', project: '', story: '' }} openTarget={null} dirtyCount={0} onTarget={vi.fn()} onSave={vi.fn()} onDiscard={vi.fn()} />)
    expect(sheet.style.height).toBe('auto')
    expect(sheet.style.maxHeight).toBe('min(86%, calc(100dvh - var(--m-top) - 12px))')
    expect(sheet.querySelector('[aria-label="Tìm dự án"]')).not.toBeNull()
    expect(sheet.querySelector('[role=listbox]')).not.toBeNull()
  })

  it('leaves the shared fixed-height sheet default unchanged', async () => {
    const sheet = await mount(<BottomSheet open onClose={vi.fn()} title="Danh sách chương"><p>Nội dung</p></BottomSheet>)
    expect(sheet.style.height).toBe('640px')
    expect(sheet.style.maxHeight).toBe('')
    expect(sheet.querySelector('.m-drag')).not.toBeNull()
  })

  it('preserves grabber-free content-fit confirmation sheets', async () => {
    const sheet = await mount(<ConfirmSheet open onClose={vi.fn()} title="Đăng xuất?" description="Phiên hiện tại sẽ kết thúc." action={{ label: 'Đăng xuất', onSelect: vi.fn() }} />)
    expect(sheet.style.height).toBe('auto')
    expect(sheet.querySelector('.m-drag')).toBeNull()
    expect(sheet.getAttribute('role')).toBe('alertdialog')
  })
})
