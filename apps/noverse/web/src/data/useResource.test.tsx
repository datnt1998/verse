// @vitest-environment happy-dom
import { Activity, act } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it } from 'vitest'
import { setTimeout as delay } from 'node:timers/promises'
import { cache } from './cache'
import { useResource } from './useResource'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

it('keeps resolved content across dependent keys, refreshes and failed refreshes', async () => {
  const host = document.createElement('div')
  const root = createRoot(host)
  let resolve!: (data: string) => void
  let reject!: (error: Error) => void
  const fetcher = () => new Promise<string>((yes, no) => { resolve = yes; reject = no })
  function Probe({ id }: { id: number }) {
    const resource = useResource(['test', id], fetcher, { keepPrevious: true })
    return <output aria-busy={resource.initialLoading}>{resource.data ?? 'skeleton'}</output>
  }
  try {
    await act(async () => root.render(<Probe id={1} />))
    expect(host.textContent).toBe('skeleton')
    await act(async () => { resolve('first') })
    await act(async () => root.render(<Probe id={2} />))
    expect(host.textContent).toBe('first')
    expect(host.querySelector('[aria-busy="true"]')).toBeNull()
    await act(async () => { resolve('second') })
    expect(host.textContent).toBe('second')
    await act(async () => { void cache.refresh(['test', 2]) })
    expect(host.textContent).toBe('second')
    await act(async () => { reject(new Error('offline')) })
    expect(host.textContent).toBe('second')
    expect(host.querySelector('[aria-busy="true"]')).toBeNull()
  } finally {
    await act(async () => root.unmount())
    cache.clear()
  }
})

it('identical SSE data does not commit visible, unrelated, or Activity-hidden subscribers', async () => {
  const key = ['render-count', 'visible']
  const other = ['render-count', 'hidden']
  cache.setData(key, { title: 'unchanged' })
  cache.setData(other, { title: 'hidden' })
  const counts = { visible: 0, unrelated: 0, hidden: 0 }
  function Probe({ name, resourceKey }: { name: keyof typeof counts; resourceKey: string[] }) {
    useResource(resourceKey, async () => ({ title: 'unchanged' }), { staleTime: Infinity })
    counts[name]++
    return null
  }
  const host = document.createElement('div')
  const root = createRoot(host)
  try {
    await act(async () => root.render(<><Probe name="visible" resourceKey={key} /><Probe name="unrelated" resourceKey={['render-count', 'other']} /><Activity mode="hidden"><Probe name="hidden" resourceKey={other} /></Activity></>))
    const baseline = { ...counts }
    await delay(5)
    await act(async () => { cache.setData(key, { title: 'unchanged' }); cache.setData(other, { title: 'changed while hidden' }) })
    expect(counts).toEqual(baseline)
    await act(async () => { cache.setData(key, { title: 'changed' }) })
    expect(counts.visible).toBe(baseline.visible + 1)
    expect(counts.unrelated).toBe(baseline.unrelated)
    expect(counts.hidden).toBe(baseline.hidden)
  } finally { await act(async () => root.unmount()); cache.clear() }
})
