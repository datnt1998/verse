// @vitest-environment happy-dom
// ConfirmDialog — 7 tình huống của Viderse: mở bằng helper rồi kiểm tiêu đề,
// lựa chọn/hậu quả/gõ tên/giữ và điều kiện mở khoá nút cam.
import { act } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { applyViderseLabels } from '../kitLabels'
import { t } from '../i18n/vi'
import {
  confirmAutonomy, confirmDeleteChannel, confirmDeleteVideo, confirmLeaveUnsaved,
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

const dialog = () => document.body

describe('7 tình huống xác nhận', () => {
  it('pause: tiêu đề kèm tên trợ lý + 2 lựa chọn radio + Huỷ riêng', async () => {
    const done = vi.fn()
    const p = confirmPause({ agent: 'artist', video: 'Midnight Tales', scene: 'cảnh 5/6', onConfirm: done })
    await act(async () => {})
    const text = dialog().textContent ?? ''
    expect(text).toContain('Tạm dừng Hoạ sĩ?')
    expect(text).toContain('Midnight Tales')
    expect(text).toContain('Sau cảnh này')
    expect(text).toContain('Ngay bây giờ')
    expect(text).toContain('Tiếp tục vẽ')
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Tiếp tục vẽ')?.click() })
    await expect(p).resolves.toBe(false)
  })

  it('overwrite: so hai bản + nút phụ Giữ cả hai', async () => {
    const keepBoth = vi.fn()
    const p = confirmOverwrite({
      file: 'Midnight-Tales.mp4',
      current: { summary: '38:12 · 172 MB' },
      next: { summary: '38:12 · 176 MB' },
      onConfirm: vi.fn(), onKeepBoth: keepBoth,
    })
    await act(async () => {})
    const text = dialog().textContent ?? ''
    expect(text).toContain('Thay bản MP4 cũ?')
    expect(text).toContain('Bản đang có')
    expect(text).toContain('Bản mới')
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Giữ cả hai')?.click() })
    expect(keepBoth).toHaveBeenCalled()
    await expect(p).resolves.toBe(true)
  })

  it('logout nhà cung cấp: nút cam nhạt ghi tên gói', async () => {
    const confirm = vi.fn()
    const p = confirmProviderLogout({ provider: 'ChatGPT', fallback: 'Gemini', onConfirm: confirm })
    await act(async () => {})
    const text = dialog().textContent ?? ''
    expect(text).toContain('Đăng xuất gói ChatGPT?')
    expect(text).toContain('Gemini')
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Đăng xuất ChatGPT')?.click() })
    expect(confirm).toHaveBeenCalled()
    await expect(p).resolves.toBe(true)
  })

  it('xoá video: hai cột hệ quả Sẽ xoá / Giữ lại + nút cam đặc', async () => {
    const p = confirmDeleteVideo({
      name: 'Midnight Tales', meta: 'Sẵn sàng · MP4 38:12 · 172 MB',
      deleting: ['File MP4 đã xuất'], keeping: ['Video nguồn + phụ đề'],
      onConfirm: vi.fn(),
    })
    await act(async () => {})
    const text = dialog().textContent ?? ''
    expect(text).toContain('Xoá video “Midnight Tales”?')
    expect(text).toContain('Sẽ xoá')
    expect(text).toContain('Giữ lại')
    expect(text).toContain('File MP4 đã xuất')
    expect(text).toContain('Video nguồn + phụ đề')
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Huỷ')?.click() })
    await expect(p).resolves.toBe(false)
  })

  it('xoá kênh: nút khoá tới khi gõ đúng tên', async () => {
    const confirm = vi.fn()
    const p = confirmDeleteChannel({ name: 'Độc Lạ Studio', onConfirm: confirm })
    await act(async () => {})
    expect(dialog().textContent).toContain('Xoá kênh “Độc Lạ Studio”?')
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
    await act(async () => {})
    const text = dialog().textContent ?? ''
    // Lý do khoá hiển thị trong hộp (runningHint), không chỉ là chữ mô tả.
    expect(text).toContain('Xoá kênh “Độc Lạ Studio”?')
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
    await act(async () => {})
    const text = dialog().textContent ?? ''
    expect(text).toContain('Dừng mọi việc đang chạy?')
    expect(text).toContain('4 video đang làm')
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
    await act(async () => {})
    const text = dialog().textContent ?? ''
    expect(text).toContain('Bỏ thay đổi chưa lưu?')
    expect(text).toContain('Ánh sáng neon')
    expect(text).toContain('Ở lại')
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Bỏ thay đổi')?.click() })
    expect(discard).toHaveBeenCalled()
    await expect(p).resolves.toBe(true)
  })

  it('tự chạy: 3 điều khoản trong thân hộp', async () => {
    const p = confirmAutonomy({ channel: 'Kênh Thực Tế', onConfirm: vi.fn() })
    await act(async () => {})
    const text = dialog().textContent ?? ''
    expect(text).toContain('Chuyển Kênh Thực Tế sang Tự chạy?')
    expect(text).toContain('Không bao giờ tự đăng')
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Huỷ')?.click() })
    await expect(p).resolves.toBe(false)
  })
})
