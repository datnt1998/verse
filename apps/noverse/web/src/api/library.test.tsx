// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { getLibrary, getProject } from './client'
import type { LibraryResponse, ProjectDetail } from './types'
import { LibraryProvider, useLibrary } from './library'

vi.mock('./client', () => ({ getLibrary: vi.fn(), getProject: vi.fn() }))
vi.mock('./session', () => ({ useSession: () => ({ user: admin }) }))
// Captures the provider's subscriptions so the test can replay a server event.
const bus = vi.hoisted(() => ({
  handlers: new Map<string, (data: unknown, meta: { type: string }) => void>(),
  reconnect: undefined as (() => void) | undefined,
}))
vi.mock('../data/events', () => ({
  subscribeEvents: (type: string | readonly string[], handler: (data: unknown, meta: { type: string }) => void) => {
    const types = typeof type === 'string' ? [type] : type
    types.forEach((name) => bus.handlers.set(name, handler))
    return () => { types.forEach((name) => bus.handlers.delete(name)) }
  },
  events: { onReconnect: (fn: () => void) => { bus.reconnect = fn; return () => { bus.reconnect = undefined } } },
}))
const admin = { username: 'admin', role: 'admin' }
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren(); vi.clearAllMocks() })

it('publishes lazily loaded project details to existing consumers', async () => {
  vi.mocked(getLibrary).mockResolvedValue({ projects: [] } as unknown as LibraryResponse)
  vi.mocked(getProject).mockResolvedValue({ stories: [{ story: '01-story' }] } as ProjectDetail)
  function Consumer() {
    const library = useLibrary()
    return <button onClick={() => library.ensureProject('proj')}>{library.projectDetail('proj')?.stories.length ?? 0}</button>
  }
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(<LibraryProvider><Consumer /></LibraryProvider>) })
  expect(container.textContent).toBe('0')
  await act(async () => { container.querySelector('button')!.click() })
  expect(getProject).toHaveBeenCalledWith('proj')
  expect(container.textContent).toBe('1')
})

it('updates visible projects after a library change or missed events', async () => {
  let projectIds = ['a']
  vi.mocked(getLibrary).mockImplementation(async () => ({
    projects: projectIds.map((id) => ({ id })),
  }) as LibraryResponse)
  function ProjectList() {
    return <output>{useLibrary().library?.projects.map((project) => project.id).join(',')}</output>
  }
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(<LibraryProvider><ProjectList /></LibraryProvider>) })
  expect(container.textContent).toBe('a')

  vi.useFakeTimers()
  try {
    projectIds = ['a', 'b']
    await act(async () => {
      bus.handlers.get('library.changed')?.({}, { type: 'library.changed' })
      await vi.advanceTimersByTimeAsync(250)
    })
    expect(container.textContent).toBe('a,b')

    projectIds = ['b']
    await act(async () => {
      bus.reconnect?.()
      await vi.advanceTimersByTimeAsync(250)
    })
    expect(container.textContent).toBe('b')

    projectIds = ['b', 'c']
    await act(async () => {
      window.dispatchEvent(new Event('focus'))
      await vi.advanceTimersByTimeAsync(250)
    })
    expect(container.textContent).toBe('b,c')
  } finally {
    vi.useRealTimers()
  }
})
