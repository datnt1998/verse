import { useState } from 'react'
import { Icon } from '@noverse/ui'
import type { Preview } from './model'

/**
 * Onboard.dc.html · aside “Thiết lập sẽ lưu”: bản bằng lời cho mọi vai trò;
 * “Tệp sẽ ghi (cho quản trị)” chỉ hiện với quản trị viên và mặc định thu gọn.
 */
export function WritePreview({ preview, showTech }: { preview: Preview; showTech: boolean }) {
  const [techOpen, setTechOpen] = useState(false)
  return (
    <>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <Icon name="document-text" size={16} style={{ color: 'var(--color-fg-3)' }} />
        <span className="lbl" style={{ flex: 1 }}>{preview.title}</span>
      </div>
      <div className="card" style={{ borderRadius: 14, padding: '4px 14px' }}>
        {preview.plain.map((row, index) => (
          <div key={row.k} style={{ display: 'flex', flexDirection: 'column', gap: 1, padding: '10px 0', borderTop: index ? '1px solid rgba(255,255,255,0.06)' : 'none' }}>
            <span style={{ fontSize: 12, color: '#7E7E8A' }}>{row.k}</span>
            <span style={{ fontSize: 13.5, color: '#EDEDEF' }}>{row.v}</span>
          </div>
        ))}
      </div>
      <span style={{ fontSize: 12, color: '#7E7E8A' }}>Lưu ngay khi bạn bấm Tiếp tục. Mọi thứ đổi lại được trong Tuỳ chỉnh.</span>
      {showTech && (
        <div style={{ borderTop: '1px solid rgba(255,255,255,0.06)', paddingTop: 10, display: 'flex', flexDirection: 'column', gap: 10 }}>
          <button type="button" onClick={() => setTechOpen((open) => !open)} aria-expanded={techOpen} style={{
            alignSelf: 'flex-start', display: 'flex', alignItems: 'center', gap: 6, padding: 0, border: 'none', background: 'transparent', color: '#7E7E8A', fontSize: 12.5,
          }}>
            <Icon name={techOpen ? 'arrow-down-2' : 'arrow-right-2'} size={14} />
            <Icon name="crown" size={12} style={{ color: '#B89A6E' }} />Tệp sẽ ghi (cho quản trị)
          </button>
          {techOpen && (
            <div className="card" style={{ borderRadius: 14, overflow: 'hidden' }}>
              <div style={{ padding: '9px 12px', borderBottom: '1px solid rgba(255,255,255,0.06)', fontSize: 12, color: '#C9C9D1', display: 'flex', justifyContent: 'space-between', gap: 8 }}>
                <span className="mono" style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{preview.file}</span>
                <span style={{ color: '#7E7E8A' }}>{preview.fileNote}</span>
              </div>
              <div className="mono" style={{ padding: '10px 0', fontSize: 11.5, lineHeight: 1.75 }}>
                {preview.lines.map((line, index) => (
                  <div key={index} style={{ padding: `0 12px 0 ${line.pad}px`, background: line.hot ? 'rgba(201,166,245,0.09)' : 'transparent', whiteSpace: 'pre', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                    {line.segs.map((seg, part) => <span key={part} style={{ color: seg.color }}>{seg.text}</span>)}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </>
  )
}
