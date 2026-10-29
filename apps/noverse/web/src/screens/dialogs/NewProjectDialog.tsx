// Dự án mới — Dialogs.dc.html › newProject: tên · mô tả · “Dùng chung trợ lý AI và quy tắc” + danh sách “Dự án mới có sẵn”.
// Không còn tên thư mục / ngôn ngữ / cây tệp: server tự sinh thư mục từ tên hiển thị; trùng tên → báo bằng lời thường.
import { useState } from 'react'
import { createProject } from '../../api/client'
import { useLibrary } from '../../api/library'
import { t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import { Button } from '@noverse/ui'
import { Dialog } from '@noverse/ui'
import { FieldError, FieldLabel, TextArea } from '@noverse/ui'
import { Checkbox } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { TextField } from '@noverse/ui'
import { keys, useAction, useConnection } from '../../data'
import { useSaveShortcut } from '@noverse/ui'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'

export const NEW_PROJECT_NAME_CLASH = 'Đã có dự án cùng tên — đặt tên khác nhé'

/** Trùng tên với dự án có sẵn: không phân biệt hoa thường và khoảng trắng đầu/cuối (giống server). */
export function isProjectNameTaken(name: string, projects: { name: string; legacy?: boolean }[]): boolean {
  const wanted = name.trim().toLowerCase()
  return wanted !== '' && projects.some((project) => !project.legacy && project.name.trim().toLowerCase() === wanted)
}

/** Danh sách “Dự án mới có sẵn” — Dialogs.dc.html npTree. `shared` = đang dùng chung với “Mọi truyện”. */
export const newProjectContents = (shared: boolean) => [
  { icon: 'lamp-on', name: 'Ý tưởng', note: 'trống', color: '#F6CB8C' },
  { icon: 'box', name: 'Tư liệu', note: 'bìa · nhạc · giọng', color: '#9DB0F7' },
  { icon: 'book-1', name: 'Truyện', note: 'chưa có', color: '#C9A6F5' },
  { icon: 'edit-2', name: 'Quy tắc viết', note: shared ? 'chép từ “Mọi truyện”' : 'trống', color: '#A6EDC1' },
  { icon: 'cpu', name: 'Trợ lý AI', note: shared ? 'theo “Mọi truyện”' : 'riêng dự án', color: '#A0A0AB' },
]

export function NewProjectDialog({ onClose }: { onClose: () => void }) {
  const connection = useConnection()
  const { library } = useLibrary()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [shared, setShared] = useState(true)

  const clash = isProjectNameTaken(name, library?.projects ?? [])
  const invalid = !name.trim() || name.length > 200 || clash

  const action = useAction(async () => {
    const created = await createProject({
      name: name.trim(), description: description.trim() || undefined, shareGlobal: shared,
    })
    navigate(paths.project(created.id), true)
    onClose()
    return created
  }, { success: 'Đã tạo dự án', invalidate: [keys.library()] })
  const submit = () => { if (!invalid && !action.busy && connection.writable) void action.run() }
  const disabledReason = !name.trim() ? 'Nhập tên dự án để tạo.' : name.length > 200 ? 'Tên dự án tối đa 200 ký tự.' : clash ? NEW_PROJECT_NAME_CLASH : (!connection.writable ? 'Chờ kết nối lại để tạo dự án.' : action.busy ? 'Đang tạo dự án…' : null)
  const error = action.error instanceof Error ? action.error.message : null
  useUnsavedGuard(!!(name.trim() || description.trim()) && !action.busy)
  useSaveShortcut(submit, !invalid && !action.busy)

  return (
    <Dialog title={t.dialogs.newProject} width={760} onClose={onClose}
      icon={<Icon name="folder-open" size={18} style={{ color: '#C9A6F5' }} />}
      footer={<>
        <Button onClick={onClose}>{t.dialogs.cancel}</Button>
        <Button variant="primary" onClick={submit} disabled={invalid || action.busy || !connection.writable} pending={action.pending} success={action.success} pendingLabel="Đang tạo…" tooltip={disabledReason ?? undefined} aria-describedby={clash ? 'np-clash' : undefined}>
          {t.dialogs.newProjectCreate}
        </Button>
      </>}>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 18, padding: 18 }}>
        <form style={{ flex: '1 1 320px', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 12 }}
          onSubmit={(event) => { event.preventDefault(); void submit() }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <FieldLabel htmlFor="np-title">Tên dự án</FieldLabel>
            <TextField id="np-title" autoFocus value={name} onChange={setName} aria-describedby={clash ? 'np-clash' : undefined}
              validate={(value) => value.length > 200 ? 'Tên dự án tối đa 200 ký tự' : undefined} placeholder="vd: Kênh truyện audio" />
          </div>
          {clash && (
            <span id="np-clash" role="alert" style={{ fontSize: 12, color: 'var(--color-err)', display: 'flex', alignItems: 'center', gap: 6 }}>
              <Icon name="warning-2" size={14} />{NEW_PROJECT_NAME_CLASH}
            </span>
          )}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <FieldLabel htmlFor="np-desc">Mô tả</FieldLabel>
            <TextArea id="np-desc" rows={2} value={description} onChange={(event) => setDescription(event.target.value)}
              placeholder="Kênh / series này nói về gì" />
          </div>
          <Checkbox checked={shared} onChange={(next) => setShared(next)} tone="mono"
            style={{ gap: 10, minHeight: 32, padding: 0, margin: 0, fontSize: 13, color: 'var(--color-fg-2)' }}
            label="Dùng chung trợ lý AI và quy tắc với “Mọi truyện”" />
          {error && <FieldError>{error}</FieldError>}
        </form>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <span className="lbl">Dự án mới có sẵn</span>
          {newProjectContents(shared).map((item) => (
            <div key={item.name} style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 13 }}>
              <Icon name={item.icon} size={16} style={{ color: item.color }} />
              <span style={{ flex: '1 1 auto' }}>{item.name}</span>
              <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{item.note}</span>
            </div>
          ))}
        </div>
      </div>
    </Dialog>
  )
}
