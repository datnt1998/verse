// @vitest-environment happy-dom
// Gói S · Dự án › Nhân vật: “Loại vai” + “Lộ vai ở chương” lưu qua PUT /characters.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { Character } from '../../../api/types'
import { putCharacters } from '../../../api/client'
import { invalidate } from '../../../data'
import { toast } from '@noverse/ui'
import { KindEditor } from './KindEditor'

vi.mock('../../../api/client', () => ({ putCharacters: vi.fn(async () => ({ characters: [], supporting: [] })) }))
vi.mock('../../../data', () => ({
  invalidate: vi.fn(),
  keys: { story: (project: string, story: string, part: string) => ['story', project, story, part] },
}))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  toast: { ok: vi.fn(), err: vi.fn() } }))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | undefined
let host: HTMLDivElement | undefined

const character: Character = {
  name: 'Phùng Chấn', role: 'Đồng phạm', description: '', arc: '', traits: [], kind: 'antagonist', revealAt: null,
}

const render = async (who: Character = character) => {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(() => root!.render(<KindEditor project="du-an" story="01-truyen" character={who} />))
  return host
}

const saveButton = () => [...host!.querySelectorAll('button')].find((button) => button.textContent?.includes('Lưu')) as HTMLButtonElement
const chapterInput = () => host!.querySelector('input[aria-label="Lộ vai ở chương"]') as HTMLInputElement

beforeEach(() => vi.clearAllMocks())
afterEach(async () => {
  await act(() => root?.unmount())
  host?.remove()
  root = undefined
  host = undefined
})

it('chưa đổi gì thì nút Lưu tắt', async () => {
  await render()
  expect(saveButton().disabled).toBe(true)
})

it('lưu gửi đúng tên, loại vai và chương lộ vai rồi đọc lại dữ liệu', async () => {
  await render()
  await act(() => {
    const input = chapterInput()
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, '7')
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
  expect(saveButton().disabled).toBe(false)
  await act(async () => { saveButton().click() })
  expect(putCharacters).toHaveBeenCalledWith('du-an', '01-truyen', [
    { name: 'Phùng Chấn', kind: 'antagonist', revealAt: 7 },
  ])
  expect(invalidate).toHaveBeenCalledWith(['story', 'du-an', '01-truyen', 'characters'])
  expect(invalidate).toHaveBeenCalledWith(['story', 'du-an', '01-truyen', 'brain'])
  expect(invalidate).toHaveBeenCalledWith(['story', 'du-an', '01-truyen', 'snapshot'])
  expect(toast.ok).toHaveBeenCalled()
})

it('xoá ô chương thì lưu null (không còn bí mật)', async () => {
  await render({ ...character, revealAt: 5 })
  // Đang có bí mật ở chương 5; xoá trắng = gửi null.
  await act(() => {
    const input = chapterInput()
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, '')
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
  expect(saveButton().disabled).toBe(false)
  await act(async () => { saveButton().click() })
  expect(putCharacters).toHaveBeenCalledWith('du-an', '01-truyen', [
    { name: 'Phùng Chấn', kind: 'antagonist', revealAt: null },
  ])
})

it('chương 0 là không hợp lệ: báo lỗi và không lưu', async () => {
  await render()
  await act(() => {
    const input = chapterInput()
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, '0')
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
  expect(chapterInput().getAttribute('aria-invalid')).toBe('true')
  expect(saveButton().disabled).toBe(true)
  expect(putCharacters).not.toHaveBeenCalled()
})
