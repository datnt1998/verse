// Tab Video của trang Kênh. Chưa có clone job/hàng chờ nên luôn là
// trạng thái "kênh mới" của artboard (`fresh`): lời dẫn + hai thẻ dẫn — video
// mới nhất của đối thủ tham chiếu (số liệu thật từ API video) và "Tự lấy video"
// (trạng thái + nhịp thật của kênh). Không vẽ bảng job, số liệu đầu kênh hay
// "Hôm nay x/y" vì chưa có dữ liệu.
import { Icon } from '@noverse/ui'
import { ActionButton } from '../../components'
import { t } from '../../i18n/vi'
import { paths } from '../../router'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { listRivalVideos, type Rival } from '../../api/rivals'
import type { Channel } from '../../api/channels'
import { fmtAge, fmtDuration, fmtScore } from '../explore/fmt'

export function VideosTab({ channel, rival }: {
  channel: Channel
  /** đối thủ tham chiếu (link sớm nhất) — nguồn của thẻ "Video mới nhất" */
  rival: Rival | undefined
}) {
  const tt = t.channels
  const latest = useResource(
    keys.rivalVideos(rival?.id ?? '_none', 'newest', 0, 0, ''),
    ({ signal }) => listRivalVideos(rival!.id, { sort: 'newest', limit: 1 }, signal),
    { enabled: !!rival },
  )
  const video = latest.data?.videos[0]
  const reference = channel.sources.find((source) => source.reference) ?? channel.sources[0]
  const rivalName = rival?.title ?? reference?.title ?? ''

  const meta = video
    ? [rivalName, video.publishedAt ? `${fmtAge(video.publishedAt)} trước` : '', fmtDuration(video.durationSec)].filter(Boolean).join(' · ')
    : ''

  return (
    <section className="vch-card vch-fresh" aria-label={tt.page.tabs.videos}>
      <h2>{tt.videos.freshTitle}</h2>
      <p>{tt.videos.freshBody}</p>
      <div className="vch-leads">
        <div className="vch-lead">
          <div className="vch-lead__head">
            <span>{tt.videos.latestTitle}</span>
            {video?.score != null && <span className="num">{fmtScore(video.score)}</span>}
          </div>
          {video ? (
            <>
              <span className="vch-lead__title" title={video.title}>{video.title}</span>
              <span className="vch-lead__meta">{meta}</span>
            </>
          ) : (
            <span className="vch-lead__meta">{reference ? tt.videos.latestEmpty : tt.videos.noRivals}</span>
          )}
          <div className="vch-lead__foot">
            <ActionButton
              size="sm"
              lockedReason={tt.queueLock}
              icon={<Icon name="video-add" size={14} />}
            >
              {tt.videos.make}
            </ActionButton>
            <a className="vch-chip" href={`#${paths.explore({ rival: rival?.handle ?? rival?.id })}`}>
              <Icon name="radar" size={14} />
              {tt.videos.explore}
            </a>
          </div>
        </div>
        <div className="vch-lead">
          <div className="vch-lead__head">
            <span>{tt.videos.autoTitle}</span>
            <span className={channel.autoPick ? 'vch-chip vch-chip--auto' : 'vch-chip'}>
              {channel.autoPick ? tt.videos.autoOn : tt.videos.autoOff}
            </span>
          </div>
          <span className="vch-lead__meta">
            {channel.sources.length > 0 ? tt.videos.pace(channel.pacePerDay) : tt.videos.autoPaused}
          </span>
          <span className="vch-lead__meta">
            {channel.sources.length > 0 ? null : tt.videos.noRivals}
          </span>
          <div className="vch-lead__foot">
            <a className="vch-chip" href={`#${paths.channel(channel.id, 'sources')}`}>
              <Icon name="calendar" size={14} />
              {tt.videos.paceLink}
            </a>
          </div>
        </div>
      </div>
    </section>
  )
}
