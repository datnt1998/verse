// ChannelSwitcher — bộ đổi kênh ở đầu trang Bàn dựng (04-components).
// Điện thoại dùng sheet MMain; máy tính dùng ActionMenu của kit để chung
// bàn phím, focus, lớp Escape và mục chọn một. Avatar nhận diện giữ đúng
// kích thước của thanh và menu; tên dài co trước các nút bên phải.
import { useState } from 'react'
import { ActionMenu, Sheet } from '@noverse/ui'
import { switcherCopy as t } from '../i18n/switcher'
import { navigate, paths } from '../router'
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
  /** số video đang chờ bạn (MobileShell truyền từ ChannelSummary.waitingYou) */
  waitingYou?: number
}

/** Một chữ cái cho avatar chồng: chữ đầu của tên tắt, không phải hai chữ bị che. */
const oneLetter = (channel: SwitcherChannel) => (channel.initials?.trim() || channel.name.trim() || '·').slice(0, 1).toUpperCase()

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
  const [open, setOpen] = useState(false)

  const isAll = current === undefined
  const selected = channels.find((c) => c.id === current)
  // MMain: dòng phụ của “Mọi kênh” = tổng nhịp hôm nay + số kênh có video chờ.
  const pace = channels.reduce((sum, channel) => {
    const match = channel.pace?.match(/(\d+)\s*\/\s*(\d+)/)
    return match ? { done: sum.done + Number(match[1]), cap: sum.cap + Number(match[2]) } : sum
  }, { done: 0, cap: 0 })
  const waitingChannels = channels.filter((channel) => channel.waiting).length
  const hasWaitingCount = channels.some((channel) => channel.waitingYou !== undefined)
  const waitingCount = hasWaitingCount
    ? channels.reduce((sum, channel) => sum + (channel.waitingYou ?? 0), 0)
    : waitingChannels
  const waitingLine = waitingCount > 0 ? (hasWaitingCount ? t.waiting(waitingCount) : t.waitingChannels(waitingCount)) : ''
  const allSub = sub ?? (pace.cap > 0 ? `Hôm nay ${pace.done}/${pace.cap}${waitingLine ? ` · ${waitingLine}` : ''}` : undefined)
  const pick = (id: string | undefined) => {
    try { localStorage.setItem('viderse:workbench-scope', id ?? 'all') } catch { /* storage unavailable */ }
    onPick(id); setOpen(false)
  }

  /* Chồng avatar của “Mọi kênh”: một chữ cái mỗi avatar để chữ không bị che
     (04-components › Đầu trang điện thoại). Điện thoại 22px chồng −7px viền
     2px; máy tính 20px chồng −6px viền 1.5px (artboard MMain / Main). */
  const stack = (
    <span aria-hidden="true" {...stylex.props(styles.stack, mobileBar && styles.stackPhone)}>
      {channels.slice(0, 3).map((c, index) => (
        <ChannelAvatar key={c.id} name={c.name || '·'} initials={oneLetter(c)} src={c.src} color={c.color}
          size={mobileBar ? 22 : 20} fontSize={10} stacked ringWidth={mobileBar ? 2 : 1.5}
          className={index > 0 ? stylex.props(mobileBar ? styles.overlapPhone : styles.overlapDesktop).className : undefined} />
      ))}
    </span>
  )


  /* Chưa có kênh: không có gì để đổi — nút cùng cỡ mở luồng tạo kênh
     (04-components › Đầu trang điện thoại › Trường hợp trống). */
  if (channels.length === 0) {
    return (
      <div className={`vv-switcher-root ${stylex.props(styles.root, mobileBar && styles.mobileRoot).className}`}>
        <a
          href={`#${paths.channelNew()}`}
          aria-label={t.createChannel}
          title={t.createChannel}
          className={`vv-switcher vv-switcher--new ${stylex.props(styles.button, styles.newTone, mobileBar ? styles.mobileNew : styles.desktopNew).className}`}
        >
          <Icon name="add" size={16} />
          <span {...stylex.props(styles.label)}>{t.createChannel}</span>
          {mobileBar && <span aria-hidden="true" {...stylex.props(styles.touchHalo)} />}
        </a>
      </div>
    )
  }
  if (!mobileBar) return <div {...stylex.props(styles.root)}>
    <ActionMenu label={`${t.title}, ${isAll ? t.viewingAllChannels : t.selectingChannel(selected?.name ?? '')}`} alwaysVisible width={280}
      triggerStyle={[styles.button, styles.desktopTrigger]}
      trigger={<>{isAll ? stack : <ChannelAvatar name={selected?.name ?? ''} initials={selected?.initials} src={selected?.src} color={selected?.color} size={20} fontSize={8} />}
        <span className={`vv-switcher__name ${stylex.props(styles.label).className}`}>{isAll ? t.allChannels : selected?.name ?? '…'}</span>
        <Icon name="arrow-down-2" size={14} xstyle={styles.chevron} />
      </>}
      items={[
        { id: 'all', label: t.allChannels, selected: isAll, description: allSub,
          icon: <span {...stylex.props(styles.menuStack)}>{channels.slice(0, 3).map((channel, index) => <ChannelAvatar key={channel.id} name={channel.name || '·'} initials={channel.initials ?? oneLetter(channel)} src={channel.src} color={channel.color} size={16} fontSize={6} stacked ringColor="var(--color-surface-2)" className={index > 0 ? stylex.props(styles.overlapMenu).className : undefined} />)}</span>,
          onSelect: () => pick(undefined) },
        ...channels.map((channel) => ({ id: channel.id, label: channel.name, selected: current === channel.id, description: channel.pace,
          icon: <ChannelAvatar name={channel.name} initials={channel.initials} src={channel.src} color={channel.color} size={30} fontSize={10.5} />,
          meta: channel.waiting ? <span role="img" aria-label="Có video chờ bạn" {...stylex.props(styles.waitdot)} /> : undefined,
          onSelect: () => pick(channel.id) })),
        { id: 'new', label: t.createChannel, icon: <Icon name="add-square" size={18} />, onSelect: () => navigate(paths.channelNew()) },
      ]} />
  </div>

  return (
    <div className={`vv-switcher-root ${stylex.props(styles.root, mobileBar && styles.mobileRoot).className}`}>
      <button
        type="button"
        aria-haspopup={mobileBar ? 'dialog' : 'menu'}
        aria-expanded={open}
        aria-label={`${t.title}, ${isAll ? t.viewingAllChannels : t.selectingChannel(selected?.name ?? '')}`}
        title={t.title}
        onClick={() => setOpen(!open)}
        className={`vv-switcher ${stylex.props(styles.button, mobileBar && styles.mobileButton).className}`}
      >
        {isAll
          ? stack
          : <ChannelAvatar name={selected?.name ?? ''} initials={selected?.initials} src={selected?.src} color={selected?.color}
              size={mobileBar ? 26 : 20} fontSize={mobileBar ? 10 : 8} />}
        <span className={`vv-switcher__name ${stylex.props(styles.label).className}`}>{isAll ? t.allChannels : selected?.name ?? '…'}</span>
        {/* Mũi tên điện thoại theo màu chữ gợi ý của artboard (.vv-mswitch > .ic). */}
        <Icon name="arrow-d" size={mobileBar ? 14 : 13} xstyle={mobileBar ? styles.chevronMobile : styles.chevron} />
        {mobileBar && <span aria-hidden="true" {...stylex.props(styles.touchHalo)} />}
      </button>
      {open && mobileBar && (
        <Sheet open onClose={() => setOpen(false)} title={t.title} size="md" height="auto" homeIndicator>
          <div {...stylex.props(styles.sheet)}>
            <button type="button" aria-pressed={isAll} onClick={() => pick(undefined)} {...stylex.props(styles.sheetRow, isAll && styles.sheetRowOn)}>
              <span aria-hidden="true" {...stylex.props(styles.sheetStack)}>
                {channels.slice(0, 3).map((c, index) => (
                  <ChannelAvatar key={c.id} name={c.name || '·'} initials={oneLetter(c)} src={c.src} color={c.color}
                    size={22} fontSize={10} stacked ringWidth={2} ringColor="var(--color-surface-3)"
                    className={index > 0 ? stylex.props(styles.overlapSheet).className : undefined} />
                ))}
              </span>
              <span {...stylex.props(styles.main)}>
                <span {...stylex.props(styles.sheetName)}>{t.allChannels}</span>
                {allSub && <span className={`num ${stylex.props(styles.sub).className}`}>{allSub}</span>}
              </span>
              {isAll && <Icon name="tick" size={18} style={{ color: 'var(--color-accent)' }} />}
            </button>
            {channels.map((c) => <button key={c.id} type="button" aria-pressed={current === c.id} onClick={() => pick(c.id)} {...stylex.props(styles.sheetRow, current === c.id && styles.sheetRowOn)}>
              <ChannelAvatar name={c.name} initials={c.initials} src={c.src} color={c.color} size={30} fontSize={10.5} />
              <span {...stylex.props(styles.main)}>
                <span {...stylex.props(styles.sheetName)}>{c.name}</span>
                <span className={`num ${stylex.props(styles.sub).className}`}>{c.pace ?? ''}{c.waitingYou ? ` · ${t.waiting(c.waitingYou)}` : c.waiting ? ` · ${t.waitingOne}` : ''}</span>
              </span>
              {current === c.id && <Icon name="tick" size={18} style={{ color: 'var(--color-accent)' }} />}
            </button>)}
            <a href={`#${paths.channelNew()}`} onClick={() => setOpen(false)} {...stylex.props(styles.sheetNew)}>
              <Icon name="add-square" size={18} />{t.createChannel}
            </a>
          </div>
        </Sheet>
      )}
    </div>
  )
}

