// @vitest-environment happy-dom
import { act, useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { RouteStage } from './RouteStage'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

it('restores visited route state without skeletons or fade and pauses hidden effects', async () => {
  const host = document.createElement('div')
  document.body.append(host)
  const root = createRoot(host)
  const effects = vi.fn()
  const cleanup = vi.fn()
  const animate = vi.fn()
  const original = HTMLElement.prototype.animate
  HTMLElement.prototype.animate = animate
  function Pane({ name }: { name: string }) {
    const [count, setCount] = useState(0)
    useEffect(() => { effects(name); return () => cleanup(name) }, [name])
    return <button onClick={() => setCount(count + 1)}>{name}: {count} — a fully loaded route with enough content for rendering</button>
  }
  try {
    await act(async () => root.render(<RouteStage routeKey="workspace:p:s"><Pane name="work" /></RouteStage>))
    await act(async () => host.querySelector('button')!.click())
    await act(async () => root.render(<RouteStage routeKey="project:p:"><Pane name="project" /></RouteStage>))
    expect(cleanup).toHaveBeenCalledWith('work')
    await act(async () => root.render(<RouteStage routeKey="workspace:p:s"><Pane name="work" /></RouteStage>))
    const shown = [...host.querySelectorAll('button')].find(button => button.style.display !== 'none' && button.textContent?.startsWith('work'))
    expect(shown?.textContent).toContain('work: 1')
    expect(host.querySelector('[aria-busy="true"]')).toBeNull()
    expect(animate).not.toHaveBeenCalled()
    expect(effects.mock.calls.filter(([name]) => name === 'work')).toHaveLength(2)
  } finally {
    await act(async () => root.unmount())
    host.remove()
    HTMLElement.prototype.animate = original
  }
})
