import { Suspense, lazy } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
// Trình phát xem thử của editor — tách chunk riêng để trang template không kéo
// cả editor vào gói đầu. Chỉ tải khi template đã có project xem thử.
const LazyPreviewPlayer = lazy(() =>
  import('../../../editor/ui/preview-player/preview-player').then((mod) => ({ default: mod.PreviewPlayer })))

export function TemplatePreviewPlayer({ projectId }: { projectId: string }) {
  return (
    <div className={`vt-preview-player ${stylex.props(sx.player).className}`} data-project={projectId}>
      <Suspense fallback={<div className={`vt-preview-player-fallback ${stylex.props(sx.fallback).className}`} role="status">Đang tải bản xem thử…</div>}>
        <LazyPreviewPlayer projectId={projectId} />
      </Suspense>
    </div>
  )
}

const sx = stylex.create({
  player: { position: 'relative', width: '100%', minWidth: 0, borderTopLeftRadius: 'var(--vv-r-9)', borderTopRightRadius: 'var(--vv-r-9)', borderBottomRightRadius: 'var(--vv-r-9)', borderBottomLeftRadius: 'var(--vv-r-9)', overflowX: 'hidden', overflowY: 'hidden', backgroundColor: 'transparent' },
  fallback: { display: 'flex', alignItems: 'center', justifyContent: 'center', aspectRatio: '16 / 9', width: '100%', borderTopWidth: 'var(--vv-s-1)', borderRightWidth: 'var(--vv-s-1)', borderBottomWidth: 'var(--vv-s-1)', borderLeftWidth: 'var(--vv-s-1)', borderTopStyle: 'solid', borderRightStyle: 'solid', borderBottomStyle: 'solid', borderLeftStyle: 'solid', borderTopColor: line['--line-2'], borderRightColor: line['--line-2'], borderBottomColor: line['--line-2'], borderLeftColor: line['--line-2'], borderTopLeftRadius: 'var(--vv-r-9)', borderTopRightRadius: 'var(--vv-r-9)', borderBottomRightRadius: 'var(--vv-r-9)', borderBottomLeftRadius: 'var(--vv-r-9)', fontSize: 'var(--vv-s-12)', color: color['--color-fg-3'] },
})
