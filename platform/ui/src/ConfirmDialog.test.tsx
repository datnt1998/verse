// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ConfirmDialog, confirm, dismissConfirmations, setConfirmLayout, HOLD_DURATION, matchesConfirmation, scheduleConfirmationHold } from './ConfirmDialog'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []

afterEach(async () => {
  await act(async () => dismissConfirmations())
  setConfirmLayout('dialog')
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.useRealTimers()
  vi.unstubAllGlobals()
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

describe('confirmation lifecycle', () => {
  it('dismisses an idle desktop confirmation from its scrim and restores the existing scroll policy', async () => {
    vi.useFakeTimers()
    document.body.style.overflow = 'auto'
    const { props } = await mountDialog({ inline: false, phrase: undefined })
    const dialog = document.querySelector('[role="alertdialog"]')!
    expect(document.body.style.overflow).toBe('hidden')
    await act(async () => { (dialog.parentElement as HTMLElement).click(); vi.advanceTimersByTime(180) })
    expect(props.onClose).toHaveBeenCalledWith(false)
    expect(props.onConfirm).not.toHaveBeenCalled()
    for (const root of roots.splice(0)) await act(() => root.unmount())
    expect(document.body.style.overflow).toBe('auto')
    document.body.style.overflow = ''
  })

  it('keeps a submitted action visible and single-flight through Esc, scrim and cancel', async () => {
    vi.useFakeTimers()
    let finish!: () => void
    const action = vi.fn(() => new Promise<void>((resolve) => { finish = resolve }))
    const { props } = await mountDialog({ inline: false, phrase: undefined, onConfirm: action })
    const dialog = document.querySelector('[role="alertdialog"]')!
    const primary = confirmButton(dialog)
    await act(async () => { primary.click(); primary.click() })
    await act(async () => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
      ;(dialog.parentElement as HTMLElement).click()
      ;(dialog.querySelector('button') as HTMLButtonElement).click()
      vi.advanceTimersByTime(1000)
    })
    expect(action).toHaveBeenCalledTimes(1)
    expect(props.onClose).not.toHaveBeenCalled()
    await act(async () => finish())
    await act(async () => { vi.advanceTimersByTime(180) })
    expect(props.onClose).toHaveBeenCalledWith(true)
  })

  it('uses one mobile sheet, retains the confirmation result until exit, and releases its lock', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('matchMedia', (query: string) => ({ matches: !query.includes('reduced-motion'), addEventListener: vi.fn(), removeEventListener: vi.fn() }))
    setConfirmLayout('responsive')
    let result!: Promise<boolean>
    const action = vi.fn()
    await act(async () => {
      result = confirm({ title: 'Xoá video QA?', body: 'Xoá 3 ảnh · 12 MB.', confirmLabel: 'Xoá', onConfirm: action })
      await Promise.resolve()
    })
    const dialog = document.querySelector('[role="alertdialog"]')!
    expect(document.querySelectorAll('[role="alertdialog"]')).toHaveLength(1)
    expect(dialog.classList.contains('m-sheet')).toBe(true)
    await act(async () => confirmButton(dialog).click())
    expect(action).toHaveBeenCalledTimes(1)
    expect(document.body.style.overflow).toBe('hidden')
    await act(async () => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
      vi.advanceTimersByTime(260)
    })
    await expect(result).resolves.toBe(true)
    expect(document.querySelector('[role="alertdialog"]')).toBeNull()
    expect(document.body.style.overflow).toBe('')
  })

  it('does not open an asynchronously imported confirmation after its session has ended', async () => {
    const action = vi.fn()
    const result = confirm({ title: 'Xoá video QA?', body: 'Xoá 3 ảnh.', confirmLabel: 'Xoá', onConfirm: action })
    dismissConfirmations()
    await expect(result).resolves.toBe(false)
    expect(document.querySelector('[role="alertdialog"]')).toBeNull()
    expect(action).not.toHaveBeenCalled()
  })

  it('discards an open session confirmation without accepting its action', async () => {
    let result!: Promise<boolean>
    const action = vi.fn()
    await act(async () => {
      result = confirm({ title: 'Xoá video QA?', body: 'Xoá 3 ảnh.', confirmLabel: 'Xoá', onConfirm: action })
      await Promise.resolve()
    })
    expect(document.querySelector('[role="alertdialog"]')).not.toBeNull()
    await act(async () => dismissConfirmations())
    await expect(result).resolves.toBe(false)
    expect(document.querySelector('[role="alertdialog"]')).toBeNull()
    expect(action).not.toHaveBeenCalled()
    expect(document.body.style.overflow).toBe('')
  })

  it('retains the typed name and lets a failed mutation be retried without closing the form', async () => {
    const action = vi.fn().mockRejectedValueOnce(new Error('Máy chủ chưa xoá được.')).mockResolvedValueOnce(undefined)
    const { container, props } = await mountDialog({ onConfirm: action })
    await typePhrase(container, 'my-channel')
    await act(async () => confirmButton(container).click())
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('Máy chủ chưa xoá được.')
    expect(container.querySelector('input')?.value).toBe('my-channel')
    expect(props.onClose).not.toHaveBeenCalled()
    await act(async () => confirmButton(container).click())
    expect(action).toHaveBeenCalledTimes(2)
  })
})
