// ReviewIssue — thẻ thẩm duyệt dùng chung (17-kit.md › “Thẻ thẩm duyệt có số mục luật”,
// 22-brief-2510.md › D4/D9): Trình đọc, Bàn viết (phase gate), Nhật ký, nhật ký điện thoại.
// Dòng nhãn [Bắt sửa | Cảnh báo] + chip số mục luật + tên mục xám; câu lỗi; đoạn trích Literata
// (gạch màu theo mức); “Gợi ý: …”; hành động chỉ ở mức Cảnh báo.
// GAP-47: “Giữ nguyên” mở ô “Vì sao giữ nguyên?” (3 chip gợi ý + ô tự ghi), lưu xong hiện
// “✓ Lý do: … · <tên>, HH:MM” + “Bỏ giữ”. Lý do lưu ở `items[].decision`.
import { useState, type CSSProperties } from 'react'
import type { ReviewDecision, ReviewItem, ReviewEntry, ReviewLevel } from '../api/types'
import { t } from '../i18n/vi'
import { Icon } from '@noverse/ui'

/** full = Trình đọc · gate = Bàn viết · log = Nhật ký · compact = nhật ký điện thoại · quality = panel phải của Chất lượng truyện */
export type ReviewIssueVariant = 'full' | 'gate' | 'log' | 'compact' | 'quality'

/** Mức hiển thị — cùng union với `ReviewItem.level` sinh từ Go. */
export type { ReviewLevel }

/** Mức đã chuẩn hoá ở máy chủ (`ReviewItem.level` là union đóng) — critical cũ đã đổi thành error khi dựng DTO. */
export function reviewLevel(item: Pick<ReviewItem, 'level'>): ReviewLevel {
  return item.level
}

export function reviewIssueCounts(items: ReviewItem[]): { all: number; errors: number; warnings: number } {
  const errors = items.filter((item) => reviewLevel(item) === 'error').length
  return { all: items.length, errors, warnings: items.length - errors }
}

/**
 * Chương còn mục Bắt sửa → chưa duyệt được (cửa duyệt đóng). Không loại trừ loại mục nào:
 * lỗi lint “Xưng hô” mức Bắt sửa chặn y như mọi mục khác (22-brief-2510.md › câu 13).
 */
export const reviewBlocking = (items: ReviewItem[]): boolean => items.some((item) => reviewLevel(item) === 'error')

/** Mục của một lần chấm; dữ liệu cũ không có `items` → mảng rỗng. */
export const reviewItemsOf = (entry: ReviewEntry | null | undefined): ReviewItem[] => entry?.items ?? []

/** “Yêu cầu sửa” điền sẵn ô góp ý: `sửa theo Master §13: …` (22-brief D4). */
export const reviewFixPrefill = (item: ReviewItem): string => `${t.review.fixPrefill(item.ref)}${item.fix.toLowerCase()}`

