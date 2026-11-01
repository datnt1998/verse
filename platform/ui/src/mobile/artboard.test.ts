// Hợp đồng số đo bộ mobile kit với artboard Viderse MKit + 03-tokens.md.
// happy-dom không dựng bố cục nên không đo được hộp; tệp này giữ đúng một chỗ
// những con số đã chốt (cùng kiểu với hợp đồng `env(safe-area-inset)` một khai báo).
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const src = (file: string) => readFileSync(new URL(file, import.meta.url), 'utf8')

describe('số đo bộ mobile kit khớp artboard MKit', () => {
  it('MobilePage: thanh trên 52, tiêu đề 16/600, dòng phụ 12', () => {
    const page = src('./page.tsx')
    expect(page).toMatch(/height: 52,\s*\n\s*minHeight: 52/)
    expect(page).toMatch(/fontSize: 16,\s*\n\s*fontWeight: 600/)
    expect(page).toMatch(/fontSize: 12,\s*\n\s*fontWeight: 400/)
  })

  it('ListRow 56/6-10/bo 12, VideoRow thumb 80×45, StatusPill 12 + chấm 7', () => {
    const rows = src('./rows.tsx')
    expect(rows).toMatch(/minHeight: 56/)
    expect(rows).toMatch(/width: 80,/)
    expect(rows).toMatch(/height: 45,/)
    expect(rows).toMatch(/fontSize: 12, whiteSpace: 'nowrap'/)
    expect(rows).toMatch(/width: 7, height: 7/)
  })

  it('Stepper điện thoại cao 44, nút 44, số mono 13', () => {
    const rows = src('./rows.tsx')
    expect(rows).toMatch(/width: 160,\s*\n\s*height: 44/)
    expect(rows).toMatch(/width: 44,/)
    expect(rows).toMatch(/fontFamily: 'var\(--font-mono\)',\s*\n\s*fontSize: 13/)
  })

  it('Chip điện thoại 36 nhìn thấy, vùng chạm 44 (::before inset -4)', () => {
    const chip = src('../Chip.tsx')
    expect(chip).toMatch(/height: 36, paddingInline: 13, borderRadius: 999, fontSize: 13\.5/)
    expect(chip).toMatch(/insetBlock: -4/)
  })

  it('ô nhập điện thoại cao 46, chữ 16 (iOS không phóng trang)', () => {
    expect(src('../Field.tsx')).toMatch(/fieldMobile: \{ height: 46, minHeight: 46, fontSize: 16, borderRadius: 12, paddingInline: 12 \}/)
  })

  it('tấm md: tiêu đề 18px, nền surface-3', () => {
    const css = src('../kit.css')
    expect(css).toContain('.m-sheet--md .m-sheet-title h2{font-size:18px;font-weight:600;line-height:1.35}')
    expect(css).toContain('.m-sheet--md{background:var(--color-surface-3)')
  })

  it('nút CTA 48 (xl) và biến thể block chiếm trọn bề ngang', () => {
    const button = src('../Button.tsx')
    expect(button).toMatch(/xl: \{ height: 48, paddingInline: 16, borderRadius: 13, fontSize: 15, fontWeight: 600, gap: 8 \}/)
    expect(button).toMatch(/block: \{ width: '100%' \}/)
  })

  it('Segmented cỡ điện thoại: mục 40, nhóm 48, chữ 14, góc 12/9', () => {
    const tabs = src('../Tabs.tsx')
    expect(tabs).toMatch(/xl: 40/)
    expect(tabs).toMatch(/segmentXl: \{ display: 'grid', gridAutoFlow: 'column', gridAutoColumns: 'minmax\(max-content, 1fr\)', borderRadius: 12 \}/)
    expect(tabs).toMatch(/segmentXlButton: \{ fontSize: 14 \}/)
  })

  it('OfflineBanner đếm ngược, nút thử ngay, mờ nội dung 55% sau 3 giây', () => {
    const bars = src('./bars.tsx')
    expect(bars).toMatch(/sustainedMs = 3000/)
    expect(bars).toContain('t.mobile.retryNow')
    const css = src('./mobile.css')
    expect(css).toMatch(/html\[data-offline\] \.m-scroll/)
    expect(css).toMatch(/opacity: \.55/)
  })

  it('thanh chọn ẩn thanh tab và đặt đáy an toàn', () => {
    const bars = src('./bars.tsx')
    expect(bars).toContain("useTabMode('hide')")
    expect(bars).toContain('calc(4px + max(26px, var(--m-safe-bottom)) + var(--kb, 0px))')
  })
})
