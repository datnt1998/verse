import { renderToStaticMarkup } from 'react-dom/server'
import { beforeEach, expect, it, vi } from 'vitest'
import type * as Client from '../../api/client'
import type { AudioChapter } from '../../api/types'

const state = vi.hoisted(() => ({ actions: [] as ((...args: unknown[]) => unknown)[], chapters: [] as AudioChapter[] }))
vi.mock('../../api/client', async original => ({ ...await original<typeof Client>(), renderAudioChapter: vi.fn(), enqueueAudioChapters: vi.fn() }))
vi.mock('../../api/session', () => ({ useSession: () => ({ user: { role: 'admin' } }) }))
vi.mock('../../data', () => ({
  keys: { story: (...args: unknown[]) => args, project: (...args: unknown[]) => args, jobs: () => ['jobs'] },
  useResource: () => ({}),
  useAction: (action: (...args: unknown[]) => unknown) => { state.actions.push(action); return { run: action } },
}))
vi.mock('./useAudio', () => ({
  useAudioData: () => ({ volume: { chapters: state.chapters }, queue: [], settings: { quality: 'high' }, cast: { mode: 'single', voiceMode: 'single' } }),
  useAudioAccess: () => ({ writable: true }), usePlayer: () => ({}), usePreviewAudio: () => ({}),
}))
vi.mock('./dialogs', () => ({ useQualityChange: () => ({}), AddVoiceDialog: () => null }))
vi.mock('./packVoice', () => ({ usePackVoice: () => ({}) }))
import { enqueueAudioChapters, renderAudioChapter } from '../../api/client'
import { TranscriptScreen } from './Transcript'

beforeEach(() => { state.actions = []; vi.clearAllMocks() })

it('Tạo lại re-locks edited review scripts through the current-source queue', async () => {
  state.chapters = [{ chapter: 3, status: 'review', scriptChanged: true } as AudioChapter]
  renderToStaticMarkup(<TranscriptScreen project="p" story="s" volume={1} />)
  await state.actions[1]('render', 3)
  expect(enqueueAudioChapters).toHaveBeenCalledWith('p', 's', 1, { chapters: [3], quality: 'high', voiceMode: 'single', lock: true })
  expect(renderAudioChapter).not.toHaveBeenCalled()
})

it('the locked missing-segment button keeps its render-only request', async () => {
  state.chapters = [{ chapter: 3, status: 'locked', scriptChanged: false, chapterChanged: false } as AudioChapter]
  renderToStaticMarkup(<TranscriptScreen project="p" story="s" volume={1} />)
  await state.actions[1]('render', 3)
  expect(renderAudioChapter).toHaveBeenCalledWith('p', 's', 1, 3)
  expect(enqueueAudioChapters).not.toHaveBeenCalled()
})
