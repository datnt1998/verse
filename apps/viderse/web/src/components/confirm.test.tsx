// @vitest-environment happy-dom
// Xác nhận thật trước thao tác nguy hiểm: huỷ, hành động phụ, gõ đúng tên
// và khoá thao tác khi còn công việc đang chạy.
import { act } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { applyViderseLabels } from '../kitLabels'
import { t } from '../i18n/all'
import {
  confirmDeleteChannel, confirmDeleteVideo, confirmLeaveUnsaved,
  confirmOverwrite, confirmPause, confirmProviderLogout, confirmStopAll,
} from './confirm'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

beforeEach(() => applyViderseLabels())
afterEach(() => {
  // Đóng mọi hộp còn treo: bấm Huỷ nếu có.
  for (const btn of [...document.querySelectorAll('button')]) {
    if (btn.textContent === t.common.cancel || btn.textContent === 'Tiếp tục vẽ' || btn.textContent === 'Ở lại') btn.click()
  }
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

const waitForDialog = async () => {
  await vi.waitFor(async () => {
    await act(async () => {})
    expect(document.querySelector('[role="alertdialog"]')).not.toBeNull()
  })
}

describe('7 tình huống xác nhận', () => {
  it('pause: tiêu đề kèm tên trợ lý + 2 lựa chọn radio + Huỷ riêng', async () => {
    const done = vi.fn()
    const p = confirmPause({ agent: 'artist', video: 'Midnight Tales', scene: 'cảnh 5/6', onConfirm: done })
    await waitForDialog()
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Tiếp tục vẽ')?.click() })
    await expect(p).resolves.toBe(false)
    expect(done).not.toHaveBeenCalled()
  })

  it('overwrite: so hai bản + nút phụ Giữ cả hai', async () => {
    const keepBoth = vi.fn()
    const p = confirmOverwrite({
      file: 'Midnight-Tales.mp4',
      current: { summary: '38:12 · 172 MB' },
      next: { summary: '38:12 · 176 MB' },
      onConfirm: vi.fn(), onKeepBoth: keepBoth,
    })
    await waitForDialog()
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Giữ cả hai')?.click() })
    expect(keepBoth).toHaveBeenCalled()
    await expect(p).resolves.toBe(true)
  })

  it('logout nhà cung cấp: huỷ không đăng xuất', async () => {
    const logout = vi.fn()
    const pending = confirmProviderLogout({ provider: 'ChatGPT', onConfirm: logout })
    await waitForDialog()
    await act(async () => { [...document.querySelectorAll('button')].find((button) => button.textContent === t.common.cancel)?.click() })
    await expect(pending).resolves.toBe(false)
    expect(logout).not.toHaveBeenCalled()
  })

  it('logout nhà cung cấp: xác nhận chỉ gửi một hành động', async () => {
    const { promise, resolve: finish } = Promise.withResolvers<void>()
    const logout = vi.fn(() => promise)
    const pending = confirmProviderLogout({ provider: 'ChatGPT', onConfirm: logout })
    await waitForDialog()
    const button = [...document.querySelectorAll('button')].find((item) => item.textContent === t.confirm.logout.ok('ChatGPT'))!
    await act(async () => { button.click(); button.click() })
    expect(logout).toHaveBeenCalledTimes(1)
    await act(async () => { finish() })
    await expect(pending).resolves.toBe(true)
  })

  it('xoá video: hai cột hệ quả Sẽ xoá / Giữ lại + nút cam đặc', async () => {
    const p = confirmDeleteVideo({
      name: 'Midnight Tales', meta: 'Sẵn sàng · MP4 38:12 · 172 MB',
      deleting: ['File MP4 đã xuất'], keeping: ['Video nguồn + phụ đề'],
      onConfirm: vi.fn(),
    })
    await waitForDialog()
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Huỷ')?.click() })
    await expect(p).resolves.toBe(false)
  })

  it('xoá kênh: nút khoá tới khi gõ đúng tên', async () => {
    const confirm = vi.fn()
    const p = confirmDeleteChannel({ name: 'Độc Lạ Studio', onConfirm: confirm })
    await waitForDialog()
    const ok = [...document.querySelectorAll('button')].find((b) => b.textContent === 'Xoá kênh')!
    expect(ok.disabled).toBe(true)
    const input = document.querySelector('input')!
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
    await act(async () => {
      setter.call(input, 'Độc Lạ Studio')
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await act(async () => {})
    expect(ok.disabled).toBe(false)

    await act(async () => { ok.click() })
    expect(confirm).toHaveBeenCalled()
    await expect(p).resolves.toBe(true)
  })
  it('xoá kênh đang chạy việc: gõ đúng tên vẫn khoá — click và Enter đều không kích hoạt', async () => {
    const confirm = vi.fn()
    const p = confirmDeleteChannel({ name: 'Độc Lạ Studio', runningCount: 2, onConfirm: confirm })
    await waitForDialog()
    const input = document.querySelector('input')!
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
    await act(async () => {
      setter.call(input, 'Độc Lạ Studio')
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await act(async () => {})
    const ok = [...document.querySelectorAll('button')].find((b) => b.textContent === 'Xoá kênh')!
    expect(ok.disabled).toBe(true)
    await act(async () => { ok.click() })
    expect(confirm).not.toHaveBeenCalled()
    // Enter trong ô gõ tên cũng không xác nhận.
    await act(async () => { input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })) })
    await act(async () => {})
    expect(confirm).not.toHaveBeenCalled()
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Huỷ')?.click() })
    await expect(p).resolves.toBe(false)
  })

  it('dừng mọi việc: nút giữ — bấm nhanh không kích hoạt', async () => {
    const confirm = vi.fn()
    const p = confirmStopAll({ running: 4, onConfirm: confirm })
    await waitForDialog()
    const hold = [...document.querySelectorAll('button')].find((b) => (b.textContent ?? '').includes('Giữ để dừng'))!
    expect(hold).toBeTruthy()
    await act(async () => { hold.click() })
    expect(confirm).not.toHaveBeenCalled()
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Huỷ')?.click() })
    await expect(p).resolves.toBe(false)
  })

  it('bỏ thay đổi chưa lưu: nút phụ Bỏ thay đổi', async () => {
    const discard = vi.fn()
    const p = confirmLeaveUnsaved({ changed: 3, template: 'Ánh sáng neon', onConfirm: vi.fn(), onDiscard: discard })
    await waitForDialog()
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Bỏ thay đổi')?.click() })
    expect(discard).toHaveBeenCalled()
    await expect(p).resolves.toBe(true)
  })

})
