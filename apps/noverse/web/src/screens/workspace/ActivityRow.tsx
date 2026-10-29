// ActivityRow — một dòng HostEvent (06-data-contract.md › HostEvent → ActivityRow).
// Dùng cho panel Nhật ký ở Workspace (compact) và trang Nhật ký đầy đủ (detail).
import { useState, type CSSProperties, type ReactNode } from 'react'
import { t } from '../../i18n/vi'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { eventView, hasPendingRetry, toolCode, wireTime, type DetailedEvent } from './events'
import { formatClock } from './format'
import { copyText } from '../../lib/clipboard'
import { ReviewIssue } from '../../ui/ReviewIssue'

export function ActivityRow({ event, now, compact = false, detail = false, highlight = false, action, total }: {
  event: DetailedEvent
  /** epoch ms cho đếm ngược retryAt */
  now: number
  /** dòng gọn trong panel Workspace (giờ 11px, padding 3px) */
  compact?: boolean
  /** hiện khối “Chi tiết kỹ thuật” + nút hành động */
  detail?: boolean
  /** Gói P · P4: dòng câu hỏi Trọng tài đang chờ — chữ và icon màu cảnh báo. */
  highlight?: boolean
  action?: ReactNode
  /** Tổng số chương của truyện — chặn số chương ngoài phạm vi trong tóm tắt công cụ. */
  total?: number | null
}) {
  const [copying, setCopying] = useState(false)
  const copyTechnical = async () => {
    setCopying(true)
    try {
      await copyText(toolCode(event))
      toast(t.activity.copiedTechnical)
    } finally { setCopying(false) }
  }
  const view = eventView(event, now, total)
  const retry = hasPendingRetry(event)
    ? ` · ${view.retryIn > 0 ? t.workspace.retryIn(view.retryIn) : t.activity.retryAwaiting}`
    : ''
  const timeStyle: CSSProperties = compact
    ? { flex: '0 0 58px', paddingTop: 3, fontSize: 11 }
    : { flex: '0 0 64px', paddingTop: 2, fontSize: 12 }
  return (
    <li className="hov-row" style={{
      display: 'flex', gap: compact ? 10 : 12, padding: compact ? '3px 10px' : '5px 12px', borderRadius: compact ? 6 : 10,
    }}>
      <span className="num mono" style={{ ...timeStyle, color: 'var(--color-fg-4)' }}>{formatClock(event.time)}</span>
      <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 6, paddingLeft: view.indent }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
          {view.tree && (
            <span className="mono" aria-hidden="true" style={{ color: 'var(--color-fg-5)', fontSize: 12, paddingTop: 1 }}>├</span>
          )}
          {view.spin && <Spinner style={{ marginTop: 3 }} />}
          {view.icon !== '' && (
            <Icon name={view.icon} size={15} style={{ marginTop: 2, color: highlight ? 'var(--color-warn)' : view.iconColor }} />
          )}
          <span title={view.title} style={{ flex: 1, minWidth: 0, fontSize: 13, color: highlight ? 'var(--color-warn)' : view.tone, whiteSpace: compact ? 'nowrap' : undefined, overflow: compact ? 'hidden' : undefined, textOverflow: compact ? 'ellipsis' : undefined }}>
            {view.who && <span style={{ fontWeight: 600, color: view.whoColor }}>{view.who}</span>}
            <span>{view.text}</span>
          </span>
          {(view.duration !== '' || retry !== '') && (
            <span className="num" style={{ flex: '0 0 auto', fontSize: 12, fontStyle: 'italic', color: 'var(--color-fg-4)' }}>
              {view.duration}{retry}
            </span>
          )}
        </div>
        {!compact && event.review && event.review.items.length > 0 && (
          <ul style={{ listStyle: 'none', margin: '0 0 0 23px', padding: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
            {event.review.items.map((item) => (
              <li key={item.id}><ReviewIssue item={item} variant="log" /></li>
            ))}
          </ul>
        )}
        {detail && action}
        {detail && (
          <details style={{ marginLeft: 23 }}>
            <summary style={{ cursor: 'pointer', fontSize: 12, color: 'var(--color-fg-4)' }}>{t.activity.technical}</summary>
            <dl className="mono" style={{ margin: '6px 0 0', display: 'grid', gridTemplateColumns: 'auto 1fr', gap: '2px 12px', fontSize: 11.5, color: 'var(--color-fg-4)' }}>
              <dt>id</dt><dd style={{ margin: 0, color: 'var(--color-fg-2)' }}>{event.id || '—'}</dd>
              <dt>category</dt><dd style={{ margin: 0, color: 'var(--color-fg-2)' }}>{event.category || '—'}</dd>
              <dt>agent</dt><dd style={{ margin: 0, color: 'var(--color-fg-2)' }}>{event.agent || '—'}</dd>
              <dt>kind</dt><dd style={{ margin: 0, color: 'var(--color-fg-2)' }}>{event.kind || '—'}</dd>
              <dt>level</dt><dd style={{ margin: 0, color: 'var(--color-fg-2)' }}>{event.level || '—'}</dd>
              <dt>depth</dt><dd style={{ margin: 0, color: 'var(--color-fg-2)' }}>{event.depth}</dd>
              <dt>durationNs</dt><dd style={{ margin: 0, color: 'var(--color-fg-2)' }}>{event.durationNs || 0}</dd>
              <dt>time</dt><dd style={{ margin: 0, color: 'var(--color-fg-2)' }}>{event.time || '—'}</dd>
              <dt>finishedAt</dt><dd style={{ margin: 0, color: 'var(--color-fg-2)' }}>{wireTime(event.finishedAt) || '—'}</dd>
              {hasPendingRetry(event) && (<><dt>retryAt</dt><dd style={{ margin: 0, color: 'var(--color-fg-2)' }}>{event.retryAt}</dd></>)}
            </dl>
            {event.tool && (
              <div style={{ marginTop: 6, display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
                <pre className="mono" style={{ margin: 0, maxWidth: '100%', padding: '10px 12px', borderRadius: 11, background: '#0E0E10', border: '1px solid rgba(255,255,255,0.06)', fontSize: 12, lineHeight: 1.65, color: '#B8B8C2', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{toolCode(event)}</pre>
                <button type="button" disabled={copying} onClick={() => void copyTechnical()} style={{
                  height: 32, marginTop: 8, padding: '0 13px', display: 'inline-flex', alignItems: 'center', gap: 6, borderRadius: 999,
                  background: '#151517', border: '1px solid rgba(255,255,255,0.1)', color: '#D6D6DC', fontSize: 12.5, whiteSpace: 'nowrap',
                }}><Icon name="copy" size={14} />{t.activity.copyTechnical}</button>
              </div>
            )}
          </details>
        )}
      </div>
    </li>
  )
}
