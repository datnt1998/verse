// Gói AR · việc 6 — hộp thoại “Mô phỏng văn phong” của QuickMode (máy tính).
// Máy tính không có artboard riêng: nội dung lấy nguyên ngăn M11 `sheet=samples`
// (MNewStory.dc.html) — câu dẫn, mỗi đoạn một ô kèm số chữ và nút xoá, Thêm đoạn,
// Chọn tệp văn bản, số đếm n/20, dòng ghi chú, nút “Tạo truyện & học văn phong”.
// Luồng chạy nằm ở simulateStart.ts (dùng chung với điện thoại).
import { useRef, useState } from 'react'
import { Button, Dialog, Icon, TextArea, useSaveShortcut } from '@noverse/ui'
import { t } from '../../i18n/vi'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'
import { SAMPLE_LIMIT, normalizeSamples, sampleWords, splitSampleText } from './simulateStart'

const EMPTY = ['']

export function SimulateDialog({ onClose, onSubmit, busy, error }: {
  onClose: () => void
  onSubmit: (samples: string[]) => void
  busy: boolean
  /** Thông báo máy chủ của lần chạy hỏng; truyện đã tạo được giữ cho lần thử lại. */
  error: string | null
}) {
  const [rows, setRows] = useState<string[]>(EMPTY)
  const file = useRef<HTMLInputElement>(null)
  const samples = normalizeSamples(rows)
  const filled = rows.filter((row) => row.trim()).length
  const full = rows.length >= SAMPLE_LIMIT

  const submit = () => { if (samples.length && !busy) onSubmit(samples) }
  useUnsavedGuard(samples.length > 0 && !busy)
  useSaveShortcut(submit, samples.length > 0 && !busy)

  const edit = (index: number, value: string) => setRows((current) => current.map((row, i) => i === index ? value : row))
  const remove = (index: number) => setRows((current) => {
    const next = current.filter((_, i) => i !== index)
    return next.length ? next : EMPTY
  })
  const add = () => setRows((current) => current.length >= SAMPLE_LIMIT ? current : [...current, ''])

  const pick = async (picked: File | undefined) => {
    if (!picked) return
    const parts = splitSampleText(await picked.text())
    setRows((current) => {
      const kept = current.filter((row) => row.trim())
      const next = [...kept, ...parts.slice(0, Math.max(0, SAMPLE_LIMIT - kept.length))]
      return next.length ? next : EMPTY
    })
    if (file.current) file.current.value = ''
  }

  return (
    <Dialog title={t.newStory.simulateTitle}
      icon={<Icon name="brush-2" size={18} style={{ color: 'var(--color-accent)' }} />}
      onClose={onClose} width={560}
      footer={<>
        <Button onClick={onClose}>Huỷ</Button>
        <Button variant="primary" onClick={submit} pending={busy} disabled={busy || samples.length === 0}>
          {error ? t.newStory.simulateRetry : t.newStory.simulateSubmit}
        </Button>
      </>}>
      <div style={{ maxHeight: '58vh', overflowY: 'auto', padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}>
        <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>{t.newStory.simulateIntro}</p>
        {busy && <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>{t.newStory.simulateCreating}</span>}
        {error && (
          <div role="alert" style={{
            display: 'flex', alignItems: 'flex-start', gap: 8, padding: '10px 14px', borderRadius: 12,
            background: 'var(--tint-err)', border: '1px solid var(--color-err)', color: 'var(--color-err)', fontSize: 13,
          }}>
            <span style={{ flex: 1 }}>
              <b style={{ fontWeight: 500 }}>{t.placeholder.initFailed}</b>
              <span style={{ display: 'block', color: 'var(--color-fg-3)', fontSize: 12 }}>{error}</span>
            </span>
          </div>
        )}
        {rows.map((row, index) => {
          const label = t.newStory.simulateSample(index + 1)
          const words = row.trim() ? ` · ${t.newStory.simulateWords(sampleWords(row))}` : ''
          return (
            <div key={index} style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ flex: 1, fontSize: 12.5, color: 'var(--color-fg-3)' }}>{label}{words}</span>
                <Button variant="icon" size="sm" aria-label={t.newStory.simulateDelete(`${label}${words}`)} disabled={busy}
                  onClick={() => remove(index)}>
                  <Icon name="trash" size={16} />
                </Button>
              </span>
              <TextArea rows={4} showCount={false} value={row} disabled={busy}
                aria-label={`${label}${words}`} placeholder={t.newStory.simulatePlaceholder}
                onChange={(event) => edit(index, event.currentTarget.value)} />
            </div>
          )
        })}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <Button size="sm" icon={<Icon name="add" size={15} />} disabled={busy || full} onClick={add}>
            {t.newStory.simulateAdd}
          </Button>
          <Button size="sm" icon={<Icon name="document-text" size={15} />} disabled={busy || full}
            onClick={() => file.current?.click()}>
            {t.newStory.simulatePickFile}
          </Button>
          <input ref={file} type="file" accept=".txt,.md,text/plain,text/markdown" hidden
            aria-label={t.newStory.simulatePickFile}
            onChange={(event) => { void pick(event.currentTarget.files?.[0]) }} />
          <span style={{ flex: 1 }} />
          <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>{t.newStory.simulateCount(filled)}</span>
        </div>
        <span style={{ fontSize: 12, color: 'var(--color-fg-3)', lineHeight: 1.5 }}>{t.newStory.simulateNote}</span>
      </div>
    </Dialog>
  )
}
