// Thẻ “Tự động” của tab Nguồn & tự động (artboard Channel › sources ·
// MChannel): ba hàng Tự lấy video mới · Video mỗi ngày (stepper 1–20 + dòng
// hôm nay/lần xem gần nhất + Xem đối thủ ngay) · Tự duyệt ảnh, rồi ghi chú
// rào chắn. Mỗi đổi gửi PATCH đúng một trường rồi áp kênh trả về; lỗi hiện
// dưới thẻ. Không còn Mức tự chủ/Ưu tiên/luân phiên (r44 · V38).
import * as stylex from '@stylexjs/stylex'
import { useState } from 'react'
import { Button, Switch, toast } from '@noverse/ui'
import { invalidate, setData } from '@noverse/ui/data'
import { t } from '../../i18n/channels'
import { keys } from '../../data/keys'
import { patchChannel, type Channel } from '../../api/channels'
import { controlRivalScan, type Rival } from '../../api/rivals'
import { useResource } from '../../data/useResource'
import { useOnline } from '../../data/status'
import { getChannelSummaries } from '../../api/workbenchChannels'
import { Orb } from '../../components/Orb'
import { PaceStepper } from './PaceStepper'
import { channelStyles } from './channel.stylex'
import { mch } from './mobile/mchannel.stylex'

const pad = (n: number) => String(n).padStart(2, '0')

export function AutomationCard({ channel, rivals, mobile = false }: {
  channel: Channel
  rivals: Rival[]
  mobile?: boolean
}) {
  const tt = t.channels.automation
  const [error, setError] = useState<string | null>(null)
  const [busyKey, setBusyKey] = useState<string | null>(null)

  const save = async (key: string, patch: Parameters<typeof patchChannel>[1], okMessage?: string) => {
    if (busyKey) return
    setBusyKey(key)
    setError(null)
    try {
      const next = await patchChannel(channel.id, patch)
      setData(keys.channel(channel.id), next)
      void invalidate(keys.channels())
      if (okMessage) toast.ok(okMessage)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.channels.sources.saveError)
    } finally {
      setBusyKey(null)
    }
  }

  const noRivals = channel.sources.length === 0
  const fetchSub = noRivals
    ? tt.fetchNoRivals
    : channel.autoFetch
      ? tt.fetchOn(channel.sources.length)
      : tt.fetchOff
  const toggleFetch = (checked: boolean) =>
    void save('fetch', { autoFetch: checked }, checked ? tt.toastFetchOn(channel.sources.length) : tt.toastFetchOff)
  const toggleApprove = (checked: boolean) =>
    void save('approve', { autoApprove: checked }, checked ? tt.toastApproveOn : tt.toastApproveOff)
  const saveDay = (perDay: number) => void save('perDay', { perDay })

  if (mobile) {
    return (
      <section {...stylex.props(mch.card16)} aria-label={tt.title}>
        <div {...stylex.props(mch.headRow)}>
          <span {...stylex.props(mch.orbs)} aria-hidden="true"><Orb of="scout" size={22} /></span>
          <div {...stylex.props(mch.headText)}>
            <h2 {...stylex.props(mch.cardTitle)}>{tt.title}</h2>
            <span {...stylex.props(mch.cardSub)}>{tt.subM}</span>
          </div>
        </div>
        <div {...stylex.props(mch.autoRow)}>
          <div {...stylex.props(mch.autoText)}>
            <span {...stylex.props(mch.autoLabel)}>{tt.fetchLabel}</span>
            <span {...stylex.props(mch.autoSub, noRivals && mch.footNoteBlocked)}>{fetchSub}</span>
          </div>
          <Switch checked={channel.autoFetch} disabled={busyKey !== null}
            aria-label={tt.fetchLabel} onChange={toggleFetch} />
        </div>
        <div {...stylex.props(mch.autoRow)}>
          <div {...stylex.props(mch.autoText)}>
            <span {...stylex.props(mch.autoLabel)}>{tt.perDayLabel}</span>
            <TodayLine channel={channel} rivals={rivals} mobile />
          </div>
          <PaceStepper value={channel.perDay} onSave={saveDay} label={tt.perDayLabel} disabled={busyKey !== null} />
        </div>
        <div {...stylex.props(mch.autoRow)}>
          <div {...stylex.props(mch.autoText)}>
            <span {...stylex.props(mch.autoLabel)}>{tt.approveLabel}</span>
            <span {...stylex.props(mch.autoSub)}>{channel.autoApprove ? tt.approveOn : tt.approveOff}</span>
          </div>
          <Switch checked={channel.autoApprove} disabled={busyKey !== null}
            aria-label={tt.approveLabel} onChange={toggleApprove} />
        </div>
        <span {...stylex.props(mch.hint)}>{tt.guardM}</span>
        {error && <p role="alert" {...stylex.props(mch.error)}>{error}</p>}
      </section>
    )
  }

  return (
    <section {...stylex.props(channelStyles.card, channelStyles.autoCard)} aria-label={tt.title}>
      <div {...stylex.props(channelStyles.autoHead)}>
        <span {...stylex.props(channelStyles.autoOrb)} aria-hidden="true"><Orb of="scout" size={22} /></span>
        <div {...stylex.props(channelStyles.autoHeadText)}>
          <h2 {...stylex.props(channelStyles.cardHeading)}>{tt.title}</h2>
          <span {...stylex.props(channelStyles.autoSub)}>{tt.sub}</span>
        </div>
      </div>
      <div {...stylex.props(channelStyles.autoRow)}>
        <div {...stylex.props(channelStyles.autoText)}>
          <span {...stylex.props(channelStyles.autoLabel)}>{tt.fetchLabel}</span>
          <span {...stylex.props(channelStyles.autoSub, noRivals && channelStyles.autoWarn)}>{fetchSub}</span>
        </div>
        <Switch checked={channel.autoFetch} disabled={busyKey !== null}
          aria-label={tt.fetchLabel} onChange={toggleFetch} />
      </div>
      <div {...stylex.props(channelStyles.autoRow)}>
        <div {...stylex.props(channelStyles.autoText)}>
          <span {...stylex.props(channelStyles.autoLabel)}>{tt.perDayLabel}</span>
          <TodayLine channel={channel} rivals={rivals} />
        </div>
        <PaceStepper value={channel.perDay} onSave={saveDay} label={tt.perDayLabel} disabled={busyKey !== null} />
      </div>
      <div {...stylex.props(channelStyles.autoRow)}>
        <div {...stylex.props(channelStyles.autoText)}>
          <span {...stylex.props(channelStyles.autoLabel)}>{tt.approveLabel}</span>
          <span {...stylex.props(channelStyles.autoSub)}>{channel.autoApprove ? tt.approveOn : tt.approveOff}</span>
        </div>
        <Switch checked={channel.autoApprove} disabled={busyKey !== null}
          aria-label={tt.approveLabel} onChange={toggleApprove} />
      </div>
      <p {...stylex.props(channelStyles.autoGuard)}>{tt.guard}</p>
      {error && <p className={`vch-field__error ${stylex.props(channelStyles.fieldError).className}`} role="alert">{error}</p>}
    </section>
  )
}

