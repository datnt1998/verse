// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ConfirmDialog, HOLD_DURATION, matchesConfirmation, scheduleConfirmationHold } from './ConfirmDialog'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []

afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.useRealTimers()
})

async function mountDialog(props: Partial<Parameters<typeof ConfirmDialog>[0]> = {}) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  const defaults = { title: 'Xoá kênh?', body: 'Xoá vĩnh viễn kênh này.', tone: 'danger' as const, confirmLabel: 'Xoá', phrase: 'my-channel', onConfirm: vi.fn(), onClose: vi.fn() }
  await act(async () => { root.render(<ConfirmDialog inline {...defaults} {...props} />) })
  return { container, props: { ...defaults, ...props } }
}

/** Gõ cụm xác nhận như người dùng. Dùng setter của prototype để vượt tracker
 * value của React — `input.value =` trực tiếp không sinh onChange trong happy-dom. */
async function typePhrase(container: Element, phrase: string) {
  const input = container.querySelector('input') as HTMLInputElement
  const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(input), 'value')!.set!
  await act(async () => {
    setter.call(input, phrase)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
  return input
}

const confirmButton = (container: Element) => [...container.querySelectorAll('button')].at(-1)! as HTMLButtonElement

describe('gõ để xác nhận', () => {
  it('chỉ chấp nhận đúng tên thư mục, cho phép khoảng trắng ngoài', () => {
    expect(matchesConfirmation('01-tra-xanh', '01-tra-xanh')).toBe(true)
    expect(matchesConfirmation(' 01-tra-xanh ', '01-tra-xanh')).toBe(true)
    expect(matchesConfirmation('01-tra-xan', '01-tra-xanh')).toBe(false)
    expect(matchesConfirmation('01-TRA-XANH', '01-tra-xanh')).toBe(false)
    expect(matchesConfirmation('', '01-tra-xanh')).toBe(false)
  })
  it('nút xoá bị khoá khi chưa gõ, hộp được gắn nhãn và mô tả', () => {
    const html = renderToStaticMarkup(<ConfirmDialog inline title="Xoá truyện?" body="Xoá vĩnh viễn 24 chương." tone="danger" confirmLabel="Xoá vĩnh viễn" phrase="01-tra-xanh" onConfirm={() => {}} onClose={() => {}} />)
    expect(html).toContain('role="alertdialog"')
    expect(html).toContain('aria-labelledby=')
    expect(html).toContain('aria-describedby=')
    expect(html).toContain('disabled=""')
    expect(html).toMatch(/Type <span class="mono"[^>]*>01-tra-xanh<\/span> to confirm/)
  })
})

describe('giữ để xác nhận', () => {
  it('không chạy ở 1199ms, chạy đúng một lần tại 1200ms', () => {
    vi.useFakeTimers()
    const complete = vi.fn()
    scheduleConfirmationHold(complete)
    vi.advanceTimersByTime(1199)
    expect(complete).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(complete).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(HOLD_DURATION * 2)
    expect(complete).toHaveBeenCalledTimes(1)
  })
  it('nhả sớm huỷ toàn bộ thời gian giữ, lần giữ sau bắt đầu lại', () => {
    vi.useFakeTimers()
    const complete = vi.fn()
    const release = scheduleConfirmationHold(complete)
    vi.advanceTimersByTime(900)
    release()
    vi.advanceTimersByTime(1000)
    expect(complete).not.toHaveBeenCalled()
    const cancel = scheduleConfirmationHold(complete)
    vi.advanceTimersByTime(1199)
    expect(complete).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(complete).toHaveBeenCalledTimes(1)
    cancel()
  })
})

describe('disabledReason — khoá có lý do', () => {
  it('hiện lý do và nút phá hoại disabled dù gõ đúng cụm', async () => {
    const { container, props } = await mountDialog({ disabledReason: 'Kênh còn 2 video đang chạy.' })
    expect(container.textContent).toContain('Kênh còn 2 video đang chạy.')
    await typePhrase(container, 'my-channel')
    const button = confirmButton(container)
    expect(button.disabled).toBe(true)
    await act(async () => { button.click() })
    expect(props.onConfirm).not.toHaveBeenCalled()
  })
  it('giữ để xác nhận không bắt đầu và không gọi onConfirm', async () => {
    vi.useFakeTimers()
    const { container, props } = await mountDialog({ phrase: undefined, hold: true, disabledReason: 'Đang chờ hoàn tất.' })
    const button = confirmButton(container)
    await act(async () => { button.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0 })) })
    await act(async () => { vi.advanceTimersByTime(HOLD_DURATION * 2) })
    expect(props.onConfirm).not.toHaveBeenCalled()
    await act(async () => { button.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true })) })
    await act(async () => { vi.advanceTimersByTime(HOLD_DURATION * 2) })
    expect(props.onConfirm).not.toHaveBeenCalled()
  })
  it('Enter trong ô gõ và Enter trên dialog đều không kích hoạt', async () => {
    const { container, props } = await mountDialog({ disabledReason: 'Không đủ quyền.' })
    const input = await typePhrase(container, 'my-channel')
    await act(async () => { input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })) })
    expect(props.onConfirm).not.toHaveBeenCalled()
    const dialog = container.querySelector('[role="alertdialog"]')!
    await act(async () => { dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })) })
    expect(props.onConfirm).not.toHaveBeenCalled()
  })
  it('không có disabledReason thì vẫn chạy bình thường (idle path giữ nguyên)', async () => {
    const { container, props } = await mountDialog()
    await typePhrase(container, 'my-channel')
    const button = confirmButton(container)
    expect(button.disabled).toBe(false)
    await act(async () => { button.click() })
    expect(props.onConfirm).toHaveBeenCalledTimes(1)
  })
})
