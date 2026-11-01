// Cài đặt › Kết nối AI › nhóm “Số liệu YouTube” (artboard Settings/MSettings ›
// section=connect · ytkey=empty|ok|edit|rejected|failed|offline). Nhóm đứng
// DƯỚI nút Thêm nhà cung cấp, ngoài danh sách nhà cung cấp AI: khoá này không
// phải AI, chỉ để Trinh sát đọc lượt xem/thích/bình luận sau đăng qua API.
//   • empty: *Thêm khoá*; ok: *Đổi khoá* + *Gỡ khoá* (dòng phụ xanh).
//   • edit: ô dán khoá + dòng gợi ý Google Cloud Console dưới ô.
//   • rejected: viền cam ở thẻ và ô, câu lỗi ngay trên dòng gợi ý.
//   • failed: chưa đọc được trạng thái → *Thử lại* (đọc lại), KHÔNG phải *Thêm khoá*.
//   • offline: mọi nút của hàng tắt tới khi kết nối lại.
import { useId, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Icon, PasswordField, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { ApiError } from '../../../api/request'
import { getDataKey, removeDataKey, saveDataKey } from '../../../api/performance'
import { keys } from '../../../data/keys'
import { useResource } from '../../../data/useResource'
import { performanceCopy } from '../../../i18n/performance'
import { t } from '../../../i18n/settings'
import { styles as settingsStyles } from '../settings.stylex'

const c = performanceCopy.dataKey

export function YoutubeDataKeyCard({ mobile = false }: {
  /** Bản điện thoại (MSettings screen=connect): nút 44px, cụm nút thành lưới,
   *  ô nhập có nhãn riêng — artboard vẽ khác bản máy tính. */
  mobile?: boolean
}) {
  const state = useResource(keys.youtubeDataKey(), ({ signal }) => getDataKey(signal))
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState('')
  const [rejected, setRejected] = useState(false)
  const [pending, setPending] = useState<'save' | 'remove' | null>(null)
  const [retrying, setRetrying] = useState(false)
  const headingId = useId()
  const hintId = useId()
  const fieldId = useId()
  const offline = state.error instanceof ApiError && state.error.status === 0
  const data = state.data
  const failed = state.error != null && !offline && data === undefined
  const configured = data?.configured === true

  const sub = offline ? { text: c.sub.offline, tone: settingsStyles.connSubErr }
    : failed ? { text: c.sub.fail, tone: settingsStyles.connSubErr }
    : data === undefined ? { text: c.sub.loading, tone: null }
    : configured ? { text: c.sub.ok(data.keyHint ?? ''), tone: settingsStyles.connSubOk }
    : { text: c.sub.empty, tone: settingsStyles.connYtSubIdle }

  function open() { setEditing(true); setValue(''); setRejected(false) }

  async function save() {
    setPending('save')
    setRejected(false)
    try {
      await saveDataKey(value)
      await invalidate(keys.youtubeDataKey())
      setEditing(false)
      setValue('')
      toast.ok(c.saved)
    } catch (error) {
      if (error instanceof ApiError && error.status === 400) setRejected(true)
      else toast.err(error instanceof Error ? error.message : t.errors.failed)
    } finally {
      setPending(null)
    }
  }

  async function remove() {
    setPending('remove')
    try {
      await removeDataKey()
      await invalidate(keys.youtubeDataKey())
      setEditing(false)
      toast.ok(c.removed)
    } catch (error) {
      toast.err(error instanceof Error ? error.message : t.errors.failed)
    } finally {
      setPending(null)
    }
  }

  /** failed: đọc lại trạng thái khoá (chưa biết đã có khoá hay chưa nên không
   *  mở ô dán — artboard: nút là *Thử lại*, không phải *Thêm khoá*). */
  async function retry() {
    setRetrying(true)
    try {
      await state.refresh()
    } finally {
      setRetrying(false)
    }
  }

  const locked = offline || (data === undefined && !failed)
  const size = mobile ? 'lg' : 'compact'
  const field = (
    <PasswordField
      id={fieldId}
      className={`conn-edit__grow ${stylex.props(settingsStyles.connEditGrow, mobile && settingsStyles.connYtMInput).className}`}
      placeholder={c.keyLabel}
      value={value}
      invalid={rejected}
      onChange={(event) => { setValue(event.target.value); setRejected(false) }}
      aria-label={c.keyLabel}
      aria-describedby={hintId}
      autoFocus={!mobile}
    />
  )
  const saveBtn = (
    <Button size={size} variant="primary" className="conn-act" xstyle={[settingsStyles.connAct]} pending={pending === 'save'} disabled={!value.trim()} onClick={() => void save()}>{c.save}</Button>
  )
  const cancelBtn = (
    <Button size={size} variant="quiet" className="conn-act conn-act--quiet" xstyle={[settingsStyles.connActQuiet, settingsStyles.connAct]} onClick={() => { setEditing(false); setRejected(false) }}>{c.cancel}</Button>
  )
  const hint = (
    <span id={hintId} className={`conn-yt-hint ${stylex.props(settingsStyles.connYtHint).className}`}>
      <Icon name="key" size={14} xstyle={settingsStyles.connYtHintIcon} />
      {c.hint}
    </span>
  )
  const rejection = rejected && (
    <div role="alert" className={`conn-err ${stylex.props(settingsStyles.connYtReject).className}`}>
      <Icon name="danger" size={15} xstyle={settingsStyles.connYtRejectIcon} />
      <span className={`conn-err__text ${stylex.props(settingsStyles.connYtRejectText).className}`}>{c.rejected}</span>
    </div>
  )

  return (
    <div role="group" aria-labelledby={headingId} className={`conn-yt ${stylex.props(settingsStyles.connYtGroup).className}`}>
      <div className={`conn-yt__head ${stylex.props(settingsStyles.connYtHead, mobile && settingsStyles.connYtHeadMobile).className}`}>
        <h2 id={headingId} className={`conn-yt__title ${stylex.props(settingsStyles.connYtTitle).className}`}>{c.groupTitle}</h2>
        <span className={`conn-yt__sub ${stylex.props(settingsStyles.connYtGroupSub).className}`}>{c.groupSub}</span>
      </div>
      <section
        aria-label={c.name}
        className={`conn-card ${stylex.props(settingsStyles.card, settingsStyles.connCard, rejected && settingsStyles.connYtCardRejected).className}`}
      >
        <div className={`conn-head ${stylex.props(settingsStyles.connHead).className}`}>
          <span aria-hidden="true" className={`conn-av ${stylex.props(settingsStyles.connAv, settingsStyles.connAvYt).className}`}>{c.avatar}</span>
          <div className={`conn-id ${stylex.props(settingsStyles.connId).className}`}>
            <div className={`conn-title ${stylex.props(settingsStyles.connTitle).className}`}>
              <span className={`conn-name ${stylex.props(settingsStyles.connName).className}`}>{c.name}</span>
              {!mobile && <span className={`conn-pill ${stylex.props(settingsStyles.connPill).className}`}>{c.kind}</span>}
              <span className={`conn-pill ${stylex.props(settingsStyles.connPill).className}`}>{c.pill}</span>
            </div>
            <span className={`conn-sub ${stylex.props(settingsStyles.connSub, sub.tone).className}`} role={offline || failed ? 'alert' : 'status'}>{sub.text}</span>
          </div>
          {!editing && !mobile && (
            <div className={`conn-acts ${stylex.props(settingsStyles.connActs).className}`}>
              {failed ? (
                <Button size={size} variant="ghost" className="conn-act" xstyle={[settingsStyles.connAct]} pending={retrying} icon={<Icon name="refresh" size={14} />} onClick={() => void retry()}>{c.retry}</Button>
              ) : (
                <>
                  <Button size={size} variant="ghost" className="conn-act" xstyle={[settingsStyles.connAct]} disabled={locked} onClick={open}>{configured ? c.change : c.add}</Button>
                  {configured && (
                    <Button size={size} variant="quiet" className="conn-act conn-act--quiet" xstyle={[settingsStyles.connActQuiet, settingsStyles.connAct]} disabled={locked} pending={pending === 'remove'} onClick={() => void remove()}>{c.remove}</Button>
                  )}
                </>
              )}
            </div>
          )}
        </div>
        {!editing && mobile && (
          <div className={`conn-yt__acts ${stylex.props(settingsStyles.connYtMActs).className}`}>
            {failed ? (
              <Button size={size} variant="ghost" block pending={retrying} icon={<Icon name="refresh" size={15} />} onClick={() => void retry()}>{c.retry}</Button>
            ) : configured ? (
              <>
                <Button size={size} variant="ghost" block disabled={locked} onClick={open}>{c.change}</Button>
                <Button size={size} variant="ghost" block disabled={locked} pending={pending === 'remove'} onClick={() => void remove()} xstyle={[settingsStyles.connYtMRemove]}>{c.remove}</Button>
              </>
            ) : (
              <Button size={size} variant="ghost" block disabled={locked} onClick={open}>{c.add}</Button>
            )}
          </div>
        )}
        {editing && (
          mobile ? (
            <div className={`conn-yt__edit ${stylex.props(settingsStyles.connYtMEdit).className}`}>
              <label className={`conn-yt__label ${stylex.props(settingsStyles.connYtMLabel).className}`} htmlFor={fieldId}>{c.keyLabel}</label>
              {field}
              {rejection}
              {hint}
              <div className={`conn-yt__acts ${stylex.props(settingsStyles.connYtMActs).className}`}>{cancelBtn}{saveBtn}</div>
            </div>
          ) : (
            <>
              <div className={`conn-edit ${stylex.props(settingsStyles.connEdit).className}`}>
                {field}{saveBtn}{cancelBtn}
              </div>
              {rejection}
              {hint}
            </>
          )
        )}
      </section>
    </div>
  )
}