/** Dòng “Hôm nay x/y · lần xem gần nhất HH:MM” + Xem đối thủ ngay (artboard
 *  Channel/MChannel): số hôm nay từ tóm tắt kênh, giờ là lần quét gần nhất của
 *  các đối thủ đã liên kết; Xem ngay bảo trinh sát kiểm tra lại từng đối thủ
 *  của kênh. Thiếu dữ liệu thì bỏ phần đó, không dựng số. */
export function TodayLine({ channel, rivals, mobile = false }: {
  channel: Channel
  rivals: Rival[]
  mobile?: boolean
}) {
  const tt = t.channels.automation
  const online = useOnline()
  const [busy, setBusy] = useState(false)
  const summaries = useResource(['channels', 'summary'], ({ signal }) => getChannelSummaries(signal))
  const today = summaries.data?.find((item) => item.channelId === channel.id)?.today
  const linked = channel.sources.map((source) => source.rivalId)
  const seen = rivals
    .filter((rival) => linked.includes(rival.id) && rival.scannedAt)
    .map((rival) => new Date(rival.scannedAt as string))
    .filter((date) => !Number.isNaN(date.getTime()))
    .sort((a, b) => b.getTime() - a.getTime())[0]

  const lookNow = async () => {
    setBusy(true)
    try {
      await Promise.all(linked.map((id) => controlRivalScan(id, 'check')))
      void invalidate(keys.rivals())
      toast.ok(tt.lookDone)
    } catch {
      toast.err(tt.lookError)
    } finally {
      setBusy(false)
    }
  }

  const time = seen ? `${pad(seen.getHours())}:${pad(seen.getMinutes())}` : ''
  const text = today && time ? tt.todayScan(today.done, today.cap, time)
    : today ? tt.today(today.done, today.cap)
      : time ? tt.lastSeen(time) : ''
  const btn = mobile ? mch.lookBtn : channelStyles.lookBtn
  return (
    <span {...stylex.props(mobile ? mch.todayLine : channelStyles.todayLine)}>
      {text && <span {...stylex.props(mobile ? mch.autoSub : channelStyles.autoSub)}>{text}</span>}
      {linked.length > 0 && (
        <Button variant="quiet" size="sm" xstyle={btn}
          pending={busy} disabled={!online}
          onClick={() => void lookNow()}
        >
          {tt.lookNow}
        </Button>
      )}
      {!text && linked.length === 0 && <span {...stylex.props(mobile ? mch.autoSub : channelStyles.autoSub)}>{'\u00a0'}</span>}
    </span>
  )
}
