// Dự án mới — Dialogs.dc.html › newProject.
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
import { styleDisplayName } from '../settings/sections/style'

export const NEW_PROJECT_NAME_CLASH = 'Đã có dự án cùng tên — đặt tên khác nhé'

/** Trùng tên với dự án có sẵn: không phân biệt hoa thường và khoảng trắng đầu/cuối (giống server). */
export function isProjectNameTaken(name: string, projects: { name: string; legacy?: boolean }[]): boolean {
  const wanted = name.trim().toLowerCase()
  return wanted !== '' && projects.some((project) => !project.legacy && project.name.trim().toLowerCase() === wanted)
}

type CopySource = { id: string; ruleCount?: number; style?: string }
export const canCopyProjectRules = (project: { ruleCount?: number }) => (project.ruleCount ?? 0) > 0
const projectStyleName = (style?: string) => !style || style === 'default' ? 'Chung · mặc định' : styleDisplayName(null, style)
export const projectCreationInput = (name: string, description: string, shareGlobal: boolean, source: CopySource | null) => ({
  name: name.trim(),
  ...(description.trim() ? { description: description.trim() } : {}),
  shareGlobal,
  ...(source && canCopyProjectRules(source) ? { copyRulesFrom: source.id } : {}),
})
export const newProjectContents = (shared: boolean, source: { ruleCount?: number; style?: string } | null) => [
  { icon: 'lamp-on', name: 'Ý tưởng', note: 'trống', color: 'var(--color-warn)' },
  { icon: 'box', name: 'Tư liệu', note: 'bìa · nhạc · giọng', color: 'var(--color-info)' },
  { icon: 'book-1', name: 'Truyện', note: 'chưa có', color: 'var(--color-accent)' },
  { icon: 'edit-2', name: 'Quy tắc viết', note: source ? `chép ${source.ruleCount} quy tắc` : 'trống', color: 'var(--color-ok)' },
  { icon: 'brush-2', name: 'Văn phong', note: projectStyleName(source?.style), color: 'var(--color-accent)' },
  { icon: 'cpu', name: 'Trợ lý AI', note: shared ? 'theo “Mọi truyện”' : 'bản riêng', color: 'var(--color-fg-3)' },
]

