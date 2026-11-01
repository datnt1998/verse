// Kết nối AI › hàng “YouTube Data API” — khoá API tuỳ chọn để Trinh sát đọc lượt
// xem sau đăng qua API thay vì trang công khai. Artboard Settings không vẽ hàng
// này nên dựng bằng đúng nhịp thẻ nhà cung cấp (conn-card, conn-head, conn-edit)
// và ghi vào hỏi-đáp #74.
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, PasswordField, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { ApiError } from '../../../api/request'
import { getDataKey, removeDataKey, saveDataKey } from '../../../api/performance'
import { keys } from '../../../data/keys'
import { useResource } from '../../../data/useResource'
import { performanceCopy } from '../../../i18n/performance'
import { t } from '../../../i18n/vi'
import { styles as settingsStyles } from '../settings.stylex'

const c = performanceCopy.dataKey

export function YoutubeDataKeyCard() {
  const state = useResource(keys.youtubeDataKey(), ({ signal }) => getDataKey(signal))
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState('')
  const [rejected, setRejected] = useState(false)
  const [pending, setPending] = useState<'save' | 'remove' | null>(null)
  const offline = state.error instanceof ApiError && state.error.status === 0
  const data = state.data
  const failed = state.error != null && !offline && data === undefined
  const configured = data?.configured === true

  const sub = offline ? { text: c.sub.offline, tone: settingsStyles.connSubErr }
    : failed ? { text: c.sub.fail, tone: settingsStyles.connSubErr }
    : data === undefined ? { text: c.sub.loading, tone: null }
    : configured ? { text: c.sub.ok(data.keyHint ?? ''), tone: settingsStyles.connSubOk }
    : { text: c.sub.empty, tone: null }

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

  const locked = offline || (data === undefined && !failed)
  return (
    <section aria-label={c.name} className={`conn-card ${stylex.props(settingsStyles.card, settingsStyles.connCard).className}`}>
      <div className={`conn-head ${stylex.props(settingsStyles.connHead).className}`}>
        <span aria-hidden="true" className={`conn-av ${stylex.props(settingsStyles.connAv).className}`}>{c.avatar}</span>
        <div className={`conn-id ${stylex.props(settingsStyles.connId).className}`}>
          <div className={`conn-title ${stylex.props(settingsStyles.connTitle).className}`}>
            <span className={`conn-name ${stylex.props(settingsStyles.connName).className}`}>{c.name}</span>
            <span className={`conn-pill ${stylex.props(settingsStyles.connPill).className}`}>{c.kind}</span>
            <span className={`conn-pill ${stylex.props(settingsStyles.connPill).className}`}>{c.pill}</span>
          </div>
          <span className={`conn-sub ${stylex.props(settingsStyles.connSub, sub.tone).className}`} role={offline || failed ? 'alert' : 'status'}>{sub.text}</span>
        </div>
        {!editing && (
          <div className={`conn-acts ${stylex.props(settingsStyles.connActs).className}`}>
            <Button size="compact" variant="ghost" className="conn-act" xstyle={[settingsStyles.connAct]} disabled={locked} onClick={open}>{configured ? c.change : c.add}</Button>
            {configured && (
              <Button size="compact" variant="quiet" className="conn-act conn-act--quiet" xstyle={[settingsStyles.connActQuiet, settingsStyles.connAct]} disabled={locked} pending={pending === 'remove'} onClick={() => void remove()}>{c.remove}</Button>
            )}
          </div>
        )}
      </div>
      {editing && (
        <>
          <div className={`conn-edit ${stylex.props(settingsStyles.connEdit).className}`}>
            <PasswordField className={`conn-edit__grow ${stylex.props(settingsStyles.connEditGrow).className}`} placeholder={c.keyLabel} value={value} invalid={rejected} onChange={(event) => { setValue(event.target.value); setRejected(false) }} aria-label={c.keyLabel} autoFocus />
            <Button size="compact" variant="primary" className="conn-act" xstyle={[settingsStyles.connAct]} pending={pending === 'save'} disabled={!value.trim()} onClick={() => void save()}>{c.save}</Button>
            <Button size="compact" variant="quiet" className="conn-act conn-act--quiet" xstyle={[settingsStyles.connActQuiet, settingsStyles.connAct]} onClick={() => setEditing(false)}>{c.cancel}</Button>
          </div>
          {rejected && <div role="alert" className={`conn-fail ${stylex.props(settingsStyles.connFail).className}`}>{c.rejected}</div>}
        </>
      )}
    </section>
  )
}
