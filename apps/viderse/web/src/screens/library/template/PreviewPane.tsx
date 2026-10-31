import { Icon } from '@noverse/ui'
// Khung Xem thử của trang template: trạng thái lấy từ API (`fresh|stale|none`),
// nhưng có sửa chưa lưu thì hiện `stale` (artboard). Nút dựng lại là nút chính
// duy nhất của màn và chỉ khi `stale`; dựng lại cần video đã làm xong nên khi
// chưa có video thì khoá kèm lý do — bấm không làm gì.
import { ActionButton } from '../../../components'
import { vi as t } from '../../../i18n/vi'
import { clock } from '../fmt'
import type { TemplateDetail } from '../../../api/library'

export function PreviewPane({ detail, dirty }: { detail: TemplateDetail; dirty: boolean }) {
  const copy = t.library.template.preview
  const state = dirty ? 'stale' : detail.preview.state
  const builtAt = detail.preview.builtAt ? clock(detail.preview.builtAt) : null
  return (
    <section className="vt-preview" aria-labelledby="vt-preview-title">
      <div className="vt-preview-head">
        <h2 id="vt-preview-title">{copy.title}</h2>
        <span className="vt-badge" data-state={state}>
          {copy.badge[state]}
        </span>
      </div>
      <div className="vt-preview-frame" data-state={state}>
        <span className="vt-preview-art" aria-hidden="true" />
        {state === 'stale' && (
          <div className="vt-preview-overlay">
            <strong>{copy.staleTitle}</strong>
            <span>{copy.staleBody}</span>
          </div>
        )}
        {state === 'none' && (
          <div className="vt-preview-overlay">
            <strong>{copy.noneTitle}</strong>
            <span>{copy.noneBody}</span>
          </div>
        )}
      </div>
      <p className="vt-hint">{builtAt ? copy.builtAt(builtAt) : copy.never}</p>
      <div className="vt-preview-actions">
        {state === 'stale'
          ? <ActionButton variant="primary" icon={<Icon name="play" size={16} />} lockedReason={copy.buildLocked}>{copy.rebuildStale}</ActionButton>
          : <ActionButton variant="outline" icon={<Icon name="play" size={16} />} lockedReason={copy.buildLocked}>{copy.rebuild}</ActionButton>}
      </div>
      <div className="vt-preview-note">
        <p>{copy.effect}</p>
        <p className="vt-hint">{copy.whenTitle}: {copy.whenBody}</p>
        <div className="vt-preview-source">
          <span className="vt-hint">{copy.videoTitle}</span>
          <ActionButton variant="quiet" lockedReason={copy.noVideo}>{copy.noVideo}</ActionButton>
        </div>
      </div>
    </section>
  )
}
