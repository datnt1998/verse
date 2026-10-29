import { renderToStaticMarkup } from 'react-dom/server'
import type { ReactNode } from 'react'
import { expect, it, vi } from 'vitest'

const file = { name: 'Khi sếp học Vibecode · Tập 1-ch1-8.mp3', path: 'tap-01/audio.mp3', format: 'audio', size: 20_000_000, volume: 1 }
vi.mock('../../../data', () => ({
  keys: { story: (...parts: unknown[]) => parts },
  useAction: () => ({ run: vi.fn(), busy: false }),
  useConnection: () => ({ writable: true }),
  useResource: () => ({ data: { files: [file], chapterFiles: [], chapterCount: 8, durationSec: 1261, chapters: [] }, refresh: vi.fn() }),
}))
vi.mock('../../../mobile/primitives', () => ({ BottomSheet: ({ children }: { children: ReactNode }) => <section>{children}</section>, useMobileToast: () => vi.fn() }))
vi.mock('../../../shell/jobs/useJobs', () => ({ useJobSelection: () => null }))
vi.mock('../../../shell/jobs/exportFlow', () => ({ exportData: () => undefined, registerExportWatch: vi.fn(), useAutoDownload: () => [false, vi.fn()] }))
vi.mock('../../../shell/useNames', () => ({ useStoryShortTitle: () => 'Khi sếp học Vibecode' }))
vi.mock('../useAudio', () => ({ usePlayer: () => ({ toggle: vi.fn() }), errText: String }))
vi.mock('../useExportForm', () => ({ useExportForm: () => ({ form: { format: 'mp3', split: false }, update: vi.fn(), setForm: vi.fn(), visible: {}, save: vi.fn() }) }))
import { ExportSheet } from './ExportSheet'

it('shows the artboard completion summary without a one-line ellipsized filename', () => {
  const html = renderToStaticMarkup(<ExportSheet project="p" story="s" volume={1} open onClose={vi.fn()} readyChapters={[1,2,3,4,5,6,7,8]} total={8} queue={[]} writable initialDone trialChapter={1} />)
  expect(html).toContain('8 chương · 21 phút')
  expect(html).not.toContain('text-overflow:ellipsis')
  expect(html).toContain(file.name)
  expect(html).toContain('white-space:normal')
})
