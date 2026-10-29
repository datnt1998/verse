// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { ProgressRing } from './ProgressRing'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | undefined
let host: HTMLDivElement

afterEach(async () => {
  await act(() => root?.unmount())
  host?.remove()
  vi.unstubAllGlobals()
})

it('hides empty arcs without a round dot', async () => {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(() => root!.render(<ProgressRing size={96} percent={0} />))
  expect(host.querySelector('[data-progress-arc]')!.getAttribute('visibility')).toBe('hidden')
})

it('normalizes known progress and closes the completed ring with unique gradients', async () => {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(() => root!.render(<>{[0, 45, 100].map((percent) => <ProgressRing key={percent} size={96} percent={percent} gradient={['var(--ring-voice-start)', 'var(--color-accent)']}>{percent}%</ProgressRing>)}</>))
  const arcs = host.querySelectorAll('[data-progress-arc]')
  expect(arcs[1].getAttribute('stroke-dasharray')).toBe('45 100')
  expect(arcs[2].getAttribute('stroke-dasharray')).toBe('100 100')
  expect(arcs[2].getAttribute('visibility')).toBeNull()
  expect(arcs[1].getAttribute('pathLength')).toBe('100')
  expect(arcs[1].getAttribute('stroke-linecap')).toBe('round')
  const ids = [...host.querySelectorAll('linearGradient')].map((gradient) => gradient.id)
  expect(new Set(ids).size).toBe(3)
  expect(arcs[1].getAttribute('stroke')).toBe(`url(#${ids[1]})`)
})

it('keeps a 25% indeterminate arc and removes spin when reduced motion changes', async () => {
  let reduced = false
  const listeners = new Set<() => void>()
  vi.stubGlobal('matchMedia', () => ({ matches: reduced, addEventListener: (_: string, listener: () => void) => listeners.add(listener), removeEventListener: (_: string, listener: () => void) => listeners.delete(listener) }))
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(() => root!.render(<ProgressRing size={38} percent={0} indeterminate />))
  const svg = host.querySelector('svg')!
  const runningClass = svg.getAttribute('class')
  expect(host.querySelector('[data-progress-arc]')!.getAttribute('stroke-dasharray')).toBe('25 100')
  reduced = true
  await act(() => { for (const listener of listeners) listener() })
  expect(svg.getAttribute('class')).not.toBe(runningClass)
  expect(host.querySelector('[data-progress-arc]')!.getAttribute('stroke-dasharray')).toBe('25 100')
  reduced = false
  await act(() => { for (const listener of listeners) listener() })
  expect(svg.getAttribute('class')).toBe(runningClass)
})
