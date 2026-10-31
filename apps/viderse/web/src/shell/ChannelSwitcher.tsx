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
import * as stylex from '@stylexjs/stylex'
import { color, line, radius } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../styles/viderse.stylex'

export type SwitcherChannel = {
  id: string
  name: string
  initials?: string
  /** ảnh avatar nhận diện của kênh (chưa có thì chữ cái đầu) */
  src?: string
  color?: { bg: string; ring?: string }
  /** “Hôm nay 3/5” */
  pace?: string
  /** có video chờ bạn → chấm vàng */
  waiting?: boolean
}

export function ChannelSwitcher({ channels, current, sub, onPick, mobileBar = false }: {
  channels: SwitcherChannel[]
  /** id kênh đang xem; undefined = Mọi kênh */
  current?: string
  /** dòng phụ “Hôm nay …” của mục Mọi kênh */
  sub?: string
  mobileBar?: boolean
  /** bắt buộc: điều hướng theo route có kiểu — shell truyền pickChannelRoute */
  onPick: (channelId: string | undefined) => void
}) {
  const { root, open, setOpen } = usePopup()

  const isAll = current === undefined
  const selected = channels.find((c) => c.id === current)
  const pick = (id: string | undefined) => {
    try { localStorage.setItem('viderse:workbench-scope', id ?? 'all') } catch { /* storage unavailable */ }
    onPick(id); setOpen(false)
  }

  const stack = (size: number, fontSize: number) => (
    <span aria-hidden="true" className="vv-stack">
      {(channels.length ? channels.slice(0, 3) : [{ id: '_', name: '', initials: '·' }]).map((c) => (
        <ChannelAvatar key={c.id} name={c.name || '·'} initials={c.initials ?? '·'} src={c.src} color={c.color} size={size} stacked
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
    <div ref={root} className={`vv-switcher-root ${stylex.props(styles.root, mobileBar && styles.mobileRoot).className}`}>
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`${t.shell.channelSwitcher} · ${isAll ? t.shell.viewingAllChannels : selected?.name ?? ''}`}
        title={t.shell.channelSwitcher}
        onClick={() => setOpen(!open)}
        className={`vv-switcher ${stylex.props(styles.button, mobileBar && styles.mobileButton).className}`}
      >
        {isAll ? stack(20, 7.5) : <ChannelAvatar name={selected?.name ?? ''} initials={selected?.initials} src={selected?.src} color={selected?.color} size={20} style={{ fontSize: 8 }} />}
        <span className="vv-switcher__label">{isAll ? t.nav.allChannels : selected?.name ?? '…'}</span>
        <Icon name="arrow-d" size={13} style={{ color: 'var(--color-fg-4)' }} />
        {mobileBar && <span aria-hidden="true" {...stylex.props(styles.touchHalo)} />}
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
                <ChannelAvatar name={c.name} initials={c.initials} src={c.src} color={c.color} size={30} style={{ fontSize: 10.5 }} />
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

const MOBILE = '@media (max-width: 767px)'
const NARROW = '@media (max-width: 430px)'
const TOUCH = '@media (max-width: 767px) and (pointer: coarse)'
const styles = stylex.create({
  root: { position: 'relative', flex: '0 0 auto' },
  button: { height: 30, padding: `0 ${s['--vv-s-8']} 0 ${s['--vv-s-5']}`, display: 'flex', alignItems: 'center', gap: s['--vv-s-7'], borderRadius: radius['--radius-control'], borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-1'], backgroundColor: { default: color['--color-surface-1'], ':hover': color['--color-surface-2'] }, color: color['--color-fg'], fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap' },
  mobileRoot: { flex: { default: null, [MOBILE]: '0 1 auto' }, minWidth: { default: null, [MOBILE]: 0 }, maxWidth: { default: null, [MOBILE]: '44%', [NARROW]: '48%' } },
  mobileButton: { height: 36, padding: `0 ${s['--vv-s-8']} 0 ${s['--vv-s-4']}`, borderRadius: r['--vv-r-pill'], gap: s['--vv-s-7'], fontSize: 13.5, minWidth: { default: null, [MOBILE]: 0 }, maxWidth: { default: null, [MOBILE]: '100%' }, overflow: { default: null, [MOBILE]: 'hidden', [TOUCH]: 'visible' }, position: { default: null, [TOUCH]: 'relative' } },
  touchHalo: { display: { default: 'none', [TOUCH]: 'block' }, position: 'absolute', inset: `calc((${s['--vv-s-44']} - 100%) / -2) 0` },
})
