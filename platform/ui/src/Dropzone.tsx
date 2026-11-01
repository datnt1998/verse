// Vùng thả tệp (.vv-drop) + dòng tệp (.vv-filerow) của kit — Library.dc.html ›
// import=ready|drag|importing|done|partial|zip|offline. Dùng chung cho Nhập
// template, Nhập prompt và Thêm hiệu ứng: mỗi tệp một dòng trạng thái thật
// (chờ / đang nhập / xong / lỗi), không bịa phần trăm.
import { useId, useState, type DragEvent, type KeyboardEvent, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit, line, radius } from './tokens.stylex'
import { Icon } from './Icon'
import { Button } from './Button'
import { ProgressBar, Spinner } from './feedback'
import './kit.css'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'

const sx = stylex.create({
  drop: {
    display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
    gap: 6, width: '100%', minHeight: 168, padding: 20, textAlign: 'center',
    borderRadius: radius['--radius-tile'], borderWidth: 1.5, borderStyle: 'dashed',
    borderColor: kit['--kit-wash-14'], backgroundColor: kit['--kit-wash-4'],
    color: color['--color-fg-2'], cursor: 'pointer',
    transition: {
      default: 'border-color .2s, background-color .2s, transform .2s',
      [NO_MOTION]: 'none',
    },
  },
  dropHover: {
    borderColor: { default: kit['--kit-wash-14'], ':hover': kit['--kit-wash-22'] },
  },
  dropDrag: {
    borderColor: line['--line-focus'], backgroundColor: kit['--kit-acc-10'], transform: 'scale(1.01)',
  },
  dropBusy: { cursor: 'progress' },
  dropIcon: { display: 'flex', color: color['--color-fg-4'], marginBottom: 2 },
  dropIconDrag: { color: color['--color-accent'] },
  dropTitle: { fontSize: 14, fontWeight: 500, color: color['--color-fg'] },
  dropHint: { fontSize: 12, color: color['--color-fg-4'] },
  dropAction: { marginTop: 8 },
  rows: { display: 'flex', flexDirection: 'column', gap: 6 },
  row: {
    display: 'flex', alignItems: 'center', gap: 10, minHeight: 44,
    paddingTop: 8, paddingBottom: 8, paddingInline: 10,
    borderRadius: radius['--radius-control'], backgroundColor: kit['--kit-wash-4'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'],
  },
  rowWait: { opacity: .6 },
  rowIcon: {
    width: 28, height: 28, flex: '0 0 auto', display: 'flex', alignItems: 'center', justifyContent: 'center',
    borderRadius: 8, backgroundColor: kit['--kit-wash-5'], color: color['--color-fg-3'],
  },
  rowIconOk: { backgroundColor: kit['--kit-ok-12'], color: color['--color-ok'] },
  rowIconFail: { backgroundColor: kit['--kit-err-12'], color: color['--color-err'] },
  rowBody: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 3 },
  rowName: { fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  rowMeta: { fontSize: 12, color: color['--color-fg-4'] },
  rowMetaFail: { color: color['--color-err'] },
  rowProgress: { marginTop: 3 },
  warnButton: { color: color['--color-warn'], flex: '0 0 auto' },
  warnings: {
    margin: 0, paddingTop: 8, paddingBottom: 8, paddingInline: 12,
    borderRadius: radius['--radius-control'], backgroundColor: kit['--kit-wash-4'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'],
    display: 'flex', flexDirection: 'column', gap: 4,
    fontSize: 12, color: color['--color-fg-2'], lineHeight: 1.55,
    listStyle: 'disc', listStylePosition: 'inside',
  },
})

export type DropzoneProps = {
  /** Câu việc cần làm: “Kéo tệp .zip vào đây”. */
  title: ReactNode
  /** Dòng phụ: “Thả nhiều tệp một lúc được”. */
  hint?: ReactNode
  icon?: ReactNode
  /** Nút chọn tệp (thường `Button size="sm"`); vùng thả cũng bấm được. */
  action?: ReactNode
  /** Bấm vùng thả / Enter / Space / nút chọn tệp. */
  onPick?: () => void
  onDrop: (files: File[]) => void
  multiple?: boolean
  disabled?: boolean
  /** Đang nhập: khoá thả, con trỏ chờ. */
  busy?: boolean
  /** Tên đọc của vùng thả. */
  label: string
  /** Tiêu đề/dòng phụ khi đang kéo tệp qua (mặc định giữ nguyên). */
  dragTitle?: ReactNode | ((count: number) => ReactNode)
  dragHint?: ReactNode | ((count: number) => ReactNode)
  xstyle?: stylex.StyleXStyles
}

/** Vùng thả tệp của kit: viền nét đứt, kéo vào đổi màu nhấn, bấm hoặc Enter để chọn. */
export function Dropzone({ title, hint, icon, action, onPick, onDrop, multiple = true, disabled = false, busy = false, label, dragTitle, dragHint, xstyle }: DropzoneProps) {
  const [dragging, setDragging] = useState(false)
  const [count, setCount] = useState(0)
  const locked = disabled || busy
  const drop = stylex.props(sx.drop, !locked && sx.dropHover, dragging && !locked && sx.dropDrag, locked && sx.dropBusy, xstyle)
  const fileCount = (event: DragEvent<HTMLDivElement>) => {
    const items = event.dataTransfer.items
    return items && items.length > 0 ? Array.from(items).filter((item) => item.kind === 'file').length : event.dataTransfer.files.length
  }
  const enter = (event: DragEvent<HTMLDivElement>) => {
    if (locked) return
    event.preventDefault()
    const n = fileCount(event)
    setCount(n)
    if (n > 0) setDragging(true)
  }
  const leave = (event: DragEvent<HTMLDivElement>) => {
    if (event.currentTarget.contains(event.relatedTarget as Node | null)) return
    setDragging(false)
  }
  const dropped = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    setDragging(false)
    if (locked) return
    const files = Array.from(event.dataTransfer.files)
    if (files.length > 0) onDrop(multiple ? files : files.slice(0, 1))
  }
  const keys = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'Enter' && event.key !== ' ') return
    event.preventDefault()
    if (!locked) onPick?.()
  }
  const hintNode = dragging && dragHint !== undefined
    ? typeof dragHint === 'function' ? dragHint(count) : dragHint
    : hint
  const titleNode = dragging && dragTitle !== undefined
    ? typeof dragTitle === 'function' ? dragTitle(count) : dragTitle
    : title
  return (
    <div
      role="button"
      tabIndex={locked ? -1 : 0}
      aria-label={label}
      aria-disabled={locked || undefined}
      aria-busy={busy || undefined}
      className={drop.className}
      style={drop.style}
      onClick={() => { if (!locked) onPick?.() }}
      onKeyDown={keys}
      onDragEnter={enter}
      onDragOver={(event) => { if (!locked) event.preventDefault() }}
      onDragLeave={leave}
      onDrop={dropped}
    >
      <span aria-hidden="true" {...stylex.props(sx.dropIcon, dragging && sx.dropIconDrag)}>{icon ?? <Icon name="document-download" size={22} />}</span>
      <span {...stylex.props(sx.dropTitle)}>{titleNode}</span>
      {hintNode !== undefined && hintNode !== null && <span {...stylex.props(sx.dropHint)}>{hintNode}</span>}
      {action !== undefined && <span {...stylex.props(sx.dropAction)} onClick={(event) => event.stopPropagation()}>{action}</span>}
    </div>
  )
}