const MOBILE = '@media (max-width: 767px)'
const TOUCH = '@media (max-width: 767px) and (pointer: coarse)'
const styles = stylex.create({
  root: { position: 'relative', flex: '0 0 auto' },
  chevron: { color: color['--color-fg-4'] },
  chevronMobile: { color: 'var(--vv-st-neutral)' },
  button: { height: 30, padding: `0 ${s['--vv-s-8']} 0 ${s['--vv-s-5']}`, display: 'flex', alignItems: 'center', gap: s['--vv-s-7'], borderRadius: radius['--radius-control'], borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-1'], backgroundColor: { default: color['--color-surface-1'], ':hover': color['--color-surface-2'] }, color: color['--color-fg'], fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap', textDecoration: 'none' },
  desktopTrigger: { width: 'auto' },
  /* Chỉ bộ đổi kênh co: nút Việc nền và avatar giữ nguyên chỗ (r36). */
  mobileRoot: { flex: { default: null, [MOBILE]: '0 1 auto' }, minWidth: { default: null, [MOBILE]: 0 } },
  mobileButton: { height: 36, minWidth: { default: null, [MOBILE]: 0 }, maxWidth: { default: null, [MOBILE]: '100%' }, padding: { default: null, [MOBILE]: `0 ${s['--vv-s-10']} 0 ${s['--vv-s-4']}` }, borderRadius: { default: null, [MOBILE]: r['--vv-r-pill'] }, borderColor: { default: line['--line-1'], [MOBILE]: line['--line-2'] }, fontSize: { default: null, [MOBILE]: 13.5 }, overflow: { default: null, [MOBILE]: 'hidden', [TOUCH]: 'visible' }, position: { default: null, [TOUCH]: 'relative' } },
  /* Nút “Tạo kênh” cùng cỡ bộ đổi kênh nhưng không có menu (r36). */
  newTone: { color: color['--color-fg-2'], gap: s['--vv-s-7'] },
  desktopNew: { padding: `0 ${s['--vv-s-12']} 0 ${s['--vv-s-10']}` },
  mobileNew: { height: { default: null, [MOBILE]: 36 }, padding: { default: null, [MOBILE]: `0 ${s['--vv-s-12']} 0 ${s['--vv-s-10']}` }, borderRadius: { default: null, [MOBILE]: r['--vv-r-pill'] }, borderColor: { default: line['--line-1'], [MOBILE]: line['--line-2'] }, fontSize: { default: null, [MOBILE]: 13.5 }, position: { default: null, [TOUCH]: 'relative' } },
  touchHalo: { display: { default: 'none', [TOUCH]: 'block' }, position: 'absolute', inset: `calc((${s['--vv-s-44']} - 100%) / -2) 0` },
  /* Nhãn kênh co được: tên dài ở thanh hẹp thì cắt ba chấm thay vì đẩy tràn trang. */
  label: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  main: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' },
  name: { fontSize: 13.5, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  sub: { fontSize: 11.5, color: color['--color-fg-4'] },
  waitdot: { width: 8, height: 8, borderRadius: '50%', backgroundColor: color['--color-warn'], flex: '0 0 auto' },
  stack: { display: 'flex', flex: '0 0 auto' },
  stackPhone: { paddingLeft: { default: null, [MOBILE]: s['--vv-s-2'] } },
  overlapPhone: { marginLeft: `calc(-1 * ${s['--vv-s-7']})` },
  overlapDesktop: { marginLeft: `calc(-1 * ${s['--vv-s-6']})` },
  /* Menu: chồng avatar 16px hai chữ cái, lệch −5px như popover máy tính. */
  menuStack: { display: 'flex', flex: '0 0 auto' },
  overlapMenu: { marginLeft: `calc(-1 * ${s['--vv-s-5']})` },
  sheet: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'], paddingBottom: s['--vv-s-8'] },
  sheetRow: { display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], width: '100%', minHeight: 56, paddingBlock: s['--vv-s-8'], paddingInline: s['--vv-s-6'], borderWidth: 0, borderRadius: r['--vv-r-12'], backgroundColor: 'transparent', color: color['--color-fg'], textAlign: 'left', cursor: 'pointer' },
  sheetRowOn: { backgroundColor: color['--color-surface-2'] },
  sheetName: { fontSize: 14, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  sheetStack: { display: 'flex', flex: '0 0 auto' },
  overlapSheet: { marginLeft: `calc(-1 * ${s['--vv-s-7']})` },
  sheetNew: { display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], padding: `${s['--vv-s-12']} ${s['--vv-s-6']}`, color: color['--color-fg-2'], fontSize: 13.5, textDecoration: 'none' },
})
