// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, listStories } from '../api/client'
import type * as Client from '../api/client'
import { cache, keys } from '../data'
import { helpTarget, rememberHelpTarget, subscribeHelpTarget, verifyTarget } from './target'

vi.mock('../api/client', async (original) => ({ ...(await original<typeof Client>()), listStories: vi.fn() }))

const stories = (...names: string[]) => ({ stories: names.map((story) => ({ id: `p/${story}`, project: 'p', story })) }) as never

beforeEach(() => {
  sessionStorage.clear()
  cache.clear()
  vi.mocked(listStories).mockReset()
})

describe('rememberHelpTarget (E2EM-20)', () => {
  it('keeps a story that the library lists', async () => {
    vi.mocked(listStories).mockResolvedValue(stories('real'))
    rememberHelpTarget('#/p/p/s/real/work')
    await verifyTarget({ project: 'p', story: 'real' })
    expect(helpTarget()).toEqual({ project: 'p', story: 'real' })
  })

  it('drops a story-shaped hash the library does not have and tells the open screens', async () => {
    vi.mocked(listStories).mockResolvedValue(stories('real'))
    const seen = vi.fn()
    const stop = subscribeHelpTarget(seen)
    rememberHelpTarget('#/p/p/s/m2-reader-locked/read/24')
    await vi.waitFor(() => expect(seen).toHaveBeenCalled())
    expect(helpTarget()).toBeNull()
    expect(sessionStorage.getItem('ainovel.commands.target')).toBeNull()
    stop()
  })

  it('does not overwrite a valid target with a story the cached library lacks', () => {
    cache.setData(keys.project('p', 'stories'), stories('real'))
    rememberHelpTarget('#/p/p/s/real/work')
    rememberHelpTarget('#/p/p/s/ghost/work')
    expect(helpTarget()).toEqual({ project: 'p', story: 'real' })
  })

  it('treats a missing project as no story, but keeps the target when the server cannot answer', async () => {
    vi.mocked(listStories).mockRejectedValueOnce(new ApiError(404, 'không tìm thấy'))
    expect(await verifyTarget({ project: 'gone', story: 's' })).toBe(false)
    vi.mocked(listStories).mockRejectedValueOnce(new ApiError(503, 'bận'))
    expect(await verifyTarget({ project: 'busy', story: 's' })).toBe(true)
  })

  it('purges an already stored stale target when the cached library shows it is gone', () => {
    sessionStorage.setItem('ainovel.commands.target', JSON.stringify({ project: 'p', story: 'old' }))
    cache.setData(keys.project('p', 'stories'), stories('real'))
    expect(helpTarget()).toBeNull()
    expect(sessionStorage.getItem('ainovel.commands.target')).toBeNull()
  })
})
