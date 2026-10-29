// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { WritePreview } from './WritePreview'
import type { Preview } from './model'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren() })

const preview: Preview = {
  title: 'Thiết lập sẽ lưu', file: '~/.ainovel/config.json', fileNote: 'toàn cục',
  plain: [{ k: 'Ngôn ngữ viết truyện', v: 'Tiếng Việt' }],
  lines: [{ pad: 12, segs: [{ text: '{', color: '#fff' }] }, { pad: 26, segs: [{ text: '"provider": "deepseek"', color: '#fff' }], hot: true }],
}

async function render(showTech: boolean) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(<WritePreview preview={preview} showTech={showTech} />) })
  return container
}

it('chỉ quản trị thấy “Tệp sẽ ghi”; người viết chỉ thấy bản bằng lời', async () => {
  const writer = await render(false)
  expect(writer.textContent).toContain('Tiếng Việt')
  expect(writer.textContent).not.toContain('Tệp sẽ ghi')
  expect(writer.textContent).not.toContain('config.json')
})

it('quản trị: thu gọn mặc định, mở ra thì thụt dòng theo pixel đúng pad và tô sáng dòng hot', async () => {
  const container = await render(true)
  const toggle = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('Tệp sẽ ghi (cho quản trị)'))!
  expect(toggle.getAttribute('aria-expanded')).toBe('false')
  expect(container.querySelector('.mono')).toBeNull()
  await act(async () => { toggle.click() })
  expect(toggle.getAttribute('aria-expanded')).toBe('true')
  const rows = [...container.querySelectorAll<HTMLElement>('.mono > div')]
  expect(rows.map((row) => row.style.paddingLeft)).toEqual(['12px', '26px'])
  expect(rows[1].style.background).toContain('rgba(201, 166, 245, 0.09)')
})
