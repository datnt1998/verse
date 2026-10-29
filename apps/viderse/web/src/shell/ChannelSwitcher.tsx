// ChannelSwitcher — bộ đổi kênh ở đầu trang Bàn dựng (04-components).
// Chưa có dữ liệu kênh thì chỉ có “Mọi kênh” + Tạo kênh; menu của kit
// (ActionMenu) chỉ nhận item phẳng nên popover này dựng trực tiếp với
// usePopup + menuitemradio theo đúng vai trò.
// Kiểu: styles/components.css .vv-switcher*/.vv-stack.
import type { ReactNode } from 'react'
import { usePopup } from '@noverse/ui'
import { t } from '../i18n/vi'
import { paths } from '../router'
import { ChannelAvatar } from '../components/ChannelAvatar'
import { Icon } from '@noverse/ui'

export type SwitcherChannel = {
  id: string
  name: string
  initials?: string
  color?: { bg: string; ring?: string }
  /** “Hôm nay 3/5” */
  pace?: string
  /** có video chờ bạn → chấm vàng */
  waiting?: boolean
}

export function ChannelSwitcher({ channels, current, sub, onPick }: {
  channels: SwitcherChannel[]
  /** id kênh đang xem; undefined = Mọi kênh */
  current?: string
  /** dòng phụ “Hôm nay …” của mục Mọi kênh */
  sub?: string
  /** bắt buộc: điều hướng theo route có kiểu — shell truyền pickChannelRoute */
  onPick: (channelId: string | undefined) => void
}) {
  const { root, open, setOpen } = usePopup()

  const isAll = current === undefined
  const selected = channels.find((c) => c.id === current)
  const pick = (id: string | undefined) => { onPick(id); setOpen(false) }

  const stack = (size: number, fontSize: number) => (
    <span aria-hidden="true" className="vv-stack">
      {(channels.length ? channels.slice(0, 3) : [{ id: '_', name: '', initials: '·' }]).map((c) => (
        <ChannelAvatar key={c.id} name={c.name || '·'} initials={c.initials ?? '·'} color={c.color} size={size} stacked
          style={{ fontSize }} />
      ))}
    </span>
  )

  const option = (key: string, checked: boolean, onSelect: () => void, body: ReactNode, trailing?: ReactNode) => (
    <button key={key} type="button" role="menuitemradio" aria-checked={checked} onClick={onSelect} className="vv-switcher__item">
      {body}
      {trailing}
      {checked && <Icon name="tick" size={16} style={{ color: 'var(--color-accent)' }} />}
    </button>
  )

  return (
    <div ref={root} style={{ position: 'relative', flex: '0 0 auto' }}>
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`${t.shell.channelSwitcher} · ${t.shell.viewingAllChannels}`}
        title={t.shell.channelSwitcher}
        onClick={() => setOpen(!open)}
        className="vv-switcher"
      >
        {isAll ? stack(20, 7.5) : <ChannelAvatar name={selected?.name ?? ''} initials={selected?.initials} color={selected?.color} size={20} style={{ fontSize: 8 }} />}
        {isAll ? t.nav.allChannels : selected?.name ?? '…'}
        <Icon name="arrow-d" size={13} style={{ color: 'var(--color-fg-4)' }} />
      </button>
      {open && (
        <div role="menu" aria-label={t.shell.channelSwitcher} className="vv-switcher__menu">
          {option('all', isAll, () => pick(undefined), (
            <>
              <span aria-hidden="true" style={{ width: 30, display: 'flex' }}>{stack(16, 6)}</span>
              <span className="vv-switcher__main">
                <span className="vv-switcher__name">{t.nav.allChannels}</span>
                {sub && <span className="vv-switcher__sub num">{sub}</span>}
              </span>
            </>
          ))}
          {channels.length > 0 && <div role="separator" className="vv-switcher__sep" />}
          {channels.map((c) =>
            option(c.id, current === c.id, () => pick(c.id), (
              <>
                <ChannelAvatar name={c.name} initials={c.initials} color={c.color} size={30} style={{ fontSize: 10.5 }} />
                <span className="vv-switcher__main">
                  <span className="vv-switcher__name">{c.name}</span>
                  {c.pace && <span className="vv-switcher__sub num">{c.pace}</span>}
                </span>
              </>
            ), c.waiting ? <span role="img" aria-label="Có video chờ bạn" className="vv-switcher__waitdot" /> : undefined),
          )}
          <div role="separator" className="vv-switcher__sep" />
          <a href={`#${paths.channelNew()}`} role="menuitem" className="vv-switcher__new">
            <Icon name="add-square" size={18} />
            {t.nav.createChannel}
          </a>
        </div>
      )}
    </div>
  )
}
