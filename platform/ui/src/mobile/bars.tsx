// Thanh chọn nhiều + banner mất mạng của điện thoại.
//
// SelectionBar: thanh dưới cùng thay chỗ thanh tab khi đang chọn nhiều
// (artboard MExplore `.vv-selbar`: nền `--color-scope-bg`, kẻ trên, chữ đếm 14/600).
// OfflineBanner: dải cảnh báo mất mạng — chỉ hiện khi mất mạng ĐÃ BỀN 3 giây
// (tránh nháy), đếm ngược lần thử lại, nút "Thử ngay", và làm mờ nội dung 55%.
import { useEffect, useState, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit, line, radius } from '../tokens.stylex'
import { getKitLabels } from '../labels'
import { useSustained, useTabMode } from './runtime'
import './mobile.css'

const styles = stylex.create({
  selection: {
    flex: '0 0 auto',
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
    paddingInline: 16,
    paddingTop: 10,
    borderTopWidth: 1,
    borderTopStyle: 'solid',
    borderTopColor: line['--line-3'],
    backgroundColor: color['--color-scope-bg'],
  },
  selectionHead: { display: 'flex', alignItems: 'center', gap: 8, minHeight: 32 },
  selectionCount: { flex: '1 1 auto', minWidth: 0, fontSize: 14, fontWeight: 600, color: color['--color-fg'] },
  selectionActions: { display: 'flex', alignItems: 'center', gap: 8, flex: '0 0 auto' },
  selectionHint: { fontSize: 11.5, lineHeight: 1.5, color: kit['--kit-fg-sub'] },

  banner: {
    position: 'fixed',
    left: 12,
    right: 12,
    top: 'calc(var(--m-top) + 52px)',
    zIndex: 65,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
    minHeight: 64,
    paddingInline: 16,
    borderRadius: 14,
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: kit['--kit-warn-30'],
    backgroundColor: color['--color-surface-1'],
    color: color['--color-warn'],
    fontSize: 12.5,
    lineHeight: 1.45,
    boxShadow: kit['--kit-shadow-toast'],
  },
  bannerText: { flex: '1 1 auto', minWidth: 0 },
  bannerButton: {
    flex: '0 0 auto',
    minHeight: 44,
    paddingInline: 12,
    borderRadius: radius['--radius-row'],
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: kit['--kit-warn-45'],
    backgroundColor: 'transparent',
    color: color['--color-warn'],
    fontSize: 12.5,
    fontWeight: 500,
    cursor: 'pointer',
    ':active': { backgroundColor: kit['--kit-warn-12'] },
    ':focus-visible': { outlineWidth: 2, outlineStyle: 'solid', outlineColor: line['--line-focus'], outlineOffset: 2 },
  },
})

export type SelectionBarProps = {
  /** số mục đang chọn — hiển thị "Đã chọn N" khi không truyền `label` */
  count?: number
  label?: ReactNode
  /** nút thao tác bên phải (Bỏ chọn, Xoá…) */
  actions?: ReactNode
  /** dòng phụ dưới dòng đếm */
  hint?: ReactNode
  xstyle?: stylex.StyleXStyles
}

/**
 * Thanh chọn nhiều: nằm ở cuối cột màn hình và ẩn thanh tab (`useTabMode('hide')`).
 * Đặt nó làm `footer` của `MobilePage` (hoặc phần tử cuối của cột trang).
 */
export function SelectionBar({ count = 0, label, actions, hint, xstyle }: SelectionBarProps) {
  useTabMode('hide')
  const sx = stylex.props(styles.selection, xstyle)
  return (
    <div className={`m-selbar ${sx.className ?? ''}`} style={{ ...sx.style, paddingBottom: 'calc(4px + max(26px, var(--m-safe-bottom)) + var(--kb, 0px))' }} role="region" aria-label={typeof label === 'string' ? label : undefined}>
      <div {...stylex.props(styles.selectionHead)}>
        <span {...stylex.props(styles.selectionCount)}>{label ?? `Đã chọn ${count}`}</span>
        {actions && <span {...stylex.props(styles.selectionActions)}>{actions}</span>}
      </div>
      {hint != null && <span {...stylex.props(styles.selectionHint)}>{hint}</span>}
    </div>
  )
}

export type OfflineBannerProps = {
  /** đang mất mạng (chưa bền thì chưa hiện) */
  offline: boolean
  /** giây còn lại tới lần thử lại tự động; 0/không có thì chỉ hiện câu mất mạng */
  retryIn?: number
  onRetry: () => void
  /** phải mất mạng liên tục bao lâu mới hiện, mặc định 3000ms */
  sustainedMs?: number
  xstyle?: stylex.StyleXStyles
}

/**
 * Dải mất mạng: chỉ hiện sau khi `offline` bền `sustainedMs`, đếm ngược lần thử lại,
 * có nút "Thử ngay". Trong lúc hiện, `<html data-offline>` bật luật làm mờ nội dung 55%
 * trong `mobile.css` (nội dung vẫn đọc được, không cạnh tranh với banner).
 */
export function OfflineBanner({ offline, retryIn, onRetry, sustainedMs = 3000, xstyle }: OfflineBannerProps) {
  const visible = useSustained(offline, sustainedMs)
  const t = getKitLabels()
  useEffect(() => {
    const root = document.documentElement
    if (!visible) return
    root.dataset.offline = ''
    return () => { delete root.dataset.offline }
  }, [visible])
  const [seconds, setSeconds] = useState(retryIn ?? 0)
  useEffect(() => { setSeconds(retryIn ?? 0) }, [retryIn, visible])
  useEffect(() => {
    if (!visible || !seconds) return
    const timer = setTimeout(() => setSeconds((n) => Math.max(0, n - 1)), 1000)
    return () => clearTimeout(timer)
  }, [visible, seconds])
  if (!visible) return null
  const sx = stylex.props(styles.banner, xstyle)
  return (
    <div className={sx.className} style={sx.style} role="status" aria-live="polite">
      <span {...stylex.props(styles.bannerText)}>
        {seconds > 0 ? t.mobile.offlineCountdown(seconds) : t.mobile.offline}
      </span>
      <button type="button" {...stylex.props(styles.bannerButton)} onClick={onRetry}>{t.mobile.retryNow}</button>
    </div>
  )
}
