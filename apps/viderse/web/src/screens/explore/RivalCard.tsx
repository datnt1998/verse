// Thẻ đối thủ ở cột phải: avatar, tên, người theo dõi, menu ⋯ (Kiểm tra video
// mới → POST scan check · Mở trên YouTube · Liên kết với kênh của tôi — khoá,
// vẫn focus được, lý do là dòng phụ ·
// Bỏ theo dõi → xác nhận cam rồi DELETE) và ba ô Mức thường / Mỗi tuần / Nổi
// trội. "Mỗi tuần" đến từ posting-slots (perWeek) nên nhận qua prop.
import { useState } from 'react'
import { ActionMenu, Icon, confirm, toast, type ActionMenuItem } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { controlRivalScan, removeRival, type Rival } from '../../api/rivals'
import { vi as t } from '../../i18n/vi'
import { keys } from '../../data/keys'
import { ChannelAvatar } from '../../components'
import { fmtSubs } from './fmt'

const num = new Intl.NumberFormat('vi-VN')
const num1 = new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 1 })
// Ô "Mỗi tuần" hẹp (cả "5,6 video" cũng xuống dòng): từ 1 video/tuần hiện số
// nguyên như artboard; dưới 1 giữ một chữ số lẻ để không thành "0 video".
const fmtPerWeek = (n: number) => (n >= 1 ? num.format(Math.round(n)) : num1.format(n))

export function RivalCard({ rival, color, rescanHours, perWeek, onRemoved }: {
  rival: Rival
  color: string
  /** chu kỳ kiểm tra video mới — hiện trong hint "không đợi chu kỳ N giờ" */
  rescanHours: number
  /** video mỗi tuần trong mẫu 30 — null khi chưa đủ giờ đăng chính xác */
  perWeek: number | null | undefined
  onRemoved: () => void
}) {
  const tt = t.screens.explore
  const [checking, setChecking] = useState(false)

  const unfollow = () => {
    void confirm({
      tone: 'danger',
      title: tt.unfollowTitle(rival.title),
      body: (
        <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>
          {tt.unfollowBody} {tt.unfollowItem(num.format(rival.videoCount))} {tt.unfollowQueue}
        </p>
      ),
      confirmLabel: tt.unfollowOk,
      successMessage: tt.unfollowDone,
      onConfirm: async () => {
        await removeRival(rival.id)
        void invalidate(keys.rivals())
        onRemoved()
      },
    })
  }

  const check = () => {
    if (checking) return
    setChecking(true)
    controlRivalScan(rival.id, 'check')
      .catch((err) => toast.err(err instanceof Error ? err.message : tt.addErrors.failed))
      .finally(() => setChecking(false))
  }

  const items: ActionMenuItem[] = [
    { id: 'check', label: tt.menuCheck, description: tt.menuCheckHint(rescanHours), icon: <Icon name="refresh" size={16} />, onSelect: check },
    { id: 'youtube', label: tt.openYouTube, description: rival.handle ? `youtube.com/${rival.handle}` : rival.url.replace(/^https?:\/\/(www\.)?/, ''), icon: <Icon name="link" size={16} />, onSelect: () => window.open(rival.url, '_blank', 'noreferrer') },
    { id: 'link', label: tt.menuLink, lockedReason: tt.lockedReason, icon: <Icon name="channel" size={16} /> },
    { id: 'unfollow', label: tt.menuUnfollow, description: tt.menuUnfollowHint, icon: <Icon name="trash" size={16} />, danger: true, onSelect: unfollow },
  ]

  const subs = rival.subscriberCount != null
    ? tt.subscribers(fmtSubs(rival.subscriberCount))
    : (rival.handle ?? '')

  return (
    <div className="vx-cardbox">
      <div className="vx-rivalhead">
        <ChannelAvatar name={rival.title} src={rival.avatarUrl ?? undefined} color={{ bg: color }} size={40} style={{ fontSize: 13, fontWeight: 700 }} />
        <div className="vx-rivalhead__text">
          <span className="vx-rivalhead__name">{rival.title}</span>
          <span className="vx-rivalhead__sub" title={[rival.handle, subs].filter(Boolean).join(' · ')}>{subs}</span>
        </div>
        <div style={{ position: 'relative' }}>
          <ActionMenu
            label={tt.rivalActions(rival.title)}
            items={items}
            width={272}
            trigger={<button type="button" className="vx-iconbtn" aria-label={tt.rivalActions(rival.title)} aria-haspopup="menu"><Icon name="more" size={16} /></button>}
          />
        </div>
      </div>
      <div className="vx-stats">
        <div className="vx-stat"><span>{tt.statBaseline}</span><b className="num">{rival.medianViews != null ? fmtSubs(rival.medianViews) : tt.statPending}</b></div>
        <div className="vx-stat"><span>{tt.statPerWeek}</span><b className="num">{perWeek != null ? tt.perWeekValue(fmtPerWeek(perWeek)) : tt.statPending}</b></div>
        <div className="vx-stat vx-stat--hot"><span>{tt.statOutliers}</span><b className="num">{num.format(rival.outliers)}</b></div>
      </div>
      <div className="vx-linkrow">
        <Icon name="channel" size={15} />
        <span title={tt.lockedReason}>{tt.notLinked}</span>
      </div>
    </div>
  )
}
