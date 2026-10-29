import { humanizeSlug } from '../../shell/labels'
import { ApiError, adoptProject } from '../../api/client'
import { useLibrary } from '../../api/library'
import { useSession } from '../../api/session'
import { keys, useAction, useConnection } from '../../data'
import { Button } from '@noverse/ui'

/** Admins see candidates in the library; other readers get `candidate:true` on the project 404. */
export function useCandidateProject(project: string | null, error?: unknown): boolean {
  const { library } = useLibrary()
  if (!project) return false
  return !!library?.candidates.includes(project)
    || (error instanceof ApiError && (error.payload as { candidate?: unknown } | undefined)?.candidate === true)
}

/** Actionable state for a folder that has stories but no project.json yet (pass the failed request error, if any). */
export function CandidateProjectNotice({ project, error }: { project: string; error?: unknown }) {
  const { user } = useSession()
  const connection = useConnection()
  const candidate = useCandidateProject(project, error)
  const { reload } = useLibrary()
  const adopt = useAction(async () => {
    const adopted = await adoptProject(project)
    reload() // library is shell context state, not a cache key
    return adopted
  }, { success: 'Đã nhận thư mục vào thư viện', invalidate: [keys.project(project)] })
  if (!candidate) return null
  const admin = user?.role === 'admin'
  return <section role="status" style={{ padding: '12px 16px', display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 10, borderBottom: '1px solid var(--line-3)', fontSize: 13 }}>
    <div style={{ flex: '1 1 240px' }}>
      <strong>Thư mục này chưa được nhận vào thư viện</strong>
      <p style={{ margin: '4px 0 0', color: 'var(--color-fg-3)' }}>
        {admin ? `Nhận “${humanizeSlug(project)}” làm dự án để quản lý truyện, ý tưởng và tư liệu trong thư mục.` : 'Nhờ quản trị viên nhận thư mục này vào thư viện để quản lý dự án.'}
      </p>
      {adopt.error instanceof Error && <p role="alert" style={{ margin: '4px 0 0', color: 'var(--color-err)' }}>{adopt.error.message}</p>}
    </div>
    {admin && <Button variant="primary" pending={adopt.pending} pendingLabel="Đang nhận…" disabled={adopt.busy || !connection.writable} tooltip={!connection.writable ? 'Chờ kết nối lại để nhận thư mục.' : undefined} onClick={() => { void adopt.run() }}>Nhận vào thư viện</Button>}
  </section>
}
