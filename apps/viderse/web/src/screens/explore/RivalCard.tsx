// Thẻ đối thủ ở cột phải: avatar, tên, người theo dõi, menu ⋯ (Kiểm tra video
// mới → POST scan check · Mở trên YouTube · Liên kết với kênh của tôi — khoá,
// vẫn focus được, lý do là dòng phụ ·
// Bỏ theo dõi → xác nhận cam rồi DELETE) và ba ô Mức thường / Mỗi tuần / Nổi
// trội. "Mỗi tuần" đến từ posting-slots (perWeek) nên nhận qua prop.
import { useState } from 'react'
import { ActionMenu, Dialog, Icon, confirm, toast, type ActionMenuItem } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { controlRivalScan, removeRival, type Rival } from '../../api/rivals'
import { vi as t } from '../../i18n/vi'
import { keys } from '../../data/keys'
import { ActionButton, ChannelAvatar } from '../../components'
import { fmtSubs } from './fmt'
import { linkSource } from '../../api/channels'

const num = new Intl.NumberFormat('vi-VN')
const num1 = new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 1 })
// Ô "Mỗi tuần" hẹp (cả "5,6 video" cũng xuống dòng): từ 1 video/tuần hiện số
// nguyên như artboard; dưới 1 giữ một chữ số lẻ để không thành "0 video".
const fmtPerWeek = (n: number) => (n >= 1 ? num.format(Math.round(n)) : num1.format(n))

export function RivalCard({ rival, color, rescanHours, perWeek, channels, linkedChannelName, onRemoved }: {
  rival: Rival
  color: string
  /** chu kỳ kiểm tra video mới — hiện trong hint "không đợi chu kỳ N giờ" */
  rescanHours: number
  /** video mỗi tuần trong mẫu 30 — null khi chưa đủ giờ đăng chính xác */
  perWeek: number | null | undefined
  /** kênh của tôi — có kênh thì "Liên kết với kênh…" mở hộp chọn kênh */
  channels: { id: string; name: string }[]
  /** kênh đang dùng đối thủ này (nếu có) — dòng dưới thẻ */
  linkedChannelName?: string
  onRemoved: () => void
}) {
  const tt = t.screens.explore
  const [checking, setChecking] = useState(false)
  const [linkOpen, setLinkOpen] = useState(false)

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
    channels.length > 0
      ? { id: 'link', label: tt.menuLink, description: tt.menuLinkHint, icon: <Icon name="channel" size={16} />, onSelect: () => setLinkOpen(true) }
      : { id: 'link', label: tt.menuLink, lockedReason: tt.lockedReason, icon: <Icon name="channel" size={16} /> },
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
        <span title={linkedChannelName ?? tt.lockedReason}>
          {linkedChannelName ? tt.linkedTo(linkedChannelName) : tt.notLinked}
        </span>
      </div>
      {linkOpen && (
        <LinkChannelDialog
          rival={rival}
          channels={channels}
          onClose={() => setLinkOpen(false)}
        />
      )}
    </div>
  )
}

/** "Liên kết với kênh…" — chọn một kênh của tôi rồi liên kết đối thủ vào kênh đó. */
function LinkChannelDialog({ rival, channels, onClose }: {
  rival: Rival
  channels: { id: string; name: string }[]
  onClose: () => void
}) {
  const tt = t.screens.explore
  const [picked, setPicked] = useState(channels[0]?.id ?? '')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const link = async () => {
    if (!picked || pending) return
    setPending(true)
    setError(null)
    try {
      await linkSource(picked, rival.id)
      void invalidate(keys.channels())
      toast.ok(tt.linkDone)
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : tt.linkFailed)
      setPending(false)
    }
  }

  return (
    <Dialog
      title={tt.linkDialogTitle(rival.title)}
      width={440}
      onClose={onClose}
      footer={(
        <>
          <span style={{ flex: 1 }} />
          <ActionButton size="md" onClick={onClose}>{t.common.cancel}</ActionButton>
          <ActionButton variant="primary" size="md" disabled={!picked} pending={pending} onClick={() => void link()}>
            {tt.linkPick}
          </ActionButton>
        </>
      )}
    >
      {channels.length === 0 ? <p className="vx-side-empty">{tt.linkEmpty}</p> : (
        <div role="radiogroup" aria-label={tt.linkDialogTitle(rival.title)} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-4)' }}>
          {channels.map((channel) => (
            <button
              key={channel.id}
              type="button"
              role="radio"
              aria-checked={picked === channel.id}
              className="vx-makeopt"
              onClick={() => setPicked(channel.id)}
            >
              <span className="vx-makeopt__text"><span className="vx-makeopt__name">{channel.name}</span></span>
              {picked === channel.id && <Icon name="tick" size={14} />}
            </button>
          ))}
        </div>
      )}
      {error && <p className="vx-side-empty" role="alert" style={{ marginTop: 'var(--vv-s-8)' }}>{error}</p>}
    </Dialog>
  )
}
