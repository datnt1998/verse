// Sân chơi brand (#/kit › Thương hiệu) — render tĩnh phải ra đủ phần điều khiển và bảng xem CSS.
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { BrandGallery } from './BrandGallery'

describe('BrandGallery', () => {
  const html = renderToStaticMarkup(<BrandGallery />)
  it('có tiêu đề, preset và phạm vi brand để applyBrand', () => {
    expect(html).toContain('Nhận diện thương hiệu')
    expect(html).toContain('aria-label="Ocean"')
    expect(html).toContain('data-brand="preview"')
  })
  it('có phạm vi brand để applyBrand và thang tông primary', () => {
    expect(html).toContain('--scale-primary-500')
  })
  it('có đủ nhóm component: nút, nhãn trạng thái, ô nhập, tab', () => {
    for (const label of ['Nút', 'Nhãn trạng thái', 'Ô nhập &amp; bật tắt', 'Tab theo brand']) expect(html).toContain(label)
  })
})
