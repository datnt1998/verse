// Màn Tạo kênh là một vùng nội dung của route: máy tính phải có đúng một
// landmark <main> (trước đây chỉ là <div> nên màn không có main nào), còn
// điện thoại đã có <main> riêng trong MobilePage của bước Tạo/Xong — khung
// ngoài không được lồng thêm main thứ hai.
// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ChannelNewScreen } from './ChannelNewScreen'
import type { Route } from '../../router'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const state = vi.hoisted(() => ({ narrow: false }))

vi.mock('../../data/useResource', () => ({ useResource: () => ({ data: { channels: [] }, error: null, initialLoading: false, refresh: vi.fn() }) }))
vi.mock('../../lib/onboard', () => ({
  onboardDismissed: () => true,
  dismissOnboard: vi.fn(),
  setLastCreatedChannel: vi.fn(),
  lastCreatedChannel: () => null,
}))
vi.mock('../../router', () => ({
  navigate: vi.fn(),
  paths: {
    workbench: () => '#/',
    channel: (id: string) => `#/channels/${id}`,
    channelNew: () => '#/channels/new',
    settings: () => '#/settings',
  },
}))
vi.mock('../../components', () => ({ LensLogo: () => null }))
vi.mock('./CreateStep', () => ({ CreateStep: () => <section data-step="create" /> }))
vi.mock('./DoneStep', () => ({ DoneStep: () => <section data-step="done" /> }))

let root: Root | undefined
let host: HTMLDivElement
const render = async (route: Extract<Route, { name: 'channel-new' }>) => {
  if (!root) {
    host = document.createElement('div')
    document.body.append(host)
    root = createRoot(host)
  }
  await act(async () => root!.render(<ChannelNewScreen route={route} />))
}

beforeEach(() => {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: state.narrow && query.includes('max-width'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
  state.narrow = false
})
afterEach(async () => {
  if (root) await act(async () => root!.unmount())
  root = undefined
  vi.unstubAllGlobals()
  document.body.replaceChildren()
})

describe('landmark của màn Tạo kênh', () => {
  it.each(['create', 'done'] as const)('máy tính: bước %s có đúng một main, không lồng main', async (step) => {
    await render({ name: 'channel-new', step })
    const mains = host.querySelectorAll('main')
    expect(mains).toHaveLength(1)
    const main = mains[0] as HTMLElement
    expect(main.parentElement).toBe(host)
    expect(main.querySelector(`[data-step="${step}"]`)).not.toBeNull()
    expect(main.querySelector('main')).toBeNull()
  })

  it('điện thoại: khung ngoài không lồng main vì MobilePage đã có main riêng', async () => {
    state.narrow = true
    await render({ name: 'channel-new', step: 'create' })
    const outer = host.firstElementChild as HTMLElement
    expect(outer.tagName).toBe('DIV')
    expect(outer.querySelector('main')).toBeNull()
  })
})
