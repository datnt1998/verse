// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { cache, useResource } from '../data'
import { PanelRefreshBar } from './PanelRefreshBar'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
afterEach(() => { cache.clear(); vi.useRealTimers() })
it('background refresh renders only a delayed absolute overlay and keeps content untouched', async () => {
  vi.useFakeTimers()
  const key = ['refresh-overlay', 'panel']
  cache.setData(key, { title: 'unchanged' })
  let finish!: (value: { title: string }) => void
  const pending = new Promise<{ title: string }>((resolve) => { finish = resolve })
  let renders = 0
  function Panel() {
    renders++
    const resource = useResource(key, () => pending, { staleTime: Infinity })
    return <section style={{ position: 'relative' }}><PanelRefreshBar resourceKeys={[key]} /><p>{resource.data?.title}</p></section>
  }
  const host = document.createElement('div'); document.body.append(host)
  const root = createRoot(host)
  try {
    await act(async () => root.render(<Panel />))
    const baseline = renders
    const content = host.querySelector('p')
    const overlay = host.querySelector<HTMLElement>('[data-panel-refresh]')!
    expect(overlay.style.position).toBe('absolute')
    expect(overlay.style.height).toBe('2px')
    let request!: Promise<unknown>
    await act(async () => { request = cache.refresh(key) })
    expect(renders).toBe(baseline)
    await act(async () => vi.advanceTimersByTime(299))
    expect(host.querySelector('[role=progressbar]')).toBeNull()
    await act(async () => vi.advanceTimersByTime(1))
    expect(host.querySelector('[role=progressbar]')).not.toBeNull()
    expect(host.querySelector('p')).toBe(content)
    await act(async () => { finish({ title: 'unchanged' }); await request })
    await act(async () => vi.advanceTimersByTime(399))
    expect(host.querySelector('[role=progressbar]')).not.toBeNull()
    await act(async () => vi.advanceTimersByTime(1))
    expect(host.querySelector('[role=progressbar]')).toBeNull()
    expect(renders).toBe(baseline)
    expect(host.querySelector('p')).toBe(content)
  } finally { await act(async () => root.unmount()); host.remove() }
})
it('a panel does not show progress for another panel resource', async () => {
  vi.useFakeTimers()
  const host = document.createElement('div'); const root = createRoot(host)
  cache.setData(['unrelated'], 'old')
  let finish!: (value: string) => void
  const release = cache.retain(['unrelated'], () => new Promise<string>((resolve) => { finish = resolve }), { staleTime: Infinity })
  try {
    await act(async () => root.render(<PanelRefreshBar resourceKeys={[[ 'own-panel' ]]} />))
    let request!: Promise<unknown>
    await act(async () => { request = cache.refresh(['unrelated']) })
    await act(async () => vi.advanceTimersByTime(1000))
    expect(host.querySelector('[role=progressbar]')).toBeNull()
    await act(async () => { finish('new'); await request })
  } finally { await act(async () => root.unmount()); release() }
})
