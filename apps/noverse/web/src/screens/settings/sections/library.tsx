// Tuỳ chỉnh › Xuất tự động — Settings.dc.html sec export. Ghi qua library.json (`libDraft.autoExport`) cùng thanh Lưu.
// Tên thư mục và mẫu tên tệp xuất nằm ở Nâng cao › Tên thư mục & tên tệp xuất (advanced/NamesRows.tsx).
import type { LibraryConfig } from '../../../api/types'
import { Page, Sw, Segc, SRow, SecCard, SectionHead } from '../parts'
import type { SettingsContext } from '../types'
import { EXPORT_FORMATS, exampleFileName, toggleFormat } from './exportLogic'
import { Checkbox } from '@noverse/ui'

const TRIGGERS: { key: 'onVolume' | 'onBook' | 'onChapter'; label: string; note: string }[] = [
  { key: 'onVolume', label: 'Xong mỗi tập', note: 'Tự xuất tập vừa xong' },
  { key: 'onBook', label: 'Xong cả truyện', note: 'Tự xuất bản đầy đủ' },
  { key: 'onChapter', label: 'Xong mỗi chương', note: 'Cập nhật bản xuất sau từng chương (nhiều tệp hơn)' },
]

export function ExportSection({ ctx }: { ctx: SettingsContext }) {
  const lib = ctx.libDraft
  if (!lib) return null
  const readOnly = !ctx.admin || ctx.library?.storageMode === 'postgres'
  const patchExport = (next: Partial<LibraryConfig['autoExport']>) =>
    ctx.setLibDraft({ ...lib, autoExport: { ...lib.autoExport, ...next } })
  const auto = lib.autoExport

  return (
    <Page top={4}>
      <SectionHead title="Xuất tự động" note="Áp dụng cho mọi dự án trong thư viện. Bản xuất xem ở Dự án › Bản xuất." />
      <SecCard>
        {TRIGGERS.map((trigger) => (
          <SRow key={trigger.key} label={trigger.label} note={trigger.note}>
            <Sw on={auto[trigger.key]} label={trigger.label} disabled={readOnly} onToggle={() => patchExport({ [trigger.key]: !auto[trigger.key] })} />
          </SRow>
        ))}
        <SRow center label="Định dạng">
          <span role="group" aria-label="Định dạng" style={{ display: 'flex', gap: 6 }}>
            {EXPORT_FORMATS.map((format) => {
              const on = auto.formats.includes(format.id)
              return (
                <Checkbox key={format.id} checked={on} disabled={readOnly} variant="chip" showMark={false}
                  label={format.label} onChange={(next) => patchExport({ formats: toggleFormat(auto.formats, format.id, next) })}
                  style={{
                    border: 'none', height: 30, padding: '0 12px', borderRadius: 7, fontSize: 12,
                    background: on ? 'rgba(201,166,245,0.14)' : 'rgba(255,255,255,.05)', color: on ? '#F3E8FF' : '#8E8E99',
                  }} />
              )
            })}
          </span>
        </SRow>
        <SRow center label="Khi đã có bản xuất cũ">
          <Segc label="Khi đã có bản xuất cũ" value={auto.onExists} disabled={readOnly}
            options={[{ id: 'replace', label: 'Thay bản cũ' }, { id: 'keep', label: 'Giữ cả hai' }]}
            onPick={(onExists) => patchExport({ onExists })} />
        </SRow>
        <SRow center label="Tên tệp" note={<>Ví dụ: <span style={{ color: '#C9C9D1' }}>{exampleFileName(ctx.sampleStoryTitle, auto.formats)}</span></>}>
          <button type="button" className="hov-ghost" onClick={() => ctx.goSection('adv', 'names')} style={{
            height: 28, padding: '0 10px', border: 'none', borderRadius: 7, background: 'transparent', color: '#C9A6F5', fontSize: 12.5,
          }}>Đổi mẫu tên trong Nâng cao</button>
        </SRow>
      </SecCard>
    </Page>
  )
}
