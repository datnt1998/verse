// @vitest-environment happy-dom
import { act, useEffect, useRef } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
vi.mock('../api/session', () => ({ useSession: () => ({ user: null, setUser: vi.fn(), refresh: vi.fn() }) }))
vi.mock('../api/library', () => ({ useLibrary: () => ({ reload: vi.fn() }) }))
vi.mock('../data', () => ({ cache: { revalidateActive: vi.fn() }, useConnection: () => ({ online: true, state: 'open', reconnectAt: null, retry: vi.fn() }) }))
vi.mock('../screens/auth/mobile', () => ({ MobileAuth: () => null }))
vi.mock('./accounts', () => ({ latestDraft: () => '', registerAccountSession: () => () => {}, rememberAccount: vi.fn(), setDraftAccount: vi.fn() }))
vi.mock('./outbox', () => ({ drainOutbox: vi.fn(), setOutboxAccount: vi.fn() }))
vi.mock('./primitives', () => ({ useMobileToast: () => vi.fn() }))
import { MobileRuntimeProvider, useMobileRuntime, useMobileScroll, useTabMode } from './runtime'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
it('unchanged scroll flags do not cause commits or rerun mount effects', async () => {
  const container = document.createElement('div'); document.body.append(container)
  const root = createRoot(container)
  let renders = 0, effects = 0
  let setScroll!: (down: boolean, immersive: boolean) => void
  function Probe() {
    renders++; setScroll = useMobileRuntime().setScroll
    const ref = useRef<HTMLDivElement>(null)
    useMobileScroll(ref); useTabMode()
    useEffect(() => { effects++ }, [])
    return <div ref={ref} />
  }
  try {
    await act(async () => root.render(<MobileRuntimeProvider><Probe /></MobileRuntimeProvider>))
    const baseline = renders
    await act(async () => { for (let i = 0; i < 10; i++) setScroll(false, false) })
    expect(renders).toBe(baseline)
    expect(effects).toBe(1)
  } finally { await act(async () => root.unmount()); container.remove() }
})
