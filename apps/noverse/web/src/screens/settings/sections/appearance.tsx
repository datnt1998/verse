// Tuỳ chỉnh › Giao diện — Settings.dc.html sec app. Chỉ lưu cục bộ trên máy (11-api-contract.md §8:
// ainovel.readerFont), không gọi API. Quyết định 27/10: v1 chỉ có nền tối nên bỏ hẳn hàng chọn giao
// diện (Tối/Theo máy); khi có bảng màu sáng sẽ thêm lại hàng đó với 3 lựa chọn Tối · Sáng · Theo máy.
import { useEffect, useState } from 'react'
import { getReaderFont, setReaderFont } from '../../../api/storage'
import { Page, Segc, SRow, SecCard } from '../parts'

const READ_FONTS = {
  serif: '"Literata", Georgia, serif',
  sans: '"Be Vietnam Pro", ui-sans-serif, system-ui, sans-serif',
} as const

type ReadFont = keyof typeof READ_FONTS

function applyPrefs(font: ReadFont) {
  const root = document.documentElement
  root.dataset.readerFont = font
  root.style.setProperty('--font-read', READ_FONTS[font])
}

export function AppearanceSection() {
  const [font, setFontState] = useState<ReadFont>(() => (getReaderFont() === 'sans' ? 'sans' : 'serif'))

  useEffect(() => {
    applyPrefs(font)
  }, [font])

  return (
    <Page top={4}>
      <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600 }}>Giao diện</h1>
      <SecCard>
        <SRow center label="Chữ trong Trình đọc" note="Có chân dễ đọc lâu; không chân gọn mắt hơn.">
          <Segc label="Chữ trong Trình đọc" value={font} onPick={(next) => { setFontState(next); setReaderFont(next) }}
            options={[{ id: 'serif', label: 'Có chân' }, { id: 'sans', label: 'Không chân' }]} />
        </SRow>
      </SecCard>
    </Page>
  )
}
