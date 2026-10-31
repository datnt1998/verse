// Tuỳ chỉnh › Cách viết — Settings.dc.html #writing + 20-settings-rules.md §2.2 + D2 (gói thể loại) + Gói AL.
// Gói AL (29/10): “Văn phong” chỉ còn sửa ở cấp “Mọi truyện” (“Văn phong chung”, ô “Văn phong của
// bạn” mờ — văn mẫu thêm theo truyện ở Dự án & truyện › Cách viết); cấp truyện/dự án thay lưới +
// văn mẫu bằng thẻ chỉ dẫn mở mục mới. Duyệt từng chương và Trọng tài hỏi giữ nguyên 3 cấp.
// Duyệt từng chương: cấp truyện là lệnh live `/review on|off` (POST …/review), cấp dự án / mọi truyện ghi
// `advance_mode` cho truyện mới. “Trọng tài hỏi bạn khi phân vân” (27/10): `ask_user` ở cả 3 cấp,
// tắt → máy chủ tự chọn hướng bám dàn ý và ghi vào nhật ký.
import { setReview } from '../../../api/client'
import { cache, keys, useAction } from '../../../data'
import { paths } from '../../../router'
import { findPack } from '../lib/packs'
import { Icon } from '@noverse/ui'
import { OvrBadge, Page, SectionHead, Sw } from '../parts'
import type { SettingsContext } from '../types'
import { PackDetailsCard, styleDisplayName, styleSourceText, StylePicker } from './style'
import '../style.css'

