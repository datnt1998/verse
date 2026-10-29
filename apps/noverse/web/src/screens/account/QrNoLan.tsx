// A1 › qrNoLan (Login.dc.html): chưa có địa chỉ nào điện thoại vào được. Dùng chung cho hộp thoại QR và bước Điện thoại.
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { t } from '../../i18n/vi'

export function QrNoLan({ canEnable, busy, error, onEnable, onRetry, height }: {
  /** Chỉ quản trị bật được “Wi-Fi nhà” (ngay trên máy chủ). */
  canEnable: boolean
  busy: boolean
  error?: string
  onEnable: () => void
  /** Có lỗi tải mã (không phải thiếu mạng) thì cho thử lại. */
  onRetry?: () => void
  height?: number
}) {
  return (
    <div style={{
      height, alignSelf: 'stretch', borderRadius: 18, border: '1.5px dashed rgba(255,255,255,0.14)', padding: 22,
      display: 'flex', flexDirection: 'column', justifyContent: 'center', gap: 12,
    }}>
      <Icon name="wifi" size={28} style={{ color: 'var(--color-warn)' }} />
      <span style={{ fontSize: 15, fontWeight: 600 }}>{t.qr.noLan}</span>
      <span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>{t.qr.noLanBody}</span>
      {busy ? (
        <>
          <span role="status" style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 13.5, color: 'var(--color-fg)' }}><Spinner />{t.qr.enablingLan}</span>
          <span style={{ fontSize: 12, color: '#7E7E8A' }}>{t.qr.enablingLanHint}</span>
        </>
      ) : canEnable && (
        <>
          <Button variant="primary" onClick={onEnable} style={{ alignSelf: 'flex-start', height: 38, padding: '0 16px', borderRadius: 10, fontSize: 13.5, fontWeight: 600 }}>
            <Icon name="wifi" size={16} />{t.qr.enableLan}
          </Button>
          <span style={{ fontSize: 12, color: '#7E7E8A' }}>{t.qr.enableLanHint}</span>
        </>
      )}
      {error && (
        <span role="alert" style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-err)' }}>
          <Icon name="warning-2" size={14} />{error}
        </span>
      )}
      {onRetry && !busy && <Button variant="ghost" onClick={onRetry} style={{ alignSelf: 'flex-start' }}>Thử lại</Button>}
      <span style={{ fontSize: 12, color: '#7E7E8A' }}>{t.qr.tailscaleHint}</span>
    </div>
  )
}
