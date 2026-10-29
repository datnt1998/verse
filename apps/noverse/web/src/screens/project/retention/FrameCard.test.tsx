// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import type { Frame } from '../../../api/types'
import { putFrame } from '../../../api/client'
import { FrameCard } from './FrameCard'

const state = vi.hoisted(() => ({ frame: { audience: [], difference: '', sequelSeed: '', source: 'architect', audiences: [{ n: 0, label: 'Phụ nữ trung niên, nghe khi làm việc nhà', wants: '' }, { n: 1, label: 'Nữ 18–35', wants: '' }] } as Frame }))
vi.mock('../../../api/client', () => ({ canWriteStory: () => true, getConfigPresets: vi.fn(), getFrame: vi.fn(), putFrame: vi.fn(async (_p, _s, input) => ({ ...state.frame, ...input })) }))
vi.mock('../../../api/session', () => ({ useSession: () => ({ user: { role: 'admin' } }) }))
vi.mock('../../../data', () => ({ keys: { story: () => ['frame'] }, setData: vi.fn(), useResource: (key: string[]) => ({ data: key[0] === 'frame' ? state.frame : undefined }) }))
vi.mock('../applied', () => ({ useEffectiveConfig: () => undefined }))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  toast: { ok: vi.fn(), err: vi.fn() } }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | undefined
let host: HTMLDivElement | undefined
async function render(difference: string) {
  state.frame = { ...state.frame, audience: [], difference }
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(() => root!.render(<FrameCard project="du-an" story="truyen" />))
}
afterEach(async () => { await act(() => root?.unmount()); host?.remove(); vi.clearAllMocks() })

it.each(['', '   ', 'other_stories', 'Không có foundation_memory.other_stories để so sánh.'])('renders an honest empty comparison for %s', async (difference) => {
  await render(difference)
  expect(host!.textContent).toContain('Chưa có truyện trước để so sánh.')
  expect(host!.textContent).not.toContain('other_stories')
})
it('preserves actual user-facing comparison text', async () => {
  await render('Nghề hậu kỳ âm thanh thay vì kinh doanh.')
  expect(host!.textContent).toContain('Nghề hậu kỳ âm thanh thay vì kinh doanh.')
})
it('offers a picker for an empty listener group and saves using the frame settings API', async () => {
  await render('')
  const picker = host!.querySelector<HTMLButtonElement>('button[aria-label="Chọn nhóm người nghe"]')
  expect(picker).not.toBeNull()
  await act(() => picker!.click())
  const group = [...host!.querySelectorAll('button')].find((b) => b.textContent === 'Nữ 18–35')!
  await act(() => group.click())
  await act(async () => { [...host!.querySelectorAll('button')].find((b) => b.textContent === 'Xong')!.click() })
  expect(putFrame).toHaveBeenCalledWith('du-an', 'truyen', { audience: [1] })
})
