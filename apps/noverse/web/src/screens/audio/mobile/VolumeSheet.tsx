import * as stylex from '@stylexjs/stylex'
import { getOutline } from '../../../api/client'
import { keys, useResource } from '../../../data'
import { t } from '../../../i18n/vi'
import { BottomSheet } from '../../../mobile/primitives'
import { color, line } from '@noverse/ui/tokens.stylex'
import { Icon } from '@noverse/ui'
import { volumeChapterCount } from '../../project/toc'

const styles = stylex.create({
  content: { padding: '0 16px 24px' },
  group: { borderRadius: 16, backgroundColor: color['--color-card'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], overflow: 'hidden' },
  row: { width: '100%', minHeight: 60, padding: '12px 14px', display: 'flex', alignItems: 'center', gap: 12, textAlign: 'left', borderWidth: 0, borderBottomWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], backgroundColor: 'transparent', color: color['--color-fg'] },
  copy: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 },
  title: { fontSize: 15, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  meta: { fontSize: 12.5, color: color['--color-fg-3'] },
  selected: { color: color['--color-accent'] },
})

/** Outline headers include planned volumes even before any script/audio exists. */
export function VolumeSheet({ project, story, volume, open, onClose, onSelect }: {
  project: string; story: string; volume: number; open: boolean; onClose: () => void; onSelect: (volume: number) => void
}) {
  const outline = useResource(keys.story(project, story, 'outline', 'vol', 0), ({ signal }) => getOutline(project, story, { signal, volume: 0 }), { enabled: open })
  return <BottomSheet open={open} onClose={onClose} title={t.audio.volumePicker.title} height="auto" style={{ maxHeight: 'min(86%, calc(100dvh - var(--m-top) - 12px))' }}>
    <div {...stylex.props(styles.content)}>
      {!!outline.error && <div className="m-error" role="alert">{outline.error instanceof Error ? outline.error.message : String(outline.error)}<button onClick={() => void outline.refresh()}>{t.audio.generation.retry}</button></div>}
      {!outline.data && !outline.error && <p role="status">{t.audio.volumePicker.loading}</p>}
      <div {...stylex.props(styles.group)}>{outline.data?.volumes.map(item => {
        const count = volumeChapterCount(item)
        const title = item.title.replace(/^Tập\s+\d+\s*[·—-]?\s*/i, '').trim()
        const label = `${t.crumb.volume(item.index)}${title ? ` · ${title}` : ''}`
        return <button type="button" key={item.index} {...stylex.props(styles.row)} disabled={count === 0} aria-current={item.index === volume ? 'true' : undefined} onClick={() => onSelect(item.index)}>
          <span {...stylex.props(styles.copy)}><span {...stylex.props(styles.title)} title={label}>{label}</span><span {...stylex.props(styles.meta)}>{count ? t.audio.volumePicker.chapters(count) : t.audio.volumePicker.locked(item.index - 1)}</span></span>
          {item.index === volume && <Icon name="tick-circle" size={20} {...stylex.props(styles.selected)} />}
        </button>
      })}</div>
    </div>
  </BottomSheet>
}
