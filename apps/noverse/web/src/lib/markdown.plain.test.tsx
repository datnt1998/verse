// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { ChapterContent, parseChapterText } from './markdown'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren() })

const plain = (value: string) => (value.includes('Nhược Vy') ? <mark data-ent="nv">{value}</mark> : value)

it('bọc tên nhân vật do Trình đọc chỉ định mà không mất in đậm/in nghiêng của markdown', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => {
    root.render(<ChapterContent blocks={parseChapterText('**Nhược Vy** cười.\n\n> Tri Hạ gọi “cô”.')} unmatchedRevisions={[]} showDiff={false} plain={plain} />)
  })
  expect(container.querySelectorAll('mark[data-ent="nv"]').length).toBe(1)
  expect(container.querySelector('strong mark[data-ent="nv"]')?.textContent).toBe('Nhược Vy')
  expect(container.querySelector('blockquote')?.textContent).toBe('Tri Hạ gọi “cô”.')
})
