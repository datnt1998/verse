// Tuỳ chỉnh › Thông tin truyện / dự án — Settings.dc.html #info. Ghi vào story.json / project.json
// (PUT kèm If-Match); Lưu/Huỷ nằm ở thanh nổi đáy của Tuỳ chỉnh (ctx.registerExtra).
import { useEffect, useRef, useState } from 'react'
import { ApiError, ConflictError, getProject, getStoryMeta, putStoryMeta, updateProject } from '../../../api/client'
import type { ProjectDetail, StoryFile, StoryStatus } from '../../../api/types'
import { invalidate, keys, setData, useConnection, useResource, type Key, type Resource } from '../../../data'
import { ConflictBar } from '@noverse/ui'
import { KitSelect } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { buildRows } from '../../../shell/version'
import { ErrorCard, LoadingRows } from '../components'
import { Page, SectionHead } from '../parts'
import type { SettingsContext } from '../types'
import { shortStoryTitle } from '../../../lib/audioName'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

// ─── Dữ liệu biểu mẫu (thuần, có test) ──────────────────────────────────────────────────────────

export type StoryForm = { title: string; status: StoryStatus; publishedAt: string; tags: string[]; idea: string; shortTitle: string }
export type ProjectForm = { name: string; description: string; notes: string; universeName: string }

/** Nhãn trạng thái đúng artboard (4 lựa chọn); `idea` chỉ hiện khi truyện đang ở trạng thái đó. */
export const STATUS_LABEL: Record<StoryStatus, string> = { idea: 'Ý tưởng', writing: 'Đang viết', paused: 'Tạm ngưng', done: 'Hoàn thành', published: 'Đã đăng' }
const STATUS_ORDER: StoryStatus[] = ['writing', 'paused', 'done', 'published']
export const statusOptions = (current: StoryStatus): StoryStatus[] => (current === 'idea' ? ['idea', ...STATUS_ORDER] : STATUS_ORDER)

const pad = (n: number) => String(n).padStart(2, '0')
/** ISO → dd/MM/yyyy (ngày địa phương); rỗng khi chưa đăng. */
export function dateText(iso: string | null | undefined): string {
  if (!iso) return ''
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? '' : `${pad(date.getDate())}/${pad(date.getMonth() + 1)}/${date.getFullYear()}`
}
/** dd/MM/yyyy → ISO nửa đêm địa phương; rỗng → null; sai định dạng/ngày không có thật → `undefined`. */
export function parseDate(text: string): string | null | undefined {
  const trimmed = text.trim()
  if (!trimmed) return null
  const match = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(trimmed)
  if (!match) return undefined
  const [day, month, year] = [Number(match[1]), Number(match[2]), Number(match[3])]
  const date = new Date(year, month - 1, day)
  if (date.getFullYear() !== year || date.getMonth() !== month - 1 || date.getDate() !== day) return undefined
  return date.toISOString()
}

export const storyFormOf = (meta: StoryFile): StoryForm => ({
  title: meta.title, status: meta.status, publishedAt: dateText(meta.publishedAt), tags: meta.tags ?? [], idea: meta.idea, shortTitle: meta.shortTitle ?? '',
})
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

export function InfoSection({ ctx, scope }: { ctx: SettingsContext; scope: 'story' | 'project' | 'global' }) {
  return scope === 'story' ? <StoryInfo ctx={ctx} /> : <ProjectInfo ctx={ctx} />
}

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

