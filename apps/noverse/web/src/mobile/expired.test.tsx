// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { ReactNode } from 'react'
import { afterEach, expect, it, vi } from 'vitest'

// Sheet `expired` của MWork.dc.html:378-390: alertdialog không đóng được (không nút đóng, scrim và Esc không làm gì)
// và góp ý đang gõ dở được giữ qua lần đăng nhập lại.
vi.mock('../api/session', () => ({
  useSession: () => ({ user: { username: 'dat', name: 'Đạt', role: 'user' }, setUser: vi.fn(), refresh: vi.fn(async () => {}) }),
}))
vi.mock('../api/library', () => ({ useLibrary: () => ({ reload: vi.fn() }) }))
vi.mock('../data', () => ({
  cache: { revalidateActive: vi.fn(async () => {}), clear: vi.fn() },
  events: { start: vi.fn(), stop: vi.fn() },
  subscribeEvents: vi.fn(() => () => {}),
  useConnection: () => ({ online: true, state: 'open', reconnectAt: null, retry: vi.fn() }),
  useResource: () => ({ data: undefined, loading: false, error: undefined, reload: vi.fn() }),
  useAction: (fn: (...args: never[]) => unknown) => ({ busy: false, pending: false, error: undefined, run: (...args: unknown[]) => (fn as (...a: unknown[]) => unknown)(...args) }),
  keys: {}, keyHash: () => 'h', matchesPrefix: () => false,
  invalidate: vi.fn(), setData: vi.fn(), createCache: vi.fn(), createAction: vi.fn(),
  DataProvider: ({ children }: { children: ReactNode }) => children,
}))
vi.mock('./outbox', () => ({ drainOutbox: vi.fn(async () => {}), setOutboxAccount: vi.fn() }))
vi.mock('../screens/auth/publicApi', () => ({
  loginPublic: vi.fn(async () => ({ user: { username: 'dat', name: 'Đạt', role: 'user' } })),
  pairPublic: vi.fn(),
}))

import { getDraft, setDraft, setDraftAccount, useAccountDraft } from './accounts'
import { sessionGate } from './session'
import { MobileRuntimeProvider } from './runtime'
import { MobileFeedback } from './primitives'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  sessionGate.complete()
  vi.clearAllMocks()
})

const render = async (ui: ReactNode) => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(ui) })
  return { container, root }
}
const type = (element: HTMLInputElement, value: string) => {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(element, value)
  element.dispatchEvent(new Event('input', { bubbles: true }))
}

it('sheet hết hạn không có nút đóng; scrim và Esc không đóng được', async () => {
  setDraftAccount('dat')
  setDraft('khóa-a', 'Cho Tri Hạ xuất hiện lại ở chương 26…')
  sessionGate.request()
  const { root } = await render(<MobileFeedback><MobileRuntimeProvider><div className="m-app" /></MobileRuntimeProvider></MobileFeedback>)
  await act(async () => {})
  const sheet = document.querySelector<HTMLElement>('.m-login-sheet [role="alertdialog"]')
  expect(sheet).toBeTruthy()
  expect(sheet!.textContent).toContain('Phiên đăng nhập đã hết')
  expect(sheet!.textContent).toContain('Góp ý đang gõ dở vẫn được giữ')
  expect(sheet!.textContent).toContain('“Cho Tri Hạ xuất hiện lại ở chương 26…”')
  // Sheet chỉ có hai điều khiển của artboard — không nút đóng.
  expect([...sheet!.querySelectorAll('button, a')].map((el) => el.textContent!.trim()))
    .toEqual(['Đăng nhập', 'Quét mã QR từ máy tính'])
  await act(async () => { document.querySelector('.m-scrim')!.dispatchEvent(new MouseEvent('click', { bubbles: true })) })
  expect(document.querySelector('.m-login-sheet')).toBeTruthy()
  await act(async () => { document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })) })
  expect(document.querySelector('.m-login-sheet')).toBeTruthy()
  expect(sessionGate.isOpen()).toBe(true)
  void root
})

it('góp ý đang gõ dở còn nguyên sau khi đăng nhập lại', async () => {
  setDraftAccount('dat')
  setDraft('khóa-b', 'Cho Tri Hạ xuất hiện lại ở chương 26…')
  sessionGate.request()
  function Composer() { const [text] = useAccountDraft('khóa-b'); return <div className="m-app"><span className="probe">{text}</span></div> }
  const { container, root } = await render(<MobileFeedback><MobileRuntimeProvider><Composer /></MobileRuntimeProvider></MobileFeedback>)
  await act(async () => {})
  const sheet = document.querySelector<HTMLElement>('.m-login-sheet')!
  // pwOff: nút mở khóa từ 8 ký tự như artboard.
  const submit = sheet.querySelector<HTMLButtonElement>('.m-button')!
  expect(submit.disabled).toBe(true)
  await act(async () => { type(sheet.querySelector<HTMLInputElement>('input[type="password"]')!, 'mat-khau-123') })
  expect(submit.disabled).toBe(false)
  await act(async () => { sheet.querySelector<HTMLFormElement>('.m-reauth-form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })) })
  await act(async () => {})
  expect(sessionGate.isOpen()).toBe(false)
  expect(document.querySelector('.m-login-sheet')).toBeNull()
  expect(container.querySelector('.m-toast')?.textContent).toContain('Đã đăng nhập lại · góp ý vẫn còn trong ô chat')
  expect(getDraft('khóa-b')).toBe('Cho Tri Hạ xuất hiện lại ở chương 26…')
  expect(container.querySelector('.probe')?.textContent).toBe('Cho Tri Hạ xuất hiện lại ở chương 26…')
  void root
})
