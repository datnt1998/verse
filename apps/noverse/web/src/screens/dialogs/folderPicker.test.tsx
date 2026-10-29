// @vitest-environment happy-dom
// 09-auth-onboard.md §10.3 — “Chọn thư mục khác…”: hộp chọn của máy chủ khi canReveal;
// rơi về bộ duyệt trong app khi từ xa (canReveal=false) hoặc pick-folder trả 501/403.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { ApiError, getFsPlaces, getSystem, inspectLibraryPath, listFsFolders, mkFsFolder, pickFolder } from '../../api/client'
import type { FsPlace, LibraryInspect, SystemInfo } from '../../api/types'
import { cache } from '@noverse/ui/data'

vi.mock('../../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/client')>()
  return {
    ...actual,
    getSystem: vi.fn(), pickFolder: vi.fn(),
    getFsPlaces: vi.fn(), listFsFolders: vi.fn(), mkFsFolder: vi.fn(), inspectLibraryPath: vi.fn(),
  }
})
vi.mock('../../shell/AppShell', () => ({ useMediaQuery: () => false }))

import { useFolderPicker } from './FolderBrowserDialog'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }))

const PLACES: FsPlace[] = [
  { id: 'home', label: 'Thư mục nhà', path: '/mock/home', icon: 'user' },
  { id: 'documents', label: 'Tài liệu', path: '/mock/home/Documents', icon: 'document-text' },
]
const EMPTY: LibraryInspect = { kind: 'empty', projects: 0, stories: 0, items: 0, place: 'Tài liệu', display: 'Tài liệu › Truyện Noverse', name: 'Truyện Noverse' }
const system = (canReveal: boolean) => ({ canReveal }) as SystemInfo

const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren(); cache.clear(); vi.clearAllMocks() })
beforeEach(() => {
  vi.mocked(getSystem).mockResolvedValue(system(true))
  vi.mocked(getFsPlaces).mockResolvedValue(PLACES)
  vi.mocked(listFsFolders).mockResolvedValue([])
  vi.mocked(mkFsFolder).mockResolvedValue({ name: 'x', path: '/mock/home/x', display: 'Thư mục nhà › x' })
  vi.mocked(inspectLibraryPath).mockResolvedValue(EMPTY)
})

/** Gắn host gọi useFolderPicker rồi render; trả nút mở bộ chọn + spy onPicked. */
async function mount(onPicked = vi.fn()) {
  function Host() {
    const picker = useFolderPicker({ onPicked })
    return <>
      <button type="button" onClick={() => void picker.pick()}>Chọn thư mục khác…</button>
      {picker.error && <span role="alert">{picker.error}</span>}
      {picker.dialog}
    </>
  }
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(<Host />) })
  return { container, onPicked }
}

const openDialog = () => document.body.querySelector('[role="dialog"]')
const clickPick = async (container: HTMLElement) =>
  act(async () => { container.querySelector('button')!.dispatchEvent(new MouseEvent('click', { bubbles: true })) })

it('máy chủ truy cập trực tiếp: gọi pick-folder, kiểm tra đường dẫn rồi onPicked — không mở bộ duyệt', async () => {
  vi.mocked(pickFolder).mockResolvedValue({ path: '/mock/home/Documents/Truyện Noverse', cancelled: false })
  const { container, onPicked } = await mount()
  await clickPick(container)
  expect(pickFolder).toHaveBeenCalled()
  expect(inspectLibraryPath).toHaveBeenCalledWith('/mock/home/Documents/Truyện Noverse')
  expect(onPicked).toHaveBeenCalledWith('/mock/home/Documents/Truyện Noverse', EMPTY)
  expect(openDialog()).toBeNull()
})

it('pick-folder trả cancelled → không làm gì', async () => {
  vi.mocked(pickFolder).mockResolvedValue({ path: '', cancelled: true })
  const { container, onPicked } = await mount()
  await clickPick(container)
  expect(onPicked).not.toHaveBeenCalled()
  expect(openDialog()).toBeNull()
})

it('pick-folder 501 (máy không có hộp chọn) → mở bộ duyệt trong app', async () => {
  vi.mocked(pickFolder).mockRejectedValue(new ApiError(501, 'Máy chủ không hỗ trợ chọn thư mục'))
  const { container } = await mount()
  await clickPick(container)
  expect(openDialog()).not.toBeNull()
  expect(openDialog()!.textContent).toContain('Chọn thư mục lưu truyện')
})

it('pick-folder 403 (không phải loopback) → mở bộ duyệt trong app', async () => {
  vi.mocked(pickFolder).mockRejectedValue(new ApiError(403, 'Chỉ chọn được thư mục khi dùng ngay trên máy chủ'))
  const { container } = await mount()
  await clickPick(container)
  expect(openDialog()).not.toBeNull()
})

it('canReveal=false (từ xa/điện thoại) → mở bộ duyệt trong app, không gọi pick-folder', async () => {
  vi.mocked(getSystem).mockResolvedValue(system(false))
  const { container } = await mount()
  await clickPick(container)
  expect(pickFolder).not.toHaveBeenCalled()
  expect(openDialog()).not.toBeNull()
})

it('lỗi khác (mất mạng…) → báo lỗi, không mở bộ duyệt', async () => {
  vi.mocked(pickFolder).mockRejectedValue(new ApiError(500, 'Máy chủ lỗi'))
  const { container } = await mount()
  await clickPick(container)
  expect(openDialog()).toBeNull()
  expect(container.textContent).toContain('Máy chủ lỗi')
})
