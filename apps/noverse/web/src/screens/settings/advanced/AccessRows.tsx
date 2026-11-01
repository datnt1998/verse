// Nhóm “Khôi phục quản trị” — một hàng: Mã khôi phục quản trị.
// Lệch artboard Settings.dc.html advGroups[3]: hai hàng truy cập mạng đã bỏ (docs/design/v1/19-gap-closure.md, 01/11/2026).
import { useState } from 'react'
import { ApiError, getRecovery, issueRecovery, rotateRecovery } from '../../../api/client'
import { keys, setData, useAction, useResource } from '../../../data'
import { t } from '../../../i18n/vi'
import { RecoveryConflictDialog } from '../../dialogs/RecoveryConflictDialog'
import { Skeleton } from '@noverse/ui'
import { AdvButton, AdvRow } from './AdvGroup'
import { recoveryValue, type RecoveryStatus } from './logic'
import { RecoveryCodeCard } from './RecoveryCodeCard'

const recoveryKey = [...keys.me(), 'recovery'] as const

export function AccessRows() {
  const recovery = useResource<RecoveryStatus>(recoveryKey, () => getRecovery())
  // Mã vừa tạo chỉ nằm trong state của thành phần này: rời Nâng cao hay tải lại là mất.
  const [revealed, setRevealed] = useState('')
  // 409 khi vẫn còn mã chưa dùng → hộp “Mã khôi phục cũ chưa dùng” (Dialogs.dc.html › state=recovery).
  const [conflict, setConflict] = useState(false)

  const recoveryActionRun = useAction(async (rotate = false) => {
    try {
      const created = rotate ? await rotateRecovery() : await issueRecovery()
      setRevealed(created.code)
      setData<RecoveryStatus>(recoveryKey, { createdAt: created.createdAt, used: false, mask: undefined })
      return created
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) { setConflict(true); return undefined }
      throw error
    }
  }, { success: t.adv.access.recoveryIssued, invalidate: [recoveryKey] })

  return (
    <>
      <AdvRow label={t.adv.access.recoveryLabel}
        value={recovery.data ? recoveryValue(recovery.data) : recovery.error ? t.adv.access.loadFailed : <Skeleton width={160} height={14} />}
        below={revealed ? <div style={{ margin: '4px 0 6px' }}><RecoveryCodeCard code={revealed} /></div> : undefined}>
        {recovery.data
          ? <AdvButton pending={recoveryActionRun.pending} onClick={() => void recoveryActionRun.run()}>{t.adv.access.recoveryNew}</AdvButton>
          : recovery.error ? <AdvButton onClick={() => void recovery.refresh()}>{t.adv.access.retry}</AdvButton> : null}
      </AdvRow>
      {conflict && recovery.data?.createdAt && (
        <RecoveryConflictDialog createdAt={recovery.data.createdAt} mask={recovery.data.mask} pending={recoveryActionRun.pending}
          onKeep={() => setConflict(false)}
          onRotate={() => { setConflict(false); void recoveryActionRun.run(true) }} />
      )}
    </>
  )
}
