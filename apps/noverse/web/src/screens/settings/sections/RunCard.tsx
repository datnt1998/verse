// Cách chạy sits below the assistants; review remains live for an existing story.
import { setReview } from '../../../api/client'
import { cache, keys, useAction } from '../../../data'
import { OvrBadge, Sw } from '../parts'
import type { SettingsContext } from '../types'

export function RunCard({ ctx }: { ctx: SettingsContext }) {
  const story = ctx.scope === 'story'
  const advance = ctx.draft.advance_mode ?? ctx.effective.advance_mode ?? 'auto'
  const reviewOn = (story && ctx.snapshot ? ctx.snapshot.advanceMode : advance) === 'review'
  const askUser = ctx.draft.ask_user ?? ctx.effective.ask_user ?? false
  const reviewLabel = story ? 'Duyệt từng chương' : 'Duyệt từng chương (truyện mới)'
  const askLabel = 'Trọng tài hỏi bạn khi phân vân'
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
  return <section className="card st-sec" aria-label="Cách chạy">
    <b style={{ fontSize: 14, fontWeight: 500 }}>Cách chạy</b>
    <div className="st-srow">
      <div className="st-stx"><b>{reviewLabel}</b><span>{story
        ? 'Dừng sau mỗi chương để bạn đọc và bấm Duyệt. Đổi ngay, không cần đợi.'
        : 'Truyện mới tạo sẽ dừng sau mỗi chương. Truyện đang viết giữ cách hiện tại.'}</span></div>
      <OvrBadge ovr={ctx.ovr(['advance_mode'])} scope={ctx.scope} />
      <Sw on={reviewOn} label={reviewLabel} disabled={!ctx.canEdit || live.busy} onToggle={toggleReview} />
    </div>
    <div className="st-srow">
      <div className="st-stx"><b>{askLabel}</b><span>Gặp hai hướng đi lớn (vd. chương dài hơn dàn ý), Trọng tài dừng lại hỏi bạn. Tắt thì Trọng tài tự chọn hướng bám dàn ý và ghi lý do vào nhật ký.</span></div>
      <OvrBadge ovr={ctx.ovr(['ask_user'])} scope={ctx.scope} disabled={!ctx.canEdit} onReset={() => ctx.resetField('ask_user')} />
      <Sw on={askUser} label={askLabel} disabled={!ctx.canEdit} onToggle={() => ctx.setField('ask_user', !askUser)} />
    </div>
  </section>
}
