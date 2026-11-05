// @vitest-environment happy-dom
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const whole = { name: 'Cả tập.mp3', path: 'tap-01/audio.mp3', format: 'audio', size: 20000, volume: 1 }
const chapters = [1, 2].map((chapter) => ({ ...whole, chapter, name: `Chương 0${chapter}.mp3`, path: `tap-01/chuong-${chapter}.mp3` }))
vi.mock('../../../data', () => ({
  keys: { story: (...parts: unknown[]) => parts },
  useAction: () => ({ run: vi.fn(), busy: false, pending: false }),
  useConnection: () => ({ writable: true }),
  useResource: () => ({ data: { files: [whole], chapterFiles: chapters, split: false, chapterCount: 2, durationSec: 60, chapters: [] }, refresh: vi.fn() }),
}))
vi.mock('../../../mobile/primitives', () => ({ BottomSheet: ({ open, children }: { open: boolean; children: ReactNode }) => open ? <section role="dialog">{children}</section> : null, useMobileToast: () => vi.fn() }))
vi.mock('../../../shell/jobs/useJobs', () => ({ useJobSelection: () => null }))
vi.mock('../../../shell/jobs/exportFlow', () => ({ exportData: () => undefined, registerExportWatch: vi.fn(), useAutoDownload: () => [false, vi.fn()] }))
vi.mock('../../../shell/useNames', () => ({ useStoryShortTitle: () => 'Truyện kiểm tra' }))
vi.mock('../useAudio', () => ({ usePlayer: () => ({ toggle: vi.fn() }), errText: String }))
vi.mock('../useExportForm', () => ({ useExportForm: () => ({ form: { format: 'mp3', split: false }, update: vi.fn(), setForm: vi.fn(), visible: {}, save: vi.fn() }) }))
import { ExportSheet } from './ExportSheet'

let root: Root
let container: HTMLDivElement
afterEach(async () => { await act(() => root?.unmount()); container?.remove() })

async function render(open: boolean, chapter: number) {
  if (!container?.isConnected) {
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
  }
  await act(() => root.render(<ExportSheet project="p" story="s" volume={1} open={open} onClose={vi.fn()}
    readyChapters={[1, 2]} total={2} queue={[]} writable trialChapter={chapter} focusChapter={chapter} initialView="exportChDone" />))
}

it('mở Chia sẻ giữ màn tệp đã xuất sau effect, chỉ hiện đúng chương dù tệp không có trong files', async () => {
  await render(true, 2)
  expect(container.textContent).toContain(chapters[1].name)
  expect(container.textContent).not.toContain(chapters[0].name)
  expect(container.textContent).not.toContain(whole.name)
  expect(container.querySelectorAll('button[aria-label]')).toHaveLength(2)
})

it('đóng rồi chia sẻ chương khác không giữ tệp chương cũ', async () => {
  await render(true, 2)
  await render(false, 2)
  await render(true, 1)
  expect(container.textContent).toContain(chapters[0].name)
  expect(container.textContent).not.toContain(chapters[1].name)
  expect(container.textContent).not.toContain(whole.name)
})
