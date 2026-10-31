// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ConfigResponse } from '../../api/types'
import { QuickMode } from './QuickMode'
import type { StoryCheck } from './useStoryCheck'

vi.mock('../../data', async importOriginal => ({
  ...await importOriginal<typeof import('../../data')>(),
  useResource: () => ({ data: { providers: [], styles: ['default', 'suspense'], reasoning: [] } }),
}))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | null = null
let container: HTMLElement | null = null
const gate: StoryCheck = {
  state: null, blocked: false, checking: false, suggesting: false,
  check: async () => true, acceptConflict: () => false, suggest: async () => {}, undo: () => {},
}
async function mount() {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  await act(async () => root!.render(<QuickMode project="p" projectName="Dự án" prompt="Một truyện mới" onPrompt={vi.fn()} onStart={vi.fn()}
    choices={{ style: null, writer: null }} onChoices={vi.fn()}
    config={{ effective: { style: 'default', provider: 'openai', model: 'gpt-6.1-sol' } } as ConfigResponse}
    busy={false} pending={false} success={false} alternativePending={false} activeAlternative={null} error={null} offline={false}
    idea={null} onClearIdea={vi.fn()} onStartFile={vi.fn()} onImport={vi.fn()} onSimulate={vi.fn()} gate={gate} />))
  return container
}
afterEach(async () => {
  if (root) await act(() => root!.unmount())
  container?.remove()
  root = null; container = null
})
describe('New-story quick pickers', () => {
  it('opens chips as buttons without changing the route and closes on Escape or outside pointer', async () => {
    const el = await mount()
    const chips = [...el.querySelectorAll<HTMLButtonElement>('button[aria-haspopup="listbox"]')]
    expect(chips).toHaveLength(2)
    expect(chips.every(chip => chip.tagName === 'BUTTON' && chip.type === 'button')).toBe(true)
    const hash = location.hash
    await act(() => chips[0].click())
    expect(el.querySelector('[role="listbox"]')).not.toBeNull()
    expect(location.hash).toBe(hash)
    await act(() => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })))
    expect(el.querySelector('[role="listbox"]')).toBeNull()
    await act(() => chips[1].click())
    expect(el.querySelector('[role="listbox"]')).not.toBeNull()
    await act(() => document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true })))
    expect(el.querySelector('[role="listbox"]')).toBeNull()
  })
})
