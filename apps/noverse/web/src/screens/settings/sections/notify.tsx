// Tuỳ chỉnh › Báo động máy chủ — Settings.dc.html sec notify: một công tắc cho mỗi sự kiện, ghi `notify.events` (cấp toàn cục).
// Thông báo trên điện thoại của từng người (NotifySettings) chỉnh ở ứng dụng điện thoại, không nằm ở đây.
import { Page, Sw, SRow, SecCard, SectionHead } from '../parts'
import type { SettingsContext } from '../types'
import { NOTIFY_ROWS, notifyStates, toggleNotify } from './notifyLogic'

export function NotifySection({ ctx }: { ctx: SettingsContext }) {
  const states = notifyStates(ctx.draft.notify)
  const toggle = (kind: (typeof NOTIFY_ROWS)[number]['kind']) => {
    const patch = toggleNotify(ctx.draft.notify, kind, !states[kind])
    ctx.setField('notify.enabled', patch.enabled)
    ctx.setField('notify.events', patch.events)
  }
  return (
    <Page top={4}>
      <SectionHead title="Báo động khi không có ai trông"
        note="Máy chủ tự báo khi truyện dừng hoặc cần người. Thông báo trên điện thoại của từng người chỉnh ở ứng dụng điện thoại." />
      <SecCard>
        {NOTIFY_ROWS.map((row) => (
          <SRow key={row.kind} center label={row.label}>
            <Sw on={states[row.kind]} label={row.label} disabled={!ctx.canEdit} onToggle={() => toggle(row.kind)} />
          </SRow>
        ))}
      </SecCard>
    </Page>
  )
}
