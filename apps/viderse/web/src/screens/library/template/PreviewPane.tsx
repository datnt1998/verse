import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
import { Icon } from '@noverse/ui'
import { useState } from 'react'
// Khung Xem thử của trang template: trạng thái sống từ
// GET /api/templates/:id/preview (dựng/huỷ qua POST/DELETE), có sửa chưa lưu
// thì `fresh` hiện `stale`. Bản xem thử là project editor đổ nội dung từ một
// video đã xong; trình phát là chunk riêng, chỉ tải khi có project.
import { ActionButton } from '../../../components'
import { ResponsiveSelect } from '../../../components/responsive-overlays'
import { t } from '../../../i18n/library'
import { clock } from '../fmt'
import type { TemplateDetail } from '../../../api/library'
import { TemplatePreviewPlayer } from './TemplatePreviewPlayer'
import { useTemplatePreview } from './useTemplatePreview'

export function PreviewPane({ detail, dirty }: { detail: TemplateDetail; dirty: boolean }) {
  const copy = t.library.template.preview
  const preview = useTemplatePreview(detail.id, detail, dirty)
  const [pickedJob, setPickedJob] = useState<string | undefined>(undefined)
  // Video không dựng được với recipe hiện tại vẫn hiện kèm lý do (disabled),
  // nhưng không được chọn mặc định và không bật nút dựng.
  const buildable = preview.sources.filter((source) => !source.disabled)
  const knownJob = preview.jobId && preview.sources.some((source) => source.jobId === preview.jobId && !source.disabled) ? preview.jobId : undefined
  const sourceJob = pickedJob ?? knownJob ?? buildable[0]?.jobId
  const canBuild = buildable.length > 0 && !preview.busy
  const lockedReason = canBuild ? undefined : (preview.sources[0]?.reason ?? copy.buildLocked)
  const builtAt = preview.builtAt ? clock(preview.builtAt) : null
  const state = preview.state
  const building = state === 'building'
  const progress = preview.task
    ? copy.progress(preview.task.phase || copy.badge.building, preview.task.done, preview.task.total)
    : copy.badge.building
  const build = () => { void preview.start(sourceJob, preview.minutes) }
  const cancel = () => { void preview.cancel() }

  return (
    <section className={`vt-preview ${stylex.props(sx.preview).className}`} aria-labelledby="vt-preview-title">
      <div className={`vt-preview-head ${stylex.props(sx.previewHead).className}`}>
        <h2 {...stylex.props(sx.vtPreviewHeadH2)} id="vt-preview-title">{copy.title}</h2>
        <span className={`vt-badge ${stylex.props(sx.badge, state === 'fresh' && sx.badgeDataStateFresh, state === 'stale' && sx.badgeDataStateStale, building && sx.badgeDataStateBuilding).className}`} data-state={state}>
          {copy.badge[state]}
        </span>
      </div>

      {preview.projectId ? (
        <TemplatePreviewPlayer projectId={preview.projectId} />
      ) : (
        <div className={`vt-preview-frame ${stylex.props(sx.previewFrame).className}`} data-state={state}>
          <span className={`vt-preview-art ${stylex.props(sx.previewArt).className}`} aria-hidden="true" />
          {state === 'stale' && (
            <div className={`vt-preview-overlay ${stylex.props(sx.previewOverlay).className}`}>
              <strong {...stylex.props(sx.vtPreviewOverlayStrong)}>{copy.staleTitle}</strong>
              <span {...stylex.props(sx.vtPreviewOverlaySpan)}>{copy.staleBody}</span>
            </div>
          )}
          {(state === 'none' || building) && (
            <div className={`vt-preview-overlay ${stylex.props(sx.previewOverlay).className}`}>
              <strong {...stylex.props(sx.vtPreviewOverlayStrong)}>{state === 'none' ? copy.noneTitle : copy.badge.building}</strong>
              <span {...stylex.props(sx.vtPreviewOverlaySpan)}>{state === 'none' ? copy.noneBody : progress}</span>
            </div>
          )}
        </div>
      )}

      <p className={`vt-hint ${stylex.props(sx.hint).className}`}>
        {builtAt ? copy.builtAt(builtAt) : copy.never}
        {state === 'stale' && preview.projectId ? ` · ${copy.staleBody}` : null}
      </p>

      <div className={`vt-preview-source ${stylex.props(sx.previewSource).className}`}>
        <label className={`vt-hint ${stylex.props(sx.sourceLabel).className}`} htmlFor="vt-preview-source">{copy.videoTitle}</label>
        {preview.sources.length > 0 ? (
          <ResponsiveSelect
            id="vt-preview-source"
            aria-label={copy.sourceAria}
            value={sourceJob ?? ''}
            disabled={building || preview.busy}
            onChange={(value) => setPickedJob(value || undefined)}
            xstyle={sx.sourceField}
            options={preview.sources.map((source) => ({
              value: source.jobId,
              // Video không dựng được vẫn hiện kèm lý do, nhưng không chọn được.
              label: `${copy.sourceOption(source.title, Math.max(1, Math.round(source.durationSec / 60)))}${source.disabled && source.reason ? ` — ${source.reason}` : ''}`,
              disabled: source.disabled,
            }))}
          />
        ) : (
          <span className={`vt-hint ${stylex.props(sx.hint3).className}`}>{copy.noVideo}</span>
        )}
      </div>

      {building ? (
        <div className={`vt-preview-actions ${stylex.props(sx.previewActions).className}`}>
          <span className={`vt-hint ${stylex.props(sx.progress).className}`} role="status">{progress}</span>
          <ActionButton xstyle={[sx.vtPreviewActionsActionButton]} variant="outline" icon={<Icon xstyle={[sx.vtPreviewActionsIcon]} name="close" size={16} />} disabled={preview.busy} onClick={cancel}>{copy.cancel}</ActionButton>
        </div>
      ) : (
        <div className={`vt-preview-actions ${stylex.props(sx.previewActions).className}`}>
          <ActionButton
            xstyle={[sx.vtPreviewActionsActionButton]}
            variant={state === 'fresh' ? 'outline' : 'primary'}
            icon={<Icon xstyle={[sx.vtPreviewActionsIcon]} name={state === 'fresh' ? 'refresh' : 'play'} size={16} />}
            lockedReason={lockedReason}
            onClick={canBuild ? build : undefined}
          >
            {state === 'fresh' ? copy.rebuild : copy.rebuildStale}
          </ActionButton>
        </div>
      )}

      {preview.error && <p className={`vt-fielderr ${stylex.props(sx.error).className}`} role="alert">{preview.error}</p>}

      <div className={`vt-preview-note ${stylex.props(sx.previewNote).className}`}>
        <p {...stylex.props(sx.vtPreviewNoteP)}>{copy.effect}</p>
        <p className={`vt-hint ${stylex.props(sx.hint2).className}`}>{copy.whenTitle}: {copy.whenBody}</p>
      </div>
    </section>
  )
}