function StoryInfo({ ctx }: { ctx: SettingsContext }) {
  const resourceKey = keys.story(ctx.project, ctx.story, 'meta')
  const state = useInfoDoc<StoryFile, StoryForm>({
    ctx, slot: 'info', resourceKey, load: (signal) => getStoryMeta(ctx.project, ctx.story, signal), formOf: storyFormOf,
    save: async (form, version) => {
      const publishedAt = parseDate(form.publishedAt)
      if (publishedAt === undefined) throw new Error('Ngày đăng cần dạng ngày/tháng/năm, ví dụ 25/10/2026.')
      return putStoryMeta(ctx.project, ctx.story, { title: form.title, status: form.status, idea: form.idea, tags: form.tags, shortTitle: form.shortTitle.trim() || undefined, publishedAt }, version)
    },
  })
  const { form, setForm } = state
  const builds = buildRows(state.resource.data?.builds)
  const [adding, setAdding] = useState(false)
  const [tagText, setTagText] = useState('')
  if (state.resource.error && !state.form) return <Page top={18}><SectionHead title="Thông tin truyện" /><ErrorCard message="Không tải được thông tin truyện." onRetry={state.resource.refresh} /></Page>
  if (!form) return <Page top={18}><SectionHead title="Thông tin truyện" /><LoadingRows rows={4} /></Page>
  const disabled = !state.writable
  const addTag = () => {
    const value = tagText.trim()
    setTagText(''); setAdding(false)
    if (value && !form.tags.includes(value)) setForm({ ...form, tags: [...form.tags, value] })
  }
  return (
    <Page>
      <SectionHead title="Thông tin truyện" />
      <Banners state={state} resourceKey={resourceKey} />
      <section className="card st-sec" style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)', gap: '14px 16px' }}>
        <Label text="Tên truyện" span>
          <input className="st-field" type="text" value={form.title} disabled={disabled} onChange={(event) => setForm({ ...form, title: event.target.value })} style={text38} />
        </Label>
        <Label text="Trạng thái">
          <KitSelect aria-label="Trạng thái" value={form.status} disabled={disabled} height={38}
            options={statusOptions(form.status).map((status) => ({ value: status, label: STATUS_LABEL[status] }))}
            onChange={(value) => setForm({ ...form, status: value as StoryStatus })} />
        </Label>
        <Label text="Tên ngắn (tệp tải về)">
          <input className="st-field" type="text" value={form.shortTitle} disabled={disabled} maxLength={32}
            onChange={(event) => setForm({ ...form, shortTitle: event.target.value })} style={text38}
            placeholder={shortStoryTitle(form.title)} />
          <small style={{ color: 'var(--color-fg-4)', fontSize: 12 }}>Dùng trong “Tên ngắn · Tập N · Chương NN.mp3”; trống = cắt tên truyện ở ranh giới từ, tối đa 32 ký tự.</small>
        </Label>
        <Label text="Ngày đăng">
          <input className="st-field" type="text" value={form.publishedAt} placeholder="Chưa đăng" disabled={disabled}
            aria-invalid={parseDate(form.publishedAt) === undefined || undefined}
            onChange={(event) => setForm({ ...form, publishedAt: event.target.value })} style={text38} />
        </Label>
        <Label text="Thẻ" span>
          <span style={{ display: 'flex', gap: 6, flexWrap: 'wrap', alignItems: 'center', minHeight: 38 }}>
            {form.tags.map((tag) => (
              <button key={tag} type="button" className="st-uchip" disabled={disabled} title="Bấm để bỏ thẻ" aria-label={`Bỏ thẻ ${tag}`}
                onClick={() => setForm({ ...form, tags: form.tags.filter((item) => item !== tag) })}
                style={{ background: 'rgba(201,166,245,0.12)', color: '#DCC8F8', border: 'none' }}>{tag}</button>
            ))}
            {adding
              ? <input className="st-field" autoFocus value={tagText} aria-label="Thêm thẻ" onChange={(event) => setTagText(event.target.value)} onBlur={addTag}
                onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); addTag() } else if (event.key === 'Escape') { setTagText(''); setAdding(false) } }}
                style={{ width: 120, height: 28 }} />
              : !disabled && <button className="st-tpl" type="button" onClick={() => setAdding(true)}>+ thẻ</button>}
          </span>
        </Label>
        <Label text="Ý tưởng ban đầu" span>
          <textarea className="st-field" rows={3} value={form.idea} disabled={disabled} onChange={(event) => setForm({ ...form, idea: event.target.value })} style={area} />
        </Label>
      </section>
      {builds.length > 0 && (
        <section className="card st-sec" style={{ gap: 8 }}>
          <b style={{ fontSize: 14, fontWeight: 500 }}>Viết bằng phiên bản</b>
          <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)', marginTop: -4 }}>Mỗi chương ghi bản Noverse đã viết ra nó, để tra lại khi cần.</span>
          {builds.map((build, index) => (
            <div key={`${build.chapters}/${index}`} style={{
              display: 'flex', alignItems: 'center', gap: 12, minHeight: 36,
              borderTop: '1px solid var(--line-1)', fontSize: 13,
            }}>
              <span className="num" style={{ flex: '0 0 110px', color: 'var(--color-fg)' }}>Noverse{build.version ? ` ${build.version}` : ''}</span>
              <span style={{ flex: '1 1 auto', color: 'var(--color-fg-3)' }}>{build.chapters}</span>
              {build.commit && <span className="mono" style={{ fontSize: 12, color: 'var(--color-fg-4)' }} title={`Bản ghi build ${build.commit}`}>{build.commit}</span>}
              <span className="num" style={{ color: 'var(--color-fg-4)' }}>{build.date}</span>
            </div>
          ))}
        </section>
      )}
    </Page>
  )
}

function ProjectInfo({ ctx }: { ctx: SettingsContext }) {
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
