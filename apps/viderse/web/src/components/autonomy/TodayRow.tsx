// Dòng cuối thẻ Nhịp và ưu tiên (artboard Channel): "Hôm nay 3/5 · lần xem gần
// nhất 23:30" + nút Xem ngay. Số hôm nay đọc từ tóm tắt kênh, giờ là lần quét
// gần nhất của các đối thủ đã liên kết; Xem ngay bảo trinh sát kiểm tra lại từng
// đối thủ của kênh. Thiếu dữ liệu thì bỏ phần đó, không dựng số.
import * as stylex from '@stylexjs/stylex'
import { useState } from 'react'
import { toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../i18n/vi'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { useOnline } from '../../data/status'
import { getChannelSummaries } from '../../api/workbenchChannels'
import { controlRivalScan, type Rival } from '../../api/rivals'
import type { Channel } from '../../api/channels'
import { autonomyStyles as sx } from './autonomy.stylex'

const pad = (n: number) => String(n).padStart(2, '0')

export function TodayRow({ channel, rivals }: { channel: Channel; rivals: Rival[] }) {
  const p = t.autonomy.pacing
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

  const seeNow = async () => {
    setBusy(true)
    try {
      await Promise.all(linked.map((id) => controlRivalScan(id, 'check')))
      void invalidate(keys.rivals())
      toast.ok(p.seeNowDone)
    } catch {
      toast.err(p.seeNowError)
    } finally {
      setBusy(false)
    }
  }

  if (!today && !seen) return null
  return (
    <div {...stylex.props(sx.today)}>
      <span>
        {today && <>{'Hôm nay '}<b {...stylex.props(sx.todayNum)}>{today.done}/{today.cap}</b></>}
        {today && seen && ' · '}
        {seen && p.lastSeen(`${pad(seen.getHours())}:${pad(seen.getMinutes())}`)}
      </span>
      {linked.length > 0 && (
        <button type="button" {...stylex.props(sx.todayBtn)} disabled={busy || !online} onClick={() => void seeNow()}>{p.seeNow}</button>
      )}
    </div>
  )
}