export function NewProjectDialog({ onClose }: { onClose: () => void }) {
  const connection = useConnection()
  const { library } = useLibrary()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [shared, setShared] = useState(true)
  const [mode, setMode] = useState<'empty' | 'copy'>('empty')
  const [sourceId, setSourceId] = useState<string | null>(null)
  const [pickerOpen, setPickerOpen] = useState(false)
  const projects = library?.projects ?? []
  const selected = mode === 'copy' ? projects.find((project) => project.id === sourceId && canCopyProjectRules(project)) ?? null : null

  const clash = isProjectNameTaken(name, library?.projects ?? [])
  const invalid = !name.trim() || name.length > 200 || clash || (mode === 'copy' && !selected)

  const action = useAction(async () => {
    const created = await createProject(projectCreationInput(name, description, shared, selected))
    navigate(paths.project(created.id), true)
    onClose()
    return created
  }, { success: 'Đã tạo dự án', invalidate: [keys.library()] })
  const submit = () => { if (!invalid && !action.busy && connection.writable) void action.run() }
  const disabledReason = !name.trim() ? 'Nhập tên dự án để tạo.' : name.length > 200 ? 'Tên dự án tối đa 200 ký tự.' : clash ? NEW_PROJECT_NAME_CLASH : mode === 'copy' && !selected ? 'Chọn dự án có quy tắc để chép.' : (!connection.writable ? 'Chờ kết nối lại để tạo dự án.' : action.busy ? 'Đang tạo dự án…' : null)
  const error = action.error instanceof Error ? action.error.message : null
  useUnsavedGuard(!!(name.trim() || description.trim()) && !action.busy)
  useSaveShortcut(submit, !invalid && !action.busy)

  return (
    <Dialog title={t.dialogs.newProject} width={760} onClose={onClose}
      icon={<Icon name="folder-open" size={18} style={{ color: 'var(--color-accent)' }} />}
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
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <span className="lbl" id="np-rules">Quy tắc &amp; văn phong ban đầu</span>
            <div role="radiogroup" aria-labelledby="np-rules" style={{ display: 'flex', padding: 3, borderRadius: 10, background: 'var(--color-surface-1)', border: '1px solid var(--line-2)' }}>
              {(['empty', 'copy'] as const).map((value) => <button key={value} type="button" role="radio" aria-checked={mode === value}
                onClick={() => { setMode(value); setPickerOpen(value === 'copy' && !sourceId) }}
                style={{ flex: '1 1 0', height: 30, border: 0, borderRadius: 7, background: mode === value ? 'var(--color-surface-5)' : 'transparent', color: mode === value ? 'var(--color-fg)' : 'var(--color-fg-3)', fontSize: 13, whiteSpace: 'nowrap', fontWeight: mode === value ? 500 : 400 }}>
                {value === 'empty' ? 'Bắt đầu trống' : 'Chép từ dự án khác'}
              </button>)}
            </div>
            {mode === 'copy' && <div style={{ position: 'relative' }}>
              <button type="button" className="field" aria-haspopup="listbox" aria-expanded={pickerOpen} aria-label="Chọn dự án để chép" onClick={() => setPickerOpen(!pickerOpen)}
                style={{ width: '100%', height: 'auto', minHeight: 44, display: 'flex', alignItems: 'center', gap: 10, padding: '7px 10px', textAlign: 'left' }}>
                <Icon name="folder-open" size={16} style={{ color: 'var(--color-info)' }} />
                <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 }}>
                  <span style={{ fontSize: 13.5, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{selected?.name ?? 'Chọn dự án…'}</span>
                  {selected && <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{selected.ruleCount} quy tắc · {projectStyleName(selected.style)}</span>}
                </span>
                <Icon name="arrow-down-2" size={14} style={{ color: 'var(--color-fg-4)' }} />
              </button>
              {pickerOpen && <div role="listbox" aria-label="Dự án" style={{ position: 'absolute', left: 0, right: 0, top: 'calc(100% + 6px)', zIndex: 5, padding: 4, borderRadius: 12, background: 'var(--color-surface-4)', border: '1px solid var(--line-3)', boxShadow: 'var(--dialog-shadow)' }}>
                {projects.map((project) => <button key={project.id} type="button" role="option" aria-selected={sourceId === project.id} disabled={!canCopyProjectRules(project)}
                  onClick={() => { setSourceId(project.id); setPickerOpen(false) }} style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px', border: 0, borderRadius: 8, background: sourceId === project.id ? 'var(--line-2)' : 'transparent', color: 'var(--color-fg)', textAlign: 'left', font: 'inherit', opacity: canCopyProjectRules(project) ? 1 : 0.5, cursor: canCopyProjectRules(project) ? 'pointer' : 'not-allowed' }}>
                  <Icon name="folder-open" size={15} style={{ color: 'var(--color-fg-4)' }} />
                  <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 }}><span style={{ fontSize: 13 }}>{project.name}</span><span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{canCopyProjectRules(project) ? `${project.ruleCount} quy tắc · ${projectStyleName(project.style)}` : 'Chưa có quy tắc riêng — không có gì để chép'}</span></span>
                </button>)}
              </div>}
            </div>}
            <span style={{ fontSize: 12, color: 'var(--color-fg-4)', lineHeight: 1.5 }}>{mode === 'empty' ? 'Dự án mới chưa có quy tắc nào; văn phong dùng “Chung” của Noverse. Thêm sau ở trang dự án › Cách viết.' : selected ? `Chép một lần quy tắc và văn phong của “${selected.name}”. Sửa ở dự án mới không ảnh hưởng dự án gốc.` : 'Chọn dự án có quy tắc để chép.'}</span>
          </div>
          <Checkbox checked={shared} onChange={(next) => setShared(next)}
            style={{ alignItems: 'flex-start', gap: 10, minHeight: 32, padding: '2px 0 0', margin: 0, fontSize: 13, color: 'var(--color-fg-2)' }}
            label={<span style={{ display: 'flex', flexDirection: 'column', gap: 2, lineHeight: 1.45 }}><span>Dùng chung trợ lý AI và ngân sách với “Mọi truyện”</span><span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>Model, mức suy nghĩ, ngân sách mỗi truyện. Tắt thì dự án giữ một bản riêng, đổi ở Tuỳ chỉnh.</span></span>} />
          {error && <FieldError>{error}</FieldError>}
        </form>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <span className="lbl">Dự án mới có sẵn</span>
          {newProjectContents(shared, selected).map((item) => (
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
