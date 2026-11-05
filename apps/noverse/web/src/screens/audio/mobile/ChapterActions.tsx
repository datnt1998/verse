// Tấm “Thao tác chương”: tiêu đề + dòng phụ, một nhóm mục có biểu tượng và mô tả,
// rồi nút Đóng. Mở menu không gọi máy chủ; chỉ mục được bấm mới chạy,
// mục có `confirm` hỏi lại trước.
import { useEffect, useId, useState } from 'react'
import { BottomSheet, ConfirmSheet } from '../../../mobile/primitives'
import { Icon, Spinner } from '@noverse/ui'
import { t } from '../../../i18n/vi'
import { errText } from '../useAudio'
import type { ChapterActionId, ChapterActionItem } from './chapter-actions'

export function ChapterActions({ open, title, meta, items, onClose, onAction }: {
  open: boolean; title: string; meta: string; items: readonly ChapterActionItem[]
  onClose: () => void
  /** Chạy thao tác thật; lỗi ném ra được hiện ngay trong tấm, tấm không tự đóng. */
  onAction: (id: ChapterActionId) => Promise<void>
}) {
  const [pending, setPending] = useState<ChapterActionId | null>(null)
  const [failure, setFailure] = useState('')
  const [confirming, setConfirming] = useState<ChapterActionItem | null>(null)
  const titleId = useId()
  // Đổi chương hoặc đóng/mở lại: không giữ lỗi hay hộp xác nhận của chương trước.
  useEffect(() => { setFailure(''); setConfirming(null); setPending(null) }, [open, title])
  const close = () => { if (pending) return; setFailure(''); setConfirming(null); onClose() }
  const run = async (item: ChapterActionItem) => {
    if (pending) return
    setConfirming(null); setPending(item.id); setFailure('')
    try { await onAction(item.id) }
    catch (cause) { setFailure(errText(cause)) }
    finally { setPending(null) }
  }
  const choose = (item: ChapterActionItem) => {
    if (pending || item.disabled) return
    if (item.confirm) { setConfirming(item); return }
    void run(item)
  }
  return <>
    <BottomSheet open={open} onClose={close} height="auto" label={title}>
      <div className="ma-chm">
        <div className="ma-chm-head"><span id={titleId}>{title}</span>{meta && <span className="ma-chm-meta">{meta}</span>}</div>
        <div className="ma-chm-group" role="menu" aria-labelledby={titleId}>
          {items.map((item) => <button key={item.id} type="button" role="menuitem" data-action={item.id}
            className={`ma-chm-item${item.tone ? ` ${item.tone}` : ''}`}
            disabled={pending !== null || item.disabled} aria-busy={pending === item.id || undefined}
            onClick={() => choose(item)}>
            {pending === item.id ? <Spinner size={18} /> : <Icon name={item.icon} size={19} />}
            <span className="ma-chm-text"><span>{item.label}</span><small>{item.detail}</small></span>
            {item.confirm && <Icon name="arrow-right-2" size={15} style={{ color: 'var(--color-fg-4)', flexShrink: 0 }} />}
          </button>)}
        </div>
        {failure && <p className="m-error" role="alert">{failure}</p>}
        <button type="button" className="ma-chm-close" disabled={pending !== null} onClick={close}>{t.dialogs.close}</button>
      </div>
    </BottomSheet>
    <ConfirmSheet open={confirming !== null} onClose={() => { if (!pending) setConfirming(null) }}
      title={confirming?.confirm?.title ?? ''} description={confirming?.confirm?.body ?? ''}
      action={{
        label: confirming?.confirm?.label ?? '',
        destructive: confirming?.confirm?.destructive,
        onSelect: async () => { if (confirming) await run(confirming) },
      }} />
  </>
}
