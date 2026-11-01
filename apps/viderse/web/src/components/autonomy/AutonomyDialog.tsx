// Hộp xác nhận nâng Mức tự chủ (artboard Dialogs › dialog=autonomy). Khác hộp
// ConfirmDialog của kit ở bố cục: rộng 520, ô icon xanh lá, nút Đóng ở góc, danh
// sách ba dòng nằm dưới phần đầu và chân hộp có gợi ý "Esc đóng". Vẫn là hộp
// modal chuẩn: Esc = Huỷ, Tab khoá trong hộp, nền #root bị inert khi mở.
import * as stylex from '@stylexjs/stylex'
import { useCallback, useEffect, useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { createRoot } from 'react-dom/client'
import { Button, Icon, Kbd, useEscapeLayer } from '@noverse/ui'
import { t } from '../../i18n/vi'
import { dialogStyles as sx } from './dialog.stylex'

export type AutonomyDialogItem = { icon: string; tone: 'warn' | 'note'; text: string }

type Props = {
  title: string
  lead: string
  items: AutonomyDialogItem[]
  confirmLabel: string
  onConfirm: () => void | Promise<void>
  onClose: (accepted: boolean) => void
}

const CLOSE_MS = 180

function AutonomyDialog({ title, lead, items, confirmLabel, onConfirm, onClose }: Props) {
  const id = useId()
  const dialog = useRef<HTMLElement>(null)
  const primary = useRef<HTMLButtonElement>(null)
  const timer = useRef<number>(undefined)
  const mounted = useRef(true)
  const [closing, setClosing] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const tt = t.autonomy

  const close = useCallback((accepted: boolean) => {
    setClosing(true)
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => onClose(accepted), CLOSE_MS)
  }, [onClose])
  useEscapeLayer(true, () => { if (!pending) close(false) })

  useEffect(() => {
    mounted.current = true
    const previous = document.activeElement as HTMLElement | null
    const root = document.getElementById('root')
    const wasInert = root?.hasAttribute('inert')
    root?.setAttribute('inert', '')
    primary.current?.focus()
    return () => {
      mounted.current = false
      window.clearTimeout(timer.current)
      if (root && !wasInert) root.removeAttribute('inert')
      previous?.focus()
    }
  }, [])

  const run = async () => {
    if (pending || closing) return
    setPending(true)
    setError('')
    try {
      await onConfirm()
      if (mounted.current) close(true)
    } catch (err) {
      if (mounted.current) setError(err instanceof Error && err.message ? err.message : tt.saveError)
    } finally {
      if (mounted.current) setPending(false)
    }
  }

  return createPortal(
    <div {...stylex.props(sx.scrim, closing && sx.scrimClosing)}>
      <section
        ref={dialog}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby={`${id}-title`}
        aria-describedby={`${id}-lead`}
        {...stylex.props(sx.dialog, closing && sx.dialogClosing)}
        onKeyDown={(event) => {
          if (event.key !== 'Tab') return
          const focusable = Array.from(dialog.current?.querySelectorAll<HTMLElement>('button:not(:disabled)') ?? [])
          const first = focusable[0]
          const last = focusable[focusable.length - 1]
          if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
          else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
        }}
      >
        <div {...stylex.props(sx.head)}>
          <span {...stylex.props(sx.chip)} aria-hidden="true"><Icon name="tick-b" size={19} /></span>
          <div {...stylex.props(sx.headText)}>
            <h2 id={`${id}-title`} {...stylex.props(sx.title)}>{title}</h2>
            <p id={`${id}-lead`} {...stylex.props(sx.lead)}>{lead}</p>
          </div>
          <button type="button" aria-label={t.common.close} {...stylex.props(sx.closeButton)} onClick={() => { if (!pending) close(false) }}>
            <Icon name="close" size={16} />
          </button>
        </div>
        <div {...stylex.props(sx.list)}>
          {items.map((item) => (
            <div key={item.icon} {...stylex.props(sx.item)}>
              <Icon name={item.icon} size={15} xstyle={[sx.itemIcon, item.tone === 'warn' ? sx.itemWarn : sx.itemNote]} />
              <span>{item.text}</span>
            </div>
          ))}
        </div>
        {error && <div role="alert" {...stylex.props(sx.error)}>{error}</div>}
        <div {...stylex.props(sx.footer)}>
          <span {...stylex.props(sx.hint)}><Kbd>Esc</Kbd> {t.common.closeHint}</span>
          <Button variant="ghost" disabled={pending} onClick={() => close(false)}>{t.common.cancel}</Button>
          <Button ref={primary} variant="primary" pending={pending} onClick={() => void run()}>{confirmLabel}</Button>
        </div>
      </section>
    </div>,
    document.body,
  )
}

export function openAutonomyDialog(props: Omit<Props, 'onClose'>): Promise<boolean> {
  return new Promise((resolve) => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    root.render(<AutonomyDialog {...props} onClose={(accepted) => { root.unmount(); container.remove(); resolve(accepted) }} />)
  })
}
