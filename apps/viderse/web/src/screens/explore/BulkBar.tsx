// Thanh chọn nhiều: chọn kênh đích trước khi xếp các video đã chọn.
import { Icon, toast, usePopup } from '@noverse/ui'
import { vi as t } from '../../i18n/vi'
import { ActionButton } from '../../components'
import type { Video } from '../../api/rivals'
import { copyText } from './VideoGrid'
import * as stylex from '@stylexjs/stylex'
import { color, line, radius } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { exploreStyles } from './explore.stylex'
import { shellChrome } from '../../shell/shell.stylex'

export function BulkBar({ count, selected, onTop, onClear, onExit, channels, onMake, busy = false }: {
  count: number
  selected: readonly Video[]
  onTop: (n: number) => void
  onClear: () => void
  onExit: () => void
  channels: readonly { id: string; name: string }[]
  onMake: (channelId: string) => void
  busy?: boolean
}) {
  const tt = t.screens.explore
  const { root, open, setOpen } = usePopup()
  const copyLinks = async () => {
    const urls = selected.map((v) => v.url)
    const ok = await copyText(urls.join('\n'))
    if (ok) toast.ok(tt.copied)
    else toast.err(tt.copyFailed)
  }
  return (
    <div className={`vx-bulk ${stylex.props(styles.root).className}`}>
      <div className={`vx-bulk__bar ${stylex.props(styles.bar).className}`} role="region" aria-label={tt.bulkRegion}>
        <span className={`vx-bulk__count num ${stylex.props(styles.count).className}`}>{tt.selected(count)}</span>
        <span className={`vx-bulk__quick ${stylex.props(styles.quick).className}`}>
          <button {...stylex.props(styles.quickButton, exploreStyles.touchTarget)} type="button" onClick={() => onTop(10)}>{tt.top10}</button>
          <button {...stylex.props(styles.quickButton, exploreStyles.touchTarget)} type="button" onClick={() => onTop(20)}>{tt.top20}</button>
          <button {...stylex.props(styles.quickButton, exploreStyles.touchTarget)} type="button" onClick={onClear}>{tt.deselect}</button>
        </span>
        <ActionButton className="vx-bulk__copy" xstyle={[styles.action, styles.copy, exploreStyles.touchTarget]} size="md" icon={<Icon name="copy" size={15} />} onClick={copyLinks}>{tt.copyLink}</ActionButton>
        <div ref={root} className={`vv-switcher-root ${stylex.props(styles.menuRoot).className}`}>
          <ActionButton className="vx-bulk__make" xstyle={[styles.action, styles.make, exploreStyles.touchTarget]} variant="primary" size="md"
            lockedReason={channels.length ? undefined : tt.lockedReason}
            disabled={busy}
            aria-haspopup="menu" aria-expanded={open}
            icon={<Icon name="clock" size={15} />}
            onClick={() => setOpen(!open)}>
            {channels[0] ? tt.bulkMakeFor(count, channels[0].name) : tt.bulkMake(count)}
            <Icon name="arrow-d" size={13} aria-hidden />
          </ActionButton>
          {open && <div role="menu" aria-label={tt.makeChannel} className={`vx-bulk__menu ${stylex.props(shellChrome.menu).className}`}>
            {channels.map((channel) => <button type="button" role="menuitem" {...stylex.props(shellChrome.item, exploreStyles.touchTarget)} key={channel.id}
              onClick={() => { setOpen(false); onMake(channel.id) }}>
              {tt.bulkMakeFor(count, channel.name)}
            </button>)}
          </div>}
        </div>
        <button type="button" className={`vx-iconbtn ${stylex.props(exploreStyles.iconbtn, styles.close, exploreStyles.touchTarget).className}`} aria-label={tt.exitSelect} onClick={onExit}>
          <Icon name="close" size={16} />
        </button>
      </div>
    </div>
  )
}

const styles = stylex.create({
  root: { position: 'sticky', bottom: s['--vv-s-14'], marginTop: `calc(-1 * ${s['--vv-s-76']})`, display: 'flex', justifyContent: 'center', pointerEvents: 'none', zIndex: 5 },
  bar: { pointerEvents: 'auto', borderRadius: radius['--radius-card'], padding: `${s['--vv-s-8']} ${s['--vv-s-8']} ${s['--vv-s-8']} ${s['--vv-s-16']}`, display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: s['--vv-s-8'], backgroundColor: color['--color-surface-4'], borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-2'], boxShadow: 'var(--vv-shadow-bulk)' },
  count: { fontSize: 13.5, fontWeight: 500, fontVariantNumeric: 'tabular-nums' },
  quick: { display: 'flex', gap: s['--vv-s-2'], padding: `0 ${s['--vv-s-4']}` },
  quickButton: { height: s['--vv-s-26'], padding: `0 ${s['--vv-s-8']}`, borderRadius: r['--vv-r-7'], borderWidth: 0, borderStyle: "none", borderColor: "currentColor", backgroundColor: { default: "transparent", ':hover': "var(--vv-hover)" }, color: { default: color['--color-fg-3'], ':hover': color['--color-fg'] }, fontFamily: "inherit", fontSize: 12, fontWeight: "inherit", lineHeight: "inherit", fontStyle: "inherit", fontVariant: "inherit", fontStretch: "inherit", cursor: 'pointer' },
  action: { height: 36, paddingBlock: 0, paddingInline: s['--vv-s-12'], gap: s['--vv-s-7'], borderRadius: radius['--radius-row'], fontSize: 13 },
  copy: { fontWeight: 400, borderColor: line['--line-4'] },
  make: { fontWeight: 600 },
  menuRoot: { position: 'relative', flex: '0 0 auto' },
  close: { width: s['--vv-s-34'], height: s['--vv-s-34'], borderRadius: radius['--radius-button'] },
})
