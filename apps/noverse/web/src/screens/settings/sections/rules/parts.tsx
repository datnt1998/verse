// Phần trình bày dùng chung: chip “Noverse hiểu là”, thẻ quy tắc, khung cam “chưa chắc”.
import type { ReactNode } from 'react'
import type { RuleChip } from '../../../../api/types'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { CHIP_STYLE, chipText, isUncertain, retryLine, type RuleEntry, type RuleRetry } from './logic'
import './rules.css'

/** Một chip: nền/chữ theo loại (đỏ nhạt · cam · xanh · tím), icon 12px. `degraded` → nhãn “Lời dặn”. */
export function UChip({ chip, degraded = false, className = 'rl-uchip' }: { chip: RuleChip; degraded?: boolean; className?: string }) {
  const style = CHIP_STYLE[chip.kind]
  return <span className={className} style={{ background: style.bg, color: style.fg }}>
    <Icon name={style.icon} size={12} />{chipText(chip, degraded)}
  </span>
}

export const UChips = ({ entry, className }: { entry: RuleEntry; className?: string }) =>
  <>{entry.chips.map((chip, index) => <UChip key={`${chip.kind}:${chip.text}:${index}`} chip={chip} degraded={entry.status === 'degraded'} className={className} />)}</>

/** Thẻ “Noverse đang đọc quy tắc…” (shimmer + spinner) trong lúc chuẩn hoá. */
export function ThinkingCard({ text }: { text: string }) {
  return <div className="rl-rule rl-drawer" role="status">
    <span style={{ fontSize: 14.5, color: 'var(--color-fg)' }}>{text}</span>
    <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5 }}>
      <Spinner size={12} /><span className="rl-thinking">Noverse đang đọc quy tắc…</span>
    </span>
  </div>
}

/** Thẻ “máy chủ đang bận, thử lại lần 2/3” (Settings.dc.html › rules=retry): viền cam + hai nút. */
export function RetryCard({ retry, onSaveRaw, onCancel }: { retry: RuleRetry; onSaveRaw: () => void; onCancel: () => void }) {
  return <div className="rl-rule rl-drawer" role="status" style={{ borderColor: 'var(--kit-warn-30)' }}>
    <span style={{ fontSize: 14.5, color: 'var(--color-fg)' }}>{retry.text}</span>
    <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5, color: 'var(--kit-warn-soft)' }}>
      <Spinner size={12} />{retryLine(retry)}
    </span>
    <span style={{ display: 'flex', gap: 8, marginTop: 2 }}>
      <button type="button" className="rl-btn-ghost" onClick={onSaveRaw}
        style={{ height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid var(--kit-line-12)', background: 'transparent', color: 'var(--color-fg)', fontSize: 12 }}>
        Lưu nguyên văn ngay
      </button>
      <button type="button" className="rl-btn-ghost" onClick={onCancel}
        style={{ height: 28, padding: '0 10px', borderRadius: 8, border: 'none', background: 'transparent', color: 'var(--color-fg-3)', fontSize: 12 }}>
        Huỷ
      </button>
    </span>
  </div>
}

/** Khung cam: lý do chưa chắc (câu đầu của backend) + 0–3 cách hiểu + “Giữ như lời dặn”. */
export function UnsureFrame({ entry, busy, onPick, readOnly = false }: { entry: RuleEntry; busy: boolean; onPick: (option: string | null) => void; readOnly?: boolean }) {
  if (!isUncertain(entry)) return null
  return <div style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: '10px 12px', borderRadius: 10, background: 'var(--kit-warn-6)', border: '1px solid var(--kit-warn-20)' }}>
    <span style={{ fontSize: 13, color: 'var(--kit-warn-body)', display: 'flex', gap: 8 }}>
      <Icon name="message-question" size={15} style={{ color: 'var(--color-warn)', marginTop: 2 }} />{entry.note}
    </span>
    {!readOnly && <span style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
      {[...entry.options.slice(0, 3), null].map((option) => (
        <button key={option ?? 'keep'} type="button" className="rl-tpl" disabled={busy} onClick={() => onPick(option)}
          style={{ borderColor: 'var(--kit-warn-30)', color: 'var(--color-warn)' }}>{option ?? 'Giữ như lời dặn'}</button>
      ))}
    </span>}
  </div>
}

export function GroupHead({ icon, color, label, sub, link, onLink }: { icon: string; color: string; label: string; sub: string; link?: string; onLink?: () => void }): ReactNode {
  return <div className="rl-ghd">
    <Icon name={icon} size={15} style={{ color }} /><b>{label}</b><span>{sub}</span><span style={{ flex: '1 1 auto' }} />
    {link && <button type="button" className="rl-btn-ghost" onClick={onLink} style={{ height: 26, padding: '0 8px', border: 'none', borderRadius: 7, background: 'transparent', color: 'var(--color-accent)', fontSize: 12.5, whiteSpace: 'nowrap' }}>{link}</button>}
  </div>
}
