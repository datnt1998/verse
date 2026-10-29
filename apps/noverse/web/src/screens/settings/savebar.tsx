// SaveBar — thanh nổi đáy “N thay đổi cho {cấp} · có hiệu lực từ chương X” · Huỷ · Áp dụng/Lưu (Settings.dc.html .savebar).
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import type { ConfigScope } from '../../api/types'

export const SCOPE_NOUN: Record<ConfigScope, string> = { story: 'truyện này', project: 'dự án', global: 'mọi truyện' }

export const dirtyText = (count: number, scope: ConfigScope, hint?: string): string =>
  `${count} thay đổi cho ${SCOPE_NOUN[scope]}${hint ? ` · ${hint}` : ''}`

export function SaveBar({ text, applyLabel, saving, disabled, onApply, onCancel }: {
  text: string
  applyLabel: 'Áp dụng' | 'Lưu'
  saving: boolean
  disabled: boolean
  onApply: () => void
  onCancel: () => void
}) {
  return (
    <div className="st-savebar" role="region" aria-label="Thay đổi chưa áp dụng">
      <Icon name="info-circle" size={17} style={{ color: '#C9A6F5' }} />
      <span style={{ fontSize: 13.5 }}>{text}</span>
      <button type="button" className="hov-ghost" onClick={onCancel} disabled={saving}
        style={{ height: 34, padding: '0 14px', borderRadius: 10, border: 'none', background: 'transparent', color: '#A0A0AB', fontSize: 13 }}>Huỷ</button>
      <button type="button" className="hov-primary" onClick={onApply} disabled={disabled} aria-busy={saving || undefined}
        style={{ height: 34, padding: '0 16px', borderRadius: 10, border: 'none', background: '#EDEDEF', color: '#0B0B0D', fontSize: 13, fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: 8, opacity: disabled && !saving ? 0.6 : 1 }}>
        {saving && <Spinner size={13} color="#0B0B0D" />}{applyLabel}
      </button>
    </div>
  )
}
