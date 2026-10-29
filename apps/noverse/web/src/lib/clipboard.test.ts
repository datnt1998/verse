// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { COPY_FAILED, copyText } from './clipboard'

const original = Object.getOwnPropertyDescriptor(navigator, 'clipboard')
afterEach(() => {
  if (original) Object.defineProperty(navigator, 'clipboard', original)
  else Reflect.deleteProperty(navigator, 'clipboard')
  vi.restoreAllMocks()
})

describe('chép vào bộ nhớ tạm', () => {
  it('trang HTTP trên iPhone (không có navigator.clipboard) vẫn chép bằng cách dự phòng', async () => {
    Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true })
    const exec = vi.fn(() => true)
    Object.defineProperty(document, 'execCommand', { value: exec, configurable: true })
    await copyText('chi tiết kỹ thuật')
    expect(exec).toHaveBeenCalledWith('copy')
    expect(document.querySelector('textarea')).toBeNull()
  })

  it('trình duyệt chặn hẳn → lỗi tiếng Việt, không lộ lỗi thô', async () => {
    Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true })
    Object.defineProperty(document, 'execCommand', { value: () => false, configurable: true })
    await expect(copyText('x')).rejects.toThrow(COPY_FAILED)
  })
})