const sx = stylex.create({
  preview: { display: 'flex', flexDirection: 'column', rowGap: 'var(--vv-s-8)', columnGap: 'var(--vv-s-8)', paddingTop: 'var(--vv-s-12)', paddingRight: 'var(--vv-s-12)', paddingBottom: 'var(--vv-s-12)', paddingLeft: 'var(--vv-s-12)', borderTopWidth: 'var(--vv-s-1)', borderTopStyle: 'solid', borderTopColor: line['--line-2'], borderRightWidth: 'var(--vv-s-1)', borderRightStyle: 'solid', borderRightColor: line['--line-2'], borderBottomWidth: 'var(--vv-s-1)', borderBottomStyle: 'solid', borderBottomColor: line['--line-2'], borderLeftWidth: 'var(--vv-s-1)', borderLeftStyle: 'solid', borderLeftColor: line['--line-2'], borderTopLeftRadius: 'var(--vv-r-12)', borderTopRightRadius: 'var(--vv-r-12)', borderBottomRightRadius: 'var(--vv-r-12)', borderBottomLeftRadius: 'var(--vv-r-12)', backgroundColor: color['--color-surface-2'], backgroundImage: 'none' },
  previewHead: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', rowGap: 'var(--vv-s-8)', columnGap: 'var(--vv-s-8)' },
  vtPreviewHeadH2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: 'var(--vv-s-13)', fontWeight: libraryVars.headingWeight },
  badge: { paddingTop: 'var(--vv-s-2)', paddingRight: 'var(--vv-s-7)', paddingBottom: 'var(--vv-s-2)', paddingLeft: 'var(--vv-s-7)', borderTopLeftRadius: 'var(--vv-r-pill)', borderTopRightRadius: 'var(--vv-r-pill)', borderBottomRightRadius: 'var(--vv-r-pill)', borderBottomLeftRadius: 'var(--vv-r-pill)', backgroundColor: color['--color-surface-6'], backgroundImage: 'none', color: color['--color-fg-2'], fontSize: libraryVars.microFontSize },
  badgeDataStateFresh: { color: color['--color-ok'], backgroundColor: 'var(--vv-ok-10)', backgroundImage: 'none' },
  badgeDataStateStale: { color: color['--color-warn'], backgroundColor: 'var(--vv-warn-bg)', backgroundImage: 'none' },
  badgeDataStateBuilding: { color: color['--color-fg-2'], backgroundColor: 'var(--tint-accent)', backgroundImage: 'none' },
  previewFrame: { position: 'relative', aspectRatio: libraryVars.videoAspectRatio, borderTopLeftRadius: 'var(--vv-r-9)', borderTopRightRadius: 'var(--vv-r-9)', borderBottomRightRadius: 'var(--vv-r-9)', borderBottomLeftRadius: 'var(--vv-r-9)', overflowX: 'hidden', overflowY: 'hidden', backgroundColor: 'transparent', backgroundImage: libraryVars.promptsTabPcardArtBackgroundImage },
  previewArt: { position: 'absolute', top: libraryVars.templatePreviewPanePreviewArtTop, right: libraryVars.templatePreviewPanePreviewArtRight, bottom: libraryVars.templatePreviewPanePreviewArtTop, left: libraryVars.templatePreviewPanePreviewArtRight, borderTopLeftRadius: 'var(--vv-r-10)', borderTopRightRadius: 'var(--vv-r-10)', borderBottomRightRadius: 'var(--vv-r-10)', borderBottomLeftRadius: 'var(--vv-r-10)', backgroundColor: 'var(--vv-tile-idle)', backgroundImage: 'none', opacity: libraryVars.templatePreviewPanePreviewArtOpacity },
  previewOverlay: { position: 'absolute', top: 0, right: 0, bottom: 0, left: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', rowGap: 'var(--vv-s-3)', columnGap: 'var(--vv-s-3)', paddingTop: 'var(--vv-s-12)', paddingRight: 'var(--vv-s-12)', paddingBottom: 'var(--vv-s-12)', paddingLeft: 'var(--vv-s-12)', textAlign: 'center', backgroundColor: 'var(--vv-thumb-veil)', backgroundImage: 'none' },
  vtPreviewOverlayStrong: { fontSize: libraryVars.bodySmallFontSize, fontWeight: libraryVars.headingWeight },
  vtPreviewOverlaySpan: { fontSize: libraryVars.captionFontSize, color: color['--color-fg-3'] },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.captionFontSize, color: color['--color-fg-4'], lineHeight: libraryVars.bodyLineHeight },
  hint2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.captionFontSize, color: color['--color-fg-4'], lineHeight: libraryVars.bodyLineHeight },
  hint3: { fontSize: libraryVars.captionFontSize, color: color['--color-fg-4'] },
  previewActions: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', rowGap: 'var(--vv-s-8)', columnGap: 'var(--vv-s-8)' },
  vtPreviewActionsActionButton: { flexGrow: { default: 1, '@media (max-width: 767px) and (pointer: coarse)': 1 }, flexShrink: 1, flexBasis: 'auto', minHeight: { default: null, '@media (max-width: 767px) and (pointer: coarse)': 'var(--vv-s-44)' } },
  vtPreviewActionsIcon: { flexGrow: 0, flexShrink: 0, flexBasis: 'auto' },
  progress: { minWidth: 0, fontVariantNumeric: 'tabular-nums' },
  error: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: 'var(--vv-s-12)', color: color['--color-err'] },
  previewNote: { display: 'flex', flexDirection: 'column', rowGap: 'var(--vv-s-6)', columnGap: 'var(--vv-s-6)', fontSize: libraryVars.captionFontSize, color: color['--color-fg-3'] },
  vtPreviewNoteP: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0 },
  previewSource: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', rowGap: 'var(--vv-s-8)', columnGap: 'var(--vv-s-8)', flexWrap: 'wrap' },
  sourceLabel: { flexShrink: 0 },
  sourceField: { flexGrow: 1, flexShrink: 1, flexBasis: "auto", minWidth: 0 },
})
