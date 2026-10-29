// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { denseBrainFixture } from '../../../mock/brainScale'
import { StoryGraph } from './StoryGraph'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
beforeAll(() => {
  // Worker transport is a browser boundary; force physics is exercised separately.
  vi.stubGlobal('Worker', class { postMessage = vi.fn(); terminate = vi.fn() })
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
    measureText: (text: string) => ({ width: text.length * 6 }),
  } as CanvasRenderingContext2D)
  window.matchMedia = vi.fn().mockImplementation((media: string) => ({
    matches: media.includes('reduced-motion'), media, addEventListener() {}, removeEventListener() {},
  }))
})
afterEach(() => { document.body.innerHTML = '' })

describe('StoryGraph shared labels', () => {
  it.each([false, true])('merges repeated pair labels with a count (compact=%s)', async (compact) => {
    const brain = denseBrainFixture()
    brain.entities = brain.entities.slice(0, 2)
    brain.edges = Array.from({ length: 4 }, (_, index) => ({
      a: brain.entities[index % 2].id, b: brain.entities[1 - index % 2].id,
      phases: [{ fromChapter: 1, type: 'rel', label: 'Liên quan' }],
    }))
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    try {
      await act(async () => { root.render(<StoryGraph brain={brain} chapter={1} selected={brain.entities[0].id} onSelect={() => {}} compact={compact} />) })
      const labels = [...container.querySelectorAll('.bl')].filter((label) => label.getAttribute('opacity') === '1')
      expect(labels.map((label) => label.querySelector('title')?.textContent)).toEqual(['Liên quan ×4'])
      expect(labels[0].querySelector('text')?.textContent).toBe('Liên quan ×4')
    } finally {
      await act(async () => { root.unmount() })
    }
  })

  it.each([false, true])('keeps all 125 cards and 150 edges collision-free (compact=%s)', async (compact) => {
    const brain = denseBrainFixture()
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    try {
      await act(async () => { root.render(<StoryGraph brain={brain} chapter={1} selected={brain.entities[0].id} onSelect={() => {}} compact={compact} />) })
      expect(container.querySelectorAll('.bn')).toHaveLength(125)
      expect(container.querySelectorAll('linearGradient[gradientUnits="userSpaceOnUse"]')).toHaveLength(150)
      const cards = [...container.querySelectorAll<SVGRectElement>('.bn-body > rect:nth-child(2)')]
        .map((rect) => {
          const [x, y] = rect.closest('.bn')!.getAttribute('transform')!.match(/-?\d+(?:\.\d+)?/g)!.map(Number)
          return {
            x: x + Number(rect.getAttribute('x')), y: y + Number(rect.getAttribute('y')),
            w: Number(rect.getAttribute('width')), h: Number(rect.getAttribute('height')),
          }
        })
      const labels = [...container.querySelectorAll<SVGGElement>('.bl[data-label-rect]')].map((label) => {
        const [x, y, x1, y1] = label.dataset.labelRect!.split(' ').map(Number)
        return { x, y, w: x1 - x, h: y1 - y }
      })
      const rects = [...cards, ...labels]
      expect(rects.length).toBeGreaterThan(125)
      let intersections = 0
      rects.forEach((rect, index) => {
        for (const other of rects.slice(index + 1)) {
          if (Math.min(rect.x + rect.w, other.x + other.w) > Math.max(rect.x, other.x)
            && Math.min(rect.y + rect.h, other.y + other.h) > Math.max(rect.y, other.y)) intersections++
        }
      })
      expect(intersections).toBe(0)
      expect(Number(container.querySelector('svg')!.getAttribute('viewBox')!.split(' ')[3])).toBeGreaterThan(690)
    } finally {
      await act(async () => { root.unmount() })
    }
  })

  it('hover highlights exactly the visible neighbours and keeps a keyboard node list', async () => {
    const brain = denseBrainFixture()
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const select = vi.fn()
    try {
      await act(async () => { root.render(<StoryGraph brain={brain} chapter={1} selected={brain.entities[0].id} onSelect={select} />) })
      const leaf = container.querySelector<SVGGElement>(`.bn[data-id="${brain.entities[1].id}"]`)!
      await act(async () => { leaf.dispatchEvent(new MouseEvent('mouseover', { bubbles: true })) })
      expect([...container.querySelectorAll('.bn[data-highlighted="true"]')].map((node) => node.getAttribute('data-id')).sort())
        .toEqual([brain.entities[0].id, brain.entities[1].id].sort())
      const buttons = [...container.querySelectorAll<HTMLButtonElement>('ul button')]
      expect(buttons).toHaveLength(125)
      await act(async () => { buttons[0].focus(); buttons[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true })) })
      expect(document.activeElement).toBe(buttons[1])
      await act(async () => { buttons[1].click() })
      expect(select).toHaveBeenCalledWith(brain.entities[1].id)
    } finally {
      await act(async () => { root.unmount() })
    }
  })
})
