// Bốn khối thay vùng lưới của Khám phá (artboard Explore/MExplore › list):
// quét lần đầu (băng + ô xương), lọc rỗng, lỗi tải, mất kết nối. Khối trống
// dùng EmptyState `frame` cỡ lớn của kit (`.vv-empty`), nút theo ActionButton.
import * as stylex from '@stylexjs/stylex'
import { Banner, EmptyState, Icon, Skeleton } from '@noverse/ui'
import type { Rival } from '../../api/rivals'
import { t } from '../../i18n/explore'
import { ActionButton } from '../../components'
import { exploreStyles } from './explore.stylex'
import { scanProgress } from './listState'

const tt = t.screens.explore
const FADE = [1, 1, 1, 1, 0.7, 0.5, 0.35, 0.2]

/** “Trinh sát đang lấy video đầu tiên · 12 / 30” — video hiện dần bên dưới. */
export function ScanBanner({ rival }: { rival: Rival }) {
  const { done, total } = scanProgress(rival)
  return (
    <Banner tone="info" busy title={tt.listScanTitle(String(done), String(total))}>
      {tt.listScanText(String(total), rival.title)}
    </Banner>
  )
}

/** Lưới ô xương trong lúc quét: 8 ô trên máy tính, 3 ô trên điện thoại. */
export function SkeletonTiles() {
  return (
    <div aria-hidden="true" data-list-skeleton="" {...stylex.props(exploreStyles.skeletonGrid)}>
      {FADE.map((opacity, index) => (
        <div key={index} {...stylex.props(exploreStyles.skeletonTile, exploreStyles.skeletonFade(opacity))}>
          <div {...stylex.props(exploreStyles.skeletonThumb)}>
            <Skeleton width="100%" height="100%" radius={0} />
          </div>
          <Skeleton width="92%" height={12} />
          <Skeleton width="64%" height={12} />
          <span {...stylex.props(exploreStyles.skeletonMeta)}><Skeleton width="48%" height={10} /></span>
        </div>
      ))}
    </div>
  )
}

/** Khối thay cả lưới: lọc rỗng (accent) · lỗi tải/mất kết nối (danger). */
export function ListStateBlock({ tone, icon, title, text, action, actionIcon, onAction }: {
  tone: 'accent' | 'danger'
  icon: 'filter' | 'danger'
  title: string
  text: string
  action: string
  actionIcon: 'close' | 'refresh'
  onAction: () => void
}) {
  return (
    <div {...stylex.props(exploreStyles.listState)}>
      <EmptyState
        variant="frame"
        size="lg"
        tone={tone}
        icon={icon}
        title={title}
        hint={text}
        action={
          <ActionButton
            className={stylex.props(exploreStyles.touchTarget).className}
            size="sm"
            icon={<Icon name={actionIcon} size={14} />}
            onClick={onAction}
          >
            {action}
          </ActionButton>
        }
      />
    </div>
  )
}
