// Tuỳ chỉnh › Thông tin dự án — Settings.dc.html #info. Ghi vào project.json.
// (PUT kèm If-Match); Lưu/Huỷ nằm ở thanh nổi đáy của Tuỳ chỉnh (ctx.registerExtra).
import { useEffect, useRef, useState } from 'react'
import { ApiError, ConflictError, getProject, updateProject } from '../../../api/client'
import type { ProjectDetail } from '../../../api/types'
import { invalidate, keys, setData, useConnection, useResource, type Key, type Resource } from '../../../data'
import { ConflictBar } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { ErrorCard, LoadingRows } from '../components'
import { Page, SectionHead } from '../parts'
import type { SettingsContext } from '../types'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

// ─── Dữ liệu biểu mẫu (thuần, có test) ──────────────────────────────────────────────────────────

export type ProjectForm = { name: string; description: string; notes: string; universeName: string }

export const projectFormOf = (project: { name: string; description: string; notes: string; universeName?: string }): ProjectForm => ({
  name: project.name, description: project.description, notes: project.notes, universeName: project.universeName ?? '',
})
export const formDirty = (form: object | null, base: object | null): boolean => !!form && !!base && JSON.stringify(form) !== JSON.stringify(base)

/**
 * Thứ tự + nhãn các ô “Thông tin dự án” (Settings.dc.html › `infoFields`, scope=project): tên vũ trụ đứng
 * sau “Mô tả” và trước “Ghi chú cho trợ lý”; ô trống nghĩa là Não dự án dùng tên dự án.
 */
export type ProjectField = {
  key: keyof ProjectForm
  label: string
  kind: 'text' | 'area'
  placeholder?: string
}
export const PROJECT_FIELDS: ProjectField[] = [
  { key: 'name', label: 'Tên dự án', kind: 'text' },
  { key: 'description', label: 'Mô tả', kind: 'text' },
  { key: 'universeName', label: 'Tên vũ trụ · hiện ở Não dự án', kind: 'text', placeholder: 'Để trống thì dùng tên dự án' },
  { key: 'notes', label: 'Ghi chú cho trợ lý', kind: 'area' },
]

/** Số thay đổi (một ô đổi = một thay đổi) cho câu “N thay đổi cho …” ở thanh nổi. */
export function changedFields(form: Record<string, unknown> | null, base: Record<string, unknown> | null): number {
  if (!form || !base) return 0
  return Object.keys(form).filter((key) => JSON.stringify(form[key]) !== JSON.stringify(base[key])).length
}

const SPAN_ALL = { gridColumn: '1 / -1' } as const

function Label({ text, span, children }: { text: string; span?: boolean; children: React.ReactNode }) {
  return (
    <label style={{ display: 'flex', flexDirection: 'column', gap: 6, ...(span ? SPAN_ALL : null) }}>
      <span style={{ fontSize: 12.5, color: '#A0A0AB' }}>{text}</span>
      {children}
    </label>
  )
}

const text38 = { height: 38, fontSize: 14 } as const
const area = { height: 'auto', padding: 10, fontSize: 14, resize: 'vertical' } as const


