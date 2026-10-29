// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { getLibrary, getProject } from './client'
import type { LibraryResponse, ProjectDetail } from './types'
import { LibraryProvider, useLibrary } from './library'

vi.mock('./client', () => ({ getLibrary: vi.fn(), getProject: vi.fn() }))
vi.mock('./session', () => ({ useSession: () => ({ user: admin }) }))
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
