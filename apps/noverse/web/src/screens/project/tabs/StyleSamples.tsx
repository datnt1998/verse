// Gói AL — “Văn phong của bạn · văn mẫu của truyện này” ở tab Cách viết của truyện
// (chuyển từ Tuỳ chỉnh › Cách viết; Outline.dc.html › tab=style, khối wSamples).
// API: GET/POST/DELETE …/stories/{s}/style-samples (GAP-35). Máy chủ trả 403/404 khi không có quyền
// viết truyện, 409 khi đã đủ số mẫu, 400 khi đoạn văn rỗng — hiện nguyên thông báo của máy chủ.
import { useState } from 'react'
import { createStyleSample, deleteStyleSample, getStyleSamples } from '../../../api/client'
import { keys, useAction, useResource } from '../../../data'
import { Icon, toast } from '@noverse/ui'
import '../../settings/style.css'

const chars = (n: number) => `${n.toLocaleString('vi-VN')} chữ`
// Hàng mẫu theo artboard: “N chữ · d/m” (bỏ năm — mẫu gần đây).
const sampleDate = (iso: string) => {
  const d = new Date(iso)
  return Number.isNaN(+d) ? iso : `${d.getDate()}/${d.getMonth() + 1}`
}

export function StyleSamples({ project, story, canEdit }: { project: string; story: string; canEdit: boolean }) {
  const key = keys.story(project, story, 'style-samples')
  const resource = useResource(key, ({ signal }) => getStyleSamples(project, story, signal))
  const [adding, setAdding] = useState(false)
  const [title, setTitle] = useState('')
  const [text, setText] = useState('')

  const add = useAction((input: { title?: string; text: string }) => createStyleSample(project, story, input), {
    invalidate: [key],
  })
  const remove = useAction((id: string) => deleteStyleSample(project, story, id), {
    success: 'Đã xoá văn mẫu', invalidate: [key],
  })

  const total = resource.data?.total
  // Mẫu mới nhất đứng đầu (máy chủ trả thứ tự tuỳ ý; giữ thứ tự máy chủ khi trùng mốc).
  const samples = [...(resource.data?.samples ?? [])].sort((a, b) => (Date.parse(b.addedAt) || 0) - (Date.parse(a.addedAt) || 0))
  const trimmed = text.trim()
  const submit = () => {
    if (!trimmed || add.busy) return
    void add.run({ title: title.trim() || undefined, text }).then((created) => {
      if (created === undefined) return
      setAdding(false)
      setTitle('')
      setText('')
      // Câu của artboard (wSaveSmp): “Đã thêm văn mẫu · trợ lý học lại giọng văn”.
      toast.ok('Đã thêm văn mẫu · trợ lý học lại giọng văn')
    })
  }

  return (
    <section className="card" aria-label="Văn phong của bạn" style={{ borderRadius: 16, overflow: 'hidden', padding: '14px 16px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, paddingBottom: 10 }}>
        <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.4 }}>
          <b style={{ fontSize: 14, fontWeight: 600 }}>Văn phong của bạn · văn mẫu của truyện này</b>
          <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>
            Dán đoạn bạn tự viết, trợ lý học giọng văn. Đủ văn mẫu thì chọn được kiểu “Văn phong của bạn” ở trên.
            {total && total.count > 0 ? ` Đã có ${total.count} văn mẫu · ${chars(total.chars)}.` : ''}
          </span>
        </span>
        {canEdit && (
          <button type="button" className="hov-ghost" aria-expanded={adding} onClick={() => setAdding((open) => !open)}
            style={{
              height: 32, padding: '0 12px', borderRadius: 9, border: '1px solid var(--kit-line-12)',
              background: 'transparent', color: 'var(--color-fg)', fontSize: 12.5, whiteSpace: 'nowrap',
              display: 'flex', alignItems: 'center', gap: 6, flex: '0 0 auto',
            }}>
            <Icon name="add" size={14} />Thêm văn mẫu
          </button>
        )}
      </div>
      {adding && (
        <div className="st-drawer" style={{ paddingBottom: 14, display: 'flex', flexDirection: 'column', gap: 8 }}>
          <input className="st-field" type="text" value={title} maxLength={120} aria-label="Tên văn mẫu"
            placeholder="Tên văn mẫu (không bắt buộc)" onChange={(event) => setTitle(event.target.value)} />
          <textarea className="st-field" value={text} rows={4} aria-label="Nội dung văn mẫu"
            placeholder="Dán đoạn văn mẫu vào đây" onChange={(event) => setText(event.target.value)}
            style={{ height: 'auto', minHeight: 104, padding: '10px 12px', lineHeight: 1.55, resize: 'vertical' }} />
          <span style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span className="num" style={{ flex: '1 1 auto', fontSize: 12, color: 'var(--color-fg-4)' }}>
              {trimmed ? `${trimmed.length.toLocaleString('vi-VN')} ký tự` : 'Nên dán từ 1.000 ký tự trở lên'}
            </span>
            <button type="button" className="hov-ghost" onClick={() => setAdding(false)}
              style={{ height: 32, padding: '0 12px', border: 'none', borderRadius: 9, background: 'transparent', color: 'var(--color-fg-3)', fontSize: 13 }}>
              Huỷ
            </button>
            <button type="button" className="hov-primary" disabled={!trimmed || add.busy} onClick={submit}
              style={{
                height: 32, padding: '0 14px', border: 'none', borderRadius: 9, background: 'var(--color-fg)',
                color: 'var(--color-bg)', fontSize: 13, fontWeight: 600, opacity: !trimmed || add.busy ? 0.5 : 1,
              }}>
              Lưu văn mẫu
            </button>
          </span>
        </div>
      )}
      {resource.error ? (
        <span style={{ fontSize: 12.5, color: 'var(--color-err)', paddingBottom: 2 }}>
          Không đọc được văn mẫu: {String((resource.error as Error)?.message ?? resource.error)}
        </span>
      ) : samples.length === 0 && !adding ? (
        <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)', lineHeight: 1.5, paddingBottom: 2 }}>
          Chưa có văn mẫu nào. Dán một đoạn bạn muốn máy chủ học giọng văn.
        </span>
      ) : samples.length > 0 ? (
        <div style={{ borderTop: '1px solid var(--line-1)', margin: '0 -16px -14px' }}>
          {samples.map((sample) => (
            <div key={sample.id} className="st-smp">
              <Icon name="document-text" size={16} style={{ color: 'var(--kit-fg-icon)', flex: '0 0 auto' }} />
              <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.4 }}>
                <span title={sample.title} style={{ fontSize: 13.5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{sample.title}</span>
                <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{chars(sample.chars)} · {sampleDate(sample.addedAt)}</span>
              </span>
              {canEdit && (
                <button type="button" className="hov-ghost" aria-label={`Xoá văn mẫu ${sample.title}`}
                  disabled={remove.busy} onClick={() => void remove.run(sample.id)}
                  style={{
                    width: 30, height: 30, border: 'none', borderRadius: 8, background: 'transparent',
                    color: 'var(--kit-fg-icon)', display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto',
                  }}>
                  <Icon name="trash" size={15} />
                </button>
              )}
            </div>
          ))}
        </div>
      ) : null}
    </section>
  )
}