export type FileRowStatus = 'ok' | 'active' | 'wait' | 'fail'

export type FileRowProps = {
  name: string
  /** Dòng phụ: “Đã nhập · 14 media vào kho media”, “Đang nhập…”, lý do lỗi. */
  meta?: ReactNode
  status?: FileRowStatus
  /** Ghi đè icon mặc định của trạng thái. */
  icon?: ReactNode
  /** 0–100 khi biết; bỏ trống = dải chạy (máy chủ đang xử lý). */
  progress?: number
  /** Danh sách cảnh báo mở tại chỗ; `warningsLabel` là nhãn nút (“2 điều cần biết”). */
  warnings?: ReactNode[]
  warningsLabel?: string
  defaultWarningsOpen?: boolean
  /** Nút cuối dòng (vd “Thử lại”). */
  trailing?: ReactNode
  xstyle?: stylex.StyleXStyles
}

/** Một tệp trong hộp nhập: đã nhập / đang nhập / chờ / chưa nhập được. */
export function FileRow({ name, meta, status = 'wait', icon, progress, warnings, warningsLabel, defaultWarningsOpen = false, trailing, xstyle }: FileRowProps) {
  const [open, setOpen] = useState(defaultWarningsOpen)
  const listId = useId()
  const hasWarnings = warnings !== undefined && warnings.length > 0
  const row = stylex.props(sx.row, status === 'wait' && sx.rowWait, xstyle)
  const badge = stylex.props(sx.rowIcon, status === 'ok' && sx.rowIconOk, status === 'fail' && sx.rowIconFail)
  const warnList = stylex.props(sx.warnings)
  return (
    <>
      <div className={row.className} style={row.style}>
        <span aria-hidden="true" className={badge.className} style={badge.style}>
          {icon ?? (status === 'active' ? <Spinner size={13} /> : <Icon name={status === 'ok' ? 'tick' : status === 'fail' ? 'danger' : 'doc'} size={15} />)}
        </span>
        <span {...stylex.props(sx.rowBody)}>
          <span {...stylex.props(sx.rowName)} title={name}>{name}</span>
          {meta !== undefined && <span {...stylex.props(sx.rowMeta, status === 'fail' && sx.rowMetaFail)}>{meta}</span>}
          {status === 'active' && (
            <span {...stylex.props(sx.rowProgress)}>
              {progress === undefined
                ? <ProgressBar indeterminate height={3} tone="info" />
                : <ProgressBar value={progress} height={3} tone="info" />}
            </span>
          )}
        </span>
        {hasWarnings && (
          <Button variant="quiet" size="sm" aria-expanded={open} aria-controls={open ? listId : undefined}
            xstyle={sx.warnButton} onClick={() => setOpen((value) => !value)}
            icon={<Icon name="arrow-d" size={13} style={{ transform: open ? 'rotate(180deg)' : undefined }} />}>
            {warningsLabel ?? String(warnings.length)}
          </Button>
        )}
        {trailing}
      </div>
      {hasWarnings && open && (
        <ul id={listId} className={warnList.className} style={warnList.style}>
          {warnings.map((warning, index) => <li key={index}>{warning}</li>)}
        </ul>
      )}
    </>
  )
}
