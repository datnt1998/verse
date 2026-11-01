// Viết tiếp tập mới — Dialogs.dc.html › reopen: hướng viết tiếp → POST /reopen.
import { useState } from 'react'
import { getStoryMeta, reopenStory } from '../../api/client'
import { t } from '../../i18n/vi'
import { storyLabel } from '../../shell/labels'
import { Button } from '@noverse/ui'
import { Dialog } from '@noverse/ui'
import { FieldError, FieldLabel, TextArea } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { keys, useAction, useConnection, useResource } from '../../data'
import { refreshProjectData } from '../project/useProject'
import { useSaveShortcut } from '@noverse/ui'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'

export function ReopenDialog({ project, story, onClose }: { project: string; story: string; onClose: () => void }) {
  const connection = useConnection()
  const resource = useResource(keys.story(project, story, 'meta'), ({ signal }) => getStoryMeta(project, story, signal))
  const title = storyLabel({ story, title: resource.data?.title })
  const [direction, setDirection] = useState('')
  const action = useAction(async () => {
    const result = await reopenStory(project, story, direction.trim())
    refreshProjectData()
    onClose()
    return result
  }, { success: 'Đã mở lại truyện để viết tiếp', invalidate: [keys.story(project, story), keys.project(project)] })
  const error = action.error instanceof Error ? action.error.message : null
  useUnsavedGuard(!!direction.trim() && !action.busy)
  useSaveShortcut(() => action.run(), !action.busy && connection.writable)

  return (
    <Dialog padBody={false} title={t.dialogs.reopen} meta="/reopen" width={560} onClose={onClose}
      icon={<Icon name="edit-2" size={18} style={{ color: 'var(--color-accent)' }} />}
      footer={<>
        <Button onClick={onClose}>{t.dialogs.cancel}</Button>
        <Button variant="primary" onClick={() => void action.run()} disabled={action.busy || !connection.writable} pending={action.pending} success={action.success}>
          {t.dialogs.reopenSubmit}
        </Button>
      </>}>
      <div style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}>
        <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-3)' }}>
          Truyện “{title}” đã hoàn thành. Mở lại sẽ đưa truyện về trạng thái sáng tác; hướng viết tiếp được
          Trọng tài xử lý như một can thiệp.
        </p>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <FieldLabel htmlFor="reopen-dir">Hướng viết tiếp · tùy chọn</FieldLabel>
          <TextArea id="reopen-dir" rows={4} autoFocus value={direction}
            onChange={(event) => setDirection(event.target.value)}
            placeholder="vd: Tập 2 xoay quanh phiên tòa tác quyền, nhân vật phản diện lộ diện…"
            style={{ padding: 10, fontSize: 14 }} />
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: 'var(--color-fg-4)' }}>
          <Icon name="folder-open" size={14} />
          Các bản xuất đã có trong thư mục xuất bản được giữ nguyên
        </div>
        {error && <FieldError>{error}</FieldError>}
      </div>
    </Dialog>
  )
}
