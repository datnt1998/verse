import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { ActivityBridge } from './ActivityBridge'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const fixture = vi.hoisted(() => ({ activity: { running: [] as unknown[], videoJobs: 0, queued: 0, unread: [], badge: 0 }, settings: { kinds: {}, doNotDisturb: { enabled: false, start: '23:00', end: '07:00' }, badge: false, merge: true, recapOnReturn: true }, toast: vi.fn(), refresh: vi.fn() }))
vi.mock('../data/useResource', () => ({ useResource: (key: string[]) => ({ data: key.length === 1 ? fixture.activity : fixture.settings, refresh: fixture.refresh }) }))
vi.mock('@noverse/ui', () => ({ toast: Object.assign(fixture.toast, { card: vi.fn() }) }))
let root: Root | undefined
afterEach(async () => { if (root) await act(async () => root?.unmount()); document.body.replaceChildren(); vi.clearAllMocks() })
it('does not toast when a video moves between progress stages', async () => {
  root = createRoot(document.body.appendChild(document.createElement('div')))
  await act(async () => root?.render(<ActivityBridge />))
  fixture.activity = { ...fixture.activity, running: [{ id: 'one', phase: 'Đạo diễn' }] }
  await act(async () => root?.render(<ActivityBridge />))
  expect(fixture.toast).not.toHaveBeenCalled()
})