/** Giờ:phút của mốc RFC3339 — phụ đề “· 21:52” và dòng “Lý do: … · Dat, 21:58”. */
export function reviewClock(iso: string | null | undefined): string {
  if (!iso) return ''
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

/** Dòng đã giữ: `Lý do: <lý do> · <tên>, HH:MM`; thiếu giờ thì bỏ vế giờ, thiếu người thì bỏ vế sau. */
export function reviewKeptLine(decision: ReviewDecision): string {
  const who = [decision.by, reviewClock(decision.at)].filter(Boolean).join(', ')
  return who ? t.review.keepSavedLine(decision.reason, who) : t.review.keepSavedOnly(decision.reason)
}

const labelChip = (level: ReviewLevel): CSSProperties => ({
  height: 20, padding: '0 7px', display: 'inline-flex', alignItems: 'center', borderRadius: 6,
  background: level === 'error' ? 'var(--kit-errf-12)' : 'var(--kit-warn-12)',
  color: level === 'error' ? 'var(--kit-err-soft)' : 'var(--kit-warn-soft)',
  fontSize: 11.5, fontWeight: 500, whiteSpace: 'nowrap', flex: 'none',
})

/**
 * Khung thẻ theo màn: `card` = thẻ riêng trong Trình đọc (bo 14, viền theo mức);
 * `inset` = khối lồng trong Bàn viết/Nhật ký (nền surface-0, bo 10).
 */
export function reviewIssueFrame(level: ReviewLevel, kind: 'card' | 'inset', compact = false): CSSProperties {
  const border = level === 'error' ? 'var(--kit-errf-22)' : 'var(--line-2)'
  return kind === 'card'
    ? { borderRadius: 14, padding: '12px 14px', border: `1px solid ${border}`, display: 'flex', flexDirection: 'column', gap: 8 }
    : { borderRadius: 10, padding: compact ? '8px 10px' : '8px 12px', background: 'var(--color-surface-0)', border: `1px solid ${border}`, display: 'flex', flexDirection: 'column', gap: 4 }
}

const ghostAction: CSSProperties = {
  height: 28, padding: '0 10px', borderRadius: 7, border: 'none', background: 'transparent',
  color: 'var(--color-fg-3)', fontSize: 12, whiteSpace: 'nowrap', flex: 'none',
}

export function ReviewIssue({ item, variant = 'full', decision, onKeep, onUnkeep, onRequestFix, saving = false, frame = true, context, aside }: {
  item: ReviewItem
  variant?: ReviewIssueVariant
  /** GAP-47 — quyết định giữ nguyên đã lưu; mặc định lấy từ `item.decision` */
  decision?: ReviewDecision
  /** Ghi lý do giữ nguyên (trả về false khi lỗi → giữ ô mở); có hàm này mới hiện nút “Giữ nguyên” */
  onKeep?: (reason: string) => void | boolean | Promise<void | boolean>
  /** Bỏ giữ (GAP-47); có hàm này mới hiện nút “Bỏ giữ” */
  onUnkeep?: () => void | Promise<unknown>
  /** Điền sẵn ô góp ý; có hàm này mới hiện nút “Yêu cầu sửa” */
  onRequestFix?: (prefill: string) => void
  saving?: boolean
  /** false khi thẻ cha đã là khung (dùng trong danh sách đã bọc) */
  frame?: boolean
  /** Thay tên mục xám trên dòng nhãn (Chất lượng truyện: “Chương N”) */
  context?: string
  /** Chữ trạng thái canh phải trên dòng nhãn (Chất lượng truyện: Còn mở · Đã sửa · Giữ nguyên) */
  aside?: { text: string; color: string }
}) {
  const [asking, setAsking] = useState(false)
  const level = reviewLevel(item)
  const kept = decision ?? item.decision
  const compact = variant === 'compact'
  const showQuote = !compact && variant !== 'gate'
  const showFix = variant === 'full' || variant === 'gate'
  const showStatus = variant === 'full'
  const showActions = variant === 'full' && level === 'warning' && (!!onKeep || !!onRequestFix)
  const canAct = showActions && !kept && !asking
  const status = level === 'error'
    ? t.review.queued
    : kept ? t.review.kept : asking ? t.review.asking : t.review.yours
  const statusColor = level === 'error' ? 'var(--kit-err-soft)' : kept ? 'var(--color-ok)' : 'var(--color-fg-4)'
  const shell = frame ? reviewIssueFrame(level, variant === 'full' ? 'card' : 'inset', compact) : undefined
  const quality = variant === 'quality'
  const label = context ?? item.refName

  const close = () => setAsking(false)

  return (
    <div className={frame && variant === 'full' ? 'card' : undefined}
      style={{ display: 'flex', flexDirection: 'column', gap: variant === 'full' ? 8 : quality ? 6 : 4, ...shell }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: quality ? 6 : 8, flexWrap: 'wrap' }}>
        <span style={labelChip(level)}>{level === 'error' ? t.review.error : t.review.warning}</span>
        {item.ref !== '' && (
          <span style={{
            height: 20, padding: '0 7px', display: 'inline-flex', alignItems: 'center', borderRadius: 6,
            background: variant === 'full' ? 'var(--kit-wash-6)' : 'var(--kit-wash-5)',
            color: 'var(--color-fg-2)', fontSize: 11.5, whiteSpace: 'nowrap', flex: 'none',
          }}>{item.ref}</span>
        )}
        {label !== '' && (
          <span title={label} style={{
            fontSize: 12, color: 'var(--color-fg-4)', minWidth: 0,
            whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
          }}>{label}</span>
        )}
        {aside && (
          <>
            <span style={{ flex: 1 }} />
            <span style={{ fontSize: 11.5, color: aside.color, whiteSpace: 'nowrap' }}>{aside.text}</span>
          </>
        )}
      </div>
      <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg)', lineHeight: 1.5 }}>{item.text}</p>
      {showQuote && item.quote !== '' && (
        <p style={{
          margin: 0, paddingLeft: 10, borderLeft: `2px solid ${level === 'error' ? 'var(--kit-errf-40)' : 'var(--kit-warn-35)'}`,
          fontFamily: 'var(--font-read)', fontSize: 13, color: 'var(--kit-fg-quote)', lineHeight: quality ? 1.55 : 1.6,
        }}>“{item.quote}”</p>
      )}
      {showFix && item.fix !== '' && (
        <p style={{ margin: 0, fontSize: 12.5, color: 'var(--color-fg-3)', lineHeight: 1.5 }}>
          <span style={{ color: 'var(--color-fg-2)' }}>{t.review.suggestion}</span> {item.fix}
        </p>
      )}
      {showStatus && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{
            fontSize: 12, color: statusColor, flex: 1, minWidth: 0,
            whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
          }}>{status}</span>
          {canAct && (
            <>
              {onKeep && (
                <button type="button" onClick={() => setAsking(true)} style={ghostAction}>{t.review.keep}</button>
              )}
              {onRequestFix && (
                <button type="button" onClick={() => onRequestFix(reviewFixPrefill(item))} style={{
                  height: 28, padding: '0 10px 0 8px', display: 'flex', alignItems: 'center', gap: 6,
                  borderRadius: 7, border: '1px solid var(--line-4)', background: 'transparent',
                  color: 'var(--color-fg-2)', fontSize: 12, whiteSpace: 'nowrap', flex: 'none',
                }}>
                  <Icon name="edit-2" size={14} />{t.review.requestFix}
                </button>
              )}
            </>
          )}
        </div>
      )}
      {showStatus && level === 'warning' && asking && (
        <ReviewKeepForm saving={saving} onCancel={close} onSave={async (reason) => {
          if (!onKeep) return
          const saved = await onKeep(reason)
          if (saved !== false) close()
        }} />
      )}
      {showStatus && level === 'warning' && kept && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: 'var(--color-fg-3)' }}>
          <span aria-hidden="true" style={{ color: 'var(--color-ok)' }}>✓</span>
          <span title={kept.reason} style={{
            flex: 1, minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
          }}>{reviewKeptLine(kept)}</span>
          {onUnkeep && (
            <button type="button" onClick={() => void onUnkeep()} disabled={saving} style={{
              border: 'none', background: 'transparent', color: 'var(--kit-fg-icon)', fontSize: 12,
              textDecoration: 'underline', whiteSpace: 'nowrap', flex: 'none', padding: 0,
            }}>{t.review.unkeep}</button>
          )}
        </div>
      )}
    </div>
  )
}

