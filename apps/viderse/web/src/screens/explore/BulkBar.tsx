// Bulk selection opens the shell-owned quick composer without leaving Explore.
import { Button, Icon, toast } from '@noverse/ui'
import { t } from '../../i18n/explore'
import { ActionButton } from '../../components'
import type { Video } from '../../api/rivals'
import { copyText } from './media'
import * as stylex from '@stylexjs/stylex'
import { color, line, radius } from '@noverse/ui/tokens.stylex'
import { space as s } from '../../styles/viderse.stylex'

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
          <Button variant="quiet" size="sm" onClick={() => onTop(10)}>{tt.top10}</Button>
          <Button variant="quiet" size="sm" onClick={() => onTop(20)}>{tt.top20}</Button>
          <Button variant="quiet" size="sm" onClick={onClear}>{tt.deselect}</Button>
        </span>
        <ActionButton size="md" icon={<Icon name="copy" size={15} />} onClick={copyLinks}>{tt.copyLink}</ActionButton>
        <ActionButton variant="primary" size="lg" disabled={busy || !count} onClick={() => onMake(channels[0]?.id ?? '')}>{tt.bulkMake(count)}</ActionButton>
        <Button variant="icon" aria-label={tt.exitSelect} onClick={onExit}><Icon name="close" size={16} /></Button>
      </div>
    </div>
  )
}

const styles = stylex.create({
  root: { position: { default: 'sticky', '@media (max-width: 767px)': 'fixed' }, left: { default: null, '@media (max-width: 767px)': s['--vv-s-12'] }, right: { default: null, '@media (max-width: 767px)': s['--vv-s-12'] }, bottom: { default: s['--vv-s-14'], '@media (max-width: 767px)': 'max(26px, var(--m-safe-bottom))' }, marginTop: { default: `calc(-1 * ${s['--vv-s-76']})`, '@media (max-width: 767px)': 0 }, display: 'flex', justifyContent: 'center', pointerEvents: 'none', zIndex: 5 },
  bar: { pointerEvents: 'auto', borderRadius: radius['--radius-card'], padding: `${s['--vv-s-8']} ${s['--vv-s-8']} ${s['--vv-s-8']} ${s['--vv-s-16']}`, display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: s['--vv-s-8'], backgroundColor: color['--color-surface-4'], borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-2'], boxShadow: 'var(--vv-shadow-bulk)' },
  count: { fontSize: 13.5, fontWeight: 500, fontVariantNumeric: 'tabular-nums' },
  quick: { display: 'flex', gap: s['--vv-s-2'], padding: `0 ${s['--vv-s-4']}` },
  primary: { width: 'auto', flex: '0 0 auto', height: s['--vv-s-32'], padding: '0 var(--vv-s-10)', gap: s['--vv-s-6'], borderRadius: '8px', borderWidth: '1px', borderStyle: 'solid', borderColor: 'transparent', backgroundColor: color['--color-fg'], color: color['--color-bg'], fontWeight: 600 },
})
