// A1 › devices · Thiết bị & bảo mật (Login.dc.html). Danh sách phiên + đổi mật khẩu.
import { useEffect, useRef, useState } from 'react'
import { ConflictError, changePassword, mySessions, revokeSession } from '../../api/client'
import { useSession } from '../../api/session'
import type { Session } from '../../api/types'
import { t } from '../../i18n/vi'
import { Dialog } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { AuthInput, StrengthMeter } from '../auth/parts'
import { SessionList } from './shared'
import { keys, useAction, useConnection, useResource } from '../../data'
import { Button } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { confirmAllDevicesLogout, confirmDeviceLogout } from '../../ui/confirmPresets'
import { toast } from '@noverse/ui'
import { useSaveShortcut } from '@noverse/ui'
import { ConflictBar } from '@noverse/ui'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'

export function DevicesDialog({ onClose }: { onClose: () => void }) {
  const { user, refresh, endSession } = useSession()
  const baselineVersion = useRef(user?.version)
  const [dismissedConflict, setDismissedConflict] = useState<unknown>(null)
  const resource = useResource(keys.sessions(), mySessions)
  const sessions = resource.data?.sessions ?? []
  const connection = useConnection()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [repeat, setRepeat] = useState('')
  useEffect(() => {
    if (!current && !next && !repeat) baselineVersion.current = user?.version
  }, [user?.version, current, next, repeat])
  const revokeAction = useAction(revokeSession, { invalidate: [keys.sessions(), keys.users()] })
  const revoke = (session: Session) => {
    void confirmDeviceLogout({ onConfirm: async () => {
      if (!await revokeAction.run(session.id)) throw new Error('Không đăng xuất được thiết bị')
    } }, { name: session.device })
  }
  const revokeOthers = () => {
    const others = sessions.filter((session) => !session.current)
    void confirmAllDevicesLogout({ onConfirm: async () => {
      for (const session of others) {
        if (!await revokeAction.run(session.id)) throw new Error('Không đăng xuất được thiết bị')
      }
    } }, { count: others.length })
  }
  // Đổi xong: máy chủ thu hồi MỌI phiên kể cả phiên này → báo đổi thành công rồi
  // đưa về đăng nhập; không refresh /api/me (sẽ 401 như thể đổi thất bại).
  const passwordAction = useAction(async (version: string | undefined = baselineVersion.current) => {
    const result = await changePassword(current, next, version)
    if (result.reauthenticate) { onClose(); toast(t.auth.passwordChanged, 'ok'); endSession(); return true }
    setCurrent(''); setNext(''); setRepeat('')
    await refresh()
    return true
  }, { error: false, invalidate: [keys.sessions(), keys.users()] })
  const conflict = passwordAction.error instanceof ConflictError && passwordAction.error !== dismissedConflict ? passwordAction.error : null

  const mismatch = repeat.length > 0 && next !== repeat
  const canChange = current.length > 0 && next.length >= 8 && next === repeat && !passwordAction.busy && connection.writable
  const submitPassword = () => { if (canChange) void passwordAction.run() }
  useUnsavedGuard(Boolean(current || next || repeat))
  useSaveShortcut(submitPassword, canChange)

  return (
    <Dialog
      title="Thiết bị & bảo mật"
      icon={<Icon name="shield-security-bulk" size={20} style={{ color: 'var(--color-accent)' }} />}
      onClose={onClose}
      width={920}
      style={{ maxHeight: 'calc(100dvh - 32px)' }}
    >
      <PanelRefreshBar resourceKeys={[keys.sessions()]} updatedAt={resource.updatedAt} />
      <StaleBanner error={resource.error || (!connection.writable ? new Error('Mất kết nối') : null)} onRetry={resource.refresh} />
      <div className="scroll" style={{ display: 'flex', flexWrap: 'wrap', gap: 24, padding: 20, overflowY: 'auto', minHeight: 0 }}>
        <section aria-labelledby="dv-s1" style={{ flex: '1 1 440px', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 10 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 8 }}>
            <h3 id="dv-s1" className="lbl" style={{ margin: 0 }}>{t.account.devices} · {sessions.length}</h3>
            <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>Mỗi phiên giữ 30 ngày kể từ lúc đăng nhập</span>
          </div>
          <div className="card" style={{ borderRadius: 14, padding: 4 }}>
            <SessionList sessions={sessions} loading={!resource.data && resource.initialLoading} error={!resource.data && resource.error instanceof Error ? resource.error.message : ''} variant="full" onRevoke={connection.writable ? revoke : undefined} />
          </div>

          <Button variant="dsoft" size="sm" disabled={!connection.writable || revokeAction.busy || sessions.every((session) => session.current)} pending={revokeAction.pending} onClick={revokeOthers}>
            <Icon name="logout" size={15} />Đăng xuất mọi thiết bị khác
          </Button>
        </section>

        <section aria-labelledby="dv-s2" style={{ flex: '1 1 300px', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 12 }}>
          <h3 id="dv-s2" className="lbl" style={{ margin: 0 }}>{t.account.changePassword}</h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label htmlFor="pw-cur" style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>Mật khẩu hiện tại</label>
            <AuthInput id="pw-cur" type="password" autoComplete="current-password" value={current} onChange={setCurrent} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label htmlFor="pw-new" style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>Mật khẩu mới</label>
            <AuthInput id="pw-new" type="password" autoComplete="new-password" placeholder={t.auth.passwordHint}
              value={next} onChange={setNext} />
            <StrengthMeter value={next} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label htmlFor="pw-re" style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>Nhập lại mật khẩu mới</label>
            <AuthInput id="pw-re" type="password" autoComplete="new-password" placeholder="Nhập lại" value={repeat}
              invalid={mismatch} onChange={setRepeat} />
            {mismatch && <span role="alert" style={{ fontSize: 12, color: 'var(--color-err)' }}>Hai lần nhập chưa khớp</span>}
          </div>
          <span style={{ display: 'flex', gap: 8, fontSize: 12, color: 'var(--color-fg-3)' }}>
            <Icon name="info-circle" size={14} style={{ marginTop: 2 }} />{t.account.passwordReauthNote}
          </span>
          {conflict && <ConflictBar pending={passwordAction.pending} onView={() => { baselineVersion.current = conflict.version; setCurrent(''); setNext(''); setRepeat(''); setDismissedConflict(conflict); void refresh() }} onOverwrite={() => { void passwordAction.run(conflict.version) }} />}
          {passwordAction.error instanceof Error && !conflict && passwordAction.error !== dismissedConflict && <span role="alert" style={{ fontSize: 12, color: 'var(--color-err)' }}>{passwordAction.error.message}</span>}
          <Button variant="primary" onClick={submitPassword} disabled={!canChange} pending={passwordAction.pending} success={passwordAction.success} tooltip="⌘S / Ctrl+S" style={{ alignSelf: 'flex-start' }}>{t.account.changePassword}</Button>
        </section>
      </div>
    </Dialog>
  )
}
