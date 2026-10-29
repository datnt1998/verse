// Nhóm “Truy cập & khôi phục” — Settings.dc.html advGroups[3]: ba hàng
// Cho phép thiết bị cùng Wi-Fi · Truy cập khi ra ngoài · Mã khôi phục quản trị.
import { useState } from 'react'
import { ApiError, getRecovery, getSystem, issueRecovery, rotateRecovery, setLanAccess } from '../../../api/client'
import type { SystemInfo } from '../../../api/types'
import { keys, setData, useAction, useResource } from '../../../data'
import { t } from '../../../i18n/vi'
import { RecoveryConflictDialog } from '../../dialogs/RecoveryConflictDialog'
import { Skeleton } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { AdvButton, AdvRow } from './AdvGroup'
import { lanView, recoveryValue, type RecoveryStatus } from './logic'
import { RecoveryCodeCard } from './RecoveryCodeCard'

const recoveryKey = [...keys.me(), 'recovery'] as const

export function AccessRows() {
  const system = useResource(keys.system(), ({ signal }) => getSystem(signal), { staleTime: 60000 })
  const recovery = useResource<RecoveryStatus>(recoveryKey, () => getRecovery())
  const [guide, setGuide] = useState(false)
  // Mã vừa tạo chỉ nằm trong state của thành phần này: rời Nâng cao hay tải lại là mất.
  const [revealed, setRevealed] = useState('')
  // 409 khi vẫn còn mã chưa dùng → hộp “Mã khôi phục cũ chưa dùng” (Dialogs.dc.html › state=recovery).
  const [conflict, setConflict] = useState(false)

  const lanAction = useAction(async (next: boolean) => {
    const result = await setLanAccess(next)
    if (system.data) setData<SystemInfo>(keys.system(), { ...system.data, lan: result.lan })
    toast.ok(result.lan ? t.adv.access.lanOpened : t.adv.access.lanClosed)
    return result
  }, { invalidate: [keys.system()] })

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

  const info = system.data
  const lan = info ? lanView(!!info.lan, info.canReveal) : null
  const lanValue = lanAction.pending && lan && !info?.lan ? t.adv.access.lanOpening : lan?.value

  return (
    <>
      <AdvRow label={t.adv.access.lanLabel}
        value={!lan ? (system.error ? t.adv.access.loadFailed : <Skeleton width={140} height={14} />) : lanValue}>
        {lan
          ? <AdvButton pending={lanAction.pending} disabled={!!lan.disabledReason} title={lan.disabledReason || undefined}
            onClick={() => void lanAction.run(!info?.lan)}>{lan.action}</AdvButton>
          : system.error ? <AdvButton onClick={() => void system.refresh()}>{t.adv.access.retry}</AdvButton> : null}
      </AdvRow>

      <AdvRow label={t.adv.access.outLabel} value={t.adv.access.outValue}
        below={guide ? (
          <span role="note" style={{ fontSize: 12, color: '#8E8E99', lineHeight: 1.5, padding: '0 0 10px' }}>{t.adv.access.outHint}</span>
        ) : undefined}>
        <AdvButton expanded={guide} onClick={() => setGuide((value) => !value)}>{t.adv.access.outGuide}</AdvButton>
      </AdvRow>

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
