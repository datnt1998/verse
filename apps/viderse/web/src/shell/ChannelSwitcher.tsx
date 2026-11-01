// ChannelSwitcher — bộ đổi kênh ở đầu trang Bàn dựng (04-components).
// Chưa có dữ liệu kênh thì chỉ có “Mọi kênh” + Tạo kênh; menu của kit
// (ActionMenu) chỉ nhận item phẳng nên popover này dựng trực tiếp với
// usePopup + menuitemradio theo đúng vai trò.
// Kiểu: stylex.create bên dưới + shellChrome cho menu dùng chung với BulkBar
// (trước ở styles/components.css .vv-switcher*/.vv-stack).
import type { ReactNode } from 'react'
import { usePopup } from '@noverse/ui'
import { t } from '../i18n/vi'
import { paths } from '../router'
import { ChannelAvatar } from '../components/ChannelAvatar'
import { Icon } from '@noverse/ui'
import * as stylex from '@stylexjs/stylex'
import { color, line, radius } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../styles/viderse.stylex'
import { shellChrome } from './shell.stylex'

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
    <span aria-hidden="true" {...stylex.props(styles.stack)}>
      {(channels.length ? channels.slice(0, 3) : [{ id: '_', name: '', initials: '·' }]).map((c, index) => (
        <ChannelAvatar key={c.id} name={c.name || '·'} initials={c.initials ?? '·'} src={c.src} color={c.color} size={size} stacked
          style={{ fontSize, ...(index > 0 ? { marginLeft: 'calc(-1 * var(--vv-s-6))' } : {}) }} />
      ))}
    </span>
  )

  const option = (key: string, checked: boolean, onSelect: () => void, body: ReactNode, trailing?: ReactNode) => (
    <button key={key} type="button" role="menuitemradio" aria-checked={checked} onClick={onSelect} {...stylex.props(shellChrome.item)}>
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
        <span {...stylex.props(styles.label)}>{isAll ? t.nav.allChannels : selected?.name ?? '…'}</span>
        <Icon name="arrow-d" size={13} style={{ color: 'var(--color-fg-4)' }} />
        {mobileBar && <span aria-hidden="true" {...stylex.props(styles.touchHalo)} />}
      </button>
      {open && (
        <div role="menu" aria-label={t.shell.channelSwitcher} {...stylex.props(shellChrome.menu)}>
          {option('all', isAll, () => pick(undefined), (
            <>
              <span aria-hidden="true" style={{ width: 30, display: 'flex' }}>{stack(16, 6)}</span>
              <span {...stylex.props(styles.main)}>
                <span {...stylex.props(styles.name)}>{t.nav.allChannels}</span>
                {sub && <span className={`num ${stylex.props(styles.sub).className}`}>{sub}</span>}
              </span>
            </>
          ))}
          {channels.length > 0 && <div role="separator" {...stylex.props(styles.sep)} />}
          {channels.map((c) =>
            option(c.id, current === c.id, () => pick(c.id), (
              <>
                <ChannelAvatar name={c.name} initials={c.initials} src={c.src} color={c.color} size={30} style={{ fontSize: 10.5 }} />
                <span {...stylex.props(styles.main)}>
                  <span {...stylex.props(styles.name)}>{c.name}</span>
                  {c.pace && <span className={`num ${stylex.props(styles.sub).className}`}>{c.pace}</span>}
                </span>
              </>
            ), c.waiting ? <span role="img" aria-label="Có video chờ bạn" {...stylex.props(styles.waitdot)} /> : undefined),
          )}
          <div role="separator" {...stylex.props(styles.sep)} />
          <a href={`#${paths.channelNew()}`} role="menuitem" {...stylex.props(styles.new)}>
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
  /* Nhãn kênh co được: tên dài ở thanh hẹp thì cắt ba chấm thay vì đẩy tràn trang. */
  label: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  main: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' },
  name: { fontSize: 13.5, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  sub: { fontSize: 11.5, color: color['--color-fg-4'] },
  sep: { height: 1, marginBlock: s['--vv-s-4'], marginInline: s['--vv-s-6'], backgroundColor: line['--line-1'] },
  waitdot: { width: 8, height: 8, borderRadius: '50%', backgroundColor: color['--color-warn'], flex: '0 0 auto' },
  new: { display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], padding: `${s['--vv-s-8']} ${s['--vv-s-10']}`, borderRadius: radius['--radius-button'], color: { default: color['--color-fg-2'], ':hover': color['--color-fg'] }, backgroundColor: { default: 'transparent', ':hover': color['--color-surface-1'] }, fontSize: 13.5, textDecoration: 'none' },
  stack: { display: 'flex', flex: '0 0 auto' },
})