export function WritingSection({ ctx }: { ctx: SettingsContext }) {
  const style = ctx.draft.style ?? ctx.effective.style ?? 'default'
  const pack = findPack(ctx.presets?.packs, style)
  const story = ctx.scope === 'story'
  const globalScope = ctx.scope === 'global'
  // 27/10 — `ask_user` 3 cấp như advance_mode: truyện ghi đè dự án ghi đè toàn cục, mặc định tắt.
  const askUser = ctx.draft.ask_user ?? ctx.effective.ask_user ?? false
  const askLabel = 'Trọng tài hỏi bạn khi phân vân'

  // Cấp truyện: chế độ thật của truyện nằm ở ảnh chụp (đổi ngay bằng lệnh live); các cấp khác: `advance_mode` trong nháp.
  const advance = ctx.draft.advance_mode ?? ctx.effective.advance_mode ?? 'auto'
  const reviewOn = (story && ctx.snapshot ? ctx.snapshot.advanceMode : advance) === 'review'

  const snapshotKey = keys.story(ctx.project, ctx.story, 'snapshot')
  const live = useAction((on: boolean) => setReview(ctx.project, ctx.story, on), {
    success: 'Đã đổi chế độ nghiệm thu', invalidate: [snapshotKey],
    optimistic: (on) => {
      const previous = cache.setData(snapshotKey, ctx.snapshot ? { ...ctx.snapshot, advanceMode: on ? 'review' : 'auto' } : ctx.snapshot)
      return () => cache.restore(snapshotKey, previous)
    },
  })
  const toggleReview = () => {
    if (story) { if (!live.busy) void live.run(!reviewOn); return }
    ctx.setField('advance_mode', reviewOn ? 'auto' : 'review')
  }
  const reviewLabel = story ? 'Duyệt từng chương' : 'Duyệt từng chương (truyện mới)'

  // Gói AL — thẻ chỉ dẫn ở cấp truyện/dự án (artboard `stylePtr`): tên văn phong + nguồn + nút mở.
  const styleName = styleDisplayName(ctx.presets, style)
  const styleSource = styleSourceText(ctx.response?.sources?.style, ctx.scope)
  const guideHref = story
    ? `#${paths.story(ctx.project, ctx.story)}?tab=style`
    : `#${paths.project(ctx.project)}?section=pstyle`

  return (
    <Page>
      <SectionHead title="Cách viết" />
      {!globalScope && (
        <section className="card st-sec" aria-label="Văn phong" style={{ flexDirection: 'row', alignItems: 'center', gap: 14 }}>
          <Icon name="brush-2" size={20} style={{ color: 'var(--color-accent)', flex: '0 0 auto' }} />
          <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 3, lineHeight: 1.45 }}>
            <b style={{ fontSize: 14, fontWeight: 500 }}>{story ? 'Văn phong của truyện này' : 'Văn phong mặc định của dự án'}</b>
            <span style={{ fontSize: 13, color: 'var(--color-fg-2)' }}>
              {styleName}{styleSource ? ` · ${styleSource}` : ''}
            </span>
            <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>
              Mở truyện › tab Cách viết, hoặc trang dự án › Cách viết của dự án
            </span>
          </span>
          <a href={guideHref} style={{
            height: 34, padding: '0 14px', borderRadius: 9, border: '1px solid var(--kit-line-12)',
            color: 'var(--color-fg)', fontSize: 13, display: 'flex', alignItems: 'center', gap: 6,
            whiteSpace: 'nowrap', flex: '0 0 auto',
          }}>
            {story ? 'Mở Cách viết của truyện' : 'Mở Cách viết của dự án'}<Icon name="arrow-right-2" size={14} />
          </a>
        </section>
      )}
      {globalScope && (
        <>
          <section className="card st-sec">
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <b style={{ fontSize: 14, fontWeight: 500, flex: '1 1 auto' }}>
                Văn phong chung <span style={{ fontWeight: 400, color: 'var(--kit-fg-icon)', fontSize: 12.5 }}>· cho truyện mới và truyện chưa chọn riêng</span>
              </b>
              <OvrBadge ovr={ctx.ovr(['style'])} scope={ctx.scope} disabled={!ctx.canEdit} onReset={() => ctx.resetField('style')} />
            </div>
            <StylePicker value={style} presets={ctx.presets} custom="blocked"
              disabled={!ctx.canEdit}
              onPick={(id) => ctx.setField('style', id)} />
          </section>
          {pack && <PackDetailsCard pack={pack} audioHref={ctx.story ? `#${paths.audio(ctx.project, ctx.story)}` : undefined} />}
          <p style={{ margin: '-4px 4px 0', fontSize: 12.5, color: 'var(--kit-fg-icon)', lineHeight: 1.5 }}>
            Truyện hoặc dự án muốn văn phong khác thì chọn ở Dự án &amp; truyện › Cách viết. Văn mẫu (“Văn phong của bạn”) thêm theo từng truyện ở đó.
          </p>
        </>
      )}
      <section className="card st-sec">
        <div className="st-srow">
          <div className="st-stx">
            <b>{reviewLabel}</b>
            <span>{story
              ? 'Dừng sau mỗi chương để bạn đọc và bấm Duyệt. Đổi ngay, không cần đợi.'
              : 'Truyện mới tạo sẽ dừng sau mỗi chương. Truyện đang viết giữ cách hiện tại.'}</span>
          </div>
          <OvrBadge ovr={ctx.ovr(['advance_mode'])} scope={ctx.scope} />
          <Sw on={reviewOn} label={reviewLabel} disabled={!ctx.canEdit || live.busy} onToggle={toggleReview} />
        </div>
        <div className="st-srow">
          <div className="st-stx">
            <b>{askLabel}</b>
            <span>Gặp hai hướng đi lớn (vd. chương dài hơn dàn ý), Trọng tài dừng lại hỏi bạn. Tắt thì Trọng tài tự chọn hướng bám dàn ý và ghi lý do vào nhật ký.</span>
          </div>
          <OvrBadge ovr={ctx.ovr(['ask_user'])} scope={ctx.scope} disabled={!ctx.canEdit} onReset={() => ctx.resetField('ask_user')} />
          <Sw on={askUser} label={askLabel} disabled={!ctx.canEdit} onToggle={() => ctx.setField('ask_user', !askUser)} />
        </div>
      </section>
    </Page>
  )
}
