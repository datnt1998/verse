import * as stylex from '@stylexjs/stylex'
import { exploreStyles } from './explore.stylex'
// Cột đối thủ bên trái Khám phá: ô thêm nhanh, nhóm "Đối thủ" với số, mục
// "Tất cả đối thủ" rồi từng kênh (avatar chữ cái — không orb, màu xoay theo
// thứ tự như artboard; sub = số video · spinner khi đang quét · huy hiệu
// "N mới" cho video nổi trội mới). Trống → câu hướng dẫn của artboard.
import { Icon } from '@noverse/ui'
import { ChannelAvatar } from '../../components'
import { t } from '../../i18n/explore'
import type { Rival, RivalList } from '../../api/rivals'
import { AddRivalForm } from './AddRivalForm'

const AVATAR_COLORS = ['var(--vv-rav-1)', 'var(--vv-rav-2)', 'var(--vv-rav-3)', 'var(--vv-rav-4)', 'var(--vv-rav-5)']

/** Màu avatar xoay vòng theo thứ tự trong danh sách (artboard: mỗi kênh một màu phẳng). */
export const rivalColor = (index: number): string => AVATAR_COLORS[index % AVATAR_COLORS.length]

const num = new Intl.NumberFormat('vi-VN')

export function RivalSidebar({ list, selected, onPick, onAdded, showAdd = true }: {
  list: RivalList | undefined
  /** 'all' hoặc id kênh */
  selected: string
  onPick: (id: string) => void
  onAdded: (rival: Rival) => void
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
      {rivals.length > 0 && (
        <button type="button" className={`vx-rival ${stylex.props(exploreStyles.rival, exploreStyles.rivalMobile).className}`} aria-pressed={selected === 'all'} onClick={() => onPick('all')}>
          <span role="img" aria-label={tt.allRivals}
            style={{ width: 30, height: 30, borderRadius: '50%', flex: '0 0 auto', background: 'var(--color-surface-4)', border: '1px solid var(--vv-line-14)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-fg-2)' }}>
            <Icon name="radar" size={14} />
          </span>
          <span className={`vx-rival__text ${stylex.props(exploreStyles.rivalText).className}`}>
            <span className={`vx-rival__name ${stylex.props(exploreStyles.rivalName).className}`}>{tt.allRivals}</span>
            <span className={`vx-rival__sub ${stylex.props(exploreStyles.rivalSub).className}`}><span {...stylex.props(exploreStyles.rivalSubText)}>{tt.videosMeta(num.format(totalVideos), num.format(fresh))}</span></span>
          </span>
          {fresh > 0 && <span className={`vx-new num ${stylex.props(exploreStyles.new).className}`} aria-label={tt.newBadgeAria(num.format(fresh))}>{num.format(fresh)}</span>}
        </button>
      )}
      {rivals.map((r, i) => (
        <button key={r.id} type="button" className={`vx-rival ${stylex.props(exploreStyles.rival, exploreStyles.rivalMobile).className}`} aria-pressed={selected === r.id} onClick={() => onPick(r.id)}>
          <ChannelAvatar name={r.title} src={r.avatarUrl ?? undefined} color={{ bg: rivalColor(i) }} size={30} style={{ fontSize: 10.5, fontWeight: 700, letterSpacing: '.02em' }} />
          <span className={`vx-rival__text ${stylex.props(exploreStyles.rivalText).className}`}>
            <span className={`vx-rival__name ${stylex.props(exploreStyles.rivalName).className}`}>{r.title}</span>
            <span className={`vx-rival__sub ${stylex.props(exploreStyles.rivalSub).className}`}>
              <span {...stylex.props(exploreStyles.rivalSubText)} title={r.handle ?? undefined}>{r.handle ?? num.format(r.videoCount)}</span>
            </span>
          </span>
          {(r.scan.discoveryRunning || r.scan.state === 'running' || r.scan.state === 'queued') && (
            <span className={`vx-spin ${stylex.props(exploreStyles.spin).className}`} role="img" aria-label={tt.scanning} />
          )}
          {r.newOutliers > 0 && <span className={`vx-new num ${stylex.props(exploreStyles.new).className}`} aria-label={tt.newBadgeAria(num.format(r.newOutliers))}>{num.format(r.newOutliers)}</span>}
        </button>
      ))}
    </aside>
  )
}
