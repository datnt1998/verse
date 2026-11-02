// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Route } from '../../../router'
import { MobileWorkbench } from './MobileWorkbench'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const fixture = vi.hoisted(() => ({ placeholder: false, refresh: vi.fn(async () => {}) }))
vi.mock('../../../data/useResource', () => ({
  useResource: (key: string[]) => ({
    data: key[0] === 'channels' ? [] : key[1] === 'list' ? { jobs: [], queuedTotal: 0 } : undefined,
    initialLoading: false,
    placeholder: key[1] === 'list' && fixture.placeholder,
    refresh: fixture.refresh,
  }),
}))
vi.mock('../../../data/events', () => ({ subscribeEvents: () => () => {} }))
vi.mock('../../../router', () => ({ navigate: vi.fn(), routePath: JSON.stringify }))
vi.mock('@noverse/ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@noverse/ui')>()
  return { ...actual, useTabMode: vi.fn(), toast: { err: vi.fn() }, Button: 'button' }
})
vi.mock('./MobileWorkbenchList', () => ({
  MobileWorkbenchList: ({ onOpenJob, onOpenRecap }: { onOpenJob: (id: string) => void; onOpenRecap: () => void }) => <div>
    <button onClick={() => onOpenJob('job')}>Mở video</button>
    <button onClick={onOpenRecap}>Mở Bản tin</button>
  </div>,
}))
vi.mock('./MobileWorkbenchDetail', () => ({ MobileWorkbenchDetail: () => <div>Chi tiết</div> }))
vi.mock('./MobileRecap', () => ({ MobileRecap: () => <div>Bản tin</div> }))
vi.mock('./MobilePlanSheet', () => ({ MobilePlanSheet: () => null }))

let root: Root | undefined
let host: HTMLDivElement
const route = (channel?: string, video?: string): Extract<Route, { name: 'workbench' }> => ({ name: 'workbench', channel, video })
async function render(next = route()) {
  if (!root) {
    host = document.createElement('div')
    host.style.overflowY = 'auto'
    document.body.append(host)
    root = createRoot(host)
  }
  await act(async () => { root!.render(<MobileWorkbench route={next} />) })
}
function scroll(top: number) {
  host.scrollTop = top
  host.dispatchEvent(new Event('scroll'))
}
afterEach(async () => {
  if (root) await act(async () => root!.unmount())
  root = undefined
  fixture.placeholder = false
  document.body.replaceChildren()
})

describe('Bàn dựng · giữ vị trí cuộn theo phạm vi', () => {
  it('khôi phục riêng Mọi kênh và từng kênh khi đổi phạm vi từ shell', async () => {
    await render()
    scroll(420)
    await render(route('channel-a'))
    expect(host.scrollTop).toBe(0)
    scroll(175)
    await render(route())
    expect(host.scrollTop).toBe(420)
    await render(route('channel-a'))
    expect(host.scrollTop).toBe(175)
  })

  it('vào chi tiết bắt đầu ở đầu; quay lại giữ vị trí danh sách', async () => {
    await render()
    scroll(380)
    await act(async () => { (host.querySelector('button') as HTMLButtonElement).click() })
    await render(route(undefined, 'job'))
    expect(host.scrollTop).toBe(0)
    await render()
    expect(host.scrollTop).toBe(380)
  })

  it('đợi dữ liệu đúng phạm vi trước khi khôi phục, không xoá vị trí cache', async () => {
    await render()
    scroll(460)
    await render(route('channel-a'))
    scroll(110)
    fixture.placeholder = true
    await render(route())
    // Nội dung cũ vẫn hiển thị khi khoá mới đang lấy dữ liệu.
    expect(host.scrollTop).toBe(110)
    fixture.placeholder = false
    await render(route())
    expect(host.scrollTop).toBe(460)
  })
})
