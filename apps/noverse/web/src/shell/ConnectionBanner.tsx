// LockedDialog — 409 errBookLocked (Dialogs.dc.html › locked).
import { t } from '../i18n/vi'
import { Icon } from '@noverse/ui'

export function LockedDialog({ storyTitle, onReadOnly, onRetry }: {
  storyTitle?: string
  onReadOnly: () => void
  onRetry: () => void
}) {
  return (
    <div style={{ position: 'fixed', inset: 0, zIndex: 40 }}>
      <div aria-hidden="true" style={{ position: 'absolute', inset: 0, background: 'rgba(5,5,7,0.62)' }} />
      <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16 }}>
        <div className="dlg" role="alertdialog" aria-label={t.locked.aria}
          style={{ width: 'min(480px, 100%)', padding: 22, display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Icon name="lock" size={26} style={{ color: 'var(--color-warn)' }} />
          <h2 style={{ margin: 0, fontSize: 17, fontWeight: 600 }}>{t.locked.title}</h2>
          <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-3)' }}>{t.locked.body}{storyTitle ? ` (${storyTitle})` : ''}</p>
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <button type="button" className="hov-ghost" onClick={onReadOnly}
              style={{ height: 36, padding: '0 14px', borderRadius: 9, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 13 }}>
              {t.locked.readonly}
            </button>
            <button type="button" className="hov-primary" onClick={onRetry}
              style={{ height: 36, padding: '0 14px 0 12px', display: 'flex', alignItems: 'center', gap: 6, borderRadius: 9, border: 'none', background: 'var(--color-fg)', color: 'var(--color-bg)', fontSize: 13, fontWeight: 600 }}>
              <Icon name="refresh-2" size={15} />{t.locked.retry}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
