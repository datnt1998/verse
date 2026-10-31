import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
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
    <section className={`vt-preview ${stylex.props(sx.preview).className}`} aria-labelledby="vt-preview-title">
      <div className={`vt-preview-head ${stylex.props(sx.previewHead).className}`}>
        <h2 {...stylex.props(sx.vtPreviewHeadH2)} id="vt-preview-title">{copy.title}</h2>
        <span className={`vt-badge ${stylex.props(sx.badge, ((state) === "fresh") && sx.badgeDataStateFresh, ((state) === "stale") && sx.badgeDataStateStale).className}`} data-state={state}>
          {copy.badge[state]}
        </span>
      </div>
      <div className={`vt-preview-frame ${stylex.props(sx.previewFrame).className}`} data-state={state}>
        <span className={`vt-preview-art ${stylex.props(sx.previewArt).className}`} aria-hidden="true" />
        {state === 'stale' && (
          <div className={`vt-preview-overlay ${stylex.props(sx.previewOverlay).className}`}>
            <strong {...stylex.props(sx.vtPreviewOverlayStrong)}>{copy.staleTitle}</strong>
            <span {...stylex.props(sx.vtPreviewOverlaySpan)}>{copy.staleBody}</span>
          </div>
        )}
        {state === 'none' && (
          <div className={`vt-preview-overlay ${stylex.props(sx.previewOverlay).className}`}>
            <strong {...stylex.props(sx.vtPreviewOverlayStrong)}>{copy.noneTitle}</strong>
            <span {...stylex.props(sx.vtPreviewOverlaySpan)}>{copy.noneBody}</span>
          </div>
        )}
      </div>
      <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{builtAt ? copy.builtAt(builtAt) : copy.never}</p>
      <div className={`vt-preview-actions ${stylex.props(sx.previewActions).className}`}>
        {state === 'stale'
          ? <ActionButton xstyle={[sx.vtPreviewActionsActionButton]} variant="primary" icon={<Icon xstyle={[sx.vtPreviewActionsIcon]} name="play" size={16} />} lockedReason={copy.buildLocked}>{copy.rebuildStale}</ActionButton>
          : <ActionButton xstyle={[sx.vtPreviewActionsActionButton]} variant="outline" icon={<Icon xstyle={[sx.vtPreviewActionsIcon]} name="play" size={16} />} lockedReason={copy.buildLocked}>{copy.rebuild}</ActionButton>}
      </div>
      <div className={`vt-preview-note ${stylex.props(sx.previewNote).className}`}>
        <p {...stylex.props(sx.vtPreviewNoteP)}>{copy.effect}</p>
        <p className={`vt-hint ${stylex.props(sx.hint2).className}`}>{copy.whenTitle}: {copy.whenBody}</p>
        <div className={`vt-preview-source ${stylex.props(sx.previewSource).className}`}>
          <span className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.videoTitle}</span>
          <ActionButton xstyle={[sx.vtPreviewSourceActionButton]} variant="quiet" lockedReason={copy.noVideo}>{copy.noVideo}</ActionButton>
        </div>
      </div>
    </section>
  )
}

const sx = stylex.create({
  preview: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", paddingTop: "var(--vv-s-12)", paddingRight: "var(--vv-s-12)", paddingBottom: "var(--vv-s-12)", paddingLeft: "var(--vv-s-12)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", backgroundColor: color["--color-card"], backgroundImage: "none" },
  previewHead: { display: "flex", alignItems: "center", justifyContent: "space-between", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  vtPreviewHeadH2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-13)", fontWeight: libraryVars.headingWeight },
  badge: { paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", backgroundColor: color["--color-surface-6"], backgroundImage: "none", color: color["--color-fg-2"], fontSize: libraryVars.microFontSize },
  badgeDataStateFresh: { color: color["--color-ok"], backgroundColor: "var(--vv-ok-10)", backgroundImage: "none" },
  badgeDataStateStale: { color: color["--color-warn"], backgroundColor: "var(--vv-warn-bg)", backgroundImage: "none" },
  previewFrame: { position: "relative", aspectRatio: libraryVars.videoAspectRatio, borderTopLeftRadius: "var(--vv-r-9)", borderTopRightRadius: "var(--vv-r-9)", borderBottomRightRadius: "var(--vv-r-9)", borderBottomLeftRadius: "var(--vv-r-9)", overflowX: "hidden", overflowY: "hidden", backgroundColor: "transparent", backgroundImage: libraryVars.promptsTabPcardArtBackgroundImage },
  previewArt: { position: "absolute", top: libraryVars.templatePreviewPanePreviewArtTop, right: libraryVars.templatePreviewPanePreviewArtRight, bottom: libraryVars.templatePreviewPanePreviewArtTop, left: libraryVars.templatePreviewPanePreviewArtRight, borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", backgroundColor: "var(--vv-tile-idle)", backgroundImage: "none", opacity: libraryVars.templatePreviewPanePreviewArtOpacity },
  previewOverlay: { position: "absolute", top: 0, right: 0, bottom: 0, left: 0, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", rowGap: "var(--vv-s-3)", columnGap: "var(--vv-s-3)", paddingTop: "var(--vv-s-12)", paddingRight: "var(--vv-s-12)", paddingBottom: "var(--vv-s-12)", paddingLeft: "var(--vv-s-12)", textAlign: "center", backgroundColor: "var(--vv-thumb-veil)", backgroundImage: "none" },
  vtPreviewOverlayStrong: { fontSize: libraryVars.bodySmallFontSize, fontWeight: libraryVars.headingWeight },
  vtPreviewOverlaySpan: { fontSize: libraryVars.captionFontSize, color: color["--color-fg-3"] },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.captionFontSize, color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  previewActions: { display: "flex" },
  vtPreviewActionsActionButton: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  vtPreviewActionsIcon: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto" },
  previewNote: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", fontSize: libraryVars.captionFontSize, color: color["--color-fg-3"] },
  vtPreviewNoteP: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0 },
  hint2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.captionFontSize, color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  previewSource: { display: "flex", alignItems: "center", justifyContent: "space-between", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  vtPreviewSourceActionButton: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
})
