import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { color, line } from '@noverse/ui/tokens.stylex'
import type { TemplateSharedAsset } from '../../../api/templateAssets'

const kinds: Record<TemplateSharedAsset['kind'], string> = { video: 'Video', audio: 'Âm thanh', image: 'Ảnh', text: 'Chữ', shape: 'Đồ hoạ', group: 'Nhóm lớp' }

export function TemplateAssetList({ assets }: { assets: readonly TemplateSharedAsset[] | undefined }) {
  return <div {...stylex.props(s.root)} role="group" aria-label="Asset dùng chung">
    <span {...stylex.props(s.heading)}>Dùng chung</span>
    {assets?.length ? <ul {...stylex.props(s.list)}>
      {assets.map(asset => <li key={asset.id} {...stylex.props(s.item)}>
        <Icon name="layer" size={16} xstyle={s.icon} />
        <span {...stylex.props(s.details)}><span>{asset.name}</span><small {...stylex.props(s.note)}>{[kinds[asset.kind], asset.loop ? 'Lặp' : '', !asset.visible ? 'Đang tắt' : ''].filter(Boolean).join(' · ')}</small></span>
      </li>)}
    </ul> : <span {...stylex.props(s.note)}>{assets ? 'Template này không có asset dùng chung.' : 'Chưa có thông tin asset của template.'}</span>}
    {!!assets?.length && <small {...stylex.props(s.note)}>Theo template đã chốt của video này.</small>}
  </div>
}

const s = stylex.create({
  root: { display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-8)' },
  heading: { color: color['--color-fg-3'], fontWeight: 500 },
  list: { listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' },
  item: { display: 'flex', alignItems: 'center', gap: 'var(--vv-s-8)', padding: 'var(--vv-s-8) var(--vv-s-10)', borderRadius: 'var(--vv-r-8)', borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], backgroundColor: color['--color-surface-0'] },
  icon: { color: color['--color-fg-3'], flexShrink: 0 },
  details: { minWidth: 0, display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-2)', overflowWrap: 'anywhere' },
  note: { color: 'var(--vv-fg-note)', lineHeight: 1.5 },
})
