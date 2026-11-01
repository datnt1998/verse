// 4 thẻ số liệu dưới đầu kênh (artboard Channel): Video đã làm · Đã đăng · Chờ bạn ·
// Chờ lượt. Số từ danh sách video của kênh; chờ tải hiện khung xương, lỗi hiện “—”.
import * as stylex from '@stylexjs/stylex'
import { Skeleton } from '@noverse/ui'
import type { Channel } from '../../api/channels'
import type { ChannelJobs } from '../../api/channelVideos'
import { getQueue } from '../../api/clone'
import { keys } from '../../data/keys'
import { useResource, type Resource } from '../../data/useResource'
import { t } from '../../i18n/channels'
import { channelStats } from './videosLogic'
import { videoStyles as sx } from './videos.stylex'

const labels = t.channels.videos.stats

export function VideoStats({ channel, jobs }: { channel: Channel; jobs: Resource<ChannelJobs> }) {
  const queue = useResource(keys.queue(), ({ signal }) => getQueue(signal), { enabled: (jobs.data?.queuedTotal ?? 0) > 0 })
  const data = jobs.data
  const etas = (queue.data?.items ?? []).filter((item) => item.channelId === channel.id && item.etaStart).map((item) => item.etaStart as string).sort()
  const stats = data
    ? channelStats(data.jobs, { createdAt: channel.createdAt, youtubeUrl: channel.youtubeUrl, autoPick: channel.autoPick, nextEta: etas[0] ?? null, now: Date.now() })
    : null
  const placeholder = [labels.made, labels.posted, labels.waiting, labels.queued]
  return (
    <div {...stylex.props(sx.stats)}>
      {(stats ?? placeholder.map((label) => ({ key: label, label, value: null, sub: '', tone: 'neutral' as const }))).map((stat) => (
        <div key={stat.key} {...stylex.props(sx.stat)}>
          <span {...stylex.props(sx.statLabel)}>{stat.label}</span>
          <span className={`num ${stylex.props(sx.statValue, stat.tone === 'warn' && sx.statValueWarn).className}`}>
            {stat.value !== null ? stat.value : jobs.error ? '—' : <Skeleton width={28} height={20} />}
          </span>
          {stat.sub ? <span title={stat.sub} {...stylex.props(sx.statSub)}>{stat.sub}</span> : <span {...stylex.props(sx.statSub)}>{'\u00a0'}</span>}
        </div>
      ))}
    </div>
  )
}
