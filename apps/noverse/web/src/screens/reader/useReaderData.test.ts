import { afterEach, expect, it, vi } from 'vitest'
import { keys } from '../../data/keys'
import { ApiError } from '../../api/client'
import type { ChapterText } from '../../api/types'

const state = vi.hoisted(() => ({
  connection: 'open', revision: 'one', previous: { current: null as string | null },
  invalidate: vi.fn(), refresh: vi.fn(), cleanups: [] as (() => void)[],
  chapterText: null as ChapterText | null, chapterError: null as ApiError | null,
}))
vi.mock('react', () => ({
  useRef: () => state.previous,
  useEffect: (effect: () => void | (() => void)) => { const cleanup = effect(); if (cleanup) state.cleanups.push(cleanup) },
}))
vi.mock('../../api/session', () => ({ useSession: () => ({ user: null }) }))
vi.mock('../../data', () => ({
  get keys() { return keys },
  invalidate: state.invalidate,
  useConnection: () => ({ state: state.connection }),
  useResource: (key: readonly string[]) => ({ data: key[3] === 'chapters' ? { revision: state.revision, chapters: [] } : key[3] === 'chapter' ? state.chapterText : null, error: key[3] === 'chapter' ? state.chapterError : null, refresh: state.refresh }),
}))
import { useChapterContent, useChapters } from './useReaderData'

afterEach(() => {
  state.cleanups.splice(0).forEach((cleanup) => cleanup())
  state.invalidate.mockClear(); state.previous.current = null; state.connection = 'open'; state.revision = 'one'
  state.chapterText = null; state.chapterError = null
})

it('SSE đã làm mới chương thì revision không làm mới lần thứ hai', () => {
  useChapters('p', 's')
  state.revision = 'two'
  useChapters('p', 's')
  expect(state.invalidate).not.toHaveBeenCalled()
})

it('revision từ polling vẫn làm mới dữ liệu phụ khi luồng ngắt', () => {
  state.connection = 'retrying'
  useChapters('p', 's')
  state.invalidate.mockClear()
  state.revision = 'two'
  useChapters('p', 's')
  expect(state.invalidate).toHaveBeenCalledTimes(6)
  expect(state.invalidate).toHaveBeenCalledWith(['story', 'p', 's', 'reviews'])
})

it('bản nháp trả 404 không được giữ chính văn chương trước trong trạng thái chưa viết', () => {
  state.chapterText = { chapter: 8, title: 'Chương trước', text: 'Không phải chương 9.', words: 4, source: 'committed' }
  state.chapterError = new ApiError(404, 'không có chương')
  const missing = useChapterContent('p', 's', 9, 'draft')
  expect(missing.content).toBeNull()
  expect(missing.error?.status).toBe(404)
  state.chapterError = new ApiError(503, 'máy chủ chưa sẵn sàng')
  expect(useChapterContent('p', 's', 8, 'committed').content?.text).toBe('Không phải chương 9.')
})
