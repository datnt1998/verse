// Tab Chi tiết cài cắm — Đã gieo · Đẩy tiếp · Thu hồi, không hiện mã tuyến (Outline.dc.html › tab fs).
import { getForeshadow } from '../../../api/client'
import type { ForeshadowEntry } from '../../../api/types'
import { keys, useResource } from '../../../data'
import { translateHost } from '../../../i18n/hostStrings'
import { EmptyState, Skeleton } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { foreshadowActionLabel } from '../../../lib/labels'
import { Chip } from '../bits'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

/** `status` của sổ cài cắm → hành động trong nhãn (`plant` · `advance` · `resolve` · `abandon`). */
const ACTION_OF_STATUS: Record<string, string> = { planted: 'plant', advanced: 'advance', resolved: 'resolve', abandoned: 'abandon' }

// Artboard chỉ vẽ hai màu (Đã gieo xanh lam · Đẩy tiếp vàng); Thu hồi dùng xanh lá của “Hoàn thành” trong cùng artboard.
// Bỏ tuyến là tuyến đã đóng mà không thu: không có màu riêng, rơi về tông xám trung tính mặc định.
const TONE: Record<string, { background: string; color: string }> = {
  plant: { background: 'rgba(185,208,247,0.12)', color: '#B9D0F7' },
  advance: { background: 'rgba(246,203,140,0.12)', color: '#F6CB8C' },
  resolve: { background: 'rgba(166,237,193,0.12)', color: '#A6EDC1' },
}

export function useForeshadow(project: string, story: string) {
  return useResource(keys.story(project, story, 'foreshadow'), ({ signal }) => getForeshadow(project, story, signal))
}

/** Chương hiện ở góc phải: chương đóng tuyến (thu hồi / bỏ) nếu đã đóng, không thì chương gieo. */
const chapterOf = (entry: ForeshadowEntry) => {
  if (entry.status === 'resolved' && entry.resolved_at) return entry.resolved_at
  if (entry.status === 'abandoned' && entry.abandoned_at) return entry.abandoned_at
  return entry.planted_at
}

/** Dòng phụ dưới mô tả: hạn thu của tuyến còn mở, hoặc lý do bỏ tuyến. */
const detailOf = (entry: ForeshadowEntry) => {
  if (entry.status === 'abandoned') return entry.abandon_reason ? `Lý do bỏ: ${entry.abandon_reason}` : ''
  if (entry.status !== 'resolved' && entry.due_by) return `Dự kiến thu ở Ch. ${entry.due_by}`
  return ''
}

export function ForeshadowTab({ project, story }: { project: string; story: string }) {
  const resource = useForeshadow(project, story)
  const entries = resource.data?.entries
  if (resource.error && !entries) {
    return (
      <EmptyState icon="warning-2" title="Không tải được chi tiết cài cắm"
        hint="Máy chủ chưa đọc được danh sách chi tiết cài cắm của truyện."
        action={
          <button type="button" className="hov-ghost" onClick={() => void resource.refresh()}
            style={{ height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)', background: 'transparent', color: '#EDEDEF', fontSize: 13 }}>
            Thử lại
          </button>
        } />
    )
  }
  if (!entries) return <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>{[0, 1, 2].map((key) => <Skeleton key={key} height={48} radius={14} />)}</div>
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <PanelRefreshBar resourceKeys={[keys.story(project, story, 'foreshadow')]} updatedAt={resource.updatedAt} />
      <StaleBanner error={entries && resource.error} onRetry={resource.refresh} />
      <p style={{ margin: '0 0 4px', fontSize: 13, color: '#8E8E99' }}>
        Những chi tiết đã gieo và cần được nhắc lại hoặc giải đáp ở chương sau. Kiến trúc sư theo dõi và xếp lịch.
      </p>
      {entries.length === 0 && <EmptyState icon="flag" title="Chưa có chi tiết cài cắm" hint="Chi tiết cài cắm được ghi lại khi truyện được viết." />}
      {entries.map((entry) => {
        const action = ACTION_OF_STATUS[entry.status] ?? entry.status
        const tone = TONE[action] ?? { background: 'rgba(255,255,255,0.06)', color: '#A0A0AB' }
        const chapter = chapterOf(entry)
        return (
          <div key={entry.id} className="card" style={{ borderRadius: 14, padding: '12px 14px', display: 'flex', gap: 12, alignItems: 'flex-start' }}>
            <Chip tone={tone} style={{ width: 72, justifyContent: 'center' }}>{foreshadowActionLabel(action)}</Chip>
            <span style={{ flex: '1 1 auto', display: 'flex', flexDirection: 'column', gap: 3 }}>
              <span style={{ fontSize: 13.5, color: entry.description ? '#C9C9D1' : '#7E7E8A', lineHeight: 1.55 }}>
                {entry.description ? translateHost(entry.description) : 'Chưa có mô tả'}
              </span>
              {detailOf(entry) && <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{detailOf(entry)}</span>}
            </span>
            <span className="num" style={{ fontSize: 12, color: '#7E7E8A', whiteSpace: 'nowrap' }}>{chapter > 0 ? `Ch. ${chapter}` : '—'}</span>
          </div>
        )
      })}
    </div>
  )
}
