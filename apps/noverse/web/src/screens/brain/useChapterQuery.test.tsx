// @vitest-environment happy-dom
// Câu 20: đổi `?ch=` trong địa chỉ lúc Não truyện đang mở phải chọn lại chương.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import type { Brain } from '../../api/types'
import { useChapterQuery } from './useChapterQuery'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const chapters: Brain['chapters'] = Array.from({ length: 8 }, (_, index) => ({
  n: index + 1,
  title: `T${index + 1}`,
  status: index < 3 ? 'written' : index === 3 ? 'rewriting' : 'unwritten',
}))

let root: Root | null = null
let container: HTMLElement | null = null

afterEach(() => {
  act(() => root?.unmount())
  container?.remove()
  root = null
  container = null
  location.hash = ''
})

function Probe({ data }: { data: Brain['chapters'] }) {
  const [chapter, setChapter] = useChapterQuery(data)
  return <>
    <span data-testid="ch">{chapter ?? '-'}</span>
    <button type="button" onClick={() => setChapter(7)}>chọn 7</button>
  </>
}

function open(hash: string) {
  location.hash = hash
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  act(() => root!.render(<Probe data={chapters} />))
}

const shown = () => container!.querySelector('[data-testid="ch"]')!.textContent

/** Đổi địa chỉ như người dùng sửa hash: `useRoute` chỉ nghe `hashchange`. */
const goTo = (hash: string) => act(() => {
  location.hash = hash
  dispatchEvent(new HashChangeEvent('hashchange'))
})

it('mở Não truyện bằng `?ch=6` thì chọn chương 6', () => {
  open('#/p/p/s/s/brain?ch=6')
  expect(shown()).toBe('6')
})

it('đổi `?ch=` lúc màn đang mở thì chọn lại chương mới', () => {
  open('#/p/p/s/s/brain?ch=2')
  expect(shown()).toBe('2')
  goTo('#/p/p/s/s/brain?ch=6')
  expect(shown()).toBe('6')
  goTo('#/p/p/s/s/brain?ch=2')
  expect(shown()).toBe('2')
})

it('chọn tay trên thanh chương không bị `?ch=` cũ kéo lại', () => {
  open('#/p/p/s/s/brain?ch=2')
  act(() => container!.querySelector('button')!.click())
  expect(shown()).toBe('7')
  // Dữ liệu làm mới (dàn ý mới) không kéo người xem về chương trong địa chỉ.
  act(() => root!.render(<Probe data={[...chapters]} />))
  expect(shown()).toBe('7')
})

it('`?ch=` ngoài dàn ý thì bỏ qua, giữ chương đang xem', () => {
  open('#/p/p/s/s/brain?ch=2')
  goTo('#/p/p/s/s/brain?ch=30')
  expect(shown()).toBe('2')
})

it('không có `?ch=` thì mở chương đang viết', () => {
  open('#/p/p/s/s/brain')
  expect(shown()).toBe('4')
})