/**
 * Ô “Vì sao giữ nguyên?” (GAP-47): 3 chip gợi ý + ô tự ghi + Huỷ/Lưu lý do. Dùng trong thẻ
 * ReviewIssue và ở “Giữ nguyên…” của Chất lượng truyện (cùng một luồng giữ nguyên).
 */
export function ReviewKeepForm({ onSave, onCancel, saving = false }: {
  onSave: (reason: string) => void | Promise<void>
  onCancel: () => void
  saving?: boolean
}) {
  const [reason, setReason] = useState('')
  const canSave = reason.trim() !== '' && !saving
  const save = async () => {
    if (!canSave) return
    await onSave(reason.trim())
  }
  return (
    <div role="group" aria-label={t.review.reasonLabel} style={{
      display: 'flex', flexDirection: 'column', gap: 8, padding: 10, borderRadius: 10,
      background: 'var(--color-surface-0)', border: '1px solid var(--line-3)',
    }}>
      <span style={{ fontSize: 12.5, fontWeight: 500, color: 'var(--color-fg)' }}>{t.review.keepAsk}</span>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
        {t.review.keepReasons.map((suggestion) => {
          const on = reason === suggestion
          return (
            <button key={suggestion} type="button" aria-pressed={on} onClick={() => setReason((current) => (current === suggestion ? '' : suggestion))} style={{
              height: 26, padding: '0 9px', borderRadius: 7, fontSize: 12, whiteSpace: 'nowrap',
              border: `1px solid ${on ? 'var(--kit-acc-50)' : 'var(--line-4)'}`,
              background: on ? 'var(--tint-accent)' : 'transparent',
              color: on ? 'var(--color-fg)' : 'var(--color-fg-2)',
            }}>{suggestion}</button>
          )
        })}
      </div>
      <label style={{ display: 'flex' }}>
        <span className="sr-only">{t.review.reasonLabel}</span>
        <input className="field" type="text" value={reason} placeholder={t.review.reasonPlaceholder}
          onChange={(event) => setReason(event.target.value)}
          onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); void save() } }}
          style={{ flex: 1, minWidth: 0, height: 32, padding: '0 10px', borderRadius: 8, fontSize: 12.5,
            border: '1px solid var(--kit-wash-12)', background: 'var(--color-surface-1)', color: 'var(--color-fg)', fontFamily: 'inherit' }} />
      </label>
      <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)', lineHeight: 1.5 }}>{t.review.keepNote}</span>
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 6 }}>
        <button type="button" onClick={onCancel} style={ghostAction}>{t.review.cancel}</button>
        <button type="button" onClick={() => void save()} disabled={!canSave} style={{
          height: 28, padding: '0 12px', borderRadius: 7, border: 'none', fontSize: 12, fontWeight: 600,
          background: canSave ? 'var(--color-fg)' : 'var(--line-3)',
          color: canSave ? 'var(--color-bg)' : 'var(--kit-fg-arrow)',
          whiteSpace: 'nowrap', flex: 'none',
        }}>{t.review.saveReason}</button>
      </div>
    </div>
  )
}
