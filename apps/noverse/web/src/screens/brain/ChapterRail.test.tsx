// @vitest-environment happy-dom
// Thanh chương (rail-v2): đường nối tô tới chương đang viết, không chạy theo chương đang xem.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import type { Brain } from '../../api/types'
import { ChapterRail } from './ChapterRail'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | null = null
let container: HTMLElement | null = null

afterEach(() => {
  act(() => root?.unmount())
  container?.remove()
  root = null
  container = null
})

const brain: Brain = {
  chapters: Array.from({ length: 8 }, (_, index) => ({
    n: index + 1,
    title: `T${index + 1}`,
    status: index < 3 ? 'written' : index === 3 ? 'rewriting' : 'unwritten',
  })),
  entities: [],
  edges: [],
}

it('xem chương 7 thì đường nối vẫn dừng ở chương 4 đang viết lại; nấc 7 được chọn', () => {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  act(() => root!.render(<ChapterRail brain={brain} chapter={7} onChapter={() => {}} />))
  const fill = container.querySelector<HTMLElement>('.brain-tlf')!
  expect(fill.style.width).toBe(`${3 / 7 * 100}%`)
  const pressed = container.querySelector('.tk[aria-pressed="true"]')!
  expect(pressed.getAttribute('title')).toBe('Chương 7 · T7 · chưa viết')
  expect(pressed.className).toBe('tk todo')
})
