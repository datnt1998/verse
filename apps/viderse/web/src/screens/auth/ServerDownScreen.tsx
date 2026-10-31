// Màn lỗi khi không kiểm được phiên: mất kết nối (status 0) hoặc kho tài
// khoản/máy chủ trả 5xx. Đứng ở cổng THAY CHO Login/Setup — không bao giờ
// đoán "store trống" hay "chưa đăng nhập" khi server không trả lời được.
// Nút Thử lại gọi lại /api/me qua refresh().
import { useState } from 'react'
import { Button, EmptyState, Icon } from '@noverse/ui'
import { useSession } from '../../api/session'
import { t } from '../../i18n/vi'
import { AuthShell } from './parts'

export function ServerDownScreen() {
  const { refresh } = useSession()
  const [busy, setBusy] = useState(false)

  const retry = async () => {
    if (busy) return
    setBusy(true)
    try { await refresh() } finally { setBusy(false) }
  }

  return (
    <AuthShell>
      <EmptyState
        variant="frame"
        icon="cloud-connection"
        title={t.auth.serverDownTitle}
        hint={t.auth.serverDownBody}
        action={
          <Button variant="primary" onClick={() => void retry()} pending={busy} disabled={busy}
            style={{ minHeight: 'var(--vv-s-44)', justifyContent: 'center', fontSize: 14, fontWeight: 600 }}>
            <Icon name="refresh-2" size={17} />{t.common.retry}
          </Button>
        }
      />
    </AuthShell>
  )
}
