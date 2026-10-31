import type { ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit, line } from '@noverse/ui/tokens.stylex'
import { Icon } from '@noverse/ui'
import type { ConfigPresets } from '../../../api/types'
import type { ModelOption } from '../lib/config'
import { modelOptionId } from '../lib/config'
import { findPack, packBadge, packCards, packLabel } from '../lib/packs'
import { styleCards, styleDisplayName } from './style'
import { amWritingCopy } from '../../../i18n/vi'

const styles = stylex.create({
  list: { display: 'flex', flexDirection: 'column', maxHeight: 390, overflowY: 'auto', padding: 6 },
  row: { width: '100%', display: 'flex', alignItems: 'center', gap: 10, padding: '7px 10px', borderWidth: 0, borderRadius: 9, backgroundColor: 'transparent', color: color['--color-fg'], textAlign: 'left', fontSize: 13.5, cursor: 'pointer', ':hover': { backgroundColor: kit['--kit-acc-8'] } },
  selected: { backgroundColor: kit['--kit-acc-8'] },
  copy: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 },
  desc: { fontSize: 12, color: kit['--kit-fg-meta'] },
  heading: { padding: '10px 10px 5px', fontSize: 11.5, fontWeight: 500, color: kit['--kit-fg-meta'] },
  footer: { borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: line['--line-1'], padding: '10px', fontSize: 12, color: color['--color-accent'] },
  footerBare: { padding: 0, borderTopWidth: 0 },
  badge: { marginLeft: 6, fontSize: 10, padding: '2px 5px', borderRadius: 4, backgroundColor: kit['--kit-ok-8'], color: color['--color-ok'], fontWeight: 400 },
  name: { display: 'flex', alignItems: 'center', flexWrap: 'wrap' },
  listFree: { maxHeight: 'none', overflowY: 'visible' },
})
function PickRow({ name, desc, badge, selected, disabled, onPick }: { name: string; desc?: string; badge?: string; selected: boolean; disabled?: boolean; onPick: () => void }) {
  return <button type="button" role="option" aria-selected={selected} disabled={disabled} {...stylex.props(styles.row, selected && styles.selected)} onClick={onPick}><span {...stylex.props(styles.copy)}><span {...stylex.props(styles.name)}>{name}{badge && <span {...stylex.props(styles.badge)}>{badge}</span>}</span>{desc && <span {...stylex.props(styles.desc)}>{desc}</span>}</span>{selected && <Icon name="tick-circle" size={16} />}</button>
}
export function StyleQuickPicker({ value, onPick, projectStyle, presets, disabled, footer, samples, variant, inheritance = 'project' }: {
  value: string | null; onPick: (value: string | null) => void; projectStyle: string; presets: ConfigPresets | null; disabled?: boolean; footer?: ReactNode; samples?: { count: number; chars: number } | null; variant?: 'newstory'; inheritance?: 'project' | 'global'
}) {
  const packs = packCards(presets?.packs)
  const cards = styleCards(presets?.styles ?? [], value ?? '', { packs: packs.map((p) => p.id), custom: 'story', samples }).filter((c) => !c.disabled && c.id)
  const parentPack = findPack(presets?.packs, projectStyle)
  const parentLabel = variant === 'newstory' && parentPack ? packLabel(parentPack) : styleDisplayName(presets, projectStyle)
  return <div role="listbox" aria-label={variant === 'newstory' ? amWritingCopy.styleTitle : 'Văn phong'} {...stylex.props(styles.list, variant === 'newstory' && styles.listFree)}>
    <PickRow name={variant === 'newstory' ? 'Theo dự án' : `${inheritance === 'global' ? 'Chung mọi truyện' : 'Theo dự án'} · ${parentLabel}`} desc={variant === 'newstory' ? `${parentLabel} · mặc định của dự án` : inheritance === 'global' ? 'Đổi theo mặc định chung' : 'Đổi theo khi dự án đổi'} selected={value === null} disabled={disabled} onPick={() => onPick(null)} />
    {packs.length > 0 && <div {...stylex.props(styles.heading)}>Gói thể loại · có chấm</div>}
    {packs.map((p) => <PickRow key={p.id} name={p.label} badge={packBadge(p.pack).ok ? 'có chấm' : 'sắp có'} desc={p.meta} selected={value === p.id} disabled={disabled || !packBadge(p.pack).ok} onPick={() => onPick(p.id)} />)}
    <div {...stylex.props(styles.heading)}>Kiểu viết</div>
    {cards.map((c) => <PickRow key={c.id} name={c.label} desc={c.desc} selected={value === c.id} disabled={disabled} onPick={() => onPick(c.id)} />)}
    {footer && <div {...stylex.props(styles.footer, variant === 'newstory' && styles.footerBare)}>{footer}</div>}
  </div>
}
export function WriterQuickPicker({ value, onPick, projectModel, models, presets, disabled, footer, variant }: {
  value: ModelOption | null; onPick: (value: ModelOption | null) => void; projectModel: string; models: ModelOption[]; presets?: ConfigPresets | null; disabled?: boolean; footer?: ReactNode; variant?: 'newstory'
}) {
  const providers = [...new Set(models.map((m) => m.provider))]
  return <div role="listbox" aria-label={variant === 'newstory' ? amWritingCopy.writerTitle : 'Người viết'} {...stylex.props(styles.list, variant === 'newstory' && styles.listFree)}>
    <PickRow name={variant === 'newstory' ? 'Theo dự án' : `Theo dự án · ${projectModel}`} desc={variant === 'newstory' ? `${projectModel} · mặc định của dự án` : undefined} selected={value === null} disabled={disabled} onPick={() => onPick(null)} />
    {providers.map((p) => <div key={p}><div {...stylex.props(styles.heading)}>{presets?.providers.find((item) => item.id === p)?.label ?? p}</div>{models.filter((m) => m.provider === p).map((m) => <PickRow key={modelOptionId(m)} name={m.model} selected={!!value && modelOptionId(m) === modelOptionId(value)} disabled={disabled} onPick={() => onPick(m)} />)}</div>)}
    {models.length === 0 && <span {...stylex.props(styles.heading)}>Chưa có model. Thêm ở Kết nối AI.</span>}
    {footer && <div {...stylex.props(styles.footer, variant === 'newstory' && styles.footerBare)}>{footer}</div>}
  </div>
}
