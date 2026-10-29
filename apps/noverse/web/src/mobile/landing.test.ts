// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { listStories } from '../api/client'
import type * as Client from '../api/client'
import { getLastStory, setLastStory } from '../api/storage'
import { cache } from '../data'
import { phoneLandingPath } from './landing'

vi.mock('../api/client', async (original) => ({ ...(await original<typeof Client>()), listStories: vi.fn() }))

beforeEach(() => {
  localStorage.clear(); sessionStorage.clear(); cache.clear()
  vi.mocked(listStories).mockReset()
})

describe('phoneLandingPath (E2EM-5)', () => {
  it('first login with no opened story stays on Thêm › Dự án', async () => {
    expect(await phoneLandingPath('viet')).toBeNull()
    expect(listStories).not.toHaveBeenCalled()
  })

  it('lands on Viết of the last opened story when the library still has it', async () => {
    setLastStory('viet', { project: 'p', story: 's' })
    vi.mocked(listStories).mockResolvedValue({ stories: [{ id: 'p/s', project: 'p', story: 's' }] } as never)
    expect(await phoneLandingPath('viet')).toBe('/p/p/s/s/work')
  })

  it('does not land on a story that was deleted', async () => {
    setLastStory('viet', { project: 'p', story: 'gone' })
    vi.mocked(listStories).mockResolvedValue({ stories: [] } as never)
    expect(await phoneLandingPath('viet')).toBeNull()
  })

  it('is per account and ignores malformed storage', () => {
    setLastStory('viet', { project: 'p', story: 's' })
    expect(getLastStory('lan')).toBeNull()
    localStorage.setItem('ainovel.lastStory.x', JSON.stringify({ project: 1 }))
    expect(getLastStory('x')).toBeNull()
  })
})