/** Khung chung: tải tài liệu, giữ bản nháp, đăng ký việc với thanh nổi, xử lý 409. */
function useInfoDoc<Doc extends { version?: string }, Form extends Record<string, unknown>>({ ctx, slot, load, resourceKey, formOf, save }: {
  ctx: SettingsContext
  slot: string
  load: (signal: AbortSignal) => Promise<Doc>
  resourceKey: Key
  formOf: (doc: Doc) => Form
  save: (form: Form, version: string | undefined, doc: Doc) => Promise<Doc>
}) {
  const connection = useConnection()
  const resource = useResource(resourceKey, ({ signal }) => load(signal), { enabled: !!ctx.project })
  const [doc, setDoc] = useState<Doc | null>(null)
  const [form, setForm] = useState<Form | null>(null)
  const [conflict, setConflict] = useState<ConflictError<Doc> | null>(null)
  const base = doc ? formOf(doc) : null
  const dirty = formDirty(form, base)

  useEffect(() => {
    if (dirty || !resource.data) return
    setDoc(resource.data); setForm(formOf(resource.data))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [resource.data])

  const latest = useRef({ form, doc, conflict })
  latest.current = { form, doc, conflict }
  const commit = async (overwrite: boolean): Promise<void> => {
    const { form: current, doc: currentDoc, conflict: currentConflict } = latest.current
    if (!current || !currentDoc) return
    if (!connection.writable) throw new ApiError(0, 'Đang mất kết nối — chưa lưu được.')
    try {
      const saved = await save(current, overwrite && currentConflict ? currentConflict.version ?? currentConflict.current.version : currentDoc.version, currentDoc)
      setDoc(saved); setForm(formOf(saved)); setConflict(null)
      setData(resourceKey, saved)
      void invalidate(['project']); void invalidate(['library'])
    } catch (error) {
      if (error instanceof ConflictError) {
        setConflict(error as ConflictError<Doc>)
        throw new Error('Có bản mới hơn trên máy chủ — xem thanh cảnh báo phía trên.')
      }
      throw error
    }
  }
  const discard = () => { if (doc) setForm(formOf(doc)); setConflict(null) }

  const { registerExtra } = ctx
  const commitRef = useRef(commit); commitRef.current = commit
  const count = changedFields(form, base)
  useEffect(() => {
    registerExtra(slot, count > 0 ? { count, apply: async () => { await commitRef.current(false) }, cancel: discard } : null)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [count, doc])
  useEffect(() => () => registerExtra(slot, null), [registerExtra, slot])

  return { resource, form, setForm, conflict, setConflict, setDoc, commit, formOf, writable: ctx.canEdit }
}

function Banners<Doc extends { version?: string }>({ state, resourceKey }: {
  state: { resource: Resource<Doc>; conflict: ConflictError<Doc> | null; setConflict: (value: null) => void; setDoc: (doc: Doc) => void; setForm: (form: never) => void; commit: (overwrite: boolean) => Promise<void>; formOf: (doc: Doc) => unknown }
  resourceKey: Key
}) {
  const { resource, conflict } = state
  return (
    <>
      <PanelRefreshBar resourceKeys={[resourceKey]} updatedAt={resource.updatedAt} />
      {!!resource.data && <StaleBanner error={resource.error} onRetry={resource.refresh} />}
      {conflict && (
        <ConflictBar pending={false}
          onOverwrite={() => { void state.commit(true).catch(() => undefined) }}
          onView={() => {
            state.setDoc(conflict.current); state.setForm(state.formOf(conflict.current) as never); setData(resourceKey, conflict.current); state.setConflict(null)
          }} />
      )}
    </>
  )
}


export function InfoSection({ ctx }: { ctx: SettingsContext }) {
  const resourceKey = keys.project(ctx.project, 'meta')
  const state = useInfoDoc<ProjectDetail, ProjectForm>({
    ctx, slot: 'info', resourceKey, load: () => getProject(ctx.project), formOf: (detail) => projectFormOf(detail.project),
    save: async (form, version, doc) => {
      const project = await updateProject(ctx.project, form, version)
      return { ...doc, project, version: project.version }
    },
  })
  const { form, setForm } = state
  if (state.resource.error && !state.form) return <Page top={18}><SectionHead title="Thông tin dự án" /><ErrorCard message="Không tải được thông tin dự án." onRetry={state.resource.refresh} /></Page>
  if (!form) return <Page top={18}><SectionHead title="Thông tin dự án" /><LoadingRows rows={3} /></Page>
  const legacy = state.resource.data?.project.legacy ?? false
  const disabled = !state.writable || legacy
  return (
    <Page>
      <SectionHead title="Thông tin dự án" />
      <Banners state={state} resourceKey={resourceKey} />
      <section className="card st-sec" style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)', gap: '14px 16px' }}>
        {PROJECT_FIELDS.map((field) => (
          <Label key={field.key} text={field.label} span>
            {field.kind === 'area'
              ? <textarea className="st-field" rows={3} value={form[field.key]} disabled={disabled}
                onChange={(event) => setForm({ ...form, [field.key]: event.target.value })} style={area} />
              : <input className="st-field" type="text" value={form[field.key]} disabled={disabled} placeholder={field.placeholder}
                onChange={(event) => setForm({ ...form, [field.key]: event.target.value })} style={text38} />}
          </Label>
        ))}
      </section>
    </Page>
  )
}
