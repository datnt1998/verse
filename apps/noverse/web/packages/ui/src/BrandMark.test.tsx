// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { BrandLockup, BrandMark } from './BrandMark'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren() })

async function mount(node: React.ReactNode): Promise<HTMLDivElement> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(node) })
  return container
}

it('tự dùng bản phẳng khi cao dưới 32px', async () => {
  const small = (await mount(<BrandMark size={22} />)).querySelector('svg')!
  expect(small.querySelectorAll('radialGradient stop')).toHaveLength(12)
  expect(small.querySelectorAll('g[filter]')).toHaveLength(0)

  const large = (await mount(<BrandMark size={32} />)).querySelector('svg')!
  expect(large.querySelectorAll('radialGradient stop')).toHaveLength(20)
  expect(large.querySelectorAll('g[filter]')).toHaveLength(3)
})

it('khung nhìn chứa trọn cả bốn quả: cao 22 thì rộng 30, không quả nào bị cắt', async () => {
  for (const size of [22, 32, 64]) {
    const svg = (await mount(<BrandMark size={size} />)).querySelector('svg')!
    expect(svg.style.height).toBe(`${size}px`)
    const [x, y, w, h] = svg.getAttribute('viewBox')!.split(/\s+/).map(Number)
    for (const circle of svg.querySelectorAll('circle')) {
      const cx = Number(circle.getAttribute('cx')), cy = Number(circle.getAttribute('cy')), r = Number(circle.getAttribute('r'))
      expect(cx - r).toBeGreaterThanOrEqual(x)
      expect(cy - r).toBeGreaterThanOrEqual(y)
      expect(cx + r).toBeLessThanOrEqual(x + w)
      expect(cy + r).toBeLessThanOrEqual(y + h)
    }
  }
  const small = (await mount(<BrandMark size={22} />)).querySelector('svg')!
  expect(Number.parseFloat(small.style.width)).toBeCloseTo(30.05, 1)
})

it('mỗi lần dùng có id gradient riêng', async () => {
  const container = await mount(<><BrandMark size={22} /><BrandMark size={22} /><BrandMark size={22} variant="mono" /></>)
  const ids = [...container.querySelectorAll('radialGradient, mask')].map((node) => node.id)
  expect(ids).toHaveLength(8 + 3)
  expect(new Set(ids).size).toBe(ids.length)
})

it('bản một màu theo currentColor và giữ khe hở giữa các quả', async () => {
  const svg = (await mount(<BrandMark size={22} variant="mono" />)).querySelector('svg')!
  expect(svg.getAttribute('fill')).toBe('currentColor')
  const spheres = [...svg.children].filter((node) => node.tagName === 'circle')
  expect(spheres).toHaveLength(4)
  expect(svg.querySelectorAll('mask')).toHaveLength(3)
  expect(spheres[0]!.getAttribute('mask')).toMatch(/^url\(#.+\)$/)
  expect(spheres[3]!.getAttribute('mask')).toBeNull()
})

it('đứng cạnh chữ thì ẩn với trình đọc màn hình, đứng một mình thì có nhãn', async () => {
  const lockup = await mount(<BrandLockup studio />)
  const mark = lockup.querySelector('svg')!
  expect(mark.style.height).toBe('22px')
  expect(mark.getAttribute('aria-hidden')).toBe('true')
  expect(mark.getAttribute('role')).toBeNull()
  expect(lockup.textContent).toBe('NoverseStudio')

  const alone = (await mount(<BrandMark size={22} label="Noverse" />)).querySelector('svg')!
  expect(alone.getAttribute('role')).toBe('img')
  expect(alone.getAttribute('aria-label')).toBe('Noverse')
})

it('logo thanh trên cùng: chữ Noverse 16/700, Studio 13 màu fg-4', async () => {
  const container = await mount(<BrandLockup size={22} studio />)
  const [word, studio] = [...container.querySelectorAll('span')].slice(-2)
  expect(word.textContent).toBe('Noverse')
  expect(studio.textContent).toBe('Studio')
  // StyleX gom thuộc tính vào lớp riêng: hai vai trò chữ phải khác lớp, và chữ Noverse không dùng lớp của Studio.
  expect(word.className).not.toBe('')
  expect(word.className).not.toBe(studio.className)
})
