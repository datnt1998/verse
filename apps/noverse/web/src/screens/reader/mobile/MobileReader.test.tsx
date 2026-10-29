// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, expect, it, vi } from 'vitest'
import type { ReactNode } from 'react'
import { ApiError } from '../../../api/client'
import type { ChapterInfo } from '../../../api/types'
import type * as Router from '../../../router'

const state = vi.hoisted(() => ({
  chapter: { chapter: 9, title: 'Tôi có ký', completed: false, words: 0, hasDraft: false, volume: 2, arc: 1 } as ChapterInfo,
  text: '', failure: 404, source: '', reload: vi.fn(), navigate: vi.fn(), reveal: vi.fn(),
}))
vi.mock('../useReaderData', () => ({
  useChapters: () => ({ chapters: [state.chapter], outline: null, error: null, reload: vi.fn() }),
  useChapterContent: (_p: string, _s: string, chapter: number, source: string) => {
    state.source = source
    return { content: state.failure ? null : { chapter, title: state.chapter.title, text: state.text, words: 17, source }, loading: false, error: state.failure ? new ApiError(state.failure, 'không có chương') : null, reload: state.reload, refreshing: false, updatedAt: null }
  },
  useChapterExtras: () => ({ review: null }),
  useStoryAccess: () => ({ locked: false, writable: false }),
  useExports: () => ({ data: null }),
  useForeshadow: () => ({ entries: [] }),
}))
vi.mock('../../../api/session', () => ({ useSession: () => ({ user: null }) }))
vi.mock('../../../data', () => ({
  keys: { story: (...parts: unknown[]) => parts }, useResource: () => ({ data: null }),
  useResourceValue: (_key: unknown, _fetcher: unknown, select: (data: undefined) => unknown) => select(undefined),
  useConnection: () => ({ state: 'open', online: true, writable: true }), useAction: () => ({ busy: false }),
  events: { subscribe: () => () => {} },
}))
vi.mock('../../../mobile/primitives', () => ({
  MobilePage: ({ title, actions, children }: { title: ReactNode; actions: ReactNode; children: ReactNode }) => <main>{title}{actions}{children}</main>,
  BottomSheet: ({ open, children }: { open: boolean; children: ReactNode }) => open ? <section role="dialog">{children}</section> : null,
  useMobileToast: () => vi.fn(),
}))
vi.mock('../../../mobile/runtime', () => ({ useMobileRuntime: () => ({ keyboard: 0 }), useMobileScroll: () => ({ immersive: false, reveal: state.reveal }) }))
vi.mock('../../../mobile/accounts', () => ({ setDraft: vi.fn(), useAccountDraft: () => ['', vi.fn()] }))
vi.mock('../../../mobile/outbox', () => ({ enqueueComment: vi.fn(), useCommentOutbox: () => [] }))
vi.mock('../comments', () => ({ PARAGRAPH_COMMENT_CHIPS: [], useParagraphComments: () => ({ data: null, add: { busy: false } }), paragraphAnchor: vi.fn(), selectionInReader: vi.fn() }))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  ScrollTopButton: () => null }))
vi.mock('../../../shell', () => ({ useMediaQuery: () => false, useShellChrome: () => ({ header: 0, tabbar: 0 }) }))
vi.mock('../../../ui/useUnsavedGuard', () => ({ useUnsavedGuard: vi.fn() }))
vi.mock('../ReaderComposer', () => ({ ReaderComposer: () => null }))
vi.mock('../ReviewPanel', () => ({ ReviewPanel: () => null }))
vi.mock('../ExportPanel', () => ({ ExportPanel: () => null }))
vi.mock('../../../router', async (importOriginal) => ({ ...await importOriginal<typeof Router>(), navigate: state.navigate }))
import { MobileReader } from './MobileReader'
import { ReaderScreen } from '../index'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
afterEach(() => {
  state.chapter.hasDraft = false; state.chapter.completed = false; state.text = ''; state.failure = 404
  state.reload.mockClear(); state.navigate.mockClear(); localStorage.clear()
})

it('a draft not written yet is a neutral empty state, not a red retry error', () => {
  const html = renderToStaticMarkup(<MobileReader project="p" story="s" chapter={9} />)
  expect(state.source).toBe('draft')
  expect(html).toContain('Chưa có bản nháp cho chương này')
  expect(html).not.toContain('role="alert"')
  expect(html).not.toContain('Thử lại')
  state.failure = 0; state.text = ' \n'
  const empty = renderToStaticMarkup(<MobileReader project="p" story="s" chapter={9} />)
  expect(empty).toContain('Chưa có bản nháp cho chương này')
  expect(empty).not.toContain('Thử lại')
})

it('an unfinished chapter renders its partial draft instead of requesting the committed source', () => {
  state.chapter.hasDraft = true; state.failure = 0; state.text = 'Tôi có ký tên, nhưng chưa bao giờ đồng ý để người khác nói thay mình.'
  const html = renderToStaticMarkup(<MobileReader project="p" story="s" chapter={9} />)
  expect(state.source).toBe('draft')
  expect(html).toContain(state.text)
  expect(html).not.toContain('Thử lại')
})

it('desktop opens the unfinished chapter draft and uses its existing empty state until text arrives', () => {
  const empty = renderToStaticMarkup(<ReaderScreen project="p" story="s" chapter={9} />)
  expect(state.source).toBe('draft')
  expect(empty).toContain('Chưa có bản nháp cho chương này')
  expect(empty).not.toContain('Chương 9 không tồn tại')
  state.chapter.hasDraft = true; state.failure = 0; state.text = 'Bản nháp đang viết dở.'
  const partial = renderToStaticMarkup(<ReaderScreen project="p" story="s" chapter={9} />)
  expect(state.source).toBe('draft')
  expect(partial).toContain(state.text)
  state.chapter.completed = true
  renderToStaticMarkup(<ReaderScreen project="p" story="s" chapter={9} />)
  expect(state.source).toBe('committed')
})

it('real reader failures and its retry/title/font actions never navigate to More', async () => {
  state.failure = 500
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  try {
    await act(async () => root.render(<MobileReader project="p" story="s" chapter={9} />))
    const button = (label: string) => [...container.querySelectorAll('button')].find((item) => item.textContent === label || item.getAttribute('aria-label') === label)!
    await act(async () => button('Thử lại').click())
    expect(state.reload).toHaveBeenCalledOnce()
    expect(state.navigate).not.toHaveBeenCalled()
    await act(async () => (container.querySelector('.mr-title') as HTMLButtonElement).click())
    expect(container.querySelector('[role="dialog"]')).not.toBeNull()
    expect(state.navigate).not.toHaveBeenCalled()
    await act(async () => button('Cỡ chữ và kiểu chữ').click())
    expect(state.navigate).not.toHaveBeenCalled()
    await act(async () => button('Nghe chương này').click())
    expect(state.navigate).toHaveBeenCalledWith('/p/p/s/s/audio/2/listen?chapter=9&play=1')
  } finally { await act(async () => root.unmount()); container.remove() }
})
