// Dự án mới — Dialogs.dc.html › newProject.
// Không còn tên thư mục / ngôn ngữ / cây tệp: server tự sinh thư mục từ tên hiển thị; trùng tên → báo bằng lời thường.
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit, line, members, surface } from '@noverse/ui/tokens.stylex'
import { createProject, getConfigPresets } from '../../api/client'
import { useLibrary } from '../../api/library'
import type { ConfigPresets } from '../../api/types'
import { t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import { Button } from '@noverse/ui'
import { Dialog } from '@noverse/ui'
import { FieldError, FieldLabel, TextArea } from '@noverse/ui'
import { Checkbox } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { TextField } from '@noverse/ui'
import { keys, useAction, useConnection, useResource } from '../../data'
import { useSaveShortcut } from '@noverse/ui'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'
import { styleDisplayName } from '../settings/sections/style'

const sx = stylex.create({
  body: { display: 'flex', flexWrap: 'wrap', gap: 18, padding: 18 },
  form: { flex: '1 1 320px', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 12 },
  field: { display: 'flex', flexDirection: 'column', gap: 6 },
  error: { fontSize: 12, color: color['--color-err'], display: 'flex', alignItems: 'center', gap: 6 },
  textarea: { height: 'auto', paddingTop: 8, paddingBottom: 8, paddingInline: 10, resize: 'none' },
  segment: { display: 'flex', padding: 3, borderRadius: 10, backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'] },
  segmentButton: { flex: '1 1 0', height: 30, borderWidth: 0, borderStyle: 'none', borderRadius: 7, backgroundColor: 'transparent', color: color['--color-fg-3'], fontSize: 13, whiteSpace: 'nowrap', fontWeight: 400 },
  segmentSelected: { backgroundColor: members['--members-seg-on'], color: color['--color-fg'], fontWeight: 500 },
  relative: { position: 'relative' },
  picker: { width: '100%', height: 'auto', minHeight: 44, display: 'flex', alignItems: 'center', gap: 10, padding: '7px 10px', textAlign: 'left' },
  pickerSelected: { minHeight: 50 },
  pickerText: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 },
  pickerTitle: { fontSize: 13.5, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  muted: { fontSize: 12, color: color['--color-fg-4'] },
  list: { position: 'absolute', left: 0, right: 0, top: 'calc(100% + 6px)', zIndex: 5, padding: 4, borderRadius: 12, backgroundColor: color['--color-surface-4'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-3'], boxShadow: surface['--dialog-shadow'] },
  option: { width: '100%', display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px', borderWidth: 0, borderStyle: 'none', borderRadius: 8, backgroundColor: 'transparent', color: color['--color-fg'], textAlign: 'left', font: 'inherit', opacity: 1, cursor: 'pointer' },
  optionSelected: { backgroundColor: line['--line-2'] },
  optionDisabled: { opacity: .5, cursor: 'not-allowed' },
  optionName: { fontSize: 13 },
  note: { fontSize: 12, color: color['--color-fg-4'], lineHeight: 1.5 },
  checkbox: { alignItems: 'flex-start', gap: 10, minHeight: 32, padding: '2px 0 0', margin: 0, fontSize: 13, color: color['--color-fg-2'] },
  checkboxText: { display: 'flex', flexDirection: 'column', gap: 2, lineHeight: 1.45 },
  summary: { display: 'flex', flexDirection: 'column', gap: 10 },
  summaryRow: { display: 'flex', alignItems: 'center', gap: 10, fontSize: 13 },
  summaryName: { flex: '1 1 auto' },
  accent: { color: color['--color-accent'] },
  info: { color: kit['--kit-info-fg'] },
  mutedIcon: { color: color['--color-fg-4'] },
  iconColor: (value: string) => ({ color: value }),
})

export const NEW_PROJECT_NAME_CLASH = 'Đã có dự án cùng tên — đặt tên khác nhé'

/** Trùng tên với dự án có sẵn: không phân biệt hoa thường và khoảng trắng đầu/cuối (giống server). */
export function isProjectNameTaken(name: string, projects: { name: string; legacy?: boolean }[]): boolean {
  const wanted = name.trim().toLowerCase()
  return wanted !== '' && projects.some((project) => !project.legacy && project.name.trim().toLowerCase() === wanted)
}

type CopySource = { id: string; ruleCount?: number; style?: string }
export const canCopyProjectRules = (project: { ruleCount?: number }) => (project.ruleCount ?? 0) > 0
const projectStyleName = (style?: string, presets?: ConfigPresets) => !style || style === 'default' ? 'Chung · mặc định' : styleDisplayName(presets, style)
export const projectCreationInput = (name: string, description: string, shareGlobal: boolean, source: CopySource | null) => ({
  name: name.trim(),
  ...(description.trim() ? { description: description.trim() } : {}),
  shareGlobal,
  ...(source && canCopyProjectRules(source) ? { copyRulesFrom: source.id } : {}),
})
export const newProjectContents = (shared: boolean, source: { ruleCount?: number; style?: string } | null, presets?: ConfigPresets) => [
  { icon: 'lamp-on', name: 'Ý tưởng', note: 'trống', color: color['--color-warn'] },
  { icon: 'box', name: 'Tư liệu', note: 'bìa · nhạc · giọng', color: kit['--kit-info-fg'] },
  { icon: 'book-1', name: 'Truyện', note: 'chưa có', color: color['--color-accent'] },
  { icon: 'edit-2', name: 'Quy tắc viết', note: source ? `chép ${source.ruleCount} quy tắc` : 'trống', color: color['--color-ok'] },
  { icon: 'brush-2', name: 'Văn phong', note: projectStyleName(source?.style, presets), color: color['--color-accent'] },
  { icon: 'cpu', name: 'Trợ lý AI', note: shared ? 'theo “Mọi truyện”' : 'bản riêng', color: color['--color-fg-3'] },
]

export function NewProjectDialog({ onClose }: { onClose: () => void }) {
  const connection = useConnection()
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
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
    <Dialog padBody={false} title={t.dialogs.newProject} width={760} onClose={onClose}
      icon={<Icon name="folder-open" size={18} xstyle={sx.accent} />}
      footer={<>
        <Button onClick={onClose}>{t.dialogs.cancel}</Button>
        <Button variant="primary" onClick={submit} disabled={invalid || action.busy || !connection.writable} pending={action.pending} success={action.success} pendingLabel="Đang tạo…" tooltip={disabledReason ?? undefined} aria-describedby={clash ? 'np-clash' : undefined}>
          {t.dialogs.newProjectCreate}
        </Button>
      </>}>
      <div {...stylex.props(sx.body)}>
        <form {...stylex.props(sx.form)}
          onSubmit={(event) => { event.preventDefault(); void submit() }}>
          <div {...stylex.props(sx.field)}>
            <FieldLabel htmlFor="np-title">Tên dự án</FieldLabel>
            <TextField id="np-title" autoFocus value={name} onChange={setName} aria-describedby={clash ? 'np-clash' : undefined}
              validate={(value) => value.length > 200 ? 'Tên dự án tối đa 200 ký tự' : undefined} placeholder="vd: Kênh truyện audio" />
          </div>
          {clash && (
            <span id="np-clash" role="alert" {...stylex.props(sx.error)}>
              <Icon name="warning-2" size={14} />{NEW_PROJECT_NAME_CLASH}
            </span>
          )}
          <div {...stylex.props(sx.field)}>
            <FieldLabel htmlFor="np-desc">Mô tả</FieldLabel>
            <TextArea id="np-desc" rows={2} showCount={false} value={description} onChange={(event) => setDescription(event.target.value)}
              xstyle={sx.textarea} placeholder="Kênh / series này nói về gì" />
          </div>
          <div {...stylex.props(sx.field)}>
            <span className="lbl" id="np-rules">Quy tắc &amp; văn phong ban đầu</span>
            <div role="radiogroup" aria-labelledby="np-rules" {...stylex.props(sx.segment)}>
              {(['empty', 'copy'] as const).map((value) => <button key={value} type="button" role="radio" aria-checked={mode === value}
                onClick={() => { setMode(value); setPickerOpen(value === 'copy' && !sourceId) }}
                {...stylex.props(sx.segmentButton, mode === value && sx.segmentSelected)}>
                {value === 'empty' ? 'Bắt đầu trống' : 'Chép từ dự án khác'}
              </button>)}
            </div>
            {mode === 'copy' && <div {...stylex.props(sx.relative)}>
              <button type="button" className={`field ${stylex.props(sx.picker, selected && sx.pickerSelected).className}`} aria-haspopup="listbox" aria-expanded={pickerOpen} aria-label="Chọn dự án để chép" onClick={() => setPickerOpen(!pickerOpen)}>
                <Icon name="folder-open" size={16} xstyle={sx.info} />
                <span {...stylex.props(sx.pickerText)}>
                  <span {...stylex.props(sx.pickerTitle)}>{selected?.name ?? 'Chọn dự án…'}</span>
                  {selected && <span {...stylex.props(sx.muted)}>{selected.ruleCount} quy tắc · {projectStyleName(selected.style, presets.data)}</span>}
                </span>
                <Icon name="arrow-down-2" size={14} xstyle={sx.mutedIcon} />
              </button>
              {pickerOpen && <div role="listbox" aria-label="Dự án" {...stylex.props(sx.list)}>
                {projects.map((project) => <button key={project.id} type="button" role="option" aria-selected={sourceId === project.id} disabled={!canCopyProjectRules(project)}
                  onClick={() => { setSourceId(project.id); setPickerOpen(false) }} {...stylex.props(sx.option, sourceId === project.id && sx.optionSelected, !canCopyProjectRules(project) && sx.optionDisabled)}>
                  <Icon name="folder-open" size={15} xstyle={sx.mutedIcon} />
                  <span {...stylex.props(sx.pickerText)}><span {...stylex.props(sx.optionName)}>{project.name}</span><span {...stylex.props(sx.muted)}>{canCopyProjectRules(project) ? `${project.ruleCount} quy tắc · ${projectStyleName(project.style, presets.data)}` : 'Chưa có quy tắc riêng — không có gì để chép'}</span></span>
                </button>)}
              </div>}
            </div>}
            <span {...stylex.props(sx.note)}>{mode === 'empty' ? 'Dự án mới chưa có quy tắc nào; văn phong dùng “Chung” của Noverse. Thêm sau ở trang dự án › Cách viết.' : selected ? `Chép một lần quy tắc và văn phong của “${selected.name}”. Sửa ở dự án mới không ảnh hưởng dự án gốc.` : 'Chọn dự án có quy tắc để chép.'}</span>
          </div>
          <Checkbox checked={shared} onChange={(next) => setShared(next)}
            xstyle={sx.checkbox}
            label={<span {...stylex.props(sx.checkboxText)}><span>Dùng chung trợ lý AI và ngân sách với “Mọi truyện”</span><span {...stylex.props(sx.muted)}>Model, mức suy nghĩ, ngân sách mỗi truyện. Tắt thì dự án giữ một bản riêng, đổi ở Tuỳ chỉnh.</span></span>} />
          {error && <FieldError>{error}</FieldError>}
        </form>

        <div {...stylex.props(sx.summary)}>
          <span className="lbl">Dự án mới có sẵn</span>
          {newProjectContents(shared, selected, presets.data).map((item) => (
            <div key={item.name} {...stylex.props(sx.summaryRow)}>
              <Icon name={item.icon} size={16} xstyle={sx.iconColor(item.color)} />
              <span {...stylex.props(sx.summaryName)}>{item.name}</span>
              <span {...stylex.props(sx.muted)}>{item.note}</span>
            </div>
          ))}
        </div>
      </div>
    </Dialog>
  )
}
