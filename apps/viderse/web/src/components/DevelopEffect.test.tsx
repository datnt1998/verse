// Hiệu ứng vẽ: nền bản trước mờ, 3 bước, ảnh xem dần, nút huỷ; vẽ xong lộ ảnh
// + viền sáng. Giảm chuyển động thì tắt vệt sáng/hạt/nhấp nháy (04-components
// › DevelopEffect).
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { DevelopEffect } from './DevelopEffect'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let reduced = false
vi.stubGlobal('matchMedia', (query: string) => ({
  matches: query.includes('reduce') ? reduced : false,
  media: query,
  addEventListener: () => {},
  removeEventListener: () => {},
}))

let root: Root | undefined
const node = () => {
  const host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  return host
}
afterEach(() => {
  if (root) act(() => root!.unmount())
  root = undefined
  reduced = false
  document.body.innerHTML = ''
})

const mount = async (element: React.ReactElement) => {
  const host = node()
  await act(async () => { root!.render(element) })
  return host
}

describe('DevelopEffect đang vẽ', () => {
  it('nền là bản trước, đủ 3 bước, bước 2 đang làm', async () => {
    const host = await mount(<DevelopEffect state="drawing" baseUrl="/old.jpg" title="Hoạ sĩ đang vẽ bản 3" detail="0:24 · thường khoảng 40 giây" step={1} />)
    expect(host.querySelector('img')?.getAttribute('src')).toBe('/old.jpg')
    const status = host.querySelector('[role="status"]')!
    expect(status.textContent).toContain('Hoạ sĩ đang vẽ bản 3')
    expect(status.textContent).toContain('0:24 · thường khoảng 40 giây')
    expect(status.textContent).toContain('Đã gửi prompt')
    expect(status.textContent).toContain('Đang vẽ')
    expect(status.textContent).toContain('Tải về')
    expect(status.textContent).toContain('Bạn duyệt tiếp ảnh khác được; xong sẽ báo ở đây và trên dải ảnh.')
    expect(host.querySelector('[data-develop-grain]')).not.toBeNull()
  })

  it('có previewUrl thì hiện ảnh xem dần mờ', async () => {
    const host = await mount(<DevelopEffect state="drawing" baseUrl="/old.jpg" previewUrl="/preview.jpg" title="Hoạ sĩ đang vẽ bản 3" />)
    expect([...host.querySelectorAll('img')].map((img) => img.getAttribute('src'))).toEqual(['/old.jpg', '/preview.jpg'])
  })

  it('nút Huỷ lần vẽ gọi onCancel', async () => {
    const onCancel = vi.fn()
    const host = await mount(<DevelopEffect state="drawing" title="Hoạ sĩ đang vẽ bản 3" onCancel={onCancel} />)
    await act(async () => { host.querySelector('button')!.click() })
    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('bản compact không có thẻ trạng thái nhưng có vòng chờ', async () => {
    const host = await mount(<DevelopEffect state="drawing" baseUrl="/old.jpg" compact />)
    expect(host.querySelector('[role="status"]')).toBeNull()
    expect(host.querySelectorAll('span').length).toBeGreaterThan(1)
  })
})

describe('DevelopEffect vẽ xong', () => {
  it('bọc ảnh thật và có viền sáng', async () => {
    const host = await mount(<DevelopEffect state="done"><img alt="" src="/new.jpg" /></DevelopEffect>)
    expect(host.querySelector('img')?.getAttribute('src')).toBe('/new.jpg')
    expect(host.querySelector('[data-develop-glint]')).not.toBeNull()
  })
})

describe('giảm chuyển động', () => {
  it('tắt hạt nhiễu và nhấp nháy bước đang làm', async () => {
    const running = await mount(<DevelopEffect state="drawing" baseUrl="/old.jpg" title="Hoạ sĩ đang vẽ bản 3" step={1} />)
    const runningClass = running.querySelector('[data-develop-dot="on"]')!.getAttribute('class')
    act(() => root!.unmount())
    root = undefined
    document.body.innerHTML = ''
    reduced = true
    const still = await mount(<DevelopEffect state="drawing" baseUrl="/old.jpg" title="Hoạ sĩ đang vẽ bản 3" step={1} />)
    expect(still.querySelector('[data-develop-grain]')).toBeNull()
    expect(still.querySelector('[data-develop-dot="on"]')!.getAttribute('class')).not.toBe(runningClass)
  })

  it('lộ ảnh thành đổi tức thì (chỉ mờ dần)', async () => {
    reduced = true
    const host = await mount(<DevelopEffect state="done"><img alt="" src="/new.jpg" /></DevelopEffect>)
    expect(host.querySelector('[data-develop-glint]')).not.toBeNull()
    expect(host.firstElementChild?.getAttribute('class')).toBeTruthy()
  })
})
