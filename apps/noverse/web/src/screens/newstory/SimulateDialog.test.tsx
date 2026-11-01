// @vitest-environment happy-dom
// Gói AR · việc 6 — hộp thoại máy tính “Mô phỏng văn phong”: nút chính chỉ bấm được khi
// đã dán ít nhất một đoạn, và gửi đúng các đoạn đã dán cho luồng simulateStart.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
vi.mock('../../ui/useUnsavedGuard', () => ({ useUnsavedGuard: vi.fn() }))
import { SimulateDialog } from './SimulateDialog'

let root: Root | null = null
const A = 'Mẹ tôi khâu áo bằng tay trái. Tôi chưa bao giờ hỏi vì sao.'
const B = 'Đêm bà mất, cuốn sổ ghi nợ vẫn nằm dưới gối.'

const buttons = () => [...document.querySelectorAll<HTMLButtonElement>('button')]
const button = (label: string) => {
  const found = buttons().find((el) => el.textContent === label)
  if (!found) throw new Error(`Thiếu nút ${label}`)
  return found
}
const areas = () => [...document.querySelectorAll<HTMLTextAreaElement>('textarea')]
const paste = async (index: number, value: string) => {
  const area = areas()[index]
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(area, value)
    area.dispatchEvent(new Event('input', { bubbles: true }))
  })
}

async function mount(props: { error?: string | null } = {}) {
  const onSubmit = vi.fn()
  const host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(async () => root!.render(<SimulateDialog onClose={vi.fn()} onSubmit={onSubmit} busy={false} error={props.error ?? null} />))
  return onSubmit
}

afterEach(async () => {
  if (root) await act(() => root!.unmount())
  document.body.replaceChildren()
  root = null
})

describe('AR · hộp thoại Mô phỏng văn phong (máy tính)', () => {
  it('keeps the start button disabled until a paragraph is pasted', async () => {
    const onSubmit = await mount()
    const start = button('Tạo truyện & học văn phong')
    expect(start.disabled).toBe(true)
    await act(() => start.click())
    expect(onSubmit).not.toHaveBeenCalled()

    await paste(0, A)
    expect(button('Tạo truyện & học văn phong').disabled).toBe(false)
    expect(document.body.textContent).toContain('Văn mẫu 1 · 14 chữ')
    expect(document.body.textContent).toContain('1/20')
  })

  it('hands both pasted paragraphs to the shared simulate flow', async () => {
    const onSubmit = await mount()
    await act(() => button('Thêm đoạn').click())
    expect(areas()).toHaveLength(2)
    await paste(0, A)
    await paste(1, B)
    expect(document.body.textContent).toContain('2/20')
    await act(() => button('Tạo truyện & học văn phong').click())
    expect(onSubmit).toHaveBeenCalledWith([A, B])
  })

  it('turns the primary button into a retry after a failure, keeping the pasted paragraphs', async () => {
    await mount({ error: 'Chưa có văn mẫu nào để phân tích' })
    expect(document.body.textContent).toContain('Khởi tạo thất bại')
    expect(document.body.textContent).toContain('Chưa có văn mẫu nào để phân tích')
    expect(button('Thử lại').disabled).toBe(true)
    await paste(0, A)
    expect(button('Thử lại').disabled).toBe(false)
  })
})
