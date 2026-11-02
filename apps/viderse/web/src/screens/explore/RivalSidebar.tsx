import * as stylex from '@stylexjs/stylex'
import { exploreStyles } from './explore.stylex'
// Cột đối thủ bên trái Khám phá: ô thêm nhanh, nhóm "Đối thủ" với số, mục
// "Tất cả đối thủ" rồi từng kênh (avatar chữ cái — không orb, màu xoay theo
// thứ tự như artboard; sub = số video · spinner khi đang quét · huy hiệu
// "N mới" cho video nổi trội mới). Trống → câu hướng dẫn của artboard.
import { Icon, RadioCard } from '@noverse/ui'
import { ChannelAvatar } from '../../components'
import { t } from '../../i18n/explore'
import type { Rival, RivalList } from '../../api/rivals'
import { AddRivalForm } from './AddRivalForm'

const AVATAR_COLORS = ['var(--vv-rav-1)', 'var(--vv-rav-2)', 'var(--vv-rav-3)', 'var(--vv-rav-4)', 'var(--vv-rav-5)']

/** Màu avatar xoay vòng theo thứ tự trong danh sách (artboard: mỗi kênh một màu phẳng). */
export const rivalColor = (index: number): string => AVATAR_COLORS[index % AVATAR_COLORS.length]

const num = new Intl.NumberFormat('vi-VN')

export function RivalSidebar({ list, selected, onPick, onAdded, channels = [], showAdd = true }: {
  list: RivalList | undefined
  /** 'all' hoặc id kênh */
  selected: string
  onPick: (id: string) => void
  onAdded: (rival: Rival) => void
  /** kênh của tôi — dòng phụ hiện tên kênh đang liên kết đối thủ (artboard) */
  channels?: { id: string; name: string; sources: { rivalId: string }[] }[]
  /** Trạng thái trống: ô thêm nằm ở khối giữa, cột trái không vẽ lại. */
  showAdd?: boolean
}) {
  const tt = t.screens.explore
  const rivals = list?.rivals ?? []
  const fresh = list?.newOutliers ?? 0
  const totalVideos = list?.totalVideos ?? 0
  return (
    <aside className={`vx-side ${stylex.props(exploreStyles.side, !showAdd && exploreStyles.sideEmptyShell).className}`} aria-label={tt.rivalsGroup} data-empty={(list !== undefined && rivals.length === 0) || undefined}>
      {showAdd && list !== undefined && (
        <AddRivalForm compact onAdded={onAdded} onOpen={onPick} />
      )}
      <div className={`vx-grp ${stylex.props(exploreStyles.grp, list !== undefined && rivals.length === 0 && exploreStyles.grpEmpty).className}`}><span>{tt.rivalsGroup}</span><span className={`num ${stylex.props(exploreStyles.grpCount).className}`}>{num.format(rivals.length)}</span></div>
      {list !== undefined && rivals.length === 0 && <p className={`vx-side-empty ${stylex.props(exploreStyles.sideEmpty).className}`}>{tt.rivalsEmpty}</p>}
      <div role="radiogroup" aria-label={tt.rivalsGroup}>
      {rivals.length > 0 && (
        <RadioCard checked={selected === 'all'} tabIndex={selected === 'all' ? 0 : -1} onChange={() => onPick('all')} mark="none" xstyle={exploreStyles.rivalRow}>
          <span className={`vx-rival__all ${stylex.props(exploreStyles.allAvatar).className}`} aria-hidden="true">
            <Icon name="radar" size={14} />
          </span>
          <span className={`vx-rival__text ${stylex.props(exploreStyles.rivalText).className}`}>
            <span className={`vx-rival__name ${stylex.props(exploreStyles.rivalName).className}`}>{tt.allRivals}</span>
            <span className={`vx-rival__sub ${stylex.props(exploreStyles.rivalSub).className}`}><span {...stylex.props(exploreStyles.rivalSubText)}>{tt.videosMeta(num.format(totalVideos), num.format(fresh))}</span></span>
          </span>
          {fresh > 0 && <span className={`vx-new num ${stylex.props(exploreStyles.new).className}`} aria-label={tt.newBadgeAria(num.format(fresh))}>{num.format(fresh)}</span>}
        </RadioCard>
      )}
      {rivals.map((r, i) => {
        const linked = channels.find((channel) => channel.sources.some((source) => source.rivalId === r.id))
        return (
          <RadioCard key={r.id} checked={selected === r.id} tabIndex={selected === r.id ? 0 : -1} onChange={() => onPick(r.id)} mark="none" xstyle={exploreStyles.rivalRow}>
            <ChannelAvatar name={r.title} src={r.avatarUrl ?? undefined} color={{ bg: rivalColor(i) }} size={30} fontSize={10.5} />
            <span className={`vx-rival__text ${stylex.props(exploreStyles.rivalText).className}`}>
              <span className={`vx-rival__name ${stylex.props(exploreStyles.rivalName).className}`}>{r.title}</span>
              <span className={`vx-rival__sub ${stylex.props(exploreStyles.rivalSub).className}`} title={linked ? tt.linkedTo(linked.name) : (r.handle ?? undefined)}>
                {linked && <Icon name="channel" size={12} />}
                <span {...stylex.props(exploreStyles.rivalSubText)}>{linked?.name ?? r.handle ?? num.format(r.videoCount)}</span>
              </span>
            </span>
            {(r.scan.discoveryRunning || r.scan.state === 'running' || r.scan.state === 'queued') && (
              <span className={`vx-spin ${stylex.props(exploreStyles.spin).className}`} role="img" aria-label={tt.scanning} />
            )}
            {r.newOutliers > 0 && <span className={`vx-new num ${stylex.props(exploreStyles.new).className}`} aria-label={tt.newBadgeAria(num.format(r.newOutliers))}>{num.format(r.newOutliers)}</span>}
          </RadioCard>
        )
      })}
      </div>
    </aside>
  )
}
