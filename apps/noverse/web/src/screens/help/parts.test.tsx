import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { ErrorMessage } from './parts'

/** Các đoạn nằm trong font mono của một câu lỗi — chữ Việt phải nằm ngoài (rà font mono 25/10). */
const monoParts = (text: string) => [...renderToStaticMarkup(<ErrorMessage text={text} />).matchAll(/<span class="mono">([^<]*)<\/span>/g)].map((m) => m[1])

describe('ErrorMessage — lỗi hay gặp của Hướng dẫn lệnh', () => {
  it('chỉ in mono tên cờ, không in mono câu chữ tiếng Việt', () => {
    expect(monoParts('chỉ dùng một trong --overwrite hoặc --keep')).toEqual(['--overwrite', '--keep'])
    expect(monoParts('tệp đã tồn tại: exports/tra-xanh.epub (dùng --overwrite để ghi đè)')).toEqual(['exports/tra-xanh.epub', '--overwrite'])
  })

  it('giữ nguyên cả câu, kể cả phần không phải cú pháp', () => {
    const text = 'khoảng chương không hợp lệ: from=30 > to=12'
    const html = renderToStaticMarkup(<ErrorMessage text={text} />)
    const plain = html.replace(/<[^>]+>/g, '').replace(/&gt;/g, '>').replace(/&lt;/g, '<').replace(/&amp;/g, '&')
    expect(plain).toBe(text)
    expect(monoParts(text)).toEqual(['from=30', 'to=12'])
  })
})
