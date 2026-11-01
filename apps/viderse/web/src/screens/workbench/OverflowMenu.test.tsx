// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CloneJobView } from '../../api/workbench'
import { OverflowMenu } from './OverflowMenu'
import { removeVideo } from './jobActions'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
vi.mock('./jobActions', () => ({ restartVideo: vi.fn(), removeVideo: vi.fn(), stopVideo: vi.fn() }))
let root: Root | undefined
const close = vi.fn()
async function render(stageGroup: string) {
  const host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(async () => root!.render(<OverflowMenu
    job={{ id: 'j1', channelId: 'c1', stageGroup, source: { title: 'Đêm ở trạm', url: '' }, steps: [] } as unknown as CloneJobView}
    onClose={close}
    onRefresh={vi.fn()}
    onError={vi.fn()}
  />))
  await act(async () => { (host.querySelector('button') as HTMLButtonElement).click() })
  return [...document.querySelectorAll<HTMLElement>('[role="menuitem"]')].find((item) => item.textContent?.includes('Xoá video'))!
}
afterEach(async () => {
  if (root) await act(async () => root!.unmount())
  root = undefined
  vi.clearAllMocks()
  document.body.replaceChildren()
})

describe('menu ⋯ của Bàn dựng · ActionMenu kit', () => {
  it('khoá Xoá video khi đang chạy, hiển thị lý do và không gọi thao tác xoá', async () => {
    const row = await render('working')
    expect(row).toBeTruthy()
    expect(row.textContent).toContain('Tạm dừng trước rồi xoá')
    expect(row.getAttribute('aria-disabled')).toBe('true')
    await act(async () => row.click())
    expect(removeVideo).not.toHaveBeenCalled()
  })
  it('cho xoá khi video không chạy và đóng menu sau khi chọn', async () => {
    const row = await render('ready')
    expect(row.textContent).toContain('Xoá MP4, ảnh và project của video này')
    expect(row.getAttribute('aria-disabled')).not.toBe('true')
    await act(async () => row.click())
    expect(removeVideo).toHaveBeenCalledOnce()
    expect(close).toHaveBeenCalled()
    expect(document.querySelector('[role="menu"]')).toBeNull()
  })
})
